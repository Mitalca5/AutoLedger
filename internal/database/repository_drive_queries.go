package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// Cross-domain drive/toll/telemetry read queries used by carpool and TCO reporting.

func (r *Repository) GetDriveByID(ctx context.Context, driveID, vehicleID string) (*models.Drive, error) {
	query := `
		SELECT id, vehicle_id, teslamate_drive_id, start_time, end_time,
		       start_odometer, end_odometer, distance_km, duration_min,
		       speed_avg, speed_max, power_max, power_min, start_address, end_address, energy_consumed_kwh,
		       consumption_kwh_100km, tags, is_manual, energy_estimated, toll_reviewed_at, created_at, updated_at
		FROM drives
		WHERE id::text = $1 AND vehicle_id = $2 AND deleted_upstream_at IS NULL;
	`
	var d models.Drive
	err := r.pool.QueryRow(ctx, query, driveID, vehicleID).Scan(
		&d.ID, &d.VehicleID, &d.TeslaMateDriveID, &d.StartTime, &d.EndTime,
		&d.StartOdometer, &d.EndOdometer, &d.DistanceKm, &d.DurationMin,
		&d.SpeedAvg, &d.SpeedMax, &d.PowerMax, &d.PowerMin,
		&d.StartAddress, &d.EndAddress, &d.EnergyConsumedKwh,
		&d.ConsumptionKwh100km, &d.Tags, &d.IsManual, &d.EnergyEstimated, &d.TollReviewedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (r *Repository) GetTripGroupDrives(ctx context.Context, vehicleID, tripGroupID string) ([]models.Drive, error) {
	query := `
		SELECT d.id, d.vehicle_id, d.teslamate_drive_id, d.start_time, d.end_time,
		       d.start_odometer, d.end_odometer, d.distance_km, d.duration_min,
		       d.speed_avg, d.speed_max, d.power_max, d.power_min, d.start_address, d.end_address, d.energy_consumed_kwh,
		       d.consumption_kwh_100km, d.tags, d.is_manual, d.energy_estimated, d.toll_reviewed_at, d.created_at, d.updated_at
		FROM drives d
		JOIN trip_group_drives tgd ON d.id = tgd.drive_id
		JOIN trip_groups tg ON tg.id = tgd.trip_group_id
		WHERE tgd.trip_group_id::text = $1 AND tg.vehicle_id = $2 AND d.vehicle_id = $2 AND d.deleted_upstream_at IS NULL
		ORDER BY tgd.order_index ASC;
	`
	rows, err := r.pool.Query(ctx, query, tripGroupID, vehicleID)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

// DriveTollAllocationCTE allocates drive expenses (in the vehicle's own currency) to drives of vehicle $1: an expense attached to
// a drive counts fully for it; an expense attached to a trip group is split across the group's drives
// proportionally to their distance (equally when the group has no distance).
var DriveTollAllocationCTE = `
	WITH group_stats AS (
		SELECT tgd.trip_group_id, SUM(d.distance_km) AS km, COUNT(*) AS n
		FROM trip_group_drives tgd
		JOIN trip_groups tg ON tg.id = tgd.trip_group_id AND tg.vehicle_id = $1
		JOIN drives d ON d.id = tgd.drive_id AND d.deleted_upstream_at IS NULL
		GROUP BY tgd.trip_group_id
	),
	allocations AS (
		SELECT e.id AS expense_id, e.drive_id, ` + amountInVehicleCurrencyExpr("e.vehicle_id") + ` AS allocated
		FROM drive_expenses e
		JOIN drives d ON d.id = e.drive_id AND d.deleted_upstream_at IS NULL
		WHERE e.vehicle_id = $1
		UNION ALL
		SELECT e.id, tgd.drive_id,
		       ` + amountInVehicleCurrencyExpr("e.vehicle_id") + ` * CASE WHEN gs.km > 0 THEN d.distance_km / gs.km ELSE 1.0 / gs.n END
		FROM drive_expenses e
		JOIN group_stats gs ON gs.trip_group_id = e.trip_group_id
		JOIN trip_group_drives tgd ON tgd.trip_group_id = e.trip_group_id
		JOIN drives d ON d.id = tgd.drive_id AND d.deleted_upstream_at IS NULL
		WHERE e.vehicle_id = $1
	)
`

// GetTollExpensesForDrives returns the drive expenses allocated to each drive (in the vehicle's currency).
func (r *Repository) GetTollExpensesForDrives(ctx context.Context, vehicleID string, driveIDs []string) (map[string]money.Cents, error) {
	result := make(map[string]money.Cents)
	ids := uniqueStrings(driveIDs)
	if len(ids) == 0 {
		return result, nil
	}

	rows, err := r.pool.Query(ctx, DriveTollAllocationCTE+`
		SELECT drive_id::text, COALESCE(SUM(allocated), 0)
		FROM allocations
		WHERE drive_id::text = ANY($2::text[])
		GROUP BY drive_id;
	`, vehicleID, ids)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var dID string
		var amt money.Cents
		if err := rows.Scan(&dID, &amt); err != nil {
			return result, err
		}
		result[dID] = amt
	}
	return result, rows.Err()
}

// DrivesNeedingTollQualification returns which of the drives are in the "to qualify" toll queue: the same rule as
// the unqualified filter of the drives list (UnqualifiedDrivePredicate).
func (r *Repository) DrivesNeedingTollQualification(ctx context.Context, vehicleID string, driveIDs []string) (map[string]bool, error) {
	result := make(map[string]bool)
	ids := uniqueStrings(driveIDs)
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT drives.id::text FROM drives
		WHERE drives.vehicle_id = $1 AND drives.id::text = ANY($2::text[]) AND (`+UnqualifiedDrivePredicate+`)
	`, vehicleID, ids)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return result, err
		}
		result[id] = true
	}
	return result, rows.Err()
}

// GetTotalTollExpensesForDrives sums the expenses allocated to a set of drives (in the vehicle's currency).
func (r *Repository) GetTotalTollExpensesForDrives(ctx context.Context, vehicleID string, driveIDs []string) (money.Cents, error) {
	perDrive, err := r.GetTollExpensesForDrives(ctx, vehicleID, driveIDs)
	var total money.Cents
	for _, amt := range perDrive {
		total += amt
	}
	return total, err
}

// GetTollExpensesForTripGroup sums the expenses allocated to the drives of a trip group (in the vehicle's currency).
func (r *Repository) GetTollExpensesForTripGroup(ctx context.Context, vehicleID, tripGroupID string) (money.Cents, error) {
	drives, err := r.GetTripGroupDrives(ctx, vehicleID, tripGroupID)
	if err != nil {
		return 0, err
	}
	ids := make([]string, len(drives))
	for i, d := range drives {
		ids[i] = d.ID
	}
	return r.GetTotalTollExpensesForDrives(ctx, vehicleID, ids)
}

// GetDriveExpensesByDriveID lists expenses attached to a drive directly or through its trip groups,
// with the share allocated to this drive.
func (r *Repository) GetDriveExpensesByDriveID(ctx context.Context, vehicleID, driveID, lang string) ([]models.DriveExpense, error) {
	start, end := tripLabelWords(lang)
	rows, err := r.pool.Query(ctx, DriveTollAllocationCTE+`
		SELECT
			e.id, e.vehicle_id, e.trip_group_id, tg.name,
			e.drive_id,
			CASE
				WHEN d.id IS NOT NULL THEN COALESCE(NULLIF(d.start_address, ''), $3) || ' → ' || COALESCE(NULLIF(d.end_address, ''), $4)
				ELSE NULL
			END,
			e.type, e.amount, e.currency, e.fx_rate, e.date, e.notes,
			e.document_id, doc.filename,
			e.source, e.created_at,
			a.allocated
		FROM allocations a
		JOIN drive_expenses e ON e.id = a.expense_id
		LEFT JOIN drives d ON e.drive_id = d.id
		LEFT JOIN trip_groups tg ON e.trip_group_id = tg.id
		LEFT JOIN expense_documents doc ON e.document_id = doc.id
		WHERE a.drive_id::text = $2
		ORDER BY e.date ASC;
	`, vehicleID, driveID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.DriveExpense
	for rows.Next() {
		var de models.DriveExpense
		if err := rows.Scan(
			&de.ID, &de.VehicleID, &de.TripGroupID, &de.TripGroupName,
			&de.DriveID, &de.DriveTitle, &de.Type,
			&de.Amount, &de.Currency, &de.FxRate, &de.Date, &de.Notes,
			&de.DocumentID, &de.DocumentFilename,
			&de.Source, &de.CreatedAt,
			&de.AllocatedAmount,
		); err != nil {
			return nil, err
		}
		list = append(list, de)
	}
	return list, rows.Err()
}

// OdometerRange represents a half-open interval [Min, Max) of vehicle odometer readings.
// Max == nil means the session is still active (no upper bound).
type OdometerRange struct {
	Min float64
	Max *float64
}

// buildDrivingTelemetryQuery constructs the SQL query and arguments for telemetry stats.
func buildDrivingTelemetryQuery(vehicleID string, ranges []OdometerRange) (string, []any) {
	if len(ranges) == 0 {
		return "", nil
	}

	args := []any{vehicleID}
	var clauses []string
	for _, rng := range ranges {
		lo := len(args) + 1
		args = append(args, rng.Min)
		if rng.Max != nil {
			hi := len(args) + 1
			args = append(args, *rng.Max)
			clauses = append(clauses, fmt.Sprintf("(end_odometer >= $%d AND end_odometer <= $%d)", lo, hi))
		} else {
			// Active session — upper bound is the vehicle's current odometer (no constraint needed)
			clauses = append(clauses, fmt.Sprintf("(end_odometer >= $%d)", lo))
		}
	}

	query := fmt.Sprintf(`
		SELECT
			COALESCE(AVG(NULLIF(power_max, 0)), 0),
			COALESCE(AVG(NULLIF(power_min, 0)), 0),
			COALESCE(AVG(NULLIF(consumption_kwh_100km, 0)), 0),
			COUNT(*)
		FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL
		  AND (%s)
	`, strings.Join(clauses, " OR "))

	return query, args
}

// GetDrivingTelemetryStats returns driving dynamics averaged over the provided odometer ranges.
// Each range corresponds to a tire mount session: only drives whose end_odometer falls within
// [range.Min, range.Max) (or >= range.Min when Max is nil) are included.
// Returns zeros and count=0 when ranges is empty or no matching drives exist.
func (r *Repository) GetDrivingTelemetryStats(ctx context.Context, vehicleID string, ranges []OdometerRange) (avgPowerMax, avgPowerMin, avgConsumption float64, count int, err error) {
	if len(ranges) == 0 {
		return 0, 0, 0, 0, nil
	}

	query, args := buildDrivingTelemetryQuery(vehicleID, ranges)

	err = r.pool.QueryRow(ctx, query, args...).Scan(&avgPowerMax, &avgPowerMin, &avgConsumption, &count)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	return avgPowerMax, avgPowerMin, avgConsumption, count, nil
}

// ============================================================================
// TeslaMate Reconciliation
// ============================================================================

// ReconcileResult reports records no longer present in TeslaMate.
type ReconcileResult struct {
	Missing int  // TeslaMate records of the covered window absent from the API response
	Marked  int  // Records flagged as deleted upstream
	Skipped bool // Guard triggered: too many records would disappear at once
}

// Share of the covered window above which missing records are considered an API anomaly rather than deletions.
const maxUpstreamDeletionShare = 0.2

// ListTripCandidateDrives returns the vehicle's drives since a date that were not ruled out as part of a trip,
// with whether each already belongs to a trip group.
func (r *Repository) ListTripCandidateDrives(ctx context.Context, vehicleID string, since time.Time) ([]models.TripCandidateDrive, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT d.id::text, d.start_time, COALESCE(d.end_time, d.start_time), d.distance_km, d.start_address, d.end_address,
		       EXISTS(SELECT 1 FROM trip_group_drives tgd WHERE tgd.drive_id = d.id)
		FROM drives d
		WHERE d.vehicle_id = $1 AND d.deleted_upstream_at IS NULL AND d.trip_reviewed_at IS NULL AND d.start_time >= $2
		ORDER BY d.start_time;
	`, vehicleID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.TripCandidateDrive
	for rows.Next() {
		var d models.TripCandidateDrive
		if err := rows.Scan(&d.ID, &d.Start, &d.End, &d.DistanceKm, &d.StartAddress, &d.EndAddress, &d.Grouped); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

// ListChargeWindows returns the charging sessions of the vehicle since a date.
func (r *Repository) ListChargeWindows(ctx context.Context, vehicleID string, since time.Time) ([]models.ChargeWindow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT date, COALESCE(end_date, date)
		FROM charge_logs
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL AND COALESCE(end_date, date) >= $2
		ORDER BY date;
	`, vehicleID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.ChargeWindow
	for rows.Next() {
		var c models.ChargeWindow
		if err := rows.Scan(&c.Start, &c.End); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// DismissTripSuggestion rules that these drives are not part of a trip, so they are no longer suggested.
func (r *Repository) DismissTripSuggestion(ctx context.Context, vehicleID string, driveIDs []string) error {
	ids := uniqueStrings(driveIDs)
	if err := ensureDrivesOwned(ctx, r.pool, vehicleID, ids); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE drives SET trip_reviewed_at = COALESCE(trip_reviewed_at, NOW()), updated_at = NOW()
		WHERE vehicle_id = $1 AND id::text = ANY($2::text[]);
	`, vehicleID, ids)
	return err
}
