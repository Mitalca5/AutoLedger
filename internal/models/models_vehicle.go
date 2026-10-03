package models

import (
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

// VehicleRole represents the role of a user on a vehicle.
type VehicleRole string

const (
	RoleOwner  VehicleRole = "OWNER"
	RoleEditor VehicleRole = "EDITOR"
	RoleViewer VehicleRole = "VIEWER"
)

func (r VehicleRole) IsValid() bool {
	return r == RoleOwner || r == RoleEditor || r == RoleViewer
}

func (r VehicleRole) CanEdit() bool {
	return r == RoleOwner || r == RoleEditor
}

// Vehicle represents an automobile owned by a user.
type Vehicle struct {
	ID                       string      `json:"id"`
	UserID                   string      `json:"user_id"`
	Role                     VehicleRole `json:"role,omitempty"`
	Name                     string      `json:"name"`
	Vin                      *string     `json:"vin,omitempty"`
	TeslaMateCarID           *int        `json:"teslamate_car_id,omitempty"`
	CurrentOdometer          float64     `json:"current_odometer"`
	TeslaMateAPIURL          *string     `json:"teslamate_api_url,omitempty"`
	TeslaMateAuthType        AuthMode    `json:"teslamate_auth_type"`
	TeslaMateAPIKeyEncrypted *string     `json:"-"`
	TeslaMateBasicUser       *string     `json:"teslamate_basic_user,omitempty"`
	TeslaMateBasicPassEnc    *string     `json:"-"`
	EstimatedKwh100km        *float64    `json:"estimated_kwh_100km,omitempty"`
	EstimatedPricePerKwh     *float64    `json:"estimated_price_per_kwh,omitempty"`
	Currency                 string      `json:"currency"`                        // ISO 4217 code, fixed at creation: see CLAUDE.md
	Powertrain               string      `json:"powertrain"`                      // PowertrainEV | PowertrainICE
	TelemetryMode            string      `json:"telemetry_mode"`                  // Derived by DerivedTelemetryMode, never entered
	Make                     string      `json:"make"`                            // Free text, empty when not given
	Model                    string      `json:"model"`                           // Free text, empty when not given
	TeslaMateGrafanaURL      *string     `json:"teslamate_grafana_url,omitempty"` // Grafana serving the TeslaMate dashboards, to link drives
	DefaultDriverID          *string     `json:"default_driver_id,omitempty"`
	TariffPlanID             *string     `json:"tariff_plan_id,omitempty"`
	IsHomeChargerDefault     bool        `json:"is_home_charger_default"`
	CreatedAt                time.Time   `json:"created_at"`
	UpdatedAt                time.Time   `json:"updated_at"`
}

// Telemetry modes describe how a vehicle's data arrives. The server derives the mode from the vehicle's
// configuration (DerivedTelemetryMode); it is not an input.
const (
	TelemetryConnected = "CONNECTED"
	TelemetrySemiAuto  = "SEMI_AUTO"
	TelemetryManual    = "MANUAL"
)

// DerivedTelemetryMode is CONNECTED for an electric vehicle with a teslamateapi URL (the condition the
// synchronization and the frontend's hasTeslaMate use), MANUAL otherwise.
func (v *Vehicle) DerivedTelemetryMode() string {
	if v.CanLinkTeslaMate() && v.TeslaMateAPIURL != nil && strings.TrimSpace(*v.TeslaMateAPIURL) != "" {
		return TelemetryConnected
	}
	return TelemetryManual
}

// Vehicle powertrains. ICE vehicles are tracked manually (fuel fill-ups) and have no TeslaMate link.
const (
	PowertrainEV  = "EV"
	PowertrainICE = "ICE"
)

// What a powertrain can record. Callers ask for a capability instead of comparing the powertrain, so a new
// powertrain only has to be described here.

// PowertrainCanCharge reports whether the powertrain has a battery to plug in and so records charging sessions.
func PowertrainCanCharge(powertrain string) bool { return powertrain != PowertrainICE }

// PowertrainCanRefuel reports whether the powertrain burns fuel and so records fill-ups.
func PowertrainCanRefuel(powertrain string) bool { return powertrain == PowertrainICE }

// PowertrainCanLinkTeslaMate reports whether a teslamateapi connection makes sense for the powertrain.
func PowertrainCanLinkTeslaMate(powertrain string) bool { return PowertrainCanCharge(powertrain) }

func (v *Vehicle) CanCharge() bool        { return PowertrainCanCharge(v.Powertrain) }
func (v *Vehicle) CanRefuel() bool        { return PowertrainCanRefuel(v.Powertrain) }
func (v *Vehicle) CanLinkTeslaMate() bool { return PowertrainCanLinkTeslaMate(v.Powertrain) }

// VehicleMember represents a user who has access to a vehicle with a specific role.
type VehicleMember struct {
	VehicleID   string      `json:"vehicle_id"`
	UserID      string      `json:"user_id"`
	Role        VehicleRole `json:"role"`
	UserEmail   string      `json:"user_email"`
	DisplayName *string     `json:"display_name,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type AddVehicleMemberRequest struct {
	Email string      `json:"email"`
	Role  VehicleRole `json:"role"`
}

type UpdateVehicleMemberRoleRequest struct {
	Role VehicleRole `json:"role"`
}

// OdometerCheckpoint represents a manual odometer milestone at a specific date.
type OdometerCheckpoint struct {
	ID        string    `json:"id"`
	VehicleID string    `json:"vehicle_id"`
	Date      time.Time `json:"date"`
	Odometer  float64   `json:"odometer"`
	Notes     *string   `json:"notes,omitempty"`
	// Source is "MANUAL" (entered by a user) or "HA" (reported by the Home Assistant integration).
	Source        string    `json:"source"`
	SourceBatchID *string   `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Odometer checkpoint sources.
const (
	OdometerSourceManual = "MANUAL"
	OdometerSourceHA     = "HA"
)

// Acquisition types of a vehicle.
const (
	AcquisitionCash = "CASH" // Cash purchase
	AcquisitionLoan = "LOAN" // Purchase financed by a loan
	AcquisitionLOA  = "LOA"  // Lease with purchase option
	AcquisitionLLD  = "LLD"  // Long-term rental
)

// VehicleOwnership describes how a vehicle is owned or leased; it generates acquisition and financing costs.
type VehicleOwnership struct {
	VehicleID       string    `json:"vehicle_id"`
	AcquisitionType string    `json:"acquisition_type"`
	StartDate       time.Time `json:"start_date"`
	StartOdometer   *float64  `json:"start_odometer,omitempty"`

	PurchasePrice         *money.Cents `json:"purchase_price,omitempty"`
	PurchaseFees          *money.Cents `json:"purchase_fees,omitempty"`
	Incentives            *money.Cents `json:"incentives,omitempty"`
	ExpectedResaleValue   *money.Cents `json:"expected_resale_value,omitempty"`
	ExpectedHoldingMonths *int         `json:"expected_holding_months,omitempty"`

	LoanAmount           *money.Cents `json:"loan_amount,omitempty"`
	LoanRatePct          *float64     `json:"loan_rate_pct,omitempty"`
	LoanDurationMonths   *int         `json:"loan_duration_months,omitempty"`
	LoanFees             *money.Cents `json:"loan_fees,omitempty"`
	LoanInsuranceMonthly *money.Cents `json:"loan_insurance_monthly,omitempty"`

	LeaseDownPayment         *money.Cents `json:"lease_down_payment,omitempty"`
	LeaseMonthlyRent         *money.Cents `json:"lease_monthly_rent,omitempty"`
	LeaseDurationMonths      *int         `json:"lease_duration_months,omitempty"`
	LeaseFees                *money.Cents `json:"lease_fees,omitempty"`
	LeaseDeposit             *money.Cents `json:"lease_deposit,omitempty"`
	LeaseKmAllowancePerYear  *float64     `json:"lease_km_allowance_per_year,omitempty"`
	LeaseExcessKmPrice       *float64     `json:"lease_excess_km_price,omitempty"`
	LeaseEndFeesEstimate     *money.Cents `json:"lease_end_fees_estimate,omitempty"`
	LeasePurchaseOptionPrice *money.Cents `json:"lease_purchase_option_price,omitempty"`
	LeaseIncludesMaintenance bool         `json:"lease_includes_maintenance"`
	LeaseIncludesInsurance   bool         `json:"lease_includes_insurance"`
	LeaseIncludesTires       bool         `json:"lease_includes_tires"`
	OptionExercisedDate      *time.Time   `json:"option_exercised_date,omitempty"`

	EndDate   *time.Time   `json:"end_date,omitempty"`
	SalePrice *money.Cents `json:"sale_price,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsLease reports whether the vehicle is under a LOA or LLD contract.
func (o *VehicleOwnership) IsLease() bool {
	return o != nil && (o.AcquisitionType == AcquisitionLOA || o.AcquisitionType == AcquisitionLLD)
}

// InLeasePhase reports whether the lease contract is still the ownership mode at a given time
// (not ended, not returned, purchase option not exercised).
func (o *VehicleOwnership) InLeasePhase(at time.Time) bool {
	if !o.IsLease() || o.LeaseDurationMonths == nil {
		return false
	}
	if o.OptionExercisedDate != nil && !at.Before(*o.OptionExercisedDate) {
		return false
	}
	if o.EndDate != nil && !at.Before(*o.EndDate) {
		return false
	}
	return at.Before(o.StartDate.AddDate(0, *o.LeaseDurationMonths, 0))
}
