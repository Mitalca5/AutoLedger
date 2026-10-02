package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

func TestHomeAssistantOdometerEventsNeedTheirVehicle(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "ha-events@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	// The home charger's default vehicle gets the charging sessions sent without a vehicle, never the odometer readings.
	homeDefault := &models.Vehicle{UserID: u.ID, Name: "Home default", CurrentOdometer: 1000, IsHomeChargerDefault: true, TeslaMateAuthType: models.AuthModeNone}
	other := &models.Vehicle{UserID: u.ID, Name: "Other", CurrentOdometer: 5000, TeslaMateAuthType: models.AuthModeNone}
	for _, v := range []*models.Vehicle{homeDefault, other} {
		if err := repo.CreateVehicle(ctx, v); err != nil {
			t.Fatal(err)
		}
	}

	h := NewHomeAssistantHandler(repo, nil)
	post := func(body string) (int, string) {
		req := httptest.NewRequest(http.MethodPost, "/api/integrations/homeassistant/event", bytes.NewBufferString(body))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, u.ID))
		rec := httptest.NewRecorder()
		h.HandleEvent(rec, req)
		var resp struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		return rec.Code, resp.Code
	}
	odometerOf := func(v *models.Vehicle) float64 {
		got, err := repo.GetVehicleByID(ctx, v.ID, u.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got.CurrentOdometer
	}

	if status, code := post(`{"event_type":"odometer_update","data":{"odometer_km":6200}}`); status != http.StatusBadRequest || code != "vehicle.not_specified" {
		t.Errorf("reading without vehicle: got %d %q, want 400 vehicle.not_specified", status, code)
	}
	if got := odometerOf(homeDefault); got != 1000 {
		t.Errorf("a reading without vehicle changed the home default vehicle's odometer to %v", got)
	}

	if status, _ := post(`{"vehicle_id":"` + other.ID + `","event_type":"odometer_update","data":{"odometer_km":6200}}`); status != http.StatusOK {
		t.Errorf("reading with its vehicle: got %d, want 200", status)
	}
	if got := odometerOf(other); got != 6200 {
		t.Errorf("odometer after the reading: got %v, want 6200", got)
	}

	if status, code := post(`{"vehicle_id":"` + other.ID + `","event_type":"telemetry_update","data":{}}`); status != http.StatusBadRequest || code != "telemetry.missing_odometer" {
		t.Errorf("telemetry without odometer: got %d %q, want 400 telemetry.missing_odometer", status, code)
	}
	if status, code := post(`{"vehicle_id":"` + other.ID + `","event_type":"odometer_updat","data":{"energy_kwh":20}}`); status != http.StatusBadRequest || code != "integration.unknown_event_type" {
		t.Errorf("misspelt event type: got %d %q, want 400 integration.unknown_event_type", status, code)
	}
}

func TestHomeAssistantOdometerReadingsKeepOnePointPerDay(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "ha-points@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "Car", CurrentOdometer: 1000, TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}

	h := NewHomeAssistantHandler(repo, nil)
	post := func(timestamp string, km float64) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"vehicle_id": v.ID, "event_type": "odometer_update", "timestamp": timestamp,
			"data": map[string]any{"odometer_km": km},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/integrations/homeassistant/event", bytes.NewBuffer(body))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, u.ID))
		rec := httptest.NewRecorder()
		h.HandleEvent(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("reading %v at %s: got %d %s", km, timestamp, rec.Code, rec.Body.String())
		}
	}
	points := func() []models.OdometerCheckpoint {
		t.Helper()
		got, err := repo.ListOdometerCheckpoints(ctx, v.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	post("2026-03-01T08:00:00Z", 1500)
	post("2026-03-01T18:00:00Z", 1520)
	post("2026-03-01T19:00:00Z", 1510) // a lower reading the same day never lowers the day's point
	got := points()
	if len(got) != 1 || got[0].Odometer != 1520 || got[0].Source != models.OdometerSourceHA {
		t.Fatalf("one HA point per day expected at 1520, got %+v", got)
	}

	post("2026-03-02T08:00:00Z", 1600)
	if got := points(); len(got) != 2 || got[1].Odometer != 1600 {
		t.Fatalf("a new day gets its own point, got %+v", got)
	}

	// A reading dated before a kept point but showing a higher mileage contradicts the history: not stored.
	post("2026-02-28T20:00:00Z", 5000)
	if got := points(); len(got) != 2 {
		t.Fatalf("an inconsistent reading must not become a point, got %+v", got)
	}

	if err := repo.UpdateOdometerCheckpoint(ctx, &models.OdometerCheckpoint{ID: points()[0].ID, VehicleID: v.ID, Date: points()[0].Date, Odometer: 1}); err == nil {
		t.Error("an HA point must not be editable")
	}
	if err := repo.DeleteOdometerCheckpoint(ctx, v.ID, points()[0].ID); err != nil {
		t.Errorf("an HA point can be deleted: %v", err)
	}
}
