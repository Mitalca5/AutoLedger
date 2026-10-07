package services

import (
	"math"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// TrackedBaseline is the tracked-vehicle side of a comparison, expressed as running-cost rates (year-1 prices) plus
// purchase and resale amounts. Every recurring line is perKm*km + yearly so that it can be filled either
// from the tracked vehicle's real per-km costs (RETROSPECTIVE) or from user-entered yearly figures (PROJECTION).
type TrackedBaseline struct {
	EnergyPerKm       float64 // EUR per km
	FuelPerKm         float64 // Fuel portion of EnergyPerKm; the rest is electricity.
	MaintenancePerKm  float64
	MaintenanceYearly float64
	InsuranceYearly   float64
	TaxYearly         float64
	PurchaseNet       float64 // Cash outlay at the start, net of incentives
	ResaleValue       float64 // Value at the end of the period
}

// YearCost is the cost of one year of ownership.
type YearCost struct {
	Year         int         `json:"year"`
	Energy       money.Cents `json:"energy"`
	Maintenance  money.Cents `json:"maintenance"`
	Insurance    money.Cents `json:"insurance"`
	Tax          money.Cents `json:"tax"`
	Depreciation money.Cents `json:"depreciation"`
	Total        money.Cents `json:"total"`
}

// CostSide is the total cost of one vehicle over the comparison period.
type CostSide struct {
	Energy       money.Cents `json:"energy"`
	Maintenance  money.Cents `json:"maintenance"`
	Insurance    money.Cents `json:"insurance"`
	Tax          money.Cents `json:"tax"`
	Depreciation money.Cents `json:"depreciation"`
	Total        money.Cents `json:"total"`
	PerYear      money.Cents `json:"per_year"`
	PerMonth     money.Cents `json:"per_month"`
	CostPerKm    float64     `json:"cost_per_km"`
	Years        []YearCost  `json:"years"`
}

// CumulativePoint is the cumulated cash outlay of both vehicles after a number of years (0 = purchase).
// Resale is not deducted: the curve shows when the running-cost savings repay the purchase difference.
type CumulativePoint struct {
	Year    int         `json:"year"`
	Tracked money.Cents `json:"tracked"`
	ICE     money.Cents `json:"ice"`
}

// SensitivityRow is the tracked-vehicle savings when one assumption varies.
type SensitivityRow struct {
	Label          *apierror.Message `json:"label"`
	TrackedSavings money.Cents       `json:"tracked_savings"`
	DeltaShift     money.Cents       `json:"delta_shift"` // Difference with the base savings
}

// ComparisonResult is the outcome of a comparison. TrackedSavings > 0 means the tracked vehicle is cheaper.
type ComparisonResult struct {
	Mode           string              `json:"mode"`
	AnnualKm       float64             `json:"annual_km"`
	YearsCount     int                 `json:"years_count"`
	Tracked        CostSide            `json:"tracked"`
	ICE            CostSide            `json:"ice"`
	TrackedSavings money.Cents         `json:"tracked_savings"`
	BreakEvenYear  *float64            `json:"break_even_year,omitempty"` // Years until cumulated tracked-vehicle cost drops below ICE
	Cumulative     []CumulativePoint   `json:"cumulative"`
	Sensitivity    []SensitivityRow    `json:"sensitivity"`
	Assumptions    []*apierror.Message `json:"assumptions"`
}

// sideRates is the internal, unit-free description of one vehicle.
type sideRates struct {
	energyPerKm, maintPerKm, maintYearly, insYearly, taxYearly float64
	purchase, resale                                           float64
	energyInfl, costInfl                                       float64
	fuelPerKm, fuelInfl                                        float64
}

func iceRates(sc *models.ComparisonScenario) sideRates {
	return sideRates{
		energyPerKm: sc.ICE.LPer100Km * sc.ICE.FuelPrice / 100,
		maintYearly: sc.ICE.MaintenanceYearly.Float(),
		insYearly:   sc.ICE.InsuranceYearly.Float(),
		taxYearly:   sc.ICE.TaxYearly.Float(),
		purchase:    sc.ICE.PurchasePrice.Float(),
		resale:      sc.ICE.ResaleValue.Float(),
		energyInfl:  sc.Options.FuelInflationPct / 100,
		costInfl:    sc.Options.CostInflationPct / 100,
	}
}

func evRates(sc *models.ComparisonScenario, ev TrackedBaseline) sideRates {
	return sideRates{
		energyPerKm: ev.EnergyPerKm - ev.FuelPerKm,
		fuelPerKm:   ev.FuelPerKm,
		fuelInfl:    sc.Options.FuelInflationPct / 100,
		maintPerKm:  ev.MaintenancePerKm,
		maintYearly: ev.MaintenanceYearly,
		insYearly:   ev.InsuranceYearly,
		taxYearly:   ev.TaxYearly,
		purchase:    ev.PurchaseNet,
		resale:      ev.ResaleValue,
		energyInfl:  sc.Options.ElectricityInflationPct / 100,
		costInfl:    sc.Options.CostInflationPct / 100,
	}
}

// run computes the yearly costs and the cumulated cash outlay (index 0 = purchase) of one vehicle.
func (s sideRates) run(km float64, years int) (CostSide, []float64) {
	var side CostSide
	var energy, maint, ins, tax, dep float64

	depPerYear := math.Max(s.purchase-s.resale, 0) / float64(years)
	cash := make([]float64, years+1)
	cash[0] = s.purchase

	for y := 1; y <= years; y++ {
		energyY := s.energyPerKm * km * math.Pow(1+s.energyInfl, float64(y-1))
		energyY += s.fuelPerKm * km * math.Pow(1+s.fuelInfl, float64(y-1))
		costFactor := math.Pow(1+s.costInfl, float64(y-1))
		maintY := (s.maintPerKm*km + s.maintYearly) * costFactor
		insY := s.insYearly * costFactor
		taxY := s.taxYearly * costFactor

		energy += energyY
		maint += maintY
		ins += insY
		tax += taxY
		dep += depPerYear
		cash[y] = cash[y-1] + energyY + maintY + insY + taxY

		side.Years = append(side.Years, YearCost{
			Year:         y,
			Energy:       money.FromFloat(energyY),
			Maintenance:  money.FromFloat(maintY),
			Insurance:    money.FromFloat(insY),
			Tax:          money.FromFloat(taxY),
			Depreciation: money.FromFloat(depPerYear),
			Total:        money.FromFloat(energyY + maintY + insY + taxY + depPerYear),
		})
	}

	total := energy + maint + ins + tax + dep
	side.Energy = money.FromFloat(energy)
	side.Maintenance = money.FromFloat(maint)
	side.Insurance = money.FromFloat(ins)
	side.Tax = money.FromFloat(tax)
	side.Depreciation = money.FromFloat(dep)
	side.Total = money.FromFloat(total)
	side.PerYear = money.FromFloat(total / float64(years))
	side.PerMonth = money.FromFloat(total / float64(years) / 12)
	if km > 0 {
		side.CostPerKm = round3(total / (km * float64(years)))
	}
	return side, cash
}

// breakEven returns the number of years after which the cumulated ICE outlay exceeds the tracked-vehicle one.
// It is 0 when the tracked vehicle is never more expensive within the period, nil when the tracked vehicle does not pay back.
func breakEven(evCash, iceCash []float64) *float64 {
	diff := func(i int) float64 { return iceCash[i] - evCash[i] }
	if diff(0) >= 0 {
		for i := range evCash {
			if diff(i) < 0 {
				return nil
			}
		}
		zero := 0.0
		return &zero
	}
	for i := 1; i < len(evCash); i++ {
		if diff(i) >= 0 {
			frac := -diff(i-1) / (diff(i) - diff(i-1))
			v := math.Round((float64(i-1)+frac)*10) / 10
			return &v
		}
	}
	return nil
}

// ComputeComparison compares the tracked-vehicle baseline with the equivalent ICE of the scenario over its period.
// It is a pure function: no I/O, informational only.
func ComputeComparison(sc *models.ComparisonScenario, ev TrackedBaseline) ComparisonResult {
	res := computeCore(sc, ev, sc.AnnualKm, 1, 1)

	base := res.TrackedSavings
	variants := []struct {
		label                 *apierror.Message
		km, fuel, electricity float64
	}{
		{apierror.NewMessage("comparison.sensitivity.fuel_down", "Fuel −20%"), sc.AnnualKm, 0.8, 1},
		{apierror.NewMessage("comparison.sensitivity.fuel_up", "Fuel +20%"), sc.AnnualKm, 1.2, 1},
		{apierror.NewMessage("comparison.sensitivity.electricity_down", "Electricity −20%"), sc.AnnualKm, 1, 0.8},
		{apierror.NewMessage("comparison.sensitivity.electricity_up", "Electricity +20%"), sc.AnnualKm, 1, 1.2},
		{apierror.NewMessage("comparison.sensitivity.km_down", "Mileage −20%"), sc.AnnualKm * 0.8, 1, 1},
		{apierror.NewMessage("comparison.sensitivity.km_up", "Mileage +20%"), sc.AnnualKm * 1.2, 1, 1},
	}
	for _, v := range variants {
		savings := computeCore(sc, ev, v.km, v.fuel, v.electricity).TrackedSavings
		res.Sensitivity = append(res.Sensitivity, SensitivityRow{Label: v.label, TrackedSavings: savings, DeltaShift: savings - base})
	}
	return res
}

func computeCore(sc *models.ComparisonScenario, ev TrackedBaseline, km, fuelFactor, electricityFactor float64) ComparisonResult {
	years := sc.Years
	ice := iceRates(sc)
	ice.energyPerKm *= fuelFactor

	tracked := evRates(sc, ev)
	tracked.energyPerKm *= electricityFactor
	tracked.fuelPerKm *= fuelFactor
	evSide, evCash := tracked.run(km, years)
	iceSide, iceCash := ice.run(km, years)

	res := ComparisonResult{
		Mode:           sc.Mode,
		AnnualKm:       km,
		YearsCount:     years,
		Tracked:        evSide,
		ICE:            iceSide,
		TrackedSavings: iceSide.Total - evSide.Total,
		BreakEvenYear:  breakEven(evCash, iceCash),
		Assumptions:    comparisonAssumptions(sc),
	}
	for i := range evCash {
		res.Cumulative = append(res.Cumulative, CumulativePoint{Year: i, Tracked: money.FromFloat(evCash[i]), ICE: money.FromFloat(iceCash[i])})
	}
	return res
}

func comparisonAssumptions(sc *models.ComparisonScenario) []*apierror.Message {
	a := []*apierror.Message{
		apierror.NewMessagef("comparison.assumption.usage", "%.0f km/year for %d year(s), same usage for both vehicles", apierror.Km(sc.AnnualKm), sc.Years),
		apierror.NewMessage("comparison.assumption.depreciation", "Depreciation = (purchase price − resale) spread linearly over the period"),
		apierror.NewMessage("comparison.assumption.no_financing", "Financing, loans and leases not included"),
		apierror.NewMessage("comparison.assumption.break_even", "The break-even compares cumulative outlays (purchase + running costs), without resale"),
	}
	if sc.Options.FuelInflationPct != 0 || sc.Options.ElectricityInflationPct != 0 || sc.Options.CostInflationPct != 0 {
		a = append(a, apierror.NewMessagef("comparison.assumption.inflation", "Yearly inflation: fuel %.1f%%, electricity %.1f%%, maintenance/insurance/taxes %.1f%%",
			sc.Options.FuelInflationPct, sc.Options.ElectricityInflationPct, sc.Options.CostInflationPct))
	} else {
		a = append(a, apierror.NewMessage("comparison.assumption.constant_prices", "Constant prices (no inflation applied)"))
	}
	return a
}
