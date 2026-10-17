package handlers

import (
	"math"
	"testing"

	"github.com/teslacost/teslacost/internal/money"
)

func TestEstimatedEnergyRateStorageBound(t *testing.T) {
	if err := validateEstimatedEnergyRate(nil); err != nil {
		t.Fatal(err)
	}
	for _, price := range []float64{0, 0.251234, 80, 1400, money.MaxRate.Float()} {
		if err := validateEstimatedEnergyRate(&price); err != nil {
			t.Errorf("valid price %v: %v", price, err)
		}
	}
	for _, price := range []float64{-0.01, money.MaxRate.Float() + 0.000001, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := validateEstimatedEnergyRate(&price); err == nil {
			t.Errorf("invalid price %v accepted", price)
		}
	}
}
