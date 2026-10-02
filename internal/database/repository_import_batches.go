package database

import (
	"context"
	"fmt"

	"github.com/teslacost/teslacost/internal/models"
)

// batchTables are the tables whose rows an import batch can own.
var batchTables = []string{"charge_logs", "drives", "fuel_logs", "odometer_checkpoints"}

// CreateImportBatch records a CSV import and returns its id.
func (r *Repository) CreateImportBatch(ctx context.Context, vehicleID, userID, importType string, rowCount int) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO import_batches (vehicle_id, user_id, import_type, row_count)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4) RETURNING id::text`,
		vehicleID, userID, importType, rowCount).Scan(&id)
	return id, err
}

// ListImportBatches returns the vehicle's imports, newest first, with the number of their rows still in place.
func (r *Repository) ListImportBatches(ctx context.Context, vehicleID string) ([]models.ImportBatch, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT b.id::text, b.vehicle_id::text, b.import_type, b.row_count, b.created_at,
		       (SELECT COUNT(*) FROM charge_logs WHERE source_batch_id = b.id)
		     + (SELECT COUNT(*) FROM drives WHERE source_batch_id = b.id)
		     + (SELECT COUNT(*) FROM fuel_logs WHERE source_batch_id = b.id)
		     + (SELECT COUNT(*) FROM odometer_checkpoints WHERE source_batch_id = b.id)
		FROM import_batches b WHERE b.vehicle_id = $1
		ORDER BY b.created_at DESC LIMIT 100`, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("failed to list import batches: %w", err)
	}
	defer rows.Close()
	batches := []models.ImportBatch{}
	for rows.Next() {
		var b models.ImportBatch
		if err := rows.Scan(&b.ID, &b.VehicleID, &b.ImportType, &b.RowCount, &b.CreatedAt, &b.Remaining); err != nil {
			return nil, fmt.Errorf("failed to read an import batch: %w", err)
		}
		batches = append(batches, b)
	}
	return batches, rows.Err()
}

// UndoImportBatch deletes the rows an import created that are still in place, then the batch itself.
// It returns the number of rows removed, or ErrNotFound when the vehicle has no such batch.
func (r *Repository) UndoImportBatch(ctx context.Context, vehicleID, batchID string) (int, error) {
	removed := 0
	err := r.WithTx(ctx, func(tx *Repository) error {
		var exists bool
		if err := tx.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM import_batches WHERE id::text = $1 AND vehicle_id = $2)`, batchID, vehicleID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		for _, table := range batchTables {
			tag, err := tx.pool.Exec(ctx, `DELETE FROM `+table+` WHERE source_batch_id::text = $1 AND vehicle_id = $2`, batchID, vehicleID)
			if err != nil {
				return err
			}
			removed += int(tag.RowsAffected())
		}
		_, err := tx.pool.Exec(ctx, `DELETE FROM import_batches WHERE id::text = $1 AND vehicle_id = $2`, batchID, vehicleID)
		return err
	})
	if err != nil {
		return 0, err
	}
	return removed, nil
}
