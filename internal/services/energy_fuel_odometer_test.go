package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestIntegrationEnergyStatisticsUseFuelOdometers(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "energy-fuel-odo@example.com")
	if _, err := db.Pool.Exec(ctx, `UPDATE vehicles SET powertrain='PHEV' WHERE id=$1`, v.ID); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct {
		date time.Time
		odo  float64
	}{{d(2026, 1, 1), 0}, {d(2026, 2, 1), 988}} {
		litres, price := 20.0, 1.5
		odo := p.odo
		f := &models.FuelLog{VehicleID: v.ID, Date: p.date, Odometer: &odo, Amount: money.FromFloat(30), Liters: &litres, PricePerLiter: &price, FuelType: func() *string { v := "SP95_E10"; return &v }()}
		if err := repo.CreateFuelLog(ctx, f); err != nil {
			t.Fatal(err)
		}
	}
	cost := money.Cents(0)
	charge := &models.ChargeLog{VehicleID: v.ID, Date: d(2026, 1, 15), KwhAdded: 18, Cost: &cost, Currency: "EUR"}
	if err := repo.CreateManualCharge(ctx, charge); err != nil {
		t.Fatal(err)
	}
	got, err := NewEnergyStatsService(db.Pool, "UTC").Compute(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary.ConsumptionKwh100km == nil || *got.Summary.ConsumptionKwh100km != 1.8 || got.Basis.Consumption != BasisDerived {
		t.Fatalf("fill-up distance missing: %+v", got)
	}
}
