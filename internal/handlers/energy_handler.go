package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

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

// GetPlaceStats returns the charging of the vehicle by month and by place (months=0 for the whole history).
func (h *EnergyHandler) GetPlaceStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	vehicle, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}

	period := services.PlacePeriod{Months: 12}
	q := r.URL.Query()
	if from, to := q.Get("from"), q.Get("to"); from != "" || to != "" {
		period = services.PlacePeriod{From: from, To: to}
	} else if raw := q.Get("months"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > 240 {
			writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_months", "months must be between 0 and 240"))
			return
		}
		period = services.PlacePeriod{Months: n}
	}

	if !vehicle.CanCharge() {
		writeJSON(w, http.StatusOK, services.EmptyPlaceStats(period))
		return
	}
	stats, err := h.service.PlaceStats(r.Context(), vehicleID, vehicle.UserID, period, time.Now())
	if errors.Is(err, services.ErrInvalidPlacePeriod) {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_date_range", "from and to must be two days (YYYY-MM-DD), from not after to"))
		return
	}
	if err != nil {
		writeRepoError(w, r, err, "Failed to compute charging statistics")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
