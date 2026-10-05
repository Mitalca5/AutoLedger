package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

func TestResolveAddressesBackfill(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "address-backfill@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "EV", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	text := func(s string) *string { return &s }
	for i, addrs := range [][2]*string{
		{text("45.89920, 6.12940"), text("45.76400, 4.83570")},
		{text("Home"), text("45.89920, 6.12940")},
		{text("Home"), text("Work")},
	} {
		d := &models.Drive{
			VehicleID: v.ID, Origin: "WEBHOOK", StartTime: time.Date(2026, 5, 1+i, 8, 0, 0, 0, time.UTC),
			DistanceKm: 10, StartAddress: addrs[0], EndAddress: addrs[1], Tags: []string{}, IsManual: true,
		}
		if err := repo.CreateManualDrive(ctx, d); err != nil {
			t.Fatal(err)
		}
	}

	h := NewDriveHandler(repo, nil, nil)
	router := chi.NewRouter()
	router.Get("/vehicles/{vehicleId}/drives/address-backfill", h.AddressBackfillStatus)
	router.Post("/vehicles/{vehicleId}/drives/resolve-addresses", h.ResolveAddresses)
	call := func(method, path string) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, u.ID))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}
	base := "/vehicles/" + v.ID + "/drives/"

	if code, body := call(http.MethodGet, base+"address-backfill"); code != http.StatusOK || body["pending"] != float64(2) || body["geocoding_enabled"] != false {
		t.Fatalf("status without geocoder: %d %v", code, body)
	}
	if code, body := call(http.MethodPost, base+"resolve-addresses"); code != http.StatusConflict || body["code"] != "geocoding.disabled" {
		t.Fatalf("resolve without geocoder: %d %v", code, body)
	}

	h.SetGeocoder(&fakeGeocoder{answers: map[string]string{"45.8992,6.1294": "1 Rue Example, Annecy"}, calls: make(chan string, 16)})
	if code, body := call(http.MethodPost, base+"resolve-addresses"); code != http.StatusAccepted || body["queued"] != float64(2) {
		t.Fatalf("resolve: %d %v", code, body)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, body := call(http.MethodGet, base+"address-backfill"); body["running"] == false {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the backfill never finished")
		}
		time.Sleep(20 * time.Millisecond)
	}
	drives, _, err := repo.ListDrives(ctx, v.ID, database.DriveFilter{}, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int][2]string{}
	for _, d := range drives {
		got[d.StartTime.Day()] = [2]string{*d.StartAddress, *d.EndAddress}
	}
	want := map[int][2]string{
		1: {"1 Rue Example, Annecy", "45.76400, 4.83570"},
		2: {"Home", "1 Rue Example, Annecy"},
		3: {"Home", "Work"},
	}
	for day, w := range want {
		if got[day] != w {
			t.Errorf("drive of day %d: got %v, want %v", day, got[day], w)
		}
	}
	if _, body := call(http.MethodGet, base+"address-backfill"); body["pending"] != float64(1) {
		t.Errorf("an unresolved position stays pending: %v", body)
	}
}
