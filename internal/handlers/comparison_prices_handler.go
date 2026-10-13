package handlers

import (
	"net/http"

	"github.com/teslacost/teslacost/internal/models"
)

// PriceHistory supplies advisory references, never additional validation limits.
func (h *ComparisonHandler) PriceHistory(w http.ResponseWriter, r *http.Request) {
	vehicleID := r.URL.Query().Get("vehicle_id")
	if requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer) == nil {
		return
	}
	history, err := h.repo.GetComparisonPriceHistory(r.Context(), vehicleID, r.URL.Query().Get("fuel_type"))
	if err != nil {
		writeRepoError(w, r, err, "Failed to read comparison price history")
		return
	}
	writeJSON(w, http.StatusOK, history)
}
