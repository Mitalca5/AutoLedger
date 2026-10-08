package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type CreateTripGroupRequest struct {
	Name     string   `json:"name"`
	Notes    *string  `json:"notes"`
	DriveIDs []string `json:"drive_ids"`
}

func (h *DriveHandler) CreateTripGroup(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req CreateTripGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if req.Name == "" {
		writeAPIError(w, http.StatusBadRequest, apierror.New("trip.name_required", "The trip name is required"))
		return
	}

	tg := &models.TripGroup{
		VehicleID: vehicleID,
		Name:      req.Name,
		Notes:     req.Notes,
	}

	if len(req.DriveIDs) == 0 {
		writeAPIError(w, http.StatusBadRequest, apierror.New("trip.group_needs_drive", "A group must contain at least one drive"))
		return
	}

	if err := h.repo.CreateTripGroup(r.Context(), tg, req.DriveIDs); err != nil {
		writeRepoError(w, r, err, "Failed to create trip group")
		return
	}

	writeJSON(w, http.StatusCreated, tg)
}

func (h *DriveHandler) ListTripGroups(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer); v == nil {
		return
	}

	groups, err := h.repo.ListTripGroups(r.Context(), vehicleID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apierror.New("internal", "Failed to list trip groups"))
		return
	}
	if groups == nil {
		groups = []models.TripGroup{}
	}

	writeJSON(w, http.StatusOK, groups)
}

// TripSuggestions lists chains of ungrouped drives that look like a single trip (short stops, or a charge in between).
func (h *DriveHandler) TripSuggestions(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer); v == nil {
		return
	}

	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 || days > 730 {
		days = 180
	}
	since := time.Now().AddDate(0, 0, -days)

	drives, err := h.repo.ListTripCandidateDrives(r.Context(), vehicleID, since)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apierror.New("internal", "Failed to detect trips"))
		return
	}
	charges, err := h.repo.ListChargeWindows(r.Context(), vehicleID, since)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apierror.New("internal", "Failed to detect trips"))
		return
	}

	suggestions := services.DetectTripSuggestions(drives, charges)
	if suggestions == nil {
		suggestions = []models.TripSuggestion{}
	}
	writeJSON(w, http.StatusOK, suggestions)
}

type DismissTripSuggestionRequest struct {
	DriveIDs []string `json:"drive_ids"`
}

// DismissTripSuggestion rules out a suggested trip: its drives are no longer proposed as a trip.
func (h *DriveHandler) DismissTripSuggestion(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req DismissTripSuggestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.DriveIDs) == 0 || len(req.DriveIDs) > 200 {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if err := h.repo.DismissTripSuggestion(r.Context(), vehicleID, req.DriveIDs); err != nil {
		writeRepoError(w, r, err, "Failed to dismiss the trip suggestion")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

type UpdateTripGroupRequest struct {
	Name     string   `json:"name"`
	Notes    *string  `json:"notes"`
	DriveIDs []string `json:"drive_ids"` // Omitted: drives unchanged
}

// UpdateTripGroup renames a trip group and optionally replaces its drives.
func (h *DriveHandler) UpdateTripGroup(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	var req UpdateTripGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeAPIError(w, http.StatusBadRequest, apierror.New("trip.name_required", "The trip name is required"))
		return
	}
	tg := &models.TripGroup{ID: chi.URLParam(r, "groupId"), VehicleID: vehicleID, Name: req.Name, Notes: req.Notes}
	if err := h.repo.UpdateTripGroup(r.Context(), tg, req.DriveIDs); err != nil {
		writeRepoError(w, r, err, "Failed to update trip group")
		return
	}
	writeJSON(w, http.StatusOK, tg)
}

// DeleteTripGroup deletes a trip group; ?delete_expenses=true also deletes the expenses attached to it.
func (h *DriveHandler) DeleteTripGroup(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	deleteExpenses := r.URL.Query().Get("delete_expenses") == "true"
	if err := h.repo.DeleteTripGroup(r.Context(), vehicleID, chi.URLParam(r, "groupId"), deleteExpenses); err != nil {
		writeRepoError(w, r, err, "Failed to delete trip group")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
