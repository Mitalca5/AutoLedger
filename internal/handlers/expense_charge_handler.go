package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services/ingest"
)

func (h *ExpenseHandler) ListCharges(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer); v == nil {
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit
	missingCostOnly := r.URL.Query().Get("missing_cost") == "true"

	charges, total, err := h.repo.ListCharges(r.Context(), vehicleID, missingCostOnly, limit, offset)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list charges")
		return
	}
	if charges == nil {
		charges = []models.ChargeLog{}
	}
	missingCount, err := h.repo.CountChargesWithoutCost(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list charges")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"charges":              charges,
		"total":                total,
		"page":                 page,
		"limit":                limit,
		"charges_without_cost": missingCount,
	})
}

type SaveChargeRequest struct {
	Date       string       `json:"date"`
	EndDate    *string      `json:"end_date"`
	Address    *string      `json:"address"`
	KwhAdded   float64      `json:"kwh_added"`
	Cost       *money.Cents `json:"cost"`
	Currency   string       `json:"currency"`
	FxRate     *float64     `json:"fx_rate"`
	Odometer   *float64     `json:"odometer"`
	Notes      *string      `json:"notes"`
	DocumentID *string      `json:"document_id"`
}

func buildCharge(vehicleID, baseCurrency string, req *SaveChargeRequest) (*models.ChargeLog, error) {
	date, err := parseDate(req.Date)
	if err != nil {
		return nil, err
	}
	endDate, err := parseOptionalDate(req.EndDate)
	if err != nil {
		return nil, err
	}
	if endDate != nil && endDate.Before(date) {
		return nil, apierror.New("charge.end_before_start", "The end of the charge is before its start")
	}
	if err := validateQuantity(req.KwhAdded, ingest.MaxChargeKwh); err != nil {
		return nil, apierror.New("charge.energy_invalid", "Invalid energy added")
	}
	if req.Cost != nil {
		if err := validateAmount(*req.Cost, true); err != nil {
			return nil, err
		}
	}
	curr, fxRate, err := normalizeCurrency(req.Currency, baseCurrency, req.FxRate)
	if err != nil {
		return nil, err
	}
	odometer := req.Odometer
	if odometer != nil && *odometer <= 0 {
		odometer = nil
	}
	return &models.ChargeLog{
		VehicleID:  vehicleID,
		Date:       date,
		EndDate:    endDate,
		Address:    req.Address,
		KwhAdded:   req.KwhAdded,
		Cost:       req.Cost,
		Currency:   curr,
		FxRate:     fxRate,
		Odometer:   odometer,
		Notes:      req.Notes,
		DocumentID: req.DocumentID,
	}, nil
}

// decodeCharge reads and validates a charge from the request body. It writes the error response and returns
// false when the payload is invalid or carries no cost (costRequired is the error for that case).
func decodeCharge(w http.ResponseWriter, r *http.Request, vehicleID, baseCurrency string, costRequired *apierror.Error) (*models.ChargeLog, bool) {
	var req SaveChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return nil, false
	}
	c, err := buildCharge(vehicleID, baseCurrency, &req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return nil, false
	}
	if c.Cost == nil {
		writeAPIError(w, http.StatusBadRequest, costRequired)
		return nil, false
	}
	return c, true
}

func (h *ExpenseHandler) CreateManualCharge(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if v == nil {
		return
	}

	c, ok := decodeCharge(w, r, vehicleID, v.Currency, apierror.New("charge.manual_cost_required", "The cost of a manual charge is required"))
	if !ok {
		return
	}
	if c.KwhAdded <= 0 {
		writeAPIError(w, http.StatusBadRequest, apierror.New("charge.energy_positive", "The energy added must be positive"))
		return
	}

	if err := h.repo.CreateManualCharge(r.Context(), c); err != nil {
		writeRepoError(w, r, err, "Failed to record charge")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *ExpenseHandler) UpdateCharge(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	chargeID := chi.URLParam(r, "chargeId")
	v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if v == nil {
		return
	}

	c, ok := decodeCharge(w, r, vehicleID, v.Currency, apierror.New("charge.cost_required", "The cost is required"))
	if !ok {
		return
	}
	c.ID = chargeID

	if err := h.repo.UpdateCharge(r.Context(), c); err != nil {
		writeRepoError(w, r, err, "Failed to update charge")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *ExpenseHandler) DeleteManualCharge(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	chargeID := chi.URLParam(r, "chargeId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	if err := h.repo.DeleteManualCharge(r.Context(), vehicleID, chargeID); err != nil {
		writeRepoError(w, r, err, "Failed to delete charge")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// ============================================================================
// Documents & Invoices
// ============================================================================
