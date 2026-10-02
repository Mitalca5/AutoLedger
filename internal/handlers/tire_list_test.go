package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

func TestTireListIsAnEmptyArrayWithoutTires(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "tires-empty@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "Car", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}

	h := NewTireHandler(repo, services.NewTireWearService(repo))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("vehicleId", v.ID)
	req := httptest.NewRequest(http.MethodGet, "/api/vehicles/"+v.ID+"/tires", nil)
	req = req.WithContext(context.WithValue(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), middleware.UserIDKey, u.ID))
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("got %d %q, want 200 []", rec.Code, rec.Body.String())
	}
}
