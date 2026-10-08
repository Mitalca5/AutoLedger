package services

import (
	"context"
	"math"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

const (
	avgMonthDays = 30.4375
	// maxProgress bounds the extrapolation past the planned resale date.
	maxProgress = 2.0
	// minCapacityRatio and maxCapacityRatio reject capacity ratios that cannot be a state of health.
	minCapacityRatio = 0.3
	maxCapacityRatio = 1.1
)

// residualStore is the slice of *database.Repository that ResidualService reads.
type residualStore interface {
	ListBatterySnapshots(ctx context.Context, vehicleID string) ([]models.BatterySnapshot, error)
	GetVehicleOwnership(ctx context.Context, vehicleID string) (*models.VehicleOwnership, error)
}

// ResidualService derives the battery state of health and the residual value of a vehicle.
type ResidualService struct {
	repo  residualStore
	stats *EnergyStatsService
	now   func() time.Time
}

func NewResidualService(repo residualStore, stats *EnergyStatsService) *ResidualService {
	return &ResidualService{repo: repo, stats: stats, now: time.Now}
}

// ResidualOptions are the user's assumptions for the projection.
type ResidualOptions struct {
	// ExpectedKm is the distance planned over the holding period; 0 means the value depends on age only.
	ExpectedKm float64
	// KmShare is the weight of the distance (0..1) against the age in the depreciation.
	KmShare float64
	// HealthWeight is the share of value lost per point of state of health lost (1 = proportional, 0 = ignored).
	HealthWeight float64
}

// BatteryHealth summarises the state of health of a vehicle's battery.
func (s *ResidualService) BatteryHealth(ctx context.Context, vehicleID string) (*models.BatteryHealthSummary, error) {
	readings, err := s.repo.ListBatterySnapshots(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	var estimated *float64
	if s.stats != nil {
		stats, err := s.stats.Compute(ctx, vehicleID)
		if err != nil {
			return nil, err
		}
		estimated = stats.Summary.EstimatedCapacityKwh
	}
	return summariseBatteryHealth(readings, estimated), nil
}

// summariseBatteryHealth picks the best available state of health: a recorded percentage, then recorded
// capacities, then the capacity estimated from charging sessions compared with the new-battery capacity.
func summariseBatteryHealth(readings []models.BatterySnapshot, estimated *float64) *models.BatteryHealthSummary {
	out := &models.BatteryHealthSummary{Readings: readings, EstimatedCapacityKwh: estimated}
	for i := len(readings) - 1; i >= 0; i-- {
		if readings[i].MaxCapacityKwh != nil && *readings[i].MaxCapacityKwh > 0 {
			out.ReferenceKwh = readings[i].MaxCapacityKwh
			break
		}
	}
	for i := len(readings) - 1; i >= 0; i-- {
		r := readings[i]
		if r.HealthPercent != nil {
			out.HealthPercent, out.Source = r.HealthPercent, "reading"
			return out
		}
		if r.CurrentCapacityKwh != nil && r.MaxCapacityKwh != nil && *r.MaxCapacityKwh > 0 {
			if pct, ok := capacityPercent(*r.CurrentCapacityKwh, *r.MaxCapacityKwh); ok {
				out.HealthPercent, out.Source = &pct, "capacity"
				return out
			}
		}
	}
	if estimated != nil && out.ReferenceKwh != nil {
		if pct, ok := capacityPercent(*estimated, *out.ReferenceKwh); ok {
			out.HealthPercent, out.Source = &pct, "charges"
		}
	}
	return out
}

func capacityPercent(current, reference float64) (float64, bool) {
	ratio := current / reference
	if ratio < minCapacityRatio || ratio > maxCapacityRatio {
		return 0, false
	}
	return math.Round(math.Min(ratio, 1)*1000) / 10, true
}

// Residual projects the resale value of a vehicle from its ownership contract.
func (s *ResidualService) Residual(ctx context.Context, vehicle *models.Vehicle, opts ResidualOptions) (*models.ResidualValue, error) {
	own, err := s.repo.GetVehicleOwnership(ctx, vehicle.ID)
	if err != nil {
		if err == database.ErrNotFound {
			return nil, apierror.New("residual.missing_inputs", "Set the purchase price, expected resale value and holding period first")
		}
		return nil, err
	}
	health, err := s.BatteryHealth(ctx, vehicle.ID)
	if err != nil {
		return nil, err
	}
	startKm := 0.0
	if own.StartOdometer != nil {
		startKm = *own.StartOdometer
	}
	ageMonths := s.now().Sub(own.StartDate).Hours() / 24 / avgMonthDays
	return computeResidual(own, ageMonths, math.Max(vehicle.CurrentOdometer-startKm, 0), health.HealthPercent, opts)
}

func computeResidual(own *models.VehicleOwnership, ageMonths, distanceKm float64, healthPct *float64, opts ResidualOptions) (*models.ResidualValue, error) {
	if own.PurchasePrice == nil || own.ExpectedResaleValue == nil || own.ExpectedHoldingMonths == nil ||
		*own.PurchasePrice <= 0 || *own.ExpectedResaleValue <= 0 || *own.ExpectedHoldingMonths <= 0 || *own.ExpectedResaleValue > *own.PurchasePrice {
		return nil, apierror.New("residual.missing_inputs", "Set the purchase price, expected resale value and holding period first")
	}
	if opts.KmShare < 0 || opts.KmShare > 1 || opts.ExpectedKm < 0 || opts.HealthWeight < 0 || opts.HealthWeight > 2 {
		return nil, apierror.New("residual.invalid_options", "Invalid projection assumptions")
	}
	price, resale, holding := own.PurchasePrice.Float(), own.ExpectedResaleValue.Float(), float64(*own.ExpectedHoldingMonths)
	share := opts.KmShare
	if opts.ExpectedKm <= 0 {
		share = 0
	}
	ageProgress := math.Max(ageMonths, 0) / holding
	progress := ageProgress
	if share > 0 {
		progress = (1-share)*ageProgress + share*distanceKm/opts.ExpectedKm
	}
	progress = math.Min(progress, maxProgress)

	factor := 1.0
	if healthPct != nil {
		factor = math.Max(1-opts.HealthWeight*(100-*healthPct)/100, 0)
	}
	valueAt := func(p float64) money.Cents { return money.FromFloat(price * math.Pow(resale/price, p) * factor) }

	out := &models.ResidualValue{
		PurchasePrice: *own.PurchasePrice, ExpectedValue: *own.ExpectedResaleValue, HoldingMonths: *own.ExpectedHoldingMonths,
		AgeMonths: math.Round(ageMonths*10) / 10, DistanceKm: math.Round(distanceKm),
		Progress: math.Round(progress*1000) / 1000, HealthFactor: math.Round(factor*1000) / 1000, HealthPercent: healthPct,
		CurrentValue: valueAt(progress),
	}
	out.Depreciation = *own.PurchasePrice - out.CurrentValue
	for m := 0; m <= *own.ExpectedHoldingMonths; m++ {
		out.Curve = append(out.Curve, models.ResidualValuePoint{Month: m, Value: valueAt(float64(m) / holding)})
	}
	return out, nil
}
