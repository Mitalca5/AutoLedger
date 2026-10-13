package handlers

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestComparisonPriceHistoryAccessAndMedian(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	owner, err := repo.CreateUser(ctx, "price-owner@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	outsider, err := repo.CreateUser(ctx, "price-outsider@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := repo.CreateUser(ctx, "price-viewer@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	vehicle := &models.Vehicle{UserID: owner.ID, Name: "Price history", Currency: "EUR", Powertrain: models.PowertrainPHEV, TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, vehicle); err != nil {
		t.Fatal(err)
	}
	other := &models.Vehicle{UserID: owner.ID, Name: "Other", Currency: "EUR", Powertrain: models.PowertrainPHEV, TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, other); err != nil {
		t.Fatal(err)
	}
	for _, price := range []float64{0, 0.2, 0.3, 0.4, 1} {
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO charge_logs (vehicle_id, date, kwh_added, cost, currency) VALUES ($1, NOW(), 10, $2, 'EUR')`, vehicle.ID, money.FromFloat(price*10)); err != nil {
			t.Fatal(err)
		}
	}
	// Foreign prices are converted; missing cost/rate, zero energy and deleted records are not observations.
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO charge_logs (vehicle_id, date, kwh_added, cost, currency, fx_rate, deleted_upstream_at) VALUES
		($1, NOW(), 50, 10, 'USD', 2, NULL),
		($1, NOW(), 10, NULL, 'EUR', NULL, NULL),
		($1, NOW(), 10, 1000, 'USD', NULL, NULL),
		($1, NOW(), 0, 1000, 'EUR', NULL, NULL),
		($1, NOW(), 10, 99999, 'EUR', NULL, NOW()),
		($2, NOW(), 10, 99999, 'EUR', NULL, NULL)`, vehicle.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	for _, price := range []float64{1.5, 1.6, 1.7, 1.8, 9} {
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO fuel_logs (vehicle_id, date, liters, amount, fuel_type) VALUES ($1, NOW(), 10, $2, 'SP95_E10')`, vehicle.ID, money.FromFloat(price*10)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO fuel_logs (vehicle_id, date, liters, amount, fuel_type) VALUES ($1, NOW(), 10, 99999, 'DIESEL')`, vehicle.ID); err != nil {
		t.Fatal(err)
	}
	// A fill-up without volume has no unit price and is not an observation.
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO fuel_logs (vehicle_id, date, liters, amount, price_per_liter, fuel_type) VALUES ($1, NOW(), NULL, 99999, 50, 'SP95_E10')`, vehicle.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddVehicleMember(ctx, vehicle.ID, viewer.Email, models.RoleViewer, nil); err != nil {
		t.Fatal(err)
	}
	h := NewComparisonHandler(repo, nil)
	request := func(userID, vehicleID string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/?vehicle_id="+vehicleID+"&fuel_type=SP95_E10", nil)
		r = r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, userID))
		w := httptest.NewRecorder()
		h.PriceHistory(w, r)
		return w
	}
	w := request(outsider.ID, vehicle.ID)
	if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "median") {
		t.Fatalf("another user's prices disclosed: %d %s", w.Code, w.Body.String())
	}
	if w = request(owner.ID, ""); w.Code != http.StatusNotFound {
		t.Fatalf("missing vehicle: status %d", w.Code)
	}
	if w = request(viewer.ID, vehicle.ID); w.Code != http.StatusOK {
		t.Fatalf("shared viewer: status %d: %s", w.Code, w.Body.String())
	}
	w = request(owner.ID, vehicle.ID)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var history models.ComparisonPriceHistory
	if err := json.Unmarshal(w.Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if history.Currency != "EUR" || history.Fuel.Count != 5 || history.Fuel.Median == nil || math.Abs(*history.Fuel.Median-1.7) > 1e-9 {
		t.Fatalf("fuel history: %+v", history)
	}
	if history.Electricity.Count != 6 || history.Electricity.Median == nil || math.Abs(*history.Electricity.Median-0.35) > 1e-9 {
		t.Fatalf("electricity history: %+v", history.Electricity)
	}
	w = request(owner.ID, other.ID)
	if err := json.Unmarshal(w.Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if history.Fuel.Count != 0 || history.Fuel.Median != nil {
		t.Fatal("empty history is not unknown")
	}
}
