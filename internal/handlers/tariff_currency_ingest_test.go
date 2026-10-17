package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

func TestHomeAssistantDoesNotRelabelForeignTariff(t *testing.T) {
	env, owner := newHAEventEnv(t, "tariff-currency-ingest@example.com")
	env.h = NewHomeAssistantHandler(env.repo, services.NewTariffService())
	vehicle := env.vehicle(owner.ID, "Car", models.PowertrainEV, 0)
	rate := money.Rate(100_000_000)
	plan := &models.TariffPlan{UserID: owner.ID, Name: "Home", Currency: "ARS", PlanType: models.TariffTypeFlat, FlatRateCents: &rate, IsDefault: true}
	if err := env.repo.CreateTariffPlan(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	body := `{"vehicle_id":"` + vehicle.ID + `","event_id":"foreign-tariff","event_type":"charging_session_end","data":{"start_time":"2026-10-01T01:00:00Z","end_time":"2026-10-01T02:00:00Z","energy_added_kwh":20}}`
	if status, result := env.post(owner.ID, body); status != http.StatusCreated {
		t.Fatalf("ingest: %d %s", status, result)
	}
	charges, _, err := env.repo.ListCharges(context.Background(), vehicle.ID, false, 10, 0)
	if err != nil || len(charges) != 1 {
		t.Fatalf("charges: %v, %v", charges, err)
	}
	if charges[0].Cost != nil || charges[0].Currency != vehicle.Currency {
		t.Fatalf("foreign tariff was relabelled: %+v", charges[0])
	}
}
