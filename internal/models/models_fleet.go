package models

import "github.com/teslacost/teslacost/internal/money"

// FleetSummaryResponse aggregates multi-vehicle household metrics.
type FleetSummaryResponse struct {
	TotalVehicles int `json:"total_vehicles"`
	// Currency is the currency of the budget and of the monthly figures below: the one of the first vehicle. Vehicles
	// in another currency are not added to them, they appear in Totals.
	Currency string `json:"currency"`
	// Totals is the cost of the month in progress, one line per currency in order of first appearance.
	Totals []FleetCurrencyTotal `json:"totals"`
	// CurrentMonth is the month in progress (YYYY-MM) in the reporting timezone.
	CurrentMonth     string      `json:"current_month"`
	CurrentMonthCost money.Cents `json:"current_month_cost"`
	// MonthlyBudget is the household spending target for a month; nil when none is set.
	MonthlyBudget          *money.Cents         `json:"monthly_budget"`
	CurrentMonthDistanceKm float64              `json:"current_month_distance_km"`
	CurrentMonthEnergyKwh  float64              `json:"current_month_energy_kwh"`
	MonthlyCosts           []FleetMonthlyCost   `json:"monthly_costs"`
	Vehicles               []VehicleFleetMetric `json:"vehicles"`
	MemberKmShares         []MemberKmShare      `json:"member_km_shares"`
}

// FleetCurrencyTotal is the cost of the month in progress of the vehicles that share one currency.
type FleetCurrencyTotal struct {
	Currency         string      `json:"currency"`
	VehicleCount     int         `json:"vehicle_count"`
	CurrentMonthCost money.Cents `json:"current_month_cost"`
}

// FleetCurrencyTotals groups the month cost of the vehicles by currency, never adding two currencies together.
func FleetCurrencyTotals(vehicles []VehicleFleetMetric) []FleetCurrencyTotal {
	totals := []FleetCurrencyTotal{}
	index := make(map[string]int)
	for _, v := range vehicles {
		i, ok := index[v.Currency]
		if !ok {
			i = len(totals)
			index[v.Currency] = i
			totals = append(totals, FleetCurrencyTotal{Currency: v.Currency})
		}
		totals[i].VehicleCount++
		totals[i].CurrentMonthCost += v.MonthCost
	}
	return totals
}

// FleetMonthlyCost represents costs for the entire fleet for a single month.
type FleetMonthlyCost struct {
	Month      string                 `json:"month"` // YYYY-MM
	TotalCost  money.Cents            `json:"total_cost"`
	EnergyCost money.Cents            `json:"energy_cost"`
	OtherCost  money.Cents            `json:"other_cost"`
	DistanceKm float64                `json:"distance_km"`
	ByVehicle  map[string]money.Cents `json:"by_vehicle"`
}

// VehicleFleetMetric provides comparison KPIs for each vehicle in the household fleet.
type VehicleFleetMetric struct {
	VehicleID       string      `json:"vehicle_id"`
	Name            string      `json:"name"`
	Make            string      `json:"make"`
	Model           string      `json:"model"`
	Currency        string      `json:"currency"`
	CurrentOdometer float64     `json:"current_odometer"`
	MonthDistanceKm float64     `json:"month_distance_km"`
	MonthCost       money.Cents `json:"month_cost"`
	// EnergyCostPer100Km is the energy-statistics figure (priced charges over the distance of the months driven), or
	// the energy cost of the TCO over its distance basis for a vehicle the statistics do not cover. nil when it is
	// not known; 0 is a known cost (free charging).
	EnergyCostPer100Km *float64 `json:"energy_cost_per_100km"`

	Powertrain string `json:"powertrain"`
	// RunningCostPerKm is what the vehicle costs to run (energy, upkeep, insurance, financing); FullCostPerKm adds
	// depreciation and amortized tires. Both rely on the distance basis of the TCO.
	RunningCostPerKm *float64 `json:"running_cost_per_km"`
	FullCostPerKm    *float64 `json:"full_cost_per_km"`
	// KwhPer100Km and LitersPer100Km are set when the vehicle consumed that energy; a plug-in hybrid has both.
	KwhPer100Km    *float64 `json:"kwh_per_100km"`
	LitersPer100Km *float64 `json:"liters_per_100km"`
	// AnnualCost is the full cost extrapolated to twelve months over the observed history; nil under three months.
	AnnualCost *money.Cents `json:"annual_cost"`
	// CompletenessPct is the TCO completeness score; a vehicle under the comparison threshold is shown but not ranked.
	CompletenessPct int  `json:"completeness_pct"`
	Comparable      bool `json:"comparable"`
	// DepreciationKnown is false when the vehicle was bought but its depreciation is not entered: it is not ranked.
	DepreciationKnown bool `json:"depreciation_known"`
}

// MemberKmShare provides distance distribution among the people who drive the fleet.
type MemberKmShare struct {
	PersonID    string  `json:"person_id"`
	DisplayName string  `json:"display_name"`
	DistanceKm  float64 `json:"distance_km"`
	Percentage  float64 `json:"percentage"`
}
