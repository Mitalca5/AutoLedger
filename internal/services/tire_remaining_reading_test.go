package services

import (
	"context"
	"math"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestTireForecastConsumesMileageAfterReading(t *testing.T) {
	mount := 0.0
	tire := wearTire(models.TirePosFL)
	tire.MountedOdometer = &mount
	store := &fakeTireWearStore{logs: []models.TireLog{{DepthMm: 6, Odometer: 10000}, {DepthMm: 8, Odometer: 0}}, sessions: []models.TireMountSession{{MountedOdometer: 0, Position: models.TirePosFL}}}
	svc := NewTireWearService(store)
	before, err := svc.CalculateTireWear(context.Background(), tire, 10000)
	if err != nil {
		t.Fatal(err)
	}
	after, err := svc.CalculateTireWear(context.Background(), tire, 20000)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(before.EstimatedRemainingKm-after.EstimatedRemainingKm-10000) > 1 || after.CurrentDepthMm != before.CurrentDepthMm {
		t.Fatalf("remaining should fall without rewriting measured depth: %+v %+v", before, after)
	}
	end, odo := d(2026, 2, 1), 10000.0
	store.sessions[0].DismountedDate = &end
	store.sessions[0].DismountedOdometer = &odo
	tire.CurrentPosition = models.TirePosStorage
	tire.AccumulatedDistanceKm = 10000
	stored, err := svc.CalculateTireWear(context.Background(), tire, 50000)
	if err != nil {
		t.Fatal(err)
	}
	if stored.EstimatedRemainingKm != before.EstimatedRemainingKm {
		t.Fatalf("storage mileage consumed tread: %+v", stored)
	}
}
func TestUnmeasuredTireForecastConsumesMountedMileage(t *testing.T) {
	mount := 0.0
	tire := wearTire(models.TirePosFL)
	tire.MountedOdometer = &mount
	svc := NewTireWearService(&fakeTireWearStore{})
	a, _ := svc.CalculateTireWear(context.Background(), tire, 0)
	b, _ := svc.CalculateTireWear(context.Background(), tire, 10000)
	if math.Abs(a.EstimatedRemainingKm-b.EstimatedRemainingKm-10000) > 1 {
		t.Fatalf("unmeasured forecast did not age: %+v %+v", a, b)
	}
}

func TestDynamicForecastConsumesUseSinceMeasuredDepth(t *testing.T) {
	mount := 0.0
	tire := wearTire(models.TirePosFL)
	tire.MountedOdometer = &mount
	store := &fakeTireWearStore{logs: []models.TireLog{{DepthMm: 6, Odometer: 10000}, {DepthMm: 8, Odometer: 0}}, sessions: []models.TireMountSession{{MountedOdometer: 0, Position: models.TirePosFL}}, powerMax: 80, powerMin: -35, consumption: 16, drives: 5}
	svc := NewTireWearService(store)
	a, err := svc.CalculateTireWear(context.Background(), tire, 10000)
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.CalculateTireWear(context.Background(), tire, 20000)
	if err != nil {
		t.Fatal(err)
	}
	if b.DynamicRemainingKm >= a.DynamicRemainingKm || b.CurrentDepthMm != 6 {
		t.Fatalf("telemetry projection ignored use or rewrote depth: %+v %+v", a, b)
	}
}

func TestTireReadingFallbackWithoutMountSessions(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		reading, current, want float64
	}{
		{"reading on current mount", 25000, 30000, 31667},
		{"reading before current mount", 15000, 30000, 26667},
		{"reading ahead of odometer", 35000, 30000, 36667},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mount := 20000.0
			tire := wearTire(models.TirePosFL)
			tire.MountedOdometer = &mount
			tire.AccumulatedDistanceKm = 12000
			store := &fakeTireWearStore{logs: []models.TireLog{{DepthMm: 6, Odometer: tc.reading}}}
			stats, err := NewTireWearService(store).CalculateTireWear(context.Background(), tire, tc.current)
			if err != nil {
				t.Fatal(err)
			}
			if stats.EstimatedRemainingKm != tc.want {
				t.Fatalf("remaining = %v, want %v", stats.EstimatedRemainingKm, tc.want)
			}
			if stats.CurrentDepthMm != 6 {
				t.Fatal("recorded depth changed")
			}
		})
	}
}

func TestTireLifetimeDistanceAtReadingIncludesInitialMileage(t *testing.T) {
	tire := &models.Tire{InitialDistanceKm: 4000}
	end := 15000.0
	sessions := []models.TireMountSession{
		{MountedOdometer: 10000, DismountedOdometer: &end},
		{MountedOdometer: 25000},
	}
	if got := tireLifetimeDistanceAtReading(tire, sessions, 28000); got != 12000 {
		t.Fatalf("lifetime distance = %v, want 12000", got)
	}
}
