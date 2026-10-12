package services

import (
	"context"
	"testing"

	"github.com/teslacost/teslacost/internal/money"
)

func TestPerKmIsUnknownWithoutDistance(t *testing.T) {
	if got := perKm(money.Cents(12345), 0); got != nil {
		t.Errorf("no distance: got %v, want nil", *got)
	}
	if got := perKm(money.Cents(12345), -5); got != nil {
		t.Errorf("negative distance: got %v, want nil", *got)
	}
	// A known zero cost over a real distance is a real zero, not an unknown.
	got := perKm(0, 100)
	if got == nil || *got != 0 {
		t.Errorf("zero cost over 100 km: got %v, want 0", got)
	}
	got = perKm(money.Cents(5000), 200)
	if got == nil || *got != 0.25 {
		t.Errorf("50 over 200 km: got %v, want 0.25", got)
	}
}

func TestIntegrationVehicleWithoutDataHasUnknownPerKmFigures(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	v := mustVehicle(t, repo, "empty_perkm@example.com")
	sum, err := NewTCOService(db.Pool, "UTC").ComputeVehicleTCO(context.Background(), v.ID)
	if err != nil {
		t.Fatal(err)
	}
	perKmFigures := map[string]*float64{
		"total": sum.TotalCostPerKm, "usage": sum.UsageCostPerKm, "full": sum.FullCostPerKm, "full net": sum.FullCostNetPerKm,
		"depreciation": sum.DepreciationCostPerKm, "energy": sum.EnergyCostPerKm, "tolls": sum.TollsCostPerKm,
		"tires": sum.TiresCostPerKm, "maintenance": sum.MaintenanceCostPerKm, "insurance": sum.InsuranceCostPerKm,
		"financing": sum.FinancingCostPerKm, "other": sum.OtherCostPerKm,
	}
	for name, v := range perKmFigures {
		if v != nil {
			t.Errorf("%s cost per km of a vehicle without data: got %v, want nil", name, *v)
		}
	}
	for _, mc := range sum.MonthlyCosts {
		if mc.CostPerKm != nil {
			t.Errorf("month %s: cost per km %v without distance, want nil", mc.Month, *mc.CostPerKm)
		}
	}
}
