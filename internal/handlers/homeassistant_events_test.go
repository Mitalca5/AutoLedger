package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

type haEventEnv struct {
	t    *testing.T
	repo *database.Repository
	h    *HomeAssistantHandler
}

func (e *haEventEnv) post(userID, body string) (int, string) {
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/homeassistant/event", bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()
	e.h.HandleEvent(rec, req)
	var resp struct {
		Code   string `json:"code"`
		Status string `json:"status"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != "" {
		return rec.Code, resp.Code
	}
	return rec.Code, resp.Status
}

func newHAEventEnv(t *testing.T, email string) (*haEventEnv, *models.User) {
	repo := authTestRepo(t)
	u, err := repo.CreateUser(context.Background(), email, "hash")
	if err != nil {
		t.Fatal(err)
	}
	return &haEventEnv{t: t, repo: repo, h: NewHomeAssistantHandler(repo, nil)}, u
}

func (e *haEventEnv) vehicle(userID, name, powertrain string, odometer float64) *models.Vehicle {
	v := &models.Vehicle{UserID: userID, Name: name, Powertrain: powertrain, CurrentOdometer: odometer, TeslaMateAuthType: models.AuthModeNone}
	if err := e.repo.CreateVehicle(context.Background(), v); err != nil {
		e.t.Fatal(err)
	}
	return v
}

func TestHomeAssistantOdometerUnits(t *testing.T) {
	env, u := newHAEventEnv(t, "ha-odo-units@example.com")
	v := env.vehicle(u.ID, "Car", models.PowertrainEV, 1000)
	odometer := func() float64 {
		got, err := env.repo.GetVehicleByID(context.Background(), v.ID, u.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got.CurrentOdometer
	}

	if status, _ := env.post(u.ID, `{"vehicle_id":"`+v.ID+`","event_type":"odometer","data":{"odometer":2000}}`); status != http.StatusOK {
		t.Fatalf("odometer in km: got %d", status)
	}
	if got := odometer(); got != 2000 {
		t.Errorf("odometer: got %v, want 2000", got)
	}
	if status, _ := env.post(u.ID, `{"vehicle_id":"`+v.ID+`","event_type":"odometer","distance_unit":"mi","data":{"odometer":2000}}`); status != http.StatusOK {
		t.Fatalf("odometer in miles: got %d", status)
	}
	if got := odometer(); math.Abs(got-3218.688) > 0.01 {
		t.Errorf("2000 mi: got %v km, want 3218.688", got)
	}
	if status, code := env.post(u.ID, `{"vehicle_id":"`+v.ID+`","event_type":"odometer","distance_unit":"furlong","data":{"odometer":5000}}`); status != http.StatusBadRequest || code != "integration.invalid_distance_unit" {
		t.Errorf("unknown unit: got %d %q", status, code)
	}
	if status, _ := env.post(u.ID, `{"vehicle_id":"`+v.ID+`","event_type":"odometer","data":{"odometer_km":9999999}}`); status != http.StatusBadRequest {
		t.Errorf("absurd reading: got %d, want 400", status)
	}
}

func TestHomeAssistantDriveEvent(t *testing.T) {
	env, u := newHAEventEnv(t, "ha-drive@example.com")
	v := env.vehicle(u.ID, "Car", models.PowertrainEV, 1000)
	ctx := context.Background()
	body := func(extra string) string {
		return `{"vehicle_id":"` + v.ID + `","event_type":"drive",` + extra + `}`
	}

	if status, code := env.post(u.ID, body(`"timestamp":"2026-05-01T08:00:00Z","data":{"distance":0}`)); status != http.StatusBadRequest || code != "drive.invalid_distance" {
		t.Errorf("zero distance: got %d %q", status, code)
	}
	if status, code := env.post(u.ID, body(`"data":{"distance":20}`)); status != http.StatusBadRequest || code != "drive.missing_start_time" {
		t.Errorf("no start: got %d %q", status, code)
	}

	miles := body(`"event_id":"d-1","distance_unit":"mi","data":{"start_time":"2026-05-01T08:00:00Z","distance":10,"end_odometer":1100}`)
	if status, code := env.post(u.ID, miles); status != http.StatusCreated || code != "recorded" {
		t.Fatalf("drive: got %d %q", status, code)
	}
	drives, total, err := env.repo.ListDrives(ctx, v.ID, database.DriveFilter{}, 10, 0)
	if err != nil || total != 1 {
		t.Fatalf("drives: %v, total %d", err, total)
	}
	d := drives[0]
	if math.Abs(d.DistanceKm-16.09344) > 0.01 {
		t.Errorf("distance: got %v km, want 16.09344", d.DistanceKm)
	}
	if d.EndOdometer == nil || math.Abs(*d.EndOdometer-1100*1.609344) > 0.01 {
		t.Errorf("end odometer: got %v, want %v", d.EndOdometer, 1100*1.609344)
	}
	if got, _ := env.repo.GetVehicleByID(ctx, v.ID, u.ID); got.CurrentOdometer < 1700 {
		t.Errorf("vehicle odometer not advanced: %v", got.CurrentOdometer)
	}

	// Same event id, then the same drive without one: both are duplicates.
	if status, code := env.post(u.ID, miles); status != http.StatusOK || code != "duplicate" {
		t.Errorf("same event id: got %d %q", status, code)
	}
	if status, code := env.post(u.ID, body(`"distance_unit":"mi","data":{"start_time":"2026-05-01T08:05:00Z","distance":10.2}`)); status != http.StatusOK || code != "duplicate" {
		t.Errorf("similar drive: got %d %q", status, code)
	}
	if _, total, _ := env.repo.ListDrives(ctx, v.ID, database.DriveFilter{}, 10, 0); total != 1 {
		t.Errorf("duplicates were stored: %d drives", total)
	}
}

func TestHomeAssistantFuelEvent(t *testing.T) {
	env, u := newHAEventEnv(t, "ha-fuel@example.com")
	ice := env.vehicle(u.ID, "Petrol", models.PowertrainICE, 10000)
	ev := env.vehicle(u.ID, "Electric", models.PowertrainEV, 1000)
	ctx := context.Background()
	fill := func(vehicleID, at string) string {
		return `{"vehicle_id":"` + vehicleID + `","event_type":"fuel","timestamp":"` + at + `","data":{"amount":60,"liters":40,"odometer_km":10500}}`
	}

	if status, code := env.post(u.ID, fill(ev.ID, "2026-05-01T10:00:00Z")); status != http.StatusBadRequest || code != "fuel.combustion_only" {
		t.Errorf("fuel on an EV: got %d %q", status, code)
	}
	if status, code := env.post(u.ID, fill(ice.ID, "2026-05-01T10:00:00Z")); status != http.StatusCreated || code != "recorded" {
		t.Fatalf("fuel: got %d %q", status, code)
	}
	if status, code := env.post(u.ID, fill(ice.ID, "2026-05-01T10:10:00Z")); status != http.StatusOK || code != "duplicate" {
		t.Errorf("same fill-up again: got %d %q", status, code)
	}
	logs, err := env.repo.ListFuelLogs(ctx, ice.ID)
	if err != nil || len(logs) != 1 {
		t.Fatalf("fuel logs: %v, %d", err, len(logs))
	}
	if status, code := env.post(u.ID, `{"vehicle_id":"`+ice.ID+`","event_type":"fuel","data":{}}`); status != http.StatusBadRequest {
		t.Errorf("fuel without amount: got %d %q, want 400", status, code)
	}
}

func TestHomeAssistantEventsRefuseOtherAccountVehicle(t *testing.T) {
	env, owner := newHAEventEnv(t, "ha-owner@example.com")
	intruder, err := env.repo.CreateUser(context.Background(), "ha-intruder@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	ice := env.vehicle(owner.ID, "Petrol", models.PowertrainICE, 10000)

	for name, body := range map[string]string{
		"drive":    `{"vehicle_id":"` + ice.ID + `","event_type":"drive","timestamp":"2026-05-01T08:00:00Z","data":{"distance":20}}`,
		"fuel":     `{"vehicle_id":"` + ice.ID + `","event_type":"fuel","data":{"amount":50}}`,
		"odometer": `{"vehicle_id":"` + ice.ID + `","event_type":"odometer","data":{"odometer":20000}}`,
	} {
		if status, _ := env.post(intruder.ID, body); status < 400 {
			t.Errorf("%s on another account's vehicle: got %d", name, status)
		}
	}
	if got, _ := env.repo.GetVehicleByID(context.Background(), ice.ID, owner.ID); got.CurrentOdometer != 10000 {
		t.Errorf("odometer changed to %v", got.CurrentOdometer)
	}
	if status, code := env.post(owner.ID, `{"event_type":"drive","timestamp":"2026-05-01T08:00:00Z","data":{"distance":20}}`); status != http.StatusBadRequest || code != "vehicle.not_specified" {
		t.Errorf("drive without vehicle: got %d %q", status, code)
	}
}

func TestHomeAssistantEventsOnHybrids(t *testing.T) {
	env, u := newHAEventEnv(t, "ha-hybrid@example.com")
	for _, powertrain := range []string{models.PowertrainPHEV, models.PowertrainREEV} {
		v := env.vehicle(u.ID, powertrain, powertrain, 10000)
		body := `{"vehicle_id":"` + v.ID + `","event_type":"fuel","timestamp":"2026-05-01T10:00:00Z","data":{"amount":60,"liters":40,"odometer_km":10500}}`
		if status, code := env.post(u.ID, body); status != http.StatusCreated || code != "recorded" {
			t.Fatalf("%s fuel: got %d %q", powertrain, status, code)
		}
		got, err := env.repo.GetVehicleByID(context.Background(), v.ID, u.ID)
		if err != nil || got.Powertrain != powertrain || got.CurrentOdometer != 10500 {
			t.Errorf("%s after fill-up: %+v, %v", powertrain, got, err)
		}
	}
}

func TestHomeAssistantDriveEventWithoutDistance(t *testing.T) {
	env, u := newHAEventEnv(t, "ha-drive-nodist@example.com")
	v := env.vehicle(u.ID, "Car", models.PowertrainEV, 1000)
	ctx := context.Background()
	body := func(extra string) string {
		return `{"vehicle_id":"` + v.ID + `","event_type":"drive",` + extra + `}`
	}

	// Positions only: no distance, no energy, coordinates stand in for the addresses.
	positions := body(`"event_id":"t-1","data":{"start_time":"2026-05-01T08:00:00Z","end_time":"2026-05-01T08:25:00Z",` +
		`"start_lat":45.8992,"start_lon":6.1294,"end_lat":45.7640,"end_lon":4.8357,"end_address":"Work"}`)
	if status, code := env.post(u.ID, positions); status != http.StatusCreated || code != "recorded" {
		t.Fatalf("positions only: got %d %q", status, code)
	}
	// Odometers on both ends give the distance.
	odometers := body(`"event_id":"t-2","data":{"start_time":"2026-05-02T08:00:00Z","start_odometer":1000,"end_odometer":1031.5}`)
	if status, code := env.post(u.ID, odometers); status != http.StatusCreated || code != "recorded" {
		t.Fatalf("odometers: got %d %q", status, code)
	}
	// One odometer alone does not invent the other.
	oneOdo := body(`"event_id":"t-3","data":{"start_time":"2026-05-03T08:00:00Z","start_odometer":1100}`)
	if status, code := env.post(u.ID, oneOdo); status != http.StatusCreated || code != "recorded" {
		t.Fatalf("one odometer: got %d %q", status, code)
	}
	if status, code := env.post(u.ID, body(`"data":{"start_time":"2026-05-04T08:00:00Z","start_lat":120,"start_lon":6}`)); status != http.StatusCreated || code != "recorded" {
		t.Fatalf("bad latitude: got %d %q", status, code)
	}

	drives, total, err := env.repo.ListDrives(ctx, v.ID, database.DriveFilter{}, 10, 0)
	if err != nil || total != 4 {
		t.Fatalf("drives: %v, total %d", err, total)
	}
	byStart := map[string]models.Drive{}
	for _, d := range drives {
		byStart[d.StartTime.Format("2006-01-02")] = d
	}

	d := byStart["2026-05-01"]
	if d.DistanceKm != 0 || d.EnergyConsumedKwh != nil || d.ConsumptionKwh100km != nil || d.EnergyEstimated {
		t.Errorf("positions only: distance %v, energy %v, consumption %v, estimated %v", d.DistanceKm, d.EnergyConsumedKwh, d.ConsumptionKwh100km, d.EnergyEstimated)
	}
	if d.DurationMin != 25 {
		t.Errorf("duration: got %d, want 25", d.DurationMin)
	}
	if d.StartAddress == nil || *d.StartAddress != "45.89920, 6.12940" {
		t.Errorf("start address: got %v", d.StartAddress)
	}
	if d.EndAddress == nil || *d.EndAddress != "Work" {
		t.Errorf("a given address wins over coordinates: got %v", d.EndAddress)
	}

	d = byStart["2026-05-02"]
	if math.Abs(d.DistanceKm-31.5) > 0.001 || d.EnergyConsumedKwh == nil || !d.EnergyEstimated {
		t.Errorf("odometer pair: distance %v, energy %v, estimated %v", d.DistanceKm, d.EnergyConsumedKwh, d.EnergyEstimated)
	}

	d = byStart["2026-05-03"]
	if d.DistanceKm != 0 || d.EndOdometer != nil {
		t.Errorf("one odometer: distance %v, end odometer %v, want unknown", d.DistanceKm, d.EndOdometer)
	}

	if d := byStart["2026-05-04"]; d.StartAddress != nil {
		t.Errorf("out of range latitude must be ignored: %v", *d.StartAddress)
	}
}
