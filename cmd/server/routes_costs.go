package main

import (
	"github.com/go-chi/chi/v5"
)

// registerCostRoutes registers carpools, tires, expenses, documents, reminders and analytics under /api/vehicles.
func (h *apiHandlers) registerCostRoutes(r chi.Router) {
	// Carpooling
	r.Get("/{vehicleId}/carpools", h.carpool.List)
	r.Post("/{vehicleId}/carpools", h.carpool.Create)
	r.Post("/{vehicleId}/carpools/recalculate", h.carpool.Recalculate)
	r.Get("/{vehicleId}/carpools/estimate", h.carpool.Estimate)
	r.Get("/{vehicleId}/carpools/{id}", h.carpool.Get)
	r.Put("/{vehicleId}/carpools/{id}", h.carpool.Update)
	r.Delete("/{vehicleId}/carpools/{id}", h.carpool.Delete)

	// Tires
	r.Get("/{vehicleId}/tires", h.tire.List)
	r.Post("/{vehicleId}/tires", h.tire.Create)
	r.Post("/{vehicleId}/tires/batch", h.tire.BatchCreate)
	r.Patch("/{vehicleId}/tires/batch", h.tire.BatchUpdate)
	r.Post("/{vehicleId}/tires/batch-dispose", h.tire.BatchDispose)
	r.Post("/{vehicleId}/tires/quick-rotate", h.tire.QuickRotate)
	r.Put("/{vehicleId}/tires/{tireId}", h.tire.Update)
	r.Delete("/{vehicleId}/tires/{tireId}", h.tire.Delete)
	r.Post("/{vehicleId}/tires/{tireId}/dispose", h.tire.Dispose)
	r.Post("/{vehicleId}/tires/{tireId}/copy-history", h.tire.CopyHistory)
	r.Get("/{vehicleId}/tires/{tireId}/history", h.tire.GetHistory)
	r.Post("/{vehicleId}/tires/{tireId}/sessions", h.tire.CreateSession)
	r.Put("/{vehicleId}/tires/{tireId}/sessions/{sessionId}", h.tire.UpdateSession)
	r.Delete("/{vehicleId}/tires/{tireId}/sessions/{sessionId}", h.tire.DeleteSession)
	r.Post("/{vehicleId}/tires/{tireId}/logs", h.tire.AddLog)
	r.Put("/{vehicleId}/tires/{tireId}/logs/{logId}", h.tire.UpdateLog)
	r.Delete("/{vehicleId}/tires/{tireId}/logs/{logId}", h.tire.DeleteLog)
	r.Post("/{vehicleId}/tire-rotations", h.tire.Rotate)

	// Expenses
	r.Get("/{vehicleId}/expenses", h.expense.ListDriveExpenses)
	r.Post("/{vehicleId}/expenses", h.expense.CreateDriveExpense)
	r.Put("/{vehicleId}/expenses/{expenseId}", h.expense.UpdateDriveExpense)
	r.Delete("/{vehicleId}/expenses/{expenseId}", h.expense.DeleteDriveExpense)
	r.Get("/{vehicleId}/maintenance", h.expense.ListMaintenance)
	r.Post("/{vehicleId}/maintenance", h.expense.CreateMaintenance)
	r.Put("/{vehicleId}/maintenance/{maintenanceId}", h.expense.UpdateMaintenance)
	r.Delete("/{vehicleId}/maintenance/{maintenanceId}", h.expense.DeleteMaintenance)
	r.Get("/{vehicleId}/charges", h.expense.ListCharges)
	r.Post("/{vehicleId}/charges", h.expense.CreateManualCharge)
	r.Put("/{vehicleId}/charges/{chargeId}", h.expense.UpdateCharge)
	r.Delete("/{vehicleId}/charges/{chargeId}", h.expense.DeleteManualCharge)

	// Documents & Invoices
	r.Get("/{vehicleId}/documents", h.expense.ListDocuments)
	r.Post("/{vehicleId}/documents", h.expense.UploadDocument)
	r.Get("/{vehicleId}/documents/{docId}", h.expense.DownloadDocument)
	r.Delete("/{vehicleId}/documents/{docId}", h.expense.DeleteDocument)

	// Maintenance Reminders & Webhooks
	r.Get("/{vehicleId}/reminders", h.reminder.List)
	r.Post("/{vehicleId}/reminders", h.reminder.Create)
	r.Post("/{vehicleId}/reminders/apply-template", h.reminder.ApplyTemplate)
	r.Put("/{vehicleId}/reminders/{reminderId}", h.reminder.Update)
	r.Delete("/{vehicleId}/reminders/{reminderId}", h.reminder.Delete)
	r.Post("/{vehicleId}/reminders/{reminderId}/complete", h.reminder.Complete)
	r.Get("/{vehicleId}/webhook", h.reminder.GetWebhook)
	r.Put("/{vehicleId}/webhook", h.reminder.SaveWebhook)
	r.Delete("/{vehicleId}/webhook", h.reminder.DeleteWebhook)
	r.Post("/{vehicleId}/webhook/test", h.reminder.TestWebhook)

	// TCO Analytics
	r.Get("/{vehicleId}/tco", h.tco.GetTCO)
	r.Get("/{vehicleId}/energy-stats", h.energy.GetStats)
}
