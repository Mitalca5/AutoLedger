package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

	summary, err := repo.GetFleetSummary(ctx, u.ID, "UTC")
	if err != nil {
		t.Fatalf("GetFleetSummary failed: %v", err)
	}
	if len(summary.Vehicles) != 1 {
		t.Fatalf("expected 1 vehicle, got %d", len(summary.Vehicles))
	}
	m := summary.Vehicles[0]
	fleet, err := services.NewFleetService(repo, services.NewTCOService(pool, "UTC")).GetSummary(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}
	if got := fleet.Vehicles[0].EnergyCostPer100Km; got != 500 {
		t.Errorf("energy cost per 100 km: got %v, want 500 (1000.00 spent over 200 km)", got)
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
	h := NewFleetHandler(services.NewFleetService(repo, services.NewTCOService(repo.Pool(), "UTC")))
	put := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/fleet/budget", strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, u.ID))
		rec := httptest.NewRecorder()
		h.SetBudget(rec, req)
		return rec
	}
	budget := func() *int64 {
		s, err := repo.GetFleetSummary(ctx, u.ID, "UTC")
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

func TestFleetSummaryCarriesComparisonFigures(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "fleet-compare@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "EV", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	pool := repo.Pool()
	if _, err := pool.Exec(ctx, `INSERT INTO charge_logs (vehicle_id, date, kwh_added, cost, cost_source, currency, is_manual)
		VALUES ($1, now(), 40, 20, 'MANUAL', 'EUR', TRUE)`, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO drives (vehicle_id, start_time, end_time, distance_km, duration_min)
		VALUES ($1, now() - interval '1 hour', now(), 200, 60)`, v.ID); err != nil {
		t.Fatal(err)
	}

	svc := services.NewFleetService(repo, services.NewTCOService(pool, "UTC"))
	summary, err := svc.GetSummary(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	m := summary.Vehicles[0]
	if m.RunningCostPerKm == nil || *m.RunningCostPerKm <= 0 || m.FullCostPerKm == nil || *m.FullCostPerKm <= 0 {
		t.Errorf("per-km costs missing: running %v full %v", m.RunningCostPerKm, m.FullCostPerKm)
	}
	if m.KwhPer100Km == nil || *m.KwhPer100Km != 20 {
		t.Errorf("kWh per 100 km: got %v, want 20 (40 kWh over 200 km)", m.KwhPer100Km)
	}
	if m.Powertrain != "EV" || m.AnnualCost != nil {
		t.Errorf("powertrain %q, annual cost %v (one month of history gives none)", m.Powertrain, m.AnnualCost)
	}
}

func TestFleetSummaryMonthsFollowTheReportingTimezone(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skip("timezone database unavailable")
	}
	now := time.Now().In(paris)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, paris)
	if now.Before(monthStart.Add(3 * time.Hour)) {
		t.Skip("too close to the month boundary to tell the timezones apart")
	}
	u, err := repo.CreateUser(ctx, "fleet-tz@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "EV", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	// Thirty minutes into the month in Paris, which is still the last day of the previous month in UTC.
	start := monthStart.Add(30 * time.Minute)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO drives (vehicle_id, start_time, end_time, distance_km, duration_min)
		VALUES ($1, $2, $3, 10, 30)`, v.ID, start, start.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}

	inParis, err := repo.GetFleetSummary(ctx, u.ID, "Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}
	inUTC, err := repo.GetFleetSummary(ctx, u.ID, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if inParis.CurrentMonth != now.Format("2006-01") {
		t.Fatalf("current month = %q, want %q", inParis.CurrentMonth, now.Format("2006-01"))
	}
	last := func(s *models.FleetSummaryResponse) models.FleetMonthlyCost {
		return s.MonthlyCosts[len(s.MonthlyCosts)-1]
	}
	if last(inParis).Month != inParis.CurrentMonth || last(inParis).DistanceKm != 10 {
		t.Fatalf("in Paris the drive belongs to the current month: %+v", last(inParis))
	}
	if last(inUTC).DistanceKm != 0 {
		t.Fatalf("in UTC the same drive belongs to the previous month: %+v", last(inUTC))
	}
}
