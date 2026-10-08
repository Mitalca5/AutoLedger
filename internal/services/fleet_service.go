package services

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

const (
	// fleetComparableMinCompleteness is the completeness score from which a vehicle is ranked against the others.
	fleetComparableMinCompleteness = 60
	// fleetAnnualMinMonths is the history needed before a cost is extrapolated to a year.
	fleetAnnualMinMonths = 3
)

// FleetService coordinates aggregated multi-vehicle analytics for households.
type FleetService struct {
	repo *database.Repository
	tco  *TCOService
}

func NewFleetService(repo *database.Repository, tco *TCOService) *FleetService {
	return &FleetService{repo: repo, tco: tco}
}

// GetSummary returns consolidated KPIs, monthly breakdown and driver shares for all vehicles of a household, with
// the per-kilometre comparison figures of each vehicle taken from its TCO.
func (s *FleetService) GetSummary(ctx context.Context, userID string) (*models.FleetSummaryResponse, error) {
	res, err := s.repo.GetFleetSummary(ctx, userID)
	if err != nil {
		return nil, err
	}
	sums := make(map[string]*TCOSummary, len(res.Vehicles))
	for i := range res.Vehicles {
		sum, err := s.tco.ComputeVehicleTCO(ctx, res.Vehicles[i].VehicleID)
		if err != nil {
			slog.Warn("fleet comparison without TCO", "vehicle_id", res.Vehicles[i].VehicleID, "error", err)
			continue
		}
		sums[res.Vehicles[i].VehicleID] = sum
		applyTCOToFleetMetric(&res.Vehicles[i], sum, time.Now())
	}
	if len(sums) == len(res.Vehicles) {
		applyTCOMonthsToFleet(res, sums, time.Now())
	}
	return res, nil
}

// applyTCOMonthsToFleet rebuilds the month figures of the fleet from the monthly costs of each vehicle's TCO, so the
// household total is the sum of the per-vehicle charts: same month boundaries (APP_TIMEZONE) and the same smoothed
// energy for months without a recorded charge.
func applyTCOMonthsToFleet(res *models.FleetSummaryResponse, sums map[string]*TCOSummary, now time.Time) {
	byVehicle := make(map[string]map[string]MonthlyCost, len(sums))
	for id, sum := range sums {
		months := make(map[string]MonthlyCost, len(sum.MonthlyCosts))
		for _, mc := range sum.MonthlyCosts {
			months[mc.Month] = mc
		}
		byVehicle[id] = months
	}

	for i := range res.MonthlyCosts {
		fm := &res.MonthlyCosts[i]
		fm.TotalCost, fm.EnergyCost, fm.OtherCost = 0, 0, 0
		fm.ByVehicle = make(map[string]money.Cents, len(sums))
		for id, months := range byVehicle {
			mc, ok := months[fm.Month]
			if !ok {
				continue
			}
			fm.TotalCost += mc.Total
			fm.EnergyCost += mc.Energy
			fm.OtherCost += mc.Total - mc.Energy
			fm.ByVehicle[id] = mc.Total
		}
	}

	current := now.Format("2006-01")
	res.CurrentMonthCost = 0
	for i := range res.Vehicles {
		cost := byVehicle[res.Vehicles[i].VehicleID][current].Total
		res.Vehicles[i].MonthCost = cost
		res.CurrentMonthCost += cost
	}
}

// applyTCOToFleetMetric fills the comparison figures of a vehicle from its TCO summary.
func applyTCOToFleetMetric(m *models.VehicleFleetMetric, sum *TCOSummary, now time.Time) {
	m.Powertrain = sum.Powertrain
	m.RunningCostPerKm = sum.TotalCostPerKm
	m.FullCostPerKm = sum.FullCostPerKm
	m.CompletenessPct = sum.Completeness.ScorePct
	m.Comparable = sum.DistanceBasisKm > 0 && sum.FullCost > 0 && sum.Completeness.ScorePct >= fleetComparableMinCompleteness

	if sum.DistanceBasisKm > 0 {
		if sum.TotalKwhAdded > 0 {
			v := round1(sum.TotalKwhAdded / sum.DistanceBasisKm * 100)
			m.KwhPer100Km = &v
		}
		switch {
		case sum.ConsumptionL100km != nil:
			v := round1(*sum.ConsumptionL100km)
			m.LitersPer100Km = &v
		case sum.TotalLiters > 0:
			v := round1(sum.TotalLiters / sum.DistanceBasisKm * 100)
			m.LitersPer100Km = &v
		}
	}

	if months := observedMonths(sum.MonthlyCosts, now); months >= fleetAnnualMinMonths {
		annual := money.Cents(math.Round(float64(sum.FullCost) * 12 / float64(months)))
		m.AnnualCost = &annual
	}
}

// observedMonths counts the calendar months from the first month with data to the current one, both included.
func observedMonths(costs []MonthlyCost, now time.Time) int {
	if len(costs) == 0 {
		return 0
	}
	first, err := time.Parse("2006-01", costs[0].Month)
	if err != nil {
		return 0
	}
	return (now.Year()-first.Year())*12 + int(now.Month()-first.Month()) + 1
}

// SetMonthlyBudget stores the household monthly budget; nil removes it.
func (s *FleetService) SetMonthlyBudget(ctx context.Context, userID string, budget *money.Cents) error {
	return s.repo.SetFleetMonthlyBudget(ctx, userID, budget)
}
