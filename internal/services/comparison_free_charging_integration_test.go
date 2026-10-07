package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestIntegrationComparisonFreeAndUnknownChargingCosts(t *testing.T) {
	for _, powertrain := range []string{models.PowertrainPHEV, models.PowertrainREEV} {
		t.Run(powertrain, func(t *testing.T) {
			db, repo := setupIntegrationDB(t, false)
			ctx := context.Background()
			v := mustVehicle(t, repo, "solar-charging@example.com")
			v.Powertrain = powertrain
			if err := repo.UpdateVehicle(ctx, v); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			mustDrive(t, repo, v.ID, 1, now.AddDate(0, 0, -10), 10000, 1000)
			liters := 35.0
			if err := repo.CreateFuelLog(ctx, &models.FuelLog{VehicleID: v.ID, Date: now.AddDate(0, 0, -5), Liters: &liters, Amount: eur(60), IsFullTank: true}); err != nil {
				t.Fatal(err)
			}
			tco := NewTCOService(db.Pool, "UTC")
			for _, tc := range []struct {
				name        string
				cost        *money.Cents
				wantUnknown bool
			}{
				{"solar free", new(money.Cents), false},
				{"unknown cost", nil, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					if _, err := db.Pool.Exec(ctx, `DELETE FROM charge_logs WHERE vehicle_id = $1`, v.ID); err != nil {
						t.Fatal(err)
					}
					note := "Own solar generation"
					if err := repo.CreateManualCharge(ctx, &models.ChargeLog{VehicleID: v.ID, Date: now.AddDate(0, 0, -3), KwhAdded: 50, Cost: tc.cost, Currency: "EUR", Notes: &note}); err != nil {
						t.Fatal(err)
					}
					sum, err := tco.ComputeVehicleTCO(ctx, v.ID)
					if err != nil {
						t.Fatal(err)
					}
					if sum.RecordedChargeCount != 1 || sum.TotalKwhAdded != 50 || sum.EnergyCost != eur(60) {
						t.Fatalf("unexpected energy totals: %+v", sum)
					}
					_, notes := evBaselineFromTCO(sum, 12000, 5, now)
					unknown := false
					for _, n := range notes {
						if n.Code == "comparison.assumption.hybrid_missing_energy_source" {
							t.Fatal("both energy sources are recorded")
						}
						if n.Code == "comparison.assumption.hybrid_unpriced_charges" {
							unknown = true
						}
					}
					if unknown != tc.wantUnknown {
						t.Fatalf("unknown cost=%v, want %v", unknown, tc.wantUnknown)
					}
				})
			}
		})
	}
}
