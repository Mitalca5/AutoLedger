package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

func TestFleetSummaryComputesEnergyCostPer100Km(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "fleet-summary@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "EV", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	pool := repo.Pool()
	if _, err := pool.Exec(ctx, `INSERT INTO charge_logs (vehicle_id, date, kwh_added, cost, cost_source, currency, is_manual)
		VALUES ($1, now(), 50, 1000, 'MANUAL', 'EUR', TRUE)`, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO drives (vehicle_id, start_time, end_time, distance_km, duration_min)
		VALUES ($1, now() - interval '1 hour', now(), 200, 60)`, v.ID); err != nil {
		t.Fatal(err)
	}

	summary, err := repo.GetFleetSummary(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetFleetSummary failed: %v", err)
	}
	if len(summary.Vehicles) != 1 {
		t.Fatalf("expected 1 vehicle, got %d", len(summary.Vehicles))
	}
	m := summary.Vehicles[0]
	if m.EnergyCostPer100Km != 500 {
		t.Errorf("energy cost per 100 km: got %v, want 500 (1000.00 spent over 200 km)", m.EnergyCostPer100Km)
	}
	if summary.CurrentMonthCost != 100000 || m.MonthCost != 100000 {
		t.Errorf("month cost in cents: fleet %d, vehicle %d, want 100000", summary.CurrentMonthCost, m.MonthCost)
	}
	if summary.MonthlyBudget != nil {
		t.Errorf("a household without budget must report none, got %v", *summary.MonthlyBudget)
	}
}

func TestFleetBudgetSetValidateAndClear(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "fleet-budget@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	h := NewFleetHandler(services.NewFleetService(repo))
	put := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/fleet/budget", strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, u.ID))
		rec := httptest.NewRecorder()
		h.SetBudget(rec, req)
		return rec
	}
	budget := func() *int64 {
		s, err := repo.GetFleetSummary(ctx, u.ID)
		if err != nil {
			t.Fatal(err)
		}
		if s.MonthlyBudget == nil {
			return nil
		}
		v := int64(*s.MonthlyBudget)
		return &v
	}

	if rec := put(`{"amount": 250}`); rec.Code != http.StatusOK {
		t.Fatalf("set: %d %s", rec.Code, rec.Body)
	}
	if b := budget(); b == nil || *b != 25000 {
		t.Fatalf("budget after set: %v", b)
	}
	for _, bad := range []string{`{"amount": 0}`, `{"amount": -5}`, `{"amount": 99999999999}`, `not json`} {
		if rec := put(bad); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", bad, rec.Code)
		}
	}
	if b := budget(); b == nil || *b != 25000 {
		t.Fatalf("a refused budget must not change the stored one: %v", b)
	}
	if rec := put(`{"amount": null}`); rec.Code != http.StatusOK {
		t.Fatalf("clear: %d", rec.Code)
	}
	if b := budget(); b != nil {
		t.Fatalf("budget after clear: %v", *b)
	}
}
