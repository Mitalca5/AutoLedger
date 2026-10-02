package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
	"github.com/teslacost/teslacost/internal/services/ingest"
)

type DriveCostBreakdown struct {
	ElectricityCost       money.Cents `json:"electricity_cost"`
	ElectricityKwh        float64     `json:"electricity_kwh"`
	ElectricityRate       float64     `json:"electricity_rate"`
	EnergySource          string      `json:"energy_source"`           // MEASURED | CONSUMPTION | DEFAULT
	ElectricityRateSource string      `json:"electricity_rate_source"` // HISTORY | DEFAULT
	TiresCost             money.Cents `json:"tires_cost"`
	TiresRate             float64     `json:"tires_rate"`
	TiresRateSource       string      `json:"tires_rate_source"`
	MaintenanceCost       money.Cents `json:"maintenance_cost"`
	MaintenanceRate       float64     `json:"maintenance_rate"`
	MaintenanceRateSource string      `json:"maintenance_rate_source"`
	InsuranceCost         money.Cents `json:"insurance_cost"`
	InsuranceRate         float64     `json:"insurance_rate"`
	InsuranceSource       string      `json:"insurance_source"`
	TollsCost             money.Cents `json:"tolls_cost"` // Direct expenses + share of trip group expenses
	TotalCost             money.Cents `json:"total_cost"`
	CostPerKm             float64     `json:"cost_per_km"`
	// HasEstimates is true when at least one component relies on a default assumption.
	HasEstimates bool `json:"has_estimates"`
}

type EnrichedDrive struct {
	models.Drive
	Costs DriveCostBreakdown `json:"costs"`
	// NeedsTollQualification is true for a highway-like drive (speed heuristic or a toll found by the GPS
	// detection) with no toll attached and no "no toll" review: the drives of the "to qualify" queue.
	NeedsTollQualification bool `json:"needs_toll_qualification"`
}

type DriveHandler struct {
	repo                 *database.Repository
	carpoolService       *services.CarpoolService
	tollDetectionService *services.TollDetectionService
}

func NewDriveHandler(repo *database.Repository, carpoolService *services.CarpoolService, tollDetectionService *services.TollDetectionService) *DriveHandler {
	return &DriveHandler{
		repo:                 repo,
		carpoolService:       carpoolService,
		tollDetectionService: tollDetectionService,
	}
}

func (h *DriveHandler) List(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	// Ensure vehicle belongs to user with at least VIEWER role
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer); v == nil {
		return
	}

	filter := parseDriveFilter(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit

	drives, total, err := h.repo.ListDrives(r.Context(), vehicleID, filter, limit, offset)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list drives")
		return
	}
	if drives == nil {
		drives = []models.Drive{}
	}

	// Calculate unit rates for real cost breakdown
	rates, err := h.carpoolService.GetVehicleUnitRates(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to compute cost rates")
		return
	}

	// Fetch toll expenses attached to these drives
	driveIDs := make([]string, len(drives))
	for i, d := range drives {
		driveIDs[i] = d.ID
	}
	tollsMap, err := h.repo.GetTollExpensesForDrives(r.Context(), vehicleID, driveIDs)
	if err != nil {
		writeRepoError(w, r, err, "Failed to load drive expenses")
		return
	}
	needsQualification, err := h.repo.DrivesNeedingTollQualification(r.Context(), vehicleID, driveIDs)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list drives")
		return
	}
	unqualifiedCount, err := h.repo.CountUnqualifiedDrives(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list drives")
		return
	}

	// Insurance is a fixed daily/monthly cost, not a per-km one: allocate each day's flat
	// insurance cost across that day's drives in proportion to distance.
	insuranceCosts, err := h.carpoolService.AllocateInsuranceCosts(r.Context(), vehicleID, drives, rates)
	if err != nil {
		writeRepoError(w, r, err, "Failed to compute cost rates")
		return
	}

	enriched := make([]EnrichedDrive, len(drives))
	for i, d := range drives {
		kwh, energySource := services.DriveEnergyKwh(d.DistanceKm, d.EnergyConsumedKwh, d.ConsumptionKwh100km)
		hasEstimates := energySource == services.EnergySourceDefault ||
			rates.ElectricitySource == services.RateSourceDefault ||
			rates.TiresSource == services.RateSourceDefault ||
			rates.MaintenanceSource == services.RateSourceDefault

		components := services.ComputeCostComponents(rates, d.DistanceKm, kwh)
		elecCost := components.ElectricityCost
		tiresCost := components.TiresCost
		maintCost := components.MaintenanceCost
		insCost := insuranceCosts[d.ID]
		tollsCost := tollsMap[d.ID]
		totalCost := elecCost + tiresCost + maintCost + insCost + tollsCost

		costPerKm := 0.0
		// The insurance rate actually applied to this drive, which may differ from the vehicle's
		// flat rates.InsurancePerKm since insurance is allocated per day, not per km.
		insRate := rates.InsurancePerKm
		if d.DistanceKm > 0 {
			costPerKm = math.Round((totalCost.Float()/d.DistanceKm)*1000) / 1000
			insRate = math.Round((insCost.Float()/d.DistanceKm)*1000) / 1000
		}

		enriched[i] = EnrichedDrive{
			Drive:                  d,
			NeedsTollQualification: needsQualification[d.ID],
			Costs: DriveCostBreakdown{
				ElectricityCost:       elecCost,
				ElectricityKwh:        math.Round(kwh*10) / 10,
				ElectricityRate:       rates.ElectricityPerKwh,
				EnergySource:          energySource,
				ElectricityRateSource: rates.ElectricitySource,
				TiresCost:             tiresCost,
				TiresRate:             rates.TiresPerKm,
				TiresRateSource:       rates.TiresSource,
				MaintenanceCost:       maintCost,
				MaintenanceRate:       rates.MaintenancePerKm,
				MaintenanceRateSource: rates.MaintenanceSource,
				InsuranceCost:         insCost,
				InsuranceRate:         insRate,
				InsuranceSource:       rates.InsuranceSource,
				TollsCost:             tollsCost,
				TotalCost:             totalCost,
				CostPerKm:             costPerKm,
				HasEstimates:          hasEstimates,
			},
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"drives":            enriched,
		"total":             total,
		"page":              page,
		"limit":             limit,
		"unqualified_count": unqualifiedCount,
	})
}

func (h *DriveHandler) GetDriveExpenses(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	driveID := chi.URLParam(r, "driveId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer); v == nil {
		return
	}

	expenses, err := h.repo.GetDriveExpensesByDriveID(r.Context(), vehicleID, driveID, requestLanguage(r, h.repo))
	if err != nil {
		writeRepoError(w, r, err, "Failed to load drive expenses")
		return
	}
	if expenses == nil {
		expenses = []models.DriveExpense{}
	}

	writeJSON(w, http.StatusOK, expenses)
}

type UpdateTagsRequest struct {
	Tags []string `json:"tags"`
}

func (h *DriveHandler) UpdateTags(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	driveID := chi.URLParam(r, "driveId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req UpdateTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if req.Tags == nil {
		req.Tags = []string{}
	}

	if err := h.repo.UpdateDriveTags(r.Context(), driveID, vehicleID, req.Tags); err != nil {
		writeRepoError(w, r, err, "Failed to update tags")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"tags":    req.Tags,
	})
}

type TollReviewRequest struct {
	Reviewed bool `json:"reviewed"`
}

// SetTollReview marks a drive as reviewed without toll (or reopens it).
func (h *DriveHandler) SetTollReview(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	driveID := chi.URLParam(r, "driveId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req TollReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if err := h.repo.SetDriveTollReviewed(r.Context(), driveID, vehicleID, req.Reviewed); err != nil {
		writeRepoError(w, r, err, "Failed to update toll review")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "reviewed": req.Reviewed})
}

// GetTollDetection returns the cached toll detection result for a drive, or null if
// detection has never been run on it.
func (h *DriveHandler) GetTollDetection(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	driveID := chi.URLParam(r, "driveId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer); v == nil {
		return
	}

	detection, err := h.repo.GetTollDetectionByDrive(r.Context(), vehicleID, driveID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeJSON(w, http.StatusOK, nil)
			return
		}
		writeRepoError(w, r, err, "Failed to get toll detection")
		return
	}

	writeJSON(w, http.StatusOK, detection)
}

// DetectTolls fetches the drive's GPS trace and matches it against the toll station
// reference, replacing any previously cached result for this drive.
func (h *DriveHandler) DetectTolls(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	driveID := chi.URLParam(r, "driveId")

	vehicle := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if vehicle == nil {
		return
	}

	detection, err := h.tollDetectionService.DetectTolls(r.Context(), vehicle, driveID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeAPIError(w, http.StatusNotFound, apierror.New("drive.not_found", "Drive not found"))
			return
		}
		if errors.Is(err, services.ErrNoGPSTrace) {
			writeErr(w, http.StatusBadRequest, services.ErrNoGPSTrace)
			return
		}
		slog.ErrorContext(r.Context(), "toll detection failed", "component", "api", "error", err)
		writeAPIError(w, http.StatusBadGateway, apierror.New("toll.detection_failed", "Toll detection failed (TeslaMateAPI unreachable or drive unavailable)"))
		return
	}

	writeJSON(w, http.StatusOK, detection)
}

// ApplyTollEstimate records the estimated toll of one drive as an AUTO_TOLL expense.
func (h *DriveHandler) ApplyTollEstimate(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	driveID := chi.URLParam(r, "driveId")

	vehicle := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if vehicle == nil {
		return
	}

	result, err := h.tollDetectionService.ApplyTollEstimate(r.Context(), vehicle, driveID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeAPIError(w, http.StatusNotFound, apierror.New("drive.not_found", "Drive not found"))
			return
		}
		slog.ErrorContext(r.Context(), "apply toll estimate failed", "component", "api", "error", err)
		writeAPIError(w, http.StatusBadGateway, apierror.New("toll.apply_failed", "Could not apply the toll fare"))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type ApplyTollEstimatesRequest struct {
	DriveIDs []string `json:"drive_ids"`
}

// ApplyTollEstimatesBulk applies the estimated toll to several drives of one vehicle.
func (h *DriveHandler) ApplyTollEstimatesBulk(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	vehicle := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if vehicle == nil {
		return
	}

	var req ApplyTollEstimatesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}
	if len(req.DriveIDs) == 0 {
		writeAPIError(w, http.StatusBadRequest, apierror.New("trip.drive_required", "Select at least one drive"))
		return
	}
	if len(req.DriveIDs) > services.MaxBulkTollDrives {
		writeAPIError(w, http.StatusBadRequest, apierror.Newf("toll.bulk_limit", "At most %d drives at a time", services.MaxBulkTollDrives))
		return
	}

	writeJSON(w, http.StatusOK, h.tollDetectionService.ApplyTollEstimatesBulk(r.Context(), vehicle, req.DriveIDs))
}

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

type SaveManualDriveRequest struct {
	StartTime         time.Time  `json:"start_time"`
	EndTime           *time.Time `json:"end_time"`
	DistanceKm        float64    `json:"distance_km"`
	DurationMin       *int       `json:"duration_min"`
	EnergyConsumedKwh *float64   `json:"energy_consumed_kwh"`
	StartOdometer     *float64   `json:"start_odometer"`
	EndOdometer       *float64   `json:"end_odometer"`
	StartAddress      *string    `json:"start_address"`
	EndAddress        *string    `json:"end_address"`
	Tags              []string   `json:"tags"`
	DriverID          *string    `json:"driver_id"`
}

// Create records a manually entered drive.
func (h *DriveHandler) Create(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	vehicle := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if vehicle == nil {
		return
	}

	var req SaveManualDriveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if req.StartTime.IsZero() {
		writeAPIError(w, http.StatusBadRequest, apierror.New("drive.missing_start_time", "Start time is required"))
		return
	}
	if !ingest.ValidDriveDistance(req.DistanceKm) {
		writeAPIError(w, http.StatusBadRequest, apierror.New("drive.invalid_distance", "Distance must be between 0.1 and 3000 km"))
		return
	}

	timings := ingest.NormalizeDrive(ingest.DriveInput{
		Start:           req.StartTime,
		End:             req.EndTime,
		DurationMin:     req.DurationMin,
		DistanceKm:      req.DistanceKm,
		EnergyKwh:       req.EnergyConsumedKwh,
		VehicleKwh100km: vehicle.EstimatedKwh100km,
	})
	req.StartOdometer, req.EndOdometer = ingest.CompleteOdometers(req.StartOdometer, req.EndOdometer, req.DistanceKm)

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	d := &models.Drive{
		VehicleID:           vehicleID,
		StartTime:           req.StartTime,
		EndTime:             timings.End,
		StartOdometer:       req.StartOdometer,
		EndOdometer:         req.EndOdometer,
		DistanceKm:          req.DistanceKm,
		DurationMin:         timings.DurationMin,
		StartAddress:        req.StartAddress,
		EndAddress:          req.EndAddress,
		EnergyConsumedKwh:   &timings.EnergyKwh,
		ConsumptionKwh100km: &timings.Kwh100km,
		Tags:                tags,
		DriverID:            req.DriverID,
		IsManual:            true,
		EnergyEstimated:     timings.EnergyEstimated,
	}

	if err := h.repo.CreateManualDrive(r.Context(), d); err != nil {
		writeRepoError(w, r, err, "Failed to create manual drive")
		return
	}

	// Update vehicle odometer if this drive ended higher than current odometer
	if req.EndOdometer != nil && *req.EndOdometer > vehicle.CurrentOdometer {
		_ = h.repo.UpdateVehicleOdometer(r.Context(), vehicleID, *req.EndOdometer)
	}

	writeJSON(w, http.StatusCreated, d)
}

// Update edits a manually entered drive.
func (h *DriveHandler) Update(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	vehicle := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if vehicle == nil {
		return
	}

	driveID := chi.URLParam(r, "driveId")
	existing, err := h.repo.GetDriveByID(r.Context(), driveID, vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to get drive")
		return
	}

	if !existing.IsManual {
		writeAPIError(w, http.StatusForbidden, apierror.New("drive.cannot_edit_synced", "Synchronized drives cannot be edited directly"))
		return
	}

	var req SaveManualDriveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if !req.StartTime.IsZero() {
		existing.StartTime = req.StartTime
	}
	if req.DistanceKm > 0 {
		if !ingest.ValidDriveDistance(req.DistanceKm) {
			writeAPIError(w, http.StatusBadRequest, apierror.New("drive.invalid_distance", "Distance must be between 0.1 and 3000 km"))
			return
		}
		existing.DistanceKm = req.DistanceKm
	}

	if req.EndTime != nil && req.EndTime.After(existing.StartTime) {
		existing.EndTime = *req.EndTime
	} else if req.DurationMin != nil && *req.DurationMin > 0 {
		existing.EndTime = existing.StartTime.Add(time.Duration(*req.DurationMin) * time.Minute)
	}
	existing.DurationMin = ingest.DurationMinutes(existing.StartTime, existing.EndTime)

	if req.EnergyConsumedKwh != nil && *req.EnergyConsumedKwh > 0 {
		energy := *req.EnergyConsumedKwh
		cons100 := ingest.Consumption100km(energy, existing.DistanceKm)
		existing.EnergyConsumedKwh = &energy
		existing.ConsumptionKwh100km = &cons100
		existing.EnergyEstimated = false
	} else if existing.EnergyEstimated {
		// Keep an estimated energy in line with the (possibly edited) distance and the vehicle's current estimate
		energy, cons100 := ingest.EstimateDriveEnergy(vehicle.EstimatedKwh100km, existing.DistanceKm)
		existing.EnergyConsumedKwh = &energy
		existing.ConsumptionKwh100km = &cons100
	} else if existing.EnergyConsumedKwh != nil && *existing.EnergyConsumedKwh > 0 {
		// A typed energy stays, its consumption follows the distance
		cons100 := ingest.Consumption100km(*existing.EnergyConsumedKwh, existing.DistanceKm)
		existing.ConsumptionKwh100km = &cons100
	}

	if req.StartOdometer != nil {
		existing.StartOdometer = req.StartOdometer
	}
	if req.EndOdometer != nil {
		existing.EndOdometer = req.EndOdometer
	}
	if req.StartAddress != nil {
		existing.StartAddress = req.StartAddress
	}
	if req.EndAddress != nil {
		existing.EndAddress = req.EndAddress
	}
	if req.Tags != nil {
		existing.Tags = req.Tags
	}
	existing.DriverID = req.DriverID

	if err := h.repo.UpdateManualDrive(r.Context(), existing); err != nil {
		writeRepoError(w, r, err, "Failed to update manual drive")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

type UpdateDriverRequest struct {
	DriverID *string `json:"driver_id"`
}

// UpdateDriver assigns or changes the driver on any drive (manual or synced).
func (h *DriveHandler) UpdateDriver(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	driveID := chi.URLParam(r, "driveId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req UpdateDriverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if err := h.repo.SetDriveDriver(r.Context(), driveID, vehicleID, req.DriverID); err != nil {
		writeRepoError(w, r, err, "Failed to update driver")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "driver_id": req.DriverID})
}

// Delete removes a manually entered drive.
func (h *DriveHandler) Delete(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	driveID := chi.URLParam(r, "driveId")
	existing, err := h.repo.GetDriveByID(r.Context(), driveID, vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to get drive")
		return
	}

	if !existing.IsManual {
		writeAPIError(w, http.StatusForbidden, apierror.New("drive.cannot_delete_synced", "Synchronized drives cannot be deleted manually"))
		return
	}

	if err := h.repo.DeleteManualDrive(r.Context(), driveID, vehicleID); err != nil {
		writeRepoError(w, r, err, "Failed to delete manual drive")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseDriveFilter(r *http.Request) database.DriveFilter {
	filter := database.DriveFilter{
		Tag:             r.URL.Query().Get("tag"),
		UnqualifiedOnly: r.URL.Query().Get("unqualified") == "true",
		HasToll:         r.URL.Query().Get("has_toll") == "true",
		TripGroupID:     r.URL.Query().Get("trip_group_id"),
		DriveID:         r.URL.Query().Get("drive_id"),
		Query:           strings.TrimSpace(r.URL.Query().Get("q")),
	}

	switch src := r.URL.Query().Get("toll_source"); src {
	case models.ExpenseSourceManual, models.ExpenseSourceAutoToll:
		filter.TollSource = src
		filter.HasToll = true
	}

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			filter.From = &t
		} else if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			filter.From = &t
		}
	}

	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			filter.To = &t
		} else if t, err := time.Parse("2006-01-02", toStr); err == nil {
			tEnd := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filter.To = &tEnd
		}
	}
	return filter
}
