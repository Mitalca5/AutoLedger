package database

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// GetFleetSummary aggregates multi-vehicle household metrics.
func (r *Repository) GetFleetSummary(ctx context.Context, userID string) (*models.FleetSummaryResponse, error) {
	vehicles, err := r.ListVehiclesByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch household vehicles: %w", err)
	}

	baseCurrency := "EUR"
	if len(vehicles) > 0 {
		baseCurrency = vehicles[0].Currency
	}

	res := &models.FleetSummaryResponse{
		TotalVehicles:  len(vehicles),
		Currency:       baseCurrency,
		MonthlyCosts:   []models.FleetMonthlyCost{},
		Vehicles:       []models.VehicleFleetMetric{},
		MemberKmShares: []models.MemberKmShare{},
	}

	if len(vehicles) == 0 {
		return res, nil
	}

	vehicleIDs := make([]string, len(vehicles))
	vehicleMap := make(map[string]models.Vehicle, len(vehicles))
	for i, v := range vehicles {
		vehicleIDs[i] = v.ID
		vehicleMap[v.ID] = v
	}

	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	startOfHistory := startOfMonth.AddDate(0, -11, 0)

	// 1. Per-vehicle current month distance
	monthlyDistQuery := `
		SELECT vehicle_id, COALESCE(SUM(distance_km), 0)
		FROM drives
		WHERE vehicle_id::text = ANY($1)
		  AND start_time >= $2
		  AND deleted_upstream_at IS NULL
		GROUP BY vehicle_id;
	`
	monthDistByVeh := make(map[string]float64)
	rows, err := r.pool.Query(ctx, monthlyDistQuery, vehicleIDs, startOfMonth)
	if err == nil {
		for rows.Next() {
			var vid string
			var km float64
			if err := rows.Scan(&vid, &km); err == nil {
				monthDistByVeh[vid] = km
			}
		}
		rows.Close()
	}

	// 2. Per-vehicle current month energy kWh
	monthlyEnergyQuery := `
		SELECT vehicle_id, COALESCE(SUM(kwh_added), 0)
		FROM charge_logs
		WHERE vehicle_id::text = ANY($1)
		  AND date >= $2
		  AND deleted_upstream_at IS NULL
		GROUP BY vehicle_id;
	`
	monthEnergyByVeh := make(map[string]float64)
	rows, err = r.pool.Query(ctx, monthlyEnergyQuery, vehicleIDs, startOfMonth)
	if err == nil {
		for rows.Next() {
			var vid string
			var kwh float64
			if err := rows.Scan(&vid, &kwh); err == nil {
				monthEnergyByVeh[vid] = kwh
			}
		}
		rows.Close()
	}

	// 3. Per-vehicle all-time energy cost & distance to compute energy cost per 100km
	energyCostPer100KmByVeh := make(map[string]float64)
	statsQuery := `
		SELECT c.vehicle_id,
		       c.amount_eur,
		       COALESCE(d.total_km, 0)
		FROM (
			SELECT vehicle_id, SUM(amount_eur) as amount_eur
			FROM cost_ledger
			WHERE vehicle_id::text = ANY($1) AND category = 'ENERGY'
			GROUP BY vehicle_id
		) c
		LEFT JOIN (
			SELECT vehicle_id, SUM(distance_km) as total_km
			FROM drives
			WHERE vehicle_id::text = ANY($1) AND deleted_upstream_at IS NULL
			GROUP BY vehicle_id
		) d ON d.vehicle_id = c.vehicle_id;
	`
	rows, err = r.pool.Query(ctx, statsQuery, vehicleIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch energy cost per distance: %w", err)
	}
	for rows.Next() {
		var vid string
		var totalEnergyEur float64
		var totalKm float64
		if err := rows.Scan(&vid, &totalEnergyEur, &totalKm); err == nil && totalKm > 0 {
			energyCostPer100KmByVeh[vid] = math.Round((totalEnergyEur/(totalKm/100.0))*100) / 100
		}
	}
	rows.Close()

	// 4. Current month costs from cost_ledger
	monthCostByVeh := make(map[string]money.Cents)
	ledgerMonthQuery := `
		SELECT vehicle_id, COALESCE(SUM(amount_eur), 0)
		FROM cost_ledger
		WHERE vehicle_id::text = ANY($1) AND entry_date >= $2
		GROUP BY vehicle_id;
	`
	rows, err = r.pool.Query(ctx, ledgerMonthQuery, vehicleIDs, startOfMonth)
	if err == nil {
		for rows.Next() {
			var vid string
			var costEur float64
			if err := rows.Scan(&vid, &costEur); err == nil {
				monthCostByVeh[vid] = money.FromFloat(costEur)
			}
		}
		rows.Close()
	}

	for _, v := range vehicles {
		mDist := monthDistByVeh[v.ID]
		mCost := monthCostByVeh[v.ID]
		mKwh := monthEnergyByVeh[v.ID]
		eff := energyCostPer100KmByVeh[v.ID]

		res.CurrentMonthCost += mCost
		res.CurrentMonthDistanceKm += mDist
		res.CurrentMonthEnergyKwh += mKwh

		res.Vehicles = append(res.Vehicles, models.VehicleFleetMetric{
			VehicleID:          v.ID,
			Name:               v.Name,
			Make:               v.Make,
			Model:              v.Model,
			Currency:           v.Currency,
			CurrentOdometer:    v.CurrentOdometer,
			MonthDistanceKm:    mDist,
			MonthCost:          mCost,
			EnergyCostPer100Km: eff,
		})
	}

	// 5. Monthly fleet history (last 12 months)
	historyQuery := `
		SELECT TO_CHAR(entry_date, 'YYYY-MM') as month_str,
		       vehicle_id,
		       category,
		       COALESCE(SUM(amount_eur), 0) as total_eur
		FROM cost_ledger
		WHERE vehicle_id::text = ANY($1) AND entry_date >= $2
		GROUP BY month_str, vehicle_id, category
		ORDER BY month_str ASC;
	`
	type monthBucket struct {
		totalCost  money.Cents
		energyCost money.Cents
		otherCost  money.Cents
		byVeh      map[string]money.Cents
	}
	buckets := make(map[string]*monthBucket)

	rows, err = r.pool.Query(ctx, historyQuery, vehicleIDs, startOfHistory)
	if err == nil {
		for rows.Next() {
			var mStr, vid, cat string
			var eur float64
			if err := rows.Scan(&mStr, &vid, &cat, &eur); err == nil {
				b, ok := buckets[mStr]
				if !ok {
					b = &monthBucket{byVeh: make(map[string]money.Cents)}
					buckets[mStr] = b
				}
				cents := money.FromFloat(eur)
				b.totalCost += cents
				if cat == "ENERGY" {
					b.energyCost += cents
				} else {
					b.otherCost += cents
				}
				b.byVeh[vid] += cents
			}
		}
		rows.Close()
	}

	// Distance per month for history
	distHistoryQuery := `
		SELECT TO_CHAR(start_time, 'YYYY-MM') as month_str,
		       COALESCE(SUM(distance_km), 0)
		FROM drives
		WHERE vehicle_id::text = ANY($1) AND start_time >= $2 AND deleted_upstream_at IS NULL
		GROUP BY month_str;
	`
	distByMonth := make(map[string]float64)
	rows, err = r.pool.Query(ctx, distHistoryQuery, vehicleIDs, startOfHistory)
	if err == nil {
		for rows.Next() {
			var mStr string
			var km float64
			if err := rows.Scan(&mStr, &km); err == nil {
				distByMonth[mStr] = km
			}
		}
		rows.Close()
	}

	// Generate ordered month list
	for i := 0; i < 12; i++ {
		mDate := startOfHistory.AddDate(0, i, 0)
		mStr := mDate.Format("2006-01")
		b := buckets[mStr]
		if b == nil {
			b = &monthBucket{byVeh: make(map[string]money.Cents)}
		}
		res.MonthlyCosts = append(res.MonthlyCosts, models.FleetMonthlyCost{
			Month:      mStr,
			TotalCost:  b.totalCost,
			EnergyCost: b.energyCost,
			OtherCost:  b.otherCost,
			DistanceKm: distByMonth[mStr],
			ByVehicle:  b.byVeh,
		})
	}

	// 6. Member Km Shares
	driverSharesQuery := `
		SELECT COALESCE(d.driver_id, v.default_driver_id, v.user_id)::text AS effective_driver_id,
		       COALESCE(u.email, 'Other') AS driver_email,
		       COALESCE(SUM(d.distance_km), 0) AS total_km
		FROM drives d
		JOIN vehicles v ON v.id = d.vehicle_id
		LEFT JOIN users u ON u.id = COALESCE(d.driver_id, v.default_driver_id, v.user_id)
		WHERE v.id::text = ANY($1)
		  AND d.deleted_upstream_at IS NULL
		  AND d.start_time >= $2
		GROUP BY effective_driver_id, u.email
		ORDER BY total_km DESC;
	`
	totalFleetKm := 0.0
	rows, err = r.pool.Query(ctx, driverSharesQuery, vehicleIDs, startOfHistory)
	if err == nil {
		type driverRaw struct {
			id    string
			email string
			km    float64
		}
		var rawList []driverRaw
		for rows.Next() {
			var dr driverRaw
			if err := rows.Scan(&dr.id, &dr.email, &dr.km); err == nil {
				totalFleetKm += dr.km
				rawList = append(rawList, dr)
			}
		}
		rows.Close()

		for _, dr := range rawList {
			pct := 0.0
			if totalFleetKm > 0 {
				pct = math.Round((dr.km/totalFleetKm)*1000) / 10.0 // 1 decimal place
			}
			res.MemberKmShares = append(res.MemberKmShares, models.MemberKmShare{
				UserID:      dr.id,
				DisplayName: dr.email,
				DistanceKm:  math.Round(dr.km*10) / 10.0,
				Percentage:  pct,
			})
		}
	}

	return res, nil
}
