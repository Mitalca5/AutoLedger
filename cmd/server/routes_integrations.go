package main

import (
	"github.com/go-chi/chi/v5"
)

// registerIntegrationRoutes registers the only routes an API token opens (Home Assistant, scripts).
func (h *apiHandlers) registerIntegrationRoutes(r chi.Router) {
	r.Post("/api/integrations/homeassistant/event", h.ha.HandleEvent)
	r.Get("/api/integrations/homeassistant/vehicles", h.ha.ListVehicles)
	r.Get("/api/integrations/homeassistant/vehicles/{vehicleId}", h.ha.GetVehicle)
	r.Get("/api/integrations/homeassistant/vehicles/{vehicleId}/metrics", h.ha.GetVehicleMetrics)
}
