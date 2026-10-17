package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateEstimatedEnergyValidation(t *testing.T) {
	h := &VehicleHandler{}

	cases := []struct {
		name       string
		body       SaveEstimatedEnergyRequest
		wantStatus int
	}{
		{
			name: "negative consumption",
			body: func() SaveEstimatedEnergyRequest {
				c := -10.0
				return SaveEstimatedEnergyRequest{EstimatedKwh100km: &c}
			}(),
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "excessive consumption",
			body: func() SaveEstimatedEnergyRequest {
				c := 150.0
				return SaveEstimatedEnergyRequest{EstimatedKwh100km: &c}
			}(),
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "negative tariff",
			body: func() SaveEstimatedEnergyRequest {
				p := -0.22
				return SaveEstimatedEnergyRequest{EstimatedPricePerKwh: &p}
			}(),
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "free supply",
			body: func() SaveEstimatedEnergyRequest {
				p := 0.0
				return SaveEstimatedEnergyRequest{EstimatedPricePerKwh: &p}
			}(),
			wantStatus: http.StatusOK,
		},
		{
			name: "excessive tariff",
			body: func() SaveEstimatedEnergyRequest {
				p := 1_000_000.0
				return SaveEstimatedEnergyRequest{EstimatedPricePerKwh: &p}
			}(),
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPut, "/api/vehicles/123/estimated-energy", bytes.NewReader(b))
			w := httptest.NewRecorder()
			h.UpdateEstimatedEnergy(w, req)
			if w.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d (body: %s)", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}
