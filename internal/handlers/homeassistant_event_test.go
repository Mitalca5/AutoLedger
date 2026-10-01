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
