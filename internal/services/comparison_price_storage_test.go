package services

import (
	"context"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestIntegrationComparisonStoresLocalCurrencyFuelPrice(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "price-storage@example.com")
	sc := &models.ComparisonScenario{UserID: v.UserID, Name: "Local price", Mode: models.ComparisonModeProjection, AnnualKm: 12000, Years: 5, ICE: models.ICEInputs{FuelType: "SP95_E10", LPer100Km: 6, FuelPrice: 1400}, Tracked: &models.TrackedInputs{KwhPer100Km: 16, EurPerKwh: 100}}
	if err := repo.CreateComparisonScenario(ctx, sc); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetComparisonScenario(ctx, v.UserID, sc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ICE.FuelPrice != 1400 || got.Tracked.EurPerKwh != 100 {
		t.Fatalf("local currency prices lost: %+v", got)
	}
}
