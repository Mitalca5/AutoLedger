package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestIntegrationCarpoolComponentsStopAtTripDate(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "historical-components@example.com")
	ref := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	mustDrive(t, repo, v.ID, 1, ref.AddDate(0, 0, -3), 10000, 1000)
	expense := &models.MaintenanceExpense{VehicleID: v.ID, Category: "MAINTENANCE", Currency: "EUR", Date: ref.AddDate(0, 0, -2), Amount: money.FromFloat(100)}
	if err := repo.CreateMaintenanceExpense(ctx, expense); err != nil {
		t.Fatal(err)
	}
	mount := 10000.0
	tire := &models.Tire{VehicleID: &v.ID, Brand: "Generic", Model: "Road", Dimension: "205/55 R16", Season: models.TireSeasonSummer, PurchaseDate: ref.AddDate(0, 0, -5), PurchasePrice: money.FromFloat(100), CurrentPosition: models.TirePosFL, EstimatedLifespanKm: 10000, MountedOdometer: &mount}
	if err := repo.CreateTire(ctx, tire); err != nil {
		t.Fatal(err)
	}
	// Record the fitting explicitly so the historical lookup is independent of defaults.
	if _, err := db.Pool.Exec(ctx, `UPDATE tire_mount_sessions SET mounted_date=$2 WHERE tire_id=$1`, tire.ID, ref.AddDate(0, 0, -5)); err != nil {
		t.Fatal(err)
	}
	svc := NewCarpoolService(db.Pool, repo)
	before, err := svc.GetVehicleUnitRatesAt(ctx, v.ID, &ref)
	if err != nil {
		t.Fatal(err)
	}
	if before.MaintenancePerKm != 0.1 || before.TiresPerKm != 0.01 {
		t.Fatalf("fixture rates: %+v", before)
	}
	mustDrive(t, repo, v.ID, 2, ref.AddDate(0, 1, 0), 11000, 3000)
	futureExpense := *expense
	futureExpense.ID = ""
	futureExpense.Date = ref.AddDate(0, 1, 0)
	futureExpense.Amount = money.FromFloat(900)
	if err := repo.CreateMaintenanceExpense(ctx, &futureExpense); err != nil {
		t.Fatal(err)
	}
	futureTire := *tire
	futureTire.ID = ""
	futureTire.PurchaseDate = ref.AddDate(0, 1, 0)
	futureTire.PurchasePrice = money.FromFloat(900)
	futureTire.CurrentPosition = models.TirePosFR
	if err := repo.CreateTire(ctx, &futureTire); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `UPDATE tire_mount_sessions SET dismounted_date=$2, dismounted_odometer=14000, distance_km=4000 WHERE tire_id=$1`, tire.ID, ref.AddDate(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	// The historical tire is now in storage: its old fitting still determines the old trip's rate.
	if _, err := db.Pool.Exec(ctx, `UPDATE tires SET current_position='STORAGE' WHERE id=$1`, tire.ID); err != nil {
		t.Fatal(err)
	}
	after, err := svc.GetVehicleUnitRatesAt(ctx, v.ID, &ref)
	if err != nil {
		t.Fatal(err)
	}
	if after.MaintenancePerKm != before.MaintenancePerKm || after.TiresPerKm != before.TiresPerKm {
		t.Fatalf("future entries moved historical rates: before=%+v after=%+v", before, after)
	}
}

// A trip just after midnight still counts the entries dated that day, and none dated the next one.
func TestIntegrationCarpoolDateBoundsUseTheTripDay(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "historical-day@example.com")
	ref := time.Date(2026, 1, 10, 0, 30, 0, 0, time.UTC)
	mustDrive(t, repo, v.ID, 1, ref.AddDate(0, 0, -3), 10000, 1000)
	for _, e := range []struct {
		day    time.Time
		amount float64
	}{{time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC), 100}, {time.Date(2026, 1, 11, 0, 0, 0, 0, time.UTC), 900}} {
		expense := &models.MaintenanceExpense{VehicleID: v.ID, Category: "MAINTENANCE", Currency: "EUR", Date: e.day, Amount: money.FromFloat(e.amount)}
		if err := repo.CreateMaintenanceExpense(ctx, expense); err != nil {
			t.Fatal(err)
		}
	}
	rates, err := NewCarpoolService(db.Pool, repo).GetVehicleUnitRatesAt(ctx, v.ID, &ref)
	if err != nil {
		t.Fatal(err)
	}
	if rates.MaintenancePerKm != 0.1 {
		t.Fatalf("maintenance rate should include the trip day only: %+v", rates)
	}
}
