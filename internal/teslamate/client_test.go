package teslamate

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientAuthentication(t *testing.T) {
	// Test Bearer Auth
	bearerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer secret-token-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"cars":[{"car_id":1,"name":"Model 3","car_details":{"model":"3","trim_badging":"Performance","vin":"5YJ3E1EB..."}}]}}`))
	}))
	defer bearerServer.Close()

	client, err := NewClient(Config{
		BaseURL:  bearerServer.URL,
		AuthType: AuthBearer,
		APIToken: "secret-token-123",
	})
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	cars, err := client.GetCars(context.Background())
	if err != nil {
		t.Fatalf("GetCars failed: %v", err)
	}
	if len(cars) != 1 || cars[0].Name != "Model 3" {
		t.Fatalf("Unexpected cars result: %+v", cars)
	}

	// Test Basic Auth
	basicServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secretpass"))
		if auth != expected {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"car":{"car_id":1,"car_name":"Model Y"},"status":{"display_name":"Model Y","state":"online","odometer":34521.8},"units":{"unit_of_length":"km"}}}`))
	}))
	defer basicServer.Close()

	basicClient, err := NewClient(Config{
		BaseURL:  basicServer.URL,
		AuthType: AuthBasic,
		Username: "admin",
		Password: "secretpass",
	})
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	status, units, err := basicClient.GetCarStatus(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetCarStatus failed: %v", err)
	}
	if status.Odometer != 34521.8 || units.UnitOfLength != "km" {
		t.Fatalf("Unexpected status result: %+v, units: %+v", status, units)
	}
}

func TestGetDrivesAndCharges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/cars/1/drives":
			if r.URL.Query().Get("page") != "1" || r.URL.Query().Get("show") != "50" {
				t.Errorf("Unexpected query params: %s", r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"data": {
					"car": {"car_id": 1, "car_name": "Model 3"},
					"drives": [
						{
							"drive_id": 101,
							"start_date": "2026-09-10T08:30:00Z",
							"end_date": "2026-09-10T09:15:00Z",
							"start_address": "Paris",
							"end_address": "Versailles",
							"odometer_details": {
								"odometer_start": 10000.0,
								"odometer_end": 10025.5,
								"odometer_distance": 25.5
							},
							"duration_min": 45,
							"speed_avg": 34.0,
							"power_max": 120
						}
					],
					"units": {"unit_of_length": "km", "unit_of_temperature": "C"}
				}
			}`))
		case "/api/v1/cars/1/charges":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"data": {
					"car": {"car_id": 1, "car_name": "Model 3"},
					"charges": [
						{
							"charge_id": 201,
							"start_date": "2026-09-10T12:00:00Z",
							"end_date": "2026-09-10T12:40:00Z",
							"address": "Supercharger Orgeval",
							"charge_energy_added": 42.5,
							"charge_energy_used": 45.0,
							"cost": 14.85,
							"odometer": 10025.5
						}
					],
					"units": {"unit_of_length": "km", "unit_of_temperature": "C"}
				}
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Test GetDrives
	drives, units, err := client.GetDrives(context.Background(), 1, DriveFilterOptions{Page: 1, Show: 50})
	if err != nil {
		t.Fatalf("GetDrives failed: %v", err)
	}
	if len(drives) != 1 {
		t.Fatalf("Expected 1 drive, got %d", len(drives))
	}
	if drives[0].DriveID != 101 || drives[0].OdometerDetails.OdometerDistance != 25.5 {
		t.Fatalf("Unexpected drive payload: %+v", drives[0])
	}
	if units.UnitOfLength != "km" {
		t.Fatalf("Expected km unit, got %s", units.UnitOfLength)
	}

	// Test GetCharges
	charges, _, err := client.GetCharges(context.Background(), 1, ChargeFilterOptions{})
	if err != nil {
		t.Fatalf("GetCharges failed: %v", err)
	}
	if len(charges) != 1 {
		t.Fatalf("Expected 1 charge, got %d", len(charges))
	}
	if charges[0].ChargeID != 201 || charges[0].Cost == nil || *charges[0].Cost != 14.85 || charges[0].ChargeEnergyAdded != 42.5 {
		t.Fatalf("Unexpected charge payload: %+v", charges[0])
	}
}

func TestDistanceConversion(t *testing.T) {
	km := ConvertDistanceToKm(10.0, "km")
	if km != 10.0 {
		t.Errorf("Expected 10.0 km, got %f", km)
	}

	miInKm := ConvertDistanceToKm(10.0, "mi")
	expected := 16.09344
	if miInKm != expected {
		t.Errorf("Expected %f km, got %f", expected, miInKm)
	}
}

// Payloads shaped after TeslaMateApi's /drives, /charges and /battery-health responses.
func TestBatteryAndTemperatureFieldsAreDecoded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/cars/1/charges":
			w.Write([]byte(`{"data":{"car":{"car_id":1},"charges":[
				{"charge_id":7,"start_date":"2026-02-03T18:00:00Z","end_date":"2026-02-03T22:00:00Z","charge_energy_added":30.5,"charge_energy_used":33.1,
				 "cost":6.2,"duration_min":240,"battery_details":{"start_battery_level":22,"end_battery_level":71},
				 "range_ideal":{"start_range":100,"end_range":330},"outside_temp_avg":41.0,"odometer":12000},
				{"charge_id":8,"start_date":"2026-02-04T18:00:00Z","end_date":"2026-02-04T19:00:00Z","charge_energy_added":10,"charge_energy_used":10,
				 "duration_min":60,"battery_details":{"start_battery_level":50,"end_battery_level":60},"outside_temp_avg":null,"odometer":12100}],
				"units":{"unit_of_length":"km","unit_of_temperature":"F"}}}`))
		case "/api/v1/cars/1/drives":
			w.Write([]byte(`{"data":{"car":{"car_id":1},"drives":[
				{"drive_id":3,"start_date":"2026-02-03T08:00:00Z","end_date":"2026-02-03T09:00:00Z","odometer_details":{"odometer_start":1,"odometer_end":41,"odometer_distance":40},
				 "battery_details":{"start_usable_battery_level":80,"start_battery_level":81,"end_usable_battery_level":70,"end_battery_level":71,"reduced_range":false,"is_sufficiently_precise":true},
				 "outside_temp_avg":-2.5,"inside_temp_avg":20.1,"energy_consumed_net":6.1,"consumption_net":152}],
				"units":{"unit_of_length":"km","unit_of_temperature":"C"}}}`))
		case "/api/v1/cars/1/battery-health":
			w.Write([]byte(`{"data":{"car":{"car_id":1},"battery_health":{"max_range":480,"current_range":462,"max_capacity":78.4,"current_capacity":75.1,"rated_efficiency":140.5,"battery_health_percentage":95.79},"units":{"unit_of_length":"km"}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL, AuthType: AuthNone})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	charges, units, err := client.GetCharges(ctx, 1, ChargeFilterOptions{})
	if err != nil || len(charges) != 2 {
		t.Fatalf("charges: %v, %d", err, len(charges))
	}
	if b := charges[0].BatteryDetails; b.StartBatteryLevel != 22 || b.EndBatteryLevel != 71 {
		t.Errorf("charge battery levels: %+v", b)
	}
	if charges[0].OutsideTempAvg == nil || *charges[0].OutsideTempAvg != 41.0 {
		t.Errorf("charge temperature: %v", charges[0].OutsideTempAvg)
	}
	if charges[1].OutsideTempAvg != nil {
		t.Errorf("a null temperature must stay nil, got %v", *charges[1].OutsideTempAvg)
	}
	if units.UnitOfTemperature != "F" {
		t.Errorf("temperature unit: %q", units.UnitOfTemperature)
	}

	drives, _, err := client.GetDrives(ctx, 1, DriveFilterOptions{})
	if err != nil || len(drives) != 1 {
		t.Fatalf("drives: %v, %d", err, len(drives))
	}
	if b := drives[0].BatteryDetails; b.StartBatteryLevel != 81 || b.EndBatteryLevel != 71 {
		t.Errorf("drive battery levels: %+v", b)
	}
	if drives[0].OutsideTempAvg == nil || *drives[0].OutsideTempAvg != -2.5 {
		t.Errorf("drive temperature: %v", drives[0].OutsideTempAvg)
	}

	health, err := client.GetBatteryHealth(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if health.MaxCapacity != 78.4 || health.CurrentCapacity != 75.1 || health.BatteryHealthPercentage != 95.79 {
		t.Errorf("battery health: %+v", health)
	}
}

func TestBatteryHealthUnavailableOnOlderTeslaMateApi(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	defer server.Close()
	client, _ := NewClient(Config{BaseURL: server.URL, AuthType: AuthNone})
	if _, err := client.GetBatteryHealth(context.Background(), 1); err == nil {
		t.Fatal("a missing endpoint must be reported as an error so the caller can skip the snapshot")
	}
}

func TestConvertTemperatureToC(t *testing.T) {
	cases := []struct {
		in   float64
		unit string
		want float64
	}{
		{41, "F", 5}, {32, "F", 0}, {14, "f", -10}, {-2.5, "C", -2.5}, {20, "", 20},
	}
	for _, c := range cases {
		if got := ConvertTemperatureToC(c.in, c.unit); got < c.want-0.001 || got > c.want+0.001 {
			t.Errorf("ConvertTemperatureToC(%v, %q) = %v, want %v", c.in, c.unit, got, c.want)
		}
	}
}
