package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

type CreateMaintenanceRequest struct {
	Category                 string      `json:"category"`
	Amount                   money.Cents `json:"amount"`
	Currency                 string      `json:"currency"`
	FxRate                   *float64    `json:"fx_rate"`
	Date                     string      `json:"date"`
	Odometer                 *float64    `json:"odometer"`
	IsRecurring              bool        `json:"is_recurring"`
	RecurrenceIntervalMonths *int        `json:"recurrence_interval_months"`
	RecurrenceEndDate        *string     `json:"recurrence_end_date"`
	AmortizationMode         string      `json:"amortization_mode"`
	CoverageKm               *float64    `json:"coverage_km"`
	CoverageMonths           *int        `json:"coverage_months"`
	ClosesMaintenanceID      *string     `json:"closes_maintenance_id"`
	Description              string      `json:"description"`
	DocumentID               *string     `json:"document_id"`
}

// buildMaintenanceExpense validates a maintenance / fixed expense payload.
func buildMaintenanceExpense(vehicleID, baseCurrency string, req *CreateMaintenanceRequest) (*models.MaintenanceExpense, error) {
	category := strings.ToUpper(strings.TrimSpace(req.Category))
	if !maintenanceCategories[category] {
		return nil, apierror.New("expense.category_invalid", "Invalid expense category")
	}
	if err := validateAmount(req.Amount, false); err != nil {
		return nil, err
	}
	mDate, err := parseDate(req.Date)
	if err != nil {
		return nil, err
	}
	curr, fxRate, err := normalizeCurrency(req.Currency, baseCurrency, req.FxRate)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Description) == "" {
		return nil, apierror.New("expense.description_required", "The description is required")
	}

	interval := req.RecurrenceIntervalMonths
	var endDate *time.Time
	if req.IsRecurring {
		if interval == nil || *interval <= 0 || *interval > 120 {
			return nil, apierror.New("expense.recurrence_interval", "A recurring expense needs a frequency in months (1 to 120)")
		}
		if endDate, err = parseOptionalDate(req.RecurrenceEndDate); err != nil {
			return nil, err
		}
		if endDate != nil && endDate.Before(mDate) {
			return nil, apierror.New("expense.recurrence_end", "The end of the recurrence is before the first due date")
		}
	} else {
		interval = nil
	}

	odometer := req.Odometer
	if odometer != nil && *odometer <= 0 {
		odometer = nil
	}

	amortMode := strings.ToUpper(strings.TrimSpace(req.AmortizationMode))
	if amortMode == "" {
		amortMode = "NONE"
	}
	if amortMode != "NONE" && amortMode != "DISTANCE" && amortMode != "DURATION" && amortMode != "HYBRID" {
		return nil, apierror.New("expense.amortization_mode", "Invalid smoothing mode (NONE, DISTANCE, DURATION, HYBRID)")
	}

	var covKm *float64
	var covMonths *int
	var closesID *string

	if amortMode == "DISTANCE" || amortMode == "HYBRID" {
		defaultKm := 50000.0
		if req.CoverageKm != nil && *req.CoverageKm > 0 {
			defaultKm = *req.CoverageKm
		}
		covKm = &defaultKm
	}
	if amortMode == "DURATION" || amortMode == "HYBRID" {
		defaultM := 24
		if req.CoverageMonths != nil && *req.CoverageMonths > 0 {
			defaultM = *req.CoverageMonths
		}
		covMonths = &defaultM
	}
	if req.ClosesMaintenanceID != nil && strings.TrimSpace(*req.ClosesMaintenanceID) != "" {
		trimmed := strings.TrimSpace(*req.ClosesMaintenanceID)
		closesID = &trimmed
	}

	return &models.MaintenanceExpense{
		VehicleID:                vehicleID,
		Category:                 category,
		Amount:                   req.Amount,
		Currency:                 curr,
		FxRate:                   fxRate,
		Date:                     mDate,
		Odometer:                 odometer,
		IsRecurring:              req.IsRecurring,
		RecurrenceIntervalMonths: interval,
		RecurrenceEndDate:        endDate,
		AmortizationMode:         amortMode,
		CoverageKm:               covKm,
		CoverageMonths:           covMonths,
		ClosesMaintenanceID:      closesID,
		Description:              req.Description,
		DocumentID:               req.DocumentID,
	}, nil
}

func (h *ExpenseHandler) CreateMaintenance(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if v == nil {
		return
	}

	var req CreateMaintenanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	m, err := buildMaintenanceExpense(vehicleID, v.Currency, &req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	if m.Odometer == nil && m.AmortizationMode != "NONE" {
		if km, _, ok, err := estimateOdometer(r.Context(), h.repo, vehicleID, m.Date); err == nil && ok && km > 0 {
			odo := math.Round(km)
			m.Odometer = &odo
		}
	}

	if err := h.repo.CreateMaintenanceExpense(r.Context(), m); err != nil {
		writeRepoError(w, r, err, "Failed to record maintenance expense")
		return
	}

	writeJSON(w, http.StatusCreated, m)
}

func (h *ExpenseHandler) ListMaintenance(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer); v == nil {
		return
	}

	list, err := h.repo.ListMaintenanceExpenses(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list maintenance expenses")
		return
	}
	if list == nil {
		list = []models.MaintenanceExpense{}
	}

	writeJSON(w, http.StatusOK, list)
}

func (h *ExpenseHandler) UpdateMaintenance(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	maintID := chi.URLParam(r, "maintenanceId")
	v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if v == nil {
		return
	}

	var req CreateMaintenanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	m, err := buildMaintenanceExpense(vehicleID, v.Currency, &req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	m.ID = maintID

	if m.Odometer == nil && m.AmortizationMode != "NONE" {
		if km, _, ok, err := estimateOdometer(r.Context(), h.repo, vehicleID, m.Date); err == nil && ok && km > 0 {
			odo := math.Round(km)
			m.Odometer = &odo
		}
	}

	if err := h.repo.UpdateMaintenanceExpense(r.Context(), m); err != nil {
		writeRepoError(w, r, err, "Failed to update maintenance expense")
		return
	}

	writeJSON(w, http.StatusOK, m)
}

func (h *ExpenseHandler) DeleteMaintenance(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	maintID := chi.URLParam(r, "maintenanceId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	if err := h.repo.DeleteMaintenanceExpense(r.Context(), vehicleID, maintID); err != nil {
		writeRepoError(w, r, err, "Failed to delete maintenance expense")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Maintenance expense deleted successfully"})
}
