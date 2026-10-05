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
)

func TestReminderLinkedToMaintenance(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "reminder-link@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	newVehicle := func(name string) *models.Vehicle {
		v := &models.Vehicle{UserID: u.ID, Name: name, Powertrain: models.PowertrainICE, TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 30000}
		if err := repo.CreateVehicle(ctx, v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	veh, other := newVehicle("Mine"), newVehicle("Other")
	newMaintenance := func(vehicleID string, date time.Time, odo float64) *models.MaintenanceExpense {
		m := &models.MaintenanceExpense{VehicleID: vehicleID, Category: "MAINTENANCE", Amount: 12000, Currency: "EUR", Date: date, Odometer: &odo, Description: "Oil change"}
		if err := repo.CreateMaintenanceExpense(ctx, m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	service := newMaintenance(veh.ID, time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), 24000)
	foreign := newMaintenance(other.ID, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), 10000)

	h := NewReminderHandler(repo, nil)
	call := func(fn func(http.ResponseWriter, *http.Request), body any, params map[string]string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		rctx := chi.NewRouteContext()
		for k, v := range params {
			rctx.URLParams.Add(k, v)
		}
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
		req = req.WithContext(context.WithValue(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), middleware.UserIDKey, u.ID))
		rec := httptest.NewRecorder()
		fn(rec, req)
		return rec
	}
	vehicleParams := map[string]string{"vehicleId": veh.ID}
	list := func() []models.MaintenanceReminder {
		rec := call(h.List, nil, vehicleParams)
		var out []models.MaintenanceReminder
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	rec := call(h.Create, map[string]any{"title": "Oil", "interval_km": 15000, "maintenance_id": foreign.ID}, vehicleParams)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("a maintenance of another vehicle must be refused: %d %s", rec.Code, rec.Body)
	}

	rec = call(h.Create, map[string]any{"title": "Oil", "interval_km": 15000, "maintenance_id": service.ID}, vehicleParams)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create linked reminder: %d %s", rec.Code, rec.Body)
	}
	var created models.MaintenanceReminder
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.LastServiceDate == nil || created.LastServiceDate.Format("2006-01-02") != "2026-03-10" ||
		created.LastServiceOdometer == nil || *created.LastServiceOdometer != 24000 {
		t.Fatalf("starting point must come from the maintenance: %+v", created)
	}
	if got := list(); len(got) != 1 || got[0].Maintenance == nil || got[0].Maintenance.Description != "Oil change" {
		t.Fatalf("listing must carry the linked maintenance summary: %+v", got)
	}

	service.Date = time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	odo := 25000.0
	service.Odometer = &odo
	if err := repo.UpdateMaintenanceExpense(ctx, service); err != nil {
		t.Fatal(err)
	}
	got := list()[0]
	if got.LastServiceDate.Format("2006-01-02") != "2026-04-02" || *got.LastServiceOdometer != 25000 {
		t.Fatalf("editing the maintenance must move its reminders: %+v", got)
	}

	next := newMaintenance(veh.ID, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), 29000)
	params := map[string]string{"vehicleId": veh.ID, "reminderId": created.ID}
	rec = call(h.Complete, map[string]any{"completed_date": "2026-09-01", "completed_odometer": 29000, "maintenance_id": next.ID}, params)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete: %d %s", rec.Code, rec.Body)
	}
	if got := list()[0]; got.MaintenanceID == nil || *got.MaintenanceID != next.ID {
		t.Fatalf("completing with an expense must link it: %+v", got)
	}
	rec = call(h.Complete, map[string]any{"completed_date": "2026-09-02", "maintenance_id": foreign.ID}, params)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("completion must refuse a foreign maintenance: %d %s", rec.Code, rec.Body)
	}
	rec = call(h.Complete, map[string]any{"completed_date": "2026-09-03"}, params)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete without expense: %d", rec.Code)
	}
	if got := list()[0]; got.MaintenanceID != nil {
		t.Fatalf("a completion with no record drops the stale link: %+v", got)
	}

	rec = call(h.Update, map[string]any{"title": "Oil", "interval_km": 15000, "maintenance_id": next.ID}, params)
	if rec.Code != http.StatusOK {
		t.Fatalf("update link: %d %s", rec.Code, rec.Body)
	}
	if err := repo.DeleteMaintenanceExpense(ctx, veh.ID, next.ID); err != nil {
		t.Fatal(err)
	}
	if got := list()[0]; got.MaintenanceID != nil || got.Maintenance != nil || got.LastServiceDate == nil {
		t.Fatalf("deleting the maintenance unlinks but keeps the starting point: %+v", got)
	}
}
