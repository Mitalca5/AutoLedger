package models

import "time"

// PendingCharge represents a charging session recorded from a shared or unassigned charger
// that awaits qualification (assignment to a vehicle and driver) by the user.
type PendingCharge struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Source      string         `json:"source"`
	ChargerName *string        `json:"charger_name,omitempty"`
	StartTime   time.Time      `json:"start_time"`
	EndTime     time.Time      `json:"end_time"`
	EnergyKwh   float64        `json:"energy_kwh"`
	Location    string         `json:"location"`
	RawData     map[string]any `json:"raw_data,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// AssignPendingChargeRequest links a pending charge to a vehicle and optional driver.
type AssignPendingChargeRequest struct {
	VehicleID string  `json:"vehicle_id"`
	DriverID  *string `json:"driver_id,omitempty"`
}
