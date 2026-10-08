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
