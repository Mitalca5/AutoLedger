package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

type PendingChargesHandler struct {
	repo          *database.Repository
	tariffService *services.TariffService
}

func NewPendingChargesHandler(repo *database.Repository, tariffService *services.TariffService) *PendingChargesHandler {
	return &PendingChargesHandler{
		repo:          repo,
		tariffService: tariffService,
	}
}

func (h *PendingChargesHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	charges, err := h.repo.ListPendingCharges(r.Context(), userID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list pending charges")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pending_charges": charges})
}

func (h *PendingChargesHandler) Assign(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	pendingID := chi.URLParam(r, "id")

	pc, err := h.repo.GetPendingChargeByID(r.Context(), pendingID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("charge.pending_not_found", "Pending charge not found"))
		return
	}

	var req models.AssignPendingChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	vehicle := requireVehicleAccess(w, r, h.repo, req.VehicleID, models.RoleEditor)
	if vehicle == nil {
		return
	}
	if !vehicle.CanCharge() {
		writeAPIError(w, http.StatusBadRequest, apierror.New("charge.electric_only", "Charging sessions only apply to electric vehicles"))
		return
	}

	// What the integration sent with the session: cost, battery levels, odometer
	event := pendingEvent(pc)
	var cost *money.Cents
	if event.Data.Cost != nil {
		c := money.FromFloat(*event.Data.Cost)
		cost = &c
	} else if event.Data.CostCents != nil {
		c := money.Cents(*event.Data.CostCents)
		cost = &c
	} else if plan, _ := h.repo.GetVehicleTariffPlanAt(r.Context(), vehicle.ID, h.tariffService.DayOf(pc.StartTime)); plan != nil {
		if computed, err := h.tariffService.CalculateSessionCost(plan, pc.StartTime, pc.EndTime, pc.EnergyKwh); err == nil && computed > 0 {
			cost = &computed
		}
	}

	charge := &models.ChargeLog{
		VehicleID:         vehicle.ID,
		Date:              pc.StartTime,
		EndDate:           &pc.EndTime,
		Address:           pc.Location,
		KwhAdded:          pc.EnergyKwh,
		Cost:              cost,
		CostSource:        "HOMEASSISTANT",
		Currency:          vehicle.Currency,
		Odometer:          event.Data.OdometerKm,
		StartBatteryLevel: event.Data.SocStart,
		EndBatteryLevel:   event.Data.SocEnd,
		ExternalID:        pc.ExternalID,
	}
	if err := h.repo.AssignPendingCharge(r.Context(), pendingID, userID, charge); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			// Assigned or dismissed meanwhile (a second click)
			writeAPIError(w, http.StatusNotFound, apierror.New("charge.pending_not_found", "Pending charge not found"))
			return
		}
		writeRepoError(w, r, err, "Failed to record charge")
		return
	}
	if charge.Odometer != nil && *charge.Odometer > vehicle.CurrentOdometer {
		_ = h.repo.UpdateVehicleOdometer(r.Context(), vehicle.ID, *charge.Odometer)
	}

	writeJSON(w, http.StatusOK, charge)
}

// pendingEvent is the event a pending charge was created from (stored in its raw data), empty when unreadable.
func pendingEvent(pc *models.PendingCharge) HAEventPayload {
	var event HAEventPayload
	if raw, ok := pc.RawData["raw_event"]; ok {
		if b, err := json.Marshal(raw); err == nil {
			_ = json.Unmarshal(b, &event)
		}
	}
	return event
}

func (h *PendingChargesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	pendingID := chi.URLParam(r, "id")

	if err := h.repo.DeletePendingCharge(r.Context(), pendingID, userID); err != nil {
		writeRepoError(w, r, err, "Failed to dismiss pending charge")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
