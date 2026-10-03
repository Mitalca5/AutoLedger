package handlers

import (
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func cents(v float64) *money.Cents { c := money.FromFloat(v); return &c }
func f64(v float64) *float64       { return &v }

func TestBuildFuelLog(t *testing.T) {
	falseV := false
	tests := []struct {
		name    string
		req     SaveFuelLogRequest
		wantErr string
		check   func(t *testing.T, f *models.FuelLog)
	}{
		{
			name: "amount only",
			req:  SaveFuelLogRequest{Date: "2026-03-01", Odometer: f64(12000), Amount: cents(75)},
			check: func(t *testing.T, f *models.FuelLog) {
				if f.Amount != money.FromFloat(75) || f.Liters != nil || f.PricePerLiter != nil || !f.IsFullTank {
					t.Errorf("unexpected: %+v", f)
				}
			},
		},
		{
			name: "amount and liters derive the price",
			req:  SaveFuelLogRequest{Date: "2026-03-01", Odometer: f64(12000), Amount: cents(75), Liters: f64(40)},
			check: func(t *testing.T, f *models.FuelLog) {
				if f.PricePerLiter == nil || *f.PricePerLiter != 1.875 {
					t.Errorf("price = %v, want 1.875", f.PricePerLiter)
				}
			},
		},
		{
			name: "liters and price derive the amount",
			req:  SaveFuelLogRequest{Date: "2026-03-01", Odometer: f64(12000), Liters: f64(40), PricePerLiter: f64(1.8), IsFullTank: &falseV},
			check: func(t *testing.T, f *models.FuelLog) {
				if f.Amount != money.FromFloat(72) || f.IsFullTank {
					t.Errorf("unexpected: %+v", f)
				}
			},
		},
		{
			name: "amount and price derive the liters",
			req:  SaveFuelLogRequest{Date: "2026-03-01", Odometer: f64(12000), Amount: cents(72), PricePerLiter: f64(1.8)},
			check: func(t *testing.T, f *models.FuelLog) {
				if f.Liters == nil || *f.Liters != 40 {
					t.Errorf("liters = %v, want 40", f.Liters)
				}
			},
		},
		{name: "no amount", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: f64(12000)}, wantErr: "fuel.amount_required"},
		{name: "liters without price or amount", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: f64(12000), Liters: f64(40)}, wantErr: "fuel.amount_required"},
		{name: "bad date", req: SaveFuelLogRequest{Date: "nope", Odometer: f64(12000), Amount: cents(50)}, wantErr: "request.invalid_date"},
		{name: "negative odometer", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: f64(-1), Amount: cents(50)}, wantErr: "odometer.range"},
		{name: "huge odometer", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: f64(3_000_000), Amount: cents(50)}, wantErr: "odometer.range"},
		{name: "too many liters", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: f64(1), Amount: cents(50), Liters: f64(900)}, wantErr: "fuel.liters_range"},
		{name: "absurd price", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: f64(1), Amount: cents(50), PricePerLiter: f64(50)}, wantErr: "fuel.price_range"},
		{name: "unknown fuel", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: f64(1), Amount: cents(50), FuelType: strPtr("KEROSENE")}, wantErr: "fuel.type_invalid"},
		{
			name: "known fuel and trimmed notes",
			req:  SaveFuelLogRequest{Date: "2026-03-01", Odometer: f64(1), Amount: cents(50), FuelType: strPtr("DIESEL"), Notes: strPtr("  Leclerc ")},
			check: func(t *testing.T, f *models.FuelLog) {
				if *f.FuelType != "DIESEL" || *f.Notes != "Leclerc" {
					t.Errorf("unexpected: %+v", f)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := buildFuelLog("veh", &tt.req)
			if tt.wantErr != "" {
				if err == nil || errorCode(err) != tt.wantErr {
					t.Fatalf("error = %v, want code %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.check != nil {
				tt.check(t, f)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

func TestCheckOdometerOrder(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC) }
	others := []models.OdometerPoint{
		{ID: "a", Kind: models.OdometerPointReading, Date: day(1), Odometer: 10000},
		{ID: "b", Kind: models.OdometerPointFuel, Date: day(10), Odometer: 10800},
	}
	tests := []struct {
		name    string
		id      string
		date    time.Time
		odo     float64
		wantErr string
	}{
		{"between the two", "", day(5), 10400, ""},
		{"same day as a neighbour is not compared", "", day(10), 10500, ""},
		{"after the last with a higher odometer", "", day(20), 11000, ""},
		{"lower than an older point", "", day(5), 9000, "odometer.inconsistent_older"},
		{"higher than a newer point", "", day(5), 11000, "odometer.inconsistent_newer"},
		{"editing a point ignores itself", "b", day(10), 10500, ""},
		{"editing still checks the others", "b", day(10), 9000, "odometer.inconsistent_older"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkOdometerOrder(others, tt.id, tt.date, tt.odo)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || errorCode(err) != tt.wantErr {
				t.Fatalf("error = %v, want code %q", err, tt.wantErr)
			}
		})
	}
}

func TestBuildFuelLogWithoutMileage(t *testing.T) {
	f, err := buildFuelLog("veh", &SaveFuelLogRequest{Date: "2026-03-01", Amount: cents(60), Liters: f64(35)})
	if err != nil {
		t.Fatal(err)
	}
	if f.Odometer != nil {
		t.Errorf("a fill-up without mileage must keep a nil odometer, got %v", *f.Odometer)
	}
	if _, err := buildFuelLog("veh", &SaveFuelLogRequest{Date: "2026-03-01", Amount: cents(60), Odometer: f64(-5)}); err == nil {
		t.Error("an entered mileage is still validated")
	}
}

func TestValidatePowertrain(t *testing.T) {
	url := "http://teslamate.local"
	empty := "  "
	tests := []struct {
		name       string
		powertrain string
		url        *string
		wantErr    string
	}{
		{"default", "", nil, ""},
		{"EV with TeslaMate", "EV", &url, ""},
		{"ICE without TeslaMate", "ICE", nil, ""},
		{"ICE with blank URL", "ICE", &empty, ""},
		{"ICE with TeslaMate", "ICE", &url, "vehicle.ice_no_teslamate"},
		{"PHEV with TeslaMate", "PHEV", &url, ""},
		{"REEV without TeslaMate", "REEV", nil, ""},
		{"unknown", "HYBRID", nil, "vehicle.powertrain_invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePowertrain(tt.powertrain, tt.url)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || errorCode(err) != tt.wantErr {
				t.Fatalf("error = %v, want code %q", err, tt.wantErr)
			}
		})
	}
}
