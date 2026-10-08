package handlers

import (
	"encoding/json"
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
	geocoder             Geocoder
	backfills            addressBackfills
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
