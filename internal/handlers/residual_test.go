package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

func TestBatteryReadingsAndResidualValue(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "residual@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "Car", TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 30000}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	h := NewResidualHandler(repo, services.NewResidualService(repo, nil))
	do := func(call func(http.ResponseWriter, *http.Request), method, url string, params map[string]string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vehicleId", v.ID)
		for k, val := range params {
			rctx.URLParams.Add(k, val)
		}
		req := httptest.NewRequest(method, url, bytes.NewReader(raw))
		req = req.WithContext(context.WithValue(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), middleware.UserIDKey, u.ID))
		rec := httptest.NewRecorder()
		call(rec, req)
		return rec
	}

	for _, bad := range []map[string]any{{"date": "2026-01-01"}, {"date": "x", "health_percent": 90}, {"date": "2026-01-01", "health_percent": 140}} {
		if rec := do(h.SaveReading, http.MethodPost, "/", nil, bad); rec.Code != http.StatusBadRequest {
			t.Fatalf("%v: got %d", bad, rec.Code)
		}
	}
	rec := do(h.SaveReading, http.MethodPost, "/", nil, map[string]any{"date": "2026-03-01", "health_percent": 93.5, "max_capacity_kwh": 60})
	var summary models.BatteryHealthSummary
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &summary) != nil || summary.HealthPercent == nil || *summary.HealthPercent != 93.5 || summary.Source != "reading" {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}

	if rec := do(h.Residual, http.MethodGet, "/", nil, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("no ownership must be refused, got %d", rec.Code)
	}
	price, resale, months, odo := money.FromFloat(40000), money.FromFloat(20000), 48, 0.0
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{
		VehicleID: v.ID, AcquisitionType: models.AcquisitionCash, StartDate: time.Now().AddDate(-2, 0, 0), StartOdometer: &odo,
		PurchasePrice: &price, ExpectedResaleValue: &resale, ExpectedHoldingMonths: &months,
	}); err != nil {
		t.Fatal(err)
	}
	rec = do(h.Residual, http.MethodGet, "/?expected_km=120000&km_share=0.5", nil, nil)
	var res models.ResidualValue
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &res) != nil {
		t.Fatalf("residual: %d %s", rec.Code, rec.Body)
	}
	if res.HealthPercent == nil || *res.HealthPercent != 93.5 || res.CurrentValue <= 0 || res.CurrentValue >= price || len(res.Curve) != 49 {
		t.Fatalf("unexpected projection: %+v", res)
	}
	if rec := do(h.Residual, http.MethodGet, "/?km_share=abc", nil, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad option: %d", rec.Code)
	}

	if rec := do(h.DeleteReading, http.MethodDelete, "/", map[string]string{"date": "2026-03-01"}, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := do(h.DeleteReading, http.MethodDelete, "/", map[string]string{"date": "2026-03-01"}, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("second delete: %d", rec.Code)
	}
}
