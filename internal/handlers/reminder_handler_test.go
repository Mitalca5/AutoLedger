package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type reminderAPI struct {
	t      *testing.T
	router chi.Router
	repo   *database.Repository
	owner  string
	viewer string
	vid    string
}

func newReminderAPI(t *testing.T, tag string) *reminderAPI {
	t.Helper()
	repo := authTestRepo(t)
	ctx := context.Background()
	mk := func(name string) *models.User {
		u, err := repo.CreateUser(ctx, name+"-"+tag+"@example.com", "hash")
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	owner, viewer := mk("owner"), mk("viewer")
	v := &models.Vehicle{UserID: owner.ID, Name: "Reminder car", Powertrain: models.PowertrainICE, CurrentOdometer: 30000}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddVehicleMember(ctx, v.ID, viewer.Email, models.RoleViewer, nil); err != nil {
		t.Fatal(err)
	}
	h := NewReminderHandler(repo, services.NewNotificationService(repo))
	r := chi.NewRouter()
	r.Get("/{vehicleId}/reminders", h.List)
	r.Post("/{vehicleId}/reminders", h.Create)
	r.Put("/{vehicleId}/reminders/{reminderId}", h.Update)
	r.Post("/{vehicleId}/reminders/{reminderId}/complete", h.Complete)
	r.Delete("/{vehicleId}/reminders/{reminderId}", h.Delete)
	r.Get("/{vehicleId}/webhook", h.GetWebhook)
	r.Put("/{vehicleId}/webhook", h.SaveWebhook)
	r.Delete("/{vehicleId}/webhook", h.DeleteWebhook)
	r.Post("/{vehicleId}/webhook/test", h.TestWebhook)
	return &reminderAPI{t: t, router: r, repo: repo, owner: owner.ID, viewer: viewer.ID, vid: v.ID}
}

func (a *reminderAPI) do(userID, method, path string, body any) *httptest.ResponseRecorder {
	var raw []byte
	switch b := body.(type) {
	case nil:
	case string:
		raw = []byte(b)
	default:
		raw, _ = json.Marshal(b)
	}
	req := httptest.NewRequest(method, "/"+a.vid+path, bytes.NewReader(raw))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	return rec
}

func (a *reminderAPI) want(rec *httptest.ResponseRecorder, status int, code string) {
	a.t.Helper()
	if rec.Code != status || (code != "" && responseCode(rec) != code) {
		a.t.Fatalf("got %d %s, want %d %s", rec.Code, rec.Body.String(), status, code)
	}
}

func TestReminderValidation(t *testing.T) {
	a := newReminderAPI(t, "validation")

	a.want(a.do(a.owner, http.MethodPost, "/reminders", "{"), http.StatusBadRequest, "request.invalid_body")
	a.want(a.do(a.owner, http.MethodPost, "/reminders", map[string]any{"title": "  "}), http.StatusBadRequest, "reminder.title_required")
	a.want(a.do(a.owner, http.MethodPost, "/reminders", map[string]any{"title": "T", "scheduled_date": "next week"}), http.StatusBadRequest, "reminder.invalid_date")
	a.want(a.do(a.owner, http.MethodPost, "/reminders", map[string]any{"title": "T", "scheduled_date": "2027-01-01", "interval_months": 6}), http.StatusBadRequest, "reminder.schedule_conflict")
	a.want(a.do(a.viewer, http.MethodPost, "/reminders", map[string]any{"title": "T", "interval_km": 1000}), http.StatusForbidden, "")
	a.want(a.do(a.owner, http.MethodPut, "/reminders/00000000-0000-0000-0000-000000000000", "{"), http.StatusBadRequest, "request.invalid_body")
	a.want(a.do(a.owner, http.MethodPut, "/reminders/00000000-0000-0000-0000-000000000000", map[string]any{"title": ""}), http.StatusBadRequest, "reminder.title_required")
	a.want(a.do(a.owner, http.MethodPut, "/reminders/00000000-0000-0000-0000-000000000000", map[string]any{"title": "Ghost", "interval_km": 1}), http.StatusNotFound, "")
	a.want(a.do(a.owner, http.MethodPost, "/reminders/00000000-0000-0000-0000-000000000000/complete", "{"), http.StatusBadRequest, "request.invalid_body")
	a.want(a.do(a.owner, http.MethodPost, "/reminders/00000000-0000-0000-0000-000000000000/complete", map[string]any{"maintenance_id": "00000000-0000-0000-0000-000000000001"}), http.StatusBadRequest, "reminder.invalid_maintenance")
	a.want(a.do(a.owner, http.MethodPost, "/reminders/00000000-0000-0000-0000-000000000000/complete", map[string]any{}), http.StatusNotFound, "")
	a.want(a.do(a.owner, http.MethodDelete, "/reminders/00000000-0000-0000-0000-000000000000", nil), http.StatusNotFound, "")
	a.want(a.do(a.viewer, http.MethodDelete, "/reminders/00000000-0000-0000-0000-000000000000", nil), http.StatusForbidden, "")
}

func TestReminderLifecycleAndObservedIntervals(t *testing.T) {
	a := newReminderAPI(t, "lifecycle")

	rec := a.do(a.owner, http.MethodPost, "/reminders", map[string]any{
		"title": "  Brake fluid ", "interval_km": 20000, "interval_months": 24,
		"last_service_date": "2025-01-10", "last_service_odometer": 10000, "lead_km": -1, "lead_days": 0,
	})
	a.want(rec, http.StatusCreated, "")
	var created models.MaintenanceReminder
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Title != "Brake fluid" || created.Category != "MAINTENANCE" || created.LeadKm != 1000 || created.LeadDays != 30 ||
		created.LastServiceDate == nil || created.LastServiceDate.Format("2006-01-02") != "2025-01-10" {
		t.Fatalf("defaults and trimming: %+v", created)
	}

	// An RFC 3339 last service date is accepted, a yearly date reminder keeps its flag, a bad last date is ignored.
	rec = a.do(a.owner, http.MethodPost, "/reminders", map[string]any{
		"title": "Inspection", "scheduled_date": "2027-06-01T10:00:00Z", "repeat_yearly": true, "last_service_date": "2025-05-01T00:00:00Z",
	})
	a.want(rec, http.StatusCreated, "")
	var yearly models.MaintenanceReminder
	_ = json.Unmarshal(rec.Body.Bytes(), &yearly)
	if !yearly.RepeatYearly || yearly.ScheduledDate == nil || yearly.LastServiceDate == nil {
		t.Fatalf("yearly reminder: %+v", yearly)
	}
	rec = a.do(a.owner, http.MethodPost, "/reminders", map[string]any{"title": "Odd", "interval_km": 5000, "last_service_date": "yesterday"})
	a.want(rec, http.StatusCreated, "")
	var odd models.MaintenanceReminder
	_ = json.Unmarshal(rec.Body.Bytes(), &odd)
	if odd.LastServiceDate != nil {
		t.Fatalf("an unreadable last service date is ignored: %+v", odd.LastServiceDate)
	}

	rec = a.do(a.owner, http.MethodPut, "/reminders/"+created.ID, map[string]any{"title": "Brake fluid", "interval_km": 30000})
	a.want(rec, http.StatusOK, "")
	var updated models.MaintenanceReminder
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.IntervalKm == nil || *updated.IntervalKm != 30000 {
		t.Fatalf("update: %+v", updated)
	}

	// Two completions give the list an observed interval.
	a.want(a.do(a.owner, http.MethodPost, "/reminders/"+created.ID+"/complete", map[string]any{"completed_date": "2025-06-01", "completed_odometer": 20000}), http.StatusOK, "")
	rec = a.do(a.owner, http.MethodPost, "/reminders/"+created.ID+"/complete", map[string]any{"completed_date": "2026-06-01", "completed_odometer": 38000})
	a.want(rec, http.StatusOK, "")
	var completed models.MaintenanceReminder
	_ = json.Unmarshal(rec.Body.Bytes(), &completed)
	if completed.LastServiceOdometer == nil || *completed.LastServiceOdometer != 38000 {
		t.Fatalf("complete: %+v", completed)
	}

	rec = a.do(a.viewer, http.MethodGet, "/reminders", nil)
	a.want(rec, http.StatusOK, "")
	var list []models.MaintenanceReminder
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 3 {
		t.Fatalf("list: %v %s", err, rec.Body.String())
	}
	for _, r := range list {
		if r.ID != created.ID {
			continue
		}
		if r.ObservedIntervalKm == nil || r.ObservedIntervalMonths == nil {
			t.Fatalf("a reminder completed twice must report its observed interval: %+v", r)
		}
	}

	a.want(a.do(a.owner, http.MethodDelete, "/reminders/"+created.ID, nil), http.StatusOK, "")
	rec = a.do(a.owner, http.MethodGet, "/reminders", nil)
	list = nil
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 2 {
		t.Fatalf("list after delete: %d", len(list))
	}
}

func TestReminderWebhook(t *testing.T) {
	a := newReminderAPI(t, "webhook")

	var received int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failing.Close()

	a.want(a.do(a.owner, http.MethodPut, "/webhook", "{"), http.StatusBadRequest, "request.invalid_body")
	a.want(a.do(a.owner, http.MethodPut, "/webhook", map[string]any{"url": "  "}), http.StatusBadRequest, "vehicle.url_required")
	a.want(a.do(a.viewer, http.MethodPut, "/webhook", map[string]any{"url": target.URL}), http.StatusForbidden, "")

	rec := a.do(a.owner, http.MethodPut, "/webhook", map[string]any{"url": " " + target.URL + " ", "type": "discord", "enabled": true})
	a.want(rec, http.StatusOK, "")
	var saved models.VehicleWebhook
	_ = json.Unmarshal(rec.Body.Bytes(), &saved)
	if saved.URL != target.URL || saved.Type != "DISCORD" || !saved.Enabled {
		t.Fatalf("saved = %+v", saved)
	}
	rec = a.do(a.owner, http.MethodPut, "/webhook", map[string]any{"url": target.URL})
	_ = json.Unmarshal(rec.Body.Bytes(), &saved)
	if saved.Type != "GENERIC" {
		t.Fatalf("the type defaults to GENERIC: %+v", saved)
	}
	a.want(a.do(a.viewer, http.MethodGet, "/webhook", nil), http.StatusOK, "")

	a.want(a.do(a.owner, http.MethodPost, "/webhook/test", "{"), http.StatusBadRequest, "request.invalid_body")
	a.want(a.do(a.owner, http.MethodPost, "/webhook/test", map[string]any{"url": ""}), http.StatusBadRequest, "vehicle.url_required")
	a.want(a.do(a.owner, http.MethodPost, "/webhook/test", map[string]any{"url": target.URL, "type": "SLACK"}), http.StatusOK, "")
	if received != 1 {
		t.Fatalf("the test notification must reach the endpoint once, got %d", received)
	}
	a.want(a.do(a.owner, http.MethodPost, "/webhook/test", map[string]any{"url": failing.URL}), http.StatusBadGateway, "webhook.test_failed")

	a.want(a.do(a.viewer, http.MethodDelete, "/webhook", nil), http.StatusForbidden, "")
	a.want(a.do(a.owner, http.MethodDelete, "/webhook", nil), http.StatusOK, "")
}
