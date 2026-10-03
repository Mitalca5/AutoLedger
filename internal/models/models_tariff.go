package models

import (
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

const (
	TariffTypeFlat      = "FLAT"
	TariffTypeTimeOfUse = "TIME_OF_USE"
	TariffTypeBands     = "BANDS"

	TimeWindowPeak    = "PEAK"
	TimeWindowOffPeak = "OFFPEAK"
)

// TimeWindow represents a recurring hourly interval within a 24h day.
type TimeWindow struct {
	Start string `json:"start"` // HH:MM
	End   string `json:"end"`   // HH:MM
	Kind  string `json:"kind"`  // PEAK | OFFPEAK
}

// TariffBand is a named price: "peak", "off-peak", "blue", "red"... A plan holds as many as its contract has.
type TariffBand struct {
	Name      string      `json:"name"`
	RateCents money.Cents `json:"rate_cents"` // per kWh
}

// TariffRule assigns a band to a daily interval. Days are 0 (Sunday) to 6 (Saturday); none means every day.
// An interval that ends before it starts crosses midnight, and its days are the ones it starts on.
// Rules are read in order and the first match wins; the plan's default band covers the rest.
type TariffRule struct {
	Days  []int  `json:"days,omitempty"`
	Start string `json:"start"` // HH:MM
	End   string `json:"end"`   // HH:MM
	Band  string `json:"band"`
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
	// Bands, Rules and DefaultBand describe a PlanType BANDS plan; FLAT and TIME_OF_USE plans keep their rates above.
	Bands       []TariffBand `json:"bands"`
	Rules       []TariffRule `json:"rules"`
	DefaultBand string       `json:"default_band"`
	// StandingChargeCents is the fixed monthly subscription of the contract, kept next to the rates for reference.
	StandingChargeCents *money.Cents `json:"standing_charge_cents,omitempty"`
	// ValidFrom and ValidTo (YYYY-MM-DD, inclusive) bound the dates the prices apply to. Plans of one user that
	// share a name are the successive versions of one tariff: the one covering the session date prices it.
	ValidFrom *string   `json:"valid_from,omitempty"`
	ValidTo   *string   `json:"valid_to,omitempty"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AppliesOn reports whether the plan prices the given day (YYYY-MM-DD).
func (p *TariffPlan) AppliesOn(day string) bool {
	if p.ValidFrom != nil && day < *p.ValidFrom {
		return false
	}
	return p.ValidTo == nil || day <= *p.ValidTo
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
