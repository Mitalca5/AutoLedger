package handlers

import (
	"math"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestComparisonCurrencyNeutralPriceBounds(t *testing.T) {
	for _, price := range []float64{0, 1400, models.MaxFuelPricePerLiter} {
		r := validComparisonRequest()
		r.ICE.FuelPrice = price
		if err := validateComparisonRequest(&r); err != nil {
			t.Fatalf("fuel %v: %v", price, err)
		}
	}
	for _, price := range []float64{0, 100, money.MaxRate.Float()} {
		r := validComparisonRequest()
		r.Mode = models.ComparisonModeProjection
		r.Tracked = &models.TrackedInputs{KwhPer100Km: 16, EurPerKwh: price}
		if err := validateComparisonRequest(&r); err != nil {
			t.Fatalf("electricity %v: %v", price, err)
		}
	}
	for _, price := range []float64{-1, models.MaxFuelPricePerLiter + 1, math.NaN(), math.Inf(1)} {
		r := validComparisonRequest()
		r.ICE.FuelPrice = price
		if err := validateComparisonRequest(&r); err == nil {
			t.Fatalf("accepted fuel %v", price)
		}
	}
}
