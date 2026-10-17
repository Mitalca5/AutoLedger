package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestEstimatedEnergyRateAllWritePaths(t *testing.T) {
	a := newVehicleAPI(t, "rate-storage")
	raw := a.do(a.ownerID, http.StatusCreated, "POST", "/", `{"name":"Manual","currency":"ARS","estimated_price_per_kwh":1400.251234}`)
	var vehicle models.Vehicle
	if err := json.Unmarshal(raw, &vehicle); err != nil {
		t.Fatal(err)
	}
	path := "/" + vehicle.ID
	for _, tc := range []struct {
		path, body string
		want       float64
	}{
		{path, `{"name":"Renamed","estimated_price_per_kwh":1800.123456}`, 1800.123456},
		{path + "/estimated-energy", `{"estimated_price_per_kwh":999999.999999}`, 999999.999999},
		{path + "/estimated-energy", `{"estimated_price_per_kwh":0}`, 0},
	} {
		a.do(a.ownerID, http.StatusOK, "PUT", tc.path, tc.body)
		got, err := a.repo.GetVehicleByID(context.Background(), vehicle.ID, a.ownerID)
		if err != nil || got.EstimatedPricePerKwh == nil || *got.EstimatedPricePerKwh != tc.want {
			t.Fatalf("stored rate: %+v, %v", got, err)
		}
	}
	a.do(a.ownerID, http.StatusBadRequest, "POST", "/", `{"name":"Invalid","estimated_price_per_kwh":1000000}`)
	a.do(a.ownerID, http.StatusBadRequest, "PUT", path, `{"name":"Invalid","estimated_price_per_kwh":1000000}`)
	a.do(a.ownerID, http.StatusBadRequest, "PUT", path+"/estimated-energy", `{"estimated_price_per_kwh":1000000}`)
}
