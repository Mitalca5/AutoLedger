package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/database"
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

func TestCSVImportBatchesListAndUndo(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "csv-batches@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "Car", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	svc := services.NewCSVImportService(repo, "Europe/Paris")
	run := func(typ services.ImportType, csv string) *services.CSVExecuteResult {
		t.Helper()
		res, err := svc.Execute(ctx, v, []byte(csv), services.CSVImportOptions{Type: typ, SkipDuplicates: true, DistanceUnit: "km", UserID: u.ID})
		if err != nil || !res.Committed || res.BatchID == "" {
			t.Fatalf("import %s: %v %+v", typ, err, res)
		}
		return res
	}
	count := func(table string) int {
		var n int
		if err := repo.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM `+table+` WHERE vehicle_id = $1`, v.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// A manual row stays untouched whatever happens to the batches.
	if err := repo.CreateOdometerCheckpoint(ctx, &models.OdometerCheckpoint{VehicleID: v.ID, Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Odometer: 100}); err != nil {
		t.Fatal(err)
	}

	charges := run(services.ImportTypeCharges, "date,kwh,cost\n2026-09-15 10:00,10,3\n2026-09-16 10:00,12,4\n")
	drives := run(services.ImportTypeDrives, "start_time,end_time,distance_km\n2026-09-15 08:00,2026-09-15 09:00,40\n")
	fuel := run(services.ImportTypeFuel, "date,liters,amount\n2026-09-15,30,50\n")
	odo := run(services.ImportTypeOdometer, "date,odometer_km\n2026-09-20,5000\n")

	batches, err := repo.ListImportBatches(ctx, v.ID)
	if err != nil || len(batches) != 4 {
		t.Fatalf("list: %v %+v", err, batches)
	}
	for _, b := range batches {
		if b.RowCount != b.Remaining || b.RowCount == 0 {
			t.Fatalf("a fresh batch must still own its rows: %+v", b)
		}
	}

	// Deleting one imported row lowers what remains; undo removes only the rest of that batch.
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM charge_logs WHERE source_batch_id::text = $1 AND kwh_added = 10`, charges.BatchID); err != nil {
		t.Fatal(err)
	}
	removed, err := repo.UndoImportBatch(ctx, v.ID, charges.BatchID)
	if err != nil || removed != 1 || count("charge_logs") != 0 {
		t.Fatalf("undo charges: %v removed=%d", err, removed)
	}
	if count("drives") != 1 || count("fuel_logs") != 1 || count("odometer_checkpoints") != 2 {
		t.Fatal("undoing one batch must not touch the others")
	}
	for _, res := range []*services.CSVExecuteResult{drives, fuel, odo} {
		if _, err := repo.UndoImportBatch(ctx, v.ID, res.BatchID); err != nil {
			t.Fatal(err)
		}
	}
	if count("drives") != 0 || count("fuel_logs") != 0 || count("odometer_checkpoints") != 1 {
		t.Fatal("undo must remove every imported row and keep the manual one")
	}
	if batches, _ := repo.ListImportBatches(ctx, v.ID); len(batches) != 0 {
		t.Fatalf("undone batches must disappear: %+v", batches)
	}

	// Unknown batch, and a batch of another vehicle.
	if _, err := repo.UndoImportBatch(ctx, v.ID, charges.BatchID); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("second undo: %v", err)
	}
	other := &models.Vehicle{UserID: u.ID, Name: "Other", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, other); err != nil {
		t.Fatal(err)
	}
	again := run(services.ImportTypeOdometer, "date,odometer_km\n2026-09-21,6000\n")
	if _, err := repo.UndoImportBatch(ctx, other.ID, again.BatchID); !errors.Is(err, database.ErrNotFound) || count("odometer_checkpoints") != 2 {
		t.Fatalf("a batch must only be undone through its own vehicle: %v", err)
	}
}
