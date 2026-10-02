package services

import (
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

func ptrF(v float64) *float64 { return &v }

func ptrCents(v float64) *money.Cents {
	c := money.FromFloat(v)
	return &c
}

func session(month string, hours float64, kwhAdded float64, kwhUsed *float64, cost *money.Cents) energyCharge {
	start := time.Date(2026, 1, 10, 18, 0, 0, 0, time.UTC)
	end := start.Add(time.Duration(hours * float64(time.Hour)))
	return energyCharge{Month: month, Start: start, End: &end, KwhAdded: kwhAdded, KwhUsed: kwhUsed, Cost: cost}
}

func eq(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Errorf("%s: got nil, want %v", name, want)
		return
	}
	if *got != want {
		t.Errorf("%s: got %v, want %v", name, *got, want)
	}
}

func isNil(t *testing.T, name string, got *float64) {
	t.Helper()
	if got != nil {
		t.Errorf("%s: got %v, want nil", name, *got)
	}
}

func TestClassifyCharge(t *testing.T) {
	start := time.Date(2026, 1, 10, 18, 0, 0, 0, time.UTC)
	after := func(h float64) *time.Time {
		e := start.Add(time.Duration(h * float64(time.Hour)))
		return &e
	}
	zero := start

	tests := []struct {
		name  string
		kwh   float64
		end   *time.Time
		class string
		ok    bool
	}{
		{"domestic socket 2.3 kW", 11.5, after(5), ChargeClassSlow, true},
		{"wallbox 7.4 kW", 22.2, after(3), ChargeClassAC, true},
		{"supercharger 120 kW", 60, after(0.5), ChargeClassDC, true},
		{"exactly the slow limit is AC", 3.5, after(1), ChargeClassAC, true},
		{"exactly the AC limit is DC", 25, after(1), ChargeClassDC, true},
		{"manual entry without end", 30, nil, ChargeClassUnknown, false},
		{"zero duration", 30, &zero, ChargeClassUnknown, false},
		{"no energy", 0, after(1), ChargeClassUnknown, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			class, ok := classifyCharge(tt.kwh, start, tt.end)
			if class != tt.class || ok != tt.ok {
				t.Errorf("got (%s, %v), want (%s, %v)", class, ok, tt.class, tt.ok)
			}
		})
	}
}

func TestEfficiencyRatioRejectsImplausibleReadings(t *testing.T) {
	if r, ok := efficiencyRatio(30, ptrF(33)); !ok || r < 0.909 || r > 0.910 {
		t.Errorf("typical AC session: got (%v, %v)", r, ok)
	}
	for name, used := range map[string]*float64{
		"grid energy missing":                              nil,
		"grid energy zero":                                 ptrF(0),
		"stored more than drawn":                           ptrF(25),
		"grid energy equal to energy added (not measured)": ptrF(30),
		"far more drawn than stored":                       ptrF(60),
		"just below the plausible minimum":                 ptrF(30 / 0.59),
	} {
		if _, ok := efficiencyRatio(30, used); ok {
			t.Errorf("%s: reading accepted", name)
		}
	}
}

func TestPreviousMonthsAcrossYearBoundary(t *testing.T) {
	got := previousMonths("2026-02", 3)
	want := []string{"2025-12", "2026-01", "2026-02"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestComputeEnergyStats(t *testing.T) {
	drives := []energyDriveMonth{
		{Month: "2026-01", DistanceKm: 1000, MeasuredKm: 800, Kwh: 120}, // 200 km without energy reading
		{Month: "2026-02", DistanceKm: 500, MeasuredKm: 500, Kwh: 80},
		{Month: "2026-03", DistanceKm: 40, MeasuredKm: 40, Kwh: 6}, // too short for a cost per 100 km
	}
	charges := []energyCharge{
		session("2025-12", 4, 30, nil, ptrCents(9)),      // before tracking started: cost without distance
		session("2026-01", 4, 30, ptrF(33), ptrCents(6)), // 7.5 kW: AC
		session("2026-01", 0.5, 40, nil, ptrCents(20)),   // 80 kW: DC, grid energy not reported
		session("2026-02", 5, 10, ptrF(12), ptrCents(2)), // 2 kW: socket
		session("2026-02", 3, 20, ptrF(22), nil),         // AC, no tariff known
	}

	got := computeEnergyStats(drives, charges, nil)

	if len(got.Months) != 4 {
		t.Fatalf("months: got %d, want 4 (Dec to Mar)", len(got.Months))
	}
	dec, jan, feb, mar := got.Months[0], got.Months[1], got.Months[2], got.Months[3]

	// January
	eq(t, "jan consumption", jan.ConsumptionKwh100km, 15.0) // only the 800 km with a reading count
	eq(t, "jan cost/100km", jan.CostPer100km, 2.6)
	eq(t, "jan price/kWh", jan.PricePerKwh, 0.371)
	eq(t, "jan charge efficiency", jan.ChargeEfficiency, 0.909) // the DC session has no grid reading
	eq(t, "jan trailing", jan.CostPer100kmTrailing, 2.6)        // December has no distance: left out
	if jan.EnergyCost != money.FromFloat(26) {
		t.Errorf("jan energy cost: got %v", jan.EnergyCost)
	}

	// February: only the priced session counts towards cost
	eq(t, "feb consumption", feb.ConsumptionKwh100km, 16.0)
	eq(t, "feb cost/100km", feb.CostPer100km, 0.4)
	eq(t, "feb trailing", feb.CostPer100kmTrailing, 1.87) // 28 EUR over 1500 km
	eq(t, "feb price/kWh", feb.PricePerKwh, 0.2)          // 2 EUR over the 10 priced kWh only

	// Months without enough distance give no cost per 100 km
	isNil(t, "dec cost/100km", dec.CostPer100km)
	eq(t, "dec price/kWh", dec.PricePerKwh, 0.3)
	isNil(t, "mar cost/100km", mar.CostPer100km)
	eq(t, "mar trailing", mar.CostPer100kmTrailing, 1.82) // the window still spans Jan to Mar: 28 EUR over 1540 km

	// Summary
	eq(t, "summary consumption", got.Summary.ConsumptionKwh100km, 15.4)
	eq(t, "summary cost/100km", got.Summary.CostPer100km, 1.82) // December energy excluded: no matching distance
	eq(t, "summary price/kWh", got.Summary.PricePerKwh, 0.336)
	eq(t, "summary efficiency", got.Summary.ChargeEfficiency, 0.896)
	if got.Summary.SessionsWithoutCost != 1 {
		t.Errorf("sessions without cost: got %d, want 1", got.Summary.SessionsWithoutCost)
	}

	// Classes, in a fixed order
	if len(got.ChargeClasses) != 3 {
		t.Fatalf("classes: got %+v", got.ChargeClasses)
	}
	slow, ac, dc := got.ChargeClasses[0], got.ChargeClasses[1], got.ChargeClasses[2]
	if slow.Class != ChargeClassSlow || ac.Class != ChargeClassAC || dc.Class != ChargeClassDC {
		t.Fatalf("class order: %s %s %s", slow.Class, ac.Class, dc.Class)
	}
	if ac.Sessions != 3 || ac.KwhAdded != 80 {
		t.Errorf("AC: got %d sessions / %v kWh", ac.Sessions, ac.KwhAdded)
	}
	eq(t, "AC price/kWh", ac.PricePerKwh, 0.25) // 15 EUR over the 60 priced kWh
	eq(t, "DC price/kWh", dc.PricePerKwh, 0.5)
	isNil(t, "DC efficiency", dc.ChargeEfficiency)
	eq(t, "socket efficiency", slow.ChargeEfficiency, 0.833)
}

func TestComputeEnergyStatsWithoutData(t *testing.T) {
	got := computeEnergyStats(nil, nil, nil)
	if got.Months == nil || got.ChargeClasses == nil {
		t.Fatal("empty statistics must serialise as empty lists, not null")
	}
	if len(got.Months) != 0 || len(got.ChargeClasses) != 0 {
		t.Errorf("got %+v", got)
	}
	isNil(t, "consumption", got.Summary.ConsumptionKwh100km)
	isNil(t, "cost/100km", got.Summary.CostPer100km)
}

func TestComputeEnergyStatsUnknownClassForManualCharges(t *testing.T) {
	start := time.Date(2026, 1, 10, 18, 0, 0, 0, time.UTC)
	got := computeEnergyStats(nil, []energyCharge{{Month: "2026-01", Start: start, KwhAdded: 25, Cost: ptrCents(7.5)}}, nil)
	if len(got.ChargeClasses) != 1 || got.ChargeClasses[0].Class != ChargeClassUnknown {
		t.Fatalf("got %+v", got.ChargeClasses)
	}
	eq(t, "unknown price/kWh", got.ChargeClasses[0].PricePerKwh, 0.3)
}

func soc(v int) *int { return &v }

func sessionSoc(start time.Time, kwh float64, from, to int, cost *money.Cents) energyCharge {
	end := start.Add(4 * time.Hour)
	return energyCharge{Month: start.Format("2006-01"), Start: start, End: &end, KwhAdded: kwh, Cost: cost, StartSoc: soc(from), EndSoc: soc(to)}
}

func TestUsableCapacityNeedsAWideSwingAndAPlausibleResult(t *testing.T) {
	d := time.Date(2026, 1, 10, 18, 0, 0, 0, time.UTC)
	if c, ok := usableCapacity(sessionSoc(d, 40, 20, 70, nil)); !ok || c != 80 {
		t.Errorf("40 kWh over 50 points: got (%v, %v), want 80 kWh", c, ok)
	}
	for name, c := range map[string]energyCharge{
		"swing under 30 points":    sessionSoc(d, 12, 40, 60, nil),
		"level going down":         sessionSoc(d, 40, 70, 20, nil),
		"unknown levels":           {KwhAdded: 40, Start: d},
		"implausibly small result": sessionSoc(d, 5, 10, 90, nil),
		"implausibly large result": sessionSoc(d, 200, 10, 60, nil),
	} {
		if _, ok := usableCapacity(c); ok {
			t.Errorf("%s: estimate accepted", name)
		}
	}
}

func TestCapacityEstimateUsesMedianAndMostRecentSessions(t *testing.T) {
	base := time.Date(2026, 1, 1, 18, 0, 0, 0, time.UTC)
	var charges []energyCharge
	// One session every 10 days, each a 50 point swing (20 to 70 %): kWh added = capacity / 2. The newest session
	// (90 kWh, so 180 kWh) is not a plausible capacity and is skipped, so the window covers the ten sessions
	// before it and drops the oldest one (75 kWh).
	kwhs := []float64{37.5, 37.5, 39.5, 40, 40, 40.5, 39.5, 40, 40, 40.5, 39.5, 90}
	for i, k := range kwhs {
		charges = append(charges, sessionSoc(base.AddDate(0, 0, i*10), k, 20, 70, nil))
	}
	got := computeEnergyStats(nil, charges, nil)

	// Window: 75, 79, 80, 80, 81, 79, 80, 80, 79, 81 (the newest plausible ones) => median 80.
	eq(t, "current capacity", got.Summary.EstimatedCapacityKwh, 80)
	if got.Summary.CapacitySamples != 10 {
		t.Errorf("samples: got %d, want 10", got.Summary.CapacitySamples)
	}

	// January holds the first four sessions: 75, 75, 79, 80 => median 77.
	var jan *EnergyMonth
	for i := range got.Months {
		if got.Months[i].Month == "2026-01" {
			jan = &got.Months[i]
		}
	}
	if jan == nil || jan.EstimatedCapacityKwh == nil || jan.CapacitySamples != 4 {
		t.Fatalf("january: %+v", jan)
	}
	eq(t, "jan capacity", jan.EstimatedCapacityKwh, 77)
}

func TestCostPerFullChargeGroupsByClass(t *testing.T) {
	d := time.Date(2026, 2, 1, 18, 0, 0, 0, time.UTC)
	// AC: 50 points for 10 EUR => 20 EUR per 100 points. DC: 40 points for 16 EUR => 40 EUR per 100 points.
	ac := sessionSoc(d, 40, 20, 70, ptrCents(10))
	dcEnd := d.Add(15 * time.Minute)
	dc := energyCharge{Month: "2026-02", Start: d, End: &dcEnd, KwhAdded: 32, Cost: ptrCents(16), StartSoc: soc(10), EndSoc: soc(50)}
	small := sessionSoc(d, 8, 50, 60, ptrCents(100)) // 10 points: too small a swing to price a full charge
	unpriced := sessionSoc(d, 40, 20, 70, nil)

	got := computeEnergyStats(nil, []energyCharge{ac, dc, small, unpriced}, nil)

	eq(t, "overall", got.Summary.CostPerFullCharge, 28.89) // 26 EUR over 90 points
	for _, c := range got.ChargeClasses {
		switch c.Class {
		case ChargeClassAC:
			eq(t, "AC full charge", c.CostPerFullCharge, 20)
		case ChargeClassDC:
			eq(t, "DC full charge", c.CostPerFullCharge, 40)
		}
	}
}

func TestChargesWithoutLevelsGiveNoCapacityNorFullChargeCost(t *testing.T) {
	got := computeEnergyStats(nil, []energyCharge{session("2026-01", 4, 30, ptrF(33), ptrCents(6))}, nil)
	isNil(t, "capacity", got.Summary.EstimatedCapacityKwh)
	isNil(t, "full charge", got.Summary.CostPerFullCharge)
	isNil(t, "monthly capacity", got.Months[0].EstimatedCapacityKwh)
}

func TestComputeTemperatureWinterEffect(t *testing.T) {
	price := 0.25
	raw := []tempBinRaw{
		{MinC: -5, Drives: 4, DistanceKm: 60, Kwh: 12},   // -5..0 °C: 20 kWh/100 km
		{MinC: 0, Drives: 30, DistanceKm: 500, Kwh: 95},  // 0..5 °C: 19 kWh/100 km
		{MinC: 5, Drives: 20, DistanceKm: 300, Kwh: 51},  // 5..10 °C: not part of the comparison
		{MinC: 15, Drives: 40, DistanceKm: 400, Kwh: 60}, // 15..20 °C: 15 kWh/100 km
		{MinC: 20, Drives: 10, DistanceKm: 100, Kwh: 14}, // 20..25 °C: 14 kWh/100 km
		{MinC: 30, Drives: 1, DistanceKm: 20, Kwh: 4},    // too little distance to be displayed
	}
	bins, effect := computeTemperature(raw, &price)

	if len(bins) != 5 {
		t.Fatalf("bins: got %d, want 5 (the 20 km bin is left out)", len(bins))
	}
	if bins[0].MinC != -5 || bins[0].MaxC != 0 || bins[0].ConsumptionKwh100km != 20 {
		t.Errorf("first bin: %+v", bins[0])
	}
	if bins[len(bins)-1].MinC != 20 {
		t.Errorf("bins must be ordered by temperature, last: %+v", bins[len(bins)-1])
	}
	// Cold pool: 560 km, 107 kWh => 19.1. Mild pool: 500 km, 74 kWh => 14.8.
	eq(t, "cold", effect.ColdConsumptionKwh100km, 19.1)
	eq(t, "mild", effect.MildConsumptionKwh100km, 14.8)
	eq(t, "extra percent", effect.ExtraPercent, 29.1)
	eq(t, "extra cost", effect.ExtraCostPer100km, 1.08) // 4.3 kWh more per 100 km at 0.25 EUR
}

func TestComputeTemperatureNeedsBothGroups(t *testing.T) {
	_, effect := computeTemperature([]tempBinRaw{{MinC: 0, Drives: 10, DistanceKm: 400, Kwh: 80}}, nil)
	isNil(t, "cold without mild data", effect.ColdConsumptionKwh100km)
	isNil(t, "extra percent", effect.ExtraPercent)

	// Mild weather consuming more than cold weather (unusual data) reports no extra rather than a negative one.
	_, effect = computeTemperature([]tempBinRaw{
		{MinC: 0, Drives: 10, DistanceKm: 400, Kwh: 60},
		{MinC: 15, Drives: 10, DistanceKm: 400, Kwh: 80},
	}, nil)
	eq(t, "cold", effect.ColdConsumptionKwh100km, 15)
	isNil(t, "extra percent", effect.ExtraPercent)
	isNil(t, "extra cost without a price", effect.ExtraCostPer100km)
}

func TestComputeTemperatureWithoutDrives(t *testing.T) {
	bins, _ := computeTemperature(nil, nil)
	if bins == nil || len(bins) != 0 {
		t.Errorf("empty input must give an empty, non-nil list, got %v", bins)
	}
}

func TestMonthlyOdometerDistanceSpreadsReadingsOverMonths(t *testing.T) {
	d := func(m time.Month, day int) time.Time { return time.Date(2026, m, day, 0, 0, 0, 0, time.UTC) }
	got := monthlyOdometerDistance([]odometerPoint{
		{d(time.March, 1), 1000},
		{d(time.January, 1), 0},  // out of order on purpose
		{d(time.April, 1), 1000}, // same mileage: parked
		{d(time.May, 1), 900},    // goes backwards: ignored
	}, time.UTC)
	// Jan 1 -> Mar 1 is 59 days for 1000 km, spread by day.
	if v := got["2026-01"]; v < 525 || v > 526 {
		t.Errorf("january = %v, want ~525.4", v)
	}
	if v := got["2026-02"]; v < 474 || v > 475 {
		t.Errorf("february = %v, want ~474.6", v)
	}
	if got["2026-03"] != 0 || got["2026-05"] != 0 {
		t.Errorf("no distance expected after the last increase, got %+v", got)
	}
}

func TestComputeEnergyStatsDerivesFromOdometerWithoutDrives(t *testing.T) {
	charges := []energyCharge{
		session("2026-01", 4, 30, nil, ptrCents(6)),
		session("2026-02", 4, 30, nil, ptrCents(6)),
		session("2026-03", 4, 30, nil, ptrCents(6)),
	}
	odo := map[string]float64{"2026-01": 400, "2026-02": 400, "2026-03": 400}
	got := computeEnergyStats(nil, charges, odo)

	eq(t, "derived consumption", got.Summary.ConsumptionKwh100km, 7.5) // 90 kWh over 1200 km
	eq(t, "derived cost per 100 km", got.Summary.CostPer100km, 1.5)    // 18 over 1200 km
	if got.Basis.Consumption != BasisDerived || got.Basis.Cost != BasisDerived {
		t.Errorf("basis = %+v, want derived consumption and cost", got.Basis)
	}
	if got.Basis.Charging != BasisMeasured || got.Basis.Temperature != BasisUnavailable || got.Basis.Battery != BasisUnavailable {
		t.Errorf("basis = %+v", got.Basis)
	}
}

func TestComputeEnergyStatsBasisIsMeasuredWithDrives(t *testing.T) {
	drives := []energyDriveMonth{{Month: "2026-01", DistanceKm: 1000, MeasuredKm: 1000, Kwh: 150}}
	got := computeEnergyStats(drives, []energyCharge{session("2026-01", 4, 30, nil, ptrCents(6))}, map[string]float64{"2026-01": 999})
	if got.Basis.Consumption != BasisMeasured || got.Basis.Cost != BasisMeasured {
		t.Errorf("basis = %+v, want measured", got.Basis)
	}
}

func TestComputeEnergyStatsBasisUnavailableWithoutAnything(t *testing.T) {
	got := computeEnergyStats(nil, []energyCharge{session("2026-01", 4, 30, nil, ptrCents(6))}, nil)
	if got.Basis.Consumption != BasisUnavailable || got.Basis.Cost != BasisUnavailable {
		t.Errorf("basis = %+v, want unavailable without distance", got.Basis)
	}
}
