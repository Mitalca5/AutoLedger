package database

import (
	"context"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

// Manual odometer checkpoints (used to bridge history before tracking started).
// ListOdometerCheckpoints lists all manual odometer checkpoints for a vehicle, ordered by date ASC, odometer ASC.
func (r *Repository) ListOdometerCheckpoints(ctx context.Context, vehicleID string) ([]models.OdometerCheckpoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, vehicle_id, date, odometer, notes, source, created_at, updated_at
		FROM odometer_checkpoints
		WHERE vehicle_id = $1
		ORDER BY date ASC, odometer ASC;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []models.OdometerCheckpoint{}
	for rows.Next() {
		var c models.OdometerCheckpoint
		if err := rows.Scan(&c.ID, &c.VehicleID, &c.Date, &c.Odometer, &c.Notes, &c.Source, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// CreateOdometerCheckpoint records a new odometer checkpoint.
func (r *Repository) CreateOdometerCheckpoint(ctx context.Context, c *models.OdometerCheckpoint) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx, `
		INSERT INTO odometer_checkpoints (vehicle_id, date, odometer, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id, source, created_at, updated_at;
	`, c.VehicleID, c.Date, c.Odometer, c.Notes).Scan(&c.ID, &c.Source, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return err
	}
	if err := syncOdometerFromManualPoints(ctx, tx, c.VehicleID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UpdateOdometerCheckpoint updates an existing odometer checkpoint.
func (r *Repository) UpdateOdometerCheckpoint(ctx context.Context, c *models.OdometerCheckpoint) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE odometer_checkpoints
		SET date = $3, odometer = $4, notes = $5, updated_at = NOW()
		WHERE id = $1 AND vehicle_id = $2 AND source = 'MANUAL';
	`, c.ID, c.VehicleID, c.Date, c.Odometer, c.Notes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := syncOdometerFromManualPoints(ctx, tx, c.VehicleID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RecordIntegrationOdometer keeps the mileage an integration reported on the given day as that day's single point,
// raising it when the day's reading goes up. A manual reading with the same date and mileage already says it.
func (r *Repository) RecordIntegrationOdometer(ctx context.Context, vehicleID string, day time.Time, odometer float64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO odometer_checkpoints (vehicle_id, date, odometer, source)
		SELECT $1, $2::date, $3, 'HA'
		WHERE NOT EXISTS (
			SELECT 1 FROM odometer_checkpoints
			WHERE vehicle_id = $1 AND date = $2::date AND odometer = $3 AND source <> 'HA')
		ON CONFLICT (vehicle_id, date) WHERE source = 'HA'
		DO UPDATE SET odometer = GREATEST(odometer_checkpoints.odometer, EXCLUDED.odometer), updated_at = NOW();
	`, vehicleID, day, odometer)
	return err
}

// DeleteOdometerCheckpoint deletes an odometer checkpoint.
func (r *Repository) DeleteOdometerCheckpoint(ctx context.Context, vehicleID, checkpointID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM odometer_checkpoints
		WHERE id = $1 AND vehicle_id = $2;
	`, checkpointID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// Expense Documents & Invoices
// ============================================================================
