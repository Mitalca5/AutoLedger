package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

// ComparisonHandler serves the informational EV vs ICE cost comparison.
type ComparisonHandler struct {
	repo    *database.Repository
	service *services.ComparisonService
}

func NewComparisonHandler(repo *database.Repository, service *services.ComparisonService) *ComparisonHandler {
	return &ComparisonHandler{repo: repo, service: service}
}

// SaveComparisonRequest is the payload shared by Create and Update.
type SaveComparisonRequest struct {
	VehicleID *string                `json:"vehicle_id"`
	Name      string                 `json:"name"`
	Mode      string                 `json:"mode"`
	AnnualKm  float64                `json:"annual_km"`
	Years     int                    `json:"years"`
	ICE       models.ICEInputs       `json:"ice"`
	EV        *models.EVInputs       `json:"ev"`
	Options   models.ScenarioOptions `json:"options"`
}

func validateRange(v, min, max float64, failure *apierror.Error) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < min || v > max {
		return failure
	}
	return nil
}

func validateAmounts(amounts ...money.Cents) error {
	for _, a := range amounts {
		if err := validateAmount(a, true); err != nil {
			return err
		}
	}
	return nil
}

// validateComparisonRequest checks the bounds of every input and normalizes the scenario name and links.
func validateComparisonRequest(req *SaveComparisonRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 100 {
		return apierror.New("comparison.name_invalid", "Invalid name (1 to 100 characters)")
	}
	if req.Mode != models.ComparisonModeRetrospective && req.Mode != models.ComparisonModeProjection {
		return apierror.New("comparison.mode_invalid", "Invalid mode (RETROSPECTIVE or PROJECTION)")
	}
	if err := validateRange(req.AnnualKm, 1, 200_000, apierror.Newf("comparison.annual_km", "Invalid yearly mileage (%.0f to %.0f km)", apierror.Km(1), apierror.Km(200_000))); err != nil {
		return err
	}
	if req.Years < 1 || req.Years > 15 {
		return apierror.New("comparison.years", "Invalid duration (1 to 15 years)")
	}

	ice := req.ICE
	if !models.FuelTypes[ice.FuelType] {
		return apierror.New("fuel.type_invalid", "Invalid fuel")
	}
	if err := validateRange(ice.LPer100Km, 0.1, 50, apierror.Newf("comparison.ice_consumption", "Invalid combustion consumption (%.1f to %.0f L/100 km)", apierror.PerKm(0.1), apierror.PerKm(50))); err != nil {
		return err
	}
	if err := validateRange(ice.FuelPrice, 0, 10, apierror.New("comparison.fuel_price", "Invalid fuel price (0 to 10 per litre)")); err != nil {
		return err
	}
	if err := validateAmounts(ice.PurchasePrice, ice.ResaleValue, ice.MaintenanceYearly, ice.InsuranceYearly, ice.TaxYearly); err != nil {
		return err
	}

	for _, pct := range []float64{req.Options.FuelInflationPct, req.Options.ElectricityInflationPct, req.Options.CostInflationPct} {
		if err := validateRange(pct, -10, 30, apierror.New("comparison.inflation", "Invalid inflation (−10 to 30%)")); err != nil {
			return err
		}
	}
	if err := validateAmounts(req.Options.EVIncentives); err != nil {
		return err
	}

	switch req.Mode {
	case models.ComparisonModeRetrospective:
		if req.VehicleID == nil || *req.VehicleID == "" {
			return apierror.New("comparison.vehicle_required", "A vehicle is required in retrospective mode")
		}
		req.EV = nil
	case models.ComparisonModeProjection:
		req.VehicleID = nil
		if req.EV == nil {
			return apierror.New("comparison.ev_required", "The electric vehicle data is required in projection mode")
		}
		if err := validateRange(req.EV.KwhPer100Km, 0.1, 100, apierror.Newf("comparison.ev_consumption", "Invalid electric consumption (%.1f to %.0f kWh/100 km)", apierror.PerKm(0.1), apierror.PerKm(100))); err != nil {
			return err
		}
		if err := validateRange(req.EV.EurPerKwh, 0, 5, apierror.New("comparison.electricity_price", "Invalid electricity price (0 to 5 per kWh)")); err != nil {
			return err
		}
		if err := validateAmounts(req.EV.PurchasePrice, req.EV.ResaleValue, req.EV.MaintenanceYearly, req.EV.InsuranceYearly, req.EV.TaxYearly); err != nil {
			return err
		}
	}
	return nil
}

func (r *SaveComparisonRequest) toScenario(userID string) *models.ComparisonScenario {
	return &models.ComparisonScenario{
		UserID:    userID,
		VehicleID: r.VehicleID,
		Name:      r.Name,
		Mode:      r.Mode,
		AnnualKm:  r.AnnualKm,
		Years:     r.Years,
		ICE:       r.ICE,
		EV:        r.EV,
		Options:   r.Options,
	}
}

// decodeAndCheck reads the request, validates it and checks the access to the reference vehicle.
func (h *ComparisonHandler) decodeAndCheck(w http.ResponseWriter, r *http.Request) (*SaveComparisonRequest, bool) {
	var req SaveComparisonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return nil, false
	}
	if err := validateComparisonRequest(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return nil, false
	}
	if req.VehicleID != nil {
		v := requireVehicleAccess(w, r, h.repo, *req.VehicleID, models.RoleViewer)
		if v == nil {
			return nil, false
		}
		if !v.CanCharge() {
			writeAPIError(w, http.StatusBadRequest, apierror.New("comparison.needs_ev", "The tracked comparison requires an electric or plug-in hybrid vehicle; use projection mode for a combustion vehicle"))
			return nil, false
		}
	}
	return &req, true
}

func (h *ComparisonHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListComparisonScenarios(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		writeRepoError(w, r, err, "Failed to list comparisons")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *ComparisonHandler) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeAndCheck(w, r)
	if !ok {
		return
	}
	sc := req.toScenario(middleware.GetUserID(r.Context()))
	if err := h.repo.CreateComparisonScenario(r.Context(), sc); err != nil {
		writeRepoError(w, r, err, "Failed to create comparison")
		return
	}
	writeJSON(w, http.StatusCreated, sc)
}

func (h *ComparisonHandler) Update(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeAndCheck(w, r)
	if !ok {
		return
	}
	sc := req.toScenario(middleware.GetUserID(r.Context()))
	sc.ID = chi.URLParam(r, "scenarioId")
	if err := h.repo.UpdateComparisonScenario(r.Context(), sc); err != nil {
		writeRepoError(w, r, err, "Failed to update comparison")
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (h *ComparisonHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.DeleteComparisonScenario(r.Context(), middleware.GetUserID(r.Context()), chi.URLParam(r, "scenarioId")); err != nil {
		writeRepoError(w, r, err, "Failed to delete comparison")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// Result evaluates a saved scenario. Nothing is written.
func (h *ComparisonHandler) Result(w http.ResponseWriter, r *http.Request) {
	sc, err := h.repo.GetComparisonScenario(r.Context(), middleware.GetUserID(r.Context()), chi.URLParam(r, "scenarioId"))
	if err != nil {
		writeRepoError(w, r, err, "Failed to load comparison")
		return
	}
	if sc.VehicleID != nil && requireVehicleAccess(w, r, h.repo, *sc.VehicleID, models.RoleViewer) == nil {
		return
	}
	res, err := h.service.Compare(r.Context(), sc)
	if err != nil {
		writeRepoError(w, r, err, "Failed to compute comparison")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// Defaults returns the form prefill, using the vehicle's real data when vehicle_id is given.
func (h *ComparisonHandler) Defaults(w http.ResponseWriter, r *http.Request) {
	vehicleID := r.URL.Query().Get("vehicle_id")
	if vehicleID != "" && requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer) == nil {
		return
	}
	d, err := h.service.Defaults(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to compute defaults")
		return
	}
	writeJSON(w, http.StatusOK, d)
}
