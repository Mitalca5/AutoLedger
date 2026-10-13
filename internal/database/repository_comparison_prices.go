package database

import (
	"context"

	"github.com/teslacost/teslacost/internal/models"
)

// GetComparisonPriceHistory reads only the reference vehicle, after the handler checks access.
// Each purchase is one observation; unknown costs and missing foreign-currency conversions are excluded.
func (r *Repository) GetComparisonPriceHistory(ctx context.Context, vehicleID, fuelType string) (*models.ComparisonPriceHistory, error) {
	var history models.ComparisonPriceHistory
	err := r.pool.QueryRow(ctx, `
		WITH prices AS (
			SELECT 'electricity' AS kind,
				(CASE WHEN c.currency = v.currency THEN c.cost ELSE c.cost * c.fx_rate END)
				/ c.kwh_added AS price
			FROM charge_logs c JOIN vehicles v ON v.id = c.vehicle_id
			WHERE c.vehicle_id = $1 AND c.deleted_upstream_at IS NULL
				AND c.cost IS NOT NULL AND c.kwh_added > 0
			UNION ALL
			SELECT 'fuel', f.amount / f.liters
			FROM fuel_logs f WHERE f.vehicle_id = $1 AND f.fuel_type = $2
				AND f.liters > 0
		), usable AS (
			SELECT kind, price FROM prices WHERE price >= 0
		)
		SELECT v.currency,
			COUNT(*) FILTER (WHERE kind = 'fuel'),
			percentile_cont(0.5) WITHIN GROUP (ORDER BY price) FILTER (WHERE kind = 'fuel'),
			COUNT(*) FILTER (WHERE kind = 'electricity'),
			percentile_cont(0.5) WITHIN GROUP (ORDER BY price) FILTER (WHERE kind = 'electricity')
		FROM vehicles v LEFT JOIN usable ON TRUE WHERE v.id = $1 GROUP BY v.currency;
	`, vehicleID, fuelType).Scan(&history.Currency, &history.Fuel.Count, &history.Fuel.Median,
		&history.Electricity.Count, &history.Electricity.Median)
	return &history, err
}
