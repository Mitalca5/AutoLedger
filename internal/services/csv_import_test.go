package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
)

func testVehicle(powertrain string) *models.Vehicle {
	return &models.Vehicle{ID: "veh-1", Powertrain: powertrain, Currency: "EUR"}
}

func previewOf(t *testing.T, vehicle *models.Vehicle, csvData string, opts CSVImportOptions) *CSVPreviewResult {
	t.Helper()
	svc := NewCSVImportService(nil, "Europe/Paris")
	res, err := svc.Preview(context.Background(), vehicle, []byte(csvData), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return res
}

func rowCodes(res *CSVPreviewResult) []string {
	var codes []string
	for _, e := range res.Errors {
		codes = append(codes, e.Code)
	}
	return codes
}

func TestCSVImportPreview(t *testing.T) {
	ev := testVehicle(models.PowertrainEV)

	t.Run("charges csv with comma", func(t *testing.T) {
		res := previewOf(t, ev, "date,kwh,cost,currency,location\n2026-09-15 14:30:00,42.5,18.50,EUR,Ionity Aire\n2026-09-16 10:00:00,20.0,8.00,EUR,Home Wallbox\n", CSVImportOptions{})
		if res.Type != ImportTypeCharges || res.TotalRows != 2 || res.ValidRows != 2 || res.InvalidRows != 0 {
			t.Errorf("unexpected preview: %+v", res)
		}
		if len(res.SampleRows) != 2 {
			t.Errorf("expected 2 sample rows, got %d", len(res.SampleRows))
		}
	})

	t.Run("drives csv with semicolon", func(t *testing.T) {
		res := previewOf(t, ev, "start_time;end_time;distance_km;kwh;start_address;end_address;tag\n2026-09-15 08:00;2026-09-15 08:45;45,5;7,2;Nantes;Rennes;Pro\n", CSVImportOptions{})
		if res.Type != ImportTypeDrives || res.ValidRows != 1 {
			t.Errorf("unexpected preview: %+v", res)
		}
	})

	t.Run("fuel and odometer types are detected", func(t *testing.T) {
		ice := testVehicle(models.PowertrainICE)
		fuel := previewOf(t, ice, "date,liters,price_per_liter,fuel_type\n2026-09-15,40,1.80,SP95_E10\n", CSVImportOptions{})
		if fuel.Type != ImportTypeFuel || fuel.ValidRows != 1 {
			t.Errorf("unexpected fuel preview: %+v", fuel)
		}
		odo := previewOf(t, ice, "date,odometer\n2026-09-15,12345\n", CSVImportOptions{})
		if odo.Type != ImportTypeOdometer || odo.ValidRows != 1 {
			t.Errorf("unexpected odometer preview: %+v", odo)
		}
	})

	t.Run("empty csv returns a coded error", func(t *testing.T) {
		svc := NewCSVImportService(nil, "UTC")
		_, err := svc.Preview(context.Background(), ev, []byte("   \n"), CSVImportOptions{})
		apiErr, ok := apierror.As(err)
		if !ok || apiErr.Code != "import.no_rows" {
			t.Errorf("expected import.no_rows, got %v", err)
		}
	})

	t.Run("a combustion vehicle cannot import charges", func(t *testing.T) {
		svc := NewCSVImportService(nil, "UTC")
		_, err := svc.Preview(context.Background(), testVehicle(models.PowertrainICE), []byte("date,kwh,cost\n2026-09-15,10,3\n"), CSVImportOptions{})
		if apiErr, ok := apierror.As(err); !ok || apiErr.Code != "import.ice_charges" {
			t.Errorf("expected import.ice_charges, got %v", err)
		}
	})

	t.Run("unrecognized headers are rejected", func(t *testing.T) {
		svc := NewCSVImportService(nil, "UTC")
		_, err := svc.Preview(context.Background(), ev, []byte("foo,bar\n1,2\n"), CSVImportOptions{})
		if apiErr, ok := apierror.As(err); !ok || apiErr.Code != "import.unsupported_type" {
			t.Errorf("expected import.unsupported_type, got %v", err)
		}
	})
}

func TestCSVImportRowErrorsCarryTheLineNumber(t *testing.T) {
	ev := testVehicle(models.PowertrainEV)
	res := previewOf(t, ev, "date,kwh,cost,currency\n2026-09-15,10,3,EUR\nnot-a-date,10,3,EUR\n2026-09-17,10,,EUR\n2026-09-18,10,3,USD\n2026-09-19,-4,3,EUR\n", CSVImportOptions{})
	if res.TotalRows != 5 || res.ValidRows != 1 || res.InvalidRows != 4 {
		t.Fatalf("unexpected counts: %+v", res)
	}
	want := []string{"import.row.invalid_date", "import.row.cost_required", "import.row.fx_required", "import.row.invalid_kwh"}
	got := rowCodes(res)
	for i, code := range want {
		if i >= len(got) || got[i] != code {
			t.Fatalf("codes = %v, want %v", got, want)
		}
	}
	if fmt.Sprint(res.Errors[0].Params["p0"]) != "3" {
		t.Errorf("first error should name line 3, got params %v", res.Errors[0].Params)
	}
}

func TestCSVImportForeignCurrencyNeedsARateOnlyWhenItDiffers(t *testing.T) {
	ev := testVehicle(models.PowertrainEV)
	res := previewOf(t, ev, "date,kwh,cost,currency,fx_rate\n2026-09-15,10,3,USD,0.92\n2026-09-16,10,3,eur,\n", CSVImportOptions{})
	if res.ValidRows != 2 {
		t.Errorf("expected both rows valid, got %+v (%v)", res, rowCodes(res))
	}
	bad := previewOf(t, ev, "date,kwh,cost,currency\n2026-09-15,10,3,EURO\n", CSVImportOptions{})
	if got := rowCodes(bad); len(got) != 1 || got[0] != "import.row.invalid_currency" {
		t.Errorf("codes = %v", got)
	}
}

func TestCSVImportDistanceUnits(t *testing.T) {
	ev := testVehicle(models.PowertrainEV)
	rc := &rowContext{columns: map[string]int{"distance": 0, "distance_km": 1, "distance_mi": 2, "odometer": 3}, vehicle: ev, loc: time.UTC, unit: "mi"}
	if km, _, _ := rc.distance([]string{"100", "", "", ""}, "distance"); km < 160.9 || km > 161 {
		t.Errorf("bare column in an mi account = %v km, want ~160.93", km)
	}
	if km, _, _ := rc.distance([]string{"100", "50", "", ""}, "distance"); km != 50 {
		t.Errorf("_km suffix must win over the account unit, got %v", km)
	}
	if km, _, _ := rc.distance([]string{"", "", "10", ""}, "distance"); km < 16.09 || km > 16.1 {
		t.Errorf("_mi suffix = %v km", km)
	}
	rc.unit = "km"
	if km, _, _ := rc.distance([]string{"100", "", "", ""}, "distance"); km != 100 {
		t.Errorf("bare column in a km account = %v", km)
	}
	if _, found, _ := rc.distance([]string{"", "", "", ""}, "distance"); found {
		t.Error("empty cells must not be found")
	}
	if _, found, err := rc.distance([]string{"abc", "", "", ""}, "distance"); !found || err == nil {
		t.Error("an unreadable value must be reported")
	}
}

func TestCSVImportDatesAreReadInTheConfiguredTimezone(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	got, err := parseFlexibleTime("2026-07-15 14:30", paris)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 7, 15, 12, 30, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("got %v, want %v", got.UTC(), want)
	}
	explicit, _ := parseFlexibleTime("2026-07-15T14:30:00Z", paris)
	if !explicit.Equal(time.Date(2026, 7, 15, 14, 30, 0, 0, time.UTC)) {
		t.Errorf("an explicit offset must be kept, got %v", explicit)
	}
	if svc := NewCSVImportService(nil, "Nowhere/Land"); svc.loc != time.UTC {
		t.Error("an unknown timezone must fall back to UTC")
	}
}

func TestCSVImportFuelRows(t *testing.T) {
	ice := testVehicle(models.PowertrainICE)
	for name, tc := range map[string]struct {
		csv  string
		code string
	}{
		"amount derived from litres and price": {"date,liters,price_per_liter\n2026-09-15,40,1.8\n", ""},
		"price derived from amount and litres": {"date,liters,amount\n2026-09-15,40,72\n", ""},
		"litres derived from amount and price": {"date,amount,price_per_liter\n2026-09-15,72,1.8\n", ""},
		"amount missing":                       {"date,fuel_type\n2026-09-15,DIESEL\n", "import.row.amount_required"},
		"invalid fuel type":                    {"date,amount,fuel_type\n2026-09-15,72,WATER\n", "import.row.invalid_fuel_type"},
		"invalid litres":                       {"date,amount,liters\n2026-09-15,72,9999\n", "import.row.invalid_liters"},
		"invalid odometer":                     {"date,amount,odometer\n2026-09-15,72,-5\n", "import.row.invalid_odometer"},
	} {
		t.Run(name, func(t *testing.T) {
			res := previewOf(t, ice, tc.csv, CSVImportOptions{Type: ImportTypeFuel})
			got := rowCodes(res)
			if tc.code == "" && (len(got) != 0 || res.ValidRows != 1) {
				t.Errorf("expected a valid row, got %v", got)
			}
			if tc.code != "" && (len(got) != 1 || got[0] != tc.code) {
				t.Errorf("codes = %v, want %s", got, tc.code)
			}
		})
	}
}

func TestCSVImportOdometerRows(t *testing.T) {
	ice := testVehicle(models.PowertrainICE)
	if res := previewOf(t, ice, "date,odometer_mi\n2026-09-15,1000\n", CSVImportOptions{Type: ImportTypeOdometer}); res.ValidRows != 1 {
		t.Errorf("expected a valid row: %v", rowCodes(res))
	}
	if res := previewOf(t, ice, "date,odometer\n2026-09-15,0\n", CSVImportOptions{Type: ImportTypeOdometer}); res.ValidRows != 0 {
		t.Error("a zero odometer is not a reading")
	}
	if res := previewOf(t, ice, "date,odometer\n2026-09-15,99999999\n", CSVImportOptions{Type: ImportTypeOdometer}); res.ValidRows != 0 {
		t.Error("an absurd odometer must be rejected")
	}
}

func TestCSVImportDriveRows(t *testing.T) {
	ev := testVehicle(models.PowertrainEV)
	if res := previewOf(t, ev, "start_time,distance_km\n2026-09-15 08:00,0\n", CSVImportOptions{Type: ImportTypeDrives}); len(rowCodes(res)) != 1 {
		t.Error("a zero distance must be rejected")
	}
	if res := previewOf(t, ev, "start_time,distance_mi,kwh\n2026-09-15 08:00,20,x\n", CSVImportOptions{Type: ImportTypeDrives}); res.ValidRows != 1 {
		t.Errorf("an unreadable kwh falls back to the estimate: %v", rowCodes(res))
	}
}

func TestParseFlag(t *testing.T) {
	for raw, want := range map[string]bool{"yes": true, "OUI": true, "0": false, "partiel": false, "": true, "maybe": true} {
		if got := parseFlag(raw, true); got != want {
			t.Errorf("parseFlag(%q) = %v, want %v", raw, got, want)
		}
	}
	if parseFlag("", false) {
		t.Error("the default must apply to an empty cell")
	}
}

func TestDetectTypeFromHeaders(t *testing.T) {
	for want, headers := range map[ImportType][]string{
		ImportTypeDrives:   {"Start_Time", "Distance_MI"},
		ImportTypeFuel:     {"date", "Litres"},
		ImportTypeCharges:  {"date", "\ufeffkwh"},
		ImportTypeOdometer: {"date", "odometer_km"},
		ImportTypeUnknown:  {"foo"},
	} {
		if got := detectTypeFromHeaders(headers); got != want {
			t.Errorf("detectTypeFromHeaders(%v) = %v, want %v", headers, got, want)
		}
	}
}

func TestFlexibleParsers(t *testing.T) {
	t.Run("parseFlexibleFloat", func(t *testing.T) {
		tests := []struct {
			input string
			want  float64
		}{
			{"12.34", 12.34},
			{"12,34", 12.34},
			{" 1 234,56 ", 1234.56},
		}
		for _, tc := range tests {
			got, err := parseFlexibleFloat(tc.input)
			if err != nil || got != tc.want {
				t.Errorf("parseFlexibleFloat(%q) = %v, err=%v; want %v", tc.input, got, err, tc.want)
			}
		}
	})

	t.Run("parseFlexibleTime", func(t *testing.T) {
		validDates := []string{
			"2026-09-30T14:00:00Z",
			"2026-09-30 14:00:00",
			"2026-09-30 14:00",
			"30/09/2026 14:00",
			"2026-09-30",
		}
		for _, vd := range validDates {
			if _, err := parseFlexibleTime(vd, time.UTC); err != nil {
				t.Errorf("parseFlexibleTime(%q) failed: %v", vd, err)
			}
		}
	})
}
