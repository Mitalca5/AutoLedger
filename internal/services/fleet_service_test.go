package services

import (
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

func TestApplyTCOToFleetMetric(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	l := 6.54
	cases := []struct {
		name        string
		sum         TCOSummary
		wantComp    bool
		wantKwh     *float64
		wantLiters  *float64
		wantAnnual  int64
		wantNoAnnum bool
	}{
		{
			name: "electric vehicle with a year of history",
			sum: TCOSummary{Powertrain: "EV", DistanceBasisKm: 10000, TotalKwhAdded: 1500, FullCost: 600000, FullCostPerKm: 0.6,
				Completeness: TCOCompleteness{ScorePct: 90}, MonthlyCosts: []MonthlyCost{{Month: "2025-10"}, {Month: "2026-03"}}},
			wantComp: true, wantKwh: fp(15), wantAnnual: 600000 * 12 / 13,
		},
		{
			name: "fuel vehicle uses the measured consumption",
			sum: TCOSummary{Powertrain: "ICE", DistanceBasisKm: 10000, TotalLiters: 700, ConsumptionL100km: &l, FullCost: 100000,
				Completeness: TCOCompleteness{ScorePct: 60}, MonthlyCosts: []MonthlyCost{{Month: "2026-10"}}},
			wantComp: true, wantLiters: fp(6.5), wantNoAnnum: true,
		},
		{
			name: "plug-in hybrid has both energies",
			sum: TCOSummary{Powertrain: "PHEV", DistanceBasisKm: 5000, TotalKwhAdded: 400, TotalLiters: 200, FullCost: 100000,
				Completeness: TCOCompleteness{ScorePct: 75}, MonthlyCosts: []MonthlyCost{{Month: "2026-07"}}},
			wantComp: true, wantKwh: fp(8), wantLiters: fp(4), wantAnnual: 100000 * 12 / 4,
		},
		{
			name:     "incomplete vehicle is shown but not ranked",
			sum:      TCOSummary{Powertrain: "EV", DistanceBasisKm: 5000, FullCost: 100000, Completeness: TCOCompleteness{ScorePct: 59}},
			wantComp: false, wantNoAnnum: true,
		},
		{
			name:     "no distance, no ranking",
			sum:      TCOSummary{Powertrain: "EV", FullCost: 100000, Completeness: TCOCompleteness{ScorePct: 100}},
			wantComp: false, wantNoAnnum: true,
		},
	}
	for _, c := range cases {
		var m models.VehicleFleetMetric
		applyTCOToFleetMetric(&m, &c.sum, now)
		if m.Comparable != c.wantComp {
			t.Errorf("%s: comparable %v, want %v", c.name, m.Comparable, c.wantComp)
		}
		if !samePtr(m.KwhPer100Km, c.wantKwh) || !samePtr(m.LitersPer100Km, c.wantLiters) {
			t.Errorf("%s: energy per 100 km got kWh %v L %v", c.name, m.KwhPer100Km, m.LitersPer100Km)
		}
		if c.wantNoAnnum && m.AnnualCost != nil {
			t.Errorf("%s: annual cost should be unknown, got %d", c.name, *m.AnnualCost)
		}
		if !c.wantNoAnnum && (m.AnnualCost == nil || int64(*m.AnnualCost) != c.wantAnnual) {
			t.Errorf("%s: annual cost %v, want %d", c.name, m.AnnualCost, c.wantAnnual)
		}
	}
}

func fp(v float64) *float64 { return &v }

func samePtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
