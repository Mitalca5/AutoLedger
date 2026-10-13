package services

import (
	"context"
	"math"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestMeasuredWearComparedWithDeclaredOrAverageLifespan(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		rate                      float64
		declared                  int
		measured, expected, delta float64
		reference                 string
	}{
		{"declared lifespan, faster wear", 2, 40000, 32000, 40000, -20, "declared"},
		{"average wear", 2.4, 0, 26667, 53333, -50, "average"},
		{"slower wear", 0.8, 40000, 80000, 40000, 100, "declared"},
		{"matching wear", 1.6, 40000, 40000, 40000, 0, "declared"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := compareMeasuredTireWear(tc.rate, 6.4, tc.declared)
			if got == nil || got.MeasuredLifespanKm != tc.measured || got.ExpectedLifespanKm != tc.expected || got.DeltaPct != tc.delta || got.Reference != tc.reference {
				t.Fatalf("unexpected comparison: %+v", got)
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
	if stats.WearComparison == nil || stats.WearComparison.MeasuredLifespanKm != 32000 || stats.WearComparison.ExpectedLifespanKm != 40000 || stats.WearComparison.DeltaPct != -20 {
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
