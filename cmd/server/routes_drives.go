package main

import (
	"github.com/go-chi/chi/v5"
)

// registerDriveRoutes registers fuel logs, drives, trips, reports, exports and imports under /api/vehicles.
func (h *apiHandlers) registerDriveRoutes(r chi.Router) {
	// Fuel fill-ups (combustion vehicles)
	r.Get("/{vehicleId}/fuel-logs", h.fuel.List)
	r.Post("/{vehicleId}/fuel-logs", h.fuel.Create)
	r.Put("/{vehicleId}/fuel-logs/{fuelLogId}", h.fuel.Update)
	r.Delete("/{vehicleId}/fuel-logs/{fuelLogId}", h.fuel.Delete)

	// Drives
	r.Get("/{vehicleId}/drives", h.drive.List)
	r.Get("/{vehicleId}/drives/address-backfill", h.drive.AddressBackfillStatus)
	r.Post("/{vehicleId}/drives/resolve-addresses", h.drive.ResolveAddresses)
	r.Post("/{vehicleId}/drives", h.drive.Create)
	r.Put("/{vehicleId}/drives/{driveId}", h.drive.Update)
	r.Delete("/{vehicleId}/drives/{driveId}", h.drive.Delete)
	r.Put("/{vehicleId}/drives/{driveId}/driver", h.drive.UpdateDriver)
	r.Get("/{vehicleId}/drives/{driveId}/expenses", h.drive.GetDriveExpenses)

	// Export (CSV / JSON)
	r.Get("/{vehicleId}/export", h.export.Export)
	r.Get("/{vehicleId}/service-book", h.serviceBook.Download)
	r.Get("/{vehicleId}/mileage-report", h.mileage.Report)
	r.Get("/{vehicleId}/battery-health", h.residual.BatteryHealth)
	r.Post("/{vehicleId}/battery-health", h.residual.SaveReading)
	r.Delete("/{vehicleId}/battery-health/{date}", h.residual.DeleteReading)
	r.Get("/{vehicleId}/residual-value", h.residual.Residual)

	// Import (CSV Charges & Drives)
	r.Post("/{vehicleId}/import/preview", h.csvImport.Preview)
	r.Post("/{vehicleId}/import/execute", h.csvImport.Execute)
	r.Get("/{vehicleId}/import/batches", h.csvImport.ListBatches)
	r.Delete("/{vehicleId}/import/batches/{batchId}", h.csvImport.UndoBatch)
	r.Patch("/{vehicleId}/drives/{driveId}/tags", h.drive.UpdateTags)
	r.Patch("/{vehicleId}/drives/{driveId}/toll-review", h.drive.SetTollReview)
	r.Get("/{vehicleId}/drives/{driveId}/toll-detection", h.drive.GetTollDetection)
	r.Post("/{vehicleId}/drives/{driveId}/detect-tolls", h.drive.DetectTolls)
	r.Post("/{vehicleId}/drives/{driveId}/apply-toll-estimate", h.drive.ApplyTollEstimate)
	r.Post("/{vehicleId}/drives/apply-toll-estimates", h.drive.ApplyTollEstimatesBulk)
	r.Post("/{vehicleId}/trip-groups", h.drive.CreateTripGroup)
	r.Get("/{vehicleId}/trip-groups", h.drive.ListTripGroups)
	r.Get("/{vehicleId}/trip-suggestions", h.drive.TripSuggestions)
	r.Post("/{vehicleId}/trip-suggestions/dismiss", h.drive.DismissTripSuggestion)
	r.Put("/{vehicleId}/trip-groups/{groupId}", h.drive.UpdateTripGroup)
	r.Delete("/{vehicleId}/trip-groups/{groupId}", h.drive.DeleteTripGroup)
}
