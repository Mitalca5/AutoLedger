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

// fleetStore is the slice of *database.Repository that FleetService uses.
type fleetStore interface {
	GetFleetSummary(ctx context.Context, userID, timezone string) (*models.FleetSummaryResponse, error)
	SetFleetMonthlyBudget(ctx context.Context, userID string, budget *money.Cents) error
}

// vehicleTCO is the slice of *TCOService that FleetService uses.
type vehicleTCO interface {
	ComputeVehicleTCO(ctx context.Context, vehicleID string) (*TCOSummary, error)
}

// energyStatsReader is the slice of *EnergyStatsService that FleetService uses.
type energyStatsReader interface {
	Compute(ctx context.Context, vehicleID string) (*EnergyStats, error)
}

// FleetService coordinates aggregated multi-vehicle analytics for households.
type FleetService struct {
	repo fleetStore
	tco  vehicleTCO
	// energy gives the energy cost per 100 km, the one definition shared with the dashboard.
	energy energyStatsReader
	// timezone is the reporting timezone: its calendar months bound the monthly figures.
	timezone string
}

func NewFleetService(repo *database.Repository, tco *TCOService) *FleetService {
	return &FleetService{repo: repo, tco: tco, energy: NewEnergyStatsService(tco.pool, tco.timezone), timezone: tco.timezone}
}

// GetSummary returns consolidated KPIs, monthly breakdown and driver shares for all vehicles of a household, with
// the per-kilometre comparison figures of each vehicle taken from its TCO.
func (s *FleetService) GetSummary(ctx context.Context, userID string) (*models.FleetSummaryResponse, error) {
	res, err := s.repo.GetFleetSummary(ctx, userID, s.timezone)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	for i := range res.Vehicles {
		res.Vehicles[i].EnergyCostPer100Km = s.energyCostPer100Km(ctx, res.Vehicles[i].VehicleID)
	}
	sums := make(map[string]*TCOSummary, len(res.Vehicles))
	for i := range res.Vehicles {
		sum, err := s.tco.ComputeVehicleTCO(ctx, res.Vehicles[i].VehicleID)
		if err != nil {
			slog.Warn("fleet comparison without TCO", "vehicle_id", res.Vehicles[i].VehicleID, "error", err)
			continue
		}
		sums[res.Vehicles[i].VehicleID] = sum
		applyTCOToFleetMetric(&res.Vehicles[i], sum, now)
	}
	if len(sums) == len(res.Vehicles) {
		applyTCOMonthsToFleet(res, sums, now)
	}
	return res, nil
}

// energyCostPer100Km is the cost per 100 km of the energy statistics, 0 when there is no tracked distance or no priced
// charge to derive it from.
func (s *FleetService) energyCostPer100Km(ctx context.Context, vehicleID string) float64 {
	stats, err := s.energy.Compute(ctx, vehicleID)
	if err != nil {
		slog.Warn("fleet energy cost without energy statistics", "vehicle_id", vehicleID, "error", err)
		return 0
	}
	if stats.Summary.CostPer100km == nil {
		return 0
	}
	return *stats.Summary.CostPer100km
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
	m.DepreciationKnown = sum.DepreciationKnown
	// A bought vehicle without depreciation leaves out its largest cost item: ranking it first would reward the gap.
	m.Comparable = sum.DistanceBasisKm > 0 && sum.FullCost > 0 && sum.DepreciationKnown && sum.Completeness.ScorePct >= fleetComparableMinCompleteness

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
