package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
)

// Charging sessions pushed by integrations (Home Assistant, scripts): storage and duplicate detection.

// Tolerance within which a session without event_id is taken for one already recorded.
const (
	ingestDuplicateWindow = 30 * time.Minute
	ingestDuplicateKwh    = 0.5
)

// rowQuerier is what inserting a charge needs: a pool or a transaction.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// CreateIngestedCharge records a charging session pushed by an integration. Unlike CreateManualCharge it keeps
// the given cost source, the battery levels and the event identifier.
func (r *Repository) CreateIngestedCharge(ctx context.Context, c *models.ChargeLog) error {
	return insertIngestedCharge(ctx, r.pool, c)
}

func insertIngestedCharge(ctx context.Context, q rowQuerier, c *models.ChargeLog) error {
	c.IsManual = true
	return q.QueryRow(ctx, `
		INSERT INTO charge_logs (
			vehicle_id, date, end_date, address, kwh_added, cost, cost_source, currency, odometer,
			start_battery_level, end_battery_level, is_manual, notes, external_id, origin
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, TRUE, $12, $13, 'WEBHOOK')
		RETURNING id, created_at;
	`,
		c.VehicleID, c.Date, c.EndDate, c.Address, c.KwhAdded, c.Cost, c.CostSource, c.Currency, c.Odometer,
		c.StartBatteryLevel, c.EndBatteryLevel, c.Notes, c.ExternalID,
	).Scan(&c.ID, &c.CreatedAt)
}

// AssignPendingCharge turns pending charge pendingID of userID into charge c in one transaction: the pending row is
// locked, the charge inserted and the pending row deleted together, so a second submission finds nothing to assign
// (ErrNotFound) instead of recording the charge twice.
func (r *Repository) AssignPendingCharge(ctx context.Context, pendingID, userID string, c *models.ChargeLog) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var locked string
	if err := tx.QueryRow(ctx, `SELECT id FROM pending_charges WHERE id::text = $1 AND user_id = $2 FOR UPDATE;`, pendingID, userID).Scan(&locked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if err := insertIngestedCharge(ctx, tx, c); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pending_charges WHERE id = $1;`, locked); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FindIngestedCharge returns the ID of a charge of one of vehicleIDs matching the session: the same event_id when
// one is given, otherwise a start within ingestDuplicateWindow and an energy within ingestDuplicateKwh.
func (r *Repository) FindIngestedCharge(ctx context.Context, vehicleIDs []string, externalID *string, start time.Time, kwh float64) (string, bool, error) {
	if len(vehicleIDs) == 0 {
		return "", false, nil
	}
	var id string
	var err error
	if externalID != nil {
		err = r.pool.QueryRow(ctx, `
			SELECT id FROM charge_logs WHERE vehicle_id = ANY($1) AND external_id = $2 LIMIT 1;
		`, vehicleIDs, *externalID).Scan(&id)
	} else {
		err = r.pool.QueryRow(ctx, `
			SELECT id FROM charge_logs
			WHERE vehicle_id = ANY($1) AND deleted_upstream_at IS NULL
			  AND ABS(EXTRACT(EPOCH FROM (date - $2))) <= $3 AND ABS(kwh_added - $4) <= $5
			LIMIT 1;
		`, vehicleIDs, start, ingestDuplicateWindow.Seconds(), kwh, ingestDuplicateKwh).Scan(&id)
	}
	return foundID(id, err)
}

// FindPendingCharge returns the ID of a pending charge of userID matching the session, with the same rule as
// FindIngestedCharge.
func (r *Repository) FindPendingCharge(ctx context.Context, userID string, externalID *string, start time.Time, kwh float64) (string, bool, error) {
	var id string
	var err error
	if externalID != nil {
		err = r.pool.QueryRow(ctx, `
			SELECT id FROM pending_charges WHERE user_id = $1 AND external_id = $2 LIMIT 1;
		`, userID, *externalID).Scan(&id)
	} else {
		err = r.pool.QueryRow(ctx, `
			SELECT id FROM pending_charges
			WHERE user_id = $1 AND ABS(EXTRACT(EPOCH FROM (start_time - $2))) <= $3 AND ABS(energy_kwh - $4) <= $5
			LIMIT 1;
		`, userID, start, ingestDuplicateWindow.Seconds(), kwh, ingestDuplicateKwh).Scan(&id)
	}
	return foundID(id, err)
}

func foundID(id string, err error) (string, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}
