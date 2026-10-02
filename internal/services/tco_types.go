package services

import (
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

// Cost ledger categories (see the cost_ledger view).
const (
	LedgerEnergy      = "ENERGY"
	LedgerToll        = "TOLL"
	LedgerParking     = "PARKING"
	LedgerTravelOther = "TRAVEL_OTHER"
	LedgerTires       = "TIRES"
	LedgerMaintenance = "MAINTENANCE"
	LedgerRepair      = "REPAIR"
	LedgerInsurance   = "INSURANCE"
	LedgerFinancing   = "FINANCING"
	LedgerTax         = "TAX"
	LedgerSubscr      = "SUBSCRIPTION"
	LedgerAcquisition = "ACQUISITION"
)

// Insurance sources.
const (
	InsuranceSourceRecordedExpenses = "RECORDED_EXPENSES"
	InsuranceSourceIncluded         = "INCLUDED_IN_LEASE"
	InsuranceSourceInsufficientKm   = "INSUFFICIENT_DISTANCE"
	InsuranceSourceNone             = "NONE"
)

// MonthlyCost represents monthly expenditure (cash basis, acquisition excluded) and mileage.
type MonthlyCost struct {
	Month                string      `json:"month"`                         // YYYY-MM
	DistanceKm           float64     `json:"distance_km"`                   // Total effective distance (tracked + smoothed)
	TrackedDistanceKm    float64     `json:"tracked_distance_km"`           // Exact GPS drives distance
	SmoothedKm           float64     `json:"smoothed_km"`                   // Linearly smoothed / interpolated distance
	EstimatedEnergyKm    float64     `json:"estimated_energy_km,omitempty"` // Distance driven before tracking started
	SmoothedKwh          float64     `json:"smoothed_kwh,omitempty"`        // Estimated kWh for the untracked distance
	SmoothedEnergy       money.Cents `json:"smoothed_energy,omitempty"`     // Estimated energy cost for the untracked distance
	Energy               money.Cents `json:"energy"`
	Tolls                money.Cents `json:"tolls"`
	Maintenance          money.Cents `json:"maintenance"`
	MaintenanceAmortized money.Cents `json:"maintenance_amortized"`
	Insurance            money.Cents `json:"insurance"`
	Financing            money.Cents `json:"financing"`
	FinancingAmortized   money.Cents `json:"financing_amortized"`
	Other                money.Cents `json:"other"`           // Subscriptions, taxes, accessories, other
	Tires                money.Cents `json:"tires"`           // Cash basis: full price in the purchase month
	TiresAmortized       money.Cents `json:"tires_amortized"` // Prorated by km driven while mounted, used for cost_per_km
	Total                money.Cents `json:"total"`
	CostPerKm            float64     `json:"cost_per_km"`
}

// TagCostBreakdown represents costs split by tag (e.g. Pro vs Perso).
type TagCostBreakdown struct {
	Tag         string      `json:"tag"` // empty for the drives that carry no tag
	DistanceKm  float64     `json:"distance_km"`
	EnergyKwh   float64     `json:"energy_kwh"`
	TollsAmount money.Cents `json:"tolls_amount"`
	Percentage  float64     `json:"percentage"`
}

// TCOCompleteness lists the known gaps of the TCO figures.
type TCOCompleteness struct {
	IsComplete          bool     `json:"is_complete"`
	ChargesWithoutCost  int      `json:"charges_without_cost"`
	KwhWithoutCost      float64  `json:"kwh_without_cost"`
	UnconvertedExpenses int      `json:"unconverted_expenses"`
	UnqualifiedDrives   int      `json:"unqualified_drives"`
	UntrackedDistanceKm float64  `json:"untracked_distance_km"`
	OdometerGaps        int      `json:"odometer_gaps"`
	OdometerAnomalies   int      `json:"odometer_anomalies"`
	InsuranceMissing    bool     `json:"insurance_missing"`
	AcquisitionMissing  bool     `json:"acquisition_missing"`
	Warnings            []string `json:"warnings"`
	// ScorePct is a weighted completeness score (0-100) over the dimensions below.
	ScorePct   int                     `json:"score_pct"`
	Dimensions []CompletenessDimension `json:"dimensions"`
}

// CompletenessDimension is one weighted component of the completeness score.
type CompletenessDimension struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	ScorePct int     `json:"score_pct"`
	Weight   float64 `json:"weight"`
	// Applicable is false when there is nothing to evaluate for the dimension; it then carries no weight.
	Applicable bool `json:"applicable"`
}

// TCOSummary represents the global TCO calculation, built from the cost_ledger view.
//
//   - TotalCost: running costs actually paid (tires at purchase), acquisition excluded.
//   - FullCost: economic cost of ownership: running costs with tires amortized per km, plus depreciation.
//   - Per-km figures use DistanceBasisKm: the largest of the distance tracked by drives, the odometer span
//     of those drives and the distance driven since acquisition.
//   - UsageCostPerKm is the marginal cost of driving (energy + tolls/parking).
type TCOSummary struct {
	TotalDistanceKm    float64     `json:"total_distance_km"`
	OdometerDistanceKm float64     `json:"odometer_distance_km"`
	SmoothedDistanceKm float64     `json:"smoothed_distance_km"`
	DistanceBasisKm    float64     `json:"distance_basis_km"`
	TotalCost          money.Cents `json:"total_cost"`
	TotalCostPerKm     float64     `json:"total_cost_per_km"`
	UsageCostPerKm     float64     `json:"usage_cost_per_km"`
	FullCost           money.Cents `json:"full_cost"`
	FullCostPerKm      float64     `json:"full_cost_per_km"`

	AcquisitionType        string      `json:"acquisition_type"` // CASH | LOAN | LOA | LLD
	AcquisitionCost        money.Cents `json:"acquisition_cost"` // Purchase price and fees net of incentives, exercised LOA option
	DepreciationCost       money.Cents `json:"depreciation_cost"`
	DepreciationCostPerKm  float64     `json:"depreciation_cost_per_km"`
	ContractStartDate      *time.Time  `json:"contract_start_date,omitempty"` // Lease / loan start
	ContractEndDate        *time.Time  `json:"contract_end_date,omitempty"`   // Lease term
	ContractDurationMonths *int        `json:"contract_duration_months,omitempty"`
	OwnershipEndDate       *time.Time  `json:"ownership_end_date,omitempty"` // Sale or return

	LeaseExcessKmCost        money.Cents  `json:"lease_excess_km_cost"`      // Accrued against the pro-rata allowance
	LeaseExcessKmProjected   money.Cents  `json:"lease_excess_km_projected"` // Expected at contract end at the current pace
	LeaseKmDriven            float64      `json:"lease_km_driven"`
	LeaseKmAllowanceToDate   float64      `json:"lease_km_allowance_to_date"`
	LeaseKmAllowanceTotal    *float64     `json:"lease_km_allowance_total,omitempty"`
	LeaseKmAllowancePerYear  *float64     `json:"lease_km_allowance_per_year,omitempty"`
	LeaseMonthlyRent         *money.Cents `json:"lease_monthly_rent,omitempty"`
	LeaseDownPayment         *money.Cents `json:"lease_down_payment,omitempty"`
	LeasePurchaseOptionPrice *money.Cents `json:"lease_purchase_option_price,omitempty"`
	OptionExercisedDate      *time.Time   `json:"option_exercised_date,omitempty"`
	LeaseExcessKmPrice       *float64     `json:"lease_excess_km_price,omitempty"`
	LeaseIncludesMaintenance bool         `json:"lease_includes_maintenance"`
	LeaseIncludesInsurance   bool         `json:"lease_includes_insurance"`
	LeaseIncludesTires       bool         `json:"lease_includes_tires"`

	CarpoolRevenue   money.Cents `json:"carpool_revenue"`
	FullCostNet      money.Cents `json:"full_cost_net"` // Full cost minus carpool revenue
	FullCostNetPerKm float64     `json:"full_cost_net_per_km"`

	EnergyCost      money.Cents `json:"energy_cost"`
	EnergyCostPerKm float64     `json:"energy_cost_per_km"`
	TotalKwhAdded   float64     `json:"total_kwh_added"`
	AvgCostPerKwh   float64     `json:"avg_cost_per_kwh"`

	Powertrain        string   `json:"powertrain"` // EV | ICE
	FuelFillUps       int      `json:"fuel_fill_ups,omitempty"`
	TotalLiters       float64  `json:"total_liters,omitempty"`
	AvgCostPerLiter   float64  `json:"avg_cost_per_liter,omitempty"`
	ConsumptionL100km *float64 `json:"consumption_l_100km,omitempty"` // Measured between full tanks

	EstimatedEnergyDistanceKm float64     `json:"estimated_energy_distance_km,omitempty"`
	EstimatedEnergyKwh        float64     `json:"estimated_energy_kwh,omitempty"`
	EstimatedEnergyCost       money.Cents `json:"estimated_energy_cost,omitempty"`
	EstimatedKwh100km         *float64    `json:"estimated_kwh_100km,omitempty"`
	EstimatedPricePerKwh      *float64    `json:"estimated_price_per_kwh,omitempty"`

	TollsCost      money.Cents `json:"tolls_cost"` // Tolls, parking, ferries
	TollsCostPerKm float64     `json:"tolls_cost_per_km"`

	TiresCost               money.Cents `json:"tires_cost"`
	TiresCostPerKm          float64     `json:"tires_cost_per_km"`
	TiresAmortizedCost      money.Cents `json:"tires_amortized_cost"`
	TiresAmortizedCostPerKm float64     `json:"tires_amortized_cost_per_km"`

	MaintenanceCost      money.Cents `json:"maintenance_cost"`
	MaintenanceCostPerKm float64     `json:"maintenance_cost_per_km"`
	RepairCost           money.Cents `json:"repair_cost"` // Unplanned repairs, insurance deductibles
	RepairCostPerKm      float64     `json:"repair_cost_per_km"`

	InsuranceCost      money.Cents `json:"insurance_cost"`
	InsuranceCostPerKm float64     `json:"insurance_cost_per_km"`
	InsuranceSource    string      `json:"insurance_source"`

	FinancingCost      money.Cents `json:"financing_cost"`      // Cash: rents, down payment, fees, loan interest
	FinancingFullCost  money.Cents `json:"financing_full_cost"` // Prepaid amounts spread, return fees and excess mileage accrued
	FinancingCostPerKm float64     `json:"financing_cost_per_km"`

	SubscriptionCost money.Cents `json:"subscription_cost"`
	TaxCost          money.Cents `json:"tax_cost"`
	OtherCost        money.Cents `json:"other_cost"`
	OtherCostPerKm   float64     `json:"other_cost_per_km"` // Subscriptions + taxes + other

	TagBreakdown []TagCostBreakdown `json:"tag_breakdown"`
	MonthlyCosts []MonthlyCost      `json:"monthly_costs"`

	Completeness TCOCompleteness `json:"completeness"`
}
