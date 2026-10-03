package services

import (
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestCalculateSessionCostFlat(t *testing.T) {
	svc := NewTariffService()
	flatRate := money.Cents(25) // 0.25 EUR / kWh
	plan := &models.TariffPlan{
		PlanType:      models.TariffTypeFlat,
		FlatRateCents: &flatRate,
	}

	start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	cost, err := svc.CalculateSessionCost(plan, start, end, 20.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 20 kWh * 25 cents = 500 cents (5.00 EUR)
	if cost != 500 {
		t.Errorf("expected 500 cents, got %d", cost)
	}
}

func TestCalculateSessionCostTimeOfUse(t *testing.T) {
	svc := NewTariffService()
	peakRate := money.Cents(30)    // 0.30 EUR / kWh
	offpeakRate := money.Cents(15) // 0.15 EUR / kWh

	plan := &models.TariffPlan{
		PlanType:         models.TariffTypeTimeOfUse,
		PeakRateCents:    &peakRate,
		OffpeakRateCents: &offpeakRate,
		TimeWindows: []models.TimeWindow{
			{Start: "22:00", End: "06:00", Kind: models.TimeWindowOffPeak},
		},
	}

	// 1. Fully within off-peak (23:00 to 02:00 = 3h)
	start1 := time.Date(2026, 10, 1, 23, 0, 0, 0, time.UTC)
	end1 := time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC)
	cost1, err := svc.CalculateSessionCost(plan, start1, end1, 10.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 10 kWh * 15 cents = 150 cents
	if cost1 != 150 {
		t.Errorf("expected 150 cents for full offpeak, got %d", cost1)
	}

	// 2. Straddling boundary: 21:00 to 23:00 (1h peak 21-22, 1h offpeak 22-23)
	start2 := time.Date(2026, 10, 1, 21, 0, 0, 0, time.UTC)
	end2 := time.Date(2026, 10, 1, 23, 0, 0, 0, time.UTC)
	cost2, err := svc.CalculateSessionCost(plan, start2, end2, 20.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 10 kWh * 30 + 10 kWh * 15 = 300 + 150 = 450 cents
	if cost2 != 450 {
		t.Errorf("expected 450 cents for split peak/offpeak, got %d", cost2)
	}
}

func TestCalculatePublicCharging(t *testing.T) {
	svc := NewTariffService()

	req := models.PublicChargingCalculationRequest{
		Kwh:                 30.0,
		ChargingMinutes:     45,
		TotalPluggedMinutes: 60,               // 15 min idle
		ConnectionFee:       money.Cents(100), // 1.00 EUR
		PricePerKwh:         money.Cents(40),  // 0.40 EUR / kWh -> 1200 cents
		PricePerMinute:      money.Cents(10),  // 0.10 EUR / min -> 45 * 10 = 450 cents
		IdleFeePerMinute:    money.Cents(50),  // 0.50 EUR / min
		IdleGraceMinutes:    5,                // 15 - 5 = 10 min billed -> 500 cents
	}

	bd := svc.CalculatePublicCharging(req)
	if bd.ConnectionCost != 100 {
		t.Errorf("expected connection fee 100, got %d", bd.ConnectionCost)
	}
	if bd.EnergyCost != 1200 {
		t.Errorf("expected energy cost 1200, got %d", bd.EnergyCost)
	}
	if bd.DurationCost != 450 {
		t.Errorf("expected duration cost 450, got %d", bd.DurationCost)
	}
	if bd.IdleMinutes != 15 {
		t.Errorf("expected 15 idle minutes, got %d", bd.IdleMinutes)
	}
	if bd.IdleCost != 500 {
		t.Errorf("expected 500 idle cost, got %d", bd.IdleCost)
	}
	// Total = 100 + 1200 + 450 + 500 = 2250 cents
	if bd.TotalCost != 2250 {
		t.Errorf("expected total cost 2250, got %d", bd.TotalCost)
	}
}

func bandsPlan() *models.TariffPlan {
	return &models.TariffPlan{
		PlanType:    models.TariffTypeBands,
		DefaultBand: "white",
		Bands: []models.TariffBand{
			{Name: "blue", RateCents: 10},
			{Name: "white", RateCents: 20},
			{Name: "red", RateCents: 50},
		},
		Rules: []models.TariffRule{
			{Days: []int{1, 2, 3, 4, 5}, Start: "22:00", End: "06:00", Band: "blue"},
			{Days: []int{0, 6}, Start: "00:00", End: "24:00", Band: "blue"},
			{Days: []int{1, 2, 3, 4, 5}, Start: "18:00", End: "20:00", Band: "red"},
		},
	}
}

func TestBandsThreeColours(t *testing.T) {
	svc := NewTariffServiceIn("UTC")
	plan := bandsPlan()
	// Wednesday 2026-10-07
	cases := []struct {
		name       string
		start, end time.Time
		want       money.Cents
	}{
		{"default band", time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC), time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC), 200},
		{"red evening", time.Date(2026, 10, 7, 18, 0, 0, 0, time.UTC), time.Date(2026, 10, 7, 20, 0, 0, 0, time.UTC), 500},
		{"half red half white", time.Date(2026, 10, 7, 19, 0, 0, 0, time.UTC), time.Date(2026, 10, 7, 21, 0, 0, 0, time.UTC), 350},
		{"weekend all blue", time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC), time.Date(2026, 10, 10, 14, 0, 0, 0, time.UTC), 100},
		{"midnight crossing, weekday night", time.Date(2026, 10, 7, 23, 0, 0, 0, time.UTC), time.Date(2026, 10, 8, 2, 0, 0, 0, time.UTC), 100},
		{"friday night into saturday morning", time.Date(2026, 10, 9, 23, 0, 0, 0, time.UTC), time.Date(2026, 10, 10, 5, 0, 0, 0, time.UTC), 100},
		{"sunday 03:00 is blue by the weekend rule", time.Date(2026, 10, 11, 3, 0, 0, 0, time.UTC), time.Date(2026, 10, 11, 5, 0, 0, 0, time.UTC), 100},
		{"monday 03:00 follows sunday night: weekday rule does not cover it", time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC), time.Date(2026, 10, 12, 5, 0, 0, 0, time.UTC), 200},
	}
	for _, c := range cases {
		got, err := svc.CalculateSessionCost(plan, c.start, c.end, 10)
		if err != nil || got != c.want {
			t.Errorf("%s: want %d got %d (err %v)", c.name, c.want, got, err)
		}
	}
}

func TestBandsFollowWallClockAcrossDST(t *testing.T) {
	svc := NewTariffServiceIn("Europe/Paris")
	plan := bandsPlan()
	// Night of 2026-10-25 (Sunday, clocks go back): a Saturday-night session stays blue the whole way.
	start := time.Date(2026, 10, 24, 22, 30, 0, 0, time.UTC) // 00:30 local Sunday
	end := time.Date(2026, 10, 25, 6, 0, 0, 0, time.UTC)     // 07:00 local
	got, _ := svc.CalculateSessionCost(plan, start, end, 10)
	if got != 100 {
		t.Errorf("weekend session across DST: want 100 got %d", got)
	}
	// 23:00 local Thursday to 07:00 local Friday: blue until 06:00 local, then white for one hour.
	start = time.Date(2026, 10, 8, 21, 0, 0, 0, time.UTC)
	end = time.Date(2026, 10, 9, 5, 0, 0, 0, time.UTC)
	got, _ = svc.CalculateSessionCost(plan, start, end, 8)
	if got != 8*10*7/8+8*20/8 {
		t.Errorf("weekday night: got %d", got)
	}
}

func TestPlanValidity(t *testing.T) {
	svc := NewTariffServiceIn("UTC")
	from, to := "2026-10-01", "2026-12-31"
	rate := money.Cents(10)
	plan := &models.TariffPlan{PlanType: models.TariffTypeFlat, FlatRateCents: &rate, ValidFrom: &from, ValidTo: &to}
	at := func(m time.Month, d int) money.Cents {
		ts := time.Date(2026, m, d, 10, 0, 0, 0, time.UTC)
		c, _ := svc.CalculateSessionCost(plan, ts, ts.Add(time.Hour), 10)
		return c
	}
	if at(9, 30) != 0 || at(10, 1) != 100 || at(12, 31) != 100 {
		t.Errorf("validity bounds not honoured: %d %d %d", at(9, 30), at(10, 1), at(12, 31))
	}
}

func TestLegacyPlanReadAsGrid(t *testing.T) {
	flat := money.Cents(25)
	bands, rules, def, ok := effectiveGrid(&models.TariffPlan{PlanType: models.TariffTypeFlat, FlatRateCents: &flat})
	if !ok || len(rules) != 0 || bands[def] != 25 {
		t.Errorf("flat plan should be one band: %v %v %s", bands, rules, def)
	}
	if _, _, _, ok := effectiveGrid(&models.TariffPlan{PlanType: models.TariffTypeFlat}); ok {
		t.Error("a plan without price should not be usable")
	}
}
