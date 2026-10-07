package models

import (
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

// Comparison scenario modes.
const (
	ComparisonModeRetrospective = "RETROSPECTIVE" // tracked-vehicle side comes from the tracked vehicle's real TCO
	ComparisonModeProjection    = "PROJECTION"    // tracked-vehicle side is described by the user
)

// FuelTypes lists the supported ICE fuels.
var FuelTypes = map[string]bool{
	"SP95_E10": true,
	"SP98":     true,
	"DIESEL":   true,
	"E85":      true,
	"GPL":      true,
}

// ICEInputs describes the equivalent combustion vehicle of a comparison.
type ICEInputs struct {
	FuelType          string      `json:"fuel_type"`
	LPer100Km         float64     `json:"l_per_100km"`
	FuelPrice         float64     `json:"fuel_price"` // EUR per litre
	PurchasePrice     money.Cents `json:"purchase_price"`
	ResaleValue       money.Cents `json:"resale_value"` // Expected value at the end of the period
	MaintenanceYearly money.Cents `json:"maintenance_yearly"`
	InsuranceYearly   money.Cents `json:"insurance_yearly"`
	TaxYearly         money.Cents `json:"tax_yearly"`
}

// TrackedInputs describes the electric vehicle of a PROJECTION comparison.
type TrackedInputs struct {
	KwhPer100Km       float64     `json:"kwh_per_100km"`
	EurPerKwh         float64     `json:"eur_per_kwh"`
	PurchasePrice     money.Cents `json:"purchase_price"`
	ResaleValue       money.Cents `json:"resale_value"`
	MaintenanceYearly money.Cents `json:"maintenance_yearly"`
	InsuranceYearly   money.Cents `json:"insurance_yearly"`
	TaxYearly         money.Cents `json:"tax_yearly"`
}

// ScenarioOptions holds the optional refinements of a comparison.
type ScenarioOptions struct {
	FuelInflationPct        float64     `json:"fuel_inflation_pct,omitempty"`
	ElectricityInflationPct float64     `json:"electricity_inflation_pct,omitempty"`
	CostInflationPct        float64     `json:"cost_inflation_pct,omitempty"` // Maintenance, insurance, taxes
	TrackedIncentives       money.Cents `json:"tracked_incentives,omitempty"` // Deducted from the tracked vehicle purchase price (PROJECTION)
}

// ComparisonScenario is a saved EV vs ICE comparison. It is informative only and never changes real data.
type ComparisonScenario struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	VehicleID *string         `json:"vehicle_id,omitempty"` // Reference tracked vehicle (required in RETROSPECTIVE mode)
	Name      string          `json:"name"`
	Mode      string          `json:"mode"`
	AnnualKm  float64         `json:"annual_km"`
	Years     int             `json:"years"`
	ICE       ICEInputs       `json:"ice"`
	Tracked   *TrackedInputs  `json:"tracked,omitempty"`
	Options   ScenarioOptions `json:"options"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
