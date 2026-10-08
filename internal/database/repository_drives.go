package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
)

// TeslaMate drive ingestion, trip groups and per-drive expenses (tolls, parking...).

func (r *Repository) UpsertTeslaMateDrive(ctx context.Context, d *models.Drive) (bool, error) {
	query := `
		INSERT INTO drives (
			vehicle_id, teslamate_drive_id, start_time, end_time,
			start_odometer, end_odometer, distance_km, duration_min,
			speed_avg, speed_max, power_max, power_min, start_address, end_address, energy_consumed_kwh,
			consumption_kwh_100km, tags, is_manual,
			start_battery_level, end_battery_level, outside_temp_c,
			driver_id, origin, external_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, FALSE, $18, $19, $20, (SELECT default_driver_id FROM vehicles WHERE id = $1), 'TESLAMATE', ($2::int)::text)
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
		       consumption_kwh_100km, tags, is_manual, energy_estimated, toll_reviewed_at, created_at, updated_at,
		       driver_id, (SELECT name FROM vehicle_people WHERE id = drives.driver_id) AS driver_name
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
		var driverName *string
		if err := rows.Scan(
			&d.ID, &d.VehicleID, &d.TeslaMateDriveID, &d.StartTime, &d.EndTime,
			&d.StartOdometer, &d.EndOdometer, &d.DistanceKm, &d.DurationMin,
			&d.SpeedAvg, &d.SpeedMax, &d.PowerMax, &d.PowerMin,
			&d.StartAddress, &d.EndAddress, &d.EnergyConsumedKwh,
			&d.ConsumptionKwh100km, &d.Tags, &d.IsManual, &d.EnergyEstimated, &d.TollReviewedAt, &d.CreatedAt, &d.UpdatedAt,
			&d.DriverID, &driverName,
		); err != nil {
			return nil, 0, err
		}
		d.DriverName = driverName
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

func (r *Repository) CreateManualDrive(ctx context.Context, d *models.Drive) error {
	if d.Tags == nil {
		d.Tags = []string{}
	}
	query := `
		INSERT INTO drives (
			vehicle_id, start_time, end_time,
			start_odometer, end_odometer, distance_km, duration_min,
			start_address, end_address, energy_consumed_kwh,
			consumption_kwh_100km, tags, is_manual, driver_id, energy_estimated, origin, external_id, source_batch_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, TRUE, COALESCE($13, (SELECT default_driver_id FROM vehicles WHERE id = $1)), $14, COALESCE(NULLIF($15, ''), 'MANUAL'), $16, $17)
		RETURNING id, created_at, updated_at;
	`
	return r.pool.QueryRow(ctx, query,
		d.VehicleID, d.StartTime, d.EndTime,
		d.StartOdometer, d.EndOdometer, d.DistanceKm, d.DurationMin,
		d.StartAddress, d.EndAddress, d.EnergyConsumedKwh,
		d.ConsumptionKwh100km, d.Tags, d.DriverID, d.EnergyEstimated, d.Origin, d.ExternalID, d.SourceBatchID,
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
}

// ReplaceDriveAddress sets one end of a drive to a resolved address, only while it still holds the placeholder
// it was created with, so an address the user edited in the meantime is kept.
func (r *Repository) ReplaceDriveAddress(ctx context.Context, driveID string, end DriveEnd, placeholder, address string) error {
	column := "start_address"
	if end == DriveEndArrival {
		column = "end_address"
	}
	_, err := r.pool.Exec(ctx, "UPDATE drives SET "+column+" = $3, updated_at = NOW() WHERE id = $1 AND "+column+" = $2;", driveID, placeholder, address)
	return err
}

// DriveEnd selects the departure or the arrival of a drive.
type DriveEnd int

const (
	DriveEndDeparture DriveEnd = iota
	DriveEndArrival
)

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
		    energy_consumed_kwh = $9, consumption_kwh_100km = $10,
		    tags = $11, driver_id = $12, energy_estimated = $13, updated_at = NOW()
		WHERE id = $14 AND vehicle_id = $15 AND is_manual = TRUE;
	`
	tag, err := r.pool.Exec(ctx, query,
		d.StartTime, d.EndTime,
		d.StartOdometer, d.EndOdometer,
		d.DistanceKm, d.DurationMin,
		d.StartAddress, d.EndAddress,
		d.EnergyConsumedKwh, d.ConsumptionKwh100km,
		d.Tags, d.DriverID, d.EnergyEstimated, d.ID, d.VehicleID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetDriveDriver assigns or clears the driver of any drive (manual or synced).
func (r *Repository) SetDriveDriver(ctx context.Context, driveID, vehicleID string, driverID *string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE drives
		SET driver_id = $1, updated_at = NOW()
		WHERE id::text = $2 AND vehicle_id = $3;
	`, driverID, driveID, vehicleID)
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

// CoordinateAddressPattern matches an address that is only a position ("46.05858, 6.57810"), as stored for a
// drive sent without an address.
const CoordinateAddressPattern = `^-?[0-9]{1,3}\.[0-9]+, -?[0-9]{1,3}\.[0-9]+$`

// CoordinateDrive is a drive with at least one end whose address is only coordinates.
type CoordinateDrive struct {
	ID           string
	StartAddress *string
	EndAddress   *string
}

// CountCoordinateAddressDrives counts the drives of a vehicle with a departure or an arrival stored as coordinates.
func (r *Repository) CountCoordinateAddressDrives(ctx context.Context, vehicleID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM drives
		WHERE vehicle_id = $1 AND (start_address ~ $2 OR end_address ~ $2);`, vehicleID, CoordinateAddressPattern).Scan(&n)
	return n, err
}

// ListCoordinateAddressDrives returns the most recent drives of a vehicle with an end stored as coordinates.
func (r *Repository) ListCoordinateAddressDrives(ctx context.Context, vehicleID string, limit int) ([]CoordinateDrive, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, start_address, end_address FROM drives
		WHERE vehicle_id = $1 AND (start_address ~ $2 OR end_address ~ $2)
		ORDER BY start_time DESC
		LIMIT $3;`, vehicleID, CoordinateAddressPattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CoordinateDrive
	for rows.Next() {
		var d CoordinateDrive
		if err := rows.Scan(&d.ID, &d.StartAddress, &d.EndAddress); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
