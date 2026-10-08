package services

import (
	"context"

	"github.com/teslacost/teslacost/internal/money"
)

// financingDueLaterThisMonth sums the financing payments of the current month that are scheduled after today:
// the cost_ledger view only holds payments whose date has passed, so the month in progress would otherwise
// miss an instalment due on a later day. The conditions mirror the financing branches of that view (migration 34).
func (s *TCOService) financingDueLaterThisMonth(ctx context.Context, vehicleID string) (string, money.Cents, error) {
	var month string
	var amount money.Cents
	err := s.pool.QueryRow(ctx, `
		WITH bounds AS (
			SELECT CURRENT_DATE AS today,
			       (date_trunc('month', CURRENT_DATE) + interval '1 month')::date AS next_month
		)
		SELECT TO_CHAR(b.today, 'YYYY-MM'), COALESCE(SUM(due.amount), 0)
		FROM bounds b
		LEFT JOIN LATERAL (
			SELECT CASE WHEN o.acquisition_type = 'LOAN' THEN COALESCE(o.loan_fees, 0)
			            ELSE COALESCE(o.lease_down_payment, 0) + COALESCE(o.lease_fees, 0) END AS amount
			FROM vehicle_ownership o
			WHERE o.vehicle_id = $1 AND o.start_date > b.today AND o.start_date < b.next_month
			  AND ((o.acquisition_type = 'LOAN' AND o.loan_fees > 0)
			    OR (o.acquisition_type IN ('LOA', 'LLD') AND COALESCE(o.lease_down_payment, 0) + COALESCE(o.lease_fees, 0) > 0))
			UNION ALL
			SELECT ROUND(
			           CASE WHEN COALESCE(o.loan_rate_pct, 0) = 0 THEN 0
			                ELSE (o.loan_amount * power(1 + o.loan_rate_pct / 1200, k - 1)
			                      - (o.loan_amount * (o.loan_rate_pct / 1200) / (1 - power(1 + o.loan_rate_pct / 1200, -o.loan_duration_months)))
			                        * (power(1 + o.loan_rate_pct / 1200, k - 1) - 1) / (o.loan_rate_pct / 1200))
			                     * (o.loan_rate_pct / 1200)
			           END + COALESCE(o.loan_insurance_monthly, 0), 2)
			FROM vehicle_ownership o
			CROSS JOIN LATERAL generate_series(1, o.loan_duration_months) AS k
			WHERE o.vehicle_id = $1 AND o.acquisition_type = 'LOAN'
			  AND o.loan_amount > 0 AND o.loan_duration_months > 0
			  AND o.start_date + make_interval(months => k) > b.today
			  AND o.start_date + make_interval(months => k) < b.next_month
			  AND o.start_date + make_interval(months => k) <= COALESCE(o.end_date, 'infinity'::date)
			UNION ALL
			SELECT o.lease_monthly_rent
			FROM vehicle_ownership o
			CROSS JOIN LATERAL generate_series(0, o.lease_duration_months - 1) AS k
			WHERE o.vehicle_id = $1 AND o.acquisition_type IN ('LOA', 'LLD')
			  AND o.lease_monthly_rent > 0 AND o.lease_duration_months > 0
			  AND o.start_date + make_interval(months => k) > b.today
			  AND o.start_date + make_interval(months => k)
			      < LEAST(b.next_month, COALESCE(o.end_date, 'infinity'::date), COALESCE(o.option_exercised_date, 'infinity'::date))
		) due ON true
		GROUP BY b.today
	`, vehicleID).Scan(&month, &amount)
	return month, amount, err
}
