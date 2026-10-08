package database

import (
	"context"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestTariffBandsMigrationAndVersions(t *testing.T) {
	pool := sourcesTestPool(t)
	ctx := context.Background()
	migrateBefore(t, pool, "000051")

	var userID, planID, vehicleID string
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ('tar@example.com', 'x') RETURNING id::text`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO tariff_plans (user_id, name, plan_type, peak_rate_cents, offpeak_rate_cents, time_windows)
		VALUES ($1, 'Home', 'TIME_OF_USE', 30, 15, '[{"start":"22:00","end":"06:00","kind":"OFFPEAK"}]') RETURNING id::text`, userID).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO vehicles (user_id, name, tariff_plan_id) VALUES ($1, 'Car', $2) RETURNING id::text`, userID, planID).Scan(&vehicleID); err != nil {
		t.Fatal(err)
	}

	if err := (&DB{Pool: pool}).Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)

	plan, err := repo.GetTariffPlanByID(ctx, planID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.PlanType != models.TariffTypeBands || len(plan.Bands) != 2 || len(plan.Rules) != 1 ||
		plan.Rules[0].Band != "OFFPEAK" || plan.Rules[0].Start != "22:00" || plan.DefaultBand != "PEAK" {
		t.Fatalf("legacy plan not converted: %+v", plan)
	}
	if plan.Bands[0].RateCents != 300000 || plan.Bands[1].RateCents != 150000 {
		t.Errorf("rates lost: %+v", plan.Bands)
	}

	from, to := "2026-01-01", "2026-12-31"
	oldYear := &models.TariffPlan{UserID: userID, Name: "home", PlanType: models.TariffTypeBands, Currency: "EUR", DefaultBand: "A",
		Bands: []models.TariffBand{{Name: "A", RateCents: money.Rate(100000)}}, ValidFrom: &from, ValidTo: &to}
	if err := repo.CreateTariffPlan(ctx, oldYear); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetVehicleTariffPlanAt(ctx, vehicleID, "2026-06-15")
	if err != nil || got.ID != oldYear.ID {
		t.Fatalf("2026 day should use the dated version, got %+v err %v", got, err)
	}
	got, err = repo.GetVehicleTariffPlanAt(ctx, vehicleID, "2027-03-01")
	if err != nil || got.ID != planID {
		t.Fatalf("day outside the dated version should use the open-ended plan, got %+v err %v", got, err)
	}
}
