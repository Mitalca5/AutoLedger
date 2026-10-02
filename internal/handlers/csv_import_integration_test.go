package handlers

import (
	"context"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

func TestCSVImportIsAllOrNothingAndPreviewMatchesExecute(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "csv-import@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "EV", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	svc := services.NewCSVImportService(repo, "Europe/Paris")
	opts := services.CSVImportOptions{SkipDuplicates: true, DistanceUnit: "km"}
	count := func() int {
		var n int
		if err := repo.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM charge_logs WHERE vehicle_id = $1`, v.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// The last row passes validation but a table constraint refuses it at INSERT time: the whole file must roll back.
	broken := "date,kwh,cost,currency,fx_rate\n" +
		"2026-09-15 10:00,10,3,EUR,\n" +
		"2026-09-16 10:00,10,3,EUR,\n" +
		"2026-09-17 10:00,10,3,USD,999999\n"
	if _, err := repo.Pool().Exec(ctx, `ALTER TABLE charge_logs ADD CONSTRAINT test_no_usd CHECK (currency = 'EUR')`); err != nil {
		t.Fatal(err)
	}
	preview, err := svc.Preview(ctx, v, []byte(broken), opts)
	if err != nil || preview.ValidRows != 3 {
		t.Fatalf("preview should accept the 3 rows: %v %+v", err, preview)
	}
	res, err := svc.Execute(ctx, v, []byte(broken), opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Committed || res.ImportedCount != 0 || res.ErrorCount != 1 || res.Errors[0].Code != "import.row.database_error" {
		t.Fatalf("expected a rollback, got %+v", res)
	}
	if n := count(); n != 0 {
		t.Fatalf("a failed import must leave no row, found %d", n)
	}
	if _, err := repo.Pool().Exec(ctx, `ALTER TABLE charge_logs DROP CONSTRAINT test_no_usd`); err != nil {
		t.Fatal(err)
	}

	good := "date,kwh,cost\n2026-09-15 10:00,10,3\n2026-09-16 10:00,12,4\n"
	preview, _ = svc.Preview(ctx, v, []byte(good), opts)
	res, err = svc.Execute(ctx, v, []byte(good), opts)
	if err != nil || !res.Committed || res.ImportedCount != preview.ValidRows || count() != 2 {
		t.Fatalf("execute must import what preview announced: %v %+v", err, res)
	}

	again, _ := svc.Preview(ctx, v, []byte(good), opts)
	if again.DuplicateRows != 2 || again.ValidRows != 0 {
		t.Fatalf("a second pass must be all duplicates: %+v", again)
	}
	res, _ = svc.Execute(ctx, v, []byte(good), opts)
	if res.ImportedCount != 0 || res.SkippedCount != 2 || count() != 2 {
		t.Fatalf("duplicates must be skipped: %+v", res)
	}

	// A row invalid at validation level cancels the whole file too.
	mixed := "date,kwh,cost\n2026-10-01 10:00,10,3\n2026-10-02 10:00,10,\n"
	res, _ = svc.Execute(ctx, v, []byte(mixed), opts)
	if res.Committed || res.ErrorCount != 1 || count() != 2 {
		t.Fatalf("an invalid row must cancel the import: %+v", res)
	}
}
