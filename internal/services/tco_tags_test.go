package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// The cost of a tag is the sum of the drives the list shows for that tag, computed the way the drive list computes
// them; a drive with two tags counts in both and is reported as shared.
func TestIntegrationTagBreakdownCostMatchesTheTaggedDriveList(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "tag_costs@example.com")

	base := time.Date(2026, 8, 3, 8, 0, 0, 0, time.UTC)
	for i, cost := range []money.Cents{1200, 1800} {
		c := &models.ChargeLog{VehicleID: v.ID, Date: base.Add(-time.Duration(i+1) * 24 * time.Hour), KwhAdded: 40, Cost: &cost, Currency: "EUR"}
		if err := repo.CreateManualCharge(ctx, c); err != nil {
			t.Fatal(err)
		}
	}

	tagsByDrive := [][]string{{"Pro"}, {"Perso"}, {"Pro", "Perso"}, {}}
	for i, tags := range tagsByDrive {
		d := mustDrive(t, repo, v.ID, 100+i, base.Add(time.Duration(i)*24*time.Hour), 20000+float64(i)*100, 30+float64(i)*10)
		if len(tags) > 0 {
			if err := repo.UpdateDriveTags(ctx, d.ID, v.ID, tags); err != nil {
				t.Fatal(err)
			}
		}
	}

	sum, err := NewTCOService(db.Pool, "UTC").ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	byTag := map[string]TagCostBreakdown{}
	for _, tb := range sum.TagBreakdown {
		byTag[tb.Tag] = tb
	}

	carpool := NewCarpoolService(db.Pool, repo)
	rates, err := carpool.GetVehicleUnitRates(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"Pro", "Perso"} {
		drives, _, err := repo.ListDrives(ctx, v.ID, database.DriveFilter{Tag: tag}, 100, 0)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, len(drives))
		for i, d := range drives {
			ids[i] = d.ID
		}
		insurance, err := carpool.AllocateInsuranceCosts(ctx, v.ID, drives, rates)
		if err != nil {
			t.Fatal(err)
		}
		tolls, err := repo.GetTollExpensesForDrives(ctx, v.ID, ids)
		if err != nil {
			t.Fatal(err)
		}
		var want money.Cents
		for _, d := range drives {
			want += ComputeDriveCost(rates, d, insurance[d.ID], tolls[d.ID]).TotalCost
		}
		got := byTag[tag]
		if want == 0 {
			t.Fatalf("%s: the fixture must price the drives (rates %+v)", tag, rates)
		}
		if got.Cost != want {
			t.Errorf("%s: cost %d, drive list sums to %d", tag, got.Cost, want)
		}
		if got.SharedDrives != 1 {
			t.Errorf("%s: shared drives = %d, want 1", tag, got.SharedDrives)
		}
	}
	if untagged, ok := byTag[""]; !ok || untagged.SharedDrives != 0 {
		t.Errorf("untagged drives stay on their own entry: %+v", byTag)
	}
}

func TestIntegrationTagBreakdownWithoutTagsSkipsCosts(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "tag_none@example.com")
	mustDrive(t, repo, v.ID, 1, time.Date(2026, 8, 3, 8, 0, 0, 0, time.UTC), 20000, 40)

	sum, err := NewTCOService(db.Pool, "UTC").ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.TagBreakdown) != 1 || sum.TagBreakdown[0].Tag != "" || sum.TagBreakdown[0].Cost != 0 {
		t.Fatalf("one untagged entry without cost: %+v", sum.TagBreakdown)
	}
	if sum.TagBreakdown[0].DistanceKm != 40 {
		t.Fatalf("distance kept: %+v", sum.TagBreakdown[0])
	}
}
