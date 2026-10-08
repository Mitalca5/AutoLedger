package database

import (
	"context"
	"errors"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func vehicleRepoFixture(t *testing.T, tag string) (*Repository, context.Context, *models.User, *models.User, *models.Vehicle) {
	t.Helper()
	pool := sourcesTestPool(t)
	ctx := context.Background()
	if err := (&DB{Pool: pool}).Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	owner, err := repo.CreateUser(ctx, "owner-"+tag+"@example.com", "x")
	if err != nil {
		t.Fatal(err)
	}
	guest, err := repo.CreateUser(ctx, "guest-"+tag+"@example.com", "x")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: owner.ID, Name: "Fixture", Powertrain: models.PowertrainEV}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	return repo, ctx, owner, guest, v
}

func requireNotFound(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestVehicleLookupsAndWrites(t *testing.T) {
	repo, ctx, owner, guest, v := vehicleRepoFixture(t, "lookups")

	got, err := repo.GetVehicleByID(ctx, v.ID, owner.ID)
	if err != nil || got.Role != models.RoleOwner {
		t.Fatalf("owner lookup = %+v, %v", got, err)
	}
	_, err = repo.GetVehicleByID(ctx, v.ID, guest.ID)
	requireNotFound(t, err)

	internal, err := repo.GetVehicleByIDInternal(ctx, v.ID)
	if err != nil || internal.Name != "Fixture" || internal.Role != models.RoleOwner {
		t.Fatalf("internal lookup = %+v, %v", internal, err)
	}
	_, err = repo.GetVehicleByIDInternal(ctx, "00000000-0000-0000-0000-000000000000")
	requireNotFound(t, err)

	kwh, price := 17.5, 0.21
	if err := repo.UpdateVehicleEstimatedEnergy(ctx, v.ID, owner.ID, &kwh, &price); err != nil {
		t.Fatal(err)
	}
	requireNotFound(t, repo.UpdateVehicleEstimatedEnergy(ctx, v.ID, guest.ID, &kwh, &price))
	got, _ = repo.GetVehicleByID(ctx, v.ID, owner.ID)
	if got.EstimatedKwh100km == nil || *got.EstimatedKwh100km != 17.5 {
		t.Fatalf("estimated energy = %v", got.EstimatedKwh100km)
	}

	if err := repo.UpdateVehicleOdometer(ctx, v.ID, 12345); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetVehicleByID(ctx, v.ID, owner.ID)
	if got.CurrentOdometer != 12345 {
		t.Fatalf("odometer = %v", got.CurrentOdometer)
	}

	missing := &models.Vehicle{ID: "00000000-0000-0000-0000-000000000000", Name: "x", Powertrain: models.PowertrainEV}
	requireNotFound(t, repo.UpdateVehicle(ctx, missing))

	requireNotFound(t, repo.DeleteVehicle(ctx, v.ID, guest.ID))
	if err := repo.DeleteVehicle(ctx, v.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	requireNotFound(t, repo.DeleteVehicle(ctx, v.ID, owner.ID))
}

func TestVehicleMembership(t *testing.T) {
	repo, ctx, owner, guest, v := vehicleRepoFixture(t, "members")

	_, err := repo.AddVehicleMember(ctx, v.ID, guest.Email, "ADMIN", nil)
	requireAPIError(t, err, "member.invalid_role")
	_, err = repo.AddVehicleMember(ctx, v.ID, "nobody@example.com", models.RoleViewer, nil)
	requireNotFound(t, err)

	_, err = repo.AddVehicleMember(ctx, v.ID, "  "+guest.Email, models.RoleViewer, nil)
	requireNotFound(t, err)

	m, err := repo.AddVehicleMember(ctx, v.ID, guest.Email, models.RoleViewer, nil)
	if err != nil || m.Role != models.RoleViewer || m.UserID != guest.ID {
		t.Fatalf("add = %+v, %v", m, err)
	}
	_, err = repo.AddVehicleMember(ctx, v.ID, guest.Email, models.RoleEditor, nil)
	requireAPIError(t, err, "member.already_member")

	members, err := repo.ListVehicleMembers(ctx, v.ID)
	if err != nil || len(members) == 0 {
		t.Fatalf("members = %+v, %v", members, err)
	}

	role, err := repo.GetVehicleMemberRole(ctx, v.ID, guest.ID)
	if err != nil || role != models.RoleViewer {
		t.Fatalf("role = %v, %v", role, err)
	}
	role, err = repo.GetVehicleMemberRole(ctx, v.ID, owner.ID)
	if err != nil || role != models.RoleOwner {
		t.Fatalf("owner role = %v, %v", role, err)
	}
	third, err := repo.CreateUser(ctx, "third-members@example.com", "x")
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.GetVehicleMemberRole(ctx, v.ID, third.ID)
	requireNotFound(t, err)

	requireAPIError(t, repo.UpdateVehicleMemberRole(ctx, v.ID, guest.ID, "ADMIN"), "member.invalid_role")
	requireNotFound(t, repo.UpdateVehicleMemberRole(ctx, v.ID, third.ID, models.RoleEditor))
	if err := repo.UpdateVehicleMemberRole(ctx, v.ID, guest.ID, models.RoleEditor); err != nil {
		t.Fatal(err)
	}
	if role, _ = repo.GetVehicleMemberRole(ctx, v.ID, guest.ID); role != models.RoleEditor {
		t.Fatalf("role after update = %v", role)
	}
}

func TestVehicleLastOwnerGuards(t *testing.T) {
	repo, ctx, _, guest, v := vehicleRepoFixture(t, "lastowner")
	if _, err := repo.AddVehicleMember(ctx, v.ID, guest.Email, models.RoleOwner, nil); err != nil {
		t.Fatal(err)
	}
	members, err := repo.ListVehicleMembers(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	var owners []models.VehicleMember
	for _, m := range members {
		if m.Role == models.RoleOwner {
			owners = append(owners, m)
		}
	}
	if len(owners) < 1 {
		t.Fatalf("expected at least one owner member, got %+v", members)
	}

	// Reduce to a single owner membership, then check neither demotion nor removal is allowed for it.
	for _, m := range owners[1:] {
		if err := repo.RemoveVehicleMember(ctx, v.ID, m.UserID); err != nil {
			t.Fatal(err)
		}
	}
	last := owners[0]
	requireAPIError(t, repo.UpdateVehicleMemberRole(ctx, v.ID, last.UserID, models.RoleViewer), "member.last_owner_demote")
	requireAPIError(t, repo.RemoveVehicleMember(ctx, v.ID, last.UserID), "member.last_owner_remove")

	requireNotFound(t, repo.RemoveVehicleMember(ctx, v.ID, "00000000-0000-0000-0000-000000000000"))
}
