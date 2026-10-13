package services

import (
	"context"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestIntegrationRetrospectiveElectricityPriceOverride(t *testing.T) {
	db, repo, v, now := setupHybridComparison(t, models.PowertrainEV, 1000)
	ctx := context.Background()
	cost := eur(30)
	if err := repo.CreateManualCharge(ctx, &models.ChargeLog{VehicleID: v.ID, Date: now.AddDate(0, 0, -3), KwhAdded: 100, Cost: &cost, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	svc := NewComparisonService(NewTCOService(db.Pool, "UTC"))
	scenario := func(override float64) *models.ComparisonScenario {
		return &models.ComparisonScenario{
			VehicleID: &v.ID,
			Mode:      models.ComparisonModeRetrospective,
			AnnualKm:  12000,
			Years:     5,
			ICE:       models.ICEInputs{FuelType: "SP95_E10", LPer100Km: 6, FuelPrice: 1.8},
			Options:   models.ScenarioOptions{ElectricityPriceOverride: override},
		}
	}

	recorded, err := svc.Compare(ctx, scenario(0))
	if err != nil {
		t.Fatal(err)
	}
	if m := recorded.TrackedMeasured; m == nil || m.EurPerKwh != 0.3 || m.KwhPer100Km != 10 || m.ElectricityOverridden {
		t.Fatalf("measured figures of the recorded scenario = %+v", m)
	}

	halved, err := svc.Compare(ctx, scenario(0.15))
	if err != nil {
		t.Fatal(err)
	}
	if m := halved.TrackedMeasured; m == nil || m.EurPerKwh != 0.3 || !m.ElectricityOverridden {
		t.Fatalf("an override keeps the measured price and says it is overridden, got %+v", m)
	}
	if want := recorded.Tracked.Energy / 2; halved.Tracked.Energy != want {
		t.Fatalf("tracked energy = %v, want half of %v", halved.Tracked.Energy, recorded.Tracked.Energy)
	}
	if halved.ICE.Energy != recorded.ICE.Energy {
		t.Fatalf("the combustion side must not move: %v vs %v", halved.ICE.Energy, recorded.ICE.Energy)
	}
}
