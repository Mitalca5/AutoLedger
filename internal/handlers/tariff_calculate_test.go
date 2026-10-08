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
	"github.com/teslacost/teslacost/internal/services"
)

func TestCalculateSessionCostForVehicle(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	owner, err := repo.CreateUser(ctx, "tariff-calc@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := repo.CreateUser(ctx, "tariff-calc-other@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	plan := &models.TariffPlan{
		UserID: owner.ID, Name: "Night", PlanType: "BANDS", Currency: "EUR", DefaultBand: "day",
		Bands: []models.TariffBand{{Name: "day", RateCents: 200000}, {Name: "night", RateCents: 100000}},
		Rules: []models.TariffRule{{Start: "22:00", End: "06:00", Band: "night"}},
	}
	if err := repo.CreateTariffPlan(ctx, plan); err != nil {
		t.Fatal(err)
	}
	priced := &models.Vehicle{UserID: owner.ID, Name: "Priced", TeslaMateAuthType: models.AuthModeNone, TariffPlanID: &plan.ID}
	bare := &models.Vehicle{UserID: stranger.ID, Name: "Bare", TeslaMateAuthType: models.AuthModeNone}
	for _, v := range []*models.Vehicle{priced, bare} {
		if err := repo.CreateVehicle(ctx, v); err != nil {
			t.Fatal(err)
		}
	}

	// Rules are wall-clock hours in the application timezone: 22:00-06:00 in Paris (UTC+2 in July) is 20:00-04:00Z
	h := NewTariffHandler(repo, services.NewTariffServiceIn("Europe/Paris"))
	call := func(userID, body string) (int, map[string]any) {
		req := httptest.NewRequest(http.MethodPost, "/calculate-session", bytes.NewBufferString(body))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
		rec := httptest.NewRecorder()
		h.Calculate(rec, req)
		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		return rec.Code, resp
	}
	session := func(vehicleID string) string {
		return `{"vehicle_id":"` + vehicleID + `","start_time":"2026-07-10T20:00:00Z","end_time":"2026-07-11T04:00:00Z","kwh":10}`
	}

	status, resp := call(owner.ID, session(priced.ID))
	if status != http.StatusOK || resp["cost"] != 1.0 || resp["plan"] != "Night" {
		t.Fatalf("session across midnight: got %d %v, want 1.00 on the night band", status, resp)
	}
	status, resp = call(stranger.ID, session(bare.ID))
	if status != http.StatusOK || resp["plan"] != nil {
		t.Fatalf("account without a tariff: got %d %v, want no plan", status, resp)
	}
	if status, _ = call(stranger.ID, session(priced.ID)); status != http.StatusNotFound {
		t.Fatalf("another account's vehicle: got %d, want 404", status)
	}
}
