package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

func TestPendingChargeAssignment(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "pending-assign@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	ev1 := &models.Vehicle{UserID: u.ID, Name: "EV 1", TeslaMateAuthType: models.AuthModeNone}
	ev2 := &models.Vehicle{UserID: u.ID, Name: "EV 2", TeslaMateAuthType: models.AuthModeNone}
	for _, v := range []*models.Vehicle{ev1, ev2} {
		if err := repo.CreateVehicle(ctx, v); err != nil {
			t.Fatal(err)
		}
	}

	tariffs := services.NewTariffService()
	ha := NewHomeAssistantHandler(repo, tariffs)
	pending := NewPendingChargesHandler(repo, tariffs)
	router := chi.NewRouter()
	router.Post("/event", ha.HandleEvent)
	router.Post("/pending-charges/{id}/assign", pending.Assign)
	call := func(path, body string) (int, map[string]any) {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, u.ID))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		return rec.Code, resp
	}

	session := `{"event_id":"garage:2026-10-05T21:00:00Z","data":{"start_time":"2026-10-05T21:00:00Z","end_time":"2026-10-06T01:00:00Z",
		"energy_kwh":24,"cost":4.5,"soc_start":20,"soc_end":70,"charger_name":"Garage"}}`
	status, resp := call("/event", session)
	if status != http.StatusCreated || resp["status"] != "pending_qualification" {
		t.Fatalf("shared charger session: got %d %v", status, resp)
	}
	pendingID := resp["pending_id"].(string)

	// Two clicks at once: one charge
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i], _ = call("/pending-charges/"+pendingID+"/assign", `{"vehicle_id":"`+ev1.ID+`"}`)
		}(i)
	}
	wg.Wait()
	if !(codes[0] == http.StatusOK && codes[1] == http.StatusNotFound) && !(codes[0] == http.StatusNotFound && codes[1] == http.StatusOK) {
		t.Errorf("two simultaneous assignments: got %v, want one 200 and one 404", codes)
	}
	list, _, err := repo.ListCharges(ctx, ev1.ID, false, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("charges after the double assignment: %d, want 1", len(list))
	}
	c := list[0]
	if c.Cost == nil || *c.Cost != money.FromFloat(4.5) || c.KwhAdded != 24 {
		t.Errorf("assigned charge: cost %v, energy %v; want the 4.50 and 24 kWh sent by the integration", c.Cost, c.KwhAdded)
	}
	var socStart, socEnd *int
	var externalID *string
	if err := repo.Pool().QueryRow(ctx, `SELECT start_battery_level, end_battery_level, external_id FROM charge_logs WHERE id = $1`, c.ID).
		Scan(&socStart, &socEnd, &externalID); err != nil {
		t.Fatal(err)
	}
	if socStart == nil || *socStart != 20 || socEnd == nil || *socEnd != 70 || externalID == nil || *externalID != "garage:2026-10-05T21:00:00Z" {
		t.Errorf("assigned charge keeps SoC %v -> %v and event %v; want 20 -> 70 and the event_id", socStart, socEnd, externalID)
	}

	// The integration sends the same session again after the assignment: recognised, not pending again
	if status, resp := call("/event", session); status != http.StatusOK || resp["status"] != "duplicate" || resp["charge_id"] != c.ID {
		t.Errorf("session resent after assignment: got %d %v, want 200 duplicate of %s", status, resp, c.ID)
	}
	if left, _ := repo.ListPendingCharges(ctx, u.ID); len(left) != 0 {
		t.Errorf("pending charges left: %d, want 0", len(left))
	}
}
