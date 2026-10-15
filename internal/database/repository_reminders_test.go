package database

import (
	"context"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestListVehiclesWithActiveWebhook(t *testing.T) {
	repo, ctx, owner, _, enabled := vehicleRepoFixture(t, "webhook-list")
	if err := repo.UpdateVehicleOdometer(ctx, enabled.ID, 41250); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertVehicleWebhook(ctx, &models.VehicleWebhook{VehicleID: enabled.ID, URL: "https://hooks.example/a", Type: "GENERIC", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	disabled := &models.Vehicle{UserID: owner.ID, Name: "Disabled hook", Powertrain: models.PowertrainICE}
	blank := &models.Vehicle{UserID: owner.ID, Name: "Blank URL", Powertrain: models.PowertrainICE}
	none := &models.Vehicle{UserID: owner.ID, Name: "No hook", Powertrain: models.PowertrainICE}
	for _, v := range []*models.Vehicle{disabled, blank, none} {
		if err := repo.CreateVehicle(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.UpsertVehicleWebhook(ctx, &models.VehicleWebhook{VehicleID: disabled.ID, URL: "https://hooks.example/b", Type: "GENERIC", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertVehicleWebhook(ctx, &models.VehicleWebhook{VehicleID: blank.ID, URL: "  ", Type: "GENERIC", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	vehicles, err := repo.ListVehiclesWithActiveWebhook(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]models.Vehicle{}
	for _, v := range vehicles {
		byID[v.ID] = v
	}
	got, ok := byID[enabled.ID]
	if !ok {
		t.Fatal("the vehicle with an enabled webhook must be listed")
	}
	if got.UserID != owner.ID || got.Name != "Fixture" || got.CurrentOdometer != 41250 {
		t.Fatalf("listed vehicle = %+v", got)
	}
	for _, v := range []*models.Vehicle{disabled, blank, none} {
		if _, listed := byID[v.ID]; listed {
			t.Fatalf("%q has no usable webhook and must not be listed", v.Name)
		}
	}
}
