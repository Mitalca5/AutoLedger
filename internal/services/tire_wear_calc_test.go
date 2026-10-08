package services

import (
	"context"
	"errors"
	"testing"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
)

type fakeTireWearStore struct {
	logs        []models.TireLog
	sessions    []models.TireMountSession
	logsErr     error
	sessionsErr error
	powerMax    float64
	powerMin    float64
	consumption float64
	drives      int
	telemetry   error
	gotRanges   []database.OdometerRange
	calls       int
}

func (f *fakeTireWearStore) ListTireLogs(context.Context, string) ([]models.TireLog, error) {
	return f.logs, f.logsErr
}

func (f *fakeTireWearStore) ListTireMountSessions(context.Context, string) ([]models.TireMountSession, error) {
	return f.sessions, f.sessionsErr
}

func (f *fakeTireWearStore) GetDrivingTelemetryStats(_ context.Context, _ string, ranges []database.OdometerRange) (float64, float64, float64, int, error) {
	f.calls++
	f.gotRanges = ranges
	return f.powerMax, f.powerMin, f.consumption, f.drives, f.telemetry
}

func wearTire(pos models.TirePosition) *models.Tire {
	vid := "veh-1"
	return &models.Tire{ID: "t1", VehicleID: &vid, CurrentPosition: pos}
}

func TestCalculateTireWearStoreErrors(t *testing.T) {
	boom := errors.New("boom")
	for name, store := range map[string]*fakeTireWearStore{
		"logs":     {logsErr: boom},
		"sessions": {sessionsErr: boom},
	} {
		if _, err := NewTireWearService(store).CalculateTireWear(context.Background(), wearTire(models.TirePosFL), 0); !errors.Is(err, boom) {
			t.Errorf("%s: expected store error, got %v", name, err)
		}
	}
}

func TestCalculateTireWearDefaultsWithoutLogs(t *testing.T) {
	store := &fakeTireWearStore{}
	tire := wearTire(models.TirePosStorage)
	got, err := NewTireWearService(store).CalculateTireWear(context.Background(), tire, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if got.InitialDepthMm != 8 || got.MinLegalDepthMm != 1.6 || got.EstimatedLifespanKm != 40000 {
		t.Errorf("defaults not applied: %+v", got)
	}
	if got.CurrentDepthMm != 8 || got.WearPercentage != 0 || got.Condition != "GOOD" {
		t.Errorf("a new tire without logs should be unworn: %+v", got)
	}
	if got.WearRatePer10kKm != 1.2 {
		t.Errorf("expected fallback wear rate 1.2, got %v", got.WearRatePer10kKm)
	}
	if store.calls != 0 || got.DrivingStyle != "" || got.WearExplanation != nil {
		t.Errorf("a stored tire without sessions must not query telemetry")
	}
}

func TestCalculateTireWearMeasuredRate(t *testing.T) {
	// Two logs, 10 000 km apart on a tire mounted since odometer 0: 2 mm worn per 10 000 km.
	store := &fakeTireWearStore{
		logs: []models.TireLog{
			{DepthMm: 6, Odometer: 10000},
			{DepthMm: 8, Odometer: 0},
		},
		sessions: []models.TireMountSession{{MountedOdometer: 0}},
	}
	tire := wearTire(models.TirePosStorage)
	got, err := NewTireWearService(store).CalculateTireWear(context.Background(), tire, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if got.WearRatePer10kKm != 2 {
		t.Errorf("expected measured rate 2 mm/10k km, got %v", got.WearRatePer10kKm)
	}
	// (6 - 1.6) mm remaining at 2 mm / 10 000 km.
	if got.EstimatedRemainingKm != 22000 {
		t.Errorf("expected 22000 km remaining, got %v", got.EstimatedRemainingKm)
	}
	if got.DistanceTraveledKm != 10000 || got.LogsCount != 2 {
		t.Errorf("unexpected distance/logs: %+v", got)
	}
}

func TestCalculateTireWearSingleLogCountsInitialDistance(t *testing.T) {
	store := &fakeTireWearStore{
		logs:     []models.TireLog{{DepthMm: 7, Odometer: 5000}},
		sessions: []models.TireMountSession{{MountedOdometer: 0}},
	}
	tire := wearTire(models.TirePosStorage)
	tire.InitialDistanceKm = 5000
	got, err := NewTireWearService(store).CalculateTireWear(context.Background(), tire, 5000)
	if err != nil {
		t.Fatal(err)
	}
	// 1 mm worn over 5000 (before) + 5000 (mount session) km.
	if got.DistanceTraveledKm != 10000 {
		t.Errorf("expected 10000 km, got %v", got.DistanceTraveledKm)
	}
	if got.WearRatePer10kKm != 1 {
		t.Errorf("expected 1 mm/10k km, got %v", got.WearRatePer10kKm)
	}
}

func TestCalculateTireWearShortOrTinyWearUsesFallback(t *testing.T) {
	// Distance below 500 km and wear below 0.05 mm are too noisy to extrapolate.
	for name, logs := range map[string][]models.TireLog{
		"short distance": {{DepthMm: 7, Odometer: 400}, {DepthMm: 8, Odometer: 0}},
		"tiny wear":      {{DepthMm: 7.97, Odometer: 20000}, {DepthMm: 8, Odometer: 0}},
	} {
		store := &fakeTireWearStore{logs: logs, sessions: []models.TireMountSession{{MountedOdometer: 0}}}
		got, err := NewTireWearService(store).CalculateTireWear(context.Background(), wearTire(models.TirePosStorage), 20000)
		if err != nil {
			t.Fatal(err)
		}
		if got.WearRatePer10kKm != 1.2 {
			t.Errorf("%s: expected fallback rate, got %v", name, got.WearRatePer10kKm)
		}
	}
}

func TestCalculateTireWearCondition(t *testing.T) {
	for _, tc := range []struct {
		depth float64
		want  string
	}{{2.5, "CRITICAL"}, {1.0, "CRITICAL"}, {2.6, "WARNING"}, {4.0, "WARNING"}, {4.1, "GOOD"}} {
		store := &fakeTireWearStore{logs: []models.TireLog{{DepthMm: tc.depth}}}
		got, err := NewTireWearService(store).CalculateTireWear(context.Background(), wearTire(models.TirePosStorage), 0)
		if err != nil {
			t.Fatal(err)
		}
		if got.Condition != tc.want {
			t.Errorf("depth %v: expected %s, got %s", tc.depth, tc.want, got.Condition)
		}
	}
}

func TestCalculateTireWearMountedRunAndLifespan(t *testing.T) {
	odo := 20000.0
	tire := wearTire(models.TirePosFL)
	tire.MountedOdometer = &odo
	tire.AccumulatedDistanceKm = 5000
	tire.EstimatedLifespanKm = 20000
	tire.PurchasePrice = 20000 // cents
	store := &fakeTireWearStore{sessions: []models.TireMountSession{{MountedOdometer: odo}}}
	got, err := NewTireWearService(store).CalculateTireWear(context.Background(), tire, 25000)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentRunKm != 5000 || got.TotalDistanceKm != 10000 {
		t.Errorf("unexpected run/total: %v / %v", got.CurrentRunKm, got.TotalDistanceKm)
	}
	if got.LifeProgressPct != 50 {
		t.Errorf("expected 50%% life progress, got %v", got.LifeProgressPct)
	}
	if got.CostPerKm != 0.01 {
		t.Errorf("expected 0.01 per km, got %v", got.CostPerKm)
	}
	if got.Sessions[0].DistanceKm != 5000 {
		t.Errorf("the open session should show the live run, got %v", got.Sessions[0].DistanceKm)
	}

	// An odometer before the mount reading adds no run; the progress is capped at 100 %.
	over, err := NewTireWearService(store).CalculateTireWear(context.Background(), tire, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if over.CurrentRunKm != 0 {
		t.Errorf("expected no run, got %v", over.CurrentRunKm)
	}
	tire.AccumulatedDistanceKm = 90000
	capped, _ := NewTireWearService(store).CalculateTireWear(context.Background(), tire, 10000)
	if capped.LifeProgressPct != 100 {
		t.Errorf("expected progress capped at 100, got %v", capped.LifeProgressPct)
	}
}

func TestCalculateTireWearTelemetryRanges(t *testing.T) {
	odo := 100.0
	end := 900.0

	// Closed and open sessions are forwarded as odometer windows.
	store := &fakeTireWearStore{sessions: []models.TireMountSession{
		{MountedOdometer: 0, DismountedOdometer: &end},
		{MountedOdometer: odo},
	}}
	tire := wearTire(models.TirePosFL)
	tire.MountedOdometer = &odo
	if _, err := NewTireWearService(store).CalculateTireWear(context.Background(), tire, 1000); err != nil {
		t.Fatal(err)
	}
	if len(store.gotRanges) != 2 || store.gotRanges[0].Max == nil || store.gotRanges[1].Max != nil {
		t.Errorf("unexpected ranges: %+v", store.gotRanges)
	}

	// Without sessions, a mounted tire falls back to its own mount odometer.
	store = &fakeTireWearStore{}
	if _, err := NewTireWearService(store).CalculateTireWear(context.Background(), tire, 1000); err != nil {
		t.Fatal(err)
	}
	if len(store.gotRanges) != 1 || store.gotRanges[0].Min != odo || store.gotRanges[0].Max != nil {
		t.Errorf("unexpected fallback range: %+v", store.gotRanges)
	}

	// No vehicle: no telemetry lookup.
	store = &fakeTireWearStore{}
	noVehicle := &models.Tire{ID: "t2", CurrentPosition: models.TirePosFL, MountedOdometer: &odo}
	if _, err := NewTireWearService(store).CalculateTireWear(context.Background(), noVehicle, 1000); err != nil {
		t.Fatal(err)
	}
	if store.calls != 0 {
		t.Errorf("telemetry must not be queried without a vehicle")
	}

	// A telemetry failure degrades to the static estimate instead of failing.
	store = &fakeTireWearStore{telemetry: errors.New("db down"), sessions: []models.TireMountSession{{MountedOdometer: odo}}}
	got, err := NewTireWearService(store).CalculateTireWear(context.Background(), tire, 1000)
	if err != nil {
		t.Fatalf("telemetry error must not fail the calculation: %v", err)
	}
	if got.DrivesCount != 0 || got.WearExplanation != nil {
		t.Errorf("expected no dynamic data after a telemetry error: %+v", got)
	}
}

func TestCalculateTireWearDrivingStyles(t *testing.T) {
	odo := 0.0
	for _, tc := range []struct {
		name                    string
		pos                     models.TirePosition
		powerMax, powerMin, kwh float64
		style                   string
		stress                  float64
	}{
		{"gentle front", models.TirePosFR, 45, -20, 13.5, "ECO", 0.86},
		{"balanced any axle", models.TirePosStorage, 80, -35, 16, "BALANCED", 1},
		{"aggressive rear", models.TirePosRL, 200, -90, 22, "SPORT", 1.43},
		{"no power data front", models.TirePosFL, 0, 0, 0, "ECO", 0.92},
	} {
		tire := wearTire(tc.pos)
		tire.MountedOdometer = &odo
		store := &fakeTireWearStore{
			powerMax: tc.powerMax, powerMin: tc.powerMin, consumption: tc.kwh, drives: 5,
			sessions: []models.TireMountSession{{MountedOdometer: 0}},
		}
		got, err := NewTireWearService(store).CalculateTireWear(context.Background(), tire, 1000)
		if err != nil {
			t.Fatal(err)
		}
		if got.DrivingStyle != tc.style {
			t.Errorf("%s: expected style %s, got %s (stress %v)", tc.name, tc.style, got.DrivingStyle, got.DrivingStressIndex)
		}
		if tc.stress != 0 && got.DrivingStressIndex != tc.stress {
			t.Errorf("%s: expected stress %v, got %v", tc.name, tc.stress, got.DrivingStressIndex)
		}
		if got.WearExplanation == nil || got.DrivesCount != 5 {
			t.Errorf("%s: expected an explanation for %d drives", tc.name, got.DrivesCount)
		}
	}
}
