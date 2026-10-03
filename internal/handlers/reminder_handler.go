package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type ReminderHandler struct {
	repo          *database.Repository
	notifications *services.NotificationService
}

func NewReminderHandler(repo *database.Repository, notifications *services.NotificationService) *ReminderHandler {
	return &ReminderHandler{
		repo:          repo,
		notifications: notifications,
	}
}

type ReminderPayload struct {
	Title               string   `json:"title"`
	Category            string   `json:"category"`
	IntervalKm          *int     `json:"interval_km"`
	IntervalMonths      *int     `json:"interval_months"`
	LastServiceOdometer *float64 `json:"last_service_odometer"`
	LastServiceDate     *string  `json:"last_service_date"`
	LeadKm              int      `json:"lead_km"`
	LeadDays            int      `json:"lead_days"`
	WebhookEnabled      bool     `json:"webhook_enabled"`
}

func (h *ReminderHandler) List(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	veh := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer)
	if veh == nil {
		return
	}

	reminders, err := h.repo.ListMaintenanceReminders(r.Context(), vehicleID, veh.CurrentOdometer)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list maintenance reminders")
		return
	}
	if reminders == nil {
		reminders = []models.MaintenanceReminder{}
	}
	if observed, oerr := h.repo.ObservedReminderIntervals(r.Context(), vehicleID); oerr == nil {
		for i := range reminders {
			o, ok := observed[reminders[i].ID]
			if !ok {
				continue
			}
			if o.AvgKm != nil {
				km := int(math.Round(*o.AvgKm))
				reminders[i].ObservedIntervalKm = &km
			}
			if o.AvgDays != nil {
				months := math.Round(*o.AvgDays/30.4375*10) / 10
				reminders[i].ObservedIntervalMonths = &months
			}
		}
	}

	writeJSON(w, http.StatusOK, reminders)
}

func (h *ReminderHandler) buildReminder(req ReminderPayload, vehicleID, reminderID string) (*models.MaintenanceReminder, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, apierror.New("reminder.title_required", "Title is required")
	}

	leadKm := req.LeadKm
	if leadKm <= 0 {
		leadKm = 1000
	}
	leadDays := req.LeadDays
	if leadDays <= 0 {
		leadDays = 30
	}
	cat := strings.TrimSpace(req.Category)
	if cat == "" {
		cat = "MAINTENANCE"
	}

	rem := &models.MaintenanceReminder{
		ID:                  reminderID,
		VehicleID:           vehicleID,
		Title:               title,
		Category:            cat,
		IntervalKm:          req.IntervalKm,
		IntervalMonths:      req.IntervalMonths,
		LastServiceOdometer: req.LastServiceOdometer,
		LeadKm:              leadKm,
		LeadDays:            leadDays,
		WebhookEnabled:      req.WebhookEnabled,
	}

	if req.LastServiceDate != nil && *req.LastServiceDate != "" {
		if t, err := time.Parse("2006-01-02", *req.LastServiceDate); err == nil {
			rem.LastServiceDate = &t
		} else if t, err := time.Parse(time.RFC3339, *req.LastServiceDate); err == nil {
			rem.LastServiceDate = &t
		}
	}

	return rem, nil
}

func (h *ReminderHandler) Create(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	veh := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if veh == nil {
		return
	}

	var req ReminderPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	rem, err := h.buildReminder(req, vehicleID, "")
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	if err := h.repo.CreateMaintenanceReminder(r.Context(), rem); err != nil {
		writeRepoError(w, r, err, "Failed to create maintenance reminder")
		return
	}

	rem.ComputeStatus(veh.CurrentOdometer, time.Now())
	writeJSON(w, http.StatusCreated, rem)
}

func (h *ReminderHandler) Update(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	reminderID := chi.URLParam(r, "reminderId")

	veh := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if veh == nil {
		return
	}

	var req ReminderPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	rem, err := h.buildReminder(req, vehicleID, reminderID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	if err := h.repo.UpdateMaintenanceReminder(r.Context(), rem); err != nil {
		writeRepoError(w, r, err, "Failed to update maintenance reminder")
		return
	}

	rem.ComputeStatus(veh.CurrentOdometer, time.Now())
	writeJSON(w, http.StatusOK, rem)
}

type CompletePayload struct {
	CompletedDate     string   `json:"completed_date"`
	CompletedOdometer *float64 `json:"completed_odometer"`
}

func (h *ReminderHandler) Complete(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	reminderID := chi.URLParam(r, "reminderId")

	veh := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if veh == nil {
		return
	}

	var req CompletePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	completedDate := time.Now()
	if req.CompletedDate != "" {
		if t, err := time.Parse("2006-01-02", req.CompletedDate); err == nil {
			completedDate = t
		}
	}

	completedOdo := veh.CurrentOdometer
	if req.CompletedOdometer != nil && *req.CompletedOdometer > 0 {
		completedOdo = *req.CompletedOdometer
	}

	if err := h.repo.CompleteMaintenanceReminder(r.Context(), vehicleID, reminderID, completedDate, completedOdo); err != nil {
		writeRepoError(w, r, err, "Failed to complete maintenance reminder")
		return
	}

	rem, err := h.repo.GetMaintenanceReminderByID(r.Context(), vehicleID, reminderID, veh.CurrentOdometer)
	if err != nil {
		writeRepoError(w, r, err, "Failed to fetch updated reminder")
		return
	}

	writeJSON(w, http.StatusOK, rem)
}

func (h *ReminderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	reminderID := chi.URLParam(r, "reminderId")

	if veh := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); veh == nil {
		return
	}

	if err := h.repo.DeleteMaintenanceReminder(r.Context(), vehicleID, reminderID); err != nil {
		writeRepoError(w, r, err, "Failed to delete maintenance reminder")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// Webhook endpoints

func (h *ReminderHandler) GetWebhook(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	if veh := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer); veh == nil {
		return
	}

	webhook, err := h.repo.GetVehicleWebhook(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to fetch vehicle webhook")
		return
	}

	writeJSON(w, http.StatusOK, webhook)
}

type WebhookPayload struct {
	URL     string `json:"url"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

func (h *ReminderHandler) SaveWebhook(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	if veh := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); veh == nil {
		return
	}

	var req WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	url := strings.TrimSpace(req.URL)
	if url == "" {
		writeAPIError(w, http.StatusBadRequest, apierror.New("vehicle.url_required", "URL is required"))
		return
	}

	wType := strings.ToUpper(strings.TrimSpace(req.Type))
	if wType == "" {
		wType = "GENERIC"
	}

	webhook := &models.VehicleWebhook{
		VehicleID: vehicleID,
		URL:       url,
		Type:      wType,
		Enabled:   req.Enabled,
	}

	if err := h.repo.UpsertVehicleWebhook(r.Context(), webhook); err != nil {
		writeRepoError(w, r, err, "Failed to save vehicle webhook")
		return
	}

	writeJSON(w, http.StatusOK, webhook)
}

func (h *ReminderHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	if veh := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); veh == nil {
		return
	}

	if err := h.repo.DeleteVehicleWebhook(r.Context(), vehicleID); err != nil {
		writeRepoError(w, r, err, "Failed to delete vehicle webhook")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *ReminderHandler) TestWebhook(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	veh := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if veh == nil {
		return
	}

	var req WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	url := strings.TrimSpace(req.URL)
	if url == "" {
		writeAPIError(w, http.StatusBadRequest, apierror.New("vehicle.url_required", "URL is required"))
		return
	}

	webhook := &models.VehicleWebhook{
		VehicleID: vehicleID,
		URL:       url,
		Type:      req.Type,
		Enabled:   true,
	}

	if err := h.notifications.TestWebhook(r.Context(), webhook, veh); err != nil {
		writeAPIError(w, http.StatusBadGateway, apierror.Newf("webhook.test_failed", "Webhook test failed: %s", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Webhook test sent successfully"})
}
