package database

import (
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

func TestTollDetectionActiveFollowsTheModeAndTheEvidence(t *testing.T) {
	repo, ctx, owner, _, v := vehicleRepoFixture(t, "toll-active")
	pool := repo.Pool()

	active := func() bool {
		t.Helper()
		got, err := repo.TollDetectionActive(ctx, v.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	setMode := func(mode string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `UPDATE vehicles SET toll_detection = $1 WHERE id::text = $2`, mode, v.ID); err != nil {
			t.Fatal(err)
		}
	}
	setLanguage := func(lang string) {
		t.Helper()
		if err := repo.UpdateUserLanguage(ctx, owner.ID, lang); err != nil {
			t.Fatal(err)
		}
	}

	// A new vehicle is on auto; an English-speaking owner with no sign of France sees no toll detection.
	got, err := repo.GetVehicleByID(ctx, v.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TollDetection != models.TollDetectionAuto || got.TollDetectionActive {
		t.Fatalf("new vehicle = mode %q active %v, want auto and inactive", got.TollDetection, got.TollDetectionActive)
	}

	// The French interface is enough on auto, and an explicit off still wins over it.
	setLanguage("fr")
	if !active() {
		t.Error("auto with a French owner should be active")
	}
	setMode(models.TollDetectionOff)
	if active() {
		t.Error("off should win over the French interface")
	}
	setLanguage("en")

	// On is explicit, whatever the evidence.
	setMode(models.TollDetectionOn)
	if !active() {
		t.Error("on should be active without any evidence")
	}
	setMode(models.TollDetectionAuto)
	if active() {
		t.Fatal("back to auto without evidence should be inactive")
	}

	// An address outside France is no evidence; one naming France is.
	vid := v.ID
	start, end := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC), time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	elsewhere, home := "Berlin, Deutschland", "Lyon, France"
	d := &models.Drive{VehicleID: vid, StartTime: start, EndTime: end, DistanceKm: 50, StartAddress: &elsewhere, EndAddress: &elsewhere}
	if err := repo.CreateManualDrive(ctx, d); err != nil {
		t.Fatal(err)
	}
	if active() {
		t.Error("an address outside France should not activate the detection")
	}
	if _, err := pool.Exec(ctx, `UPDATE drives SET end_address = $1 WHERE id::text = $2`, home, d.ID); err != nil {
		t.Fatal(err)
	}
	if !active() {
		t.Error("an address naming France should activate the detection")
	}
	if _, err := pool.Exec(ctx, `UPDATE drives SET end_address = $1 WHERE id::text = $2`, elsewhere, d.ID); err != nil {
		t.Fatal(err)
	}

	// A detection that matched stations is evidence, an empty one is not.
	if _, err := pool.Exec(ctx, `INSERT INTO toll_detections (drive_id, vehicle_id, segments) VALUES ($1::uuid, $2::uuid, '[]'::jsonb)`, d.ID, vid); err != nil {
		t.Fatal(err)
	}
	if active() {
		t.Error("a detection without segment should not activate the detection")
	}
	if _, err := pool.Exec(ctx, `UPDATE toll_detections SET segments = '[{"entry":"A"}]'::jsonb WHERE drive_id::text = $1`, d.ID); err != nil {
		t.Fatal(err)
	}
	if !active() {
		t.Error("a detection with a segment should activate the detection")
	}
	if _, err := pool.Exec(ctx, `DELETE FROM toll_detections WHERE drive_id::text = $1`, d.ID); err != nil {
		t.Fatal(err)
	}

	// So is an automatic toll expense.
	if _, err := pool.Exec(ctx, `INSERT INTO drive_expenses (vehicle_id, drive_id, type, amount, date, source) VALUES ($1::uuid, $2::uuid, 'TOLL', 5, NOW(), 'AUTO_TOLL')`, vid, d.ID); err != nil {
		t.Fatal(err)
	}
	if !active() {
		t.Error("an automatic toll expense should activate the detection")
	}

	_, err = repo.TollDetectionActive(ctx, "00000000-0000-0000-0000-000000000000")
	requireNotFound(t, err)
}
