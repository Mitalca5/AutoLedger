package database

import (
	"context"

	"github.com/teslacost/teslacost/internal/models"
)

// ListBatterySnapshots returns the battery readings of a vehicle, oldest first.
func (r *Repository) ListBatterySnapshots(ctx context.Context, vehicleID string) ([]models.BatterySnapshot, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT captured_on::text, max_capacity_kwh::float8, current_capacity_kwh::float8, health_percent::float8
		FROM battery_health_snapshots
		WHERE vehicle_id = $1
		ORDER BY captured_on;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.BatterySnapshot{}
	for rows.Next() {
		var s models.BatterySnapshot
		if err := rows.Scan(&s.Date, &s.MaxCapacityKwh, &s.CurrentCapacityKwh, &s.HealthPercent); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeleteBatterySnapshot removes the reading of a day; ErrNotFound when there is none.
func (r *Repository) DeleteBatterySnapshot(ctx context.Context, vehicleID, day string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM battery_health_snapshots WHERE vehicle_id = $1 AND captured_on = $2::date;`, vehicleID, day)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
