package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestChargingPlaces(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	owner, err := repo.CreateUser(ctx, "places-owner@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := repo.CreateUser(ctx, "places-stranger@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	ev := &models.Vehicle{UserID: owner.ID, Name: "EV", Powertrain: models.PowertrainEV, TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, ev); err != nil {
		t.Fatal(err)
	}
	own := &models.Vehicle{UserID: stranger.ID, Name: "Other EV", Powertrain: models.PowertrainEV, TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, own); err != nil {
		t.Fatal(err)
	}

	places := NewChargingPlaceHandler(repo)
	router := chi.NewRouter()
	router.Get("/vehicles/{vehicleId}/charging-places", places.List)
	router.Post("/vehicles/{vehicleId}/charging-places", places.Create)
	router.Put("/vehicles/{vehicleId}/charging-places/aliases", places.SetAlias)
	router.Delete("/vehicles/{vehicleId}/charging-places/{placeId}", places.Delete)
	router.Put("/vehicles/{vehicleId}/charging-places/{placeId}", places.Update)
	expense := NewExpenseHandler(repo, nil)
	router.Post("/vehicles/{vehicleId}/charges", expense.CreateManualCharge)
	router.Put("/vehicles/{vehicleId}/charges/{chargeId}", expense.UpdateCharge)
	call := func(userID, method, path, body string) (int, map[string]any) {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		return rec.Code, resp
	}

	day := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	cost := money.Cents(400)
	for i, addr := range []string{"Maison", "  maison ", "Valence Supercharger, Valence"} {
		a := addr
		c := &models.ChargeLog{VehicleID: ev.ID, Date: day.Add(time.Duration(i) * 24 * time.Hour), Address: &a, KwhAdded: 10, Cost: &cost, Currency: "EUR"}
		if err := repo.CreateManualCharge(ctx, c); err != nil {
			t.Fatal(err)
		}
	}

	base := "/vehicles/" + ev.ID + "/charging-places"
	status, home := call(owner.ID, http.MethodPost, base, `{"name":"Home","category":"HOME"}`)
	if status != http.StatusCreated {
		t.Fatalf("create: %d %v", status, home)
	}
	homeID := home["id"].(string)
	if status, _ := call(owner.ID, http.MethodPost, base, `{"name":"HOME","category":"WORK"}`); status != http.StatusConflict {
		t.Errorf("a name differing only by case must conflict, got %d", status)
	}
	if status, _ := call(owner.ID, http.MethodPost, base, `{"name":"Gym","category":"SPA"}`); status != http.StatusBadRequest {
		t.Errorf("an unknown category must be refused, got %d", status)
	}

	// One link covers both spellings of the same text
	if status, _ := call(owner.ID, http.MethodPut, base+"/aliases", `{"text":"MAISON","place_id":"`+homeID+`"}`); status != http.StatusOK {
		t.Fatalf("link: %d", status)
	}
	locations := func() map[string]any {
		_, listed := call(owner.ID, http.MethodGet, base, "")
		out := map[string]any{}
		for _, raw := range listed["locations"].([]any) {
			l := raw.(map[string]any)
			out[l["text"].(string)] = l
		}
		return out
	}
	loc := locations()
	if len(loc) != 2 {
		t.Fatalf("expected 2 distinct locations, got %v", loc)
	}
	linked := loc["Maison"].(map[string]any)
	if linked["place_id"] != homeID || linked["sessions"].(float64) != 2 {
		t.Errorf("both spellings resolve to the place: %v", linked)
	}
	if loc["Valence Supercharger, Valence"].(map[string]any)["place_id"] != nil {
		t.Errorf("an unlinked text stays unclassified: %v", loc)
	}

	// Another account sees neither the places nor can it link to them
	if status, _ := call(stranger.ID, http.MethodGet, base, ""); status != http.StatusNotFound {
		t.Errorf("a stranger must not read the vehicle's places, got %d", status)
	}
	strangerBase := "/vehicles/" + own.ID + "/charging-places"
	if status, _ := call(stranger.ID, http.MethodPut, strangerBase+"/aliases", `{"text":"Maison","place_id":"`+homeID+`"}`); status != http.StatusNotFound {
		t.Errorf("linking to a place of another account must fail, got %d", status)
	}
	if status, _ := call(stranger.ID, http.MethodDelete, strangerBase+"/"+homeID, ""); status != http.StatusNotFound {
		t.Errorf("deleting a place of another account must fail, got %d", status)
	}

	// Renaming and recategorising; the same rules as on creation apply
	if status, _ := call(owner.ID, http.MethodPut, base+"/"+homeID, `{"name":"Maison","category":"HOME"}`); status != http.StatusOK {
		t.Fatalf("rename: %d", status)
	}
	_, otherPlace := call(owner.ID, http.MethodPost, base, `{"name":"Garage","category":"OTHER"}`)
	if status, _ := call(owner.ID, http.MethodPut, base+"/"+otherPlace["id"].(string), `{"name":"MAISON","category":"OTHER"}`); status != http.StatusConflict {
		t.Errorf("renaming onto an existing name must conflict, got %d", status)
	}
	if status, _ := call(owner.ID, http.MethodPut, base+"/"+homeID, `{"name":"Maison","category":"SPA"}`); status != http.StatusBadRequest {
		t.Errorf("an unknown category must be refused on update, got %d", status)
	}
	if status, _ := call(owner.ID, http.MethodPut, base+"/"+homeID, `{"name":"  ","category":"HOME"}`); status != http.StatusBadRequest {
		t.Errorf("a blank name must be refused, got %d", status)
	}
	if status, _ := call(owner.ID, http.MethodPut, base+"/"+homeID, `{not json`); status != http.StatusBadRequest {
		t.Errorf("a malformed body must be refused, got %d", status)
	}
	if status, _ := call(stranger.ID, http.MethodPut, strangerBase+"/"+homeID, `{"name":"Stolen","category":"HOME"}`); status != http.StatusNotFound {
		t.Errorf("updating a place of another account must fail, got %d", status)
	}
	if status, _ := call(owner.ID, http.MethodPut, base+"/aliases", `{"text":"   ","place_id":"`+homeID+`"}`); status != http.StatusBadRequest {
		t.Errorf("a blank location text must be refused, got %d", status)
	}

	// A new charge can name a place of its owner too, never another account's
	newCharge := func(placeID string) int {
		status, _ := call(owner.ID, http.MethodPost, "/vehicles/"+ev.ID+"/charges", `{"date":"2026-03-10T08:00:00Z","kwh_added":12,"cost":3,"currency":"EUR","place_id":"`+placeID+`"}`)
		return status
	}

	// A single charge can point at a place of its owner, never at another account's
	listed, _, err := repo.ListCharges(ctx, ev.ID, false, 10, 0)
	if err != nil || len(listed) == 0 {
		t.Fatalf("charges: %v %d", err, len(listed))
	}
	chargePath := "/vehicles/" + ev.ID + "/charges/" + listed[0].ID
	update := func(placeID string) int {
		status, _ := call(owner.ID, http.MethodPut, chargePath, `{"date":"2026-03-01T08:00:00Z","kwh_added":10,"cost":4,"currency":"EUR","place_id":"`+placeID+`"}`)
		return status
	}
	foreign, err := repo.CreateChargingPlace(ctx, stranger.ID, "Foreign", models.PlaceHome)
	if err != nil {
		t.Fatal(err)
	}
	if status := update(foreign.ID); status != http.StatusBadRequest {
		t.Errorf("a place of another account must be refused, got %d", status)
	}
	if status := update(homeID); status != http.StatusOK {
		t.Fatalf("own place: %d", status)
	}
	if status := newCharge(foreign.ID); status != http.StatusBadRequest {
		t.Errorf("a new charge at another account's place must be refused, got %d", status)
	}
	if status := newCharge(homeID); status != http.StatusCreated {
		t.Errorf("a new charge at the owner's place: got %d", status)
	}
	if got, _, _ := repo.ListCharges(ctx, ev.ID, false, 10, 0); got[0].PlaceID == nil && got[1].PlaceID == nil && got[2].PlaceID == nil {
		t.Errorf("the chosen place must be stored on the charge")
	}

	// Unlinking and deleting
	if status, _ := call(owner.ID, http.MethodPut, base+"/aliases", `{"text":"maison","place_id":null}`); status != http.StatusOK {
		t.Fatalf("unlink: %d", status)
	}
	if loc := locations(); loc["Maison"].(map[string]any)["place_id"] != nil {
		t.Errorf("unlinked text must be unclassified again: %v", loc)
	}
	if status, _ := call(owner.ID, http.MethodDelete, base+"/"+homeID, ""); status != http.StatusOK {
		t.Fatalf("delete: %d", status)
	}
}
