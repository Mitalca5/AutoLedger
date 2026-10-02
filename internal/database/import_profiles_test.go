package database

import (
	"context"
	"errors"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestImportProfiles(t *testing.T) {
	pool := sourcesTestPool(t)
	ctx := context.Background()
	if err := (&DB{Pool: pool}).Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	var alice, bob string
	for email, dst := range map[string]*string{"alice-prof@example.com": &alice, "bob-prof@example.com": &bob} {
		if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, 'x') RETURNING id::text`, email).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}

	p := &models.ImportProfile{Name: "Bank", ImportType: "CHARGES", Columns: map[string]string{"Truc": "kwh", "Skip": ""}, DateOrder: "mdy", DecimalSeparator: ","}
	if err := repo.SaveImportProfile(ctx, alice, p); err != nil || p.ID == "" {
		t.Fatalf("save: %v", err)
	}
	replaced := &models.ImportProfile{Name: "Bank", ImportType: "CHARGES", Columns: map[string]string{"Truc": "cost"}}
	if err := repo.SaveImportProfile(ctx, alice, replaced); err != nil {
		t.Fatal(err)
	}
	list, err := repo.ListImportProfiles(ctx, alice)
	if err != nil || len(list) != 1 || list[0].Columns["Truc"] != "cost" || list[0].DateOrder != "" {
		t.Fatalf("saving the same name replaces the profile: %+v %v", list, err)
	}
	if other, _ := repo.ListImportProfiles(ctx, bob); len(other) != 0 {
		t.Errorf("profiles are private to their owner: %+v", other)
	}
	if err := repo.DeleteImportProfile(ctx, bob, list[0].ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleting someone else's profile = %v, want ErrNotFound", err)
	}
	if err := repo.DeleteImportProfile(ctx, alice, "not-a-uuid"); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleting an unknown id = %v, want ErrNotFound", err)
	}
	if err := repo.DeleteImportProfile(ctx, alice, list[0].ID); err != nil {
		t.Fatal(err)
	}
}
