package services

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

// computeMonthlyMaintenanceAmortization computes month-by-month amortized maintenance costs.
// For expenses with amortization_mode != 'NONE':
//   - DISTANCE: amortized by km driven up to coverage_km (default 50,000 km)
//   - DURATION: amortized evenly across coverage_months (default 24 months)
//   - HYBRID: amortized at the faster of the two paces (au premier des deux termes échu)
//   - Closes: if a subsequent expense has closes_maintenance_id pointing to this expense,
//     amortization for this expense ceases starting at that closing month.
//
// For expenses with amortization_mode == 'NONE', the full amount is recognized in the expense month.
func (s *TCOService) computeMonthlyMaintenanceAmortization(ctx context.Context, vehicleID string, monthlyDistances map[string]float64, now time.Time) (map[string]money.Cents, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, date, TO_CHAR(date AT TIME ZONE $2, 'YYYY-MM') AS m,
		       ROUND((CASE WHEN currency = (SELECT currency FROM vehicles WHERE id = $1) THEN amount ELSE amount * COALESCE(fx_rate, 1.0) END) * 100, 0)::bigint,
		       COALESCE(amortization_mode, 'NONE'),
		       COALESCE(coverage_km, 50000),
		       COALESCE(coverage_months, 24),
		       closes_maintenance_id::text
		FROM maintenance_expenses
		WHERE vehicle_id = $1 AND amount > 0
		  AND category IN ('MAINTENANCE', 'REPAIR')
		ORDER BY date ASC, created_at ASC;
	`, vehicleID, s.timezone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*MaintenanceAmortItem
	for rows.Next() {
		var item MaintenanceAmortItem
		var closesID *string
		if err := rows.Scan(&item.ID, &item.Date, &item.Month, &item.AmountEur, &item.Mode, &item.CoverageKm, &item.CoverageMonths, &closesID); err != nil {
			return nil, err
		}
		item.ClosesMaintenanceID = closesID
		if item.CoverageKm <= 0 {
			item.CoverageKm = 50000
		}
		if item.CoverageMonths <= 0 {
			item.CoverageMonths = 24
		}
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return calculateMaintenanceAmortization(items, monthlyDistances, now.Format("2006-01")), nil
}

// MaintenanceAmortItem represents an expense line input for maintenance amortization.
type MaintenanceAmortItem struct {
	ID                  string
	Date                time.Time
	Month               string
	AmountEur           money.Cents
	Mode                string
	CoverageKm          float64
	CoverageMonths      int
	ClosesMaintenanceID *string
}

// calculateMaintenanceAmortization prorates maintenance expenses across months without modifying past history.
func calculateMaintenanceAmortization(items []*MaintenanceAmortItem, monthlyDistances map[string]float64, nowMonth string) map[string]money.Cents {
	if len(items) == 0 {
		return nil
	}

	// Map of which expense is closed by which other expense and at which month
	closedAtMonth := make(map[string]string)
	for _, item := range items {
		if item.ClosesMaintenanceID != nil && *item.ClosesMaintenanceID != "" {
			closedAtMonth[*item.ClosesMaintenanceID] = item.Month
		}
	}

	// Collect and sort all relevant months
	allMonthsMap := make(map[string]bool)
	for m := range monthlyDistances {
		allMonthsMap[m] = true
	}
	for _, item := range items {
		allMonthsMap[item.Month] = true
	}
	if nowMonth != "" {
		allMonthsMap[nowMonth] = true
	}

	sortedMonths := make([]string, 0, len(allMonthsMap))
	for m := range allMonthsMap {
		sortedMonths = append(sortedMonths, m)
	}
	sort.Strings(sortedMonths)

	result := make(map[string]money.Cents)

	for _, item := range items {
		if item.Mode == "NONE" || item.Mode == "" {
			result[item.Month] += item.AmountEur
			continue
		}

		cutoffMonth := closedAtMonth[item.ID]
		var cumAmortized money.Cents
		var cumKm float64
		startMonthIdx := -1

		for idx, m := range sortedMonths {
			if m < item.Month {
				continue
			}
			if cutoffMonth != "" && m >= cutoffMonth {
				break
			}
			if cumAmortized >= item.AmountEur {
				break
			}

			if startMonthIdx == -1 {
				startMonthIdx = idx
			}
			monthElapsed := idx - startMonthIdx

			kmInMonth := monthlyDistances[m]
			cumKm += kmInMonth

			var monthShare money.Cents
			switch item.Mode {
			case "DISTANCE":
				covKm := item.CoverageKm
				if covKm <= 0 {
					covKm = 50000
				}
				rate := float64(item.AmountEur) / covKm
				targetCum := money.Cents(math.Round(math.Min(cumKm, covKm) * rate))
				if targetCum > item.AmountEur {
					targetCum = item.AmountEur
				}
				if delta := targetCum - cumAmortized; delta > 0 {
					monthShare = delta
				}
			case "DURATION":
				monthsCount := item.CoverageMonths
				if monthsCount <= 0 {
					monthsCount = 24
				}
				if monthElapsed < monthsCount {
					if monthElapsed == monthsCount-1 {
						monthShare = item.AmountEur - cumAmortized
					} else {
						monthShare = money.Cents(math.Round(float64(item.AmountEur) / float64(monthsCount)))
					}
				}
			case "HYBRID":
				covKm := item.CoverageKm
				if covKm <= 0 {
					covKm = 50000
				}
				monthsCount := item.CoverageMonths
				if monthsCount <= 0 {
					monthsCount = 24
				}
				fracDist := math.Min(1.0, cumKm/covKm)
				fracTime := math.Min(1.0, float64(monthElapsed+1)/float64(monthsCount))
				fraction := math.Max(fracDist, fracTime)
				targetCum := money.Cents(math.Round(float64(item.AmountEur) * fraction))
				if targetCum > item.AmountEur {
					targetCum = item.AmountEur
				}
				if delta := targetCum - cumAmortized; delta > 0 {
					monthShare = delta
				}
			}

			if monthShare > 0 {
				if cumAmortized+monthShare > item.AmountEur {
					monthShare = item.AmountEur - cumAmortized
				}
				result[m] += monthShare
				cumAmortized += monthShare
			}
		}
	}

	return result
}
