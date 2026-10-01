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
