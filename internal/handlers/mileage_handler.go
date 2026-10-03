package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type MileageHandler struct {
	repo    *database.Repository
	mileage *services.MileageService
}

func NewMileageHandler(repo *database.Repository, mileage *services.MileageService) *MileageHandler {
	return &MileageHandler{repo: repo, mileage: mileage}
}

// Report totals the vehicle's trips per tag over a period, with the allowance of the chosen scale.
func (h *MileageHandler) Report(w http.ResponseWriter, r *http.Request) {
	vehicle := requireVehicleAccess(w, r, h.repo, chi.URLParam(r, "vehicleId"), models.RoleViewer)
	if vehicle == nil {
		return
	}
	q := r.URL.Query()
	from, err := parseExportDate(q.Get("from"), false)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	to, err := parseExportDate(q.Get("to"), true)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	report, err := h.mileage.Report(r.Context(), vehicle, middleware.GetUserID(r.Context()), services.MileageOptions{
		From: from, To: to, Tag: strings.TrimSpace(q.Get("tag")), RateLabel: strings.TrimSpace(q.Get("rates")),
	})
	if err != nil {
		writeRepoError(w, r, err, "Failed to build the mileage report")
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *MileageHandler) ListRates(w http.ResponseWriter, r *http.Request) {
	rates, err := h.repo.ListMileageRates(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		writeRepoError(w, r, err, "Failed to list mileage rates")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rates": rates})
}

type saveMileageRateRequest struct {
	Label     string  `json:"label"`
	Year      int     `json:"year"`
	FromKm    int     `json:"from_km"`
	ToKm      *int    `json:"to_km"`
	RatePerKm float64 `json:"rate_per_km"`
}

func (h *MileageHandler) CreateRate(w http.ResponseWriter, r *http.Request) {
	var req saveMileageRateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}
	req.Label = strings.TrimSpace(req.Label)
	if req.Label == "" || len(req.Label) > 100 {
		writeAPIError(w, http.StatusBadRequest, apierror.New("mileage.label_required", "A name for the scale is required"))
		return
	}
	if req.Year < 1970 || req.Year > 2200 || req.FromKm < 0 || req.RatePerKm < 0 || req.RatePerKm > 1000 ||
		(req.ToKm != nil && *req.ToKm <= req.FromKm) {
		writeAPIError(w, http.StatusBadRequest, apierror.New("mileage.rate_invalid", "Invalid year, distance slice or rate"))
		return
	}
	m := &models.MileageRate{
		UserID: middleware.GetUserID(r.Context()), Label: req.Label, Year: req.Year,
		FromKm: req.FromKm, ToKm: req.ToKm, RatePerKm: req.RatePerKm,
	}
	if err := h.repo.CreateMileageRate(r.Context(), m); err != nil {
		writeRepoError(w, r, err, "Failed to create the mileage rate")
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (h *MileageHandler) DeleteRate(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.DeleteMileageRate(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r.Context())); err != nil {
		writeRepoError(w, r, err, "Failed to delete the mileage rate")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
