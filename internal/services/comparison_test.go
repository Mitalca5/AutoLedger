package services

import (
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func eur(v float64) money.Cents { return money.FromFloat(v) }

func baseScenario() *models.ComparisonScenario {
	return &models.ComparisonScenario{
		Mode:     models.ComparisonModeProjection,
		AnnualKm: 10000,
		Years:    5,
		ICE: models.ICEInputs{
			FuelType: "SP95_E10", LPer100Km: 6, FuelPrice: 1.5,
			PurchasePrice: eur(25000), ResaleValue: eur(10000),
			MaintenanceYearly: eur(600), InsuranceYearly: eur(700), TaxYearly: eur(0),
		},
	}
}

func baseEV() TrackedBaseline {
	return TrackedBaseline{
		EnergyPerKm:       15.0 * 0.20 / 100, // 15 kWh/100 km at 0.20 EUR/kWh
		MaintenanceYearly: 300,
		InsuranceYearly:   800,
		PurchaseNet:       35000,
		ResaleValue:       15000,
	}
}

func TestComputeComparisonTotals(t *testing.T) {
	res := ComputeComparison(baseScenario(), baseEV())

	// ICE: energy 10000*6*1.5/100 = 900/yr; maintenance 600; insurance 700; depreciation (25000-10000)/5 = 3000/yr
	if res.ICE.Energy != eur(4500) {
		t.Errorf("ICE energy = %v, want 4500", res.ICE.Energy)
	}
	if res.ICE.Depreciation != eur(15000) {
		t.Errorf("ICE depreciation = %v, want 15000", res.ICE.Depreciation)
	}
	if res.ICE.Total != eur(4500+3000+3500+15000) {
		t.Errorf("ICE total = %v, want 26000", res.ICE.Total)
	}
	// Tracked: energy 10000*0.03 = 300/yr; depreciation (35000-15000)/5 = 4000/yr
	if res.Tracked.Energy != eur(1500) {
		t.Errorf("EV energy = %v, want 1500", res.Tracked.Energy)
	}
	if res.Tracked.Total != eur(1500+1500+4000+20000) {
		t.Errorf("EV total = %v, want 27000", res.Tracked.Total)
	}
	if res.TrackedSavings != res.ICE.Total-res.Tracked.Total || res.TrackedSavings != eur(-1000) {
		t.Errorf("EV savings = %v, want -1000", res.TrackedSavings)
	}
	if res.ICE.PerYear != eur(5200) || res.ICE.PerMonth != money.FromFloat(5200.0/12) {
		t.Errorf("ICE per year/month = %v/%v", res.ICE.PerYear, res.ICE.PerMonth)
	}
	if res.ICE.CostPerKm != 0.52 {
		t.Errorf("ICE cost per km = %v, want 0.52", res.ICE.CostPerKm)
	}
	if len(res.ICE.Years) != 5 || len(res.Cumulative) != 6 {
		t.Errorf("years = %d, cumulative points = %d", len(res.ICE.Years), len(res.Cumulative))
	}
	if len(res.Sensitivity) != 6 || len(res.Assumptions) == 0 {
		t.Errorf("sensitivity = %d, assumptions = %d", len(res.Sensitivity), len(res.Assumptions))
	}
}

func TestComputeComparisonBreakEven(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(sc *models.ComparisonScenario, ev *TrackedBaseline)
		wantNil  bool
		wantYear float64
	}{
		{
			name: "EV dearer to buy and to run never pays back",
			mutate: func(sc *models.ComparisonScenario, ev *TrackedBaseline) {
				sc.Years = 10
			},
			// Purchase gap 10000; yearly running: ICE 2200, EV 1100+... insurance makes the EV dearer to run
			wantNil: true,
		},
		{
			name: "cheap EV running costs repay the purchase gap",
			mutate: func(sc *models.ComparisonScenario, ev *TrackedBaseline) {
				sc.Years = 10
				ev.InsuranceYearly = 0
				ev.MaintenanceYearly = 0
				sc.ICE.FuelPrice = 2.0 // ICE running 1200+600+700 = 2500/yr; EV 300/yr -> 2200/yr saved
			},
			wantYear: 4.5, // gap 10000 / 2200 = 4.545 -> rounded to 4.5
		},
		{
			name: "EV cheaper at purchase and cheaper to run",
			mutate: func(sc *models.ComparisonScenario, ev *TrackedBaseline) {
				ev.PurchaseNet = 20000
				ev.InsuranceYearly = 0
				ev.MaintenanceYearly = 0
			},
			wantYear: 0,
		},
		{
			name: "EV cheaper at purchase but dearer to run",
			mutate: func(sc *models.ComparisonScenario, ev *TrackedBaseline) {
				ev.PurchaseNet = 20000
				ev.InsuranceYearly = 5000
			},
			wantNil: true,
		},
		{
			name: "payback beyond the period",
			mutate: func(sc *models.ComparisonScenario, ev *TrackedBaseline) {
				sc.Years = 2
				ev.InsuranceYearly = 0
				ev.MaintenanceYearly = 0
			},
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc, ev := baseScenario(), baseEV()
			tt.mutate(sc, &ev)
			res := ComputeComparison(sc, ev)
			if tt.wantNil {
				if res.BreakEvenYear != nil {
					t.Fatalf("break-even = %v, want none", *res.BreakEvenYear)
				}
				return
			}
			if res.BreakEvenYear == nil || *res.BreakEvenYear != tt.wantYear {
				t.Fatalf("break-even = %v, want %v", res.BreakEvenYear, tt.wantYear)
			}
		})
	}
}

func TestComputeComparisonInflationAndEdges(t *testing.T) {
	sc := baseScenario()
	sc.Years = 2
	sc.Options.FuelInflationPct = 10
	res := ComputeComparison(sc, baseEV())
	// ICE energy: 900 then 990
	if res.ICE.Energy != eur(1890) {
		t.Errorf("inflated ICE energy = %v, want 1890", res.ICE.Energy)
	}
	if res.Tracked.Energy != eur(600) {
		t.Errorf("EV energy must ignore fuel inflation, got %v", res.Tracked.Energy)
	}

	// Resale above purchase never yields negative depreciation.
	sc2 := baseScenario()
	sc2.ICE.ResaleValue = eur(30000)
	if res2 := ComputeComparison(sc2, baseEV()); res2.ICE.Depreciation != 0 {
		t.Errorf("depreciation = %v, want 0 when resale exceeds purchase", res2.ICE.Depreciation)
	}
}

func TestComputeComparisonSensitivity(t *testing.T) {
	res := ComputeComparison(baseScenario(), baseEV())
	byLabel := map[string]SensitivityRow{}
	for _, r := range res.Sensitivity {
		byLabel[r.Label.Code] = r
	}
	// More expensive fuel makes the EV relatively better.
	if byLabel["comparison.sensitivity.fuel_up"].DeltaShift <= 0 || byLabel["comparison.sensitivity.fuel_down"].DeltaShift >= 0 {
		t.Errorf("unexpected fuel sensitivity: %+v", res.Sensitivity)
	}
	// A dearer electricity bill hurts the EV; a cheaper one helps it.
	if byLabel["comparison.sensitivity.electricity_up"].DeltaShift >= 0 || byLabel["comparison.sensitivity.electricity_down"].DeltaShift <= 0 {
		t.Errorf("unexpected electricity sensitivity: %+v", res.Sensitivity)
	}
	// Driving more favours the EV here (lower per-km energy).
	if byLabel["comparison.sensitivity.km_up"].DeltaShift <= 0 {
		t.Errorf("unexpected mileage sensitivity: %+v", res.Sensitivity)
	}
	if res.AnnualKm != 10000 {
		t.Errorf("base result must keep the scenario mileage, got %v", res.AnnualKm)
	}
}

func TestTrackedBaselineFromTCO(t *testing.T) {
	sum := &TCOSummary{
		DistanceBasisKm:    10000,
		EnergyCost:         eur(500),
		TiresAmortizedCost: eur(200),
		MaintenanceCost:    eur(100),
		RepairCost:         eur(50),
		InsuranceCost:      eur(1000),
		AcquisitionCost:    eur(30000),
		DepreciationCost:   eur(5000),
	}
	ev, notes := trackedBaselineFromTCO(sum, 10000, 4, time.Now())
	if ev.EnergyPerKm != 0.05 || ev.InsuranceYearly != 0 || ev.MaintenancePerKm != 0.035 {
		t.Errorf("unexpected rates: %+v", ev)
	}
	// depreciation 0.5 EUR/km * 10000 km * 4 years = 20000 -> resale 10000
	if ev.PurchaseNet != 30000 || ev.ResaleValue != 10000 {
		t.Errorf("purchase/resale = %v/%v", ev.PurchaseNet, ev.ResaleValue)
	}
	if len(notes) != 2 || notes[1].Code != "comparison.assumption.purchase_basis" {
		t.Errorf("want the actor note then the purchase basis note, got %v", notes)
	}

	// Unknown purchase price (lease) and no tracked distance are flagged.
	ev2, notes2 := trackedBaselineFromTCO(&TCOSummary{}, 10000, 4, time.Now())
	if ev2.PurchaseNet != 0 || ev2.ResaleValue != 0 || len(notes2) != 3 {
		t.Errorf("unexpected fallback: %+v %v", ev2, notes2)
	}

	// Depreciation exceeding the purchase price floors the resale at zero.
	sum.DepreciationCost = eur(20000)
	if ev3, _ := trackedBaselineFromTCO(sum, 10000, 4, time.Now()); ev3.ResaleValue != 0 {
		t.Errorf("resale = %v, want 0", ev3.ResaleValue)
	}
}

func TestTrackedBaselineFromInputsAndAnnualKm(t *testing.T) {
	ev := trackedBaselineFromInputs(&models.TrackedInputs{
		KwhPer100Km: 16, EurPerKwh: 0.25,
		PurchasePrice: eur(40000), ResaleValue: eur(18000),
		MaintenanceYearly: eur(250), InsuranceYearly: eur(900), TaxYearly: eur(0),
	}, eur(4000))
	if ev.EnergyPerKm != 0.04 || ev.PurchaseNet != 36000 || ev.ResaleValue != 18000 || ev.InsuranceYearly != 900 {
		t.Errorf("unexpected baseline: %+v", ev)
	}

	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	since := func(month string) []MonthlyCost { return []MonthlyCost{{Month: month}, {Month: "2026-06"}} }
	// Two months carry data but they span six calendar months: the gap counts in the pace.
	if km, ok := annualKmFromTCO(&TCOSummary{DistanceBasisKm: 6000, MonthlyCosts: since("2026-01")}, now); !ok || km != 12000 {
		t.Errorf("annual km = %v (%v), want 12000 from data", km, ok)
	}
	if km, ok := annualKmFromTCO(&TCOSummary{DistanceBasisKm: 6000, MonthlyCosts: since("2026-05")}, now); ok || km != DefaultAnnualKm {
		t.Errorf("short history should fall back, got %v (%v)", km, ok)
	}
	if km, ok := annualKmFromTCO(&TCOSummary{MonthlyCosts: since("2026-01")}, now); ok || km != DefaultAnnualKm {
		t.Errorf("no distance should fall back, got %v (%v)", km, ok)
	}
}

func TestAnnualTaxKeepsShortHistoryAndAveragesLongOne(t *testing.T) {
	for _, c := range []struct {
		total  money.Cents
		months int
		want   money.Cents
	}{
		{eur(0), 24, 0},
		{eur(120), 1, eur(120)},
		{eur(120), 12, eur(120)},
		{eur(300), 24, eur(150)},
	} {
		if got := annualTax(c.total, c.months); got != c.want {
			t.Errorf("annualTax(%v, %d) = %v, want %v", c.total, c.months, got, c.want)
		}
	}
}

func TestHybridComparisonInflationAndFuelSensitivity(t *testing.T) {
	sc := baseScenario()
	sc.Years = 2
	sc.Options.FuelInflationPct = 10
	sc.Options.ElectricityInflationPct = 20
	// 10,000 km/year: fuel 400/year, electricity 200/year.
	ev := TrackedBaseline{EnergyPerKm: 0.06, FuelPerKm: 0.04}
	res := ComputeComparison(sc, ev)
	if res.Tracked.Energy != eur(1280) {
		t.Fatalf("hybrid energy = %v, want 400+440+200+240", res.Tracked.Energy)
	}
	for _, row := range res.Sensitivity {
		// Fuel-price changes affect both the conventional vehicle and the hybrid.
		if row.Label.Code == "comparison.sensitivity.fuel_up" && row.DeltaShift != eur(210) {
			t.Errorf("fuel-up shift = %v, want 210", row.DeltaShift)
		}
		if row.Label.Code == "comparison.sensitivity.fuel_down" && row.DeltaShift != eur(-210) {
			t.Errorf("fuel-down shift = %v, want -210", row.DeltaShift)
		}
		if row.Label.Code == "comparison.sensitivity.electricity_up" && row.DeltaShift != eur(-88) {
			t.Errorf("electricity-up shift = %v, want -88", row.DeltaShift)
		}
		if row.Label.Code == "comparison.sensitivity.electricity_down" && row.DeltaShift != eur(88) {
			t.Errorf("electricity-down shift = %v, want 88", row.DeltaShift)
		}
	}
}

func TestTrackedBaselineFromTCOIncludesRecordedTaxAndFlagsExcludedCosts(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	sum := &TCOSummary{
		DistanceBasisKm: 1000,
		TaxCost:         eur(240),
		TollsCost:       eur(30),
		MonthlyCosts:    []MonthlyCost{{Month: "2024-07"}, {Month: "2026-06"}},
	}
	ev, notes := trackedBaselineFromTCO(sum, 10000, 3, now)
	if ev.TaxYearly != 120 {
		t.Errorf("tax yearly = %v, want 120 (240 over 24 months)", ev.TaxYearly)
	}
	var excluded bool
	for _, n := range notes {
		if n.Code == "comparison.assumption.costs_excluded" {
			excluded = true
		}
	}
	if !excluded {
		t.Error("tolls recorded but no costs_excluded assumption")
	}
	_, notes = trackedBaselineFromTCO(&TCOSummary{DistanceBasisKm: 1000, AcquisitionCost: eur(30000)}, 10000, 3, now)
	for _, n := range notes {
		if n.Code == "comparison.assumption.costs_excluded" {
			t.Error("costs_excluded flagged without any excluded cost")
		}
	}
}

func TestHybridBaselineUsesRecordedCosts(t *testing.T) {
	for _, powertrain := range []string{models.PowertrainPHEV, models.PowertrainREEV} {
		t.Run(powertrain, func(t *testing.T) {
			sum := &TCOSummary{Powertrain: powertrain, DistanceBasisKm: 1000, EnergyCost: eur(90), FuelEnergyCost: eur(60)}
			baseline, _ := trackedBaselineFromTCO(sum, 10000, 2, time.Now())
			if baseline.EnergyPerKm != 0.09 || baseline.FuelPerKm != 0.06 {
				t.Fatalf("unexpected baseline: %+v", baseline)
			}
			sc := baseScenario()
			sc.Years = 1
			if result := ComputeComparison(sc, baseline); result.Tracked.Energy != eur(900) {
				t.Errorf("energy = %v, want 900", result.Tracked.Energy)
			}
		})
	}
}

func TestHybridWithOneEnergySourceIsFlagged(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	has := func(notes []*apierror.Message, code string) bool {
		for _, n := range notes {
			if n.Code == code {
				return true
			}
		}
		return false
	}
	for name, c := range map[string]struct {
		powertrain string
		energy     money.Cents
		fuel       money.Cents
		fillUps    int
		charges    int
		want       bool
	}{
		"fuel only":        {models.PowertrainPHEV, eur(60), eur(60), 1, 0, true},
		"charging only":    {models.PowertrainREEV, eur(30), 0, 0, 1, true},
		"both recorded":    {models.PowertrainPHEV, eur(90), eur(60), 1, 1, false},
		"free charging":    {models.PowertrainPHEV, eur(60), eur(60), 1, 1, false},
		"both free":        {models.PowertrainREEV, 0, 0, 1, 1, false},
		"no records":       {models.PowertrainPHEV, 0, 0, 0, 0, true},
		"electric ignores": {models.PowertrainEV, eur(30), 0, 0, 1, false},
	} {
		sum := &TCOSummary{Powertrain: c.powertrain, DistanceBasisKm: 1000, EnergyCost: c.energy, FuelEnergyCost: c.fuel, FuelFillUps: c.fillUps, RecordedChargeCount: c.charges}
		_, notes := trackedBaselineFromTCO(sum, 10000, 2, now)
		if got := has(notes, "comparison.assumption.hybrid_missing_energy_source"); got != c.want {
			t.Errorf("%s: missing-source flag = %v, want %v", name, got, c.want)
		}
		wording := "comparison.assumption.ev_actual"
		if models.PowertrainCanRefuel(c.powertrain) {
			wording = "comparison.assumption.hybrid_actual"
		}
		if !has(notes, wording) {
			t.Errorf("%s: missing %s", name, wording)
		}
	}
}

func TestDefaultsForCurrency(t *testing.T) {
	measured, lper100 := 1.9, 6.2
	newDefaults := func() *ComparisonDefaults {
		kwh := 0.2
		return &ComparisonDefaults{
			ICE:               iceDefaults,
			MaintenanceYearly: money.FromFloat(700),
			InsuranceYearly:   money.FromFloat(650),
			TrackedEurPerKwh:  &kwh,
			ICEFuelPrice:      &measured,
			ICELPer100Km:      &lper100,
			IndicativePrices:  true,
		}
	}

	for _, currency := range []string{"", "EUR", "eur"} {
		d := newDefaults()
		d.ForCurrency(currency)
		if !d.IndicativePrices || d.MaintenanceYearly != money.FromFloat(700) || d.ICE[0].FuelPrice != 1.75 || d.TrackedEurPerKwh == nil {
			t.Errorf("currency %q: indicative prices must stay", currency)
		}
	}

	d := newDefaults()
	d.ForCurrency("USD")
	if d.IndicativePrices || d.MaintenanceYearly != 0 || d.InsuranceYearly != 0 || d.TrackedEurPerKwh != nil {
		t.Errorf("prices must be dropped for USD: %+v", d)
	}
	for _, f := range d.ICE {
		if f.FuelPrice != 0 || f.LPer100Km <= 0 {
			t.Errorf("fuel price must be dropped and consumption kept: %+v", f)
		}
	}
	if iceDefaults[0].FuelPrice != 1.75 {
		t.Error("the shared catalog must not be modified")
	}
	if d.ICELPer100Km == nil || *d.ICELPer100Km != 6.2 || d.ICEFuelPrice == nil || *d.ICEFuelPrice != 1.9 {
		t.Error("figures measured on the vehicle must stay")
	}
	if d.Source == nil || d.Source.Code != "comparison.defaults_source_no_prices" {
		t.Errorf("unexpected source: %+v", d.Source)
	}
}

func TestBreakEvenNet(t *testing.T) {
	years := func(v float64) *float64 { return &v }
	tests := []struct {
		name          string
		evNet, iceNet []float64
		want          *float64
	}{
		{"tracked costlier at first then cheaper", []float64{0, 5, 8, 10}, []float64{0, 2, 7, 13}, years(2.3)},
		{"tracked never costlier to own", []float64{0, 1, 2}, []float64{0, 2, 4}, years(0)},
		{"tracked never repays", []float64{0, 5, 10}, []float64{0, 2, 4}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := breakEvenNet(tt.evNet, tt.iceNet)
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Fatalf("breakEvenNet = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeComparisonBreakEvenCountsResale(t *testing.T) {
	sc := &models.ComparisonScenario{
		Mode: models.ComparisonModeProjection, AnnualKm: 10000, Years: 10,
		ICE: models.ICEInputs{LPer100Km: 6, FuelPrice: 2, PurchasePrice: money.FromFloat(20000), ResaleValue: money.FromFloat(2000)},
	}
	// The tracked vehicle costs 10000 more but keeps 12000 more value: its outlay never pays back within
	// the period yet it is cheaper to own once resale is counted.
	ev := TrackedBaseline{EnergyPerKm: 0.11, PurchaseNet: 30000, ResaleValue: 14000}
	res := ComputeComparison(sc, ev)
	if res.BreakEvenNetYear == nil {
		t.Fatalf("net break-even missing, outlay break-even = %v", res.BreakEvenYear)
	}
	if res.BreakEvenYear != nil && *res.BreakEvenNetYear >= *res.BreakEvenYear {
		t.Fatalf("net break-even %v should come before outlay break-even %v", *res.BreakEvenNetYear, *res.BreakEvenYear)
	}
}
