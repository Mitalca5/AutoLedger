package models

import "github.com/teslacost/teslacost/internal/money"

// BatteryHealthSummary is the state of health of a traction battery, whatever the source of the readings
// (TeslaMate, an OBD2 reading typed by the user, a reading pushed through the ingestion API) or, failing
// that, the capacity estimated from complete charging sessions.
type BatteryHealthSummary struct {
	Readings []BatterySnapshot `json:"readings"`
	// HealthPercent is the latest known state of health; absent when it cannot be established.
	HealthPercent *float64 `json:"health_percent,omitempty"`
	// Source is "reading" (a recorded percentage), "capacity" (recorded capacities) or "charges" (estimated from sessions).
	Source string `json:"source,omitempty"`
	// ReferenceKwh is the capacity of the new battery, when known.
	ReferenceKwh *float64 `json:"reference_kwh,omitempty"`
	// EstimatedCapacityKwh is the usable capacity estimated from the most recent charging sessions.
	EstimatedCapacityKwh *float64 `json:"estimated_capacity_kwh,omitempty"`
}

// ResidualValuePoint is the projected value of a vehicle after a number of months of ownership.
type ResidualValuePoint struct {
	Month int         `json:"month"`
	Value money.Cents `json:"value"`
}

// ResidualValue is the projected resale value of a vehicle, derived from the user's own purchase price and
// expected resale value, then adjusted for the actual age, distance and battery state of health.
type ResidualValue struct {
	PurchasePrice money.Cents `json:"purchase_price"`
	ExpectedValue money.Cents `json:"expected_resale_value"`
	HoldingMonths int         `json:"holding_months"`
	AgeMonths     float64     `json:"age_months"`
	DistanceKm    float64     `json:"distance_km"`
	// Progress is the share of the planned depreciation already consumed (0 = new, 1 = at the planned resale date).
	Progress float64 `json:"progress"`
	// HealthFactor multiplies the value; 1 when the state of health is unknown or ignored.
	HealthFactor  float64  `json:"health_factor"`
	HealthPercent *float64 `json:"health_percent,omitempty"`
	// CurrentValue is the estimated value today.
	CurrentValue money.Cents `json:"current_value"`
	// Depreciation is the purchase price minus the current value.
	Depreciation money.Cents          `json:"depreciation"`
	Curve        []ResidualValuePoint `json:"curve"`
}
