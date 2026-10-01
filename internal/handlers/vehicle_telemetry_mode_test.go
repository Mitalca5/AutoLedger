package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

func TestVehicleTelemetryModeIsDerivedAndMakeIsFreeText(t *testing.T) {
	repo := authTestRepo(t)
	u, err := repo.CreateUser(context.Background(), "telemetry-mode@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := crypto.NewEncryptor("test-key")
	if err != nil {
		t.Fatal(err)
	}
	h := NewVehicleHandler(repo, enc, nil)
	router := chi.NewRouter()
	router.Post("/vehicles", h.Create)
	router.Put("/vehicles/{id}", h.Update)
	send := func(method, path, body string) models.Vehicle {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, u.ID))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
			t.Fatalf("%s %s: status %d, body %s", method, path, rec.Code, rec.Body.String())
		}
		var v models.Vehicle
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	check := func(step string, v models.Vehicle, mode, make, model string) {
		t.Helper()
		stored, err := repo.GetVehicleByID(context.Background(), v.ID, u.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, got := range []models.Vehicle{v, *stored} {
			if got.TelemetryMode != mode || got.Make != make || got.Model != model {
				t.Errorf("%s: got mode %q, make %q, model %q; want %q, %q, %q", step, got.TelemetryMode, got.Make, got.Model, mode, make, model)
			}
		}
	}

	// A mode sent by the client is ignored, and no make is invented.
	v := send(http.MethodPost, "/vehicles", `{"name":"EV","telemetry_mode":"CONNECTED"}`)
	check("created without TeslaMate", v, models.TelemetryManual, "", "")

	path := "/vehicles/" + v.ID
	check("TeslaMate added", send(http.MethodPut, path, `{"name":"EV","teslamate_api_url":"http://teslamate:8080","make":" Some make ","model":"Some model"}`),
		models.TelemetryConnected, "Some make", "Some model")
	check("make and model omitted are kept", send(http.MethodPut, path, `{"name":"EV","teslamate_api_url":"http://teslamate:8080","telemetry_mode":"MANUAL"}`),
		models.TelemetryConnected, "Some make", "Some model")
	check("TeslaMate removed, make cleared", send(http.MethodPut, path, `{"name":"EV","teslamate_api_url":"","make":""}`),
		models.TelemetryManual, "", "Some model")
}
