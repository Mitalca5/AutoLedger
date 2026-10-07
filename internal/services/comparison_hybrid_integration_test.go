package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

func TestIntegrationHybridInsuranceIndependentOfMileage(t *testing.T) {
	for _, powertrain := range []string{models.PowertrainPHEV, models.PowertrainREEV} {
		t.Run(powertrain, func(t *testing.T) {
			db, repo := setupIntegrationDB(t, false)
			ctx := context.Background()
			v := mustVehicle(t, repo, "hybrid-insurance@example.com")
			v.Powertrain = powertrain
			if err := repo.UpdateVehicle(ctx, v); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			mustDrive(t, repo, v.ID, 1, now.AddDate(0, 0, -10), 10000, 988)
			year := 12
			premium := &models.MaintenanceExpense{VehicleID: v.ID, Category: "INSURANCE", Currency: "EUR", Amount: eur(1200), Date: now.AddDate(0, 0, -5), IsRecurring: true, RecurrenceIntervalMonths: &year}
			if err := repo.CreateMaintenanceExpense(ctx, premium); err != nil {
				t.Fatal(err)
			}
			svc := NewComparisonService(NewTCOService(db.Pool, "UTC"))
			for _, km := range []float64{5000, 20000, 40000} {
				sc := &models.ComparisonScenario{Mode: models.ComparisonModeRetrospective, VehicleID: &v.ID, AnnualKm: km, Years: 5, ICE: models.ICEInputs{InsuranceYearly: eur(1200)}}
				result, err := svc.Compare(ctx, sc)
				if err != nil {
					t.Fatal(err)
				}
				if result.Tracked.Insurance != eur(6000) || result.ICE.Insurance != eur(6000) {
					t.Fatalf("km=%v: hybrid=%v combustion=%v, want 6000 each", km, result.Tracked.Insurance, result.ICE.Insurance)
				}
			}
		})
	}
}
