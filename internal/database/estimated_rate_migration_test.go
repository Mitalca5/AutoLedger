package database

import (
	"testing"

	"github.com/teslacost/teslacost/migrations"
)

func TestEstimatedRateMigrationPreservesPrecisionOnDowngrade(t *testing.T) {
	repo, ctx, owner, _, vehicle := vehicleRepoFixture(t, "rate-migration")
	down, err := migrations.FS.ReadFile("000066_estimated_energy_rate_precision.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, price := range []float64{1400, 0.251234} {
		if err := repo.UpdateVehicleEstimatedEnergy(ctx, vehicle.ID, owner.ID, nil, &price); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.pool.Exec(ctx, string(down)); err == nil {
			t.Fatalf("downgrade must reject %v", price)
		}
		got, err := repo.GetVehicleByID(ctx, vehicle.ID, owner.ID)
		if err != nil || got.EstimatedPricePerKwh == nil || *got.EstimatedPricePerKwh != price {
			t.Fatalf("failed downgrade lost %v: %+v, %v", price, got, err)
		}
	}
	price := 0.2512
	if err := repo.UpdateVehicleEstimatedEnergy(ctx, vehicle.ID, owner.ID, nil, &price); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, string(down)); err != nil {
		t.Fatalf("representable price: %v", err)
	}
	up, err := migrations.FS.ReadFile("000066_estimated_energy_rate_precision.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetVehicleByID(ctx, vehicle.ID, owner.ID)
	if err != nil || *got.EstimatedPricePerKwh != price {
		t.Fatalf("migration round trip: %+v, %v", got, err)
	}
}
