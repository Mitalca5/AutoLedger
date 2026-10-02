package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestIntegrationProvenanceIsRecordedBySource(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "provenance@example.com")
	day := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)

	mustDrive(t, repo, v.ID, 77, day, 20000, 10)
	// A resync of the same TeslaMate drive updates it instead of adding one.
	mustDrive(t, repo, v.ID, 77, day, 20000, 12)

	tmCharge := 5
	cost := money.Cents(400)
	tm := &models.ChargeLog{VehicleID: v.ID, TeslaMateChargeID: &tmCharge, Date: day, KwhAdded: 20, Cost: &cost, Currency: "EUR"}
	if _, err := repo.UpsertTeslaMateCharge(ctx, tm); err != nil {
		t.Fatal(err)
	}

	manualCost := money.Cents(500)
	manual := &models.ChargeLog{VehicleID: v.ID, Date: day.Add(24 * time.Hour), KwhAdded: 10, Cost: &manualCost, Currency: "EUR"}
	if err := repo.CreateManualCharge(ctx, manual); err != nil {
		t.Fatal(err)
	}
	csvCharge := &models.ChargeLog{VehicleID: v.ID, Origin: "CSV", Date: day.Add(48 * time.Hour), KwhAdded: 11, Cost: &manualCost, Currency: "EUR"}
	if err := repo.CreateManualCharge(ctx, csvCharge); err != nil {
		t.Fatal(err)
	}
	eventID := "evt-1"
	hook := &models.ChargeLog{VehicleID: v.ID, Date: day.Add(72 * time.Hour), KwhAdded: 9, Cost: &manualCost, CostSource: "MANUAL", Currency: "EUR", ExternalID: &eventID}
	if err := repo.CreateIngestedCharge(ctx, hook); err != nil {
		t.Fatal(err)
	}
	dup := &models.ChargeLog{VehicleID: v.ID, Date: day.Add(96 * time.Hour), KwhAdded: 9, Cost: &manualCost, CostSource: "MANUAL", Currency: "EUR", ExternalID: &eventID}
	if err := repo.CreateIngestedCharge(ctx, dup); err == nil {
		t.Fatal("the same webhook event id must not be recorded twice for a vehicle")
	}

	odo := 20100.0
	csvDrive := &models.Drive{VehicleID: v.ID, Origin: "CSV", StartTime: day.Add(time.Hour * 30), EndTime: day.Add(time.Hour * 31), StartOdometer: &odo, DistanceKm: 5, Tags: []string{}}
	if err := repo.CreateManualDrive(ctx, csvDrive); err != nil {
		t.Fatal(err)
	}
	manualDrive := &models.Drive{VehicleID: v.ID, StartTime: day.Add(time.Hour * 40), EndTime: day.Add(time.Hour * 41), DistanceKm: 3, Tags: []string{}}
	if err := repo.CreateManualDrive(ctx, manualDrive); err != nil {
		t.Fatal(err)
	}

	type row struct{ origin, externalID string }
	got := map[string]row{}
	rows, err := db.Pool.Query(ctx, `SELECT 'drive:' || id::text, origin, COALESCE(external_id, '') FROM drives WHERE vehicle_id = $1
		UNION ALL SELECT 'charge:' || id::text, origin, COALESCE(external_id, '') FROM charge_logs WHERE vehicle_id = $1`, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := map[row]int{}
	for rows.Next() {
		var id string
		var r row
		if err := rows.Scan(&id, &r.origin, &r.externalID); err != nil {
			t.Fatal(err)
		}
		got[id] = r
		count[r]++
	}
	want := map[row]int{
		{"TESLAMATE", "77"}:  1,
		{"TESLAMATE", "5"}:   1,
		{"MANUAL", ""}:       2,
		{"CSV", ""}:          2,
		{"WEBHOOK", "evt-1"}: 1,
	}
	if len(got) != 7 {
		t.Fatalf("expected 7 rows, got %d: %v", len(got), got)
	}
	for r, n := range want {
		if count[r] != n {
			t.Errorf("origin %s / external id %q: want %d row(s), got %d", r.origin, r.externalID, n, count[r])
		}
	}
}
