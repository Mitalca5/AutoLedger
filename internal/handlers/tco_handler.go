package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/services"
)

type TCOHandler struct {
	repo       *database.Repository
	tcoService *services.TCOService
}

func NewTCOHandler(repo *database.Repository, tcoService *services.TCOService) *TCOHandler {
	return &TCOHandler{
		repo:       repo,
		tcoService: tcoService,
	}
}

func (h *TCOHandler) GetTCO(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}

	summary, err := h.tcoService.ComputeVehicleTCO(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to compute TCO")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// GetMonthlyContributions lists the recorded entries behind one category of a monthly total.
func (h *TCOHandler) GetMonthlyContributions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	month := chi.URLParam(r, "month")
	category := r.URL.Query().Get("category")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}

	out, err := h.tcoService.MonthlyCostContributions(r.Context(), vehicleID, month, category)
	switch {
	case errors.Is(err, services.ErrInvalidContributionMonth):
		writeAPIError(w, http.StatusBadRequest, apierror.New("tco.contributions_invalid_month", "Month must be formatted as YYYY-MM"))
		return
	case errors.Is(err, services.ErrUnsupportedContributionCategory):
		writeAPIError(w, http.StatusBadRequest, apierror.New("tco.contributions_unsupported_category", "Unsupported cost category"))
		return
	case err != nil:
		writeRepoError(w, r, err, "Failed to list monthly contributions")
		return
	}

	writeJSON(w, http.StatusOK, out)
}

// GetDataQuality lists odometer continuity issues of the vehicle drives.
func (h *TCOHandler) GetDataQuality(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}

	issues, err := h.repo.ListDataQualityIssues(r.Context(), vehicleID, 100)
	if err != nil {
		writeRepoError(w, r, err, "Failed to check data quality")
		return
	}
	gaps, gapKm, anomalies, err := h.repo.DataQualitySummary(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to check data quality")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"issues":             issues,
		"odometer_gaps":      gaps,
		"odometer_gap_km":    gapKm,
		"odometer_anomalies": anomalies,
	})
}
