package services

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

// fakeNotificationStore is an in-memory notificationStore, letting CheckAndNotify's business
// logic (anti-spam window, webhook dispatch) be tested without a real database.
type fakeNotificationStore struct {
	webhook          *models.VehicleWebhook
	reminders        []models.MaintenanceReminder
	markedNotifiedID string
	language         string
	activeVehicles   []models.Vehicle
	checkedOdometers []float64
	listErr          error
	webhookErr       error
}

func (f *fakeNotificationStore) GetVehicleWebhook(ctx context.Context, vehicleID string) (*models.VehicleWebhook, error) {
	if f.webhookErr != nil {
		return nil, f.webhookErr
	}
	return f.webhook, nil
}

func (f *fakeNotificationStore) ListMaintenanceReminders(ctx context.Context, vehicleID string, currentOdo float64) ([]models.MaintenanceReminder, error) {
	f.checkedOdometers = append(f.checkedOdometers, currentOdo)
	return f.reminders, nil
}

func (f *fakeNotificationStore) ListVehiclesWithActiveWebhook(ctx context.Context) ([]models.Vehicle, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.activeVehicles, nil
}

func (f *fakeNotificationStore) MarkReminderNotified(ctx context.Context, reminderID string, notifiedAt time.Time, notifiedOdo float64) error {
	f.markedNotifiedID = reminderID
	return nil
}

func (f *fakeNotificationStore) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	lang := f.language
	if lang == "" {
		lang = "en"
	}
	return &models.User{ID: id, Language: lang}, nil
}

func TestCheckAndNotifyDispatchesDueReminder(t *testing.T) {
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := &fakeNotificationStore{
		webhook: &models.VehicleWebhook{URL: server.URL, Type: "GENERIC", Enabled: true},
		reminders: []models.MaintenanceReminder{
			{ID: "rem-1", Title: "Vidange", Status: "OVERDUE", WebhookEnabled: true},
		},
	}
	svc := NewNotificationService(store)

	if err := svc.CheckAndNotify(context.Background(), &models.Vehicle{ID: "v1", Name: "Model 3"}, 50000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected exactly 1 webhook call, got %d", callCount)
	}
	if store.markedNotifiedID != "rem-1" {
		t.Fatalf("expected reminder rem-1 to be marked notified, got %q", store.markedNotifiedID)
	}
}

func TestCheckAndNotifySkipsRecentlyNotifiedReminder(t *testing.T) {
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifiedAt := time.Now().Add(-24 * time.Hour) // 1 day ago: inside the 7-day anti-spam window
	notifiedOdo := 49900.0                        // 100 km ago: below the 500 km anti-spam threshold
	store := &fakeNotificationStore{
		webhook: &models.VehicleWebhook{URL: server.URL, Type: "GENERIC", Enabled: true},
		reminders: []models.MaintenanceReminder{
			{
				ID: "rem-1", Title: "Vidange", Status: "OVERDUE", WebhookEnabled: true,
				LastNotifiedAt: &notifiedAt, LastNotifiedOdometer: &notifiedOdo,
			},
		},
	}
	svc := NewNotificationService(store)

	if err := svc.CheckAndNotify(context.Background(), &models.Vehicle{ID: "v1", Name: "Model 3"}, 50000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 0 {
		t.Fatalf("expected the anti-spam window to suppress the webhook, got %d calls", callCount)
	}
}

func TestFormatPayloadDiscord(t *testing.T) {
	remKm := 450.0
	rem := &models.MaintenanceReminder{
		Title:       "Permutation des pneus",
		Status:      "DUE_SOON",
		RemainingKm: &remKm,
	}

	payload, err := formatPayload("en", "km", "DISCORD", "Model 3", rem, 42000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", payload)
	}
	embeds, ok := m["embeds"].([]map[string]any)
	if !ok || len(embeds) == 0 {
		t.Fatalf("expected embeds array")
	}
	if embeds[0]["color"] != 16753920 { // Amber for DUE_SOON
		t.Errorf("expected amber color, got %v", embeds[0]["color"])
	}
	if !strings.Contains(embeds[0]["title"].(string), "Due soon") {
		t.Errorf("expected the English status label, got %v", embeds[0]["title"])
	}

	frPayload, err := formatPayload("fr", "km", "DISCORD", "Model 3", rem, 42000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	frEmbeds := frPayload.(map[string]any)["embeds"].([]map[string]any)
	if !strings.Contains(frEmbeds[0]["footer"].(map[string]string)["text"], "Suivi d'entretien") {
		t.Errorf("expected the French footer, got %v", frEmbeds[0]["footer"])
	}
}

func TestFormatPayloadTelegramAndGotify(t *testing.T) {
	remKm := -120.0
	rem := &models.MaintenanceReminder{
		Title:       "Filtre habitacle",
		Status:      "OVERDUE",
		RemainingKm: &remKm,
	}

	// Telegram
	tgPayload, err := formatPayload("en", "km", "TELEGRAM", "Model Y", rem, 55000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tgMap := tgPayload.(map[string]any)
	if tgMap["parse_mode"] != "Markdown" {
		t.Errorf("expected Markdown parse mode")
	}

	// Gotify
	gotifyPayload, err := formatPayload("en", "km", "GOTIFY", "Model Y", rem, 55000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotifyMap := gotifyPayload.(map[string]any)
	if gotifyMap["priority"] != 8 {
		t.Errorf("expected priority 8 for OVERDUE, got %v", gotifyMap["priority"])
	}
}

func TestFormatPayloadInMiles(t *testing.T) {
	remKm := 1609.344
	rem := &models.MaintenanceReminder{Title: "Tire rotation", Status: "DUE_SOON", RemainingKm: &remKm}

	payload, err := formatPayload("en", "mi", "GOTIFY", "Model 3", rem, 16093.44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	message := payload.(map[string]any)["message"].(string)
	for _, want := range []string{"Odometer: 10000 mi", "Mileage: in 1000 mi"} {
		if !strings.Contains(message, want) {
			t.Errorf("expected %q in %q", want, message)
		}
	}
	if strings.Contains(message, "km") {
		t.Errorf("expected no km in a miles reader's message, got %q", message)
	}

	kmPayload, _ := formatPayload("en", "km", "GOTIFY", "Model 3", rem, 16093.44)
	if !strings.Contains(kmPayload.(map[string]any)["message"].(string), "Odometer: 16093 km") {
		t.Errorf("expected the odometer in km, got %v", kmPayload)
	}
}

func TestFormatSyncAlertPayload(t *testing.T) {
	cause := context.DeadlineExceeded
	retryAt := time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)

	discord := formatSyncAlertPayload("en", "DISCORD", "Model 3", cause, retryAt).(map[string]any)
	embeds, ok := discord["embeds"].([]map[string]any)
	if !ok || len(embeds) == 0 {
		t.Fatalf("expected embeds array")
	}
	if embeds[0]["color"] != 15158332 {
		t.Errorf("expected red color for a sync failure alert, got %v", embeds[0]["color"])
	}
	if !strings.Contains(embeds[0]["title"].(string), "[ALERT]") {
		t.Errorf("expected the English alert title, got %v", embeds[0]["title"])
	}

	frDiscord := formatSyncAlertPayload("fr", "DISCORD", "Model 3", cause, retryAt).(map[string]any)
	frEmbeds := frDiscord["embeds"].([]map[string]any)
	if !strings.Contains(frEmbeds[0]["title"].(string), "[ALERTE]") {
		t.Errorf("expected the French alert title, got %v", frEmbeds[0]["title"])
	}

	generic := formatSyncAlertPayload("en", "GENERIC", "Model 3", cause, retryAt).(map[string]any)
	if generic["event"] != "sync_circuit_open" {
		t.Errorf("expected sync_circuit_open event, got %v", generic["event"])
	}
	if generic["vehicle_name"] != "Model 3" {
		t.Errorf("expected vehicle_name to be set, got %v", generic["vehicle_name"])
	}
}

func TestNotifySyncCircuitOpenWithoutRepoIsNoop(t *testing.T) {
	svc := NewNotificationService(nil)
	err := svc.NotifySyncCircuitOpen(context.Background(), &models.Vehicle{ID: "v1", Name: "Model 3"}, errors.New("boom"), time.Now())
	if err != nil {
		t.Fatalf("expected no error without a repo, got %v", err)
	}
}

func TestSendReminderWebhook(t *testing.T) {
	serverCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := NewNotificationService(nil)
	webhook := &models.VehicleWebhook{
		URL:     server.URL,
		Type:    "DISCORD",
		Enabled: true,
	}

	err := svc.TestWebhook(context.Background(), webhook, &models.Vehicle{Name: "Tesla Test"})
	if err != nil {
		t.Fatalf("unexpected test webhook error: %v", err)
	}
	if !serverCalled {
		t.Errorf("expected webhook endpoint to be called")
	}
}

func TestComputeStatus(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	intKm := 10000
	lastOdo := 30000.0
	rem := &models.MaintenanceReminder{
		IntervalKm:          &intKm,
		LastServiceOdometer: &lastOdo,
		LeadKm:              1000,
	}

	// 1. Odo = 35000 -> OK (5000 km left)
	rem.ComputeStatus(35000, now)
	if rem.Status != "OK" || *rem.RemainingKm != 5000 {
		t.Errorf("expected OK with 5000 km left, got %s, %v", rem.Status, rem.RemainingKm)
	}

	// 2. Odo = 39200 -> DUE_SOON (800 km left <= 1000 leadKm)
	rem.ComputeStatus(39200, now)
	if rem.Status != "DUE_SOON" || *rem.RemainingKm != 800 {
		t.Errorf("expected DUE_SOON with 800 km left, got %s, %v", rem.Status, rem.RemainingKm)
	}

	// 3. Odo = 40100 -> OVERDUE (-100 km)
	rem.ComputeStatus(40100, now)
	if rem.Status != "OVERDUE" || *rem.RemainingKm != -100 {
		t.Errorf("expected OVERDUE with -100 km left, got %s, %v", rem.Status, rem.RemainingKm)
	}
}

func TestCheckAllVehiclesEvaluatesEachVehicleAtItsRecordedOdometer(t *testing.T) {
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := &fakeNotificationStore{
		webhook: &models.VehicleWebhook{URL: server.URL, Type: "GENERIC", Enabled: true},
		reminders: []models.MaintenanceReminder{
			{ID: "rem-1", Title: "Inspection", Status: "OVERDUE", WebhookEnabled: true},
		},
		activeVehicles: []models.Vehicle{
			{ID: "v1", Name: "Zoe", CurrentOdometer: 41000},
			{ID: "v2", Name: "Clio", CurrentOdometer: 87000},
		},
	}
	svc := NewNotificationService(store)

	if err := svc.CheckAllVehicles(context.Background()); err != nil {
		t.Fatalf("CheckAllVehicles: %v", err)
	}
	if len(store.checkedOdometers) != 2 || store.checkedOdometers[0] != 41000 || store.checkedOdometers[1] != 87000 {
		t.Fatalf("reminders must be evaluated at each vehicle's own odometer, got %v", store.checkedOdometers)
	}
	if callCount != 2 {
		t.Fatalf("a due reminder must be sent for each vehicle, got %d webhook calls", callCount)
	}
}

func TestCheckAllVehiclesWithoutWebhookVehiclesDoesNothing(t *testing.T) {
	store := &fakeNotificationStore{}
	svc := NewNotificationService(store)
	if err := svc.CheckAllVehicles(context.Background()); err != nil {
		t.Fatalf("CheckAllVehicles: %v", err)
	}
	if len(store.checkedOdometers) != 0 {
		t.Fatalf("no vehicle has an active webhook, got %v", store.checkedOdometers)
	}
}

func TestCheckAllVehiclesReportsAListingFailure(t *testing.T) {
	cause := errors.New("database unavailable")
	svc := NewNotificationService(&fakeNotificationStore{listErr: cause})
	if err := svc.CheckAllVehicles(context.Background()); !errors.Is(err, cause) {
		t.Fatalf("CheckAllVehicles = %v, want the listing error", err)
	}
}

func TestCheckAllVehiclesKeepsGoingWhenOneVehicleFails(t *testing.T) {
	store := &fakeNotificationStore{
		webhookErr:     errors.New("webhook lookup failed"),
		activeVehicles: []models.Vehicle{{ID: "v1", Name: "Zoe"}, {ID: "v2", Name: "Clio"}},
	}
	svc := NewNotificationService(store)
	if err := svc.CheckAllVehicles(context.Background()); err != nil {
		t.Fatalf("a failing vehicle must not fail the run, got %v", err)
	}
}

func TestCheckAllVehiclesStopsWhenTheContextEnds(t *testing.T) {
	store := &fakeNotificationStore{activeVehicles: []models.Vehicle{{ID: "v1", Name: "Zoe"}}}
	svc := NewNotificationService(store)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := svc.CheckAllVehicles(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("CheckAllVehicles = %v, want context.Canceled", err)
	}
	if len(store.checkedOdometers) != 0 {
		t.Fatalf("no vehicle must be evaluated after the context ended, got %v", store.checkedOdometers)
	}
}
