package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type ResidualHandler struct {
	repo     *database.Repository
	residual *services.ResidualService
}

func NewResidualHandler(repo *database.Repository, residual *services.ResidualService) *ResidualHandler {
	return &ResidualHandler{repo: repo, residual: residual}
}

// BatteryHealth returns the readings and the best known state of health of the battery.
func (h *ResidualHandler) BatteryHealth(w http.ResponseWriter, r *http.Request) {
	vehicle := requireVehicleAccess(w, r, h.repo, chi.URLParam(r, "vehicleId"), models.RoleViewer)
	if vehicle == nil {
		return
	}
	summary, err := h.residual.BatteryHealth(r.Context(), vehicle.ID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to read the battery health")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

type batteryReadingRequest struct {
	Date               string   `json:"date"`
	HealthPercent      *float64 `json:"health_percent"`
	CurrentCapacityKwh *float64 `json:"current_capacity_kwh"`
	MaxCapacityKwh     *float64 `json:"max_capacity_kwh"`
}

func plausible(v *float64, lo, hi float64) bool { return v == nil || (*v >= lo && *v <= hi) }

// SaveReading records a battery reading typed by the user (an OBD2 scan, a garage report); one reading a day.
func (h *ResidualHandler) SaveReading(w http.ResponseWriter, r *http.Request) {
	vehicle := requireVehicleAccess(w, r, h.repo, chi.URLParam(r, "vehicleId"), models.RoleEditor)
	if vehicle == nil {
		return
	}
	var req batteryReadingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}
	day, err := parseDate(req.Date)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("battery.reading_invalid", "Invalid battery reading"))
		return
	}
	empty := req.HealthPercent == nil && req.CurrentCapacityKwh == nil && req.MaxCapacityKwh == nil
	if empty || !plausible(req.HealthPercent, 1, 100) || !plausible(req.CurrentCapacityKwh, 1, 500) || !plausible(req.MaxCapacityKwh, 1, 500) {
		writeAPIError(w, http.StatusBadRequest, apierror.New("battery.reading_invalid", "Invalid battery reading"))
		return
	}
	snap := models.BatterySnapshot{HealthPercent: req.HealthPercent, CurrentCapacityKwh: req.CurrentCapacityKwh, MaxCapacityKwh: req.MaxCapacityKwh}
	if err := h.repo.UpsertBatterySnapshot(r.Context(), vehicle.ID, day, snap); err != nil {
		writeRepoError(w, r, err, "Failed to save the battery reading")
		return
	}
	h.BatteryHealth(w, r)
}

func (h *ResidualHandler) DeleteReading(w http.ResponseWriter, r *http.Request) {
	vehicle := requireVehicleAccess(w, r, h.repo, chi.URLParam(r, "vehicleId"), models.RoleEditor)
	if vehicle == nil {
		return
	}
	day, err := parseDate(chi.URLParam(r, "date"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("battery.reading_invalid", "Invalid battery reading"))
		return
	}
	if err := h.repo.DeleteBatterySnapshot(r.Context(), vehicle.ID, day.Format("2006-01-02")); err != nil {
		writeRepoError(w, r, err, "Failed to delete the battery reading")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Residual projects the resale value; the assumptions come from the query string.
func (h *ResidualHandler) Residual(w http.ResponseWriter, r *http.Request) {
	vehicle := requireVehicleAccess(w, r, h.repo, chi.URLParam(r, "vehicleId"), models.RoleViewer)
	if vehicle == nil {
		return
	}
	q := r.URL.Query()
	opts := services.ResidualOptions{KmShare: 0.5, HealthWeight: 1}
	for key, dst := range map[string]*float64{"expected_km": &opts.ExpectedKm, "km_share": &opts.KmShare, "health_weight": &opts.HealthWeight} {
		raw := strings.TrimSpace(q.Get(key))
		if raw == "" {
			continue
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, apierror.New("residual.invalid_options", "Invalid projection assumptions"))
			return
		}
		*dst = v
	}
	result, err := h.residual.Residual(r.Context(), vehicle, opts)
	if err != nil {
		writeRepoError(w, r, err, "Failed to project the residual value")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
