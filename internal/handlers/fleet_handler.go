package handlers

import (
	"net/http"

	"github.com/teslacost/teslacost/internal/middleware"
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
