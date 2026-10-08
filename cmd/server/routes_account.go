package main

import (
	"github.com/go-chi/chi/v5"
)

// registerAccountRoutes registers the routes that belong to the signed-in owner rather than to one vehicle.
func (h *apiHandlers) registerAccountRoutes(r chi.Router) {
	// Tariffs & Public Charging Calculator
	r.Route("/api/mileage-rates", func(r chi.Router) {
		r.Get("/", h.mileage.ListRates)
		r.Post("/", h.mileage.CreateRate)
		r.Delete("/{id}", h.mileage.DeleteRate)
	})
	// Reminder templates
	r.Route("/api/reminder-templates", func(r chi.Router) {
		r.Get("/", h.reminder.ListTemplates)
		r.Post("/", h.reminder.CreateTemplate)
		r.Delete("/{id}", h.reminder.DeleteTemplate)
	})
	r.Route("/api/tariffs", func(r chi.Router) {
		r.Get("/plans", h.tariff.List)
		r.Post("/plans", h.tariff.Create)
		r.Put("/plans/{id}", h.tariff.Update)
		r.Delete("/plans/{id}", h.tariff.Delete)
		r.Post("/calculate-session", h.tariff.Calculate)
		r.Get("/public-presets", h.tariff.ListPublicPresets)
		r.Post("/public-presets", h.tariff.CreatePublicPreset)
		r.Delete("/public-presets/{id}", h.tariff.DeletePublicPreset)
		r.Post("/calculate-public", h.tariff.CalculatePublic)
	})

	// Pending Charges ("Recharges à qualifier")
	r.Route("/api/pending-charges", func(r chi.Router) {
		r.Get("/", h.pendingCharges.List)
		r.Post("/{id}/assign", h.pendingCharges.Assign)
		r.Delete("/{id}", h.pendingCharges.Delete)
	})

	// Saved CSV import mappings
	r.Route("/api/import-profiles", func(r chi.Router) {
		r.Get("/", h.importProfile.List)
		r.Post("/", h.importProfile.Save)
		r.Delete("/{profileId}", h.importProfile.Delete)
	})

	// Household Fleet Dashboard
	r.Get("/api/fleet/summary", h.fleet.GetSummary)
	r.Put("/api/fleet/budget", h.fleet.SetBudget)

	// EV vs ICE cost comparison (informational)
	r.Route("/api/comparison-scenarios", func(r chi.Router) {
		r.Get("/", h.comparison.List)
		r.Post("/", h.comparison.Create)
		r.Get("/defaults", h.comparison.Defaults)
		r.Put("/{scenarioId}", h.comparison.Update)
		r.Delete("/{scenarioId}", h.comparison.Delete)
		r.Get("/{scenarioId}/result", h.comparison.Result)
	})
}
