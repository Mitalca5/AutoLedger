package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

func TestIntegrationVehicleRoutesExposeOnlyTheIntegrationView(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	owner, err := repo.CreateUser(ctx, "ha-owner@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := repo.CreateUser(ctx, "ha-other@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	apiURL, apiKey := "http://teslamate:8080", "secret-ciphertext"
	mine := &models.Vehicle{UserID: owner.ID, Name: "Shared EV", Make: "Some make", Model: "Some model", CurrentOdometer: 12345.6,
		TeslaMateAPIURL: &apiURL, TeslaMateAPIKeyEncrypted: &apiKey, TeslaMateAuthType: models.AuthModeBearer}
	theirs := &models.Vehicle{UserID: other.ID, Name: "Not mine", TeslaMateAuthType: models.AuthModeNone}
	for _, v := range []*models.Vehicle{mine, theirs} {
		if err := repo.CreateVehicle(ctx, v); err != nil {
			t.Fatal(err)
		}
	}

	h := NewHomeAssistantHandler(repo, nil)
	router := chi.NewRouter()
	router.Get("/vehicles", h.ListVehicles)
	router.Get("/vehicles/{vehicleId}", h.GetVehicle)
	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, owner.ID))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	wantKeys := []string{"currency", "current_odometer", "id", "make", "model", "name", "powertrain"}
	keysOf := func(m map[string]any) []string {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}

	rec := get("/vehicles")
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("list: status %d, body %s", rec.Code, rec.Body.String())
	}
	if len(list) != 1 || list[0]["id"] != mine.ID {
		t.Fatalf("list: want only the account's vehicle, got %v", list)
	}
	if got := keysOf(list[0]); len(got) != len(wantKeys) || fmt.Sprint(got) != fmt.Sprint(wantKeys) {
		t.Errorf("list: fields %v, want exactly %v", got, wantKeys)
	}

	rec = get("/vehicles/" + mine.ID)
	var one map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &one); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("get: status %d, body %s", rec.Code, rec.Body.String())
	}
	if one["name"] != "Shared EV" || one["current_odometer"] != 12345.6 || fmt.Sprint(keysOf(one)) != fmt.Sprint(wantKeys) {
		t.Errorf("get: got %v", one)
	}

	if rec := get("/vehicles/" + theirs.ID); rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Errorf("another account's vehicle: status %d, want 404 or 403", rec.Code)
	}
}
