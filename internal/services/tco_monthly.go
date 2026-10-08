package services

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// monthlyCosts builds the cash-basis monthly timeline (acquisition excluded) in the reporting timezone,
// including linear smoothing of missing mileage between odometer checkpoints.
func (s *TCOService) monthlyCosts(ctx context.Context, vehicleID string, ownership *models.VehicleOwnership, currentOdometer float64, estKwh100km, estPricePerKwh *float64, now time.Time) ([]MonthlyCost, float64, float64, error) {
	monthlyMap := make(map[string]*MonthlyCost)
	get := func(m string) *MonthlyCost {
		if _, ok := monthlyMap[m]; !ok {
			monthlyMap[m] = &MonthlyCost{Month: m}
		}
		return monthlyMap[m]
	}

	rows, err := s.pool.Query(ctx, `
		SELECT TO_CHAR(entry_date AT TIME ZONE $2, 'YYYY-MM') AS m, category, SUM(amount_eur)
		FROM cost_ledger
		WHERE vehicle_id = $1 AND category <> 'ACQUISITION'
		GROUP BY m, category;
	`, vehicleID, s.timezone)
	if err != nil {
		return nil, 0, 0, err
	}
	for rows.Next() {
		var m, category string
		var amount money.Cents
		if err := rows.Scan(&m, &category, &amount); err != nil {
			rows.Close()
			return nil, 0, 0, err
		}
		mc := get(m)
		switch category {
		case LedgerEnergy:
			mc.Energy += amount
		case LedgerToll, LedgerParking, LedgerTravelOther:
			mc.Tolls += amount
		case LedgerTires:
			mc.Tires += amount
		case LedgerMaintenance, LedgerRepair:
			mc.Maintenance += amount
		case LedgerInsurance:
			mc.Insurance += amount
		case LedgerFinancing:
			mc.Financing += amount
		default:
			mc.Other += amount
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, 0, err
	}

	distRows, err := s.pool.Query(ctx, `
		SELECT TO_CHAR(start_time AT TIME ZONE $2, 'YYYY-MM') AS m, COALESCE(SUM(distance_km), 0)
		FROM drives WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL GROUP BY m;
	`, vehicleID, s.timezone)
	if err != nil {
		return nil, 0, 0, err
	}
	for distRows.Next() {
		var m string
		var km float64
		if err := distRows.Scan(&m, &km); err != nil {
			distRows.Close()
			return nil, 0, 0, err
		}
		get(m).DistanceKm += km
	}
	distRows.Close()
	if err := distRows.Err(); err != nil {
		return nil, 0, 0, err
	}

	// Compute smoothed missing distance between checkpoints
	smoothedMap, preTmMap, err := s.computeMileageSmoothing(ctx, vehicleID, ownership, currentOdometer, now)
	if err != nil {
		return nil, 0, 0, err
	}
	var rawSmoothedTotal float64
	for m, smoothed := range smoothedMap {
		if smoothed > 0 {
			get(m).SmoothedKm += round1(smoothed)
			rawSmoothedTotal += smoothed
		}
	}
	targetSmoothed := round1(rawSmoothedTotal)

	var rawPreTmTotal float64
	for m, preKm := range preTmMap {
		if preKm > 0 {
			get(m).EstimatedEnergyKm += round1(preKm)
			rawPreTmTotal += preKm
		}
	}
	targetPreTm := round1(rawPreTmTotal)

	// Tires: prorate each purchase by km driven while mounted instead of dumping the full
	// price into the purchase month, so a low-mileage purchase month doesn't spike cost/km.
	tireAmortMap, err := s.computeMonthlyTireAmortization(ctx, vehicleID, now, smoothedMap)
	if err != nil {
		return nil, 0, 0, err
	}
	for m, amount := range tireAmortMap {
		get(m).TiresAmortized += amount
	}

	var totalSmoothed, totalEstimated float64
	var lastSmoothedMc, lastEstimatedMc *MonthlyCost
	for _, mc := range monthlyMap {
		mc.TrackedDistanceKm = round1(mc.DistanceKm)
		mc.SmoothedKm = round1(mc.SmoothedKm)
		mc.EstimatedEnergyKm = round1(mc.EstimatedEnergyKm)
		if mc.SmoothedKm > 0 {
			lastSmoothedMc = mc
		}
		if mc.EstimatedEnergyKm > 0 {
			lastEstimatedMc = mc
		}
		mc.DistanceKm = round1(mc.TrackedDistanceKm + mc.SmoothedKm)
		totalSmoothed += mc.SmoothedKm
		totalEstimated += mc.EstimatedEnergyKm
		if estKwh100km != nil && *estKwh100km > 0 && estPricePerKwh != nil && *estPricePerKwh > 0 && mc.EstimatedEnergyKm > 0 {
			mc.SmoothedKwh = round1(mc.EstimatedEnergyKm * (*estKwh100km / 100.0))
			mc.SmoothedEnergy = money.FromFloat(mc.SmoothedKwh * *estPricePerKwh)
			mc.Energy += mc.SmoothedEnergy
		}
	}

	if lastSmoothedMc != nil && targetSmoothed > 0 {
		diff := round1(targetSmoothed - totalSmoothed)
		if math.Abs(diff) > 0.001 && math.Abs(diff) < 1.0 {
			lastSmoothedMc.SmoothedKm = round1(lastSmoothedMc.SmoothedKm + diff)
			lastSmoothedMc.DistanceKm = round1(lastSmoothedMc.TrackedDistanceKm + lastSmoothedMc.SmoothedKm)
			totalSmoothed = targetSmoothed
		}
	}

	if lastEstimatedMc != nil && targetPreTm > 0 {
		diff := round1(targetPreTm - totalEstimated)
		if math.Abs(diff) > 0.001 && math.Abs(diff) < 1.0 {
			lastEstimatedMc.EstimatedEnergyKm = round1(lastEstimatedMc.EstimatedEnergyKm + diff)
			totalEstimated = targetPreTm
			if estKwh100km != nil && *estKwh100km > 0 && estPricePerKwh != nil && *estPricePerKwh > 0 && lastEstimatedMc.EstimatedEnergyKm > 0 {
				oldEnergy := lastEstimatedMc.SmoothedEnergy
				lastEstimatedMc.SmoothedKwh = round1(lastEstimatedMc.EstimatedEnergyKm * (*estKwh100km / 100.0))
				lastEstimatedMc.SmoothedEnergy = money.FromFloat(lastEstimatedMc.SmoothedKwh * *estPricePerKwh)
				lastEstimatedMc.Energy = lastEstimatedMc.Energy - oldEnergy + lastEstimatedMc.SmoothedEnergy
			}
		}
	}

	monthlyDistances := make(map[string]float64, len(monthlyMap))
	for _, mc := range monthlyMap {
		monthlyDistances[mc.Month] = mc.DistanceKm
	}

	maintAmortMap, err := s.computeMonthlyMaintenanceAmortization(ctx, vehicleID, monthlyDistances, now)
	if err != nil {
		return nil, 0, 0, err
	}
	for m, amount := range maintAmortMap {
		get(m).MaintenanceAmortized += amount
	}

	if dueMonth, due, err := s.financingDueLaterThisMonth(ctx, vehicleID); err != nil {
		return nil, 0, 0, err
	} else if due > 0 {
		get(dueMonth).Financing += due
	}

	s.computeMonthlyFinancingAmortization(ownership, monthlyMap, now)

	monthlyCosts := make([]MonthlyCost, 0, len(monthlyMap))
	for _, mc := range monthlyMap {
		mc.Total = mc.Energy + mc.Tolls + mc.Maintenance + mc.Insurance + mc.Financing + mc.Other + mc.Tires
		costForPerKm := mc.Energy + mc.Tolls + mc.MaintenanceAmortized + mc.Insurance + mc.FinancingAmortized + mc.Other + mc.TiresAmortized
		mc.CostPerKm = perKm(costForPerKm, mc.DistanceKm)
		monthlyCosts = append(monthlyCosts, *mc)
	}

	sort.Slice(monthlyCosts, func(i, j int) bool {
		return monthlyCosts[i].Month < monthlyCosts[j].Month
	})

	if len(monthlyCosts) == 0 {
		monthlyCosts = append(monthlyCosts, MonthlyCost{Month: now.Format("2006-01")})
	}
	return monthlyCosts, round1(totalSmoothed), round1(totalEstimated), nil
}
