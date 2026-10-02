package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
)

// Charging session ingestion and manual charge entries.
// UpsertTeslaMateCharge inserts or refreshes a TeslaMate charge. A cost entered manually is never overwritten.
func (r *Repository) UpsertTeslaMateCharge(ctx context.Context, c *models.ChargeLog) (bool, error) {
	query := `
		INSERT INTO charge_logs (
			vehicle_id, teslamate_charge_id, date, end_date,
			address, kwh_added, kwh_used, cost, cost_source, currency, odometer, is_manual,
			start_battery_level, end_battery_level, outside_temp_c, origin, external_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'TESLAMATE', $9, $10, FALSE, $11, $12, $13, 'TESLAMATE', ($2::int)::text)
		ON CONFLICT (vehicle_id, teslamate_charge_id) DO UPDATE
		SET date = EXCLUDED.date,
		    end_date = EXCLUDED.end_date,
		    address = EXCLUDED.address,
		    kwh_added = EXCLUDED.kwh_added,
		    kwh_used = EXCLUDED.kwh_used,
		    cost = CASE WHEN charge_logs.cost_source = 'MANUAL' THEN charge_logs.cost ELSE EXCLUDED.cost END,
		    currency = CASE WHEN charge_logs.cost_source = 'MANUAL' THEN charge_logs.currency ELSE EXCLUDED.currency END,
		    odometer = EXCLUDED.odometer,
		    start_battery_level = EXCLUDED.start_battery_level,
		    end_battery_level = EXCLUDED.end_battery_level,
		    outside_temp_c = EXCLUDED.outside_temp_c,
		    deleted_upstream_at = NULL
		RETURNING id, (xmax = 0) AS is_inserted;
	`
	var isInserted bool
	err := r.pool.QueryRow(ctx, query,
		c.VehicleID, c.TeslaMateChargeID, c.Date, c.EndDate,
		c.Address, c.KwhAdded, c.KwhUsed, c.Cost, c.Currency, c.Odometer,
		c.StartBatteryLevel, c.EndBatteryLevel, c.OutsideTempC,
	).Scan(&c.ID, &isInserted)
	return isInserted, err
}

func (r *Repository) GetLatestTeslaMateChargeDate(ctx context.Context, vehicleID string) (*time.Time, error) {
	query := `
		SELECT date
		FROM charge_logs
		WHERE vehicle_id = $1 AND teslamate_charge_id IS NOT NULL AND deleted_upstream_at IS NULL
		ORDER BY date DESC
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

const chargeSelectColumns = `
	c.id, c.vehicle_id, c.teslamate_charge_id, c.date, c.end_date,
	c.address, c.kwh_added, c.kwh_used, c.cost, c.cost_source, c.currency, c.fx_rate, c.odometer, c.is_manual, c.notes,
	c.document_id, doc.filename, c.created_at
`

func scanCharge(row pgx.Row, c *models.ChargeLog) error {
	return row.Scan(
		&c.ID, &c.VehicleID, &c.TeslaMateChargeID, &c.Date, &c.EndDate,
		&c.Address, &c.KwhAdded, &c.KwhUsed, &c.Cost, &c.CostSource, &c.Currency, &c.FxRate, &c.Odometer,
		&c.IsManual, &c.Notes,
		&c.DocumentID, &c.DocumentFilename,
		&c.CreatedAt,
	)
}

// ListCharges lists charges; missingCostOnly restricts to charges whose cost is still unknown.
func (r *Repository) ListCharges(ctx context.Context, vehicleID string, missingCostOnly bool, limit, offset int) ([]models.ChargeLog, int, error) {
	whereCount := `vehicle_id = $1 AND deleted_upstream_at IS NULL AND (NOT $2 OR cost IS NULL)`
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM charge_logs WHERE `+whereCount, vehicleID, missingCostOnly).Scan(&total); err != nil {
		return nil, 0, err
	}

	whereSelect := `c.vehicle_id = $1 AND c.deleted_upstream_at IS NULL AND (NOT $2 OR c.cost IS NULL)`
	rows, err := r.pool.Query(ctx, `
		SELECT `+chargeSelectColumns+`
		FROM charge_logs c
		LEFT JOIN expense_documents doc ON c.document_id = doc.id
		WHERE `+whereSelect+`
		ORDER BY c.date DESC
		LIMIT $3 OFFSET $4;
	`, vehicleID, missingCostOnly, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []models.ChargeLog
	for rows.Next() {
		var c models.ChargeLog
		if err := scanCharge(rows, &c); err != nil {
			return nil, 0, err
		}
		list = append(list, c)
	}
	return list, total, rows.Err()
}

// CountChargesWithoutCost counts charges with an unknown cost.
func (r *Repository) CountChargesWithoutCost(ctx context.Context, vehicleID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM charge_logs WHERE vehicle_id = $1 AND cost IS NULL AND deleted_upstream_at IS NULL;`, vehicleID).Scan(&count)
	return count, err
}

// CreateManualCharge records a charge that TeslaMate did not capture.
func (r *Repository) CreateManualCharge(ctx context.Context, c *models.ChargeLog) error {
	c.IsManual = true
	c.CostSource = "MANUAL"
	err := r.pool.QueryRow(ctx, `
		INSERT INTO charge_logs (
			vehicle_id, date, end_date, address, kwh_added, cost, cost_source,
			currency, fx_rate, odometer, is_manual, notes, document_id, origin, source_batch_id
		) VALUES ($1, $2, $3, $4, $5, $6, 'MANUAL', $7, $8, $9, TRUE, $10, $11, COALESCE(NULLIF($12, ''), 'MANUAL'), $13)
		RETURNING id, created_at;
	`,
		c.VehicleID, c.Date, c.EndDate, c.Address, c.KwhAdded, c.Cost,
		c.Currency, c.FxRate, c.Odometer, c.Notes, c.DocumentID, c.Origin, c.SourceBatchID,
	).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return err
	}

	if c.DocumentID != nil {
		_ = r.pool.QueryRow(ctx, `SELECT filename FROM expense_documents WHERE id = $1;`, *c.DocumentID).Scan(&c.DocumentFilename)
	}

	return nil
}

// UpdateCharge updates a charge. TeslaMate charges only accept cost corrections (protected from resyncs);
// manual charges are fully editable.
func (r *Repository) UpdateCharge(ctx context.Context, c *models.ChargeLog) error {
	var existing models.ChargeLog
	err := scanCharge(r.pool.QueryRow(ctx, `
		SELECT `+chargeSelectColumns+`
		FROM charge_logs c
		LEFT JOIN expense_documents doc ON c.document_id = doc.id
		WHERE c.id::text = $1 AND c.vehicle_id = $2;
	`, c.ID, c.VehicleID), &existing)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if !existing.IsManual {
		c.Date, c.EndDate, c.Address = existing.Date, existing.EndDate, existing.Address
		c.KwhAdded, c.Odometer = existing.KwhAdded, existing.Odometer
	}

	cmdTag, err := r.pool.Exec(ctx, `
		UPDATE charge_logs
		SET date = $1, end_date = $2, address = $3, kwh_added = $4, odometer = $5,
		    cost = $6, cost_source = 'MANUAL', currency = $7, fx_rate = $8, notes = $9,
		    document_id = $10
		WHERE id::text = $11 AND vehicle_id = $12;
	`,
		c.Date, c.EndDate, c.Address, c.KwhAdded, c.Odometer,
		c.Cost, c.Currency, c.FxRate, c.Notes, c.DocumentID,
		c.ID, c.VehicleID,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return scanCharge(r.pool.QueryRow(ctx, `
		SELECT `+chargeSelectColumns+`
		FROM charge_logs c
		LEFT JOIN expense_documents doc ON c.document_id = doc.id
		WHERE c.id::text = $1 AND c.vehicle_id = $2;
	`, c.ID, c.VehicleID), c)
}

// DeleteManualCharge deletes a manually entered charge (TeslaMate charges would come back on next sync).
func (r *Repository) DeleteManualCharge(ctx context.Context, vehicleID, chargeID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM charge_logs WHERE id::text = $1 AND vehicle_id = $2 AND is_manual = TRUE;`, chargeID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// Sync State
// ============================================================================
