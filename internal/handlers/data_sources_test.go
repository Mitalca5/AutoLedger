package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

func TestGetDataSourcesSummarisesOriginsAndHidesConnectionFromNonOwners(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	owner, err := repo.CreateUser(ctx, "ds-owner@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := repo.CreateUser(ctx, "ds-stranger@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	url := "http://teslamate.local:8080"
	v := &models.Vehicle{UserID: owner.ID, Name: "Car", TeslaMateAPIURL: &url, TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	manual := &models.OdometerCheckpoint{VehicleID: v.ID, Date: day, Odometer: 1000}
	if err := repo.CreateOdometerCheckpoint(ctx, manual); err != nil {
		t.Fatal(err)
	}
	for i, km := range []float64{1100, 1200} {
		if _, err := repo.Pool().Exec(ctx,
			`INSERT INTO odometer_checkpoints (vehicle_id, date, odometer, source) VALUES ($1, $2, $3, 'HA')`,
			v.ID, day.AddDate(0, 0, i+1), km); err != nil {
			t.Fatal(err)
		}
	}

	h := NewVehicleHandler(repo, nil, nil)
	type body struct {
		Configured bool `json:"teslamate_configured"`
		Activity   []struct {
			Origin           string `json:"origin"`
			OdometerReadings int    `json:"odometer_readings"`
		} `json:"activity"`
	}
	get := func(userID string) (int, body) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", v.ID)
		req := httptest.NewRequest(http.MethodGet, "/api/vehicles/"+v.ID+"/data-sources", nil)
		req = req.WithContext(context.WithValue(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), middleware.UserIDKey, userID))
		rec := httptest.NewRecorder()
		h.GetDataSources(rec, req)
		var b body
		_ = json.Unmarshal(rec.Body.Bytes(), &b)
		return rec.Code, b
	}

	code, b := get(owner.ID)
	if code != http.StatusOK || !b.Configured {
		t.Fatalf("owner: got %d configured=%v", code, b.Configured)
	}
	counts := map[string]int{}
	for _, a := range b.Activity {
		counts[a.Origin] = a.OdometerReadings
	}
	if len(counts) != 2 || counts["MANUAL"] != 1 || counts["WEBHOOK"] != 2 {
		t.Errorf("activity by origin: got %v, want MANUAL=1 WEBHOOK=2", counts)
	}
	if code, _ := get(stranger.ID); code != http.StatusNotFound {
		t.Errorf("another account: got %d, want 404", code)
	}
}
