package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/money"
)

var (
	ErrNotFound = errors.New("record not found")
)

// Repository encapsulates database operations.
type Repository struct {
	pool querier
	root *pgxpool.Pool
}

// querier is what a pool and a transaction have in common: a Repository bound to a transaction runs every
// method of the pool-bound one inside it (a nested Begin becomes a savepoint).
type querier interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// NewRepository creates a new Repository instance.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, root: pool}
}

// Pool returns the underlying pgxpool.Pool.
func (r *Repository) Pool() *pgxpool.Pool {
	return r.root
}

// WithTx runs fn with a Repository bound to one transaction: it commits when fn returns nil and rolls back
// on any error, so a batch of writes is applied entirely or not at all.
func (r *Repository) WithTx(ctx context.Context, fn func(tx *Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(&Repository{pool: tx, root: r.root}); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

// HasDuplicateFuelLog checks if a fill-up of the same amount already exists within 30 minutes.
func (r *Repository) HasDuplicateFuelLog(ctx context.Context, vehicleID string, t time.Time, amount money.Cents) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM fuel_logs
		WHERE vehicle_id = $1
		  AND ABS(EXTRACT(EPOCH FROM (date - $2))) <= 1800
		  AND amount = $3;
	`, vehicleID, t, amount).Scan(&count)
	return count > 0, err
}

// HasDuplicateOdometerCheckpoint checks if a checkpoint with the same reading already exists within a day.
func (r *Repository) HasDuplicateOdometerCheckpoint(ctx context.Context, vehicleID string, t time.Time, odometer float64) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM odometer_checkpoints
		WHERE vehicle_id = $1
		  AND ABS(EXTRACT(EPOCH FROM (date::timestamptz - $2))) <= 86400
		  AND ABS(odometer - $3) < 0.5;
	`, vehicleID, t, odometer).Scan(&count)
	return count > 0, err
}
