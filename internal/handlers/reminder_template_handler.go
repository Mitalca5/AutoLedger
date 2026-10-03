package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

func (h *ReminderHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListReminderTemplates(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		writeRepoError(w, r, err, "Failed to list reminder templates")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": list})
}

type saveReminderTemplateRequest struct {
	Name  string                        `json:"name"`
	Items []models.ReminderTemplateItem `json:"items"`
	// FromVehicleID builds the items from that vehicle's reminders instead of Items.
	FromVehicleID string `json:"from_vehicle_id"`
}

func validTemplateItem(it *models.ReminderTemplateItem) bool {
	it.Title = strings.TrimSpace(it.Title)
	if it.Title == "" || len(it.Title) > 200 {
		return false
	}
	if it.Category == "" {
		it.Category = "MAINTENANCE"
	}
	for _, v := range []*int{it.IntervalKm, it.IntervalMonths} {
		if v != nil && *v <= 0 {
			return false
		}
	}
	return it.IntervalKm != nil || it.IntervalMonths != nil
}

func (h *ReminderHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req saveReminderTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 100 {
		writeAPIError(w, http.StatusBadRequest, apierror.New("reminder_template.name_required", "A name for the template is required"))
		return
	}
	if req.FromVehicleID != "" {
		veh := requireVehicleAccess(w, r, h.repo, req.FromVehicleID, models.RoleViewer)
		if veh == nil {
			return
		}
		reminders, err := h.repo.ListMaintenanceReminders(r.Context(), veh.ID, veh.CurrentOdometer)
		if err != nil {
			writeRepoError(w, r, err, "Failed to read the vehicle reminders")
			return
		}
		req.Items = nil
		for _, rem := range reminders {
			req.Items = append(req.Items, models.ReminderTemplateItem{
				Title: rem.Title, Category: rem.Category, IntervalKm: rem.IntervalKm, IntervalMonths: rem.IntervalMonths,
				LeadKm: rem.LeadKm, LeadDays: rem.LeadDays,
			})
		}
	}
	items := make([]models.ReminderTemplateItem, 0, len(req.Items))
	for _, it := range req.Items {
		if validTemplateItem(&it) {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		writeAPIError(w, http.StatusBadRequest, apierror.New("reminder_template.items_required", "A template needs at least one reminder with a title and an interval"))
		return
	}
	t := &models.ReminderTemplate{UserID: middleware.GetUserID(r.Context()), Name: req.Name, Items: items}
	if err := h.repo.CreateReminderTemplate(r.Context(), t); err != nil {
		writeRepoError(w, r, err, "Failed to create the reminder template")
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *ReminderHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.DeleteReminderTemplate(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r.Context())); err != nil {
		writeRepoError(w, r, err, "Failed to delete the reminder template")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// ApplyTemplate creates the template's reminders on a vehicle, counting from today and the current odometer.
// A reminder whose title the vehicle already has (case-insensitive) is skipped.
func (h *ReminderHandler) ApplyTemplate(w http.ResponseWriter, r *http.Request) {
	veh := requireVehicleAccess(w, r, h.repo, chi.URLParam(r, "vehicleId"), models.RoleEditor)
	if veh == nil {
		return
	}
	var req struct {
		TemplateID string `json:"template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}
	tpl, err := h.repo.GetReminderTemplate(r.Context(), req.TemplateID, middleware.GetUserID(r.Context()))
	if err != nil {
		writeRepoError(w, r, err, "Failed to read the reminder template")
		return
	}
	existing, err := h.repo.ListMaintenanceReminders(r.Context(), veh.ID, veh.CurrentOdometer)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list maintenance reminders")
		return
	}
	have := map[string]bool{}
	for _, e := range existing {
		have[strings.ToLower(e.Title)] = true
	}
	created := []models.MaintenanceReminder{}
	skipped := []string{}
	now := time.Now()
	for _, it := range tpl.Items {
		if have[strings.ToLower(it.Title)] {
			skipped = append(skipped, it.Title)
			continue
		}
		odo := veh.CurrentOdometer
		leadKm, leadDays := it.LeadKm, it.LeadDays
		if leadKm <= 0 {
			leadKm = 1000
		}
		if leadDays <= 0 {
			leadDays = 30
		}
		rem := &models.MaintenanceReminder{
			VehicleID: veh.ID, Title: it.Title, Category: it.Category, IntervalKm: it.IntervalKm, IntervalMonths: it.IntervalMonths,
			LastServiceOdometer: &odo, LastServiceDate: &now, LeadKm: leadKm, LeadDays: leadDays,
		}
		if err := h.repo.CreateMaintenanceReminder(r.Context(), rem); err != nil {
			writeRepoError(w, r, err, "Failed to create maintenance reminder")
			return
		}
		rem.ComputeStatus(veh.CurrentOdometer, now)
		created = append(created, *rem)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"created": created, "skipped": skipped})
}
