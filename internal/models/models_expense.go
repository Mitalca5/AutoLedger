package models

import (
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

// MaintenanceExpense represents vehicle maintenance, insurance, subscriptions, etc.
type MaintenanceExpense struct {
	ID                       string      `json:"id"`
	VehicleID                string      `json:"vehicle_id"`
	Category                 string      `json:"category"`
	Amount                   money.Cents `json:"amount"`
	Currency                 string      `json:"currency"`
	FxRate                   *float64    `json:"fx_rate,omitempty"`
	Date                     time.Time   `json:"date"`
	Odometer                 *float64    `json:"odometer,omitempty"`
	IsRecurring              bool        `json:"is_recurring"`
	RecurrenceIntervalMonths *int        `json:"recurrence_interval_months,omitempty"`
	RecurrenceEndDate        *time.Time  `json:"recurrence_end_date,omitempty"`
	AmortizationMode         string      `json:"amortization_mode"`
	CoverageKm               *float64    `json:"coverage_km,omitempty"`
	CoverageMonths           *int        `json:"coverage_months,omitempty"`
	ClosesMaintenanceID      *string     `json:"closes_maintenance_id,omitempty"`
	Description              string      `json:"description"`
	DocumentID               *string     `json:"document_id,omitempty"`
	DocumentFilename         *string     `json:"document_filename,omitempty"`
	CreatedAt                time.Time   `json:"created_at"`
	UpdatedAt                time.Time   `json:"updated_at"`
}

// ChargeLog records an EV charging event with costs and kWh.
type ChargeLog struct {
	ID                string       `json:"id"`
	VehicleID         string       `json:"vehicle_id"`
	TeslaMateChargeID *int         `json:"teslamate_charge_id,omitempty"`
	Date              time.Time    `json:"date"`
	EndDate           *time.Time   `json:"end_date,omitempty"`
	Address           *string      `json:"address,omitempty"`
	KwhAdded          float64      `json:"kwh_added"`
	KwhUsed           *float64     `json:"kwh_used,omitempty"`
	Cost              *money.Cents `json:"cost"`        // nil = unknown cost (to be completed)
	CostSource        string       `json:"cost_source"` // TESLAMATE | MANUAL
	Currency          string       `json:"currency"`
	FxRate            *float64     `json:"fx_rate,omitempty"`
	Odometer          *float64     `json:"odometer,omitempty"`
	// Stored by the synchronization and read by the energy statistics, not exposed with the charge.
	StartBatteryLevel *int     `json:"-"`
	EndBatteryLevel   *int     `json:"-"`
	OutsideTempC      *float64 `json:"-"`
	IsManual          bool     `json:"is_manual"`
	Notes             *string  `json:"notes,omitempty"`
	DocumentID        *string  `json:"document_id,omitempty"`
	// ExternalID is the event_id an integration sent with the charge, to recognise a resent event.
	ExternalID *string `json:"-"`
	// Origin is the source that wrote the charge (TESLAMATE, WEBHOOK, CSV, MANUAL); empty means MANUAL on insert.
	Origin           string    `json:"-"`
	DocumentFilename *string   `json:"document_filename,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// ExpenseDocument represents a file attachment or invoice stored on the filesystem volume.
type ExpenseDocument struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	VehicleID   string    `json:"vehicle_id"`
	Filename    string    `json:"filename"`
	MimeType    string    `json:"mime_type"`
	FileSize    int64     `json:"file_size"`
	StoragePath *string   `json:"-"` // Relative path on the Docker volume (vehicleID/docID)
	Data        []byte    `json:"-"` // Legacy binary data from PostgreSQL (used for migration & fallback)
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ExpenseDocumentHeader represents document metadata without binary payload.
type ExpenseDocumentHeader struct {
	ID                  string    `json:"id"`
	VehicleID           string    `json:"vehicle_id"`
	Filename            string    `json:"filename"`
	MimeType            string    `json:"mime_type"`
	FileSize            int64     `json:"file_size"`
	Description         *string   `json:"description,omitempty"`
	LinkedExpensesCount int       `json:"linked_expenses_count"`
	CreatedAt           time.Time `json:"created_at"`
}

// MaintenanceReminder represents a recurring or scheduled maintenance task.
type MaintenanceReminder struct {
	ID                   string     `json:"id"`
	VehicleID            string     `json:"vehicle_id"`
	Title                string     `json:"title"`
	Category             string     `json:"category"` // MAINTENANCE, TIRES, INSPECTION, OTHER
	IntervalKm           *int       `json:"interval_km,omitempty"`
	IntervalMonths       *int       `json:"interval_months,omitempty"`
	LastServiceOdometer  *float64   `json:"last_service_odometer,omitempty"`
	LastServiceDate      *time.Time `json:"last_service_date,omitempty"`
	LeadKm               int        `json:"lead_km"`
	LeadDays             int        `json:"lead_days"`
	WebhookEnabled       bool       `json:"webhook_enabled"`
	LastNotifiedAt       *time.Time `json:"last_notified_at,omitempty"`
	LastNotifiedOdometer *float64   `json:"last_notified_odometer,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`

	// Computed dynamic status
	Status        string     `json:"status"` // OK, DUE_SOON, OVERDUE
	RemainingKm   *float64   `json:"remaining_km,omitempty"`
	RemainingDays *int       `json:"remaining_days,omitempty"`
	DueOdometer   *float64   `json:"due_odometer,omitempty"`
	DueDate       *time.Time `json:"due_date,omitempty"`
}

// ComputeStatus calculates the status (OK, DUE_SOON, OVERDUE) and remaining km/days.
func (r *MaintenanceReminder) ComputeStatus(currentOdometer float64, now time.Time) {
	r.Status = "OK"

	// 1. Kilométrage
	if r.IntervalKm != nil && *r.IntervalKm > 0 {
		baseOdo := 0.0
		if r.LastServiceOdometer != nil {
			baseOdo = *r.LastServiceOdometer
		}
		dueOdo := baseOdo + float64(*r.IntervalKm)
		r.DueOdometer = &dueOdo

		remKm := dueOdo - currentOdometer
		r.RemainingKm = &remKm

		if remKm <= 0 {
			r.Status = "OVERDUE"
		} else if remKm <= float64(r.LeadKm) {
			r.Status = "DUE_SOON"
		}
	}

	// 2. Date
	if r.IntervalMonths != nil && *r.IntervalMonths > 0 {
		baseDate := r.CreatedAt
		if r.LastServiceDate != nil {
			baseDate = *r.LastServiceDate
		}
		dueDate := baseDate.AddDate(0, *r.IntervalMonths, 0)
		r.DueDate = &dueDate

		remDays := int(dueDate.Sub(now).Hours() / 24)
		r.RemainingDays = &remDays

		if remDays <= 0 {
			r.Status = "OVERDUE"
		} else if remDays <= r.LeadDays {
			if r.Status != "OVERDUE" {
				r.Status = "DUE_SOON"
			}
		}
	}
}

// VehicleWebhook holds outgoing homelab webhook settings for notifications.
type VehicleWebhook struct {
	ID        string    `json:"id"`
	VehicleID string    `json:"vehicle_id"`
	URL       string    `json:"url"`
	Type      string    `json:"type"` // DISCORD | TELEGRAM | GOTIFY | GENERIC
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
