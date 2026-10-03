package services

import (
	"testing"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestSummariseBatteryHealthPicksBestSource(t *testing.T) {
	// A recorded percentage wins over everything else.
	s := summariseBatteryHealth([]models.BatterySnapshot{
		{Date: "2026-01-01", HealthPercent: fp(95)},
		{Date: "2026-06-01", HealthPercent: fp(92.5), MaxCapacityKwh: fp(60)},
	}, fp(50))
	if s.Source != "reading" || *s.HealthPercent != 92.5 || *s.ReferenceKwh != 60 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	// Recorded capacities give a percentage.
	s = summariseBatteryHealth([]models.BatterySnapshot{{Date: "2026-06-01", CurrentCapacityKwh: fp(54), MaxCapacityKwh: fp(60)}}, nil)
	if s.Source != "capacity" || *s.HealthPercent != 90 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	// Charges alone need the new-battery capacity from some reading.
	s = summariseBatteryHealth([]models.BatterySnapshot{{Date: "2026-01-01", MaxCapacityKwh: fp(80)}}, fp(72))
	if s.Source != "charges" || *s.HealthPercent != 90 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	s = summariseBatteryHealth(nil, fp(72))
	if s.HealthPercent != nil || s.EstimatedCapacityKwh == nil {
		t.Fatalf("no reference must leave the percentage unknown: %+v", s)
	}
	// An implausible ratio is ignored; a capacity above nominal is capped at 100.
	if _, ok := capacityPercent(10, 60); ok {
		t.Fatal("ratio below 30% must be rejected")
	}
	if pct, _ := capacityPercent(61, 60); pct != 100 {
		t.Fatalf("got %v", pct)
	}
}

func ownershipFor(price, resale float64, months int) *models.VehicleOwnership {
	p, r := money.FromFloat(price), money.FromFloat(resale)
	return &models.VehicleOwnership{PurchasePrice: &p, ExpectedResaleValue: &r, ExpectedHoldingMonths: &months}
}

func TestComputeResidual(t *testing.T) {
	own := ownershipFor(40000, 20000, 48)
	// Half-way through the holding period by age only: geometric mean of the two anchors.
	r, err := computeResidual(own, 24, 30000, nil, ResidualOptions{KmShare: 0.5, HealthWeight: 1})
	if err != nil {
		t.Fatal(err)
	}
	if r.Progress != 0.5 || r.CurrentValue.String() != "28284.27" {
		t.Fatalf("age only: progress %v value %s", r.Progress, r.CurrentValue)
	}
	if len(r.Curve) != 49 || r.Curve[0].Value.String() != "40000.00" || r.Curve[48].Value.String() != "20000.00" {
		t.Fatalf("curve ends: %v %v", r.Curve[0], r.Curve[48])
	}
	// Driving more than planned pulls the progress forward.
	r, _ = computeResidual(own, 24, 90000, nil, ResidualOptions{ExpectedKm: 120000, KmShare: 0.5, HealthWeight: 1})
	if r.Progress != 0.625 {
		t.Fatalf("blended progress %v", r.Progress)
	}
	// Each lost point of health removes a share of the value, scaled by the weight.
	r, _ = computeResidual(own, 24, 0, fp(90), ResidualOptions{HealthWeight: 1})
	if r.HealthFactor != 0.9 || r.CurrentValue.String() != "25455.84" {
		t.Fatalf("health: %v %s", r.HealthFactor, r.CurrentValue)
	}
	r, _ = computeResidual(own, 24, 0, fp(90), ResidualOptions{HealthWeight: 0})
	if r.HealthFactor != 1 {
		t.Fatalf("weight 0 must ignore health, got %v", r.HealthFactor)
	}
	if r.Depreciation.String() != "11715.73" {
		t.Fatalf("depreciation %s", r.Depreciation)
	}
}

func TestComputeResidualRejectsUnusableInputs(t *testing.T) {
	if _, err := computeResidual(&models.VehicleOwnership{}, 1, 1, nil, ResidualOptions{}); err == nil {
		t.Fatal("missing anchors must fail")
	}
	if _, err := computeResidual(ownershipFor(1000, 2000, 12), 1, 1, nil, ResidualOptions{}); err == nil {
		t.Fatal("resale above purchase must fail")
	}
	for _, o := range []ResidualOptions{{KmShare: 1.5}, {ExpectedKm: -1}, {HealthWeight: 3}} {
		if _, err := computeResidual(ownershipFor(1000, 500, 12), 1, 1, nil, o); err == nil {
			t.Fatalf("options %+v must fail", o)
		}
	}
}
