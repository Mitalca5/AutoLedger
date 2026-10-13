package services

import (
	"context"
	"math"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestMeasuredWearComparedWithDeclaredOrAverageLifespan(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		rate                          float64
		declared                      int
		expectedRate, lifespan, delta float64
		reference                     string
	}{
		{"declared lifespan", 2, 40000, 1.6, 40000, 25, "declared"},
		{"average wear", 2.4, 0, 1.2, 53333, 100, "average"},
		{"slower wear", 0.8, 40000, 1.6, 40000, -50, "declared"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := compareMeasuredTireWear(tc.rate, 6.4, tc.declared)
			if got == nil || math.Abs(got.ExpectedRatePer10kKm-tc.expectedRate) > 1e-9 || got.ExpectedLifespanKm != tc.lifespan || got.DeltaPct != tc.delta || got.Reference != tc.reference {
				t.Fatalf("unexpected comparison: %+v", got)
			}
			if got.MeasuredLifespanKm != math.Round(6.4/tc.rate*10000) {
				t.Fatal("measured lifespan does not follow the raw measured rate")
			}
		})
	}
	for _, rate := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if compareMeasuredTireWear(rate, 6.4, 0) != nil {
			t.Fatal("invalid measurement should not produce a comparison")
		}
	}
}

func TestWearComparisonUsesMeasuredRateNotBlendedForecast(t *testing.T) {
	tire := wearTire(models.TirePosFL)
	tire.EstimatedLifespanKm = 40000
	store := &fakeTireWearStore{
		logs:     []models.TireLog{{DepthMm: 6, Odometer: 10000}, {DepthMm: 8, Odometer: 0}},
		sessions: []models.TireMountSession{{MountedOdometer: 0, Position: models.TirePosFL}},
	}
	stats, err := NewTireWearService(store).CalculateTireWear(context.Background(), tire, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if stats.WearComparison == nil || stats.WearComparison.MeasuredRatePer10kKm != 2 || stats.WearComparison.MeasuredLifespanKm != 32000 || stats.WearComparison.DeltaPct != 25 {
		t.Fatalf("unexpected measured comparison: %+v", stats.WearComparison)
	}
	if stats.WearRatePer10kKm >= 2 || stats.CurrentDepthMm != 6 {
		t.Fatal("forecast blend or measured depth changed")
	}
	store.logs = nil
	stats, err = NewTireWearService(store).CalculateTireWear(context.Background(), tire, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if stats.WearComparison != nil {
		t.Fatal("default wear has no measurement to compare")
	}
}
