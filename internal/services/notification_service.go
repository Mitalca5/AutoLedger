package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/servertext"
)

// notificationStore is the narrow slice of *database.Repository that NotificationService
// actually needs. Consumer-defined so tests can supply a fake without a real database
// (mirrors the syncStore interface in sync_service.go).
type notificationStore interface {
	GetVehicleWebhook(ctx context.Context, vehicleID string) (*models.VehicleWebhook, error)
	ListMaintenanceReminders(ctx context.Context, vehicleID string, currentOdo float64) ([]models.MaintenanceReminder, error)
	MarkReminderNotified(ctx context.Context, reminderID string, notifiedAt time.Time, notifiedOdo float64) error
	GetUserByID(ctx context.Context, id string) (*models.User, error)
	ListVehiclesWithActiveWebhook(ctx context.Context) ([]models.Vehicle, error)
}

// NotificationService handles evaluation of maintenance reminders and dispatching homelab webhooks.
type NotificationService struct {
	repo       notificationStore
	httpClient *http.Client
}

// NewNotificationService creates a new NotificationService instance.
func NewNotificationService(repo notificationStore) *NotificationService {
	return &NotificationService{
		repo: repo,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// language is the stored language of userID ("en" or "fr", defaulting to "en"), used for webhook
// text built here in the background: there is no HTTP request whose Accept-Language or vue-i18n
// catalog could translate it instead.
func (s *NotificationService) language(ctx context.Context, userID string) string {
	lang, _ := s.readerPrefs(ctx, userID)
	return lang
}

// readerPrefs is the vehicle owner's language ("en" or "fr") and distance unit ("km" or "mi"), for text
// built here without a request to take them from; English and kilometres when unknown.
func (s *NotificationService) readerPrefs(ctx context.Context, userID string) (lang, unit string) {
	lang, unit = "en", "km"
	if s.repo == nil || userID == "" {
		return
	}
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return
	}
	if user.Language == "fr" {
		lang = "fr"
	}
	if user.DistanceUnit == "mi" {
		unit = "mi"
	}
	return
}

// CheckAndNotify evaluates reminders for a vehicle and sends webhooks if due thresholds are crossed.
func (s *NotificationService) CheckAndNotify(ctx context.Context, vehicle *models.Vehicle, currentOdometer float64) error {
	if s.repo == nil || vehicle == nil {
		return nil
	}

	webhook, err := s.repo.GetVehicleWebhook(ctx, vehicle.ID)
	if err != nil {
		return fmt.Errorf("failed to get vehicle webhook: %w", err)
	}
	if webhook == nil || !webhook.Enabled || strings.TrimSpace(webhook.URL) == "" {
		return nil // No active webhook configured
	}

	reminders, err := s.repo.ListMaintenanceReminders(ctx, vehicle.ID, currentOdometer)
	if err != nil {
		return fmt.Errorf("failed to list reminders: %w", err)
	}

	lang, unit := s.readerPrefs(ctx, vehicle.UserID)
	now := time.Now()
	for _, rem := range reminders {
		if !rem.WebhookEnabled || rem.Status == "OK" {
			continue
		}

		// Anti-spam rule: do not re-notify if notified in the last 7 days AND odometer hasn't changed by >= 500 km
		if rem.LastNotifiedAt != nil {
			timeSinceNotification := now.Sub(*rem.LastNotifiedAt)
			odoDelta := 0.0
			if rem.LastNotifiedOdometer != nil {
				odoDelta = currentOdometer - *rem.LastNotifiedOdometer
				if odoDelta < 0 {
					odoDelta = -odoDelta
				}
			}
			if timeSinceNotification < 7*24*time.Hour && odoDelta < 500 {
				continue
			}
		}

		// Dispatch notification
		if err := s.sendReminderWebhook(ctx, webhook, lang, unit, vehicle.Name, &rem, currentOdometer); err != nil {
			slog.Error("failed to dispatch webhook", "component", "notification", "vehicle_id", vehicle.ID, "reminder_id", rem.ID, "error", err)
			continue
		}

		// Update last notified
		if err := s.repo.MarkReminderNotified(ctx, rem.ID, now, currentOdometer); err != nil {
			slog.Error("failed to mark reminder notified", "component", "notification", "reminder_id", rem.ID, "error", err)
		}
	}

	return nil
}

// CheckAllVehicles evaluates the reminders of every vehicle with an active webhook against its recorded odometer,
// whatever feeds that vehicle (TeslaMate, Home Assistant, manual entries). It is the scheduled counterpart of the
// check run after a TeslaMate odometer change, and the only one that catches a reminder reaching its due date.
// A failing vehicle is logged and does not stop the others.
func (s *NotificationService) CheckAllVehicles(ctx context.Context) error {
	if s.repo == nil {
		return nil
	}
	vehicles, err := s.repo.ListVehiclesWithActiveWebhook(ctx)
	if err != nil {
		return fmt.Errorf("failed to list vehicles with a webhook: %w", err)
	}
	for i := range vehicles {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		v := &vehicles[i]
		if err := s.CheckAndNotify(ctx, v, v.CurrentOdometer); err != nil {
			slog.Error("reminder check failed", "component", "notification", "vehicle_id", v.ID, "error", err)
		}
	}
	return nil
}

// TestWebhook dispatches a test message to verify connectivity and configuration.
func (s *NotificationService) TestWebhook(ctx context.Context, webhook *models.VehicleWebhook, vehicle *models.Vehicle) error {
	lang, unit := s.readerPrefs(ctx, vehicle.UserID)
	testReminder := &models.MaintenanceReminder{
		Title:  servertext.Text(lang, "reminder.test_title"),
		Status: "DUE_SOON",
	}
	remKm := 500.0
	testReminder.RemainingKm = &remKm
	remDays := 15
	testReminder.RemainingDays = &remDays

	return s.sendReminderWebhook(ctx, webhook, lang, unit, vehicle.Name, testReminder, 50000)
}

func (s *NotificationService) sendReminderWebhook(
	ctx context.Context,
	webhook *models.VehicleWebhook,
	lang, unit string,
	vehicleName string,
	rem *models.MaintenanceReminder,
	currentOdo float64,
) error {
	payload, err := formatPayload(lang, unit, webhook.Type, vehicleName, rem, currentOdo)
	if err != nil {
		return err
	}
	return s.postWebhook(ctx, webhook.URL, payload)
}

// NotifySyncCircuitOpen alerts a vehicle's configured webhook that its TeslaMate
// synchronization has failed repeatedly and has been automatically suspended until retryAt.
// Unlike maintenance reminders this is not evaluated on a schedule: callers are expected to
// invoke it exactly once, right when a circuit breaker trips from CLOSED/HALF_OPEN to OPEN.
func (s *NotificationService) NotifySyncCircuitOpen(ctx context.Context, vehicle *models.Vehicle, cause error, retryAt time.Time) error {
	if s.repo == nil || vehicle == nil {
		return nil
	}

	webhook, err := s.repo.GetVehicleWebhook(ctx, vehicle.ID)
	if err != nil {
		return fmt.Errorf("failed to get vehicle webhook: %w", err)
	}
	if webhook == nil || !webhook.Enabled || strings.TrimSpace(webhook.URL) == "" {
		return nil // No active webhook configured
	}

	payload := formatSyncAlertPayload(s.language(ctx, vehicle.UserID), webhook.Type, vehicle.Name, cause, retryAt)
	return s.postWebhook(ctx, webhook.URL, payload)
}

// postWebhook serializes and POSTs payload to url, shared by all webhook notification kinds.
func (s *NotificationService) postWebhook(ctx context.Context, webhookURL string, payload any) error {
	url := strings.TrimSpace(webhookURL)
	if url == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AutoLedger-NotificationService/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook endpoint returned HTTP %d", resp.StatusCode)
	}

	return nil
}

func formatPayload(
	lang, unit, webhookType, vehicleName string,
	rem *models.MaintenanceReminder,
	currentOdo float64,
) (any, error) {
	statusLabel := servertext.Text(lang, "reminder.status_due_soon")
	statusEmoji := "??"
	color := 16753920 // Amber

	if rem.Status == "OVERDUE" {
		statusLabel = servertext.Text(lang, "reminder.status_overdue")
		statusEmoji = "??"
		color = 15158332 // Red
	}

	details := buildDetailsString(lang, unit, rem)
	odometer := servertext.Distance(unit, currentOdo)

	switch strings.ToUpper(webhookType) {
	case "DISCORD":
		return map[string]any{
			"username":   "AutoLedger",
			"avatar_url": "https://raw.githubusercontent.com/Rem7474/TeslaCost/main/web/public/favicon.svg",
			"embeds": []map[string]any{
				{
					"title":       servertext.Text(lang, "reminder.discord_title", statusEmoji, statusLabel, rem.Title),
					"description": servertext.Text(lang, "reminder.discord_description", vehicleName, odometer, details),
					"color":       color,
					"footer": map[string]string{
						"text": servertext.Text(lang, "reminder.discord_footer"),
					},
					"timestamp": time.Now().Format(time.RFC3339),
				},
			},
		}, nil

	case "TELEGRAM":
		text := servertext.Text(lang, "reminder.telegram_text", statusEmoji, statusLabel, rem.Title, vehicleName, odometer, details)
		return map[string]any{
			"text":       text,
			"parse_mode": "Markdown",
		}, nil

	case "GOTIFY":
		priority := 5
		if rem.Status == "OVERDUE" {
			priority = 8
		}
		return map[string]any{
			"title":    servertext.Text(lang, "reminder.gotify_title", rem.Title, statusLabel),
			"message":  servertext.Text(lang, "reminder.gotify_message", vehicleName, odometer, details),
			"priority": priority,
		}, nil

	default: // GENERIC
		return map[string]any{
			"event":            "maintenance_reminder",
			"status":           rem.Status,
			"title":            rem.Title,
			"vehicle_name":     vehicleName,
			"current_odometer": currentOdo,
			"remaining_km":     rem.RemainingKm,
			"remaining_days":   rem.RemainingDays,
			"timestamp":        time.Now().Format(time.RFC3339),
		}, nil
	}
}

// formatSyncAlertPayload builds a webhook payload announcing that automatic TeslaMate
// synchronization has been suspended for a vehicle after repeated failures.
func formatSyncAlertPayload(lang, webhookType, vehicleName string, cause error, retryAt time.Time) any {
	message := servertext.Text(lang, "sync_alert.body", retryAt.Format("02/01/2006 15:04 MST"), cause)

	switch strings.ToUpper(webhookType) {
	case "DISCORD":
		return map[string]any{
			"username":   "AutoLedger",
			"avatar_url": "https://raw.githubusercontent.com/Rem7474/TeslaCost/main/web/public/favicon.svg",
			"embeds": []map[string]any{
				{
					"title":       servertext.Text(lang, "sync_alert.discord_title", vehicleName),
					"description": message,
					"color":       15158332, // Red
					"footer": map[string]string{
						"text": servertext.Text(lang, "sync_alert.discord_footer"),
					},
					"timestamp": time.Now().Format(time.RFC3339),
				},
			},
		}

	case "TELEGRAM":
		return map[string]any{
			"text":       servertext.Text(lang, "sync_alert.telegram_text", vehicleName, message),
			"parse_mode": "Markdown",
		}

	case "GOTIFY":
		return map[string]any{
			"title":    servertext.Text(lang, "sync_alert.gotify_title", vehicleName),
			"message":  message,
			"priority": 8,
		}

	default: // GENERIC
		return map[string]any{
			"event":        "sync_circuit_open",
			"vehicle_name": vehicleName,
			"error":        cause.Error(),
			"retry_at":     retryAt.Format(time.RFC3339),
			"timestamp":    time.Now().Format(time.RFC3339),
		}
	}
}

func buildDetailsString(lang, unit string, rem *models.MaintenanceReminder) string {
	var parts []string
	if rem.RemainingKm != nil {
		if *rem.RemainingKm <= 0 {
			parts = append(parts, servertext.Text(lang, "reminder.details_mileage_overdue", servertext.Distance(unit, -*rem.RemainingKm)))
		} else {
			parts = append(parts, servertext.Text(lang, "reminder.details_mileage_in", servertext.Distance(unit, *rem.RemainingKm)))
		}
	}
	if rem.RemainingDays != nil {
		if *rem.RemainingDays <= 0 {
			parts = append(parts, servertext.Text(lang, "reminder.details_due_overdue", -*rem.RemainingDays))
		} else {
			parts = append(parts, servertext.Text(lang, "reminder.details_due_in", *rem.RemainingDays))
		}
	}
	if len(parts) == 0 {
		return servertext.Text(lang, "reminder.details_due_reached")
	}
	return strings.Join(parts, " • ")
}
