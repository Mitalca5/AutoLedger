package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("record not found")
)

// Repository encapsulates database operations.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository instance.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Pool returns the underlying pgxpool.Pool.
func (r *Repository) Pool() *pgxpool.Pool {
	return r.pool
}

// HasDuplicateCharge checks if a similar charge already exists within 30 minutes and 0.5 kWh.
func (r *Repository) HasDuplicateCharge(ctx context.Context, vehicleID string, t time.Time, kwh float64) (bool, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM charge_logs
		WHERE vehicle_id = $1
		  AND ABS(EXTRACT(EPOCH FROM (date - $2))) <= 1800
		  AND ABS(kwh_added - $3) <= 0.5;
	`
	err := r.pool.QueryRow(ctx, query, vehicleID, t, kwh).Scan(&count)
	return count > 0, err
}

// HasDuplicateDrive checks if a similar drive already exists within 15 minutes and 1 km.
func (r *Repository) HasDuplicateDrive(ctx context.Context, vehicleID string, startTime time.Time, distanceKm float64) (bool, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM drives
		WHERE vehicle_id = $1
		  AND ABS(EXTRACT(EPOCH FROM (start_time - $2))) <= 900
		  AND ABS(distance_km - $3) <= 1.0;
	`
	err := r.pool.QueryRow(ctx, query, vehicleID, startTime, distanceKm).Scan(&count)
	return count > 0, err
}
