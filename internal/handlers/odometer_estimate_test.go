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

func TestOdometerEstimateInterpolatesBetweenReadings(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	owner, err := repo.CreateUser(ctx, "odo-est@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := repo.CreateUser(ctx, "odo-est-other@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: owner.ID, Name: "Car", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	for _, c := range []models.OdometerCheckpoint{
		{VehicleID: v.ID, Date: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Odometer: 10000, Source: models.OdometerSourceManual},
		{VehicleID: v.ID, Date: time.Date(2025, 1, 11, 0, 0, 0, 0, time.UTC), Odometer: 11000, Source: models.OdometerSourceManual},
	} {
		c := c
		if err := repo.CreateOdometerCheckpoint(ctx, &c); err != nil {
			t.Fatal(err)
		}
	}

	h := NewVehicleHandler(repo, nil, nil)
	get := func(userID, date string) (int, struct {
		Odometer *float64 `json:"odometer"`
		Source   string   `json:"source"`
	}) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", v.ID)
		req := httptest.NewRequest(http.MethodGet, "/api/vehicles/"+v.ID+"/odometer-estimate?date="+date, nil)
		req = req.WithContext(context.WithValue(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), middleware.UserIDKey, userID))
		rec := httptest.NewRecorder()
		h.GetOdometerEstimate(rec, req)
		var body struct {
			Odometer *float64 `json:"odometer"`
			Source   string   `json:"source"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}

	if code, body := get(owner.ID, "2025-01-06"); code != http.StatusOK || body.Odometer == nil || *body.Odometer != 10500 || body.Source != "interpolated" {
		t.Errorf("between two readings: got %d %+v, want 10500 interpolated", code, body)
	}
	if code, _ := get(owner.ID, "not-a-date"); code != http.StatusBadRequest {
		t.Errorf("invalid date: got %d, want 400", code)
	}
	if code, _ := get(stranger.ID, "2025-01-06"); code != http.StatusNotFound {
		t.Errorf("another account's vehicle: got %d, want 404", code)
	}
}
