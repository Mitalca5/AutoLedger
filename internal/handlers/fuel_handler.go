package handlers

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

// FuelHandler manages the fuel fill-ups of combustion vehicles.
type FuelHandler struct {
	repo *database.Repository
}

func NewFuelHandler(repo *database.Repository) *FuelHandler {
	return &FuelHandler{repo: repo}
}

// SaveFuelLogRequest is the payload shared by Create and Update. Amount, liters and price per liter
// are linked: any two of them determine the third.
type SaveFuelLogRequest struct {
	Date          string       `json:"date"`
	Odometer      *float64     `json:"odometer"` // Optional: estimated from the odometer readings when absent
	Amount        *money.Cents `json:"amount"`
	Liters        *float64     `json:"liters"`
	PricePerLiter *float64     `json:"price_per_liter"`
	FuelType      *string      `json:"fuel_type"`
	IsFullTank    *bool        `json:"is_full_tank"` // Defaults to true
	Notes         *string      `json:"notes"`
}

func positiveOrNil(v *float64) *float64 {
	if v == nil || *v == 0 {
		return nil
	}
	return v
}

// buildFuelLog validates a request and derives the missing one of amount / liters / price per liter.
func buildFuelLog(vehicleID string, req *SaveFuelLogRequest) (*models.FuelLog, error) {
	date, err := parseDate(req.Date)
	if err != nil {
		return nil, apierror.New("request.invalid_date", "Invalid date")
	}
	if req.Odometer != nil {
		if err := validateRange(*req.Odometer, 0, 2_000_000, apierror.Newf("odometer.range", "Invalid mileage (0 to %.0f km)", apierror.Km(2_000_000))); err != nil {
			return nil, err
		}
	}

	liters, price := positiveOrNil(req.Liters), positiveOrNil(req.PricePerLiter)
	if liters != nil {
		if err := validateRange(*liters, 0.01, 500, apierror.New("fuel.liters_range", "Invalid quantity (0.01 to 500 L)")); err != nil {
			return nil, err
		}
	}
	if price != nil {
		if err := validateRange(*price, 0.001, 10, apierror.New("fuel.price_range", "Invalid price per litre (0 to 10)")); err != nil {
			return nil, err
		}
	}

	var amount money.Cents
	if req.Amount != nil {
		amount = *req.Amount
	}
	switch {
	case amount == 0 && liters != nil && price != nil:
		amount = money.FromFloat(*liters * *price)
	case amount > 0 && liters != nil && price == nil:
		p := math.Round(amount.Float() / *liters * 1000) / 1000
		price = &p
	case amount > 0 && liters == nil && price != nil:
		l := math.Round(amount.Float() / *price * 100) / 100
		liters = &l
	}
	if err := validateAmount(amount, false); err != nil {
		return nil, apierror.New("fuel.amount_required", "The fill-up amount is required (or litres and price per litre)")
	}

	var fuelType *string
	if req.FuelType != nil && *req.FuelType != "" {
		if !models.FuelTypes[*req.FuelType] {
			return nil, apierror.New("fuel.type_invalid", "Invalid fuel")
		}
		fuelType = req.FuelType
	}
	var notes *string
	if req.Notes != nil && strings.TrimSpace(*req.Notes) != "" {
		n := strings.TrimSpace(*req.Notes)
		notes = &n
	}
	isFull := true
	if req.IsFullTank != nil {
		isFull = *req.IsFullTank
	}

	return &models.FuelLog{
		VehicleID: vehicleID, Date: date, Odometer: req.Odometer, Amount: amount,
		Liters: liters, PricePerLiter: price, FuelType: fuelType, IsFullTank: isFull, Notes: notes,
	}, nil
}

// checkOdometerOrder rejects a mileage that contradicts the manual points (readings and fill-ups with a
// mileage) around it in time. The point being edited (id) is ignored; points at the same instant are not compared.
func checkOdometerOrder(others []models.OdometerPoint, id string, date time.Time, odometer float64) error {
	for _, o := range others {
		if o.ID == id {
			continue
		}
		if o.Date.Before(date) && o.Odometer > odometer {
			return apierror.New("odometer.inconsistent_older", "Inconsistent mileage: an older reading or fill-up already shows a higher mileage")
		}
		if o.Date.After(date) && o.Odometer < odometer {
			return apierror.New("odometer.inconsistent_newer", "Inconsistent mileage: a newer reading or fill-up shows a lower mileage")
		}
	}
	return nil
}

// requireRefuelable loads the vehicle for a write and checks that it can be refuelled.
func (h *FuelHandler) requireRefuelable(w http.ResponseWriter, r *http.Request, vehicleID string) bool {
	v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if v == nil {
		return false
	}
	if !v.CanRefuel() {
		writeAPIError(w, http.StatusBadRequest, apierror.New("fuel.combustion_only", "Fuel fill-ups do not apply to electric vehicles"))
		return false
	}
	return true
}

// buildChecked decodes and validates a request against the existing fill-ups; id is empty on creation.
func (h *FuelHandler) buildChecked(w http.ResponseWriter, r *http.Request, vehicleID, id string) (*models.FuelLog, bool) {
	var req SaveFuelLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return nil, false
	}
	f, err := buildFuelLog(vehicleID, &req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return nil, false
	}
	if f.Odometer != nil {
		points, err := h.repo.ListManualOdometerPoints(r.Context(), vehicleID)
		if err != nil {
			writeRepoError(w, r, err, "Failed to check odometer points")
			return nil, false
		}
		if err := checkOdometerOrder(points, id, f.Date, *f.Odometer); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return nil, false
		}
	}
	return f, true
}

// List returns the fill-ups with the consumption measured on each segment, and the overall statistics.
func (h *FuelHandler) List(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer) == nil {
		return
	}
	logs, err := h.repo.ListFuelLogs(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list fill-ups")
		return
	}
	readings, err := h.repo.ListOdometerCheckpoints(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list odometer readings")
		return
	}
	ownership, err := h.repo.GetVehicleOwnership(r.Context(), vehicleID)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		writeRepoError(w, r, err, "Failed to load ownership")
		return
	}
	writeJSON(w, http.StatusOK, services.ComputeFuelStats(logs, services.BuildOdometerRefs(readings, ownership)))
}

func (h *FuelHandler) Create(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if !h.requireRefuelable(w, r, vehicleID) {
		return
	}
	f, ok := h.buildChecked(w, r, vehicleID, "")
	if !ok {
		return
	}
	if err := h.repo.CreateFuelLog(r.Context(), f); err != nil {
		writeRepoError(w, r, err, "Failed to record fill-up")
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func (h *FuelHandler) Update(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if !h.requireRefuelable(w, r, vehicleID) {
		return
	}
	id := chi.URLParam(r, "fuelLogId")
	f, ok := h.buildChecked(w, r, vehicleID, id)
	if !ok {
		return
	}
	f.ID = id
	if err := h.repo.UpdateFuelLog(r.Context(), f); err != nil {
		writeRepoError(w, r, err, "Failed to update fill-up")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (h *FuelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor) == nil {
		return
	}
	if err := h.repo.DeleteFuelLog(r.Context(), vehicleID, chi.URLParam(r, "fuelLogId")); err != nil {
		writeRepoError(w, r, err, "Failed to delete fill-up")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
