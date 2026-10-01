package models

import (
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

const (
	TariffTypeFlat      = "FLAT"
	TariffTypeTimeOfUse = "TIME_OF_USE"

	TimeWindowPeak    = "PEAK"
	TimeWindowOffPeak = "OFFPEAK"
)

// TimeWindow represents a recurring hourly interval within a 24h day.
type TimeWindow struct {
	Start string `json:"start"` // HH:MM
	End   string `json:"end"`   // HH:MM
	Kind  string `json:"kind"`  // PEAK | OFFPEAK
}

// TariffPlan defines electricity rate rules for residential and scheduled charging.
type TariffPlan struct {
	ID               string       `json:"id"`
	UserID           string       `json:"user_id"`
	Name             string       `json:"name"`
	PlanType         string       `json:"plan_type"` // FLAT | TIME_OF_USE
	Currency         string       `json:"currency"`
	FlatRateCents    *money.Cents `json:"flat_rate_cents,omitempty"`
	PeakRateCents    *money.Cents `json:"peak_rate_cents,omitempty"`
	OffpeakRateCents *money.Cents `json:"offpeak_rate_cents,omitempty"`
	TimeWindows      []TimeWindow `json:"time_windows"`
	IsDefault        bool         `json:"is_default"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

// PublicChargingPreset represents a reusable pricing model for a public charging network.
type PublicChargingPreset struct {
	ID               string      `json:"id"`
	UserID           string      `json:"user_id"`
	Name             string      `json:"name"`
	ConnectionFee    money.Cents `json:"connection_fee"`
	PricePerKwh      money.Cents `json:"price_per_kwh"`
	PricePerMinute   money.Cents `json:"price_per_minute"`
	IdleFeePerMinute money.Cents `json:"idle_fee_per_minute"`
	IdleGraceMinutes int         `json:"idle_grace_minutes"`
	Currency         string      `json:"currency"`
	CreatedAt        time.Time   `json:"created_at"`
}

// PublicChargingCalculationRequest inputs for computing broken-down public charging costs.
type PublicChargingCalculationRequest struct {
	Kwh                 float64     `json:"kwh"`
	ChargingMinutes     int         `json:"charging_minutes"`
	TotalPluggedMinutes int         `json:"total_plugged_minutes"`
	ConnectionFee       money.Cents `json:"connection_fee"`
	PricePerKwh         money.Cents `json:"price_per_kwh"`
	PricePerMinute      money.Cents `json:"price_per_minute"`
	IdleFeePerMinute    money.Cents `json:"idle_fee_per_minute"`
	IdleGraceMinutes    int         `json:"idle_grace_minutes"`
}

// PublicChargingBreakdown details the calculated parts of a public charge.
type PublicChargingBreakdown struct {
	ConnectionCost money.Cents `json:"connection_cost"`
	EnergyCost     money.Cents `json:"energy_cost"`
	DurationCost   money.Cents `json:"duration_cost"`
	IdleMinutes    int         `json:"idle_minutes"`
	IdleCost       money.Cents `json:"idle_cost"`
	TotalCost      money.Cents `json:"total_cost"`
}
