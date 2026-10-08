package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
)

// Tire lifecycle: mounts, sessions, rotations, disposal and wear logs.

func (r *Repository) CreateTire(ctx context.Context, t *models.Tire) error {
	return r.CreateTiresBatch(ctx, []*models.Tire{t})
}

func (r *Repository) ListTires(ctx context.Context, vehicleID string) ([]models.Tire, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+tireColumns+`
		FROM tires
		WHERE vehicle_id = $1 AND is_archived = FALSE
		ORDER BY current_position ASC, purchase_date DESC;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Tire
	for rows.Next() {
		var t models.Tire
		if err := scanTire(rows, &t); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *Repository) GetTireByID(ctx context.Context, id, vehicleID string) (*models.Tire, error) {
	var t models.Tire
	err := scanTire(r.pool.QueryRow(ctx, `
		SELECT `+tireColumns+`
		FROM tires
		WHERE id::text = $1 AND vehicle_id = $2;
	`, id, vehicleID), &t)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

// UpdateTire updates descriptive tire fields and recomputes its lifetime distance.
// Position changes go through mount sessions / rotations, never through this method.
func (r *Repository) UpdateTire(ctx context.Context, t *models.Tire) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := updateTireTx(ctx, tx, t); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	updated, err := r.GetTireByID(ctx, t.ID, *t.VehicleID)
	if err != nil {
		return err
	}
	*t = *updated
	return nil
}

func updateTireTx(ctx context.Context, tx pgx.Tx, t *models.Tire) error {
	lifespan := t.EstimatedLifespanKm
	if lifespan <= 0 {
		lifespan = 40000
	}
	tag, err := tx.Exec(ctx, `
		UPDATE tires
		SET brand = $1, model = $2, dimension = $3, season = $4,
		    purchase_date = $5, purchase_price = $6,
		    initial_depth_mm = $7, min_legal_depth_mm = $8, dot_code = $9,
		    initial_distance_km = $10, estimated_lifespan_km = $11,
		    updated_at = NOW()
		WHERE id::text = $12 AND vehicle_id = $13;
	`,
		t.Brand, t.Model, t.Dimension, t.Season,
		t.PurchaseDate, t.PurchasePrice,
		t.InitialDepthMm, t.MinLegalDepthMm, t.DotCode,
		t.InitialDistanceKm, lifespan,
		t.ID, t.VehicleID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return recalcTireDistance(ctx, tx, t.ID)
}

// DeleteTire permanently deletes a tire with its sessions and wear logs (erroneous entry).
func (r *Repository) DeleteTire(ctx context.Context, vehicleID, tireID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tires WHERE id::text = $1 AND vehicle_id = $2;`, tireID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DisposeTire retires a tire (worn out, damaged, sold): closes its mount session and moves it to DISPOSED.
// Its purchase price is then fully counted as consumed in the amortized cost.
func (r *Repository) DisposeTire(ctx context.Context, vehicleID, tireID string, at time.Time, odometer *float64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	current, err := lockVehicleTires(ctx, tx, vehicleID)
	if err != nil {
		return err
	}
	t, ok := current[tireID]
	if !ok {
		return ErrNotFound
	}
	if isMountedPosition(t.CurrentPosition) {
		if odometer == nil {
			return apierror.New("tire.dismount_odometer_required", "The removal odometer is required for a fitted tire")
		}
		if t.MountedOdometer != nil && *odometer < *t.MountedOdometer {
			return apierror.Newf("tire.odometer_below_mount", "The odometer (%.0f km) is lower than the fitting odometer (%.0f km)", apierror.Km(*odometer), apierror.Km(*t.MountedOdometer))
		}
		if _, err := tx.Exec(ctx, `
			UPDATE tire_mount_sessions
			SET dismounted_date = $1, dismounted_odometer = $2,
			    distance_km = GREATEST($2 - mounted_odometer, 0), updated_at = NOW()
			WHERE tire_id = $3 AND dismounted_date IS NULL;
		`, at, *odometer, tireID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE tires SET current_position = 'DISPOSED', updated_at = NOW() WHERE id = $1;`, tireID); err != nil {
		return err
	}
	if err := recalcTireDistance(ctx, tx, tireID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

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
