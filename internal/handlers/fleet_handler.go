package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

type FleetHandler struct {
	fleetService *services.FleetService
}

func NewFleetHandler(fleetService *services.FleetService) *FleetHandler {
	return &FleetHandler{fleetService: fleetService}
}

func (h *FleetHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	summary, err := h.fleetService.GetSummary(r.Context(), userID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to load fleet summary")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

type setFleetBudgetRequest struct {
	Amount *money.Cents `json:"amount"`
}

// SetBudget stores the household monthly budget, or removes it when the amount is null.
func (h *FleetHandler) SetBudget(w http.ResponseWriter, r *http.Request) {
	var req setFleetBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}
	if req.Amount != nil && (*req.Amount <= 0 || *req.Amount > money.Max) {
		writeAPIError(w, http.StatusBadRequest, apierror.New("fleet.invalid_budget", "The budget must be a positive amount"))
		return
	}
	if err := h.fleetService.SetMonthlyBudget(r.Context(), middleware.GetUserID(r.Context()), req.Amount); err != nil {
		writeRepoError(w, r, err, "Failed to save the fleet budget")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"monthly_budget": req.Amount})
}
