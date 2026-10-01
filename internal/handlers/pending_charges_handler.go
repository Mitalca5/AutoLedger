package handlers

import (
	"encoding/json"
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

	// Compute cost if needed
	var cost *money.Cents
	plan, _ := h.repo.GetVehicleTariffPlan(r.Context(), req.VehicleID)
	if plan != nil {
		computed, err := h.tariffService.CalculateSessionCost(plan, pc.StartTime, pc.EndTime, pc.EnergyKwh)
		if err == nil && computed > 0 {
			cost = &computed
		}
	}

	noteStr := "Qualified charging session"
	if pc.ChargerName != nil && *pc.ChargerName != "" {
		noteStr += " from " + *pc.ChargerName
	}

	charge := &models.ChargeLog{
		VehicleID:  req.VehicleID,
		Date:       pc.StartTime,
		EndDate:    &pc.EndTime,
		Address:    &pc.Location,
		KwhAdded:   pc.EnergyKwh,
		Cost:       cost,
		CostSource: "HOMEASSISTANT",
		Currency:   vehicle.Currency,
		IsManual:   true,
		Notes:      &noteStr,
	}

	if err := h.repo.CreateManualCharge(r.Context(), charge); err != nil {
		writeRepoError(w, r, err, "Failed to record charge")
		return
	}

	_ = h.repo.DeletePendingCharge(r.Context(), pendingID, userID)

	writeJSON(w, http.StatusOK, charge)
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
