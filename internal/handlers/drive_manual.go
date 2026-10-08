package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services/ingest"
)

type SaveManualDriveRequest struct {
	StartTime         time.Time  `json:"start_time"`
	EndTime           *time.Time `json:"end_time"`
	DistanceKm        float64    `json:"distance_km"`
	DurationMin       *int       `json:"duration_min"`
	EnergyConsumedKwh *float64   `json:"energy_consumed_kwh"`
	StartOdometer     *float64   `json:"start_odometer"`
	EndOdometer       *float64   `json:"end_odometer"`
	StartAddress      *string    `json:"start_address"`
	EndAddress        *string    `json:"end_address"`
	Tags              []string   `json:"tags"`
	DriverID          *string    `json:"driver_id"`
}

// Create records a manually entered drive.
func (h *DriveHandler) Create(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	vehicle := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if vehicle == nil {
		return
	}

	var req SaveManualDriveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if req.StartTime.IsZero() {
		writeAPIError(w, http.StatusBadRequest, apierror.New("drive.missing_start_time", "Start time is required"))
		return
	}
	if !ingest.ValidDriveDistance(req.DistanceKm) {
		writeAPIError(w, http.StatusBadRequest, apierror.New("drive.invalid_distance", "Distance must be between 0.1 and 3000 km"))
		return
	}

	if !h.driverBelongs(w, r, vehicleID, req.DriverID) {
		return
	}

	timings := ingest.NormalizeDrive(ingest.DriveInput{
		Start:           req.StartTime,
		End:             req.EndTime,
		DurationMin:     req.DurationMin,
		DistanceKm:      req.DistanceKm,
		EnergyKwh:       req.EnergyConsumedKwh,
		VehicleKwh100km: vehicle.EstimatedKwh100km,
	})
	req.StartOdometer, req.EndOdometer = ingest.CompleteOdometers(req.StartOdometer, req.EndOdometer, req.DistanceKm)

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	d := &models.Drive{
		VehicleID:           vehicleID,
		StartTime:           req.StartTime,
		EndTime:             timings.End,
		StartOdometer:       req.StartOdometer,
		EndOdometer:         req.EndOdometer,
		DistanceKm:          req.DistanceKm,
		DurationMin:         timings.DurationMin,
		StartAddress:        req.StartAddress,
		EndAddress:          req.EndAddress,
		EnergyConsumedKwh:   &timings.EnergyKwh,
		ConsumptionKwh100km: &timings.Kwh100km,
		Tags:                tags,
		DriverID:            req.DriverID,
		IsManual:            true,
		EnergyEstimated:     timings.EnergyEstimated,
	}

	if err := h.repo.CreateManualDrive(r.Context(), d); err != nil {
		writeRepoError(w, r, err, "Failed to create manual drive")
		return
	}

	// Update vehicle odometer if this drive ended higher than current odometer
	if req.EndOdometer != nil && *req.EndOdometer > vehicle.CurrentOdometer {
		_ = h.repo.UpdateVehicleOdometer(r.Context(), vehicleID, *req.EndOdometer)
	}

	writeJSON(w, http.StatusCreated, d)
}

// Update edits a manually entered drive.
func (h *DriveHandler) Update(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	vehicle := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if vehicle == nil {
		return
	}

	driveID := chi.URLParam(r, "driveId")
	existing, err := h.repo.GetDriveByID(r.Context(), driveID, vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to get drive")
		return
	}

	if !existing.IsManual {
		writeAPIError(w, http.StatusForbidden, apierror.New("drive.cannot_edit_synced", "Synchronized drives cannot be edited directly"))
		return
	}

	var req SaveManualDriveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if !req.StartTime.IsZero() {
		existing.StartTime = req.StartTime
	}
	if req.DistanceKm > 0 {
		if !ingest.ValidDriveDistance(req.DistanceKm) {
			writeAPIError(w, http.StatusBadRequest, apierror.New("drive.invalid_distance", "Distance must be between 0.1 and 3000 km"))
			return
		}
		existing.DistanceKm = req.DistanceKm
	}

	if req.EndTime != nil && req.EndTime.After(existing.StartTime) {
		existing.EndTime = *req.EndTime
	} else if req.DurationMin != nil && *req.DurationMin > 0 {
		existing.EndTime = existing.StartTime.Add(time.Duration(*req.DurationMin) * time.Minute)
	}
	existing.DurationMin = ingest.DurationMinutes(existing.StartTime, existing.EndTime)

	if req.EnergyConsumedKwh != nil && *req.EnergyConsumedKwh > 0 {
		energy := *req.EnergyConsumedKwh
		cons100 := ingest.Consumption100km(energy, existing.DistanceKm)
		existing.EnergyConsumedKwh = &energy
		existing.ConsumptionKwh100km = &cons100
		existing.EnergyEstimated = false
	} else if existing.EnergyEstimated {
		// Keep an estimated energy in line with the (possibly edited) distance and the vehicle's current estimate
		energy, cons100 := ingest.EstimateDriveEnergy(vehicle.EstimatedKwh100km, existing.DistanceKm)
		existing.EnergyConsumedKwh = &energy
		existing.ConsumptionKwh100km = &cons100
	} else if existing.EnergyConsumedKwh != nil && *existing.EnergyConsumedKwh > 0 {
		// A typed energy stays, its consumption follows the distance
		cons100 := ingest.Consumption100km(*existing.EnergyConsumedKwh, existing.DistanceKm)
		existing.ConsumptionKwh100km = &cons100
	}

	if req.StartOdometer != nil {
		existing.StartOdometer = req.StartOdometer
	}
	if req.EndOdometer != nil {
		existing.EndOdometer = req.EndOdometer
	}
	if req.StartAddress != nil {
		existing.StartAddress = req.StartAddress
	}
	if req.EndAddress != nil {
		existing.EndAddress = req.EndAddress
	}
	if req.Tags != nil {
		existing.Tags = req.Tags
	}
	existing.DriverID = req.DriverID

	if err := h.repo.UpdateManualDrive(r.Context(), existing); err != nil {
		writeRepoError(w, r, err, "Failed to update manual drive")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

type UpdateDriverRequest struct {
	DriverID *string `json:"driver_id"`
}

// UpdateDriver assigns or changes the driver on any drive (manual or synced).
func (h *DriveHandler) UpdateDriver(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	driveID := chi.URLParam(r, "driveId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req UpdateDriverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if !h.driverBelongs(w, r, vehicleID, req.DriverID) {
		return
	}
	if err := h.repo.SetDriveDriver(r.Context(), driveID, vehicleID, req.DriverID); err != nil {
		writeRepoError(w, r, err, "Failed to update driver")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "driver_id": req.DriverID})
}

// Delete removes a manually entered drive.
func (h *DriveHandler) Delete(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	driveID := chi.URLParam(r, "driveId")
	existing, err := h.repo.GetDriveByID(r.Context(), driveID, vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to get drive")
		return
	}

	if !existing.IsManual {
		writeAPIError(w, http.StatusForbidden, apierror.New("drive.cannot_delete_synced", "Synchronized drives cannot be deleted manually"))
		return
	}

	if err := h.repo.DeleteManualDrive(r.Context(), driveID, vehicleID); err != nil {
		writeRepoError(w, r, err, "Failed to delete manual drive")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// driverBelongs answers 400 when a driver is given that is not a person of the vehicle.
func (h *DriveHandler) driverBelongs(w http.ResponseWriter, r *http.Request, vehicleID string, driverID *string) bool {
	if driverID == nil || *driverID == "" {
		return true
	}
	ok, err := h.repo.VehiclePersonExists(r.Context(), vehicleID, *driverID)
	if err != nil || !ok {
		writeAPIError(w, http.StatusBadRequest, apierror.New("person.not_found", "Driver not found"))
		return false
	}
	return true
}
