package main

import (
	"github.com/go-chi/chi/v5"
)

// registerVehicleRoutes registers the vehicle record, its sharing and its odometer checkpoints under /api/vehicles.
func (h *apiHandlers) registerVehicleRoutes(r chi.Router) {
	r.Get("/", h.vehicle.List)
	r.Post("/", h.vehicle.Create)
	r.Post("/test-connection", h.vehicle.TestTeslaMateRaw)
	r.Get("/{id}", h.vehicle.Get)
	r.Put("/{id}", h.vehicle.Update)
	r.Delete("/{id}", h.vehicle.Delete)
	r.Post("/{id}/teslamate/test", h.vehicle.TestTeslaMate)
	r.Post("/{id}/sync", h.vehicle.Sync)
	r.Get("/{id}/sync", h.vehicle.GetSyncStatus)
	r.Get("/{id}/ownership", h.vehicle.GetOwnership)
	r.Put("/{id}/ownership", h.vehicle.SaveOwnership)
	r.Delete("/{id}/ownership", h.vehicle.DeleteOwnership)
	r.Put("/{id}/estimated-energy", h.vehicle.UpdateEstimatedEnergy)
	r.Get("/{id}/odometer-at", h.vehicle.GetOdometerAtDate)
	r.Get("/{id}/odometer-estimate", h.vehicle.GetOdometerEstimate)
	r.Get("/{id}/data-sources", h.vehicle.GetDataSources)
	r.Get("/{vehicleId}/data-quality", h.tco.GetDataQuality)

	// Shared Vehicle Members
	r.Get("/{id}/members", h.vehicleMember.ListMembers)
	r.Post("/{id}/members", h.vehicleMember.AddMember)
	r.Put("/{id}/members/{memberId}", h.vehicleMember.UpdateMemberRole)
	r.Delete("/{id}/members/{memberId}", h.vehicleMember.RemoveMember)
	r.Get("/{id}/people", h.vehicleMember.ListPeople)
	r.Post("/{id}/people", h.vehicleMember.CreatePerson)
	r.Put("/{id}/people/{personId}", h.vehicleMember.UpdatePerson)
	r.Delete("/{id}/people/{personId}", h.vehicleMember.DeletePerson)
	r.Put("/{id}/people/{personId}/link", h.vehicleMember.LinkPerson)
	r.Put("/{id}/people/{personId}/default", h.vehicleMember.SetDefaultPerson)

	// Odometer Checkpoints
	r.Get("/{vehicleId}/odometer-checkpoints", h.checkpoint.List)
	r.Post("/{vehicleId}/odometer-checkpoints", h.checkpoint.Create)
	r.Put("/{vehicleId}/odometer-checkpoints/{checkpointId}", h.checkpoint.Update)
	r.Delete("/{vehicleId}/odometer-checkpoints/{checkpointId}", h.checkpoint.Delete)
}
