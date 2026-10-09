package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

func TestTariffCurrencyDoesNotRelabelAutomaticCharges(t *testing.T) {
	a := newVehicleAPI(t, "tariff-currency")
	ctx := context.Background()
	rate := money.Rate(150_000_000)
	ars := &models.TariffPlan{UserID: a.ownerID, Name: "Home", Currency: "ARS", PlanType: models.TariffTypeFlat, FlatRateCents: &rate, IsDefault: true}
	if err := a.repo.CreateTariffPlan(ctx, ars); err != nil {
		t.Fatal(err)
	}
	body := `{"name":"Hybrid","powertrain":"PHEV","currency":"USD"}`
	created := decodeObj(t, a.do(a.ownerID, 201, "POST", "/", body))
	vid := created["id"].(string)
	rejected := decodeObj(t, a.do(a.ownerID, 400, "POST", "/", `{"name":"Hybrid","currency":"USD","tariff_plan_id":"`+ars.ID+`"}`))
	if rejected["code"] != "tariff.currency_mismatch" {
		t.Fatalf("rejected: %v", rejected)
	}
	a.do(a.ownerID, 400, "PUT", "/"+vid, `{"tariff_plan_id":"`+ars.ID+`"}`)
	a.do(a.other, 404, "POST", "/", `{"name":"Other","currency":"ARS","tariff_plan_id":"`+ars.ID+`"}`)

	service := services.NewTariffService()
	ha := NewHomeAssistantHandler(a.repo, service)
	a.router.Post("/event", ha.HandleEvent)
	calc := NewTariffHandler(a.repo, service)
	a.router.Post("/calculate", calc.Calculate)
	window := `"start_time":"2026-10-05T10:00:00Z","end_time":"2026-10-05T12:00:00Z"`
	result := decodeObj(t, a.do(a.ownerID, 200, "POST", "/calculate", `{"vehicle_id":"`+vid+`",`+window+`,"kwh":20}`))
	if result["plan"] != nil {
		t.Fatalf("foreign-currency tariff applied: %v", result)
	}
	a.do(a.ownerID, 201, "POST", "/event", `{"vehicle_id":"`+vid+`","event_id":"foreign-plan","data":{`+window+`,"energy_kwh":20}}`)
	charges, _, err := a.repo.ListCharges(ctx, vid, false, 10, 0)
	if err != nil || len(charges) != 1 || charges[0].Cost != nil || charges[0].Currency != "USD" {
		t.Fatalf("charges: %+v, %v", charges, err)
	}

	usdRate := money.Rate(250000)
	usd := &models.TariffPlan{UserID: a.ownerID, Name: "Home", Currency: "USD", PlanType: models.TariffTypeFlat, FlatRateCents: &usdRate}
	if err := a.repo.CreateTariffPlan(ctx, usd); err != nil {
		t.Fatal(err)
	}
	// Same-name versions in another currency must not shadow the eligible one.
	from := "2026-01-01"
	ars.ValidFrom = &from
	if err := a.repo.UpdateTariffPlan(ctx, ars); err != nil {
		t.Fatal(err)
	}
	plan, err := a.repo.GetVehicleTariffPlanAt(ctx, vid, "2026-10-05")
	if err != nil || plan.ID != usd.ID {
		t.Fatalf("plan=%+v, err=%v", plan, err)
	}
	a.do(a.ownerID, 200, "PUT", "/"+vid, `{"tariff_plan_id":"`+usd.ID+`"}`)
	result = decodeObj(t, a.do(a.ownerID, 200, "POST", "/calculate", `{"vehicle_id":"`+vid+`",`+window+`,"kwh":20}`))
	if result["cost"] != float64(5) {
		t.Fatalf("same-currency calculation: %v", result)
	}
	// A later currency edit of an assigned plan also stops automatic pricing.
	usd.Currency = "ARS"
	if err := a.repo.UpdateTariffPlan(ctx, usd); err != nil {
		t.Fatal(err)
	}
	if _, err := a.repo.GetVehicleTariffPlan(ctx, vid); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("changed plan: %v", err)
	}
}
