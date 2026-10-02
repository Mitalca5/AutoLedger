package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

type TireHandler struct {
	repo            *database.Repository
	tireWearService *services.TireWearService
}

func NewTireHandler(repo *database.Repository, tireWearService *services.TireWearService) *TireHandler {
	return &TireHandler{
		repo:            repo,
		tireWearService: tireWearService,
	}
}

func (h *TireHandler) List(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer)
	if v == nil {
		return
	}

	tires, err := h.repo.ListTires(r.Context(), vehicleID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apierror.New("internal", "Failed to list tires"))
		return
	}

	statsList := []services.TireWearStats{}
	for i := range tires {
		stats, err := h.tireWearService.CalculateTireWear(r.Context(), &tires[i], v.CurrentOdometer)
		if err == nil && stats != nil {
			statsList = append(statsList, *stats)
		}
	}

	writeJSON(w, http.StatusOK, statsList)
}

type CreateTireRequest struct {
	Brand                 string              `json:"brand"`
	Model                 string              `json:"model"`
	Dimension             string              `json:"dimension"`
	Season                models.TireSeason   `json:"season"`
	PurchaseDate          string              `json:"purchase_date"` // YYYY-MM-DD
	PurchasePrice         money.Cents         `json:"purchase_price"`
	CurrentPosition       models.TirePosition `json:"current_position"`
	InitialDepthMm        float64             `json:"initial_depth_mm"`
	MinLegalDepthMm       float64             `json:"min_legal_depth_mm"`
	DotCode               *string             `json:"dot_code"`
	MountedOdometer       *float64            `json:"mounted_odometer"`
	AccumulatedDistanceKm float64             `json:"accumulated_distance_km"`
	EstimatedLifespanKm   int                 `json:"estimated_lifespan_km"`
}

func (h *TireHandler) Create(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req CreateTireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if req.Brand == "" || req.Model == "" || req.Dimension == "" {
		writeAPIError(w, http.StatusBadRequest, apierror.New("tire.fields_required", "Brand, model and size are required"))
		return
	}

	purchaseDate, err := parseDate(req.PurchaseDate)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := validateAmount(req.PurchasePrice, true); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	pos := req.CurrentPosition
	if pos == "" {
		pos = models.TirePosStorage
	}

	season := req.Season
	if season == "" {
		season = models.TireSeasonSummer
	}

	initialDepth := req.InitialDepthMm
	if initialDepth <= 0 {
		initialDepth = 8.0
	}
	minDepth := req.MinLegalDepthMm
	if minDepth <= 0 {
		minDepth = 1.6
	}

	lifespan := req.EstimatedLifespanKm
	if lifespan <= 0 {
		lifespan = 40000
	}

	t := &models.Tire{
		VehicleID:             &vehicleID,
		Brand:                 req.Brand,
		Model:                 req.Model,
		Dimension:             req.Dimension,
		Season:                season,
		PurchaseDate:          purchaseDate,
		PurchasePrice:         req.PurchasePrice,
		CurrentPosition:       pos,
		InitialDepthMm:        initialDepth,
		MinLegalDepthMm:       minDepth,
		DotCode:               req.DotCode,
		MountedOdometer:       req.MountedOdometer,
		AccumulatedDistanceKm: req.AccumulatedDistanceKm,
		EstimatedLifespanKm:   lifespan,
	}

	if err := h.repo.CreateTire(r.Context(), t); err != nil {
		writeRepoError(w, r, err, "Failed to create tire")
		return
	}

	writeJSON(w, http.StatusCreated, t)
}

// UpdateTireRequest updates descriptive fields; omitted fields are left unchanged.
// Positions are changed through mount sessions or rotations only.
type UpdateTireRequest struct {
	Brand               *string            `json:"brand"`
	Model               *string            `json:"model"`
	Dimension           *string            `json:"dimension"`
	Season              *models.TireSeason `json:"season"`
	PurchaseDate        *string            `json:"purchase_date"`
	PurchasePrice       *money.Cents       `json:"purchase_price"`
	InitialDepthMm      *float64           `json:"initial_depth_mm"`
	MinLegalDepthMm     *float64           `json:"min_legal_depth_mm"`
	DotCode             *string            `json:"dot_code"`
	InitialDistanceKm   *float64           `json:"initial_distance_km"`
	EstimatedLifespanKm *int               `json:"estimated_lifespan_km"`
}

func (h *TireHandler) Update(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	t, err := h.repo.GetTireByID(r.Context(), tireID, vehicleID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("tire.not_found", "Tire not found"))
		return
	}

	var req UpdateTireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if req.Brand != nil && *req.Brand != "" {
		t.Brand = *req.Brand
	}
	if req.Model != nil && *req.Model != "" {
		t.Model = *req.Model
	}
	if req.Dimension != nil && *req.Dimension != "" {
		t.Dimension = *req.Dimension
	}
	if req.Season != nil && *req.Season != "" {
		t.Season = *req.Season
	}
	if req.PurchaseDate != nil {
		pd, err := parseDate(*req.PurchaseDate)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		t.PurchaseDate = pd
	}
	if req.PurchasePrice != nil {
		if err := validateAmount(*req.PurchasePrice, true); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		t.PurchasePrice = *req.PurchasePrice
	}
	if req.InitialDepthMm != nil && *req.InitialDepthMm > 0 {
		t.InitialDepthMm = *req.InitialDepthMm
	}
	if req.MinLegalDepthMm != nil && *req.MinLegalDepthMm > 0 {
		t.MinLegalDepthMm = *req.MinLegalDepthMm
	}
	if req.DotCode != nil {
		t.DotCode = req.DotCode
	}
	if req.InitialDistanceKm != nil {
		if *req.InitialDistanceKm < 0 {
			writeAPIError(w, http.StatusBadRequest, apierror.New("odometer.initial_negative", "The initial mileage cannot be negative"))
			return
		}
		t.InitialDistanceKm = *req.InitialDistanceKm
	}
	if req.EstimatedLifespanKm != nil && *req.EstimatedLifespanKm > 0 {
		t.EstimatedLifespanKm = *req.EstimatedLifespanKm
	}

	if err := h.repo.UpdateTire(r.Context(), t); err != nil {
		writeRepoError(w, r, err, "Failed to update tire")
		return
	}

	writeJSON(w, http.StatusOK, t)
}

func isMountedPosition(pos models.TirePosition) bool {
	switch pos {
	case models.TirePosFL, models.TirePosFR, models.TirePosRL, models.TirePosRR:
		return true
	}
	return false
}

// Delete permanently removes a tire entered by mistake, with its history.
func (h *TireHandler) Delete(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	if err := h.repo.DeleteTire(r.Context(), vehicleID, chi.URLParam(r, "tireId")); err != nil {
		writeRepoError(w, r, err, "Failed to delete tire")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

type DisposeTireRequest struct {
	Date     string   `json:"date"`
	Odometer *float64 `json:"odometer"`
}

// parseDisposal reads the date (today when empty) and the optional odometer of a disposal request. It writes the
// error response and returns false when either is invalid.
func parseDisposal(w http.ResponseWriter, date string, odometer *float64) (time.Time, bool) {
	at := time.Now().UTC()
	if date != "" {
		parsed, err := parseDate(date)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return time.Time{}, false
		}
		at = parsed
	}
	if odometer != nil {
		if err := validateQuantity(*odometer, 2_000_000); err != nil {
			writeAPIError(w, http.StatusBadRequest, apierror.New("odometer.invalid", "Invalid odometer"))
			return time.Time{}, false
		}
	}
	return at, true
}

// Dispose retires a worn out or damaged tire while keeping its history and cost.
func (h *TireHandler) Dispose(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	var req DisposeTireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}
	at, ok := parseDisposal(w, req.Date, req.Odometer)
	if !ok {
		return
	}
	if err := h.repo.DisposeTire(r.Context(), vehicleID, chi.URLParam(r, "tireId"), at, req.Odometer); err != nil {
		writeRepoError(w, r, err, "Failed to dispose tire")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
