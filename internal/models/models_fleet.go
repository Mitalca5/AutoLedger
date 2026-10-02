package models

import "github.com/teslacost/teslacost/internal/money"

// FleetSummaryResponse aggregates multi-vehicle household metrics.
type FleetSummaryResponse struct {
	TotalVehicles    int         `json:"total_vehicles"`
	Currency         string      `json:"currency"`
	CurrentMonthCost money.Cents `json:"current_month_cost"`
	// MonthlyBudget is the household spending target for a month; nil when none is set.
	MonthlyBudget          *money.Cents         `json:"monthly_budget"`
	CurrentMonthDistanceKm float64              `json:"current_month_distance_km"`
	CurrentMonthEnergyKwh  float64              `json:"current_month_energy_kwh"`
	MonthlyCosts           []FleetMonthlyCost   `json:"monthly_costs"`
	Vehicles               []VehicleFleetMetric `json:"vehicles"`
	MemberKmShares         []MemberKmShare      `json:"member_km_shares"`
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
	VehicleID          string      `json:"vehicle_id"`
	Name               string      `json:"name"`
	Make               string      `json:"make"`
	Model              string      `json:"model"`
	Currency           string      `json:"currency"`
	CurrentOdometer    float64     `json:"current_odometer"`
	MonthDistanceKm    float64     `json:"month_distance_km"`
	MonthCost          money.Cents `json:"month_cost"`
	EnergyCostPer100Km float64     `json:"energy_cost_per_100km"`
}

// MemberKmShare provides distance distribution among household members.
type MemberKmShare struct {
	UserID      string  `json:"user_id"`
	DisplayName string  `json:"display_name"`
	DistanceKm  float64 `json:"distance_km"`
	Percentage  float64 `json:"percentage"`
}
