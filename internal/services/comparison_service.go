package services

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// DefaultAnnualKm is used when a vehicle has too little history to estimate its yearly mileage.
const DefaultAnnualKm = 12000

// minMonthsForAnnualKm is the history needed before the tracked mileage is extrapolated to a year.
const minMonthsForAnnualKm = 3

// ErrComparisonNeedsVehicle is returned when a RETROSPECTIVE comparison has no reference vehicle.
var ErrComparisonNeedsVehicle = apierror.New("comparison.vehicle_required", "A vehicle is required in retrospective mode")

// ErrComparisonNeedsEV is returned when the tracked vehicle cannot record charging sessions.
var ErrComparisonNeedsEV = apierror.New("comparison.needs_ev", "The tracked comparison requires an electric or plug-in hybrid vehicle; use projection mode for a combustion vehicle")

// ICEDefault is an indicative starting point for the equivalent combustion vehicle of a given fuel.
type ICEDefault struct {
	FuelType  string  `json:"fuel_type"`
	Label     string  `json:"label"`
	LPer100Km float64 `json:"l_per_100km"`
	FuelPrice float64 `json:"fuel_price"` // vehicle currency per litre
}

// iceDefaults are indicative figures; users are expected to adjust them to their market and currency.
var iceDefaults = []ICEDefault{
	{"SP95_E10", "Petrol E10", 6.5, 1.75},
	{"SP98", "Petrol (premium)", 6.5, 1.85},
	{"DIESEL", "Diesel", 5.5, 1.70},
	{"E85", "Ethanol E85", 9.0, 0.80},
	{"GPL", "LPG", 8.0, 0.95},
}

// ComparisonDefaults prefills the comparison form. Every ICE value is an editable assumption.
type ComparisonDefaults struct {
	AnnualKm          float64           `json:"annual_km"`
	AnnualKmFromData  bool              `json:"annual_km_from_data"` // False when the default mileage is a fallback
	EVKwhPer100Km     *float64          `json:"ev_kwh_per_100km,omitempty"`
	EVEurPerKwh       *float64          `json:"ev_eur_per_kwh,omitempty"`
	Powertrain        string            `json:"powertrain,omitempty"`      // Of the reference vehicle
	ICELPer100Km      *float64          `json:"ice_l_per_100km,omitempty"` // Measured on a tracked combustion vehicle
	ICEFuelPrice      *float64          `json:"ice_fuel_price,omitempty"`  // Average EUR per litre paid
	ICE               []ICEDefault      `json:"ice"`
	MaintenanceYearly money.Cents       `json:"maintenance_yearly"`
	InsuranceYearly   money.Cents       `json:"insurance_yearly"`
	Source            *apierror.Message `json:"source"`
}

// ComparisonService builds EV baselines from the real TCO and evaluates comparison scenarios.
type ComparisonService struct {
	tco *TCOService
}

// NewComparisonService creates a ComparisonService.
func NewComparisonService(tco *TCOService) *ComparisonService {
	return &ComparisonService{tco: tco}
}

func ratePerKm(amount money.Cents, km float64) float64 {
	if km <= 0 {
		return 0
	}
	return amount.Float() / km
}

// evBaselineFromTCO derives the EV side from the tracked vehicle's real costs.
// Insurance is supplied separately from policy periods; financing, tolls, parking, subscriptions and
// other costs are not included, since the combustion side has no matching input.
func evBaselineFromTCO(sum *TCOSummary, annualKm float64, years int, now time.Time) (EVBaseline, []*apierror.Message) {
	basis := sum.DistanceBasisKm
	ev := EVBaseline{
		EnergyPerKm:      ratePerKm(sum.EnergyCost, basis),
		FuelPerKm:        ratePerKm(sum.FuelEnergyCost, basis),
		MaintenancePerKm: ratePerKm(sum.TiresAmortizedCost+sum.MaintenanceCost+sum.RepairCost, basis),
		TaxYearly:        annualTax(sum.TaxCost, observedMonths(sum.MonthlyCosts, now)).Float(),
		PurchaseNet:      sum.AcquisitionCost.Float(),
	}
	hybrid := models.PowertrainCanCharge(sum.Powertrain) && models.PowertrainCanRefuel(sum.Powertrain)
	var notes []*apierror.Message
	if hybrid {
		notes = append(notes, apierror.NewMessage("comparison.assumption.hybrid_actual", "Hybrid side: fuel, electricity, maintenance and depreciation from recorded costs per km; insurance and taxes from annual amounts"))
		if basis > 0 && (sum.FuelEnergyCost <= 0 || sum.EnergyCost <= sum.FuelEnergyCost) {
			notes = append(notes, apierror.NewMessage("comparison.assumption.hybrid_missing_energy_source", "This hybrid has only fuel or only charging records: its recorded energy cost is incomplete, so the comparison is understated"))
		}
	} else {
		notes = append(notes, apierror.NewMessage("comparison.assumption.ev_actual", "Electric side: energy, maintenance and depreciation from recorded costs per km; insurance and taxes from annual amounts"))
	}
	if sum.TollsCost+sum.SubscriptionCost+sum.OtherCost > 0 {
		notes = append(notes, apierror.NewMessage("comparison.assumption.costs_excluded", "Tolls, parking, subscriptions and other costs are not compared: they have no combustion-side input"))
	}

	if ev.PurchaseNet > 0 {
		dep := ratePerKm(sum.DepreciationCost, basis) * annualKm * float64(years)
		ev.ResaleValue = math.Max(ev.PurchaseNet-dep, 0)
	} else {
		notes = append(notes, apierror.NewMessage("comparison.assumption.ev_price_unknown", "Tracked vehicle purchase price unknown (lease or missing entry): depreciation not included"))
	}
	if basis <= 0 {
		notes = append(notes, apierror.NewMessage("comparison.assumption.ev_no_distance", "No tracked mileage: recorded costs cannot provide a reliable comparison"))
	}
	return ev, notes
}

// annualTax turns the recorded taxes into a yearly amount. Under twelve months of history the recorded
// total is kept as is, so a one-off tax is never multiplied up.
func annualTax(total money.Cents, months int) money.Cents {
	if total <= 0 {
		return 0
	}
	return money.Cents(math.Round(float64(total) * 12 / float64(max(months, 12))))
}

// evBaselineFromInputs builds the EV side of a PROJECTION scenario.
func evBaselineFromInputs(in *models.EVInputs, incentives money.Cents) EVBaseline {
	return EVBaseline{
		EnergyPerKm:       in.KwhPer100Km * in.EurPerKwh / 100,
		MaintenanceYearly: in.MaintenanceYearly.Float(),
		InsuranceYearly:   in.InsuranceYearly.Float(),
		TaxYearly:         in.TaxYearly.Float(),
		PurchaseNet:       (in.PurchasePrice - incentives).Float(),
		ResaleValue:       in.ResaleValue.Float(),
	}
}

// Compare evaluates a scenario. It reads the vehicle's TCO in RETROSPECTIVE mode and writes nothing.
func (s *ComparisonService) Compare(ctx context.Context, sc *models.ComparisonScenario) (*ComparisonResult, error) {
	var ev EVBaseline
	var notes []*apierror.Message

	if sc.Mode == models.ComparisonModeRetrospective {
		if sc.VehicleID == nil {
			return nil, ErrComparisonNeedsVehicle
		}
		sum, err := s.tco.ComputeVehicleTCO(ctx, *sc.VehicleID)
		if err != nil {
			return nil, err
		}
		if !models.PowertrainCanCharge(sum.Powertrain) {
			return nil, ErrComparisonNeedsEV
		}
		now := time.Now()
		ev, notes = evBaselineFromTCO(sum, sc.AnnualKm, sc.Years, now)
		annualInsurance, estimated, err := s.annualInsurance(ctx, *sc.VehicleID, now)
		if err != nil {
			return nil, err
		}
		ev.InsuranceYearly = annualInsurance.Float()
		if estimated {
			notes = append(notes, apierror.NewMessage("comparison.assumption.insurance_estimated", "Insurance payments without a coverage period: the last 12 months of payments are used as the annual estimate"))
		}
		if annualInsurance == 0 && sum.InsuranceCost > 0 {
			notes = append(notes, apierror.NewMessage("comparison.assumption.insurance_expired", "Historical insurance payments have no current coverage: reference insurance is not projected; record the current premium"))
		}
	} else {
		if sc.EV == nil {
			return nil, errors.New("comparison: projection mode requires EV inputs")
		}
		ev = evBaselineFromInputs(sc.EV, sc.Options.EVIncentives)
	}

	res := ComputeComparison(sc, ev)
	res.Assumptions = append(notes, res.Assumptions...)
	return &res, nil
}

// annualKmFromTCO extrapolates the tracked mileage to a year over the calendar months since the first
// recorded month (gaps included), or falls back to DefaultAnnualKm.
func annualKmFromTCO(sum *TCOSummary, now time.Time) (float64, bool) {
	months := observedMonths(sum.MonthlyCosts, now)
	if sum.DistanceBasisKm <= 0 || months < minMonthsForAnnualKm {
		return DefaultAnnualKm, false
	}
	return math.Round(sum.DistanceBasisKm/float64(months)*12/100) * 100, true
}

// Defaults returns the form prefill; vehicleID may be empty (PROJECTION without a tracked vehicle).
func (s *ComparisonService) Defaults(ctx context.Context, vehicleID string) (*ComparisonDefaults, error) {
	d := &ComparisonDefaults{
		AnnualKm:          DefaultAnnualKm,
		ICE:               iceDefaults,
		MaintenanceYearly: money.FromFloat(700),
		InsuranceYearly:   money.FromFloat(650),
		Source:            apierror.NewMessage("comparison.defaults_source", "Indicative values for France, to adjust"),
	}
	if vehicleID == "" {
		return d, nil
	}

	sum, err := s.tco.ComputeVehicleTCO(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	d.AnnualKm, d.AnnualKmFromData = annualKmFromTCO(sum, time.Now())
	d.Powertrain = sum.Powertrain
	if models.PowertrainIsFuelOnly(sum.Powertrain) {
		d.ICELPer100Km = sum.ConsumptionL100km
		if sum.AvgCostPerLiter > 0 {
			v := sum.AvgCostPerLiter
			d.ICEFuelPrice = &v
		}
		return d, nil
	}
	if !models.PowertrainIsElectricOnly(sum.Powertrain) {
		return d, nil
	}
	if sum.TotalKwhAdded > 0 && sum.DistanceBasisKm > 0 {
		v := round1(sum.TotalKwhAdded / sum.DistanceBasisKm * 100)
		d.EVKwhPer100Km = &v
	}
	if sum.AvgCostPerKwh > 0 {
		v := round3(sum.AvgCostPerKwh)
		d.EVEurPerKwh = &v
	}
	return d, nil
}
