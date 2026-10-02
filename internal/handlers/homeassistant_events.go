package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services/ingest"
)

const (
	kmPerMileEvent     = 1.609344
	maxEventOdometerKm = 2_000_000
)

// distanceFactor is the multiplier that turns the distances of an event into kilometres.
func distanceFactor(w http.ResponseWriter, unit string) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "km":
		return 1, true
	case "mi":
		return kmPerMileEvent, true
	}
	writeAPIError(w, http.StatusBadRequest, apierror.Newf("integration.invalid_distance_unit", "Unknown distance unit %q (km or mi)", unit))
	return 0, false
}

func scaled(v *float64, factor float64) *float64 {
	if v == nil {
		return nil
	}
	out := *v * factor
	return &out
}

// eventOdometerKm is the odometer reading of an event in km: odometer_km as sent, or odometer in the event's unit.
func eventOdometerKm(d *HAEventData, factor float64) *float64 {
	if d.OdometerKm != nil {
		return d.OdometerKm
	}
	return scaled(d.Odometer, factor)
}

// namedVehicle loads the vehicle an event names, for the kinds of event that are never attributed by guess.
func (h *HomeAssistantHandler) namedVehicle(w http.ResponseWriter, r *http.Request, req *HAEventPayload) *models.Vehicle {
	id := optionalText(req.VehicleID)
	if id == nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("vehicle.not_specified", "The event must name its vehicle"))
		return nil
	}
	return requireVehicleAccess(w, r, h.repo, *id, models.RoleEditor)
}

func eventStart(req *HAEventPayload) *time.Time {
	switch {
	case req.Data.StartTime != nil:
		t := req.Data.StartTime.UTC()
		return &t
	case req.Timestamp != nil:
		t := req.Timestamp.UTC()
		return &t
	}
	return nil
}

// recordDrive stores a drive reported by an integration, once per event id and once per similar drive.
func (h *HomeAssistantHandler) recordDrive(w http.ResponseWriter, r *http.Request, req *HAEventPayload) {
	vehicle := h.namedVehicle(w, r, req)
	if vehicle == nil {
		return
	}
	factor, ok := distanceFactor(w, req.DistanceUnit)
	if !ok {
		return
	}
	start := eventStart(req)
	if start == nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("drive.missing_start_time", "Start time is required"))
		return
	}
	distance := 0.0
	if req.Data.Distance != nil {
		distance = *req.Data.Distance * factor
	}
	if !ingest.ValidDriveDistance(distance) {
		writeAPIError(w, http.StatusBadRequest, apierror.New("drive.invalid_distance", "Distance must be between 0.1 and 3000 km"))
		return
	}

	eventID := optionalText(req.EventID)
	if id, found, err := h.repo.FindIngestedDrive(r.Context(), vehicle.ID, eventID, *start, distance); err != nil {
		writeRepoError(w, r, err, "Failed to check duplicate drives")
		return
	} else if found {
		writeJSON(w, http.StatusOK, map[string]any{"status": "duplicate", "drive_id": id})
		return
	}

	var end *time.Time
	if req.Data.EndTime != nil {
		e := req.Data.EndTime.UTC()
		end = &e
	}
	timings := ingest.NormalizeDrive(ingest.DriveInput{
		Start: *start, End: end, DurationMin: req.Data.DurationMin, DistanceKm: distance,
		EnergyKwh: req.Data.EnergyKwh, VehicleKwh100km: vehicle.EstimatedKwh100km,
	})
	startOdo, endOdo := ingest.CompleteOdometers(scaled(req.Data.StartOdometer, factor), scaled(req.Data.EndOdometer, factor), distance)

	drive := &models.Drive{
		VehicleID:           vehicle.ID,
		Origin:              "WEBHOOK",
		ExternalID:          eventID,
		StartTime:           *start,
		EndTime:             timings.End,
		StartOdometer:       startOdo,
		EndOdometer:         endOdo,
		DistanceKm:          distance,
		DurationMin:         timings.DurationMin,
		StartAddress:        optionalText(req.Data.StartAddress),
		EndAddress:          optionalText(req.Data.EndAddress),
		EnergyConsumedKwh:   &timings.EnergyKwh,
		ConsumptionKwh100km: &timings.Kwh100km,
		Tags:                []string{},
		IsManual:            true,
		EnergyEstimated:     timings.EnergyEstimated,
	}
	if err := h.repo.CreateManualDrive(r.Context(), drive); err != nil {
		writeRepoError(w, r, err, "Failed to store drive")
		return
	}
	if endOdo != nil && *endOdo > vehicle.CurrentOdometer {
		_ = h.repo.UpdateVehicleOdometer(r.Context(), vehicle.ID, *endOdo)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "recorded", "vehicle_id": vehicle.ID, "drive_id": drive.ID})
}

// recordFuel stores a fill-up of a combustion vehicle. A fill-up of the same amount within 30 minutes is a duplicate.
func (h *HomeAssistantHandler) recordFuel(w http.ResponseWriter, r *http.Request, req *HAEventPayload) {
	vehicle := h.namedVehicle(w, r, req)
	if vehicle == nil {
		return
	}
	if vehicle.Powertrain != models.PowertrainICE {
		writeAPIError(w, http.StatusBadRequest, apierror.New("fuel.combustion_only", "Fuel fill-ups only apply to combustion vehicles"))
		return
	}
	factor, ok := distanceFactor(w, req.DistanceUnit)
	if !ok {
		return
	}
	when := eventStart(req)
	if when == nil {
		now := time.Now().UTC()
		when = &now
	}

	var amount *money.Cents
	switch {
	case req.Data.Amount != nil:
		c := money.FromFloat(*req.Data.Amount)
		amount = &c
	case req.Data.Cost != nil:
		c := money.FromFloat(*req.Data.Cost)
		amount = &c
	}
	fuel, err := buildFuelLog(vehicle.ID, &SaveFuelLogRequest{
		Date:          when.Format(time.RFC3339),
		Odometer:      scaled(eventOdometerKm(&req.Data, factor), 1),
		Amount:        amount,
		Liters:        req.Data.Liters,
		PricePerLiter: req.Data.PricePerLiter,
		FuelType:      req.Data.FuelType,
		IsFullTank:    req.Data.IsFullTank,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if dup, err := h.repo.HasDuplicateFuelLog(r.Context(), vehicle.ID, fuel.Date, fuel.Amount); err != nil {
		writeRepoError(w, r, err, "Failed to check duplicate fill-ups")
		return
	} else if dup {
		writeJSON(w, http.StatusOK, map[string]any{"status": "duplicate"})
		return
	}
	if fuel.Odometer != nil {
		points, err := h.repo.ListManualOdometerPoints(r.Context(), vehicle.ID)
		if err == nil {
			err = checkOdometerOrder(points, "", fuel.Date, *fuel.Odometer)
		}
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
	}
	if err := h.repo.CreateFuelLog(r.Context(), fuel); err != nil {
		writeRepoError(w, r, err, "Failed to store fill-up")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "recorded", "vehicle_id": vehicle.ID, "fuel_log_id": fuel.ID})
}
