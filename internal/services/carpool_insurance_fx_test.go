package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestIntegrationCarpoolInsuranceConvertsAnnualPremium(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "carpool-fx@example.com")
	ref := time.Now().UTC()
	months := 1
	fx := 2.0
	m := &models.MaintenanceExpense{VehicleID: v.ID, Category: "INSURANCE", Date: ref.AddDate(0, 0, -1), Amount: money.FromFloat(100), Currency: "USD", FxRate: &fx, IsRecurring: true, RecurrenceIntervalMonths: &months}
	if err := repo.CreateMaintenanceExpense(ctx, m); err != nil {
		t.Fatal(err)
	}
	got, err := NewCarpoolService(db.Pool, repo).GetVehicleUnitRatesAt(ctx, v.ID, &ref)
	if err != nil {
		t.Fatal(err)
	}
	if got.MonthlyInsuranceCost != money.FromFloat(200) || got.DailyInsuranceCost != money.FromFloat(2400/365.25) {
		t.Fatalf("foreign premium not converted: %+v", got)
	}
}
