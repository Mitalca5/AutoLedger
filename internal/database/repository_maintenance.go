package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
)

// Maintenance expenses (services, repairs...) and odometer-at-date lookups.

func (r *Repository) CreateMaintenanceExpense(ctx context.Context, m *models.MaintenanceExpense) error {
	mode := m.AmortizationMode
	if mode == "" {
		mode = "NONE"
	}
	m.AmortizationMode = mode

	query := `
		INSERT INTO maintenance_expenses (
			vehicle_id, category, amount, currency, fx_rate, date,
			odometer, is_recurring, recurrence_interval_months, recurrence_end_date, description,
			amortization_mode, coverage_km, coverage_months, closes_maintenance_id, document_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, created_at, updated_at;
	`
	err := r.pool.QueryRow(ctx, query,
		m.VehicleID, m.Category, m.Amount, m.Currency, m.FxRate, m.Date,
		m.Odometer, m.IsRecurring, m.RecurrenceIntervalMonths, m.RecurrenceEndDate, m.Description,
		m.AmortizationMode, m.CoverageKm, m.CoverageMonths, m.ClosesMaintenanceID, m.DocumentID,
	).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return err
	}

	if m.DocumentID != nil {
		_ = r.pool.QueryRow(ctx, `SELECT filename FROM expense_documents WHERE id = $1;`, *m.DocumentID).Scan(&m.DocumentFilename)
	}

	return nil
}

func (r *Repository) ListMaintenanceExpenses(ctx context.Context, vehicleID string) ([]models.MaintenanceExpense, error) {
	query := `
		SELECT m.id, m.vehicle_id, m.category, m.amount, m.currency, m.fx_rate, m.date,
		       m.odometer, m.is_recurring, m.recurrence_interval_months, m.recurrence_end_date, m.description,
		       m.amortization_mode, m.coverage_km, m.coverage_months, m.closes_maintenance_id,
		       m.document_id, doc.filename,
		       m.created_at, m.updated_at
		FROM maintenance_expenses m
		LEFT JOIN expense_documents doc ON m.document_id = doc.id
		WHERE m.vehicle_id = $1
		ORDER BY m.date DESC;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.MaintenanceExpense
	for rows.Next() {
		var m models.MaintenanceExpense
		if err := rows.Scan(
			&m.ID, &m.VehicleID, &m.Category, &m.Amount, &m.Currency, &m.FxRate, &m.Date,
			&m.Odometer, &m.IsRecurring, &m.RecurrenceIntervalMonths, &m.RecurrenceEndDate, &m.Description,
			&m.AmortizationMode, &m.CoverageKm, &m.CoverageMonths, &m.ClosesMaintenanceID,
			&m.DocumentID, &m.DocumentFilename,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *Repository) UpdateMaintenanceExpense(ctx context.Context, m *models.MaintenanceExpense) error {
	mode := m.AmortizationMode
	if mode == "" {
		mode = "NONE"
	}
	m.AmortizationMode = mode

	query := `
		UPDATE maintenance_expenses
		SET category = $1,
		    amount = $2,
		    currency = $3,
		    fx_rate = $4,
		    date = $5,
		    odometer = $6,
		    is_recurring = $7,
		    recurrence_interval_months = $8,
		    recurrence_end_date = $9,
		    description = $10,
		    amortization_mode = $11,
		    coverage_km = $12,
		    coverage_months = $13,
		    closes_maintenance_id = $14,
		    document_id = $15,
		    updated_at = NOW()
		WHERE id::text = $16 AND vehicle_id = $17
		RETURNING created_at, updated_at;
	`
	err := r.pool.QueryRow(ctx, query,
		m.Category, m.Amount, m.Currency, m.FxRate, m.Date,
		m.Odometer, m.IsRecurring, m.RecurrenceIntervalMonths, m.RecurrenceEndDate, m.Description,
		m.AmortizationMode, m.CoverageKm, m.CoverageMonths, m.ClosesMaintenanceID, m.DocumentID,
		m.ID, m.VehicleID,
	).Scan(&m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	// Reminders that follow from this record keep its date, and its odometer when it has one.
	if _, err := r.pool.Exec(ctx, `
		UPDATE maintenance_reminders
		SET last_service_date = $1, last_service_odometer = COALESCE($2, last_service_odometer), updated_at = NOW()
		WHERE maintenance_id::text = $3;`, m.Date, m.Odometer, m.ID); err != nil {
		return err
	}

	if m.DocumentID != nil {
		_ = r.pool.QueryRow(ctx, `SELECT filename FROM expense_documents WHERE id = $1;`, *m.DocumentID).Scan(&m.DocumentFilename)
	}

	return nil
}

func (r *Repository) DeleteMaintenanceExpense(ctx context.Context, vehicleID, maintenanceID string) error {
	query := `DELETE FROM maintenance_expenses WHERE id::text = $1 AND vehicle_id = $2;`
	cmdTag, err := r.pool.Exec(ctx, query, maintenanceID, vehicleID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// Charges
// ============================================================================

// OdometerAnchor is a dated odometer reading of a vehicle.
type OdometerAnchor struct {
	Date   time.Time
	Km     float64
	Origin bool
}

// ListOdometerAnchors lists every dated odometer reading of a vehicle: drive starts and ends, checkpoints,
// fill-ups, the start of the ownership (an ownership starting at 0 km marks the origin of the vehicle) and the current odometer (dated now, as the monthly mileage smoothing does).
func (r *Repository) ListOdometerAnchors(ctx context.Context, vehicleID string) ([]OdometerAnchor, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT start_time, start_odometer, false FROM drives
		  WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL AND start_odometer > 0
		UNION ALL
		SELECT end_time, end_odometer, false FROM drives
		  WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL AND end_odometer > 0
		UNION ALL
		SELECT date::timestamptz, odometer, false FROM odometer_checkpoints WHERE vehicle_id = $1
		UNION ALL
		SELECT date, odometer, false FROM fuel_logs WHERE vehicle_id = $1 AND odometer IS NOT NULL
		UNION ALL
		SELECT start_date::timestamptz, start_odometer, start_odometer = 0 FROM vehicle_ownership
		  WHERE vehicle_id = $1 AND start_odometer IS NOT NULL
		UNION ALL
		SELECT now(), current_odometer, false FROM vehicles WHERE id = $1 AND current_odometer > 0;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OdometerAnchor
	for rows.Next() {
		var a OdometerAnchor
		if err := rows.Scan(&a.Date, &a.Km, &a.Origin); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
