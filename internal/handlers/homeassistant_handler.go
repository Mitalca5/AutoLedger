package handlers

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

type HomeAssistantHandler struct {
	repo          *database.Repository
	tariffService *services.TariffService
	loc           *time.Location
}

// SetTimezone sets the timezone that decides which day an odometer reading belongs to (UTC by default).
func (h *HomeAssistantHandler) SetTimezone(name string) {
	if loc, err := time.LoadLocation(name); err == nil {
		h.loc = loc
	}
}

func NewHomeAssistantHandler(repo *database.Repository, tariffService *services.TariffService) *HomeAssistantHandler {
	return &HomeAssistantHandler{
		repo:          repo,
		tariffService: tariffService,
		loc:           time.UTC,
	}
}

type HAEventPayload struct {
	// EventID identifies the event for the sender: an event sent again with the same ID is not recorded twice.
	EventID   *string    `json:"event_id"`
	VehicleID *string    `json:"vehicle_id"`
	EventType string     `json:"event_type"`
	Source    string     `json:"source"`
	Timestamp *time.Time `json:"timestamp"`
	// DistanceUnit is the unit of the distances and odometer readings of the event ("km", the default, or "mi").
	DistanceUnit string      `json:"distance_unit"`
	Data         HAEventData `json:"data"`
}

type HAEventData struct {
	StartTime      *time.Time `json:"start_time"`
	EndTime        *time.Time `json:"end_time"`
	EnergyKwh      *float64   `json:"energy_kwh"`
	EnergyAddedKwh *float64   `json:"energy_added_kwh"`
	ChargerName    *string    `json:"charger_name"`
	Location       *string    `json:"location"`
	Cost           *float64   `json:"cost"`
	CostCents      *int64     `json:"cost_cents"`
	OdometerKm     *float64   `json:"odometer_km"`
	SocStart       *int       `json:"soc_start"`
	SocEnd         *int       `json:"soc_end"`

	// Odometer readings, drives and fill-ups. Distances are in the event's distance_unit, except odometer_km.
	Odometer      *float64 `json:"odometer"`
	Distance      *float64 `json:"distance"`
	StartOdometer *float64 `json:"start_odometer"`
	EndOdometer   *float64 `json:"end_odometer"`
	DurationMin   *int     `json:"duration_min"`
	StartAddress  *string  `json:"start_address"`
	EndAddress    *string  `json:"end_address"`
	Amount        *float64 `json:"amount"`
	Liters        *float64 `json:"liters"`
	PricePerLiter *float64 `json:"price_per_liter"`
	FuelType      *string  `json:"fuel_type"`
	IsFullTank    *bool    `json:"is_full_tank"`
}

type VehicleMetricsResponse struct {
	LastChargeCost  *float64 `json:"last_charge_cost"`
	EnergyKwh       *float64 `json:"energy_kwh"`
	DurationMinutes *int     `json:"duration_minutes"`
	Date            *string  `json:"date"`
	Currency        string   `json:"currency"`
	CostPer100Km    *float64 `json:"cost_per_100km"`
}

// HandleEvent ingests charging sessions or telemetry updates from Home Assistant webhooks / custom component.
func (h *HomeAssistantHandler) HandleEvent(w http.ResponseWriter, r *http.Request) {
	var req HAEventPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	switch strings.TrimSpace(req.EventType) {
	case "", haEventChargingSessionEnd:
		// A charging session, handled below
	case haEventOdometer, haEventOdometerUpdate, haEventTelemetryUpdate:
		h.recordOdometer(w, r, &req)
		return
	case haEventDrive:
		h.recordDrive(w, r, &req)
		return
	case haEventFuel:
		h.recordFuel(w, r, &req)
		return
	default:
		writeAPIError(w, http.StatusBadRequest, apierror.Newf("integration.unknown_event_type", "Unknown event type %q", req.EventType))
		return
	}

	h.recordChargingSession(w, r, &req)
}

// maxSessionKwh bounds the energy of one session: a larger value is a cumulative meter reading, not a session.
const maxSessionKwh = 500.0

// recordChargingSession stores a charging session for its vehicle, or as a pending charge when the vehicle cannot
// be told (a charger shared by several vehicles). A session already received is answered as a duplicate.
func (h *HomeAssistantHandler) recordChargingSession(w http.ResponseWriter, r *http.Request, req *HAEventPayload) {
	userID := middleware.GetUserID(r.Context())

	endTime := time.Now().UTC()
	if req.Data.EndTime != nil {
		endTime = req.Data.EndTime.UTC()
	} else if req.Timestamp != nil {
		endTime = req.Timestamp.UTC()
	}
	startTime := endTime
	if req.Data.StartTime != nil {
		startTime = req.Data.StartTime.UTC()
	}
	if startTime.After(endTime) {
		writeAPIError(w, http.StatusBadRequest, apierror.New("charge.invalid_period", "The session starts after it ends"))
		return
	}

	energyKwh := 0.0
	if req.Data.EnergyKwh != nil {
		energyKwh = *req.Data.EnergyKwh
	} else if req.Data.EnergyAddedKwh != nil {
		energyKwh = *req.Data.EnergyAddedKwh
	}
	if energyKwh <= 0 || energyKwh > maxSessionKwh {
		writeAPIError(w, http.StatusBadRequest, apierror.Newf("charge.invalid_energy", "The session energy must be above 0 and at most %g kWh", maxSessionKwh))
		return
	}

	location := optionalText(req.Data.Location)
	eventID := optionalText(req.EventID)

	vehicles, err := h.repo.ListVehiclesByUserID(r.Context(), userID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to inspect user vehicles")
		return
	}
	var target *models.Vehicle
	if requested := optionalText(req.VehicleID); requested != nil {
		if target = requireVehicleAccess(w, r, h.repo, *requested, models.RoleEditor); target == nil {
			return
		}
		if target.Powertrain == models.PowertrainICE {
			writeAPIError(w, http.StatusBadRequest, apierror.New("charge.electric_only", "Charging sessions only apply to electric vehicles"))
			return
		}
	} else {
		target = chargingSessionVehicle(vehicles)
	}

	// A session already received: recorded on any of the account's vehicles, or still pending
	vehicleIDs := make([]string, 0, len(vehicles))
	for _, v := range vehicles {
		vehicleIDs = append(vehicleIDs, v.ID)
	}
	if id, found, err := h.repo.FindIngestedCharge(r.Context(), vehicleIDs, eventID, startTime, energyKwh); err != nil {
		writeRepoError(w, r, err, "Failed to check duplicate charges")
		return
	} else if found {
		writeJSON(w, http.StatusOK, map[string]any{"status": "duplicate", "charge_id": id})
		return
	}
	if id, found, err := h.repo.FindPendingCharge(r.Context(), userID, eventID, startTime, energyKwh); err != nil {
		writeRepoError(w, r, err, "Failed to check duplicate pending charges")
		return
	} else if found {
		writeJSON(w, http.StatusOK, map[string]any{"status": "duplicate", "pending_id": id})
		return
	}

	if target == nil {
		pending := &models.PendingCharge{
			UserID:      userID,
			Source:      "homeassistant",
			ChargerName: optionalText(req.Data.ChargerName),
			StartTime:   startTime,
			EndTime:     endTime,
			EnergyKwh:   energyKwh,
			Location:    location,
			RawData:     map[string]any{"raw_event": req},
			ExternalID:  eventID,
		}
		if err := h.repo.CreatePendingCharge(r.Context(), pending); err != nil {
			writeRepoError(w, r, err, "Failed to record pending charge")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"status": "pending_qualification", "pending_id": pending.ID})
		return
	}

	// Cost: the one sent, otherwise the vehicle's tariff plan, otherwise left to complete
	var cost *money.Cents
	if req.Data.Cost != nil {
		c := money.FromFloat(*req.Data.Cost)
		cost = &c
	} else if req.Data.CostCents != nil {
		c := money.Cents(*req.Data.CostCents)
		cost = &c
	} else if plan, _ := h.repo.GetVehicleTariffPlan(r.Context(), target.ID); plan != nil {
		if computed, err := h.tariffService.CalculateSessionCost(plan, startTime, endTime, energyKwh); err == nil && computed > 0 {
			cost = &computed
		}
	}

	charge := &models.ChargeLog{
		VehicleID:         target.ID,
		Date:              startTime,
		EndDate:           &endTime,
		Address:           location,
		KwhAdded:          energyKwh,
		Cost:              cost,
		CostSource:        "HOMEASSISTANT",
		Currency:          target.Currency,
		Odometer:          req.Data.OdometerKm,
		StartBatteryLevel: req.Data.SocStart,
		EndBatteryLevel:   req.Data.SocEnd,
		ExternalID:        eventID,
	}
	if err := h.repo.CreateIngestedCharge(r.Context(), charge); err != nil {
		writeRepoError(w, r, err, "Failed to store charge log")
		return
	}
	if req.Data.OdometerKm != nil && *req.Data.OdometerKm > target.CurrentOdometer {
		_ = h.repo.UpdateVehicleOdometer(r.Context(), target.ID, *req.Data.OdometerKm)
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":     "recorded",
		"vehicle_id": target.ID,
		"charge_id":  charge.ID,
		"cost":       charge.Cost,
	})
}

// chargingSessionVehicle guesses the vehicle of a session sent without one, among the vehicles the account can
// edit: the one marked as the home charger's default, otherwise the only electric vehicle. Nil when it cannot tell.
func chargingSessionVehicle(vehicles []models.Vehicle) *models.Vehicle {
	var electric []*models.Vehicle
	for i := range vehicles {
		v := &vehicles[i]
		if v.Role != models.RoleOwner && v.Role != models.RoleEditor {
			continue
		}
		if v.IsHomeChargerDefault {
			return v
		}
		if v.Powertrain != models.PowertrainICE {
			electric = append(electric, v)
		}
	}
	if len(electric) == 1 {
		return electric[0]
	}
	return nil
}

// optionalText is the trimmed value of an optional text, nil when absent or blank.
func optionalText(s *string) *string {
	if s == nil {
		return nil
	}
	if t := strings.TrimSpace(*s); t != "" {
		return &t
	}
	return nil
}

// Event types accepted by HandleEvent. An empty type is a charging session (the first blueprint sent none).
const (
	haEventChargingSessionEnd = "charging_session_end"
	haEventOdometer           = "odometer"
	haEventDrive              = "drive"
	haEventFuel               = "fuel"
	// Older names of the odometer event, still sent by existing automations.
	haEventOdometerUpdate  = "odometer_update"
	haEventTelemetryUpdate = "telemetry_update"
)

// recordOdometer applies an odometer reading. Unlike a charging session it is never attributed by guess:
// the event must name its vehicle, since a reading sent to the wrong vehicle would overwrite its mileage.
func (h *HomeAssistantHandler) recordOdometer(w http.ResponseWriter, r *http.Request, req *HAEventPayload) {
	if req.VehicleID == nil || strings.TrimSpace(*req.VehicleID) == "" {
		writeAPIError(w, http.StatusBadRequest, apierror.New("vehicle.not_specified", "The event must name its vehicle"))
		return
	}
	factor, ok := distanceFactor(w, req.DistanceUnit)
	if !ok {
		return
	}
	reading := eventOdometerKm(&req.Data, factor)
	if reading == nil || *reading <= 0 || *reading > maxEventOdometerKm {
		writeAPIError(w, http.StatusBadRequest, apierror.New("telemetry.missing_odometer", "No valid odometer reading provided"))
		return
	}
	vehicle := requireVehicleAccess(w, r, h.repo, strings.TrimSpace(*req.VehicleID), models.RoleEditor)
	if vehicle == nil {
		return
	}
	odometer := *reading
	if odometer > vehicle.CurrentOdometer {
		if err := h.repo.UpdateVehicleOdometer(r.Context(), vehicle.ID, odometer); err != nil {
			writeRepoError(w, r, err, "Failed to update vehicle odometer")
			return
		}
	}
	h.recordOdometerPoint(r, vehicle.ID, req.Timestamp, odometer)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "recorded",
		"vehicle_id":       vehicle.ID,
		"current_odometer": math.Max(vehicle.CurrentOdometer, odometer),
	})
}

// recordOdometerPoint keeps the reading in the vehicle's mileage history. A reading that contradicts the readings
// and fill-ups around it still raises the current odometer but is not kept as a point: it would make the history
// inconsistent and block later manual entries.
func (h *HomeAssistantHandler) recordOdometerPoint(r *http.Request, vehicleID string, at *time.Time, odometer float64) {
	when := time.Now()
	if at != nil && !at.IsZero() && !at.After(when) {
		when = *at
	}
	local := when.In(h.loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)

	points, err := h.repo.ListManualOdometerPoints(r.Context(), vehicleID)
	if err == nil {
		err = checkOdometerOrder(points, "", day, odometer)
	}
	if err == nil {
		err = h.repo.RecordIntegrationOdometer(r.Context(), vehicleID, day, odometer)
	}
	if err != nil {
		slog.Warn("odometer reading not kept in the history", "vehicle_id", vehicleID, "error", err)
	}
}

// IntegrationVehicle is the view of a vehicle given to Home Assistant and scripts: what an integration needs to
// name its devices and entities, without the TeslaMate connection settings or any other account data.
type IntegrationVehicle struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Make            string  `json:"make"`
	Model           string  `json:"model"`
	Powertrain      string  `json:"powertrain"`
	Currency        string  `json:"currency"`
	CurrentOdometer float64 `json:"current_odometer"`
}

func integrationVehicle(v *models.Vehicle) IntegrationVehicle {
	return IntegrationVehicle{ID: v.ID, Name: v.Name, Make: v.Make, Model: v.Model, Powertrain: v.Powertrain,
		Currency: v.Currency, CurrentOdometer: v.CurrentOdometer}
}

// ListVehicles lists the vehicles the token's account can see, in the integration view.
func (h *HomeAssistantHandler) ListVehicles(w http.ResponseWriter, r *http.Request) {
	vehicles, err := h.repo.ListVehiclesByUserID(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		writeRepoError(w, r, err, "Failed to list vehicles")
		return
	}
	out := make([]IntegrationVehicle, 0, len(vehicles))
	for i := range vehicles {
		out = append(out, integrationVehicle(&vehicles[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// GetVehicle returns one vehicle in the integration view.
func (h *HomeAssistantHandler) GetVehicle(w http.ResponseWriter, r *http.Request) {
	vehicle := requireVehicleAccess(w, r, h.repo, chi.URLParam(r, "vehicleId"), models.RoleViewer)
	if vehicle == nil {
		return
	}
	writeJSON(w, http.StatusOK, integrationVehicle(vehicle))
}

// GetVehicleMetrics provides aggregated financial and efficiency indicators for Home Assistant sensors.
func (h *HomeAssistantHandler) GetVehicleMetrics(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if vehicleID == "" {
		vehicleID = chi.URLParam(r, "id")
	}

	vehicle := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer)
	if vehicle == nil {
		return
	}

	charges, _, err := h.repo.ListCharges(r.Context(), vehicleID, false, 1, 0)
	if err != nil {
		writeRepoError(w, r, err, "Failed to load vehicle charge history")
		return
	}

	resp := VehicleMetricsResponse{
		Currency: vehicle.Currency,
	}

	if len(charges) > 0 {
		latest := charges[0]
		resp.EnergyKwh = &latest.KwhAdded
		dateStr := latest.Date.Format(time.RFC3339)
		resp.Date = &dateStr

		if latest.EndDate != nil {
			mins := int(latest.EndDate.Sub(latest.Date).Minutes())
			if mins < 0 {
				mins = 0
			}
			resp.DurationMinutes = &mins
		}

		if latest.Cost != nil {
			f := latest.Cost.Float()
			resp.LastChargeCost = &f
		}
	}

	// Calculate cost per 100km
	fleetSummary, err := h.repo.GetFleetSummary(r.Context(), vehicle.UserID)
	if err == nil && fleetSummary != nil {
		for _, vm := range fleetSummary.Vehicles {
			if vm.VehicleID == vehicleID && vm.EnergyCostPer100Km > 0 {
				eff := vm.EnergyCostPer100Km
				resp.CostPer100Km = &eff
				break
			}
		}
	}

	// Fallback to estimated kWh / 100km * price/kWh if fleet calculation has no drives
	if resp.CostPer100Km == nil && vehicle.EstimatedKwh100km != nil && vehicle.EstimatedPricePerKwh != nil {
		val := math.Round((*vehicle.EstimatedKwh100km**vehicle.EstimatedPricePerKwh)*100) / 100
		resp.CostPer100Km = &val
	}

	writeJSON(w, http.StatusOK, resp)
}
