package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
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

// driveAddress is the address of a drive end: the one sent, else the coordinates as text.
func driveAddress(address *string, lat, lon *float64) *string {
	if a := optionalText(address); a != nil {
		return a
	}
	if lat == nil || lon == nil || *lat < -90 || *lat > 90 || *lon < -180 || *lon > 180 {
		return nil
	}
	text := fmt.Sprintf("%.5f, %.5f", *lat, *lon)
	return &text
}

// resolveDriveAddresses replaces, in the background, the coordinates stored as addresses with real addresses.
func (h *HomeAssistantHandler) resolveDriveAddresses(drive *models.Drive, data *HAEventData) {
	if h.geocoder == nil {
		return
	}
	type job struct {
		end         database.DriveEnd
		lat, lon    float64
		placeholder string
	}
	var jobs []job
	if optionalText(data.StartAddress) == nil && drive.StartAddress != nil {
		jobs = append(jobs, job{database.DriveEndDeparture, *data.StartLat, *data.StartLon, *drive.StartAddress})
	}
	if optionalText(data.EndAddress) == nil && drive.EndAddress != nil {
		jobs = append(jobs, job{database.DriveEndArrival, *data.EndLat, *data.EndLon, *drive.EndAddress})
	}
	if len(jobs) == 0 {
		return
	}
	driveID := drive.ID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		for _, j := range jobs {
			address, err := h.geocoder.Reverse(ctx, j.lat, j.lon)
			if err != nil {
				slog.Warn("reverse geocoding failed, the drive keeps its coordinates", "drive_id", driveID, "error", err)
				continue
			}
			if err := h.repo.ReplaceDriveAddress(ctx, driveID, j.end, j.placeholder, address); err != nil {
				slog.Warn("could not store the geocoded address", "drive_id", driveID, "error", err)
			}
		}
	}()
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
	startOdoKm := scaled(req.Data.StartOdometer, factor)
	endOdoKm := scaled(req.Data.EndOdometer, factor)
	distance, ok := ingest.DeriveDistance(scaled(req.Data.Distance, factor), startOdoKm, endOdoKm)
	if !ok {
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
	startOdo, endOdo := startOdoKm, endOdoKm
	if distance > 0 {
		startOdo, endOdo = ingest.CompleteOdometers(startOdoKm, endOdoKm, distance)
	}

	drive := &models.Drive{
		VehicleID:       vehicle.ID,
		Origin:          "WEBHOOK",
		ExternalID:      eventID,
		StartTime:       *start,
		EndTime:         timings.End,
		StartOdometer:   startOdo,
		EndOdometer:     endOdo,
		DistanceKm:      distance,
		DurationMin:     timings.DurationMin,
		StartAddress:    driveAddress(req.Data.StartAddress, req.Data.StartLat, req.Data.StartLon),
		EndAddress:      driveAddress(req.Data.EndAddress, req.Data.EndLat, req.Data.EndLon),
		Tags:            []string{},
		IsManual:        true,
		EnergyEstimated: timings.EnergyEstimated,
	}
	if timings.EnergyKnown {
		drive.EnergyConsumedKwh = &timings.EnergyKwh
	}
	if distance > 0 && timings.EnergyKnown {
		drive.ConsumptionKwh100km = &timings.Kwh100km
	}
	if err := h.repo.CreateManualDrive(r.Context(), drive); err != nil {
		writeRepoError(w, r, err, "Failed to store drive")
		return
	}
	h.resolveDriveAddresses(drive, &req.Data)
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
	if !vehicle.CanRefuel() {
		writeAPIError(w, http.StatusBadRequest, apierror.New("fuel.combustion_only", "Fuel fill-ups do not apply to electric vehicles"))
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
