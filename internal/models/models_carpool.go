package models

import (
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

// CarpoolTrip represents a shared trip with detailed real cost breakdown and passenger revenues.
type CarpoolTrip struct {
	ID              string      `json:"id"`
	VehicleID       string      `json:"vehicle_id"`
	DriveID         *string     `json:"drive_id,omitempty"`
	TripGroupID     *string     `json:"trip_group_id,omitempty"`
	Title           string      `json:"title"`
	Date            time.Time   `json:"date"`
	DistanceKm      float64     `json:"distance_km"`
	ElectricityCost money.Cents `json:"electricity_cost"`
	TollsCost       money.Cents `json:"tolls_cost"`
	TiresCost       money.Cents `json:"tires_cost"`
	MaintenanceCost money.Cents `json:"maintenance_cost"`
	InsuranceCost   money.Cents `json:"insurance_cost"`
	OtherCost       money.Cents `json:"other_cost"`
	TotalCost       money.Cents `json:"total_cost"`
	TotalRevenue    money.Cents `json:"total_revenue"`
	NetCost         money.Cents `json:"net_cost"`
	Notes           *string     `json:"notes,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

// CarpoolPassenger represents a booking / passenger contribution for a full trip or sub-leg ("bout de trajet").
type CarpoolPassenger struct {
	ID            string      `json:"id"`
	CarpoolTripID string      `json:"carpool_trip_id"`
	PassengerName string      `json:"passenger_name"`
	Origin        *string     `json:"origin,omitempty"`
	Destination   *string     `json:"destination,omitempty"`
	Seats         int         `json:"seats"`
	AmountPaid    money.Cents `json:"amount_paid"`
	Notes         *string     `json:"notes,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`

	// Stops where the passenger boards and alights (leg i goes from stop i to stop i + 1)
	BoardStopIndex  int `json:"board_stop_index"`
	AlightStopIndex int `json:"alight_stop_index"`

	// Computed: fair share of the costs of the legs ridden, and amount paid minus that share
	CostShare money.Cents `json:"cost_share"`
	Balance   money.Cents `json:"balance"`
}

// CarpoolLeg is one leg of a carpool trip, usually one TeslaMate drive, with its own costs.
type CarpoolLeg struct {
	ID              string      `json:"id"`
	CarpoolTripID   string      `json:"carpool_trip_id"`
	OrderIndex      int         `json:"order_index"`
	DriveID         *string     `json:"drive_id,omitempty"`
	StartLabel      *string     `json:"start_label,omitempty"`
	EndLabel        *string     `json:"end_label,omitempty"`
	DistanceKm      float64     `json:"distance_km"`
	ElectricityCost money.Cents `json:"electricity_cost"`
	TollsCost       money.Cents `json:"tolls_cost"`
	TiresCost       money.Cents `json:"tires_cost"`
	MaintenanceCost money.Cents `json:"maintenance_cost"`
	InsuranceCost   money.Cents `json:"insurance_cost"`
	OtherCost       money.Cents `json:"other_cost"`

	// Computed
	TotalCost      money.Cents `json:"total_cost"`
	PassengerSeats int         `json:"passenger_seats"` // Seats occupied by passengers on this leg
	CostPerPerson  money.Cents `json:"cost_per_person"` // Driver included
}

// Total returns the sum of the leg cost components.
func (l *CarpoolLeg) Total() money.Cents {
	return l.ElectricityCost + l.TollsCost + l.TiresCost + l.MaintenanceCost + l.InsuranceCost + l.OtherCost
}

// CarpoolTripWithPassengers bundles a trip with its passengers.
type CarpoolTripWithPassengers struct {
	CarpoolTrip
	Legs       []CarpoolLeg       `json:"legs"`
	Passengers []CarpoolPassenger `json:"passengers"`

	// Computed: fair split of the trip costs between the driver and the passengers
	DriverCostShare     money.Cents `json:"driver_cost_share"`
	PassengersCostShare money.Cents `json:"passengers_cost_share"`
}

// CarpoolCostEstimate provides suggested real cost breakdown based on vehicle TCO metrics.
type CarpoolCostEstimate struct {
	StartDate             *time.Time   `json:"start_date,omitempty"`
	DistanceKm            float64      `json:"distance_km"`
	ElectricityCost       money.Cents  `json:"electricity_cost"`
	TollsCost             money.Cents  `json:"tolls_cost"`
	TiresCost             money.Cents  `json:"tires_cost"`
	MaintenanceCost       money.Cents  `json:"maintenance_cost"`
	InsuranceCost         money.Cents  `json:"insurance_cost"`
	OtherCost             money.Cents  `json:"other_cost"`
	TotalCost             money.Cents  `json:"total_cost"`
	ElectricityRatePerKwh float64      `json:"electricity_rate_per_kwh"`
	TiresRatePerKm        float64      `json:"tires_rate_per_km"`
	MaintenanceRatePerKm  float64      `json:"maintenance_rate_per_km"`
	InsuranceRatePerKm    float64      `json:"insurance_rate_per_km"`
	DailyInsuranceCost    *money.Cents `json:"daily_insurance_cost,omitempty"`
	MonthlyInsuranceCost  *money.Cents `json:"monthly_insurance_cost,omitempty"`
	InsuranceSource       string       `json:"insurance_source"` // "VEHICLE_SETTINGS", "RECORDED_EXPENSES", "DEFAULT"
	// Insurance share: insurance paid over the reference window divided by the kilometers driven over it.
	InsuranceWindowCost   *money.Cents `json:"insurance_window_cost,omitempty"`
	InsuranceWindowKm     *float64     `json:"insurance_window_km,omitempty"`
	EnergySource          string       `json:"energy_source"`           // MEASURED | CONSUMPTION | DEFAULT
	ElectricityRateSource string       `json:"electricity_rate_source"` // HISTORY | DEFAULT
	TiresRateSource       string       `json:"tires_rate_source"`       // MOUNTED_TIRES | HISTORY | DEFAULT
	MaintenanceRateSource string       `json:"maintenance_rate_source"` // HISTORY | DEFAULT
	// One estimated leg per drive (chronological), or a single leg for a manual distance
	Legs []CarpoolLeg `json:"legs"`
}

// CarpoolSummary aggregates global carpooling KPIs for the vehicle.
type CarpoolSummary struct {
	TotalTrips      int         `json:"total_trips"`
	TotalPassengers int         `json:"total_passengers"`
	TotalDistanceKm float64     `json:"total_distance_km"`
	TotalRealCost   money.Cents `json:"total_real_cost"`
	TotalRevenue    money.Cents `json:"total_revenue"`
	TotalNetCost    money.Cents `json:"total_net_cost"`
	TotalSaved      money.Cents `json:"total_saved"`
	// CoverageRatePct is the share of the real cost the passengers paid back; nil (null) when there is no cost yet
	CoverageRatePct *float64 `json:"coverage_rate_pct"`
	NetCostPerKm    float64  `json:"net_cost_per_km"`
	// Fair shares over all trips: what passengers should cover given the legs they rode, and the driver's own share
	TotalPassengersShare money.Cents `json:"total_passengers_share"`
	TotalDriverShare     money.Cents `json:"total_driver_share"`
}
