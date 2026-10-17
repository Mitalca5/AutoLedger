package database

import (
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestVehicleTariffRequiresMatchingCurrency(t *testing.T) {
	repo, ctx, owner, _, vehicle := vehicleRepoFixture(t, "tariff-currency")
	foreign := flatPlan(owner.ID, "Household", 100_000_000, true)
	foreign.Currency = "ARS"
	if err := repo.CreateTariffPlan(ctx, foreign); err != nil {
		t.Fatal(err)
	}
	_, err := repo.GetVehicleTariffPlan(ctx, vehicle.ID)
	requireNotFound(t, err)
	if _, err := repo.pool.Exec(ctx, `UPDATE vehicles SET tariff_plan_id = $1 WHERE id = $2`, foreign.ID, vehicle.ID); err != nil {
		t.Fatal(err)
	}
	_, err = repo.GetVehicleTariffPlanAt(ctx, vehicle.ID, "2026-10-01")
	requireNotFound(t, err)

	matching := flatPlan(owner.ID, foreign.Name, 250000, false)
	matching.Currency = vehicle.Currency
	if err := repo.CreateTariffPlan(ctx, matching); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, `UPDATE vehicles SET tariff_plan_id = $1 WHERE id = $2`, matching.ID, vehicle.ID); err != nil {
		t.Fatal(err)
	}
	// A newer foreign-currency version with the same name must not override this tariff.
	from := "2026-01-01"
	foreign.ValidFrom = &from
	if err := repo.UpdateTariffPlan(ctx, foreign); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetVehicleTariffPlanAt(ctx, vehicle.ID, "2026-10-01")
	if err != nil || got.ID != matching.ID || got.Currency != vehicle.Currency {
		t.Fatalf("matching tariff = %+v, %v", got, err)
	}
	// Other vehicles in the account can still use the foreign-currency plan legitimately.
	other := &models.Vehicle{UserID: owner.ID, Name: "Second", Currency: "ARS", Powertrain: models.PowertrainEV}
	if err := repo.CreateVehicle(ctx, other); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetVehicleTariffPlanAt(ctx, other.ID, "2026-10-01")
	if err != nil || got.ID != foreign.ID {
		t.Fatalf("foreign-currency vehicle tariff = %+v, %v", got, err)
	}
}
