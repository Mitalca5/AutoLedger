package services

import (
	"math"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/teslamate"
)

func fptr(v float64) *float64 { return &v }

func TestBuildChargeStoresBatteryLevelsAndCelsius(t *testing.T) {
	start := time.Date(2026, 2, 3, 18, 0, 0, 0, time.UTC)
	tc := teslamate.Charge{
		ChargeID: 7, StartDate: start.Format(time.RFC3339), EndDate: start.Add(4 * time.Hour).Format(time.RFC3339),
		ChargeEnergyAdded: 30.5, OutsideTempAvg: fptr(41),
		BatteryDetails: teslamate.BatteryDetails{StartBatteryLevel: 22, EndBatteryLevel: 71},
	}

	// TeslaMate configured in Fahrenheit: 41 F is 5 C
	c := buildCharge("v1", "EUR", tc, &teslamate.Units{UnitOfLength: "km", UnitOfTemperature: "F"}, start)
	if c.StartBatteryLevel == nil || *c.StartBatteryLevel != 22 || c.EndBatteryLevel == nil || *c.EndBatteryLevel != 71 {
		t.Errorf("battery levels: %v %v", c.StartBatteryLevel, c.EndBatteryLevel)
	}
	if c.OutsideTempC == nil || *c.OutsideTempC != 5 {
		t.Errorf("temperature: %v", c.OutsideTempC)
	}

	// Celsius is kept as is, and an unknown temperature stays unknown
	tc.OutsideTempAvg = fptr(7.34)
	if c = buildCharge("v1", "EUR", tc, &teslamate.Units{UnitOfTemperature: "C"}, start); c.OutsideTempC == nil || *c.OutsideTempC != 7.3 {
		t.Errorf("rounded Celsius: %v", c.OutsideTempC)
	}
	tc.OutsideTempAvg = nil
	if c = buildCharge("v1", "EUR", tc, nil, start); c.OutsideTempC != nil {
		t.Errorf("unknown temperature: got %v", *c.OutsideTempC)
	}
}

func TestBuildChargeWithoutEndLevelHasNoLevels(t *testing.T) {
	start := time.Date(2026, 2, 3, 18, 0, 0, 0, time.UTC)
	// TeslaMateApi reports 0 for a level it does not know: storing 0 % would fake a huge state-of-charge swing
	tc := teslamate.Charge{ChargeID: 8, ChargeEnergyAdded: 10, BatteryDetails: teslamate.BatteryDetails{StartBatteryLevel: 40}}
	c := buildCharge("v1", "EUR", tc, nil, start)
	if c.StartBatteryLevel != nil || c.EndBatteryLevel != nil {
		t.Errorf("levels must be unknown, got %v %v", c.StartBatteryLevel, c.EndBatteryLevel)
	}
}

func TestBuildDriveStoresBatteryLevelsAndCelsius(t *testing.T) {
	start := time.Date(2026, 2, 3, 8, 0, 0, 0, time.UTC)
	td := teslamate.Drive{
		DriveID: 3, OdometerDetails: teslamate.OdometerDetails{OdometerStart: 1, OdometerEnd: 41, OdometerDistance: 40},
		OutsideTempAvg: fptr(23), // 23 F is -5 C
		BatteryDetails: teslamate.BatteryDetails{StartBatteryLevel: 81, EndBatteryLevel: 71},
	}
	d := buildDrive("v1", td, &teslamate.Units{UnitOfLength: "km", UnitOfTemperature: "F"}, start, start.Add(time.Hour))
	if d.OutsideTempC == nil || *d.OutsideTempC != -5 {
		t.Errorf("temperature: %v", d.OutsideTempC)
	}
	if d.StartBatteryLevel == nil || *d.StartBatteryLevel != 81 || d.EndBatteryLevel == nil || *d.EndBatteryLevel != 71 {
		t.Errorf("battery levels: %v %v", d.StartBatteryLevel, d.EndBatteryLevel)
	}
}

func TestBuildDriveConsumptionIsDerivedFromEnergyAndDistanceInKm(t *testing.T) {
	start := time.Date(2026, 10, 13, 22, 20, 0, 0, time.UTC)
	cases := []struct {
		name   string
		td     teslamate.Drive
		units  *teslamate.Units
		wantKm *float64
	}{
		{
			name: "km",
			td: teslamate.Drive{
				OdometerDetails:   teslamate.OdometerDetails{OdometerDistance: 8.3},
				EnergyConsumedNet: fptr(1.2),
			},
			units:  &teslamate.Units{UnitOfLength: "km"},
			wantKm: fptr(14.457831325301205),
		},
		{
			name: "miles",
			td: teslamate.Drive{
				OdometerDetails:   teslamate.OdometerDetails{OdometerDistance: 10},
				EnergyConsumedNet: fptr(3.2186),
			},
			units:  &teslamate.Units{UnitOfLength: "mi"},
			wantKm: fptr(20), // 10 mi is 16.0934 km
		},
		{
			name: "no energy",
			td:   teslamate.Drive{OdometerDetails: teslamate.OdometerDetails{OdometerDistance: 8.3}},
		},
		{
			name: "no distance",
			td:   teslamate.Drive{EnergyConsumedNet: fptr(1.2)},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := buildDrive("v1", c.td, c.units, start, start.Add(11*time.Minute))
			switch {
			case c.wantKm == nil && d.ConsumptionKwh100km != nil:
				t.Fatalf("consumption = %v, want none", *d.ConsumptionKwh100km)
			case c.wantKm != nil && d.ConsumptionKwh100km == nil:
				t.Fatalf("consumption missing, want %v", *c.wantKm)
			case c.wantKm != nil && math.Abs(*d.ConsumptionKwh100km-*c.wantKm) > 0.01:
				t.Fatalf("consumption = %v, want %v", *d.ConsumptionKwh100km, *c.wantKm)
			}
		})
	}
}
