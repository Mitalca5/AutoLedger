package services

import (
	"testing"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestComparisonPublishesStorageAndValidationLimits(t *testing.T) {
	sc := &models.ComparisonScenario{Mode: models.ComparisonModeProjection, AnnualKm: 12000, Years: 5, ICE: models.ICEInputs{FuelType: "SP95_E10", LPer100Km: 6, FuelPrice: 1.5}, Tracked: &models.TrackedInputs{KwhPer100Km: 16, EurPerKwh: 0.2}}
	result := ComputeComparison(sc, TrackedBaseline{})
	if result.InputLimits.FuelPriceMax != models.MaxFuelPricePerLiter || result.InputLimits.ElectricityPriceMax != money.MaxRate.Float() || result.InputLimits.AnnualKmMax != models.MaxComparisonAnnualKm {
		t.Fatalf("result lost API limits: %+v", result.InputLimits)
	}
}
