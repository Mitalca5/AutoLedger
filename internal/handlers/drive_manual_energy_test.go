package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

func TestManualDriveEnergyIsFlaggedWhenEstimated(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "manual-energy@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	kwh100 := 20.0
	v := &models.Vehicle{UserID: u.ID, Name: "EV", TeslaMateAuthType: models.AuthModeNone, EstimatedKwh100km: &kwh100}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}

	h := NewDriveHandler(repo, nil, nil)
	router := chi.NewRouter()
	router.Post("/vehicles/{vehicleId}/drives", h.Create)
	router.Put("/vehicles/{vehicleId}/drives/{driveId}", h.Update)
	send := func(method, path, body string) models.Drive {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, u.ID))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
			t.Fatalf("%s %s: status %d, body %s", method, path, rec.Code, rec.Body.String())
		}
		var d models.Drive
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		return d
	}
	check := func(step string, d models.Drive, energy, cons float64, estimated bool) {
		t.Helper()
		if d.EnergyConsumedKwh == nil || *d.EnergyConsumedKwh != energy || d.ConsumptionKwh100km == nil || *d.ConsumptionKwh100km != cons || d.EnergyEstimated != estimated {
			t.Errorf("%s: got energy %v, consumption %v, estimated %v; want %v, %v, %v",
				step, d.EnergyConsumedKwh, d.ConsumptionKwh100km, d.EnergyEstimated, energy, cons, estimated)
		}
	}

	created := send(http.MethodPost, "/vehicles/"+v.ID+"/drives", `{"start_time":"2026-09-01T08:00:00Z","distance_km":50}`)
	check("created without energy", created, 10, 20, true)

	drivePath := "/vehicles/" + v.ID + "/drives/" + created.ID
	check("distance edited", send(http.MethodPut, drivePath, `{"distance_km":100}`), 20, 20, true)
	check("energy typed", send(http.MethodPut, drivePath, `{"energy_consumed_kwh":15}`), 15, 15, false)
	check("distance edited after typing the energy", send(http.MethodPut, drivePath, `{"distance_km":50}`), 15, 30, false)
}
