package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

func validComparisonRequest() SaveComparisonRequest {
	vid := "veh-1"
	return SaveComparisonRequest{
		VehicleID: &vid,
		Name:      "  Vs SUV essence ",
		Currency:  "EUR",
		Mode:      models.ComparisonModeRetrospective,
		AnnualKm:  12000,
		Years:     5,
		ICE: models.ICEInputs{
			FuelType: "SP95_E10", LPer100Km: 6.5, FuelPrice: 1.75,
			PurchasePrice: money.FromFloat(25000), ResaleValue: money.FromFloat(10000),
			MaintenanceYearly: money.FromFloat(700), InsuranceYearly: money.FromFloat(650),
		},
	}
}

func TestValidateComparisonRequest(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(r *SaveComparisonRequest)
		wantErr string
	}{
		{"valid retrospective", func(r *SaveComparisonRequest) {}, ""},
		{"empty name", func(r *SaveComparisonRequest) { r.Name = "   " }, "comparison.name_invalid"},
		{"long name", func(r *SaveComparisonRequest) { r.Name = strings.Repeat("a", 101) }, "comparison.name_invalid"},
		{"unknown mode", func(r *SaveComparisonRequest) { r.Mode = "OTHER" }, "comparison.mode_invalid"},
		{"zero km", func(r *SaveComparisonRequest) { r.AnnualKm = 0 }, "comparison.annual_km"},
		{"too many km", func(r *SaveComparisonRequest) { r.AnnualKm = 500_000 }, "comparison.annual_km"},
		{"zero years", func(r *SaveComparisonRequest) { r.Years = 0 }, "comparison.years"},
		{"too many years", func(r *SaveComparisonRequest) { r.Years = 16 }, "comparison.years"},
		{"unknown fuel", func(r *SaveComparisonRequest) { r.ICE.FuelType = "KEROSENE" }, "fuel.type_invalid"},
		{"zero consumption", func(r *SaveComparisonRequest) { r.ICE.LPer100Km = 0 }, "comparison.ice_consumption"},
		{"negative fuel price", func(r *SaveComparisonRequest) { r.ICE.FuelPrice = -1 }, "comparison.fuel_price"},
		{"negative purchase", func(r *SaveComparisonRequest) { r.ICE.PurchasePrice = -1 }, "expense.amount_positive"},
		{"inflation out of range", func(r *SaveComparisonRequest) { r.Options.FuelInflationPct = 50 }, "comparison.inflation"},
		{"retrospective without vehicle", func(r *SaveComparisonRequest) { r.VehicleID = nil }, "comparison.vehicle_required"},
		{"projection without EV", func(r *SaveComparisonRequest) { r.Mode = models.ComparisonModeProjection }, "comparison.ev_required"},
		{"projection bad EV consumption", func(r *SaveComparisonRequest) {
			r.Mode = models.ComparisonModeProjection
			r.Tracked = &models.TrackedInputs{KwhPer100Km: 0, EurPerKwh: 0.2}
		}, "comparison.ev_consumption"},
		{"projection bad electricity price", func(r *SaveComparisonRequest) {
			r.Mode = models.ComparisonModeProjection
			r.Tracked = &models.TrackedInputs{KwhPer100Km: 16, EurPerKwh: money.MaxRate.Float() + 1}
		}, "comparison.electricity_price"},
		{"projection negative EV amount", func(r *SaveComparisonRequest) {
			r.Mode = models.ComparisonModeProjection
			r.Tracked = &models.TrackedInputs{KwhPer100Km: 16, EurPerKwh: 0.2, PurchasePrice: -5}
		}, "expense.amount_positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validComparisonRequest()
			tt.mutate(&req)
			err := validateComparisonRequest(&req)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || errorCode(err) != tt.wantErr {
				t.Fatalf("error = %v, want code %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateComparisonRequestNormalizesLinks(t *testing.T) {
	req := validComparisonRequest()
	req.Tracked = &models.TrackedInputs{KwhPer100Km: 16, EurPerKwh: 0.2}
	if err := validateComparisonRequest(&req); err != nil {
		t.Fatal(err)
	}
	if req.Name != "Vs SUV essence" || req.Tracked != nil {
		t.Errorf("retrospective must trim the name and drop EV inputs, got %q / %v", req.Name, req.Tracked)
	}

	proj := validComparisonRequest()
	proj.Mode = models.ComparisonModeProjection
	proj.Tracked = &models.TrackedInputs{KwhPer100Km: 16, EurPerKwh: 0.2, PurchasePrice: money.FromFloat(38000)}
	if err := validateComparisonRequest(&proj); err != nil {
		t.Fatal(err)
	}
	if proj.VehicleID != nil {
		t.Errorf("projection must not keep a vehicle, got %v", *proj.VehicleID)
	}
}

func TestComparisonServiceErrorsAreClientErrors(t *testing.T) {
	for name, err := range map[string]error{"needs EV": services.ErrComparisonNeedsEV, "needs vehicle": services.ErrComparisonNeedsVehicle} {
		rec := httptest.NewRecorder()
		writeRepoError(rec, httptest.NewRequest(http.MethodGet, "/", nil), err, "Failed to compute comparison")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, rec.Code)
		}
	}
}

func TestCreateTrackedHybridComparisonAccess(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	owner, err := repo.CreateUser(ctx, "hybrid-compare-owner@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	outsider, err := repo.CreateUser(ctx, "hybrid-compare-other@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	h := NewComparisonHandler(repo, services.NewComparisonService(services.NewTCOService(repo.Pool(), "UTC")))
	for _, powertrain := range []string{models.PowertrainPHEV, models.PowertrainREEV, models.PowertrainICE} {
		v := &models.Vehicle{UserID: owner.ID, Name: powertrain, Powertrain: powertrain, TeslaMateAuthType: models.AuthModeNone}
		if err := repo.CreateVehicle(ctx, v); err != nil {
			t.Fatal(err)
		}
		payload := validComparisonRequest()
		payload.VehicleID = &v.ID
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		for _, userID := range []string{owner.ID, outsider.ID} {
			req := httptest.NewRequest(http.MethodPost, "/api/comparisons", bytes.NewReader(body))
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
			rec := httptest.NewRecorder()
			h.Create(rec, req)
			if userID == outsider.ID {
				if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
					t.Fatalf("non-member status = %d: %s", rec.Code, rec.Body)
				}
			} else {
				want := http.StatusCreated
				if powertrain == models.PowertrainICE {
					want = http.StatusBadRequest
				}
				if rec.Code != want {
					t.Fatalf("%s: status %d, want %d: %s", powertrain, rec.Code, want, rec.Body)
				}
			}
		}
	}
}

func previewRequest(t *testing.T, h *ComparisonHandler, payload SaveComparisonRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/comparison-scenarios/preview", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()
	h.Preview(rec, req)
	return rec
}

func projectionPreviewRequest() SaveComparisonRequest {
	req := validComparisonRequest()
	req.VehicleID = nil
	req.Name = ""
	req.Mode = models.ComparisonModeProjection
	req.Tracked = &models.TrackedInputs{KwhPer100Km: 16, EurPerKwh: 0.2, PurchasePrice: money.FromFloat(38000)}
	return req
}

func TestPreviewComparisonDoesNotNeedASavedScenario(t *testing.T) {
	h := NewComparisonHandler(nil, services.NewComparisonService(nil))

	base := previewRequest(t, h, projectionPreviewRequest())
	if base.Code != http.StatusOK {
		t.Fatalf("status %d: %s", base.Code, base.Body)
	}
	var baseRes services.ComparisonResult
	if err := json.Unmarshal(base.Body.Bytes(), &baseRes); err != nil {
		t.Fatal(err)
	}

	// More kilometres and a dearer fuel widen the saving of the electric vehicle.
	explored := projectionPreviewRequest()
	explored.AnnualKm = 20000
	explored.ICE.FuelPrice = 2.2
	rec := previewRequest(t, h, explored)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var res services.ComparisonResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.AnnualKm != 20000 || res.TrackedSavings <= baseRes.TrackedSavings {
		t.Errorf("explored savings %v with %v km, want more than %v", res.TrackedSavings, res.AnnualKm, baseRes.TrackedSavings)
	}
}

func TestPreviewComparisonRefusesOtherModesAndInvalidInputs(t *testing.T) {
	h := NewComparisonHandler(nil, services.NewComparisonService(nil))

	retro := validComparisonRequest()
	if rec := previewRequest(t, h, retro); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "comparison.preview_projection_only") {
		t.Errorf("retrospective: status %d: %s", rec.Code, rec.Body)
	}

	for name, mutate := range map[string]func(r *SaveComparisonRequest){
		"fuel price":     func(r *SaveComparisonRequest) { r.ICE.FuelPrice = -1 },
		"electricity":    func(r *SaveComparisonRequest) { r.Tracked.EurPerKwh = money.MaxRate.Float() + 1 },
		"annual mileage": func(r *SaveComparisonRequest) { r.AnnualKm = 0 },
	} {
		req := projectionPreviewRequest()
		mutate(&req)
		if rec := previewRequest(t, h, req); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, rec.Code)
		}
	}
}
