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
	// ElectricityPriceOverride replaces the recorded average price per kWh of the tracked vehicle (RETROSPECTIVE),
	// e.g. an off-peak tariff. Zero keeps the recorded price.
	ElectricityPriceOverride float64 `json:"electricity_price_override,omitempty"`
}

// ComparisonScenario is a saved EV vs ICE comparison. It is informative only and never changes real data.
type ComparisonScenario struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	VehicleID *string         `json:"vehicle_id,omitempty"` // Reference tracked vehicle (required in RETROSPECTIVE mode)
	Name      string          `json:"name"`
	Currency  string          `json:"currency"`
	Mode      string          `json:"mode"`
	AnnualKm  float64         `json:"annual_km"`
	Years     int             `json:"years"`
	ICE       ICEInputs       `json:"ice"`
	Tracked   *TrackedInputs  `json:"tracked,omitempty"`
	Options   ScenarioOptions `json:"options"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// ComparisonInputLimits is the API contract shared by validation and the what-if editor.
type ComparisonInputLimits struct {
	AnnualKmMax         float64 `json:"annual_km_max"`
	FuelPriceMax        float64 `json:"fuel_price_max"`
	ElectricityPriceMax float64 `json:"electricity_price_max"`
	YearsMax            int     `json:"years_max"`
	AmountMax           float64 `json:"amount_max"`
	InflationMin        float64 `json:"inflation_min"`
	InflationMax        float64 `json:"inflation_max"`
	// Consumption of the combustion vehicle (L/100 km) and of an electric one described by the user (kWh/100 km)
	IceConsumptionMin float64 `json:"ice_consumption_min"`
	IceConsumptionMax float64 `json:"ice_consumption_max"`
	EVConsumptionMin  float64 `json:"ev_consumption_min"`
	EVConsumptionMax  float64 `json:"ev_consumption_max"`
}

const (
	MaxComparisonAnnualKm     = 200_000
	MaxComparisonYears        = 15
	MinComparisonInflationPct = -10.0
	MaxComparisonInflationPct = 30.0
	MinComparisonConsumption  = 0.1
	MaxComparisonIceLPer100Km = 50.0
	MaxComparisonKwhPer100Km  = 100.0
)

func ComparisonLimits() ComparisonInputLimits {
	return ComparisonInputLimits{
		AnnualKmMax: MaxComparisonAnnualKm, FuelPriceMax: MaxFuelPricePerLiter, ElectricityPriceMax: money.MaxRate.Float(),
		YearsMax: MaxComparisonYears, AmountMax: money.Max.Float(),
		InflationMin: MinComparisonInflationPct, InflationMax: MaxComparisonInflationPct,
		IceConsumptionMin: MinComparisonConsumption, IceConsumptionMax: MaxComparisonIceLPer100Km,
		EVConsumptionMin: MinComparisonConsumption, EVConsumptionMax: MaxComparisonKwhPer100Km,
	}
}
