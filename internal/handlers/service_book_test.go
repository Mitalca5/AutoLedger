package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

func TestServiceBookDownload(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	owner, err := repo.CreateUser(ctx, "book-owner@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := repo.CreateUser(ctx, "book-stranger@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	veh := &models.Vehicle{UserID: owner.ID, Name: "Book car", Powertrain: models.PowertrainICE, TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 30000}
	if err := repo.CreateVehicle(ctx, veh); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"MAINTENANCE", "INSURANCE"} {
		m := &models.MaintenanceExpense{VehicleID: veh.ID, Category: c, Amount: 9000, Currency: "EUR", Date: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Description: c}
		if err := repo.CreateMaintenanceExpense(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	h := NewServiceBookHandler(repo, services.NewServiceBookService(repo, func(string) ([]byte, error) { return nil, nil }))
	call := func(userID, query string) *httptest.ResponseRecorder {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vehicleId", veh.ID)
		req := httptest.NewRequest(http.MethodGet, "/?"+query, nil)
		req = req.WithContext(context.WithValue(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), middleware.UserIDKey, userID))
		rec := httptest.NewRecorder()
		h.Download(rec, req)
		return rec
	}

	rec := call(owner.ID, "attachments=1")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(rec.Body.String(), "%PDF-") {
		t.Fatalf("owner download: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "service-book") {
		t.Fatalf("disposition = %q", rec.Header().Get("Content-Disposition"))
	}
	if rec := call(owner.ID, "from=2026-13-45"); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad date: %d", rec.Code)
	}
	if rec := call(stranger.ID, ""); rec.Code == http.StatusOK {
		t.Fatal("a user without access must not get the book")
	}
}
