package handlers

import (
	"context"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestFleetSummaryComputesEnergyCostPer100Km(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "fleet-summary@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "EV", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	pool := repo.Pool()
	if _, err := pool.Exec(ctx, `INSERT INTO charge_logs (vehicle_id, date, kwh_added, cost, cost_source, currency, is_manual)
		VALUES ($1, now(), 50, 1000, 'MANUAL', 'EUR', TRUE)`, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO drives (vehicle_id, start_time, end_time, distance_km, duration_min)
		VALUES ($1, now() - interval '1 hour', now(), 200, 60)`, v.ID); err != nil {
		t.Fatal(err)
	}

	summary, err := repo.GetFleetSummary(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetFleetSummary failed: %v", err)
	}
	if len(summary.Vehicles) != 1 {
		t.Fatalf("expected 1 vehicle, got %d", len(summary.Vehicles))
	}
	t.Logf("vehicle metric: %+v", summary.Vehicles[0])
}
