package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestValidateContributionQuery(t *testing.T) {
	tests := []struct {
		month, category string
		want            error
	}{
		{"2026-03", ContributionEnergy, nil},
		{"2026-12", ContributionInsurance, nil},
		{"2026-13", ContributionEnergy, ErrInvalidContributionMonth},
		{"2026-3", ContributionEnergy, ErrInvalidContributionMonth},
		{"", ContributionEnergy, ErrInvalidContributionMonth},
		{"2026-03", "TIRES", ErrUnsupportedContributionCategory},
		{"2026-03", "", ErrUnsupportedContributionCategory},
	}
	for _, tt := range tests {
		if got := ValidateContributionQuery(tt.month, tt.category); !errors.Is(got, tt.want) {
			t.Errorf("ValidateContributionQuery(%q, %q) = %v, want %v", tt.month, tt.category, got, tt.want)
		}
	}
}

func TestAssembleContributionsEnergyIncludesEstimate(t *testing.T) {
	mc := MonthlyCost{Month: "2026-03", Energy: 5000, SmoothedEnergy: 1500, SmoothedKwh: 75, EstimatedEnergyKm: 450, Insurance: 9900}
	items := []MonthlyContributionItem{{SourceTable: "charge_logs", Amount: 3500}}

	got := assembleContributions("2026-03", ContributionEnergy, items, mc)
	if got.Total != 5000 {
		t.Fatalf("total = %d, want the month's energy total", got.Total)
	}
	if got.Estimate == nil || got.Estimate.Amount != 1500 || got.Estimate.Kwh != 75 || got.Estimate.Km != 450 {
		t.Fatalf("estimate = %+v", got.Estimate)
	}
	var sum money.Cents = got.Estimate.Amount
	for _, it := range got.Items {
		sum += it.Amount
	}
	if sum != got.Total {
		t.Fatalf("items + estimate = %d, total = %d", sum, got.Total)
	}
}

func TestAssembleContributionsInsuranceHasNoEstimate(t *testing.T) {
	mc := MonthlyCost{Energy: 5000, SmoothedEnergy: 1500, Insurance: 9900}
	got := assembleContributions("2026-03", ContributionInsurance, nil, mc)
	if got.Total != 9900 || got.Estimate != nil {
		t.Fatalf("got total %d estimate %+v", got.Total, got.Estimate)
	}
	if got.Items == nil {
		t.Fatal("items must serialize as an empty list, not null")
	}
}

func TestIntegrationMonthlyContributionsReconcileAndStayScoped(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	alice := mustVehicle(t, repo, "contrib-alice@example.com")
	bob := mustVehicle(t, repo, "contrib-bob@example.com")
	svc := NewTCOService(db.Pool, "UTC")

	monthly := 1
	start := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	for _, v := range []*models.Vehicle{alice, bob} {
		ins := &models.MaintenanceExpense{VehicleID: v.ID, Category: "INSURANCE", Currency: "EUR", Date: start,
			Amount: eur(100), IsRecurring: true, RecurrenceIntervalMonths: &monthly, Description: "Home policy", AmortizationMode: "NONE"}
		if err := repo.CreateMaintenanceExpense(ctx, ins); err != nil {
			t.Fatal(err)
		}
	}
	fuel := &models.FuelLog{VehicleID: alice.ID, Date: time.Date(2026, 3, 2, 8, 0, 0, 0, time.UTC), Amount: eur(60), IsFullTank: true}
	if err := repo.CreateFuelLog(ctx, fuel); err != nil {
		t.Fatal(err)
	}

	ins, err := svc.MonthlyCostContributions(ctx, alice.ID, "2026-03", ContributionInsurance)
	if err != nil {
		t.Fatal(err)
	}
	if len(ins.Items) != 1 || ins.Items[0].Amount != eur(100) || !ins.Items[0].Recurring || ins.Items[0].IntervalMonths != 1 || ins.Items[0].Label != "Home policy" {
		t.Fatalf("insurance items = %+v", ins.Items)
	}
	assertContributionsReconcile(t, ins)

	energy, err := svc.MonthlyCostContributions(ctx, alice.ID, "2026-03", ContributionEnergy)
	if err != nil {
		t.Fatal(err)
	}
	if len(energy.Items) != 1 || energy.Items[0].SourceTable != "fuel_logs" || energy.Items[0].SourceID != fuel.ID {
		t.Fatalf("energy items = %+v", energy.Items)
	}
	assertContributionsReconcile(t, energy)

	// The other vehicle's own entries never appear in alice's breakdown.
	other, err := svc.MonthlyCostContributions(ctx, bob.ID, "2026-03", ContributionEnergy)
	if err != nil {
		t.Fatal(err)
	}
	if len(other.Items) != 0 {
		t.Fatalf("bob has no energy entries, got %+v", other.Items)
	}
}

func assertContributionsReconcile(t *testing.T, c *MonthlyContributions) {
	t.Helper()
	var sum money.Cents
	for _, it := range c.Items {
		sum += it.Amount
	}
	if c.Estimate != nil {
		sum += c.Estimate.Amount
	}
	if sum != c.Total {
		t.Fatalf("%s %s: items sum to %d, month total is %d", c.Month, c.Category, sum, c.Total)
	}
}
