package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teslacost/teslacost/internal/services"
)

// The browser sends currency units, and receives the same units in the breakdown.
func TestCalculatePublicJSONContract(t *testing.T) {
	h := NewTariffHandler(nil, services.NewTariffService())
	for _, tc := range []struct {
		name, payload                 string
		energy, duration, idle, total float64
		idleMinutes                   float64
	}{
		{"energy and connection", `{"kwh":20,"charging_minutes":0,"total_plugged_minutes":0,"connection_fee":1,"price_per_kwh":0.25,"price_per_minute":0,"idle_fee_per_minute":0,"idle_grace_minutes":0}`, 5, 0, 0, 6, 0},
		{"duration and idle grace", `{"kwh":20,"charging_minutes":60,"total_plugged_minutes":75,"connection_fee":1,"price_per_kwh":0.25,"price_per_minute":0.01,"idle_fee_per_minute":0.1,"idle_grace_minutes":5}`, 5, 0.6, 1, 7.6, 15},
		{"free charge", `{"kwh":20,"charging_minutes":0,"total_plugged_minutes":0,"connection_fee":0,"price_per_kwh":0,"price_per_minute":0,"idle_fee_per_minute":0,"idle_grace_minutes":0}`, 0, 0, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.CalculatePublic(rec, httptest.NewRequest(http.MethodPost, "/api/tariffs/calculate-public", bytes.NewBufferString(tc.payload)))
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			var result map[string]float64
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			for key, expected := range map[string]float64{"energy_cost": tc.energy, "duration_cost": tc.duration, "idle_cost": tc.idle, "total_cost": tc.total, "idle_minutes": tc.idleMinutes} {
				if result[key] != expected {
					t.Errorf("%s = %v, want %v", key, result[key], expected)
				}
			}
			if _, ok := result["connection_cost"]; !ok {
				t.Fatal("missing connection_cost")
			}
		})
	}
}
