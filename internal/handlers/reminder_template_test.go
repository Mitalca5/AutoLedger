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

func TestReminderTemplatesApplyAndObservedIntervals(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "reminder-tpl@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := repo.CreateUser(ctx, "reminder-tpl-other@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	newVehicle := func(name string, odo float64) *models.Vehicle {
		v := &models.Vehicle{UserID: u.ID, Name: name, Powertrain: models.PowertrainICE, TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: odo}
		if err := repo.CreateVehicle(ctx, v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	src, dst := newVehicle("Source", 50000), newVehicle("Target", 12000)

	h := NewReminderHandler(repo, nil)
	do := func(userID, vehicleID string, call func(http.ResponseWriter, *http.Request), body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vehicleId", vehicleID)
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
		req = req.WithContext(context.WithValue(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), middleware.UserIDKey, userID))
		rec := httptest.NewRecorder()
		call(rec, req)
		return rec
	}

	km, months := 15000, 12
	for _, title := range []string{"Oil change", "Brake fluid"} {
		rem := &models.MaintenanceReminder{VehicleID: src.ID, Title: title, Category: "MAINTENANCE", IntervalKm: &km, IntervalMonths: &months, LeadKm: 1000, LeadDays: 30}
		if err := repo.CreateMaintenanceReminder(ctx, rem); err != nil {
			t.Fatal(err)
		}
	}

	rec := do(u.ID, "", h.CreateTemplate, map[string]any{"name": "My plan", "from_vehicle_id": src.ID})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create template: %d %s", rec.Code, rec.Body)
	}
	var tpl models.ReminderTemplate
	_ = json.Unmarshal(rec.Body.Bytes(), &tpl)
	if len(tpl.Items) != 2 {
		t.Fatalf("template items: %+v", tpl.Items)
	}
	if rec := do(u.ID, "", h.CreateTemplate, map[string]any{"name": "Empty", "items": []any{}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("an empty template must be refused: %d", rec.Code)
	}

	if rec := do(stranger.ID, dst.ID, h.ApplyTemplate, map[string]any{"template_id": tpl.ID}); rec.Code == http.StatusCreated {
		t.Fatalf("a stranger must not apply to another user's vehicle: %d", rec.Code)
	}
	otherTpl := &models.ReminderTemplate{UserID: stranger.ID, Name: "Theirs", Items: tpl.Items}
	if err := repo.CreateReminderTemplate(ctx, otherTpl); err != nil {
		t.Fatal(err)
	}
	if rec := do(u.ID, dst.ID, h.ApplyTemplate, map[string]any{"template_id": otherTpl.ID}); rec.Code == http.StatusCreated {
		t.Fatalf("another user's template must not be applied: %d", rec.Code)
	}

	// The target already has one of the two titles: only the other is created.
	have := &models.MaintenanceReminder{VehicleID: dst.ID, Title: "oil CHANGE", Category: "MAINTENANCE", IntervalKm: &km, LeadKm: 1000, LeadDays: 30}
	if err := repo.CreateMaintenanceReminder(ctx, have); err != nil {
		t.Fatal(err)
	}
	rec = do(u.ID, dst.ID, h.ApplyTemplate, map[string]any{"template_id": tpl.ID})
	var applied struct {
		Created []models.MaintenanceReminder `json:"created"`
		Skipped []string                     `json:"skipped"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &applied)
	if rec.Code != http.StatusCreated || len(applied.Created) != 1 || applied.Created[0].Title != "Brake fluid" || len(applied.Skipped) != 1 {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body)
	}
	if applied.Created[0].Status != "OK" || applied.Created[0].LastServiceOdometer == nil || *applied.Created[0].LastServiceOdometer != 12000 {
		t.Fatalf("a new reminder counts from the current odometer: %+v", applied.Created[0])
	}

	// Two completions give an observed interval.
	rem := applied.Created[0]
	for _, c := range []struct {
		day string
		odo float64
	}{{"2025-01-10", 20000}, {"2026-01-10", 32000}} {
		d, _ := time.Parse("2006-01-02", c.day)
		if err := repo.CompleteMaintenanceReminder(ctx, dst.ID, rem.ID, d, c.odo, nil); err != nil {
			t.Fatal(err)
		}
	}
	list := httptest.NewRecorder()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("vehicleId", dst.ID)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), middleware.UserIDKey, u.ID))
	h.List(list, req)
	var reminders []models.MaintenanceReminder
	_ = json.Unmarshal(list.Body.Bytes(), &reminders)
	for _, r := range reminders {
		if r.ID == rem.ID {
			if r.ObservedIntervalKm == nil || *r.ObservedIntervalKm != 12000 || r.ObservedIntervalMonths == nil || *r.ObservedIntervalMonths != 12 {
				t.Fatalf("observed interval: %v %v", r.ObservedIntervalKm, r.ObservedIntervalMonths)
			}
			return
		}
		if r.ObservedIntervalKm != nil {
			t.Fatalf("one completion or none gives no observed interval: %+v", r)
		}
	}
	t.Fatal("reminder missing from the list")
}
