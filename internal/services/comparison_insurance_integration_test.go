package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

func TestIntegrationComparisonAnnualInsurance(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "annual-insurance@example.com")
	svc := NewComparisonService(NewTCOService(db.Pool, "UTC"))
	at := time.Now().UTC().Truncate(24 * time.Hour)
	month, quarter, year, half := 1, 3, 12, 6
	ended := at.AddDate(0, 0, -1)
	fx := 0.5
	base := models.MaintenanceExpense{VehicleID: v.ID, Category: "INSURANCE", Currency: "EUR", Date: at.AddDate(0, 0, -10), Amount: eur(1200)}
	tests := []struct {
		name      string
		edit      func(*models.MaintenanceExpense)
		want      float64
		estimated bool
	}{
		{"annual", func(m *models.MaintenanceExpense) { m.IsRecurring = true; m.RecurrenceIntervalMonths = &year }, 1200, false},
		{"monthly", func(m *models.MaintenanceExpense) {
			m.Amount = eur(100)
			m.IsRecurring = true
			m.RecurrenceIntervalMonths = &month
		}, 1200, false},
		{"quarterly", func(m *models.MaintenanceExpense) {
			m.Amount = eur(300)
			m.IsRecurring = true
			m.RecurrenceIntervalMonths = &quarter
		}, 1200, false},
		{"explicit coverage", func(m *models.MaintenanceExpense) { m.Amount = eur(600); m.CoverageMonths = &half }, 1200, false},
		{"cash fallback short history", func(m *models.MaintenanceExpense) {}, 1200, true},
		{"old cash excluded", func(m *models.MaintenanceExpense) { m.Date = at.AddDate(0, -13, 0) }, 0, false},
		{"expired coverage excluded", func(m *models.MaintenanceExpense) { m.Date = at.AddDate(0, -7, 0); m.CoverageMonths = &half }, 0, false},
		{"expired recurring excluded", func(m *models.MaintenanceExpense) {
			m.IsRecurring = true
			m.RecurrenceIntervalMonths = &year
			m.RecurrenceEndDate = &ended
		}, 0, false},
		{"future excluded", func(m *models.MaintenanceExpense) {
			m.Date = at.AddDate(0, 0, 1)
			m.IsRecurring = true
			m.RecurrenceIntervalMonths = &year
		}, 0, false},
		{"currency converted", func(m *models.MaintenanceExpense) {
			m.Amount = eur(200)
			m.Currency = "USD"
			m.FxRate = &fx
			m.IsRecurring = true
			m.RecurrenceIntervalMonths = &month
		}, 1200, false},
		{"missing exchange rate excluded", func(m *models.MaintenanceExpense) { m.Currency = "USD" }, 0, false},
		{"invalid recurrence uses cash fallback", func(m *models.MaintenanceExpense) { m.IsRecurring = true }, 1200, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := db.Pool.Exec(ctx, `DELETE FROM maintenance_expenses WHERE vehicle_id = $1`, v.ID); err != nil {
				t.Fatal(err)
			}
			m := base
			tt.edit(&m)
			if err := repo.CreateMaintenanceExpense(ctx, &m); err != nil {
				t.Fatal(err)
			}
			got, estimated, err := svc.annualInsurance(ctx, v.ID, at)
			if err != nil {
				t.Fatal(err)
			}
			if got != eur(tt.want) || estimated != tt.estimated {
				t.Fatalf("annual=%v estimated=%v, want %v/%v", got, estimated, eur(tt.want), tt.estimated)
			}
		})
	}
}

func TestIntegrationComparisonInsuranceIndependentOfMileage(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "insurance-mileage@example.com")
	now := time.Now().UTC()
	// A recently paid annual premium must not be extrapolated from this short mileage history.
	mustDrive(t, repo, v.ID, 1, now.AddDate(0, 0, -10), 10000, 988)
	year := 12
	premium := &models.MaintenanceExpense{VehicleID: v.ID, Category: "INSURANCE", Currency: "EUR", Amount: eur(1200), Date: now.AddDate(0, 0, -5), IsRecurring: true, RecurrenceIntervalMonths: &year}
	if err := repo.CreateMaintenanceExpense(ctx, premium); err != nil {
		t.Fatal(err)
	}
	tco := NewTCOService(db.Pool, "UTC")
	svc := NewComparisonService(tco)
	before, err := tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, inflation := range []float64{0, 10} {
		want := eur(6000)
		if inflation == 10 {
			want = eur(7326.12)
		}
		for _, km := range []float64{5000, 20000, 40000} {
			sc := &models.ComparisonScenario{Mode: models.ComparisonModeRetrospective, VehicleID: &v.ID, AnnualKm: km, Years: 5, ICE: models.ICEInputs{InsuranceYearly: eur(1200)}, Options: models.ScenarioOptions{CostInflationPct: inflation}}
			res, err := svc.Compare(ctx, sc)
			if err != nil {
				t.Fatal(err)
			}
			if res.EV.Insurance != want || res.ICE.Insurance != want {
				t.Fatalf("km=%v inflation=%v: reference=%v combustion=%v want=%v", km, inflation, res.EV.Insurance, res.ICE.Insurance, want)
			}
		}
	}
	after, err := tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.InsuranceCost != after.InsuranceCost || before.TotalCost != after.TotalCost {
		t.Fatal("comparison altered recorded costs")
	}
}
