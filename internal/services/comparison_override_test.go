package services

import (
	"math"
	"testing"

	"github.com/teslacost/teslacost/internal/money"
)

func TestWithElectricityPrice(t *testing.T) {
	tests := []struct {
		name       string
		ev         TrackedBaseline
		sum        TCOSummary
		price      float64
		wantEnergy float64
	}{
		{
			name:       "scales the recorded energy cost by the price ratio",
			ev:         TrackedBaseline{EnergyPerKm: 0.04},
			sum:        TCOSummary{AvgCostPerKwh: 0.20, TotalKwhAdded: 1000, DistanceBasisKm: 5000},
			price:      0.10,
			wantEnergy: 0.02,
		},
		{
			name:       "free charging stays diluted instead of being repriced",
			ev:         TrackedBaseline{EnergyPerKm: 0.02},
			sum:        TCOSummary{AvgCostPerKwh: 0.20, TotalKwhAdded: 1000, DistanceBasisKm: 5000},
			price:      0.40,
			wantEnergy: 0.04,
		},
		{
			name:       "prices the energy drawn per km when no kWh was priced",
			ev:         TrackedBaseline{EnergyPerKm: 0},
			sum:        TCOSummary{TotalKwhAdded: 800, DistanceBasisKm: 4000},
			price:      0.25,
			wantEnergy: 0.05,
		},
		{
			name:       "keeps the fuel part of a hybrid",
			ev:         TrackedBaseline{EnergyPerKm: 0.10, FuelPerKm: 0.06},
			sum:        TCOSummary{AvgCostPerKwh: 0.20, TotalKwhAdded: 1000, DistanceBasisKm: 5000},
			price:      0.10,
			wantEnergy: 0.08,
		},
		{
			name:       "nothing recorded: unchanged",
			ev:         TrackedBaseline{EnergyPerKm: 0.03},
			sum:        TCOSummary{},
			price:      0.10,
			wantEnergy: 0.03,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withElectricityPrice(tt.ev, &tt.sum, tt.price)
			if math.Abs(got.EnergyPerKm-tt.wantEnergy) > 1e-9 {
				t.Fatalf("EnergyPerKm = %v, want %v", got.EnergyPerKm, tt.wantEnergy)
			}
			if got.FuelPerKm != tt.ev.FuelPerKm {
				t.Fatalf("FuelPerKm changed: %v", got.FuelPerKm)
			}
		})
	}
}

func TestMeasuredTracked(t *testing.T) {
	sum := &TCOSummary{AvgCostPerKwh: 0.2149, TotalKwhAdded: 1234.5, DistanceBasisKm: 7000}
	ev := TrackedBaseline{MaintenancePerKm: 0.02, InsuranceYearly: 600, TaxYearly: 40, PurchaseNet: 38000}
	m := measuredTracked(sum, ev, 10000)
	if m.KwhPer100Km != 17.6 || m.EurPerKwh != 0.215 {
		t.Errorf("energy figures = %v kWh/100 km, %v per kWh", m.KwhPer100Km, m.EurPerKwh)
	}
	if m.MaintenanceYearly != money.FromFloat(200) || m.InsuranceYearly != money.FromFloat(600) || m.TaxYearly != money.FromFloat(40) || m.PurchasePrice != money.FromFloat(38000) {
		t.Errorf("amounts = %+v", m)
	}
	if empty := measuredTracked(&TCOSummary{}, TrackedBaseline{}, 10000); empty.KwhPer100Km != 0 {
		t.Errorf("no distance must not invent a consumption, got %v", empty.KwhPer100Km)
	}
}
