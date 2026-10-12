package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// A hybrid's price per kWh is its electricity over the priced kWh: neither the fuel nor a charge without a cost take part.
func TestIntegrationHybridPricesPerKwhIgnoreFuelAndUnpricedCharges(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "hybrid-price@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "PHEV", Powertrain: models.PowertrainPHEV, TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 10000}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	odometer := func(f float64) *float64 { return &f }
	for i, odo := range []float64{10000, 10600} {
		f := &models.FuelLog{VehicleID: v.ID, Date: now.AddDate(0, -3+i, 0), Odometer: odometer(odo), Amount: money.FromFloat(60), Liters: odometer(35), IsFullTank: true}
		if err := repo.CreateFuelLog(ctx, f); err != nil {
			t.Fatal(err)
		}
	}
	cost := money.FromFloat(10)
	priced := &models.ChargeLog{VehicleID: v.ID, Date: now.AddDate(0, -1, 0), KwhAdded: 50, Cost: &cost, Currency: "EUR"}
	if err := repo.CreateManualCharge(ctx, priced); err != nil {
		t.Fatal(err)
	}
	// A synced charge whose price is unknown: its kWh count as added, not as priced.
	if _, err := db.Pool.Exec(ctx, `INSERT INTO charge_logs (vehicle_id, date, kwh_added, cost, cost_source, currency, is_manual)
		VALUES ($1, $2, 50, NULL, 'TESLAMATE', 'EUR', FALSE)`, v.ID, now.AddDate(0, 0, -10)); err != nil {
		t.Fatal(err)
	}

	tco := NewTCOService(db.Pool, "UTC")
	sum, err := tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.AvgCostPerKwh != 0.2 {
		t.Fatalf("avg cost per kWh = %v, want 0.2 (10 of electricity over the 50 priced kWh)", sum.AvgCostPerKwh)
	}
	defaults, err := NewComparisonService(tco).Defaults(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if defaults.TrackedEurPerKwh == nil || *defaults.TrackedEurPerKwh != 0.2 {
		t.Fatalf("comparison prefill = %v, want 0.2", defaults.TrackedEurPerKwh)
	}
}
