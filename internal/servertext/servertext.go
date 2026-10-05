// Package servertext holds short phrases the server builds itself in English and French: text
// sent where there is no HTTP request to translate from (background reminder and sync-failure
// webhooks), or that ends up stored as plain text rather than an apierror code the frontend can
// look up (auto-generated toll notes, a trip's default name). See internal/apierror for
// everything else — text that flows through an HTTP response and can be translated client-side.
package servertext

import "fmt"

var catalog = map[string]map[string]string{
	"reminder.status_due_soon": {"en": "Due soon", "fr": "À prévoir prochainement"},
	"reminder.status_overdue":  {"en": "OVERDUE", "fr": "EN RETARD"},
	"reminder.test_title":      {"en": "Notification test", "fr": "Test de notification"},

	"reminder.discord_title":       {"en": "%s %s: %s", "fr": "%s %s : %s"},
	"reminder.discord_description": {"en": "**Vehicle:** %s\n**Odometer:** %s\n\n%s", "fr": "**Véhicule :** %s\n**Odomètre :** %s\n\n%s"},
	"reminder.discord_footer":      {"en": "AutoLedger • Maintenance tracking", "fr": "AutoLedger • Suivi d'entretien"},

	"reminder.telegram_text": {
		"en": "?? *AutoLedger — Maintenance Reminder*\n\n%s *%s*\nOperation: *%s*\nVehicle: *%s*\nOdometer: %s\n%s",
		"fr": "?? *AutoLedger — Rappel d'Entretien*\n\n%s *%s*\nOpération : *%s*\nVéhicule : *%s*\nOdomètre : %s\n%s",
	},

	"reminder.gotify_title":   {"en": "AutoLedger: %s (%s)", "fr": "AutoLedger : %s (%s)"},
	"reminder.gotify_message": {"en": "Vehicle: %s\nOdometer: %s\n%s", "fr": "Véhicule : %s\nOdomètre : %s\n%s"},

	"reminder.details_mileage_overdue": {"en": "Mileage: %s overdue", "fr": "Kilométrage : Dépassé de %s"},
	"reminder.details_mileage_in":      {"en": "Mileage: in %s", "fr": "Kilométrage : Dans %s"},
	"reminder.details_due_overdue":     {"en": "Due date: %d day(s) overdue", "fr": "Échéance : Dépassée de %d jour(s)"},
	"reminder.details_due_in":          {"en": "Due date: in %d day(s)", "fr": "Échéance : Dans %d jour(s)"},
	"reminder.details_due_reached":     {"en": "Due date reached", "fr": "Échéance atteinte"},

	"sync_alert.body":           {"en": "TeslaMate synchronization failed several times in a row and was automatically suspended (next retry after %s).\nLast error: %v", "fr": "La synchronisation TeslaMate a échoué plusieurs fois de suite et a été suspendue automatiquement (nouvel essai après %s).\nDernière erreur : %v"},
	"sync_alert.discord_title":  {"en": "[ALERT] TeslaMate synchronization failed: %s", "fr": "[ALERTE] Synchronisation TeslaMate en échec : %s"},
	"sync_alert.discord_footer": {"en": "AutoLedger • System alert", "fr": "AutoLedger • Alerte système"},
	"sync_alert.telegram_text":  {"en": "*AutoLedger — Synchronization Alert*\n\nVehicle: *%s*\n%s", "fr": "*AutoLedger — Alerte synchronisation*\n\nVéhicule : *%s*\n%s"},
	"sync_alert.gotify_title":   {"en": "AutoLedger: synchronization failed (%s)", "fr": "AutoLedger : synchronisation en échec (%s)"},

	"toll.auto_prefix":       {"en": "Auto toll: ", "fr": "Péage auto : "},
	"toll.unidentified_exit": {"en": "(exit not identified)", "fr": "(sortie non identifiée)"},
	"toll.barrier":           {"en": "Barrier", "fr": "Barrière"},
	"toll.trip_not_found":    {"en": "Trip not found", "fr": "Trajet introuvable"},
	"toll.detection_failed":  {"en": "Detection or save failed", "fr": "Échec de la détection ou de l'enregistrement"},

	"expense.multi_leg_trip": {"en": "Multi-leg trip", "fr": "Trajet multi-étapes"},

	"servicebook.title":           {"en": "Service book", "fr": "Carnet d'entretien"},
	"servicebook.vehicle":         {"en": "Vehicle", "fr": "Véhicule"},
	"servicebook.vin":             {"en": "VIN", "fr": "VIN"},
	"servicebook.odometer":        {"en": "Odometer", "fr": "Kilométrage"},
	"servicebook.generated":       {"en": "Generated on %s", "fr": "Généré le %s"},
	"servicebook.period":          {"en": "Period: %s to %s", "fr": "Période : du %s au %s"},
	"servicebook.col_date":        {"en": "Date", "fr": "Date"},
	"servicebook.col_km":          {"en": "Odometer", "fr": "Kilométrage"},
	"servicebook.col_work":        {"en": "Work", "fr": "Intervention"},
	"servicebook.col_amount":      {"en": "Amount", "fr": "Montant"},
	"servicebook.col_doc":         {"en": "Proof", "fr": "Justificatif"},
	"servicebook.total":           {"en": "Total (%s)", "fr": "Total (%s)"},
	"servicebook.empty":           {"en": "No maintenance recorded for this period.", "fr": "Aucun entretien enregistré sur cette période."},
	"servicebook.annex_page":      {"en": "Proof %d: %s", "fr": "Justificatif %d : %s"},
	"servicebook.not_embedded":    {"en": "Not embedded (file format): download it from the app", "fr": "Non intégré (format du fichier) : à télécharger depuis l'application"},
	"servicebook.page":            {"en": "Page %d", "fr": "Page %d"},
	"servicebook.cat.MAINTENANCE": {"en": "Maintenance", "fr": "Entretien"},
	"servicebook.cat.REPAIR":      {"en": "Repair", "fr": "Réparation"},
	"servicebook.cat.TIRES":       {"en": "Tires", "fr": "Pneumatiques"},
	"servicebook.cat.ACCESSORY":   {"en": "Accessory", "fr": "Accessoire"},
	"servicebook.cat.OTHER":       {"en": "Other", "fr": "Autre"},

	"trip.departure": {"en": "Start", "fr": "Départ"},
	"trip.arrival":   {"en": "Arrival", "fr": "Arrivée"},
}

// kmPerMile converts the stored kilometres for a reader who chose miles.
const kmPerMile = 1.609344

// Distance formats a distance stored in km in the reader's unit ("km" or "mi", anything else is km),
// rounded to the unit: "42000 km", "26098 mi".
func Distance(unit string, km float64) string {
	if unit == "mi" {
		return fmt.Sprintf("%.0f mi", km/kmPerMile)
	}
	return fmt.Sprintf("%.0f km", km)
}

// Text returns the key's phrase in lang ("en" or "fr", falling back to English for an unknown
// language or a missing translation), formatted with args when the phrase takes any. An unknown
// key returns itself, so a typo shows up in the output rather than panicking.
func Text(lang, key string, args ...any) string {
	entry, ok := catalog[key]
	if !ok {
		return key
	}
	tpl, ok := entry[lang]
	if !ok {
		tpl = entry["en"]
	}
	if len(args) == 0 {
		return tpl
	}
	return fmt.Sprintf(tpl, args...)
}
