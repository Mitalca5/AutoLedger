package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
)

// addressBackfillBatch bounds one run: the geocoding service is rate limited, so a larger history is resolved
// over several runs.
const addressBackfillBatch = 100

var coordinateAddress = regexp.MustCompile(`^(-?[0-9]{1,3}\.[0-9]+), (-?[0-9]{1,3}\.[0-9]+)$`)

// SetGeocoder turns on the resolution of drive addresses stored as coordinates.
func (h *DriveHandler) SetGeocoder(g Geocoder) { h.geocoder = g }

type addressBackfills struct {
	mu      sync.Mutex
	running map[string]bool
}

func (b *addressBackfills) start(vehicleID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.running == nil {
		b.running = map[string]bool{}
	}
	if b.running[vehicleID] {
		return false
	}
	b.running[vehicleID] = true
	return true
}

func (b *addressBackfills) done(vehicleID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.running, vehicleID)
}

type addressBackfillStatus struct {
	GeocodingEnabled bool `json:"geocoding_enabled"`
	Pending          int  `json:"pending"`
	Running          bool `json:"running"`
}

// AddressBackfillStatus tells whether the drives of a vehicle hold coordinates instead of addresses and whether
// the server can resolve them.
func (h *DriveHandler) AddressBackfillStatus(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer) == nil {
		return
	}
	pending, err := h.repo.CountCoordinateAddressDrives(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to count drives")
		return
	}
	h.backfills.mu.Lock()
	running := h.backfills.running[vehicleID]
	h.backfills.mu.Unlock()
	writeJSON(w, http.StatusOK, addressBackfillStatus{GeocodingEnabled: h.geocoder != nil, Pending: pending, Running: running})
}

// ResolveAddresses starts, in the background, the resolution of the drives stored with coordinates as addresses.
func (h *DriveHandler) ResolveAddresses(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor) == nil {
		return
	}
	if h.geocoder == nil {
		writeAPIError(w, http.StatusConflict, apierror.New("geocoding.disabled", "Reverse geocoding is not enabled on this server"))
		return
	}
	if !h.backfills.start(vehicleID) {
		writeAPIError(w, http.StatusConflict, apierror.New("geocoding.already_running", "An address resolution is already running for this vehicle"))
		return
	}
	drives, err := h.repo.ListCoordinateAddressDrives(r.Context(), vehicleID, addressBackfillBatch)
	if err != nil || len(drives) == 0 {
		h.backfills.done(vehicleID)
		if err != nil {
			writeRepoError(w, r, err, "Failed to list drives")
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"queued": 0})
		return
	}
	go h.resolveBackfill(vehicleID, drives)
	writeJSON(w, http.StatusAccepted, map[string]int{"queued": len(drives)})
}

func (h *DriveHandler) resolveBackfill(vehicleID string, drives []database.CoordinateDrive) {
	defer h.backfills.done(vehicleID)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	for _, d := range drives {
		h.resolveEnd(ctx, d.ID, database.DriveEndDeparture, d.StartAddress)
		h.resolveEnd(ctx, d.ID, database.DriveEndArrival, d.EndAddress)
		if ctx.Err() != nil {
			return
		}
	}
}

func (h *DriveHandler) resolveEnd(ctx context.Context, driveID string, end database.DriveEnd, stored *string) {
	if stored == nil {
		return
	}
	m := coordinateAddress.FindStringSubmatch(*stored)
	if m == nil {
		return
	}
	lat, lon, ok := parseCoordinates(m)
	if !ok {
		return
	}
	address, err := h.geocoder.Reverse(ctx, lat, lon)
	if err != nil {
		slog.Warn("reverse geocoding failed, the drive keeps its coordinates", "drive_id", driveID, "error", err)
		return
	}
	if err := h.repo.ReplaceDriveAddress(ctx, driveID, end, *stored, address); err != nil {
		slog.Warn("could not store the geocoded address", "drive_id", driveID, "error", err)
	}
}

func parseCoordinates(groups []string) (lat, lon float64, ok bool) {
	lat, errLat := strconv.ParseFloat(groups[1], 64)
	lon, errLon := strconv.ParseFloat(groups[2], 64)
	if errLat != nil || errLon != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, false
	}
	return lat, lon, true
}
