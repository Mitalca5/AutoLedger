package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/services"
)

type EnergyHandler struct {
	repo    *database.Repository
	service *services.EnergyStatsService
}

func NewEnergyHandler(repo *database.Repository, service *services.EnergyStatsService) *EnergyHandler {
	return &EnergyHandler{repo: repo, service: service}
}

// GetStats returns the consumption, charge cost and charging habits of an electric vehicle.
// A combustion vehicle has none of these figures: its fuel statistics come with the TCO summary.
func (h *EnergyHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	vehicle, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}
	if !vehicle.CanCharge() {
		writeJSON(w, http.StatusOK, &services.EnergyStats{
			Months:        []services.EnergyMonth{},
			ChargeClasses: []services.ChargeClassStat{},
		})
		return
	}

	stats, err := h.service.Compute(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to compute energy statistics")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
