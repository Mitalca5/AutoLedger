package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestIntegrationCarpoolHistoryStopsAtTripDate(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "carpool-history@example.com")
	ref := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		date   time.Time
		amount float64
	}{{ref.AddDate(0, -2, 0), 1}, {ref.AddDate(0, 1, 0), 9}} {
		cost := money.FromFloat(c.amount)
		charge := &models.ChargeLog{VehicleID: v.ID, Date: c.date, KwhAdded: 10, Cost: &cost, Currency: "EUR"}
		if err := repo.CreateManualCharge(ctx, charge); err != nil {
			t.Fatal(err)
		}
	}
	got, err := NewCarpoolService(db.Pool, repo).GetVehicleUnitRatesAt(ctx, v.ID, &ref)
	if err != nil {
		t.Fatal(err)
	}
	if got.ElectricitySource != RateSourceHistory || got.ElectricityPerKwh != 0.1 {
		t.Fatalf("future charge changed historical rate: %+v", got)
	}
}
