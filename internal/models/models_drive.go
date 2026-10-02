package models

import (
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

// Drive represents a single vehicle trip.
type Drive struct {
	ID               string `json:"id"`
	VehicleID        string `json:"vehicle_id"`
	TeslaMateDriveID *int   `json:"teslamate_drive_id,omitempty"`
	// Origin is the source that wrote the drive (TESLAMATE, WEBHOOK, CSV, MANUAL); empty means MANUAL on insert.
	Origin              string    `json:"-"`
	StartTime           time.Time `json:"start_time"`
	EndTime             time.Time `json:"end_time"`
	StartOdometer       *float64  `json:"start_odometer,omitempty"`
	EndOdometer         *float64  `json:"end_odometer,omitempty"`
	DistanceKm          float64   `json:"distance_km"`
	DurationMin         int       `json:"duration_min"`
	SpeedAvg            *float64  `json:"speed_avg,omitempty"`
	SpeedMax            *int      `json:"speed_max,omitempty"`
	PowerMax            *int      `json:"power_max,omitempty"`
	PowerMin            *int      `json:"power_min,omitempty"`
	StartAddress        *string   `json:"start_address,omitempty"`
	EndAddress          *string   `json:"end_address,omitempty"`
	EnergyConsumedKwh   *float64  `json:"energy_consumed_kwh,omitempty"`
	ConsumptionKwh100km *float64  `json:"consumption_kwh_100km,omitempty"`
	// Stored by the synchronization and read by the energy statistics, not exposed with the drive.
	StartBatteryLevel *int     `json:"-"`
	EndBatteryLevel   *int     `json:"-"`
	OutsideTempC      *float64 `json:"-"`
	Tags              []string `json:"tags"`
	DriverID          *string  `json:"driver_id,omitempty"`
	DriverName        *string  `json:"driver_name,omitempty"`
	IsManual          bool     `json:"is_manual"`
	// EnergyEstimated marks an energy derived from the vehicle's average consumption rather than measured or typed.
	EnergyEstimated bool       `json:"energy_estimated"`
	TollReviewedAt  *time.Time `json:"toll_reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// IsHighway returns true if the drive matches the highway detection heuristic. It favours recall:
// a false positive only puts a drive in the toll review queue, a miss hides a toll cost.
//   - long and reasonably fast: distance >= 40 km and speed_avg >= 70 km/h;
//   - over 20 km with a highway top speed: speed_max > 125 km/h;
//   - over 20 km at the reduced limit (rain, works): speed_max >= 110 km/h and speed_avg >= 70 km/h;
//   - a short hop between two exits: distance >= 8 km, speed_max >= 105 km/h and speed_avg >= 70 km/h.
//
// The same rules exist in database.HighwayDrivePredicate and in the drives page (isHighwayDrive).
func (d Drive) IsHighway() bool {
	speedAvg := 0.0
	if d.SpeedAvg != nil {
		speedAvg = *d.SpeedAvg
	}
	speedMax := 0
	if d.SpeedMax != nil {
		speedMax = *d.SpeedMax
	}
	return (d.DistanceKm >= 40 && speedAvg >= 70) ||
		(d.DistanceKm >= 20 && speedMax > 125) ||
		(d.DistanceKm >= 20 && speedMax >= 110 && speedAvg >= 70) ||
		(d.DistanceKm >= 8 && speedMax >= 105 && speedAvg >= 70)
}

// TollSegment is a single toll crossing detected on a drive: either a closed-network
// entry/exit pair, a single open barrier, or a closed-network entry with no exit found
// on this drive's trace (end of trace, GPS gap, ...).
type TollSegment struct {
	Network        string       `json:"network,omitempty"` // OpenTollData network_name (closed networks only)
	Operator       string       `json:"operator,omitempty"`
	Type           string       `json:"type"` // "open" or "close"
	Entry          string       `json:"entry"`
	Exit           *string      `json:"exit,omitempty"`
	EstimatedPrice *money.Cents `json:"estimated_price,omitempty"` // class 1 (light vehicle) estimate
}

// TollDetection is the result of matching a drive's GPS trace against the toll station
// reference dataset, cached per drive so the UI doesn't need to re-run detection every time.
type TollDetection struct {
	ID         string        `json:"id"`
	DriveID    string        `json:"drive_id"`
	VehicleID  string        `json:"vehicle_id"`
	Segments   []TollSegment `json:"segments"`
	DetectedAt time.Time     `json:"detected_at"`
}

// TripGroup allows grouping multiple drives (e.g. holiday trip with stops).
type TripGroup struct {
	ID        string    `json:"id"`
	VehicleID string    `json:"vehicle_id"`
	Name      string    `json:"name"`
	Notes     *string   `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Summary (listing only)
	DriveIDs      []string    `json:"drive_ids"`
	DistanceKm    float64     `json:"distance_km"`
	StartTime     *time.Time  `json:"start_time,omitempty"`
	EndTime       *time.Time  `json:"end_time,omitempty"`
	ExpensesTotal money.Cents `json:"expenses_total"` // EUR, expenses attached to the group itself
	ExpenseCount  int         `json:"expense_count"`  // expenses attached to the group itself
	TollsTotal    money.Cents `json:"tolls_total"`    // EUR, all expenses borne by its drives: attached to a drive or to the group
	CarpoolCount  int         `json:"carpool_count"`
}

// DriveExpense holds costs directly attached to a drive or a trip group (tolls, parking).
type DriveExpense struct {
	ID                string       `json:"id"`
	VehicleID         string       `json:"vehicle_id"`
	TripGroupID       *string      `json:"trip_group_id,omitempty"`
	TripGroupName     *string      `json:"trip_group_name,omitempty"`
	DriveID           *string      `json:"drive_id,omitempty"`
	DriveTitle        *string      `json:"drive_title,omitempty"`
	TripGroupDriveIDs []string     `json:"trip_group_drive_ids,omitempty"`
	Type              string       `json:"type"` // TOLL, PARKING, etc.
	Amount            money.Cents  `json:"amount"`
	Currency          string       `json:"currency"`
	FxRate            *float64     `json:"fx_rate,omitempty"`          // Conversion rate to EUR when Currency != EUR
	AllocatedAmount   *money.Cents `json:"allocated_amount,omitempty"` // Share allocated to a given drive (EUR)
	Date              time.Time    `json:"date"`
	Notes             *string      `json:"notes,omitempty"`
	DocumentID        *string      `json:"document_id,omitempty"`
	DocumentFilename  *string      `json:"document_filename,omitempty"`
	Source            string       `json:"source"` // ExpenseSourceManual or ExpenseSourceAutoToll
	CreatedAt         time.Time    `json:"created_at"`
}

// Expense provenance: entered by the user, or created from a detected toll estimate.
const (
	ExpenseSourceManual   = "MANUAL"
	ExpenseSourceAutoToll = "AUTO_TOLL"
)

// DataQualityIssue reports an odometer continuity problem on synchronized drives.
type DataQualityIssue struct {
	Type            string    `json:"type"` // ODOMETER_GAP | ODOMETER_REGRESSION | DISTANCE_MISMATCH
	DriveID         string    `json:"drive_id"`
	PreviousDriveID *string   `json:"previous_drive_id,omitempty"`
	Date            time.Time `json:"date"`
	Km              float64   `json:"km"`
}

// TripSuggestion is a set of ungrouped consecutive drives that looks like a single trip.
type TripSuggestion struct {
	DriveIDs     []string  `json:"drive_ids"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	DistanceKm   float64   `json:"distance_km"`
	StartAddress string    `json:"start_address"`
	EndAddress   string    `json:"end_address"`
	Reason       string    `json:"reason"` // PAUSE: stops shorter than TripPauseMax; CHARGE: at least one stop with a charge in between
}

// Reasons why consecutive drives are suggested as one trip.
const (
	TripReasonPause  = "PAUSE"
	TripReasonCharge = "CHARGE"
)

// TripCandidateDrive is the part of a drive needed to detect trips.
type TripCandidateDrive struct {
	ID           string
	Start, End   time.Time
	DistanceKm   float64
	StartAddress *string
	EndAddress   *string
	Grouped      bool // already belongs to a trip group
}

// EndOrStart returns the end of the drive, or its start when the end is unknown.
func (d TripCandidateDrive) EndOrStart() time.Time {
	if d.End.IsZero() {
		return d.Start
	}
	return d.End
}

// ChargeWindow is a charging session (End equals Start when its end is unknown).
type ChargeWindow struct {
	Start, End time.Time
}
