package database

import (
	"context"
	"errors"
	"testing"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
)

func peopleByName(list []models.VehiclePerson) map[string]models.VehiclePerson {
	out := map[string]models.VehiclePerson{}
	for _, p := range list {
		out[p.Name] = p
	}
	return out
}

func requireAPIError(t *testing.T, err error, code string) {
	t.Helper()
	var ae *apierror.Error
	if !errors.As(err, &ae) || ae.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func TestVehiclePeople(t *testing.T) {
	pool := sourcesTestPool(t)
	ctx := context.Background()
	if err := (&DB{Pool: pool}).Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)

	owner, err := repo.CreateUser(ctx, "owner-people@example.com", "x")
	if err != nil {
		t.Fatal(err)
	}
	guest, err := repo.CreateUser(ctx, "guest-people@example.com", "x")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: owner.ID, Name: "Family car", Powertrain: models.PowertrainEV}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}

	// The owner is a person of the vehicle and its default driver.
	people, err := repo.ListVehiclePeople(ctx, v.ID)
	if err != nil || len(people) != 1 {
		t.Fatalf("people = %+v, %v", people, err)
	}
	ownerPerson := people[0]
	if !ownerPerson.IsDefault || ownerPerson.UserID == nil || *ownerPerson.UserID != owner.ID {
		t.Fatalf("owner person = %+v", ownerPerson)
	}

	// A person without an account can be added and drive.
	lea, err := repo.CreateVehiclePerson(ctx, v.ID, "  Léa  ")
	if err != nil || lea.Name != "Léa" || lea.UserID != nil || lea.IsDefault {
		t.Fatalf("create = %+v, %v", lea, err)
	}
	_, err = repo.CreateVehiclePerson(ctx, v.ID, "   ")
	requireAPIError(t, err, "person.name_required")

	var driveID string
	if err := pool.QueryRow(ctx, `INSERT INTO drives (vehicle_id, start_time, end_time, distance_km, duration_min, driver_id)
		VALUES ($1, NOW() - INTERVAL '2 hours', NOW() - INTERVAL '1 hour', 40, 60, $2) RETURNING id::text`, v.ID, lea.ID).Scan(&driveID); err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.VehiclePersonExists(ctx, v.ID, lea.ID); err != nil || !ok {
		t.Fatalf("exists = %v, %v", ok, err)
	}
	if ok, _ := repo.VehiclePersonExists(ctx, "00000000-0000-0000-0000-000000000000", lea.ID); ok {
		t.Error("a person belongs to its own vehicle only")
	}

	// An account that joins gets its own person; linking it to Léa merges the two and keeps the history.
	if _, err := repo.AddVehicleMember(ctx, v.ID, guest.Email, models.RoleEditor, nil); err != nil {
		t.Fatal(err)
	}
	if people, _ = repo.ListVehiclePeople(ctx, v.ID); len(people) != 3 {
		t.Fatalf("the member gets a person: %+v", people)
	}
	linked, err := repo.LinkVehiclePersonToMember(ctx, v.ID, lea.ID, guest.ID)
	if err != nil || linked.UserID == nil || *linked.UserID != guest.ID || linked.Name != "Léa" {
		t.Fatalf("link = %+v, %v", linked, err)
	}
	if people, _ = repo.ListVehiclePeople(ctx, v.ID); len(people) != 2 {
		t.Fatalf("the member's own person is merged: %+v", people)
	}
	var driver string
	if err := pool.QueryRow(ctx, `SELECT driver_id::text FROM drives WHERE id::text = $1`, driveID).Scan(&driver); err != nil || driver != lea.ID {
		t.Fatalf("history kept on %s: %v", driver, err)
	}
	_, err = repo.LinkVehiclePersonToMember(ctx, v.ID, lea.ID, owner.ID)
	requireAPIError(t, err, "person.already_linked")
	outsider, _ := repo.CreateUser(ctx, "outsider-people@example.com", "x")
	pending, _ := repo.CreateVehiclePerson(ctx, v.ID, "Tom")
	_, err = repo.LinkVehiclePersonToMember(ctx, v.ID, pending.ID, outsider.ID)
	requireAPIError(t, err, "person.link_requires_member")

	// A new member can take over an existing person right away.
	third, _ := repo.CreateUser(ctx, "third-people@example.com", "x")
	if _, err := repo.AddVehicleMember(ctx, v.ID, third.Email, models.RoleViewer, &pending.ID); err != nil {
		t.Fatal(err)
	}
	if people, _ = repo.ListVehiclePeople(ctx, v.ID); len(people) != 3 || peopleByName(people)["Tom"].UserID == nil {
		t.Fatalf("the member took over Tom: %+v", people)
	}

	// Default driver: changeable, and the default one cannot be removed.
	if err := repo.SetVehicleDefaultDriver(ctx, v.ID, lea.ID); err != nil {
		t.Fatal(err)
	}
	err = repo.DeleteVehiclePerson(ctx, v.ID, lea.ID)
	requireAPIError(t, err, "person.default_delete")
	err = repo.DeleteVehiclePerson(ctx, v.ID, ownerPerson.ID)
	requireAPIError(t, err, "person.linked_delete")

	// Removing a member keeps the person and their drives, without the account.
	if err := repo.RemoveVehicleMember(ctx, v.ID, guest.ID); err != nil {
		t.Fatal(err)
	}
	after, err := repo.ListVehiclePeople(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	lea2, ok := peopleByName(after)["Léa"]
	if !ok || lea2.UserID != nil || !lea2.IsDefault {
		t.Fatalf("Léa stays, without account: %+v", after)
	}
	if err := repo.SetVehicleDefaultDriver(ctx, v.ID, ownerPerson.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteVehiclePerson(ctx, v.ID, lea.ID); err != nil {
		t.Fatal(err)
	}
	var unassigned bool
	if err := pool.QueryRow(ctx, `SELECT driver_id IS NULL FROM drives WHERE id::text = $1`, driveID).Scan(&unassigned); err != nil || !unassigned {
		t.Fatalf("drives of a removed person become unassigned: %v %v", unassigned, err)
	}
	if _, err := repo.RenameVehiclePerson(ctx, v.ID, lea.ID, "Gone"); err == nil {
		t.Error("renaming an unknown person must fail")
	}
}

func TestFleetSharesGroupAccountAndGuestDrivers(t *testing.T) {
	pool := sourcesTestPool(t)
	ctx := context.Background()
	if err := (&DB{Pool: pool}).Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	owner, _ := repo.CreateUser(ctx, "owner-fleet-people@example.com", "x")
	v := &models.Vehicle{UserID: owner.ID, Name: "Shared", Powertrain: models.PowertrainEV}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	guest, err := repo.CreateVehiclePerson(ctx, v.ID, "Léa")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []struct {
		km     float64
		driver *string
	}{{30, nil}, {10, &guest.ID}} {
		if _, err := pool.Exec(ctx, `INSERT INTO drives (vehicle_id, start_time, end_time, distance_km, duration_min, driver_id)
			VALUES ($1, date_trunc('month', NOW()), date_trunc('month', NOW()) + INTERVAL '1 hour', $2, 60, $3)`, v.ID, d.km, d.driver); err != nil {
			t.Fatal(err)
		}
	}
	sum, err := repo.GetFleetSummary(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, s := range sum.MemberKmShares {
		got[s.DisplayName] = s.DistanceKm
	}
	if len(got) != 2 || got["Léa"] != 10 || got["owner-fleet-people@example.com"] != 30 {
		t.Errorf("shares = %+v", sum.MemberKmShares)
	}
}
