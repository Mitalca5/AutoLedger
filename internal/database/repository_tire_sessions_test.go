package database

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

func newTestTire(t *testing.T, repo *Repository, ctx context.Context, vehicleID string, pos models.TirePosition, initialKm float64) *models.Tire {
	t.Helper()
	tire := &models.Tire{
		VehicleID: &vehicleID, Brand: "B", Model: "M", Dimension: "205/55R16", Season: models.TireSeasonSummer,
		PurchaseDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), CurrentPosition: pos,
		InitialDepthMm: 8, MinLegalDepthMm: 1.6, AccumulatedDistanceKm: initialKm,
	}
	if pos != models.TirePosStorage {
		odo := 1000.0
		tire.MountedOdometer = &odo
	}
	if err := repo.CreateTire(ctx, tire); err != nil {
		t.Fatal(err)
	}
	return tire
}

func tireDistance(t *testing.T, repo *Repository, ctx context.Context, vehicleID, tireID string) float64 {
	t.Helper()
	tire, err := repo.GetTireByID(ctx, tireID, vehicleID)
	if err != nil {
		t.Fatal(err)
	}
	return tire.AccumulatedDistanceKm
}

func day(m, d int) time.Time { return time.Date(2026, time.Month(m), d, 0, 0, 0, 0, time.UTC) }

func TestTireMountSessionLifecycle(t *testing.T) {
	repo, ctx, _, _, v := vehicleRepoFixture(t, "tiresessions")
	tire := newTestTire(t, repo, ctx, v.ID, models.TirePosStorage, 500)

	closed := func(from, to float64, dismountDate *time.Time, dismountOdo *float64, dist float64) *models.TireMountSession {
		return &models.TireMountSession{TireID: tire.ID, VehicleID: v.ID, Position: models.TirePosFL, MountedDate: day(2, 1),
			MountedOdometer: from, DismountedDate: dismountDate, DismountedOdometer: dismountOdo, DistanceKm: dist}
	}
	d1, odo1 := day(3, 1), 1800.0

	// The odometers win over a manual distance.
	s1 := closed(1000, 0, &d1, &odo1, 5)
	if err := repo.CreateTireMountSession(ctx, s1); err != nil || s1.DistanceKm != 800 || s1.ID == "" {
		t.Fatalf("closed session = %+v, %v", s1, err)
	}
	if got := tireDistance(t, repo, ctx, v.ID, tire.ID); got != 1300 {
		t.Fatalf("500 initial + 800 = 1300, got %v", got)
	}

	// Without odometers a manual distance is kept.
	d2 := day(4, 1)
	s2 := closed(0, 0, &d2, nil, 100)
	if err := repo.CreateTireMountSession(ctx, s2); err != nil || s2.DistanceKm != 100 {
		t.Fatalf("manual session = %+v, %v", s2, err)
	}
	if got := tireDistance(t, repo, ctx, v.ID, tire.ID); got != 1400 {
		t.Fatalf("distance = %v, want 1400", got)
	}

	lower := 900.0
	requireAPIError(t, repo.CreateTireMountSession(ctx, closed(1000, 0, &d1, &lower, 0)), "tire.dismount_odometer_lower")
	requireAPIError(t, repo.CreateTireMountSession(ctx, closed(0, 0, &d1, nil, -5)), "tire.session_distance_negative")

	// A tire of another vehicle cannot receive a session.
	foreign := closed(1000, 0, &d1, &odo1, 0)
	foreign.VehicleID = "00000000-0000-0000-0000-000000000000"
	requireNotFound(t, repo.CreateTireMountSession(ctx, foreign))

	// An open session ignores any distance and replaces the previous open one; it never counts in the total.
	open1 := closed(2000, 0, nil, nil, 999)
	if err := repo.CreateTireMountSession(ctx, open1); err != nil || open1.DistanceKm != 0 {
		t.Fatalf("open session = %+v, %v", open1, err)
	}
	open2 := closed(2500, 0, nil, nil, 0)
	if err := repo.CreateTireMountSession(ctx, open2); err != nil {
		t.Fatal(err)
	}
	sessions, err := repo.ListTireMountSessions(ctx, tire.ID)
	if err != nil || len(sessions) != 3 {
		t.Fatalf("sessions = %d, %v (two closed + one open)", len(sessions), err)
	}
	got, _ := repo.GetTireByID(ctx, tire.ID, v.ID)
	if got.MountedOdometer == nil || *got.MountedOdometer != 2500 || got.AccumulatedDistanceKm != 1400 {
		t.Fatalf("tire = %+v", got)
	}

	// Updating a closed session recomputes the total; reopening it replaces the open one.
	odo1b := 2000.0
	s1.DismountedOdometer = &odo1b
	if err := repo.UpdateTireMountSession(ctx, s1); err != nil || s1.DistanceKm != 1000 {
		t.Fatalf("update = %+v, %v", s1, err)
	}
	if got := tireDistance(t, repo, ctx, v.ID, tire.ID); got != 1600 {
		t.Fatalf("distance after update = %v, want 1600", got)
	}
	open2.MountedOdometer = 2600
	if err := repo.UpdateTireMountSession(ctx, open2); err != nil {
		t.Fatal(err)
	}
	s2.DismountedDate = nil
	if err := repo.UpdateTireMountSession(ctx, s2); err != nil {
		t.Fatal(err)
	}
	sessions, _ = repo.ListTireMountSessions(ctx, tire.ID)
	openCount := 0
	for _, s := range sessions {
		if s.DismountedDate == nil {
			openCount++
		}
	}
	if openCount != 1 {
		t.Fatalf("a tire has one open session at a time, got %d", openCount)
	}
	requireAPIError(t, repo.UpdateTireMountSession(ctx, closed(1000, 0, &d1, &lower, 0)), "tire.dismount_odometer_lower")
	ghost := closed(1000, 0, &d1, &odo1, 0)
	ghost.ID, ghost.VehicleID = "00000000-0000-0000-0000-000000000000", v.ID
	requireNotFound(t, repo.UpdateTireMountSession(ctx, ghost))

	// Deleting a session recomputes the total and checks the vehicle.
	requireNotFound(t, repo.DeleteTireMountSession(ctx, "00000000-0000-0000-0000-000000000000", s1.ID, tire.ID))
	if err := repo.DeleteTireMountSession(ctx, v.ID, s1.ID, tire.ID); err != nil {
		t.Fatal(err)
	}
	requireNotFound(t, repo.DeleteTireMountSession(ctx, v.ID, s1.ID, tire.ID))
	if got := tireDistance(t, repo, ctx, v.ID, tire.ID); got != 500 {
		t.Fatalf("only the initial distance remains: %v", got)
	}

	none, err := repo.ListTireMountSessions(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("unknown tire gives an empty list, got %+v, %v", none, err)
	}
}

func TestCopyTireHistory(t *testing.T) {
	repo, ctx, _, _, v := vehicleRepoFixture(t, "tirecopy")
	source := newTestTire(t, repo, ctx, v.ID, models.TirePosFL, 0)
	mounted := newTestTire(t, repo, ctx, v.ID, models.TirePosFR, 0)
	stored := newTestTire(t, repo, ctx, v.ID, models.TirePosStorage, 0)

	// The source has a closed session on RL (besides its open one on FL) and two tread logs.
	d, odo := day(5, 1), 1500.0
	if err := repo.CreateTireMountSession(ctx, &models.TireMountSession{TireID: source.ID, VehicleID: v.ID, Position: models.TirePosRL,
		MountedDate: day(4, 1), MountedOdometer: 1000, DismountedDate: &d, DismountedOdometer: &odo}); err != nil {
		t.Fatal(err)
	}
	for i, depth := range []float64{7.5, 6.0} {
		if err := repo.AddTireLog(ctx, &models.TireLog{TireID: source.ID, Date: day(5, 1+i), Odometer: 1500, DepthMm: depth}); err != nil {
			t.Fatal(err)
		}
	}

	if err := repo.CopyTireHistory(ctx, v.ID, source.ID, nil, true, true, false); err != nil {
		t.Fatalf("no target is a no-op: %v", err)
	}
	requireNotFound(t, repo.CopyTireHistory(ctx, v.ID, "00000000-0000-0000-0000-000000000000", []string{mounted.ID}, true, true, false))
	requireNotFound(t, repo.CopyTireHistory(ctx, v.ID, source.ID, []string{"00000000-0000-0000-0000-000000000000"}, true, true, false))

	// Copying onto the mounted tire: the closed session keeps its position unless adapted, the open session lands
	// on the target's wheel and replaces the target's own; the source itself is skipped; logs are copied.
	if err := repo.CopyTireHistory(ctx, v.ID, source.ID, []string{source.ID, mounted.ID}, true, true, false); err != nil {
		t.Fatal(err)
	}
	sessions, _ := repo.ListTireMountSessions(ctx, mounted.ID)
	if len(sessions) != 2 {
		t.Fatalf("mounted tire sessions = %+v", sessions)
	}
	positions := map[models.TirePosition]bool{}
	for _, s := range sessions {
		positions[s.Position] = true
	}
	if !positions[models.TirePosRL] || !positions[models.TirePosFR] || positions[models.TirePosFL] {
		t.Fatalf("positions = %v, want RL (closed, kept) and FR (open, target's wheel)", positions)
	}
	logs, _ := repo.ListTireLogs(ctx, mounted.ID)
	if len(logs) != 2 {
		t.Fatalf("logs copied = %d", len(logs))
	}
	if got := tireDistance(t, repo, ctx, v.ID, mounted.ID); got != 500 {
		t.Fatalf("target distance is recomputed from the copied sessions: %v", got)
	}

	// Onto a stored tire with adaptPosition: the open session is skipped (nothing is fitted), the closed one keeps its position.
	if err := repo.CopyTireHistory(ctx, v.ID, source.ID, []string{stored.ID}, true, false, true); err != nil {
		t.Fatal(err)
	}
	sessions, _ = repo.ListTireMountSessions(ctx, stored.ID)
	if len(sessions) != 1 || sessions[0].Position != models.TirePosRL {
		t.Fatalf("stored tire sessions = %+v", sessions)
	}
	if logs, _ := repo.ListTireLogs(ctx, stored.ID); len(logs) != 0 {
		t.Fatalf("logs must not be copied when not asked: %d", len(logs))
	}

	// With adaptPosition a mounted target gets its own wheel on the closed session too.
	other := newTestTire(t, repo, ctx, v.ID, models.TirePosRR, 0)
	if err := repo.CopyTireHistory(ctx, v.ID, source.ID, []string{other.ID}, true, false, true); err != nil {
		t.Fatal(err)
	}
	sessions, _ = repo.ListTireMountSessions(ctx, other.ID)
	for _, s := range sessions {
		if s.Position != models.TirePosRR {
			t.Fatalf("adapted session on %s, want RR", s.Position)
		}
	}
}
