package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

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
