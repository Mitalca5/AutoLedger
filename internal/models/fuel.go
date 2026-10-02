package models

import (
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

// FuelLog is a manually entered fuel fill-up of a combustion vehicle.
// Amount is the minimum; the odometer is optional (it is then estimated from the odometer readings)
// and Liters (or price per liter) enables consumption figures.
type FuelLog struct {
	ID            string      `json:"id"`
	VehicleID     string      `json:"vehicle_id"`
	Date          time.Time   `json:"date"`
	Odometer      *float64    `json:"odometer,omitempty"`
	Amount        money.Cents `json:"amount"`
	Liters        *float64    `json:"liters,omitempty"`
	PricePerLiter *float64    `json:"price_per_liter,omitempty"`
	FuelType      *string     `json:"fuel_type,omitempty"`
	IsFullTank    bool        `json:"is_full_tank"`
	Notes         *string     `json:"notes,omitempty"`
	SourceBatchID *string     `json:"-"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// OdometerPoint is a manually entered odometer value: a reading or a fill-up that carries a mileage.
type OdometerPoint struct {
	ID       string    `json:"id"`
	Kind     string    `json:"kind"` // OdometerPointReading | OdometerPointFuel
	Date     time.Time `json:"date"`
	Odometer float64   `json:"odometer"`
}

// Kinds of manual odometer points.
const (
	OdometerPointReading = "READING"
	OdometerPointFuel    = "FUEL"
)
