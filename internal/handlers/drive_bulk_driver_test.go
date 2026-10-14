package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

func TestUpdateDrivesDriverAssignsAndClearsInOneCall(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "bulk-driver@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	kwh100 := 20.0
	v := &models.Vehicle{UserID: u.ID, Name: "EV", TeslaMateAuthType: models.AuthModeNone, EstimatedKwh100km: &kwh100}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	other := &models.Vehicle{UserID: u.ID, Name: "Other", TeslaMateAuthType: models.AuthModeNone, EstimatedKwh100km: &kwh100}
	if err := repo.CreateVehicle(ctx, other); err != nil {
		t.Fatal(err)
	}
	// The first person of a vehicle becomes its default driver, which new drives inherit.
	if _, err := repo.CreateVehiclePerson(ctx, v.ID, "Default"); err != nil {
		t.Fatal(err)
	}
	person, err := repo.CreateVehiclePerson(ctx, v.ID, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := repo.CreateVehiclePerson(ctx, other.ID, "Bob")
	if err != nil {
		t.Fatal(err)
	}

	h := NewDriveHandler(repo, nil, nil)
	router := chi.NewRouter()
	router.Post("/vehicles/{vehicleId}/drives", h.Create)
	router.Put("/vehicles/{vehicleId}/drives/driver", h.UpdateDrivesDriver)
	send := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, u.ID))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	create := func(day string) string {
		t.Helper()
		rec := send(http.MethodPost, "/vehicles/"+v.ID+"/drives", `{"start_time":"2026-09-`+day+`T08:00:00Z","distance_km":10}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create: status %d, body %s", rec.Code, rec.Body.String())
		}
		var d models.Drive
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		return d.ID
	}
	driverOf := func(id string) *string {
		t.Helper()
		var got *string
		if err := repo.Pool().QueryRow(ctx, `SELECT driver_id::text FROM drives WHERE id::text = $1`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	body := func(ids []string, driver any) string {
		b, _ := json.Marshal(map[string]any{"drive_ids": ids, "driver_id": driver})
		return string(b)
	}

	a, b, untouched := create("01"), create("02"), create("03")
	path := "/vehicles/" + v.ID + "/drives/driver"
	untouchedBefore := driverOf(untouched)

	if rec := send(http.MethodPut, path, body([]string{a, b}, person.ID)); rec.Code != http.StatusOK {
		t.Fatalf("assign: status %d, body %s", rec.Code, rec.Body.String())
	} else {
		var out struct{ Updated int64 }
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		if out.Updated != 2 {
			t.Errorf("updated = %d, want 2", out.Updated)
		}
	}
	for _, id := range []string{a, b} {
		if got := driverOf(id); got == nil || *got != person.ID {
			t.Errorf("drive %s driver = %v, want %s", id, got, person.ID)
		}
	}
	if got := driverOf(untouched); (got == nil) != (untouchedBefore == nil) || (got != nil && *got != *untouchedBefore) {
		t.Errorf("unselected drive driver changed: %v -> %v", untouchedBefore, got)
	}

	if rec := send(http.MethodPut, path, body([]string{a}, nil)); rec.Code != http.StatusOK {
		t.Fatalf("clear: status %d, body %s", rec.Code, rec.Body.String())
	}
	if got := driverOf(a); got != nil {
		t.Errorf("cleared drive still has driver %v", *got)
	}
	if got := driverOf(b); got == nil {
		t.Error("clearing one drive must not touch the other")
	}

	if rec := send(http.MethodPut, path, body([]string{a}, stranger.ID)); rec.Code != http.StatusBadRequest {
		t.Errorf("driver of another vehicle: status %d, want 400", rec.Code)
	}
	if rec := send(http.MethodPut, path, body(nil, person.ID)); rec.Code != http.StatusBadRequest {
		t.Errorf("empty selection: status %d, want 400", rec.Code)
	}
	if rec := send(http.MethodPut, path, `{not json`); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid body: status %d, want 400", rec.Code)
	}
	tooMany := make([]string, MaxBulkDriverDrives+1)
	for i := range tooMany {
		tooMany[i] = a
	}
	if rec := send(http.MethodPut, path, body(tooMany, person.ID)); rec.Code != http.StatusBadRequest {
		t.Errorf("over the limit: status %d, want 400", rec.Code)
	}

	// An empty driver_id clears the driver like null does.
	if rec := send(http.MethodPut, path, body([]string{b}, "")); rec.Code != http.StatusOK {
		t.Fatalf("empty driver_id: status %d, body %s", rec.Code, rec.Body.String())
	}
	if got := driverOf(b); got != nil {
		t.Errorf("empty driver_id kept driver %v", *got)
	}

	// Drives of another vehicle are not touched even when their id is sent.
	foreign := &models.Vehicle{UserID: u.ID, Name: "Foreign", TeslaMateAuthType: models.AuthModeNone, EstimatedKwh100km: &kwh100}
	if err := repo.CreateVehicle(ctx, foreign); err != nil {
		t.Fatal(err)
	}
	rec := send(http.MethodPut, "/vehicles/"+foreign.ID+"/drives/driver", body([]string{untouched}, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("foreign ids: status %d, body %s", rec.Code, rec.Body.String())
	}
	var out struct{ Updated int64 }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Updated != 0 {
		t.Errorf("foreign ids updated %d drives, want 0", out.Updated)
	}
}
