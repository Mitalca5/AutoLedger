package services

import (
	"context"
	"fmt"
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

// annualInsurance projects current premiums by their billing or coverage period, never by mileage.
// Multiple current premiums are additive. An obsolete recurring policy must have its end date set.
// Payments without a known period use a trailing 12-month cash total, explicitly marked as an estimate.
func (s *ComparisonService) annualInsurance(ctx context.Context, vehicleID string, at time.Time) (money.Cents, bool, error) {
	var annual money.Cents
	var estimated bool
	err := s.tco.pool.QueryRow(ctx, `
		WITH premiums AS (
			SELECT CASE WHEN m.currency = v.currency THEN m.amount ELSE m.amount * m.fx_rate END AS amount,
			       CASE
			         WHEN m.is_recurring AND COALESCE(m.recurrence_interval_months, 0) > 0 THEN m.recurrence_interval_months
			         WHEN COALESCE(m.coverage_months, 0) > 0 THEN m.coverage_months
			         ELSE NULL
			       END AS months
			FROM maintenance_expenses m JOIN vehicles v ON v.id = m.vehicle_id
			WHERE m.vehicle_id = $1 AND m.category = 'INSURANCE'
			  AND (m.currency = v.currency OR m.fx_rate IS NOT NULL)
			  AND m.date <= $2::date
			  AND (m.recurrence_end_date IS NULL OR m.recurrence_end_date >= $2::date)
			  AND (
			    (m.is_recurring AND COALESCE(m.recurrence_interval_months, 0) > 0)
			    OR (COALESCE(m.coverage_months, 0) > 0 AND m.date + make_interval(months => m.coverage_months) > $2::date)
			    OR (NOT (m.is_recurring AND COALESCE(m.recurrence_interval_months, 0) > 0)
			        AND COALESCE(m.coverage_months, 0) <= 0 AND m.date > $2::date - INTERVAL '12 months')
			  )
		)
		SELECT COALESCE(SUM(ROUND(CASE WHEN months IS NOT NULL THEN amount * 12.0 / months ELSE amount END, 2)), 0),
		       COALESCE(BOOL_OR(months IS NULL), false)
		FROM premiums;
	`, vehicleID, at).Scan(&annual, &estimated)
	if err != nil {
		return 0, false, fmt.Errorf("comparison insurance: %w", err)
	}
	return annual, estimated, nil
}
