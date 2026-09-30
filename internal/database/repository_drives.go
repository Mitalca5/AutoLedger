package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/servertext"
)

// TeslaMate drive ingestion, trip groups and per-drive expenses (tolls, parking...).

func (r *Repository) UpsertTeslaMateDrive(ctx context.Context, d *models.Drive) (bool, error) {
	query := `
		INSERT INTO drives (
			vehicle_id, teslamate_drive_id, start_time, end_time,
			start_odometer, end_odometer, distance_km, duration_min,
			speed_avg, speed_max, power_max, power_min, start_address, end_address, energy_consumed_kwh,
			consumption_kwh_100km, tags, is_manual,
			start_battery_level, end_battery_level, outside_temp_c
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, FALSE, $18, $19, $20)
		ON CONFLICT (vehicle_id, teslamate_drive_id) DO UPDATE
		SET start_time = EXCLUDED.start_time,
		    end_time = EXCLUDED.end_time,
		    start_odometer = EXCLUDED.start_odometer,
		    end_odometer = EXCLUDED.end_odometer,
		    distance_km = EXCLUDED.distance_km,
		    duration_min = EXCLUDED.duration_min,
		    speed_avg = EXCLUDED.speed_avg,
		    speed_max = EXCLUDED.speed_max,
		    power_max = EXCLUDED.power_max,
		    power_min = EXCLUDED.power_min,
		    start_address = EXCLUDED.start_address,
		    end_address = EXCLUDED.end_address,
		    energy_consumed_kwh = EXCLUDED.energy_consumed_kwh,
		    consumption_kwh_100km = EXCLUDED.consumption_kwh_100km,
		    start_battery_level = EXCLUDED.start_battery_level,
		    end_battery_level = EXCLUDED.end_battery_level,
		    outside_temp_c = EXCLUDED.outside_temp_c,
		    deleted_upstream_at = NULL,
		    updated_at = NOW()
		RETURNING id, (xmax = 0) AS is_inserted;
	`
	var isInserted bool
	err := r.pool.QueryRow(ctx, query,
		d.VehicleID, d.TeslaMateDriveID, d.StartTime, d.EndTime,
		d.StartOdometer, d.EndOdometer, d.DistanceKm, d.DurationMin,
		d.SpeedAvg, d.SpeedMax, d.PowerMax, d.PowerMin,
		d.StartAddress, d.EndAddress, d.EnergyConsumedKwh,
		d.ConsumptionKwh100km, d.Tags,
		d.StartBatteryLevel, d.EndBatteryLevel, d.OutsideTempC,
	).Scan(&d.ID, &isInserted)
	return isInserted, err
}

func (r *Repository) GetLatestTeslaMateDriveStartTime(ctx context.Context, vehicleID string) (*time.Time, error) {
	query := `
		SELECT start_time
		FROM drives
		WHERE vehicle_id = $1 AND teslamate_drive_id IS NOT NULL AND deleted_upstream_at IS NULL
		ORDER BY start_time DESC
		LIMIT 1;
	`
	var t time.Time
	err := r.pool.QueryRow(ctx, query, vehicleID).Scan(&t)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// DriveFilter narrows drive listings.
type DriveFilter struct {
	Tag             string
	UnqualifiedOnly bool
	HasToll         bool
	TollSource      string // "", ExpenseSourceManual or ExpenseSourceAutoToll (only with HasToll)
	TripGroupID     string
	DriveID         string // a single drive, whatever the other filters
	From            *time.Time
	To              *time.Time
	Query           string
}

// HighwayDrivePredicate matches drives likely to have used toll roads. It mirrors models.Drive.IsHighway
// (kept in sync by an integration test), and also matches any drive whose GPS toll detection found a toll segment:
// the trace is the ground truth when the speed heuristic misses a short highway drive.
const HighwayDrivePredicate = `((drives.distance_km >= 40 AND COALESCE(drives.speed_avg, 0) >= 70)
	OR (drives.distance_km >= 20 AND COALESCE(drives.speed_max, 0) > 125)
	OR (drives.distance_km >= 20 AND COALESCE(drives.speed_max, 0) >= 110 AND COALESCE(drives.speed_avg, 0) >= 70)
	OR (drives.distance_km >= 8 AND COALESCE(drives.speed_max, 0) >= 105 AND COALESCE(drives.speed_avg, 0) >= 70)
	OR EXISTS (SELECT 1 FROM toll_detections td WHERE td.drive_id = drives.id
	           AND CASE WHEN jsonb_typeof(td.segments) = 'array' THEN jsonb_array_length(td.segments) ELSE 0 END > 0))`

// Highway-like drives with no toll attached and no explicit "no toll" review.
const UnqualifiedDrivePredicate = HighwayDrivePredicate + `
	AND drives.toll_reviewed_at IS NULL
	AND NOT EXISTS (SELECT 1 FROM drive_expenses de WHERE de.drive_id = drives.id)
	AND NOT EXISTS (
		SELECT 1 FROM drive_expenses de
		JOIN trip_group_drives tgd ON tgd.trip_group_id = de.trip_group_id
		WHERE tgd.drive_id = drives.id
	)
`

// tollDrivePredicate matches drives covered by a TOLL expense, attached directly or through a trip group.
// sourceCond, when non-empty, further restricts the expense (e.g. "de.source = $3").
func tollDrivePredicate(sourceCond string) string {
	if sourceCond != "" {
		sourceCond = " AND " + sourceCond
	}
	return `EXISTS (
		SELECT 1 FROM drive_expenses de
		WHERE de.type = 'TOLL'` + sourceCond + `
		  AND (de.drive_id = drives.id
		       OR de.trip_group_id IN (SELECT tgd.trip_group_id FROM trip_group_drives tgd WHERE tgd.drive_id = drives.id))
	)`
}

func (r *Repository) ListDrives(ctx context.Context, vehicleID string, filter DriveFilter, limit, offset int) ([]models.Drive, int, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("vehicle_id = $%d", argIdx))
	args = append(args, vehicleID)
	argIdx++

	conditions = append(conditions, "deleted_upstream_at IS NULL")

	if filter.Tag != "" {
		conditions = append(conditions, fmt.Sprintf("$%d = ANY(tags)", argIdx))
		args = append(args, filter.Tag)
		argIdx++
	}

	if filter.UnqualifiedOnly {
		conditions = append(conditions, "("+UnqualifiedDrivePredicate+")")
	}

	if filter.HasToll {
		sourceCond := ""
		if filter.TollSource != "" {
			sourceCond = fmt.Sprintf("de.source = $%d", argIdx)
			args = append(args, filter.TollSource)
			argIdx++
		}
		conditions = append(conditions, tollDrivePredicate(sourceCond))
	}

	if filter.DriveID != "" {
		conditions = append(conditions, fmt.Sprintf("id::text = $%d", argIdx))
		args = append(args, filter.DriveID)
		argIdx++
	}

	if filter.TripGroupID != "" {
		conditions = append(conditions, fmt.Sprintf("id IN (SELECT drive_id FROM trip_group_drives WHERE trip_group_id::text = $%d::text)", argIdx))
		args = append(args, filter.TripGroupID)
		argIdx++
	}

	if filter.From != nil {
		conditions = append(conditions, fmt.Sprintf("start_time >= $%d", argIdx))
		args = append(args, *filter.From)
		argIdx++
	}

	if filter.To != nil {
		conditions = append(conditions, fmt.Sprintf("start_time <= $%d", argIdx))
		args = append(args, *filter.To)
		argIdx++
	}

	if strings.TrimSpace(filter.Query) != "" {
		pattern := "%" + strings.TrimSpace(filter.Query) + "%"
		conditions = append(conditions, fmt.Sprintf("(start_address ILIKE $%d OR end_address ILIKE $%d)", argIdx, argIdx))
		args = append(args, pattern)
		argIdx++
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int
	countQuery := "SELECT COUNT(*) FROM drives WHERE " + whereClause
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, vehicle_id, teslamate_drive_id, start_time, end_time,
		       start_odometer, end_odometer, distance_km, duration_min,
		       speed_avg, speed_max, power_max, power_min, start_address, end_address, energy_consumed_kwh,
		       consumption_kwh_100km, tags, is_manual, energy_estimated, toll_reviewed_at, created_at, updated_at
		FROM drives
		WHERE ` + whereClause + fmt.Sprintf(" ORDER BY start_time DESC LIMIT $%d OFFSET $%d;", argIdx, argIdx+1)

	queryArgs := append(args, limit, offset)
	rows, err := r.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []models.Drive
	for rows.Next() {
		var d models.Drive
		if err := rows.Scan(
			&d.ID, &d.VehicleID, &d.TeslaMateDriveID, &d.StartTime, &d.EndTime,
			&d.StartOdometer, &d.EndOdometer, &d.DistanceKm, &d.DurationMin,
			&d.SpeedAvg, &d.SpeedMax, &d.PowerMax, &d.PowerMin,
			&d.StartAddress, &d.EndAddress, &d.EnergyConsumedKwh,
			&d.ConsumptionKwh100km, &d.Tags, &d.IsManual, &d.EnergyEstimated, &d.TollReviewedAt, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, d)
	}
	return list, total, rows.Err()
}

// CountUnqualifiedDrives counts highway-like drives still waiting for a toll qualification.
func (r *Repository) CountUnqualifiedDrives(ctx context.Context, vehicleID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM drives WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL AND `+UnqualifiedDrivePredicate, vehicleID).Scan(&count)
	return count, err
}

// SetDriveTollReviewed marks (or unmarks) a drive as explicitly reviewed without toll.
func (r *Repository) SetDriveTollReviewed(ctx context.Context, driveID, vehicleID string, reviewed bool) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE drives
		SET toll_reviewed_at = CASE WHEN $1 THEN NOW() ELSE NULL END, updated_at = NOW()
		WHERE id::text = $2 AND vehicle_id = $3;
	`, reviewed, driveID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateDriveTags(ctx context.Context, driveID, vehicleID string, tags []string) error {
	query := `
		UPDATE drives
		SET tags = $1, updated_at = NOW()
		WHERE id = $2 AND vehicle_id = $3;
	`
	tag, err := r.pool.Exec(ctx, query, tags, driveID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CreateTripGroup(ctx context.Context, tg *models.TripGroup, driveIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := insertTripGroup(ctx, tx, tg, driveIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertTripGroup(ctx context.Context, tx pgx.Tx, tg *models.TripGroup, driveIDs []string) error {
	if err := ensureDrivesOwned(ctx, tx, tg.VehicleID, driveIDs); err != nil {
		return err
	}

	query := `
		INSERT INTO trip_groups (vehicle_id, name, notes)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at;
	`
	if err := tx.QueryRow(ctx, query, tg.VehicleID, tg.Name, tg.Notes).Scan(&tg.ID, &tg.CreatedAt, &tg.UpdatedAt); err != nil {
		return err
	}
	return linkTripGroupDrives(ctx, tx, tg.ID, driveIDs)
}

// linkTripGroupDrives links drives to a trip group, ordered chronologically.
func linkTripGroupDrives(ctx context.Context, tx pgx.Tx, tripGroupID string, driveIDs []string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO trip_group_drives (trip_group_id, drive_id, order_index)
		SELECT $1, d.id, ROW_NUMBER() OVER (ORDER BY d.start_time) - 1
		FROM drives d
		WHERE d.id::text = ANY($2::text[]) AND d.deleted_upstream_at IS NULL;
	`, tripGroupID, uniqueStrings(driveIDs))
	if err != nil {
		return fmt.Errorf("failed to link drives to trip group: %w", err)
	}
	return nil
}

func (r *Repository) ListTripGroups(ctx context.Context, vehicleID string) ([]models.TripGroup, error) {
	rows, err := r.pool.Query(ctx, DriveTollAllocationCTE+`
		SELECT tg.id, tg.vehicle_id, tg.name, tg.notes, tg.created_at, tg.updated_at,
		       ARRAY(SELECT tgd.drive_id::text FROM trip_group_drives tgd
		             JOIN drives d ON d.id = tgd.drive_id AND d.deleted_upstream_at IS NULL
		             WHERE tgd.trip_group_id = tg.id ORDER BY d.start_time),
		       COALESCE(stats.km, 0), stats.first_start, stats.last_end,
		       COALESCE((SELECT SUM(`+amountInVehicleCurrencyExpr("e.vehicle_id")+`) FROM drive_expenses e WHERE e.trip_group_id = tg.id), 0),
		       (SELECT COUNT(*) FROM drive_expenses e WHERE e.trip_group_id = tg.id),
		       (SELECT COUNT(*) FROM carpool_trips c WHERE c.trip_group_id = tg.id),
		       COALESCE((SELECT SUM(a.allocated) FROM allocations a
		                 WHERE a.drive_id IN (SELECT tgd.drive_id FROM trip_group_drives tgd WHERE tgd.trip_group_id = tg.id)), 0)
		FROM trip_groups tg
		LEFT JOIN LATERAL (
			SELECT SUM(d.distance_km) AS km, MIN(d.start_time) AS first_start, MAX(d.end_time) AS last_end
			FROM trip_group_drives tgd
			JOIN drives d ON d.id = tgd.drive_id AND d.deleted_upstream_at IS NULL
			WHERE tgd.trip_group_id = tg.id
		) stats ON TRUE
		WHERE tg.vehicle_id = $1
		ORDER BY COALESCE(stats.first_start, tg.created_at) DESC;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.TripGroup
	for rows.Next() {
		var tg models.TripGroup
		if err := rows.Scan(&tg.ID, &tg.VehicleID, &tg.Name, &tg.Notes, &tg.CreatedAt, &tg.UpdatedAt,
			&tg.DriveIDs, &tg.DistanceKm, &tg.StartTime, &tg.EndTime, &tg.ExpensesTotal, &tg.ExpenseCount, &tg.CarpoolCount, &tg.TollsTotal); err != nil {
			return nil, err
		}
		list = append(list, tg)
	}
	return list, rows.Err()
}

// UpdateTripGroup renames a trip group and, when driveIDs is not nil, replaces its drives.
func (r *Repository) UpdateTripGroup(ctx context.Context, tg *models.TripGroup, driveIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE trip_groups SET name = $1, notes = $2, updated_at = NOW()
		WHERE id::text = $3 AND vehicle_id = $4;
	`, tg.Name, tg.Notes, tg.ID, tg.VehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if driveIDs != nil {
		if len(uniqueStrings(driveIDs)) == 0 {
			return apierror.New("trip.needs_drive", "A trip must contain at least one drive")
		}
		if err := ensureDrivesOwned(ctx, tx, tg.VehicleID, driveIDs); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trip_group_drives WHERE trip_group_id::text = $1;`, tg.ID); err != nil {
			return err
		}
		if err := linkTripGroupDrives(ctx, tx, tg.ID, driveIDs); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// DeleteTripGroup deletes a trip group. Its expenses are kept (no longer linked to drives) unless deleteExpenses is set.
func (r *Repository) DeleteTripGroup(ctx context.Context, vehicleID, tripGroupID string, deleteExpenses bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := ensureTripGroupOwned(ctx, tx, vehicleID, tripGroupID); err != nil {
		return ErrNotFound
	}
	if deleteExpenses {
		if _, err := tx.Exec(ctx, `DELETE FROM drive_expenses WHERE trip_group_id::text = $1 AND vehicle_id = $2;`, tripGroupID, vehicleID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM trip_groups WHERE id::text = $1 AND vehicle_id = $2;`, tripGroupID, vehicleID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ============================================================================
// Drive Expenses (Tolls, Parking)
// ============================================================================

// amountInVehicleCurrencyExpr converts an expense amount to its vehicle's own currency (NULL when a
// foreign amount has no conversion rate). vehicleIDExpr is a trusted SQL fragment — a column
// reference or bound parameter already scoped to the right vehicle in the surrounding query, never
// user input — naming the vehicle to compare currencies against.
func amountInVehicleCurrencyExpr(vehicleIDExpr string) string {
	return fmt.Sprintf(`(CASE WHEN currency = (SELECT currency FROM vehicles WHERE id = %s) THEN amount ELSE amount * fx_rate END)`, vehicleIDExpr)
}

// SaveDriveExpense creates (exp.ID empty) or updates a drive expense in a single transaction.
// When groupDriveIDs contains several drives, the expense is attached to a trip group: the group
// previously dedicated to this expense is resynchronized, otherwise a new group named groupName is created.
func (r *Repository) SaveDriveExpense(ctx context.Context, exp *models.DriveExpense, groupDriveIDs []string, groupName string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var previousGroupID *string
	if exp.ID != "" {
		err := tx.QueryRow(ctx, `
			SELECT trip_group_id FROM drive_expenses WHERE id::text = $1 AND vehicle_id = $2 FOR UPDATE;
		`, exp.ID, exp.VehicleID).Scan(&previousGroupID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
	}

	ids := uniqueStrings(groupDriveIDs)
	switch {
	case len(ids) > 1:
		if err := ensureDrivesOwned(ctx, tx, exp.VehicleID, ids); err != nil {
			return err
		}
		groupID, err := r.reusableExpenseGroup(ctx, tx, exp, previousGroupID)
		if err != nil {
			return err
		}
		if groupID != "" {
			if _, err := tx.Exec(ctx, `DELETE FROM trip_group_drives WHERE trip_group_id = $1;`, groupID); err != nil {
				return err
			}
			if err := linkTripGroupDrives(ctx, tx, groupID, ids); err != nil {
				return err
			}
		} else {
			tg := &models.TripGroup{VehicleID: exp.VehicleID, Name: groupName, Notes: exp.Notes}
			if err := insertTripGroup(ctx, tx, tg, ids); err != nil {
				return err
			}
			groupID = tg.ID
		}
		exp.TripGroupID = &groupID
		exp.DriveID = nil
	case len(ids) == 1:
		exp.DriveID = &ids[0]
		exp.TripGroupID = nil
	}

	if exp.DriveID != nil && *exp.DriveID == "" {
		exp.DriveID = nil
	}
	if exp.TripGroupID != nil && *exp.TripGroupID == "" {
		exp.TripGroupID = nil
	}
	if exp.TripGroupID != nil {
		exp.DriveID = nil
		if err := ensureTripGroupOwned(ctx, tx, exp.VehicleID, *exp.TripGroupID); err != nil {
			return err
		}
	}
	if exp.DriveID != nil {
		if err := ensureDrivesOwned(ctx, tx, exp.VehicleID, []string{*exp.DriveID}); err != nil {
			return err
		}
	}

	if exp.Source == "" {
		exp.Source = models.ExpenseSourceManual
	}

	if exp.ID == "" {
		err = tx.QueryRow(ctx, `
			INSERT INTO drive_expenses (vehicle_id, trip_group_id, drive_id, type, amount, currency, fx_rate, date, notes, document_id, source)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id, created_at;
		`, exp.VehicleID, exp.TripGroupID, exp.DriveID, exp.Type,
			exp.Amount, exp.Currency, exp.FxRate, exp.Date, exp.Notes, exp.DocumentID, exp.Source,
		).Scan(&exp.ID, &exp.CreatedAt)
	} else {
		err = tx.QueryRow(ctx, `
			UPDATE drive_expenses
			SET trip_group_id = $1, drive_id = $2, type = $3, amount = $4,
			    currency = $5, fx_rate = $6, date = $7, notes = $8, document_id = $9, source = $10
			WHERE id::text = $11 AND vehicle_id = $12
			RETURNING created_at;
		`, exp.TripGroupID, exp.DriveID, exp.Type, exp.Amount,
			exp.Currency, exp.FxRate, exp.Date, exp.Notes, exp.DocumentID, exp.Source,
			exp.ID, exp.VehicleID,
		).Scan(&exp.CreatedAt)
	}
	if err != nil {
		return err
	}

	if exp.DocumentID != nil {
		_ = tx.QueryRow(ctx, `SELECT filename FROM expense_documents WHERE id = $1;`, *exp.DocumentID).Scan(&exp.DocumentFilename)
	}

	return tx.Commit(ctx)
}

// reusableExpenseGroup returns the expense's current trip group when no other expense or carpool uses it.
func (r *Repository) reusableExpenseGroup(ctx context.Context, tx pgx.Tx, exp *models.DriveExpense, previousGroupID *string) (string, error) {
	if previousGroupID == nil {
		return "", nil
	}
	var shared bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM drive_expenses WHERE trip_group_id = $1 AND id::text <> $2)
		    OR EXISTS(SELECT 1 FROM carpool_trips WHERE trip_group_id = $1);
	`, *previousGroupID, exp.ID).Scan(&shared)
	if err != nil {
		return "", err
	}
	if shared {
		return "", nil
	}
	return *previousGroupID, nil
}

// tripLabelWords is the fallback wording for a drive missing a reverse-geocoded address, in the
// acting user's language: the CASE below returns a display string built at read time, not a
// code, so it is translated here rather than by the frontend.
func tripLabelWords(lang string) (start, end string) {
	return servertext.Text(lang, "trip.departure"), servertext.Text(lang, "trip.arrival")
}

func (r *Repository) ListDriveExpenses(ctx context.Context, vehicleID, lang string) ([]models.DriveExpense, error) {
	start, end := tripLabelWords(lang)
	query := `
		SELECT
			e.id, e.vehicle_id, e.trip_group_id, tg.name,
			ARRAY(SELECT tgd.drive_id::text FROM trip_group_drives tgd WHERE tgd.trip_group_id = e.trip_group_id ORDER BY tgd.order_index),
			e.drive_id,
			CASE
				WHEN d.id IS NOT NULL THEN COALESCE(NULLIF(d.start_address, ''), $2) || ' → ' || COALESCE(NULLIF(d.end_address, ''), $3)
				ELSE NULL
			END,
			e.type, e.amount, e.currency, e.fx_rate, e.date, e.notes,
			e.document_id, doc.filename,
			e.source, e.created_at
		FROM drive_expenses e
		LEFT JOIN drives d ON e.drive_id = d.id AND d.vehicle_id = e.vehicle_id
		LEFT JOIN trip_groups tg ON e.trip_group_id = tg.id AND tg.vehicle_id = e.vehicle_id
		LEFT JOIN expense_documents doc ON e.document_id = doc.id
		WHERE e.vehicle_id = $1
		ORDER BY e.date DESC;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.DriveExpense
	for rows.Next() {
		var e models.DriveExpense
		if err := rows.Scan(
			&e.ID, &e.VehicleID, &e.TripGroupID, &e.TripGroupName, &e.TripGroupDriveIDs,
			&e.DriveID, &e.DriveTitle, &e.Type,
			&e.Amount, &e.Currency, &e.FxRate, &e.Date, &e.Notes,
			&e.DocumentID, &e.DocumentFilename,
			&e.Source, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

func (r *Repository) DeleteDriveExpense(ctx context.Context, vehicleID, expenseID string) error {
	query := `DELETE FROM drive_expenses WHERE id::text = $1 AND vehicle_id = $2;`
	cmdTag, err := r.pool.Exec(ctx, query, expenseID, vehicleID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CreateManualDrive(ctx context.Context, d *models.Drive) error {
	if d.Tags == nil {
		d.Tags = []string{}
	}
	query := `
		INSERT INTO drives (
			vehicle_id, start_time, end_time,
			start_odometer, end_odometer, distance_km, duration_min,
			start_address, end_address, energy_consumed_kwh,
			consumption_kwh_100km, tags, is_manual, energy_estimated
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, TRUE, $13)
		RETURNING id, created_at, updated_at;
	`
	return r.pool.QueryRow(ctx, query,
		d.VehicleID, d.StartTime, d.EndTime,
		d.StartOdometer, d.EndOdometer, d.DistanceKm, d.DurationMin,
		d.StartAddress, d.EndAddress, d.EnergyConsumedKwh,
		d.ConsumptionKwh100km, d.Tags, d.EnergyEstimated,
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
}

func (r *Repository) UpdateManualDrive(ctx context.Context, d *models.Drive) error {
	if d.Tags == nil {
		d.Tags = []string{}
	}
	query := `
		UPDATE drives
		SET start_time = $1, end_time = $2,
		    start_odometer = $3, end_odometer = $4,
		    distance_km = $5, duration_min = $6,
		    start_address = $7, end_address = $8,
		    energy_consumed_kwh = $9, consumption_kwh_100km = $10, energy_estimated = $11,
		    tags = $12, updated_at = NOW()
		WHERE id = $13 AND vehicle_id = $14 AND is_manual = TRUE;
	`
	tag, err := r.pool.Exec(ctx, query,
		d.StartTime, d.EndTime,
		d.StartOdometer, d.EndOdometer,
		d.DistanceKm, d.DurationMin,
		d.StartAddress, d.EndAddress,
		d.EnergyConsumedKwh, d.ConsumptionKwh100km, d.EnergyEstimated,
		d.Tags, d.ID, d.VehicleID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteManualDrive(ctx context.Context, driveID, vehicleID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM drives WHERE id = $1 AND vehicle_id = $2 AND is_manual = TRUE;`, driveID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// Tires, Wear Logs & Rotations
// ============================================================================

const tireColumns = `
	id, vehicle_id, brand, model, dimension, season,
	purchase_date, purchase_price, current_position,
	initial_depth_mm, min_legal_depth_mm, dot_code, is_archived,
	mounted_odometer, initial_distance_km, accumulated_distance_km, estimated_lifespan_km,
	created_at, updated_at
`

func scanTire(row pgx.Row, t *models.Tire) error {
	return row.Scan(
		&t.ID, &t.VehicleID, &t.Brand, &t.Model, &t.Dimension, &t.Season,
		&t.PurchaseDate, &t.PurchasePrice, &t.CurrentPosition,
		&t.InitialDepthMm, &t.MinLegalDepthMm, &t.DotCode, &t.IsArchived,
		&t.MountedOdometer, &t.InitialDistanceKm, &t.AccumulatedDistanceKm, &t.EstimatedLifespanKm,
		&t.CreatedAt, &t.UpdatedAt,
	)
}

func isMountedPosition(pos models.TirePosition) bool {
	switch pos {
	case models.TirePosFL, models.TirePosFR, models.TirePosRL, models.TirePosRR:
		return true
	}
	return false
}

func isValidTirePosition(pos models.TirePosition) bool {
	return isMountedPosition(pos) || pos == models.TirePosStorage || pos == models.TirePosDisposed
}
