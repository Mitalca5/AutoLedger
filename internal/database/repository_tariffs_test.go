package database

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func flatPlan(userID, name string, rate int64, isDefault bool) *models.TariffPlan {
	r := money.Rate(rate)
	return &models.TariffPlan{UserID: userID, Name: name, PlanType: models.TariffTypeFlat, Currency: "EUR", FlatRateCents: &r, IsDefault: isDefault}
}

func TestTariffPlanDefaultsAndLookups(t *testing.T) {
	repo, ctx, owner, other, v := vehicleRepoFixture(t, "tariffs")

	_, err := repo.GetDefaultTariffPlan(ctx, owner.ID)
	requireNotFound(t, err)

	first := flatPlan(owner.ID, "Home", 200000, true)
	if err := repo.CreateTariffPlan(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := flatPlan(owner.ID, "Work", 300000, true)
	if err := repo.CreateTariffPlan(ctx, second); err != nil {
		t.Fatal(err)
	}

	// Creating a default plan demotes the previous one.
	got, err := repo.GetTariffPlanByID(ctx, first.ID, owner.ID)
	if err != nil || got.IsDefault {
		t.Fatalf("first plan must no longer be the default: %+v, %v", got, err)
	}
	def, err := repo.GetDefaultTariffPlan(ctx, owner.ID)
	if err != nil || def.ID != second.ID {
		t.Fatalf("default = %+v, %v", def, err)
	}

	// Updating a plan as default demotes the others in turn.
	first.IsDefault = true
	if err := repo.UpdateTariffPlan(ctx, first); err != nil {
		t.Fatal(err)
	}
	def, _ = repo.GetDefaultTariffPlan(ctx, owner.ID)
	if def.ID != first.ID {
		t.Fatalf("default after update = %s, want %s", def.ID, first.ID)
	}

	list, err := repo.ListTariffPlans(ctx, owner.ID)
	if err != nil || len(list) != 2 || list[0].ID != first.ID {
		t.Fatalf("list = %+v, %v (the default plan comes first)", list, err)
	}
	empty, err := repo.ListTariffPlans(ctx, other.ID)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("a user without plans gets an empty list, got %+v, %v", empty, err)
	}

	// Plans are private to their owner.
	_, err = repo.GetTariffPlanByID(ctx, first.ID, other.ID)
	requireNotFound(t, err)
	ghost := flatPlan(other.ID, "Ghost", 1, false)
	ghost.ID = first.ID
	requireNotFound(t, repo.UpdateTariffPlan(ctx, ghost))
	requireNotFound(t, repo.DeleteTariffPlan(ctx, first.ID, other.ID))

	// A vehicle without an assigned plan falls back to its owner's default; an assigned plan wins.
	plan, err := repo.GetVehicleTariffPlan(ctx, v.ID)
	if err != nil || plan.ID != first.ID {
		t.Fatalf("fallback = %+v, %v", plan, err)
	}
	if _, err := repo.pool.Exec(ctx, `UPDATE vehicles SET tariff_plan_id = $1 WHERE id = $2`, second.ID, v.ID); err != nil {
		t.Fatal(err)
	}
	plan, err = repo.GetVehicleTariffPlan(ctx, v.ID)
	if err != nil || plan.ID != second.ID {
		t.Fatalf("assigned = %+v, %v", plan, err)
	}
	if _, err := repo.GetVehicleTariffPlan(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unknown vehicle: %v", err)
	}

	if err := repo.DeleteTariffPlan(ctx, second.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	requireNotFound(t, repo.DeleteTariffPlan(ctx, second.ID, owner.ID))
	plan, err = repo.GetVehicleTariffPlan(ctx, v.ID)
	if err != nil || plan.ID != first.ID {
		t.Fatalf("after the assigned plan is deleted the default applies: %+v, %v", plan, err)
	}
}

func TestVehicleTariffPlanAtPicksTheVersionCoveringTheDay(t *testing.T) {
	repo, ctx, owner, _, v := vehicleRepoFixture(t, "tariffversions")

	// Without any plan there is nothing to price with.
	_, err := repo.GetVehicleTariffPlanAt(ctx, v.ID, "2026-03-01")
	requireNotFound(t, err)

	from25, to25 := "2025-01-01", "2025-12-31"
	old := flatPlan(owner.ID, "Home", 100000, true)
	old.ValidFrom, old.ValidTo = &from25, &to25
	if err := repo.CreateTariffPlan(ctx, old); err != nil {
		t.Fatal(err)
	}
	from26 := "2026-01-01"
	current := flatPlan(owner.ID, "home", 200000, false)
	current.ValidFrom = &from26
	if err := repo.CreateTariffPlan(ctx, current); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, `UPDATE vehicles SET tariff_plan_id = $1 WHERE id = $2`, old.ID, v.ID); err != nil {
		t.Fatal(err)
	}

	for day, want := range map[string]string{"2025-06-15": old.ID, "2026-03-01": current.ID} {
		got, err := repo.GetVehicleTariffPlanAt(ctx, v.ID, day)
		if err != nil || got.ID != want {
			t.Fatalf("%s: got %+v, %v, want %s", day, got, err, want)
		}
	}
	// No version covers the day: the assigned plan is returned unchanged.
	got, err := repo.GetVehicleTariffPlanAt(ctx, v.ID, "2024-06-01")
	if err != nil || got.ID != old.ID {
		t.Fatalf("uncovered day: %+v, %v", got, err)
	}
}

func TestPublicChargingPresets(t *testing.T) {
	repo, ctx, owner, other, _ := vehicleRepoFixture(t, "presets")

	list, err := repo.ListPublicChargingPresets(ctx, owner.ID)
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("no presets must give an empty list, got %+v, %v", list, err)
	}

	b := &models.PublicChargingPreset{UserID: owner.ID, Name: "Beta", ConnectionFee: 100, PricePerKwh: money.Rate(450000),
		PricePerMinute: 5, IdleFeePerMinute: 20, IdleGraceMinutes: 10, Currency: "EUR"}
	a := &models.PublicChargingPreset{UserID: owner.ID, Name: "Alpha", Currency: "EUR"}
	for _, p := range []*models.PublicChargingPreset{b, a} {
		if err := repo.CreatePublicChargingPreset(ctx, p); err != nil || p.ID == "" {
			t.Fatalf("create %s: %v", p.Name, err)
		}
	}
	list, err = repo.ListPublicChargingPresets(ctx, owner.ID)
	if err != nil || len(list) != 2 || list[0].Name != "Alpha" || list[1].IdleGraceMinutes != 10 || list[1].PricePerKwh != money.Rate(450000) {
		t.Fatalf("list = %+v, %v (ordered by name, values kept)", list, err)
	}

	requireNotFound(t, repo.DeletePublicChargingPreset(ctx, a.ID, other.ID))
	if err := repo.DeletePublicChargingPreset(ctx, a.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	requireNotFound(t, repo.DeletePublicChargingPreset(ctx, a.ID, owner.ID))
}
