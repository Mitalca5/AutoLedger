package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type VehicleHandler struct {
	repo        *database.Repository
	encryptor   *crypto.Encryptor
	syncService *services.SyncService
}

func NewVehicleHandler(repo *database.Repository, encryptor *crypto.Encryptor, syncService *services.SyncService) *VehicleHandler {
	return &VehicleHandler{
		repo:        repo,
		encryptor:   encryptor,
		syncService: syncService,
	}
}

type SaveVehicleRequest struct {
	Name                 string          `json:"name"`
	Vin                  *string         `json:"vin"`
	TeslaMateCarID       *int            `json:"teslamate_car_id"`
	CurrentOdometer      float64         `json:"current_odometer"`
	TeslaMateAPIURL      *string         `json:"teslamate_api_url"`
	TeslaMateAuthType    models.AuthMode `json:"teslamate_auth_type"`
	TeslaMateAPIKey      *string         `json:"teslamate_api_key"` // Plain text from frontend
	TeslaMateBasicUser   *string         `json:"teslamate_basic_user"`
	TeslaMateBasicPass   *string         `json:"teslamate_basic_pass"` // Plain text from frontend
	EstimatedKwh100km    *float64        `json:"estimated_kwh_100km"`
	EstimatedPricePerKwh *float64        `json:"estimated_price_per_kwh"`
	Currency             string          `json:"currency"`              // ISO 4217 code, fixed at creation; ignored on update
	Powertrain           string          `json:"powertrain"`            // EV (default) or ICE
	TelemetryMode        string          `json:"telemetry_mode"`        // CONNECTED, SEMI_AUTO, MANUAL
	Make                 string          `json:"make"`                  // Tesla, Renault, etc.
	Model                string          `json:"model"`                 // Model 3, Megane, etc.
	TeslaMateGrafanaURL  *string         `json:"teslamate_grafana_url"` // Optional; empty clears it
}

// normalizeGrafanaURL validates the base URL of the Grafana serving the TeslaMate dashboards.
// It returns nil for an empty value, and the URL without trailing slash otherwise.
func normalizeGrafanaURL(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil, nil
	}
	if len(value) > 300 {
		return nil, apierror.New("vehicle.grafana_url_too_long", "Grafana URL too long (300 characters maximum)")
	}
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, apierror.New("vehicle.grafana_url_invalid", "Invalid Grafana URL (http:// or https:// required)")
	}
	if u.User != nil {
		return nil, apierror.New("vehicle.grafana_url_credentials", "The Grafana URL must not contain credentials")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, apierror.New("vehicle.grafana_url_base", "The Grafana URL must be a base address, without parameters")
	}
	out := strings.TrimRight(u.String(), "/")
	return &out, nil
}

// normalizeVehicleCurrency validates and uppercases a vehicle's base currency, defaulting to EUR.
// Unlike normalizeCurrency (an individual expense's currency, checked against the vehicle's own),
// this is the vehicle's own currency: fixed at creation, never read again on update.
func normalizeVehicleCurrency(raw string) (string, error) {
	cur := strings.ToUpper(strings.TrimSpace(raw))
	if cur == "" {
		cur = "EUR"
	}
	if !currencyPattern.MatchString(cur) {
		return "", apierror.Newf("vehicle.currency_invalid", "Invalid currency: %q", raw)
	}
	return cur, nil
}

// validatePowertrain checks the powertrain of a vehicle payload and that an ICE vehicle has no TeslaMate link.
// An empty value is accepted and left to the caller's default.
func validatePowertrain(powertrain string, teslamateURL *string) error {
	switch powertrain {
	case "", models.PowertrainEV:
		return nil
	case models.PowertrainICE:
		if teslamateURL != nil && strings.TrimSpace(*teslamateURL) != "" {
			return apierror.New("vehicle.ice_no_teslamate", "A combustion vehicle cannot be linked to TeslaMate")
		}
		return nil
	default:
		return apierror.New("vehicle.powertrain_invalid", "Invalid powertrain (EV or ICE)")
	}
}

func (h *VehicleHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	list, err := h.repo.ListVehiclesByUserID(r.Context(), userID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apierror.New("internal", "Failed to list vehicles"))
		return
	}
	if list == nil {
		list = []models.Vehicle{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *VehicleHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req SaveVehicleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if req.Name == "" {
		writeAPIError(w, http.StatusBadRequest, apierror.New("vehicle.name_required", "Vehicle name is required"))
		return
	}
	if err := validatePowertrain(req.Powertrain, req.TeslaMateAPIURL); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	powertrain := req.Powertrain
	if powertrain == "" {
		powertrain = models.PowertrainEV
	}
	currency, err := normalizeVehicleCurrency(req.Currency)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	grafanaURL, err := normalizeGrafanaURL(req.TeslaMateGrafanaURL)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	var encKey, encPass *string
	if req.TeslaMateAPIKey != nil && *req.TeslaMateAPIKey != "" {
		encrypted, err := h.encryptor.Encrypt(*req.TeslaMateAPIKey)
		if err == nil {
			encKey = &encrypted
		}
	}
	if req.TeslaMateBasicPass != nil && *req.TeslaMateBasicPass != "" {
		encrypted, err := h.encryptor.Encrypt(*req.TeslaMateBasicPass)
		if err == nil {
			encPass = &encrypted
		}
	}

	authType := req.TeslaMateAuthType
	if authType == "" {
		authType = models.AuthModeNone
	}

	telemetryMode := req.TelemetryMode
	if telemetryMode == "" {
		if req.TeslaMateCarID != nil || (req.TeslaMateAPIURL != nil && *req.TeslaMateAPIURL != "") {
			telemetryMode = models.TelemetryConnected
		} else {
			telemetryMode = models.TelemetryManual
		}
	}

	makeName := strings.TrimSpace(req.Make)
	if makeName == "" {
		if telemetryMode == models.TelemetryConnected {
			makeName = "Tesla"
		} else {
			makeName = "Generic"
		}
	}

	v := &models.Vehicle{
		UserID:                   userID,
		Name:                     req.Name,
		Vin:                      req.Vin,
		TeslaMateCarID:           req.TeslaMateCarID,
		CurrentOdometer:          req.CurrentOdometer,
		TeslaMateAPIURL:          req.TeslaMateAPIURL,
		TeslaMateAuthType:        authType,
		TeslaMateAPIKeyEncrypted: encKey,
		TeslaMateBasicUser:       req.TeslaMateBasicUser,
		TeslaMateBasicPassEnc:    encPass,
		EstimatedKwh100km:        req.EstimatedKwh100km,
		EstimatedPricePerKwh:     req.EstimatedPricePerKwh,
		Currency:                 currency,
		Powertrain:               powertrain,
		TelemetryMode:            telemetryMode,
		Make:                     makeName,
		Model:                    strings.TrimSpace(req.Model),
		TeslaMateGrafanaURL:      grafanaURL,
	}

	if err := h.repo.CreateVehicle(r.Context(), v); err != nil {
		writeRepoError(w, r, err, "Failed to create vehicle")
		return
	}

	writeJSON(w, http.StatusCreated, v)
}

func (h *VehicleHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}

	writeJSON(w, http.StatusOK, v)
}

func (h *VehicleHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	existing, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}
	if existing.Role != models.RoleOwner {
		writeAPIError(w, http.StatusForbidden, apierror.New("access.owner_only_configure", "Only the owner can change the vehicle configuration"))
		return
	}

	var req SaveVehicleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if req.Powertrain != "" {
		if err := validatePowertrain(req.Powertrain, req.TeslaMateAPIURL); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		existing.Powertrain = req.Powertrain
	} else if err := validatePowertrain(existing.Powertrain, req.TeslaMateAPIURL); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	if req.TeslaMateGrafanaURL != nil {
		grafanaURL, err := normalizeGrafanaURL(req.TeslaMateGrafanaURL)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		existing.TeslaMateGrafanaURL = grafanaURL
	}

	existing.Name = req.Name
	existing.Vin = req.Vin
	existing.TeslaMateCarID = req.TeslaMateCarID
	if req.CurrentOdometer > 0 {
		existing.CurrentOdometer = req.CurrentOdometer
	}
	existing.TeslaMateAPIURL = req.TeslaMateAPIURL
	if req.TeslaMateAuthType != "" {
		existing.TeslaMateAuthType = req.TeslaMateAuthType
	}

	if req.TeslaMateAPIKey != nil && *req.TeslaMateAPIKey != "" {
		encrypted, err := h.encryptor.Encrypt(*req.TeslaMateAPIKey)
		if err == nil {
			existing.TeslaMateAPIKeyEncrypted = &encrypted
		}
	}
	if req.TeslaMateBasicUser != nil {
		existing.TeslaMateBasicUser = req.TeslaMateBasicUser
	}
	if req.TeslaMateBasicPass != nil && *req.TeslaMateBasicPass != "" {
		encrypted, err := h.encryptor.Encrypt(*req.TeslaMateBasicPass)
		if err == nil {
			existing.TeslaMateBasicPassEnc = &encrypted
		}
	}
	if req.EstimatedKwh100km != nil {
		existing.EstimatedKwh100km = req.EstimatedKwh100km
	}
	if req.EstimatedPricePerKwh != nil {
		existing.EstimatedPricePerKwh = req.EstimatedPricePerKwh
	}
	if req.TelemetryMode != "" {
		existing.TelemetryMode = req.TelemetryMode
	}
	if req.Make != "" {
		existing.Make = strings.TrimSpace(req.Make)
	}
	if req.Model != "" {
		existing.Model = strings.TrimSpace(req.Model)
	}

	if err := h.repo.UpdateVehicle(r.Context(), existing); err != nil {
		writeAPIError(w, http.StatusInternalServerError, apierror.New("internal", "Failed to update vehicle"))
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

type SaveEstimatedEnergyRequest struct {
	EstimatedKwh100km    *float64 `json:"estimated_kwh_100km"`
	EstimatedPricePerKwh *float64 `json:"estimated_price_per_kwh"`
}

func (h *VehicleHandler) UpdateEstimatedEnergy(w http.ResponseWriter, r *http.Request) {
	var req SaveEstimatedEnergyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if req.EstimatedKwh100km != nil && (*req.EstimatedKwh100km <= 0 || *req.EstimatedKwh100km > 100) {
		writeAPIError(w, http.StatusBadRequest, apierror.Newf("vehicle.estimate_consumption_range", "The average consumption must be between 0 and %.0f kWh/100km", apierror.PerKm(100)))
		return
	}
	if req.EstimatedPricePerKwh != nil && (*req.EstimatedPricePerKwh <= 0 || *req.EstimatedPricePerKwh > 10) {
		writeAPIError(w, http.StatusBadRequest, apierror.New("vehicle.estimate_rate_range", "The electricity rate must be between 0 and 10 per kWh"))
		return
	}

	if h.repo == nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}

	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")
	if vehicleID == "" {
		vehicleID = chi.URLParam(r, "vehicleId")
	}

	vCheck, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}
	if !vCheck.Role.CanEdit() {
		writeAPIError(w, http.StatusForbidden, apierror.New("access.insufficient_rights", "Insufficient rights to change this setting"))
		return
	}

	if err := h.repo.UpdateVehicleEstimatedEnergy(r.Context(), vehicleID, userID, req.EstimatedKwh100km, req.EstimatedPricePerKwh); err != nil {
		writeRepoError(w, r, err, "Failed to update estimated energy")
		return
	}

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to get vehicle")
		return
	}

	writeJSON(w, http.StatusOK, v)
}

func (h *VehicleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}
	if v.Role != models.RoleOwner {
		writeAPIError(w, http.StatusForbidden, apierror.New("access.owner_only_delete", "Only the owner can delete the vehicle"))
		return
	}

	if err := h.repo.DeleteVehicle(r.Context(), vehicleID, userID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, apierror.New("internal", "Failed to delete vehicle"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Vehicle deleted successfully"})
}

type TestConnectionRequest struct {
	TeslaMateAPIURL    string          `json:"teslamate_api_url"`
	TeslaMateAuthType  models.AuthMode `json:"teslamate_auth_type"`
	TeslaMateAPIKey    *string         `json:"teslamate_api_key"`
	TeslaMateBasicUser *string         `json:"teslamate_basic_user"`
	TeslaMateBasicPass *string         `json:"teslamate_basic_pass"`
	TeslaMateCarID     *int            `json:"teslamate_car_id"`
}

func (h *VehicleHandler) TestTeslaMateRaw(w http.ResponseWriter, r *http.Request) {
	var req TestConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	apiKey := ""
	if req.TeslaMateAPIKey != nil {
		apiKey = *req.TeslaMateAPIKey
	}
	basicUser := ""
	if req.TeslaMateBasicUser != nil {
		basicUser = *req.TeslaMateBasicUser
	}
	basicPass := ""
	if req.TeslaMateBasicPass != nil {
		basicPass = *req.TeslaMateBasicPass
	}
	carID := 1
	if req.TeslaMateCarID != nil && *req.TeslaMateCarID > 0 {
		carID = *req.TeslaMateCarID
	}

	status, err := h.syncService.TestConnectionRaw(
		r.Context(),
		req.TeslaMateAPIURL,
		req.TeslaMateAuthType,
		apiKey,
		basicUser,
		basicPass,
		carID,
	)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"status":  status,
	})
}

func (h *VehicleHandler) TestTeslaMate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}
	if v.Role != models.RoleOwner {
		writeAPIError(w, http.StatusForbidden, apierror.New("access.owner_only_test_connection", "Only the owner can test the TeslaMate connection"))
		return
	}

	fullVehicle, err := h.repo.GetVehicleByIDInternal(r.Context(), vehicleID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apierror.New("internal", "Failed to load vehicle credentials"))
		return
	}

	status, err := h.syncService.TestConnection(r.Context(), fullVehicle)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"status":  status,
	})
}

func (h *VehicleHandler) Sync(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}
	if v.Role == models.RoleViewer {
		writeAPIError(w, http.StatusForbidden, apierror.New("access.viewer_cannot_sync", "Viewers cannot trigger a synchronization"))
		return
	}

	fullVehicle, err := h.repo.GetVehicleByIDInternal(r.Context(), vehicleID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apierror.New("internal", "Failed to load vehicle credentials"))
		return
	}

	job, started := h.syncService.StartSync(*fullVehicle)
	status := http.StatusAccepted
	if !started {
		status = http.StatusOK
	}
	writeJSON(w, status, job)
}

// GetSyncStatus returns the last synchronization job of the vehicle.
func (h *VehicleHandler) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}

	job := h.syncService.GetSyncJob(vehicleID)
	if job == nil {
		writeJSON(w, http.StatusOK, map[string]any{"vehicle_id": vehicleID, "status": "NONE"})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// GetOdometerAtDate returns the estimated or recorded odometer for the vehicle at a specific date.
func (h *VehicleHandler) GetOdometerAtDate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}

	dateStr := r.URL.Query().Get("date")
	targetTime := time.Now()
	if dateStr != "" {
		parsed, err := parseDate(dateStr)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_date", "Invalid date"))
			return
		}
		// If only date was provided (00:00:00), check drives up to end of that day
		if parsed.Hour() == 0 && parsed.Minute() == 0 && parsed.Second() == 0 {
			targetTime = parsed.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		} else {
			targetTime = parsed
		}
	}

	odo, source, err := h.repo.GetOdometerAtDate(r.Context(), vehicleID, targetTime)
	if err != nil {
		writeRepoError(w, r, err, "Failed to resolve odometer")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"odometer": odo,
		"source":   source,
	})
}
