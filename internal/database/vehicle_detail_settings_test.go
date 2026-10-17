package database

import "testing"

func TestVehicleDetailPreservesConfiguredDefaults(t *testing.T) {
	repo, ctx, owner, _, vehicle := vehicleRepoFixture(t, "detail-settings")
	plan := flatPlan(owner.ID, "Home", 250000, true)
	if err := repo.CreateTariffPlan(ctx, plan); err != nil {
		t.Fatal(err)
	}
	vehicle.TariffPlanID = &plan.ID
	vehicle.IsHomeChargerDefault = true
	driverID := *vehicle.DefaultDriverID
	if err := repo.UpdateVehicle(ctx, vehicle); err != nil {
		t.Fatal(err)
	}
	detail, err := repo.GetVehicleByID(ctx, vehicle.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.DefaultDriverID == nil || *detail.DefaultDriverID != driverID || detail.TariffPlanID == nil || *detail.TariffPlanID != plan.ID || !detail.IsHomeChargerDefault {
		t.Fatalf("detail lost settings: %+v", detail)
	}
	detail.Name = "Renamed"
	if err := repo.UpdateVehicle(ctx, detail); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.GetVehicleByIDInternal(ctx, vehicle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.DefaultDriverID == nil || *stored.DefaultDriverID != driverID || stored.TariffPlanID == nil || *stored.TariffPlanID != plan.ID || !stored.IsHomeChargerDefault {
		t.Fatalf("detail round trip lost settings: %+v", stored)
	}
}
