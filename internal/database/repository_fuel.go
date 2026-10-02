package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
)

// Fuel fill-ups of combustion vehicles (manual entries).

const fuelColumns = `id, vehicle_id, date, odometer, amount, liters, price_per_liter, fuel_type, is_full_tank, notes, created_at, updated_at`

func scanFuelLog(row pgx.Row) (*models.FuelLog, error) {
	var f models.FuelLog
	if err := row.Scan(&f.ID, &f.VehicleID, &f.Date, &f.Odometer, &f.Amount, &f.Liters, &f.PricePerLiter,
		&f.FuelType, &f.IsFullTank, &f.Notes, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return nil, err
	}
	return &f, nil
}

// ListFuelLogs lists the fill-ups of a vehicle, oldest first (date, then odometer).
func (r *Repository) ListFuelLogs(ctx context.Context, vehicleID string) ([]models.FuelLog, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+fuelColumns+` FROM fuel_logs WHERE vehicle_id = $1 ORDER BY date ASC, odometer ASC;`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []models.FuelLog{}
	for rows.Next() {
		f, err := scanFuelLog(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *f)
	}
	return list, rows.Err()
}

// syncOdometerFromManualPoints raises the current odometer of a combustion vehicle to the highest
// manual reading or fill-up mileage. It never lowers it: the value may come from another source
// (odometer edit). Electric vehicles are left to the TeslaMate synchronization.
func syncOdometerFromManualPoints(ctx context.Context, tx pgx.Tx, vehicleID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE vehicles
		SET current_odometer = GREATEST(
		        current_odometer,
		        COALESCE((SELECT MAX(odometer) FROM fuel_logs WHERE vehicle_id = $1), 0),
		        COALESCE((SELECT MAX(odometer) FROM odometer_checkpoints WHERE vehicle_id = $1), 0)),
		    updated_at = NOW()
		WHERE id = $1 AND powertrain = 'ICE';
	`, vehicleID)
	return err
}

// ListManualOdometerPoints lists the odometer readings and the fill-ups that carry a mileage, oldest first.
func (r *Repository) ListManualOdometerPoints(ctx context.Context, vehicleID string) ([]models.OdometerPoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, 'READING', date::timestamptz, odometer FROM odometer_checkpoints WHERE vehicle_id = $1
		UNION ALL
		SELECT id::text, 'FUEL', date, odometer FROM fuel_logs WHERE vehicle_id = $1 AND odometer IS NOT NULL
		ORDER BY 3 ASC, 4 ASC;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []models.OdometerPoint{}
	for rows.Next() {
		var p models.OdometerPoint
		if err := rows.Scan(&p.ID, &p.Kind, &p.Date, &p.Odometer); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// CreateFuelLog stores a fill-up and keeps the vehicle's current odometer up to date.
func (r *Repository) CreateFuelLog(ctx context.Context, f *models.FuelLog) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx, `
		INSERT INTO fuel_logs (vehicle_id, date, odometer, amount, liters, price_per_liter, fuel_type, is_full_tank, notes, source_batch_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at;
	`, f.VehicleID, f.Date, f.Odometer, f.Amount, f.Liters, f.PricePerLiter, f.FuelType, f.IsFullTank, f.Notes, f.SourceBatchID,
	).Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return err
	}
	if err := syncOdometerFromManualPoints(ctx, tx, f.VehicleID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UpdateFuelLog replaces the editable fields of a fill-up.
func (r *Repository) UpdateFuelLog(ctx context.Context, f *models.FuelLog) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		UPDATE fuel_logs
		SET date = $3, odometer = $4, amount = $5, liters = $6, price_per_liter = $7,
		    fuel_type = $8, is_full_tank = $9, notes = $10, updated_at = NOW()
		WHERE id::text = $1 AND vehicle_id = $2
		RETURNING created_at, updated_at;
	`, f.ID, f.VehicleID, f.Date, f.Odometer, f.Amount, f.Liters, f.PricePerLiter, f.FuelType, f.IsFullTank, f.Notes,
	).Scan(&f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := syncOdometerFromManualPoints(ctx, tx, f.VehicleID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteFuelLog deletes a fill-up. The vehicle's current odometer is left untouched.
func (r *Repository) DeleteFuelLog(ctx context.Context, vehicleID, fuelLogID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM fuel_logs WHERE id::text = $1 AND vehicle_id = $2;`, fuelLogID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
