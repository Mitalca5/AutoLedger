package teslamate

import "time"

// Units holds measurement units returned by TeslaMate.
type Units struct {
	UnitOfLength      string `json:"unit_of_length"`
	UnitOfTemperature string `json:"unit_of_temperature"`
	UnitOfPressure    string `json:"unit_of_pressure"`
}

// CarSummary represents car info in response wrapper.
type CarSummary struct {
	CarID   int    `json:"car_id"`
	CarName string `json:"car_name"`
}

// CarDetails contains vehicle hardware/model details.
type CarDetails struct {
	Model       string `json:"model"`
	TrimBadging string `json:"trim_badging"`
	Vin         string `json:"vin"`
}

// Car represents a car returned by /api/v1/cars.
type Car struct {
	CarID      int        `json:"car_id"`
	Name       string     `json:"name"`
	CarDetails CarDetails `json:"car_details"`
}

// CarsResponse represents the response of /api/v1/cars.
type CarsResponse struct {
	Data struct {
		Cars []Car `json:"cars"`
	} `json:"data"`
}

// StatusDetails contains live telemetry including current odometer.
type StatusDetails struct {
	DisplayName string  `json:"display_name"`
	State       string  `json:"state"`
	StateSince  string  `json:"state_since"`
	Odometer    float64 `json:"odometer"`
}

// StatusResponse represents the response of /api/v1/cars/:CarID/status.
type StatusResponse struct {
	Data struct {
		Car    CarSummary    `json:"car"`
		Status StatusDetails `json:"status"`
		Units  Units         `json:"units"`
	} `json:"data"`
}

// OdometerDetails contains start, end, and distance for a drive.
type OdometerDetails struct {
	OdometerStart    float64 `json:"odometer_start"`
	OdometerEnd      float64 `json:"odometer_end"`
	OdometerDistance float64 `json:"odometer_distance"`
}

// BatteryDetails contains battery state of charge during a drive or charge.
type BatteryDetails struct {
	StartBatteryLevel int `json:"start_battery_level"`
	EndBatteryLevel   int `json:"end_battery_level"`
}

// Drive represents a single drive returned by /api/v1/cars/:CarID/drives.
type Drive struct {
	DriveID           int             `json:"drive_id"`
	StartDate         string          `json:"start_date"`
	EndDate           string          `json:"end_date"`
	StartAddress      string          `json:"start_address"`
	EndAddress        string          `json:"end_address"`
	OdometerDetails   OdometerDetails `json:"odometer_details"`
	DurationMin       int             `json:"duration_min"`
	DurationStr       string          `json:"duration_str"`
	SpeedMax          int             `json:"speed_max"`
	SpeedAvg          float64         `json:"speed_avg"`
	PowerMax          int             `json:"power_max"`
	PowerMin          int             `json:"power_min"`
	EnergyConsumedNet *float64        `json:"energy_consumed_net"`
	ConsumptionNet    *float64        `json:"consumption_net"`
	BatteryDetails    BatteryDetails  `json:"battery_details"`
	OutsideTempAvg    *float64        `json:"outside_temp_avg"` // Unit of Units.UnitOfTemperature, null when unknown
}

// ParsedStartTime parses StartDate into time.Time.
func (d *Drive) ParsedStartTime() (time.Time, error) {
	return time.Parse(time.RFC3339, d.StartDate)
}

// ParsedEndTime parses EndDate into time.Time.
func (d *Drive) ParsedEndTime() (time.Time, error) {
	return time.Parse(time.RFC3339, d.EndDate)
}

// DrivesResponse represents the response of /api/v1/cars/:CarID/drives.
type DrivesResponse struct {
	Data struct {
		Car    CarSummary `json:"car"`
		Drives []Drive    `json:"drives"`
		Units  Units      `json:"units"`
	} `json:"data"`
}

// DrivePosition is a single GPS sample of a drive's route.
type DrivePosition struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Date      string  `json:"date"`
	Speed     float64 `json:"speed"` // Unit of Units.UnitOfLength per hour, 0 when TeslaMate has no speed
}

// DriveDetailResponse represents the response of /api/v1/cars/:CarID/drives/:DriveID,
// which returns the same drive summary as DrivesResponse plus the full GPS trace.
type DriveDetailResponse struct {
	Data struct {
		Drive struct {
			DriveDetails []DrivePosition `json:"drive_details"`
		} `json:"drive"`
		Units Units `json:"units"`
	} `json:"data"`
}

// Charge represents a single charging session returned by /api/v1/cars/:CarID/charges.
type Charge struct {
	ChargeID          int            `json:"charge_id"`
	StartDate         string         `json:"start_date"`
	EndDate           string         `json:"end_date"`
	Address           string         `json:"address"`
	ChargeEnergyAdded float64        `json:"charge_energy_added"`
	ChargeEnergyUsed  float64        `json:"charge_energy_used"`
	Cost              *float64       `json:"cost"` // nil when no tariff is configured in TeslaMate
	DurationMin       int            `json:"duration_min"`
	DurationStr       string         `json:"duration_str"`
	BatteryDetails    BatteryDetails `json:"battery_details"`
	OutsideTempAvg    *float64       `json:"outside_temp_avg"` // Unit of Units.UnitOfTemperature, null when unknown
	Odometer          float64        `json:"odometer"`
	Latitude          float64        `json:"latitude"`
	Longitude         float64        `json:"longitude"`
}

// ParsedStartTime parses StartDate into time.Time.
func (c *Charge) ParsedStartTime() (time.Time, error) {
	return time.Parse(time.RFC3339, c.StartDate)
}

// ParsedEndTime parses EndDate into time.Time.
func (c *Charge) ParsedEndTime() (time.Time, error) {
	return time.Parse(time.RFC3339, c.EndDate)
}

// ChargesResponse represents the response of /api/v1/cars/:CarID/charges.
type ChargesResponse struct {
	Data struct {
		Car     CarSummary `json:"car"`
		Charges []Charge   `json:"charges"`
		Units   Units      `json:"units"`
	} `json:"data"`
}

// DriveFilterOptions holds query options for listing drives.
type DriveFilterOptions struct {
	Page        int
	Show        int
	StartDate   string // YYYY-MM-DD or RFC3339
	EndDate     string // YYYY-MM-DD or RFC3339
	MinDistance float64
}

// ChargeFilterOptions holds query options for listing charges.
type ChargeFilterOptions struct {
	Page      int
	Show      int
	StartDate string // YYYY-MM-DD or RFC3339
	EndDate   string // YYYY-MM-DD or RFC3339
}

// BatteryHealth is the battery state computed by TeslaMateApi from the charging history.
// Capacities are in kWh and the health is the current capacity over the highest one observed.
type BatteryHealth struct {
	MaxCapacity             float64 `json:"max_capacity"`
	CurrentCapacity         float64 `json:"current_capacity"`
	BatteryHealthPercentage float64 `json:"battery_health_percentage"`
}

// BatteryHealthResponse represents the response of /api/v1/cars/:CarID/battery-health.
type BatteryHealthResponse struct {
	Data struct {
		BatteryHealth BatteryHealth `json:"battery_health"`
		Units         Units         `json:"units"`
	} `json:"data"`
}
