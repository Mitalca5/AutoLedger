package services

import (
	"context"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestIntegrationComparisonCurrencyRoundTrip(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "scenario-currency@example.com")
	sc := &models.ComparisonScenario{UserID: v.UserID, Name: "Projection", Currency: "USD", Mode: models.ComparisonModeProjection, AnnualKm: 12000, Years: 5, ICE: models.ICEInputs{FuelType: "SP95_E10", LPer100Km: 6, FuelPrice: 1.5}, Tracked: &models.TrackedInputs{KwhPer100Km: 16, EurPerKwh: 0.2}}
	if err := repo.CreateComparisonScenario(ctx, sc); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetComparisonScenario(ctx, v.UserID, sc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Currency != "USD" {
		t.Fatalf("lost currency: %+v", got)
	}
	got.Name = "Renamed"
	if err := repo.UpdateComparisonScenario(ctx, got); err != nil {
		t.Fatal(err)
	}
	list, err := repo.ListComparisonScenarios(ctx, v.UserID)
	if err != nil || len(list) != 1 || list[0].Currency != "USD" {
		t.Fatalf("list lost currency: %+v %v", list, err)
	}
}
