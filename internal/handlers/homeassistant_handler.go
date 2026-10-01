package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
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
}

func NewHomeAssistantHandler(repo *database.Repository, tariffService *services.TariffService) *HomeAssistantHandler {
	return &HomeAssistantHandler{
		repo:          repo,
		tariffService: tariffService,
	}
}

type HAEventPayload struct {
	VehicleID *string      `json:"vehicle_id"`
	EventType string       `json:"event_type"`
	Source    string       `json:"source"`
	Timestamp *time.Time   `json:"timestamp"`
	Data      HAChargeData `json:"data"`
}

type HAChargeData struct {
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
	userID := middleware.GetUserID(r.Context())

	var req HAEventPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	switch strings.TrimSpace(req.EventType) {
	case "", haEventChargingSessionEnd:
		// A charging session, handled below
	case haEventOdometerUpdate, haEventTelemetryUpdate:
		h.recordOdometer(w, r, &req)
		return
	default:
		writeAPIError(w, http.StatusBadRequest, apierror.Newf("integration.unknown_event_type", "Unknown event type %q", req.EventType))
		return
	}

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

	energyKwh := 0.0
	if req.Data.EnergyKwh != nil {
		energyKwh = *req.Data.EnergyKwh
	} else if req.Data.EnergyAddedKwh != nil {
		energyKwh = *req.Data.EnergyAddedKwh
	}

	location := "home"
	if req.Data.Location != nil && strings.TrimSpace(*req.Data.Location) != "" {
		location = strings.TrimSpace(*req.Data.Location)
	}

	var targetVehicle *models.Vehicle

	// Resolve target vehicle
	if req.VehicleID != nil && strings.TrimSpace(*req.VehicleID) != "" {
		targetVehicle = requireVehicleAccess(w, r, h.repo, strings.TrimSpace(*req.VehicleID), models.RoleEditor)
		if targetVehicle == nil {
			return
		}
	} else {
		// Vehicle not provided; attempt automatic single-EV or default assignment
		vehicles, err := h.repo.ListVehiclesByUserID(r.Context(), userID)
		if err != nil {
			writeRepoError(w, r, err, "Failed to inspect user vehicles")
			return
		}

		var defaultHomeVeh *models.Vehicle
		var evVehicles []models.Vehicle
		for i := range vehicles {
			v := vehicles[i]
			if v.IsHomeChargerDefault {
				defaultHomeVeh = &v
			}
			pt := strings.ToUpper(v.Powertrain)
			if pt == "BEV" || pt == "PHEV" || pt == "" {
				evVehicles = append(evVehicles, v)
			}
		}

		if defaultHomeVeh != nil {
			targetVehicle = defaultHomeVeh
		} else if len(evVehicles) == 1 {
			targetVehicle = &evVehicles[0]
		}
	}

	// If vehicle is still unresolved, send to Pending Charges
	if targetVehicle == nil {
		rawMap := map[string]any{
			"raw_event": req,
		}
		pending := &models.PendingCharge{
			UserID:      userID,
			Source:      "homeassistant",
			ChargerName: req.Data.ChargerName,
			StartTime:   startTime,
			EndTime:     endTime,
			EnergyKwh:   energyKwh,
			Location:    location,
			RawData:     rawMap,
		}
		if err := h.repo.CreatePendingCharge(r.Context(), pending); err != nil {
			writeRepoError(w, r, err, "Failed to record pending charge")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"status":     "pending_qualification",
			"pending_id": pending.ID,
			"message":    "Charging session queued for vehicle assignment",
		})
		return
	}

	// Compute financial cost if not explicitly provided
	var cost *money.Cents
	if req.Data.Cost != nil {
		c := money.FromFloat(*req.Data.Cost)
		cost = &c
	} else if req.Data.CostCents != nil {
		c := money.Cents(*req.Data.CostCents)
		cost = &c
	} else {
		plan, _ := h.repo.GetVehicleTariffPlan(r.Context(), targetVehicle.ID)
		if plan != nil && energyKwh > 0 {
			computed, err := h.tariffService.CalculateSessionCost(plan, startTime, endTime, energyKwh)
			if err == nil && computed > 0 {
				cost = &computed
			}
		}
	}

	noteStr := "Home Assistant charging session"
	if req.Data.ChargerName != nil && strings.TrimSpace(*req.Data.ChargerName) != "" {
		noteStr += " from " + strings.TrimSpace(*req.Data.ChargerName)
	}

	charge := &models.ChargeLog{
		VehicleID:         targetVehicle.ID,
		Date:              startTime,
		EndDate:           &endTime,
		Address:           &location,
		KwhAdded:          energyKwh,
		Cost:              cost,
		CostSource:        "HOMEASSISTANT",
		Currency:          targetVehicle.Currency,
		Odometer:          req.Data.OdometerKm,
		StartBatteryLevel: req.Data.SocStart,
		EndBatteryLevel:   req.Data.SocEnd,
		IsManual:          true,
		Notes:             &noteStr,
	}

	if err := h.repo.CreateManualCharge(r.Context(), charge); err != nil {
		writeRepoError(w, r, err, "Failed to store charge log")
		return
	}

	// Update odometer if provided and advances vehicle mileage
	if req.Data.OdometerKm != nil && *req.Data.OdometerKm > 0 {
		if *req.Data.OdometerKm > targetVehicle.CurrentOdometer {
			_ = h.repo.UpdateVehicleOdometer(r.Context(), targetVehicle.ID, *req.Data.OdometerKm)
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":     "recorded",
		"vehicle_id": targetVehicle.ID,
		"charge_id":  charge.ID,
		"cost":       charge.Cost,
	})
}

// Event types accepted by HandleEvent. An empty type is a charging session (the first blueprint sent none).
const (
	haEventChargingSessionEnd = "charging_session_end"
	haEventOdometerUpdate     = "odometer_update"
	haEventTelemetryUpdate    = "telemetry_update"
)

// recordOdometer applies an odometer reading. Unlike a charging session it is never attributed by guess:
// the event must name its vehicle, since a reading sent to the wrong vehicle would overwrite its mileage.
func (h *HomeAssistantHandler) recordOdometer(w http.ResponseWriter, r *http.Request, req *HAEventPayload) {
	if req.VehicleID == nil || strings.TrimSpace(*req.VehicleID) == "" {
		writeAPIError(w, http.StatusBadRequest, apierror.New("vehicle.not_specified", "The event must name its vehicle"))
		return
	}
	if req.Data.OdometerKm == nil || *req.Data.OdometerKm <= 0 {
		writeAPIError(w, http.StatusBadRequest, apierror.New("telemetry.missing_odometer", "No valid odometer reading provided"))
		return
	}
	vehicle := requireVehicleAccess(w, r, h.repo, strings.TrimSpace(*req.VehicleID), models.RoleEditor)
	if vehicle == nil {
		return
	}
	odometer := *req.Data.OdometerKm
	if odometer > vehicle.CurrentOdometer {
		if err := h.repo.UpdateVehicleOdometer(r.Context(), vehicle.ID, odometer); err != nil {
			writeRepoError(w, r, err, "Failed to update vehicle odometer")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "recorded",
		"vehicle_id":       vehicle.ID,
		"current_odometer": math.Max(vehicle.CurrentOdometer, odometer),
	})
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

const defaultBlueprintYAML = `blueprint:
  name: AutoLedger - Charging Session Sync
  description: >
    Automatically synchronizes completed EV charging sessions to AutoLedger.
    Triggers when your charger transitions from charging to idle (or when session energy stops increasing).
  domain: automation
  input:
    charger_status_entity:
      name: Charger State Entity
      description: The sensor or binary sensor indicating charger state (e.g. charging/idle or on/off).
      selector:
        entity:
          domain: [sensor, binary_sensor]
    energy_sensor_entity:
      name: Energy Meter Entity
      description: Sensor measuring session energy added or cumulative kWh.
      selector:
        entity:
          domain: sensor
          device_class: energy
    autoledger_url:
      name: AutoLedger URL
      description: The base URL of your AutoLedger instance (e.g. https://autoledger.example.com).
      default: "http://localhost:8080"
      selector:
        text:
    autoledger_api_token:
      name: AutoLedger API Token
      description: Secret API Token generated in AutoLedger (Account -> API Tokens).
      selector:
        text:
    vehicle_id:
      name: Target Vehicle ID (Optional)
      description: >
        AutoLedger Vehicle ID. Leave empty if you have 1 vehicle or want sessions sent to 'Recharges à qualifier'.
      default: ""
      selector:
        text:
    charger_name:
      name: Charger Friendly Name
      description: Name of the charging station (e.g. Wallbox Pulsar, Easee Home).
      default: "Home Charger"
      selector:
        text:
    location:
      name: Location
      description: Charging location tag.
      default: "home"
      selector:
        text:

mode: restart
max_exceeded: silent

trigger:
  - platform: state
    entity_id: !input charger_status_entity
    from:
      - "on"
      - "charging"
      - "active"
    for:
      minutes: 2

variables:
  server_url: !input autoledger_url
  api_token: !input autoledger_api_token
  target_vehicle: !input vehicle_id
  station_name: !input charger_name
  loc: !input location
  energy_entity: !input energy_sensor_entity

action:
  - choose:
      - conditions:
          - condition: template
            value_template: >
              {{ states(energy_entity) not in ['unknown', 'unavailable', 'none', ''] and (states(energy_entity) | float(0)) > 0 }}
        sequence:
          - action: uri
            data:
              url: "{{ server_url.rstrip('/') }}/api/integrations/homeassistant/event"
              method: POST
              headers:
                Authorization: "Bearer {{ api_token }}"
                Content-Type: "application/json"
              body: >
                {
                  "vehicle_id": {% if target_vehicle | trim != "" %}"{{ target_vehicle | trim }}"{% else %}null{% endif %},
                  "event_type": "charging_session_end",
                  "source": "homeassistant_blueprint",
                  "data": {
                    "charger_name": "{{ station_name }}",
                    "energy_kwh": {{ states(energy_entity) | float(0) }},
                    "location": "{{ loc }}",
                    "end_time": "{{ now().isoformat() }}"
                  }
                }
`

// GetBlueprint serves the Home Assistant automation Blueprint.
func (h *HomeAssistantHandler) GetBlueprint(w http.ResponseWriter, r *http.Request) {
	blueprintPath := "integrations/homeassistant/blueprints/autoledger_charging_session.yaml"
	content, err := os.ReadFile(blueprintPath)
	if err != nil {
		content = []byte(defaultBlueprintYAML)
	}

	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}
