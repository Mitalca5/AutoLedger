package services

import (
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestApplyTCOMonthsToFleetSumsVehicleMonths(t *testing.T) {
	res := &models.FleetSummaryResponse{
		CurrentMonthCost: 1,
		MonthlyCosts: []models.FleetMonthlyCost{
			{Month: "2026-09", TotalCost: 1, EnergyCost: 1, OtherCost: 0},
			{Month: "2026-10"},
		},
		Vehicles: []models.VehicleFleetMetric{{VehicleID: "a"}, {VehicleID: "b"}},
	}
	sums := map[string]*TCOSummary{
		"a": {MonthlyCosts: []MonthlyCost{
			{Month: "2026-09", Energy: 2579, Total: 24662},
			{Month: "2026-10", Energy: 100, Total: 5712},
		}},
		"b": {MonthlyCosts: []MonthlyCost{
			{Month: "2026-09", Energy: 5445, Total: 62691},
			{Month: "2026-10", Energy: 200, Total: 22458},
		}},
	}

	applyTCOMonthsToFleet(res, sums, time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC))

	sep := res.MonthlyCosts[0]
	if sep.TotalCost != money.Cents(24662+62691) || sep.EnergyCost != money.Cents(2579+5445) {
		t.Fatalf("september total/energy = %d/%d", sep.TotalCost, sep.EnergyCost)
	}
	if sep.OtherCost != sep.TotalCost-sep.EnergyCost {
		t.Fatalf("other = %d, want total - energy", sep.OtherCost)
	}
	if sep.ByVehicle["a"] != 24662 || sep.ByVehicle["b"] != 62691 {
		t.Fatalf("by vehicle = %v", sep.ByVehicle)
	}
	if res.CurrentMonthCost != money.Cents(5712+22458) {
		t.Fatalf("current month = %d", res.CurrentMonthCost)
	}
	if res.Vehicles[0].MonthCost != 5712 || res.Vehicles[1].MonthCost != 22458 {
		t.Fatalf("vehicle month costs = %d/%d", res.Vehicles[0].MonthCost, res.Vehicles[1].MonthCost)
	}
}

func TestApplyTCOMonthsToFleetKeepsCurrenciesApart(t *testing.T) {
	res := &models.FleetSummaryResponse{
		Currency:     "EUR",
		MonthlyCosts: []models.FleetMonthlyCost{{Month: "2026-10"}},
		Vehicles: []models.VehicleFleetMetric{
			{VehicleID: "eur-1", Currency: "EUR"},
			{VehicleID: "eur-2", Currency: "EUR"},
			{VehicleID: "chf", Currency: "CHF"},
		},
	}
	month := func(total money.Cents) *TCOSummary {
		return &TCOSummary{MonthlyCosts: []MonthlyCost{{Month: "2026-10", Energy: 0, Total: total}}}
	}
	sums := map[string]*TCOSummary{"eur-1": month(1924), "eur-2": month(8768), "chf": month(90000)}

	applyTCOMonthsToFleet(res, sums, time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC))

	if res.CurrentMonthCost != 1924+8768 {
		t.Fatalf("fleet month = %d, want the euro vehicles only", res.CurrentMonthCost)
	}
	if got := res.MonthlyCosts[0]; got.TotalCost != 1924+8768 || len(got.ByVehicle) != 2 {
		t.Fatalf("monthly chart mixes currencies: %+v", got)
	}
	want := []models.FleetCurrencyTotal{
		{Currency: "EUR", VehicleCount: 2, CurrentMonthCost: 1924 + 8768},
		{Currency: "CHF", VehicleCount: 1, CurrentMonthCost: 90000},
	}
	if len(res.Totals) != 2 || res.Totals[0] != want[0] || res.Totals[1] != want[1] {
		t.Fatalf("totals = %+v, want %+v", res.Totals, want)
	}
}

func TestApplyTCOMonthsToFleetTakesDistanceFromTheTCOMonths(t *testing.T) {
	res := &models.FleetSummaryResponse{
		Currency:     "EUR",
		MonthlyCosts: []models.FleetMonthlyCost{{Month: "2026-10", DistanceKm: 10}},
		Vehicles: []models.VehicleFleetMetric{
			{VehicleID: "ev", Currency: "EUR", MonthDistanceKm: 10},
			{VehicleID: "ice", Currency: "EUR"},
			{VehicleID: "chf", Currency: "CHF"},
		},
		CurrentMonthDistanceKm: 10,
	}
	month := func(km float64) *TCOSummary {
		return &TCOSummary{MonthlyCosts: []MonthlyCost{{Month: "2026-10", DistanceKm: km}}}
	}
	// The combustion vehicle has no drive: its distance comes from fill-up readings.
	sums := map[string]*TCOSummary{"ev": month(120.4), "ice": month(231), "chf": month(40)}

	applyTCOMonthsToFleet(res, sums, time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC))

	if res.Vehicles[1].MonthDistanceKm != 231 {
		t.Fatalf("combustion month distance = %v, want 231 from its readings", res.Vehicles[1].MonthDistanceKm)
	}
	if res.CurrentMonthDistanceKm != 120.4+231+40 {
		t.Fatalf("fleet month distance = %v, want every vehicle counted whatever its currency", res.CurrentMonthDistanceKm)
	}
	if res.MonthlyCosts[0].DistanceKm != 120.4+231+40 {
		t.Fatalf("history distance = %v", res.MonthlyCosts[0].DistanceKm)
	}
}
