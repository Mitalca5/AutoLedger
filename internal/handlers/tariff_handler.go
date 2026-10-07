package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

type TariffHandler struct {
	repo          *database.Repository
	tariffService *services.TariffService
}

func NewTariffHandler(repo *database.Repository, tariffService *services.TariffService) *TariffHandler {
	return &TariffHandler{
		repo:          repo,
		tariffService: tariffService,
	}
}

func (h *TariffHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	plans, err := h.repo.ListTariffPlans(r.Context(), userID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list tariff plans")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans})
}

type SaveTariffPlanRequest struct {
	Name             string              `json:"name"`
	PlanType         string              `json:"plan_type"`
	Currency         string              `json:"currency"`
	FlatRateCents    *money.Cents        `json:"flat_rate_cents"`
	PeakRateCents    *money.Cents        `json:"peak_rate_cents"`
	OffpeakRateCents *money.Cents        `json:"offpeak_rate_cents"`
	TimeWindows      []models.TimeWindow `json:"time_windows"`
	IsDefault        bool                `json:"is_default"`

	Bands               []models.TariffBand `json:"bands"`
	Rules               []models.TariffRule `json:"rules"`
	DefaultBand         string              `json:"default_band"`
	StandingChargeCents *money.Cents        `json:"standing_charge_cents"`
	ValidFrom           *string             `json:"valid_from"`
	ValidTo             *string             `json:"valid_to"`
}

var hhmmPattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$|^24:00$`)

func validDay(s *string) bool {
	if s == nil || *s == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", *s)
	return err == nil
}

// validateBands checks a band grid: named bands, rules that point at an existing band with valid days and times,
// and a validity range in the right order.
func validateBands(req *SaveTariffPlanRequest) *apierror.Error {
	if len(req.Bands) == 0 {
		return apierror.New("tariff.bands_required", "At least one band is required")
	}
	names := map[string]bool{}
	for i := range req.Bands {
		req.Bands[i].Name = strings.TrimSpace(req.Bands[i].Name)
		n := req.Bands[i].Name
		if n == "" || names[n] || req.Bands[i].RateCents < 0 {
			return apierror.New("tariff.band_invalid", "Invalid band")
		}
		names[n] = true
	}
	if req.DefaultBand != "" && !names[req.DefaultBand] {
		return apierror.New("tariff.band_invalid", "Invalid band")
	}
	for _, r := range req.Rules {
		if !names[r.Band] || !hhmmPattern.MatchString(r.Start) || !hhmmPattern.MatchString(r.End) {
			return apierror.New("tariff.rule_invalid", "Invalid rule")
		}
		for _, d := range r.Days {
			if d < 0 || d > 6 {
				return apierror.New("tariff.rule_invalid", "Invalid rule")
			}
		}
	}
	if !validDay(req.ValidFrom) || !validDay(req.ValidTo) {
		return apierror.New("tariff.validity_invalid", "Invalid validity range")
	}
	if req.ValidFrom != nil && req.ValidTo != nil && *req.ValidFrom != "" && *req.ValidTo != "" && *req.ValidTo < *req.ValidFrom {
		return apierror.New("tariff.validity_invalid", "Invalid validity range")
	}
	return nil
}

func blankToNil(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

func (h *TariffHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req SaveTariffPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeAPIError(w, http.StatusBadRequest, apierror.New("tariff.name_required", "Tariff plan name is required"))
		return
	}

	planType := req.PlanType
	if planType != models.TariffTypeFlat && planType != models.TariffTypeTimeOfUse && planType != models.TariffTypeBands {
		planType = models.TariffTypeTimeOfUse
	}
	if planType == models.TariffTypeBands {
		if e := validateBands(&req); e != nil {
			writeAPIError(w, http.StatusBadRequest, e)
			return
		}
	}
	if !validDay(req.ValidFrom) || !validDay(req.ValidTo) {
		writeAPIError(w, http.StatusBadRequest, apierror.New("tariff.validity_invalid", "Invalid validity range"))
		return
	}

	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "EUR"
	}

	plan := &models.TariffPlan{
		UserID:           userID,
		Name:             name,
		PlanType:         planType,
		Currency:         currency,
		FlatRateCents:    req.FlatRateCents,
		PeakRateCents:    req.PeakRateCents,
		OffpeakRateCents: req.OffpeakRateCents,
		TimeWindows:      req.TimeWindows,
		IsDefault:        req.IsDefault,

		Bands:               req.Bands,
		Rules:               req.Rules,
		DefaultBand:         req.DefaultBand,
		StandingChargeCents: req.StandingChargeCents,
		ValidFrom:           blankToNil(req.ValidFrom),
		ValidTo:             blankToNil(req.ValidTo),
	}

	if err := h.repo.CreateTariffPlan(r.Context(), plan); err != nil {
		writeRepoError(w, r, err, "Failed to create tariff plan")
		return
	}

	writeJSON(w, http.StatusCreated, plan)
}

func (h *TariffHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	planID := chi.URLParam(r, "id")

	existing, err := h.repo.GetTariffPlanByID(r.Context(), planID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("tariff.not_found", "Tariff plan not found"))
		return
	}

	var req SaveTariffPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	name := strings.TrimSpace(req.Name)
	if name != "" {
		existing.Name = name
	}
	if req.PlanType != "" {
		existing.PlanType = req.PlanType
	}
	if req.Currency != "" {
		existing.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	}
	existing.FlatRateCents = req.FlatRateCents
	existing.PeakRateCents = req.PeakRateCents
	existing.OffpeakRateCents = req.OffpeakRateCents
	if req.TimeWindows != nil {
		existing.TimeWindows = req.TimeWindows
	}
	existing.IsDefault = req.IsDefault
	if existing.PlanType == models.TariffTypeBands {
		if e := validateBands(&req); e != nil {
			writeAPIError(w, http.StatusBadRequest, e)
			return
		}
		existing.Bands, existing.Rules, existing.DefaultBand = req.Bands, req.Rules, req.DefaultBand
	}
	if !validDay(req.ValidFrom) || !validDay(req.ValidTo) {
		writeAPIError(w, http.StatusBadRequest, apierror.New("tariff.validity_invalid", "Invalid validity range"))
		return
	}
	existing.StandingChargeCents = req.StandingChargeCents
	existing.ValidFrom, existing.ValidTo = blankToNil(req.ValidFrom), blankToNil(req.ValidTo)

	if err := h.repo.UpdateTariffPlan(r.Context(), existing); err != nil {
		writeRepoError(w, r, err, "Failed to update tariff plan")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

func (h *TariffHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	planID := chi.URLParam(r, "id")

	if err := h.repo.DeleteTariffPlan(r.Context(), planID, userID); err != nil {
		writeRepoError(w, r, err, "Failed to delete tariff plan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// CalculateSessionCostRequest helper for quick cost test from frontend
type CalculateSessionCostRequest struct {
	PlanID *string `json:"plan_id"`
	// VehicleID prices the session with the tariff of that vehicle on the session day (versions included); it
	// takes precedence over PlanID.
	VehicleID string    `json:"vehicle_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Kwh       float64   `json:"kwh"`
}

func (h *TariffHandler) Calculate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req CalculateSessionCostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	var plan *models.TariffPlan
	var err error
	if req.VehicleID != "" {
		if requireVehicleAccess(w, r, h.repo, req.VehicleID, models.RoleViewer) == nil {
			return
		}
		plan, _ = h.repo.GetVehicleTariffPlanAt(r.Context(), req.VehicleID, h.tariffService.DayOf(req.StartTime))
	} else if req.PlanID != nil && *req.PlanID != "" {
		plan, err = h.repo.GetTariffPlanByID(r.Context(), *req.PlanID, userID)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, apierror.New("tariff.not_found", "Tariff plan not found"))
			return
		}
	} else {
		plan, _ = h.repo.GetDefaultTariffPlan(r.Context(), userID)
	}

	cost, err := h.tariffService.CalculateSessionCost(plan, req.StartTime, req.EndTime, req.Kwh)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apierror.New("internal", err.Error()))
		return
	}

	// plan is null when no tariff prices the session: the client keeps the cost the user types
	var planName *string
	if plan != nil && plan.AppliesOn(h.tariffService.DayOf(req.StartTime)) {
		planName = &plan.Name
	}
	writeJSON(w, http.StatusOK, map[string]any{"cost": cost, "plan": planName})
}

func (h *TariffHandler) CalculatePublic(w http.ResponseWriter, r *http.Request) {
	var req models.PublicChargingCalculationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	breakdown := h.tariffService.CalculatePublicCharging(req)
	writeJSON(w, http.StatusOK, breakdown)
}

// Public Charging Presets

func (h *TariffHandler) ListPublicPresets(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	presets, err := h.repo.ListPublicChargingPresets(r.Context(), userID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list public charging presets")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"presets": presets})
}

func (h *TariffHandler) CreatePublicPreset(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var preset models.PublicChargingPreset
	if err := json.NewDecoder(r.Body).Decode(&preset); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	preset.Name = strings.TrimSpace(preset.Name)
	if preset.Name == "" {
		writeAPIError(w, http.StatusBadRequest, apierror.New("preset.name_required", "Preset name is required"))
		return
	}
	preset.UserID = userID
	if preset.Currency == "" {
		preset.Currency = "EUR"
	}

	if err := h.repo.CreatePublicChargingPreset(r.Context(), &preset); err != nil {
		writeRepoError(w, r, err, "Failed to save public charging preset")
		return
	}

	writeJSON(w, http.StatusCreated, preset)
}

func (h *TariffHandler) DeletePublicPreset(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	presetID := chi.URLParam(r, "id")

	if err := h.repo.DeletePublicChargingPreset(r.Context(), presetID, userID); err != nil {
		writeRepoError(w, r, err, "Failed to delete preset")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
