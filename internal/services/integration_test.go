package services

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/migrations"
)

// Integration tests run against a disposable PostgreSQL database:
//   TEST_DATABASE_URL=postgres://user:pass@localhost:5432/teslacost_test?sslmode=disable go test ./internal/services/
// The public schema of that database is dropped and recreated.

func setupIntegrationDB(t *testing.T, legacyMigrations bool) (*database.DB, *database.Repository) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatal(err)
	}
	if legacyMigrations {
		// Simulates a deployment created before migration versioning: files 1-4 applied, no schema_migrations.
		for _, f := range []string{"000001_init_schema.up.sql", "000002_carpooling.up.sql", "000003_tire_enhancements.up.sql", "000004_telemetry_power_and_insurance.up.sql"} {
			sql, _ := migrations.FS.ReadFile(f)
			if _, err := pool.Exec(ctx, string(sql)); err != nil {
				t.Fatalf("legacy migration %s: %v", f, err)
			}
		}
	}
	db := &database.DB{Pool: pool}
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db, database.NewRepository(pool)
}

func mustVehicle(t *testing.T, repo *database.Repository, email string) *models.Vehicle {
	t.Helper()
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, email, "hash")
	if err != nil {
		t.Fatal(err)
	}
	carID := 1
	apiURL := "http://teslamate.invalid"
	v := &models.Vehicle{
		UserID:            u.ID,
		Name:              "Connected EV",
		TeslaMateCarID:    &carID,
		TeslaMateAPIURL:   &apiURL,
		TeslaMateAuthType: models.AuthModeNone,
		CurrentOdometer:   20000,
	}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	return v
}

func mustDrive(t *testing.T, repo *database.Repository, vehicleID string, tmID int, start time.Time, startOdo, km float64) *models.Drive {
	t.Helper()
	end := startOdo + km
	speed := 100.0
	d := &models.Drive{VehicleID: vehicleID, TeslaMateDriveID: &tmID, StartTime: start, EndTime: start.Add(time.Hour),
		StartOdometer: &startOdo, EndOdometer: &end, DistanceKm: km, SpeedAvg: &speed, Tags: []string{}}
	if _, err := repo.UpsertTeslaMateDrive(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestIntegrationMigrationsOnLegacyDatabase(t *testing.T) {
	db, _ := setupIntegrationDB(t, true)
	var count int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	entries, _ := migrations.FS.ReadDir(".")
	expected := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			expected++
		}
	}
	if count != expected {
		t.Fatalf("expected %d recorded migrations, got %d", expected, count)
	}
	// Idempotent second run.
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestIntegrationDriveExpenseOwnershipAndGroupAllocation(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	alice := mustVehicle(t, repo, "alice@example.com")
	bob := mustVehicle(t, repo, "bob@example.com")

	base := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	d1 := mustDrive(t, repo, alice.ID, 1, base, 10000, 100)
	d2 := mustDrive(t, repo, alice.ID, 2, base.Add(3*time.Hour), 10100, 300)
	bobDrive := mustDrive(t, repo, bob.ID, 1, base, 5000, 50)

	// Cross-tenant references are rejected.
	foreign := &models.DriveExpense{VehicleID: alice.ID, DriveID: &bobDrive.ID, Type: "TOLL", Amount: 500, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, foreign, nil, ""); !errors.Is(err, database.ErrForeignReference) {
		t.Fatalf("expected ErrForeignReference for a foreign drive, got %v", err)
	}
	if err := repo.CreateTripGroup(ctx, &models.TripGroup{VehicleID: alice.ID, Name: "x"}, []string{d1.ID, bobDrive.ID}); !errors.Is(err, database.ErrForeignReference) {
		t.Fatalf("expected ErrForeignReference for a foreign drive in a group, got %v", err)
	}

	// A 40 € toll over two drives (100 km + 300 km) is allocated 10 / 30.
	exp := &models.DriveExpense{VehicleID: alice.ID, Type: "TOLL", Amount: 4000, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, exp, []string{d2.ID, d1.ID}, "Paris → Lyon"); err != nil {
		t.Fatal(err)
	}
	if exp.TripGroupID == nil || exp.DriveID != nil {
		t.Fatal("expected the expense to be attached to a trip group only")
	}
	alloc, err := repo.GetTollExpensesForDrives(ctx, alice.ID, []string{d1.ID, d2.ID})
	if err != nil {
		t.Fatal(err)
	}
	if alloc[d1.ID] != 1000 || alloc[d2.ID] != 3000 {
		t.Fatalf("unexpected allocation: %v", alloc)
	}
	perDrive, err := repo.GetDriveExpensesByDriveID(ctx, alice.ID, d1.ID, "en")
	if err != nil || len(perDrive) != 1 || perDrive[0].AllocatedAmount == nil || *perDrive[0].AllocatedAmount != 1000 {
		t.Fatalf("expected one expense allocated 10 € to d1, got %+v (err %v)", perDrive, err)
	}

	// Editing the expense (same drives) keeps the same group instead of creating a new one.
	groupID := *exp.TripGroupID
	exp.Amount = 4200
	if err := repo.SaveDriveExpense(ctx, exp, []string{d1.ID, d2.ID}, "Paris → Lyon"); err != nil {
		t.Fatal(err)
	}
	if exp.TripGroupID == nil || *exp.TripGroupID != groupID {
		t.Fatal("expected the trip group to be reused on edit")
	}
	groups, _ := repo.ListTripGroups(ctx, alice.ID)
	if len(groups) != 1 {
		t.Fatalf("expected a single trip group, got %d", len(groups))
	}

	// Editing without links keeps nothing hidden: the list exposes group drive IDs for the UI.
	list, _ := repo.ListDriveExpenses(ctx, alice.ID, "en")
	if len(list) != 1 || len(list[0].TripGroupDriveIDs) != 2 {
		t.Fatalf("expected group drive IDs in listing, got %+v", list)
	}

	// Drives with an allocated toll are no longer "unqualified"; bob's fast drive is.
	if n, _ := repo.CountUnqualifiedDrives(ctx, alice.ID); n != 0 {
		t.Fatalf("expected no unqualified drive for alice, got %d", n)
	}
	if n, _ := repo.CountUnqualifiedDrives(ctx, bob.ID); n != 1 {
		t.Fatalf("expected 1 unqualified drive for bob, got %d", n)
	}
	if err := repo.SetDriveTollReviewed(ctx, bobDrive.ID, bob.ID, true); err != nil {
		t.Fatal(err)
	}
	if n, _ := repo.CountUnqualifiedDrives(ctx, bob.ID); n != 0 {
		t.Fatalf("expected reviewed drive to leave the queue, got %d", n)
	}
}

// TestIntegrationDriveExpenseTitleFallsBackToTheAskedLanguage covers a drive missing a
// reverse-geocoded address: the "Start"/"Arrival" (or "Départ"/"Arrivée") placeholder is built
// in the query itself, in the language passed to it, not translated by the frontend afterward.
func TestIntegrationDriveExpenseTitleFallsBackToTheAskedLanguage(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "dana@example.com")
	base := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	d := mustDrive(t, repo, v.ID, 1, base, 10000, 100)

	exp := &models.DriveExpense{VehicleID: v.ID, DriveID: &d.ID, Type: "PARKING", Amount: 500, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, exp, nil, ""); err != nil {
		t.Fatal(err)
	}

	enList, err := repo.ListDriveExpenses(ctx, v.ID, "en")
	if err != nil || len(enList) != 1 || enList[0].DriveTitle == nil || *enList[0].DriveTitle != "Start → Arrival" {
		t.Fatalf("expected the English fallback title, got %+v (err %v)", enList, err)
	}
	frList, err := repo.ListDriveExpenses(ctx, v.ID, "fr")
	if err != nil || len(frList) != 1 || frList[0].DriveTitle == nil || *frList[0].DriveTitle != "Départ → Arrivée" {
		t.Fatalf("expected the French fallback title, got %+v (err %v)", frList, err)
	}

	enByDrive, err := repo.GetDriveExpensesByDriveID(ctx, v.ID, d.ID, "en")
	if err != nil || len(enByDrive) != 1 || enByDrive[0].DriveTitle == nil || *enByDrive[0].DriveTitle != "Start → Arrival" {
		t.Fatalf("expected the English fallback title by drive, got %+v (err %v)", enByDrive, err)
	}
}

// TestIntegrationVehicleCurrencyIsFixedAtCreation covers the immutability guarantee the currency
// column relies on: UpdateVehicle's SET clause never includes currency, so a value sent on update
// is silently ignored rather than retroactively reconverting stored amounts.
func TestIntegrationVehicleCurrencyIsFixedAtCreation(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()

	defaultUser, err := repo.CreateUser(ctx, "eur@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	defaultVehicle := &models.Vehicle{UserID: defaultUser.ID, Name: "Model 3", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, defaultVehicle); err != nil {
		t.Fatal(err)
	}
	if defaultVehicle.Currency != "EUR" {
		t.Fatalf("expected the default currency to be EUR, got %q", defaultVehicle.Currency)
	}

	usdUser, err := repo.CreateUser(ctx, "usd@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	usdVehicle := &models.Vehicle{UserID: usdUser.ID, Name: "Bolt", Currency: "USD", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, usdVehicle); err != nil {
		t.Fatal(err)
	}
	if usdVehicle.Currency != "USD" {
		t.Fatalf("expected the chosen currency to be USD, got %q", usdVehicle.Currency)
	}

	usdVehicle.Currency = "GBP"
	usdVehicle.Name = "Bolt EUV"
	if err := repo.UpdateVehicle(ctx, usdVehicle); err != nil {
		t.Fatal(err)
	}
	reloaded, err := repo.GetVehicleByID(ctx, usdVehicle.ID, usdUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Currency != "USD" {
		t.Fatalf("expected the currency to stay USD after an update, got %q", reloaded.Currency)
	}
	if reloaded.Name != "Bolt EUV" {
		t.Fatalf("expected the name change to still go through, got %q", reloaded.Name)
	}
}

// TestIntegrationTollAllocationConvertsToTheVehicleCurrency covers amountInVehicleCurrencyExpr:
// an expense in the vehicle's own currency needs no rate, a foreign one is converted by fx_rate,
// and a foreign one without a rate is excluded — the same rule that used to hard-code the euro.
func TestIntegrationTollAllocationConvertsToTheVehicleCurrency(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "usd-tolls@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "Model 3", Currency: "USD", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	native := mustDrive(t, repo, v.ID, 1, base, 10000, 100)
	converted := mustDrive(t, repo, v.ID, 2, base.Add(time.Hour), 10100, 50)
	unconverted := mustDrive(t, repo, v.ID, 3, base.Add(2*time.Hour), 10150, 50)

	fxRate := 0.9
	nativeExp := &models.DriveExpense{VehicleID: v.ID, DriveID: &native.ID, Type: "TOLL", Amount: 1000, Currency: "USD"}
	convertedExp := &models.DriveExpense{VehicleID: v.ID, DriveID: &converted.ID, Type: "TOLL", Amount: 1000, Currency: "EUR", FxRate: &fxRate}
	unconvertedExp := &models.DriveExpense{VehicleID: v.ID, DriveID: &unconverted.ID, Type: "TOLL", Amount: 1000, Currency: "EUR"}
	for _, exp := range []*models.DriveExpense{nativeExp, convertedExp, unconvertedExp} {
		if err := repo.SaveDriveExpense(ctx, exp, nil, ""); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.GetTollExpensesForDrives(ctx, v.ID, []string{native.ID, converted.ID, unconverted.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got[native.ID] != 1000 {
		t.Errorf("same-currency expense: got %v, want 1000 (no conversion needed)", got[native.ID])
	}
	if got[converted.ID] != 900 {
		t.Errorf("foreign expense with a rate: got %v, want 900 (1000 * 0.9)", got[converted.ID])
	}
	if got[unconverted.ID] != 0 {
		t.Errorf("foreign expense without a rate: got %v, want 0 (excluded)", got[unconverted.ID])
	}
}

func TestIntegrationTCOCompletenessRecurringAndInsurance(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "carol@example.com")
	tco := NewTCOService(db.Pool, "Europe/Paris")

	now := time.Now().UTC()
	mustDrive(t, repo, v.ID, 1, now.AddDate(0, -2, 0), 10000, 400)
	mustDrive(t, repo, v.ID, 2, now.AddDate(0, -1, 0), 10900, 100) // 500 km odometer gap before this drive

	// Charges: one priced, one without tariff.
	cost := money.Cents(1200)
	c1, c2 := 1, 2
	if _, err := repo.UpsertTeslaMateCharge(ctx, &models.ChargeLog{VehicleID: v.ID, TeslaMateChargeID: &c1, Date: now.AddDate(0, -1, 0), KwhAdded: 40, Cost: &cost, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertTeslaMateCharge(ctx, &models.ChargeLog{VehicleID: v.ID, TeslaMateChargeID: &c2, Date: now.AddDate(0, 0, -3), KwhAdded: 30, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}

	// No insurance premium recorded yet: reported as missing.
	sum, err := tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.InsuranceSource != InsuranceSourceNone || !sum.Completeness.InsuranceMissing || sum.InsuranceCost != 0 {
		t.Fatalf("expected missing insurance, got %s %s", sum.InsuranceSource, sum.InsuranceCost)
	}
	if sum.Completeness.ChargesWithoutCost != 1 || sum.Completeness.IsComplete {
		t.Fatalf("expected a charge without cost to be reported, got %+v", sum.Completeness)
	}
	if sum.AvgCostPerKwh != 0.3 {
		t.Fatalf("expected €/kWh computed on priced charges only (0.30), got %.3f", sum.AvgCostPerKwh)
	}
	if sum.OdometerDistanceKm != 1000 || sum.TotalDistanceKm != 500 || sum.DistanceBasisKm != 1000 {
		t.Fatalf("expected odometer-based distance basis, got tracked=%.0f odo=%.0f basis=%.0f", sum.TotalDistanceKm, sum.OdometerDistanceKm, sum.DistanceBasisKm)
	}

	// A monthly recurring insurance started 3 months ago counts 4 occurrences and replaces vehicle settings.
	interval := 1
	start := now.AddDate(0, -3, 0).Add(-time.Hour)
	if err := repo.CreateMaintenanceExpense(ctx, &models.MaintenanceExpense{VehicleID: v.ID, Category: "INSURANCE", Amount: 5000, Currency: "EUR",
		Date: start, IsRecurring: true, RecurrenceIntervalMonths: &interval, Description: "Assurance"}); err != nil {
		t.Fatal(err)
	}
	// A CHF parking without rate is excluded and reported.
	if err := repo.SaveDriveExpense(ctx, &models.DriveExpense{VehicleID: v.ID, Type: "PARKING", Amount: 2000, Currency: "CHF", Date: now}, nil, ""); err != nil {
		t.Fatal(err)
	}
	sum, err = tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.InsuranceSource != InsuranceSourceRecordedExpenses || sum.InsuranceCost != 20000 {
		t.Fatalf("expected 4 × 50 € of recorded insurance, got %s %s", sum.InsuranceSource, sum.InsuranceCost)
	}
	if sum.Completeness.UnconvertedExpenses != 1 || sum.TollsCost != 0 {
		t.Fatalf("expected the CHF expense to be excluded and reported, got %+v tolls=%s", sum.Completeness, sum.TollsCost)
	}
	if sum.TotalCost != 21200 {
		t.Fatalf("expected total 12 € energy + 200 € insurance, got %s", sum.TotalCost)
	}
	var monthlyInsurance money.Cents
	for _, m := range sum.MonthlyCosts {
		monthlyInsurance += m.Insurance
	}
	if monthlyInsurance != 20000 {
		t.Fatalf("monthly timeline must match the total insurance, got %s", monthlyInsurance)
	}
}

func TestIntegrationTiresAndManualCharges(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "dave@example.com")
	other := mustVehicle(t, repo, "eve@example.com")

	// Used tire bought with 12,000 km, mounted at 10,000 km.
	odo := 10000.0
	tire := &models.Tire{VehicleID: &v.ID, Brand: "Michelin", Model: "PS4", Dimension: "235/40 R19", Season: models.TireSeasonSummer,
		PurchaseDate: time.Now(), PurchasePrice: 15000, CurrentPosition: models.TirePosFL, InitialDepthMm: 8, MinLegalDepthMm: 1.6,
		MountedOdometer: &odo, AccumulatedDistanceKm: 12000, EstimatedLifespanKm: 40000}
	if err := repo.CreateTire(ctx, tire); err != nil {
		t.Fatal(err)
	}

	// Sessions of another vehicle's tire cannot be touched.
	if err := repo.CreateTireMountSession(ctx, &models.TireMountSession{TireID: tire.ID, VehicleID: other.ID, Position: models.TirePosFL, MountedDate: time.Now(), MountedOdometer: 1}); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a foreign tire, got %v", err)
	}

	// Rotation with an odometer lower than the mount odometer is rejected.
	var vErr *apierror.Error
	if err := repo.QuickRotateTires(ctx, v.ID, "FRONT_BACK", 9000, nil); !errors.As(err, &vErr) {
		t.Fatalf("expected a validation error, got %v", err)
	}
	if err := repo.QuickRotateTires(ctx, v.ID, "FRONT_BACK", 15000, nil); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetTireByID(ctx, tire.ID, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentPosition != models.TirePosRL || got.AccumulatedDistanceKm != 17000 || got.InitialDistanceKm != 12000 {
		t.Fatalf("expected RL with 12,000 + 5,000 km, got %s %.0f (initial %.0f)", got.CurrentPosition, got.AccumulatedDistanceKm, got.InitialDistanceKm)
	}

	// Manual cost correction survives a TeslaMate resync.
	tmID := 7
	if _, err := repo.UpsertTeslaMateCharge(ctx, &models.ChargeLog{VehicleID: v.ID, TeslaMateChargeID: &tmID, Date: time.Now(), KwhAdded: 50, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	charges, _, _ := repo.ListCharges(ctx, v.ID, true, 10, 0)
	if len(charges) != 1 {
		t.Fatalf("expected one charge without cost, got %d", len(charges))
	}
	manualCost := money.Cents(1850)
	fix := &models.ChargeLog{ID: charges[0].ID, VehicleID: v.ID, Cost: &manualCost, Currency: "EUR"}
	if err := repo.UpdateCharge(ctx, fix); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertTeslaMateCharge(ctx, &models.ChargeLog{VehicleID: v.ID, TeslaMateChargeID: &tmID, Date: time.Now(), KwhAdded: 50, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	charges, _, _ = repo.ListCharges(ctx, v.ID, false, 10, 0)
	if charges[0].Cost == nil || *charges[0].Cost != 1850 || charges[0].CostSource != "MANUAL" || charges[0].KwhAdded != 50 {
		t.Fatalf("expected manual cost to survive resync, got %+v", charges[0])
	}
	if err := repo.DeleteManualCharge(ctx, v.ID, charges[0].ID); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("TeslaMate charges must not be deletable, got %v", err)
	}
}

func TestIntegrationUpstreamDeletions(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "frank@example.com")
	tco := NewTCOService(db.Pool, "UTC")

	base := time.Now().UTC().AddDate(0, -1, 0)
	var drives []*models.Drive
	for i := 1; i <= 12; i++ {
		drives = append(drives, mustDrive(t, repo, v.ID, i, base.Add(time.Duration(i)*time.Hour), 10000+float64(i)*100, 100))
	}
	toll := &models.DriveExpense{VehicleID: v.ID, Type: "TOLL", Amount: 3000, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, toll, []string{drives[10].ID, drives[11].ID}, "Voyage"); err != nil {
		t.Fatal(err)
	}

	// Drive 12 deleted in TeslaMate, incremental window covering drives 11 and 12.
	res, err := repo.ReconcileTeslaMateRecords(ctx, "drives", v.ID, &drives[9].StartTime, []int{11})
	if err != nil || res.Marked != 1 {
		t.Fatalf("expected 1 drive flagged, got %+v (err %v)", res, err)
	}
	list, total, _ := repo.ListDrives(ctx, v.ID, database.DriveFilter{}, 50, 0)
	if total != 11 || len(list) != 11 {
		t.Fatalf("expected deleted drive excluded from listing, got %d", total)
	}
	sum, err := tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.TotalDistanceKm != 1100 || sum.TollsCost != 3000 {
		t.Fatalf("expected 1,100 km and the toll still paid (30 €), got %.0f km / %s €", sum.TotalDistanceKm, sum.TollsCost)
	}
	alloc, _ := repo.GetTollExpensesForDrives(ctx, v.ID, []string{drives[10].ID})
	if alloc[drives[10].ID] != 3000 {
		t.Fatalf("expected the group toll fully allocated to the remaining drive, got %v", alloc)
	}

	// Guard: an empty API answer over the whole history does not wipe it.
	res, err = repo.ReconcileTeslaMateRecords(ctx, "drives", v.ID, nil, nil)
	if err != nil || !res.Skipped || res.Marked != 0 {
		t.Fatalf("expected mass deletion to be skipped, got %+v (err %v)", res, err)
	}

	// The drive comes back in TeslaMate: restored with its links.
	mustDrive(t, repo, v.ID, 12, drives[11].StartTime, 11200, 100)
	if _, total, _ = repo.ListDrives(ctx, v.ID, database.DriveFilter{}, 50, 0); total != 12 {
		t.Fatalf("expected restored drive, got %d drives", total)
	}
	alloc, _ = repo.GetTollExpensesForDrives(ctx, v.ID, []string{drives[10].ID, drives[11].ID})
	if alloc[drives[10].ID] != 1500 || alloc[drives[11].ID] != 1500 {
		t.Fatalf("expected the group toll split again after restoration, got %v", alloc)
	}
}

func TestIntegrationLedgerAcquisitionAndDepreciation(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "grace@example.com")
	tco := NewTCOService(db.Pool, "Europe/Paris")

	// Bought 24 months ago: 45,000 € − 5,000 € bonus, expected resale 20,000 € after 48 months,
	// odometer 0 at purchase, 30,000 km today (TeslaMate only saw the last 1,000 km).
	purchase := time.Now().AddDate(-2, 0, 0)
	price, bonus, resale, months, odo := money.Cents(4500000), money.Cents(500000), money.Cents(2000000), 48, 0.0
	v.CurrentOdometer = 30000
	if err := repo.UpdateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{
		VehicleID: v.ID, AcquisitionType: models.AcquisitionCash, StartDate: purchase, StartOdometer: &odo,
		PurchasePrice: &price, Incentives: &bonus, ExpectedResaleValue: &resale, ExpectedHoldingMonths: &months,
	}); err != nil {
		t.Fatal(err)
	}
	mustDrive(t, repo, v.ID, 1, time.Now().AddDate(0, 0, -5), 29000, 1000)

	// Lease-style financing and tires bought for 800 €, 4 tires with 25 % of their life used.
	interval := 1
	if err := repo.CreateMaintenanceExpense(ctx, &models.MaintenanceExpense{VehicleID: v.ID, Category: "FINANCING", Amount: 1000,
		Currency: "EUR", Date: time.Now().AddDate(0, -2, 0).Add(-time.Hour), IsRecurring: true, RecurrenceIntervalMonths: &interval,
		Description: "Intérêts crédit"}); err != nil {
		t.Fatal(err)
	}
	mountOdo := 20000.0
	for i := 0; i < 4; i++ {
		pos := []models.TirePosition{models.TirePosFL, models.TirePosFR, models.TirePosRL, models.TirePosRR}[i]
		if err := repo.CreateTire(ctx, &models.Tire{VehicleID: &v.ID, Brand: "M", Model: "X", Dimension: "235", Season: models.TireSeasonSummer,
			PurchaseDate: time.Now().AddDate(-1, 0, 0), PurchasePrice: 20000, CurrentPosition: pos, InitialDepthMm: 8, MinLegalDepthMm: 1.6,
			MountedOdometer: &mountOdo, EstimatedLifespanKm: 40000}); err != nil {
			t.Fatal(err)
		}
	}

	sum, err := tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.AcquisitionCost != 4000000 {
		t.Fatalf("expected acquisition net of bonus 40,000 €, got %s", sum.AcquisitionCost)
	}
	// (40,000 − 20,000) × 24/48 ≈ 10,000 € (± a day of rounding)
	if sum.DepreciationCost < 999000 || sum.DepreciationCost > 1001000 {
		t.Fatalf("expected ~10,000 € depreciation, got %s", sum.DepreciationCost)
	}
	if sum.DistanceBasisKm != 30000 {
		t.Fatalf("expected distance since purchase (30,000 km), got %.0f", sum.DistanceBasisKm)
	}
	if sum.FinancingCost != 3000 || sum.TiresCost != 80000 || sum.TiresAmortizedCost != 20000 {
		t.Fatalf("unexpected financing/tires: %s / %s / %s", sum.FinancingCost, sum.TiresCost, sum.TiresAmortizedCost)
	}
	if sum.TotalCost != 83000 {
		t.Fatalf("expected running cash costs 30 € + 800 €, got %s", sum.TotalCost)
	}
	if sum.FullCost != 3000+20000+sum.DepreciationCost {
		t.Fatalf("full cost must add amortized tires and depreciation, got %s", sum.FullCost)
	}

	// The ledger is the single source: its sum (acquisition excluded) equals the cash total.
	var ledgerTotal money.Cents
	if err := db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_eur), 0) FROM cost_ledger WHERE vehicle_id = $1 AND category <> 'ACQUISITION'`, v.ID).Scan(&ledgerTotal); err != nil {
		t.Fatal(err)
	}
	var monthly money.Cents
	for _, m := range sum.MonthlyCosts {
		monthly += m.Total
	}
	if ledgerTotal != sum.TotalCost || monthly != sum.TotalCost {
		t.Fatalf("ledger %s, monthly %s and total %s must match", ledgerTotal, monthly, sum.TotalCost)
	}
}

func TestIntegrationOdometerContinuity(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "heidi@example.com")
	base := time.Now().UTC().AddDate(0, 0, -10)

	mustDrive(t, repo, v.ID, 1, base, 10000, 50)                  // 10000 → 10050
	mustDrive(t, repo, v.ID, 2, base.Add(2*time.Hour), 10050, 30) // continuous
	mustDrive(t, repo, v.ID, 3, base.Add(4*time.Hour), 10200, 20) // gap of 120 km
	mustDrive(t, repo, v.ID, 4, base.Add(6*time.Hour), 10150, 10) // regression of 70 km
	d5 := mustDrive(t, repo, v.ID, 5, base.Add(8*time.Hour), 10160, 40)
	if _, err := db.Pool.Exec(ctx, `UPDATE drives SET distance_km = 60 WHERE id = $1`, d5.ID); err != nil { // mismatch: 60 vs 40
		t.Fatal(err)
	}

	gaps, gapKm, anomalies, err := repo.DataQualitySummary(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gaps != 1 || gapKm != 120 || anomalies != 2 {
		t.Fatalf("expected 1 gap of 120 km and 2 anomalies, got %d / %.0f / %d", gaps, gapKm, anomalies)
	}
	issues, err := repo.ListDataQualityIssues(ctx, v.ID, 10)
	if err != nil || len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %+v (err %v)", issues, err)
	}
	if issues[0].Type != database.IssueDistanceMismatch || issues[1].Type != database.IssueOdometerRegression || issues[2].Type != database.IssueOdometerGap {
		t.Fatalf("unexpected issue order/types: %+v", issues)
	}

	sum, err := NewTCOService(db.Pool, "UTC").ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Completeness.OdometerGaps != 1 || sum.Completeness.OdometerAnomalies != 2 {
		t.Fatalf("expected continuity issues in completeness, got %+v", sum.Completeness)
	}
}

func TestIntegrationOwnershipLoanLeaseAndInsuranceShare(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	tco := NewTCOService(db.Pool, "UTC")
	rates := NewCarpoolService(db.Pool, repo)
	now := time.Now().UTC()

	// Loan: 30,000 € at 4.8 % over 60 months (payment 563.39 €), started 3 months ago:
	// interest 120.00 € then 118.23 € then 116.45 €.
	loanCar := mustVehicle(t, repo, "loan@example.com")
	price, amount, rate, duration, fees := money.Cents(4500000), money.Cents(3000000), 4.8, 60, money.Cents(15000)
	resale, holding := money.Cents(2000000), 60
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{
		VehicleID: loanCar.ID, AcquisitionType: models.AcquisitionLoan, StartDate: now.AddDate(0, -3, 0).Add(-time.Hour),
		PurchasePrice: &price, ExpectedResaleValue: &resale, ExpectedHoldingMonths: &holding,
		LoanAmount: &amount, LoanRatePct: &rate, LoanDurationMonths: &duration, LoanFees: &fees,
	}); err != nil {
		t.Fatal(err)
	}
	sum, err := tco.ComputeVehicleTCO(ctx, loanCar.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.AcquisitionType != models.AcquisitionLoan || sum.FinancingCost != 15000+12000+11823+11645 {
		t.Fatalf("expected loan fees and 3 interest payments, got %s", sum.FinancingCost)
	}
	if len(sum.MonthlyCosts) > 0 {
		if sum.MonthlyCosts[0].FinancingAmortized != sum.MonthlyCosts[0].Financing-15000+250 {
			t.Fatalf("expected loan fees smoothed in first month, got financing %s amortized %s", sum.MonthlyCosts[0].Financing, sum.MonthlyCosts[0].FinancingAmortized)
		}
	}

	// LOA: 3,000 € down payment, 450 €/month over 36 months, started 12 months ago, 15,000 km/year allowance,
	// maintenance and insurance included, 20,000 km driven.
	leaseCar := mustVehicle(t, repo, "lease@example.com")
	start := now.AddDate(-1, 0, 0).Add(-time.Hour)
	down, rent, leaseMonths, option := money.Cents(300000), money.Cents(45000), 36, money.Cents(2200000)
	allowance, excessPrice, startOdo := 15000.0, 0.10, 0.0
	leaseCar.CurrentOdometer = 20000
	if err := repo.UpdateVehicle(ctx, leaseCar); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{
		VehicleID: leaseCar.ID, AcquisitionType: models.AcquisitionLOA, StartDate: start, StartOdometer: &startOdo,
		LeaseDownPayment: &down, LeaseMonthlyRent: &rent, LeaseDurationMonths: &leaseMonths,
		LeaseKmAllowancePerYear: &allowance, LeaseExcessKmPrice: &excessPrice, LeasePurchaseOptionPrice: &option,
		LeaseIncludesMaintenance: true, LeaseIncludesInsurance: true,
	}); err != nil {
		t.Fatal(err)
	}
	mustDrive(t, repo, leaseCar.ID, 1, now.AddDate(0, -1, 0), 19000, 1000)
	sum, err = tco.ComputeVehicleTCO(ctx, leaseCar.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Cash: down payment + 13 rents (months 0..12)
	if sum.FinancingCost != 300000+13*45000 {
		t.Fatalf("expected down payment and 13 rents, got %s", sum.FinancingCost)
	}
	// Verify monthly costs financing smoothing:
	// Month 0 has cash financing = 300,000 + 45,000 = 345,000 cents (3,450 €)
	// But amortized financing = 45,000 + 300,000/36 = 53,333 cents (533.33 €)
	if len(sum.MonthlyCosts) == 0 {
		t.Fatal("expected monthly costs for lease vehicle")
	}
	firstMonth := sum.MonthlyCosts[0]
	if firstMonth.Financing != 345000 {
		t.Fatalf("expected first month cash financing 345,000 cents, got %s", firstMonth.Financing)
	}
	if firstMonth.FinancingAmortized != 53333 {
		t.Fatalf("expected first month amortized financing 53,333 cents, got %s", firstMonth.FinancingAmortized)
	}
	if len(sum.MonthlyCosts) > 1 {
		secondMonth := sum.MonthlyCosts[1]
		if secondMonth.Financing != 45000 {
			t.Fatalf("expected second month cash financing 45,000 cents, got %s", secondMonth.Financing)
		}
		if secondMonth.FinancingAmortized != 53333 {
			t.Fatalf("expected second month amortized financing 53,333 cents, got %s", secondMonth.FinancingAmortized)
		}
	}
	// Economic: down payment spread (≈ 1/3 consumed) + excess mileage ≈ 5,000 km × 0.10 €
	if sum.FinancingFullCost >= sum.FinancingCost || sum.LeaseExcessKmCost < 40000 || sum.LeaseExcessKmProjected == 0 {
		t.Fatalf("unexpected lease economics: full %s, excess %s, projected %s", sum.FinancingFullCost, sum.LeaseExcessKmCost, sum.LeaseExcessKmProjected)
	}
	if sum.InsuranceSource != InsuranceSourceIncluded || sum.Completeness.InsuranceMissing || sum.Completeness.AcquisitionMissing {
		t.Fatalf("insurance included in the lease must not be reported missing: %+v", sum.Completeness)
	}
	r, err := rates.GetVehicleUnitRates(ctx, leaseCar.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.MaintenanceSource != RateSourceIncluded || r.MaintenancePerKm != 0 || r.InsuranceSource != InsuranceSourceIncluded {
		t.Fatalf("included services must not be charged per km: %+v", r)
	}

	// Insurance share: monthly premium of 60 € started 6 months ago (7 premiums), 3,500 km driven since.
	insured := mustVehicle(t, repo, "insured@example.com")
	interval := 1
	if err := repo.CreateMaintenanceExpense(ctx, &models.MaintenanceExpense{VehicleID: insured.ID, Category: "INSURANCE", Amount: 6000,
		Currency: "EUR", Date: now.AddDate(0, -6, 0).Add(-time.Hour), IsRecurring: true, RecurrenceIntervalMonths: &interval,
		Description: "Prime mensuelle"}); err != nil {
		t.Fatal(err)
	}
	mustDrive(t, repo, insured.ID, 1, now.AddDate(0, -3, 0), 1000, 3500)
	r, err = rates.GetVehicleUnitRates(ctx, insured.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.InsuranceSource != InsuranceSourceRecordedExpenses || r.InsuranceWindowCost == nil || *r.InsuranceWindowCost != 42000 || r.InsurancePerKm != 0.12 {
		t.Fatalf("expected 420 € over 3,500 km = 0.12 €/km, got %+v", r)
	}

	// Sale: recurring premiums stop at the end of ownership.
	end := now.AddDate(0, -2, 0)
	sale := money.Cents(3000000)
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{
		VehicleID: insured.ID, AcquisitionType: models.AcquisitionCash, StartDate: now.AddDate(-1, 0, 0),
		PurchasePrice: &price, EndDate: &end, SalePrice: &sale,
	}); err != nil {
		t.Fatal(err)
	}
	sum, err = tco.ComputeVehicleTCO(ctx, insured.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.InsuranceCost != 5*6000 || sum.DepreciationCost != 1500000 {
		t.Fatalf("expected 5 premiums until the sale and realized depreciation 15,000 €, got %s / %s", sum.InsuranceCost, sum.DepreciationCost)
	}
}

func TestIntegrationEditCapabilities(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "edit@example.com")
	other := mustVehicle(t, repo, "intruder@example.com")
	base := time.Now().UTC().AddDate(0, -1, 0)

	// Set of 4 tires created mounted with rough values, then fixed in one batch.
	odo := 5000.0
	var ids []string
	for _, pos := range []models.TirePosition{models.TirePosFL, models.TirePosFR, models.TirePosRL, models.TirePosRR} {
		tire := &models.Tire{VehicleID: &v.ID, Brand: "X", Model: "Y", Dimension: "235", Season: models.TireSeasonSummer,
			PurchaseDate: time.Now(), PurchasePrice: 1, CurrentPosition: pos, InitialDepthMm: 8, MinLegalDepthMm: 1.6,
			MountedOdometer: &odo, EstimatedLifespanKm: 40000}
		if err := repo.CreateTire(ctx, tire); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, tire.ID)
	}
	brand, total, mounted, mountedOdo := "Michelin", money.Cents(99999), base, 1200.0
	if err := repo.BatchUpdateTires(ctx, v.ID, ids, database.TirePatch{Brand: &brand, TotalPrice: &total, MountedDate: &mounted, MountedOdometer: &mountedOdo}); err != nil {
		t.Fatal(err)
	}
	var sum money.Cents
	for _, id := range ids {
		tire, _ := repo.GetTireByID(ctx, id, v.ID)
		sum += tire.PurchasePrice
		if tire.Brand != "Michelin" || tire.MountedOdometer == nil || *tire.MountedOdometer != 1200 {
			t.Fatalf("batch update not applied: %+v", tire)
		}
	}
	if sum != 99999 {
		t.Fatalf("total price must be split exactly, got %s", sum)
	}
	if err := repo.BatchUpdateTires(ctx, other.ID, ids, database.TirePatch{Brand: &brand}); !errors.Is(err, database.ErrForeignReference) {
		t.Fatalf("batch update of foreign tires must be rejected, got %v", err)
	}

	// Dispose a mounted tire: session closed, DISPOSED, amortized cost fully counted.
	at, disposeOdo := time.Now().UTC(), 1100.0
	var vErr *apierror.Error
	if err := repo.DisposeTire(ctx, v.ID, ids[0], at, &disposeOdo); !errors.As(err, &vErr) {
		t.Fatalf("odometer below the mount odometer must be rejected, got %v", err)
	}
	disposeOdo = 21200
	if err := repo.DisposeTire(ctx, v.ID, ids[0], at, &disposeOdo); err != nil {
		t.Fatal(err)
	}
	disposed, _ := repo.GetTireByID(ctx, ids[0], v.ID)
	if disposed.CurrentPosition != models.TirePosDisposed || disposed.AccumulatedDistanceKm != 20000 || disposed.MountedOdometer != nil {
		t.Fatalf("unexpected disposed tire: %+v", disposed)
	}

	// Wear logs can be corrected and deleted, not across vehicles.
	log := &models.TireLog{TireID: ids[1], Date: base, Odometer: 3000, DepthMm: 7.5}
	if err := repo.AddTireLog(ctx, log); err != nil {
		t.Fatal(err)
	}
	log.DepthMm = 6.9
	if err := repo.UpdateTireLog(ctx, other.ID, log); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("foreign log update must be rejected, got %v", err)
	}
	if err := repo.UpdateTireLog(ctx, v.ID, log); err != nil {
		t.Fatal(err)
	}
	if logs, _ := repo.ListTireLogs(ctx, ids[1]); len(logs) != 1 || logs[0].DepthMm != 6.9 {
		t.Fatalf("log not updated: %+v", logs)
	}
	if err := repo.DeleteTireLog(ctx, v.ID, ids[1], log.ID); err != nil {
		t.Fatal(err)
	}

	// Delete a tire entered by mistake.
	if err := repo.DeleteTire(ctx, other.ID, ids[3]); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("foreign tire delete must be rejected, got %v", err)
	}
	if err := repo.DeleteTire(ctx, v.ID, ids[3]); err != nil {
		t.Fatal(err)
	}

	// Trip group: rename, change drives, delete while keeping or deleting its expenses.
	d1 := mustDrive(t, repo, v.ID, 1, base, 10000, 100)
	d2 := mustDrive(t, repo, v.ID, 2, base.Add(2*time.Hour), 10100, 100)
	d3 := mustDrive(t, repo, v.ID, 3, base.Add(4*time.Hour), 10200, 200)
	exp := &models.DriveExpense{VehicleID: v.ID, Type: "TOLL", Amount: 3000, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, exp, []string{d1.ID, d2.ID}, "Aller"); err != nil {
		t.Fatal(err)
	}
	tg := &models.TripGroup{ID: *exp.TripGroupID, VehicleID: v.ID, Name: "Vacances - aller"}
	if err := repo.UpdateTripGroup(ctx, tg, []string{d1.ID, d2.ID, d3.ID}); err != nil {
		t.Fatal(err)
	}
	groups, _ := repo.ListTripGroups(ctx, v.ID)
	if len(groups) != 1 || groups[0].Name != "Vacances - aller" || len(groups[0].DriveIDs) != 3 || groups[0].DistanceKm != 400 || groups[0].ExpensesTotal != 3000 {
		t.Fatalf("unexpected trip group summary: %+v", groups)
	}
	if groups[0].TollsTotal != 3000 {
		t.Fatalf("the group toll is shared over the drives of the trip, got %s", groups[0].TollsTotal)
	}
	// A toll entered on one drive of the trip counts in the trip's tolls too, without becoming a group expense.
	driveToll := &models.DriveExpense{VehicleID: v.ID, Type: "TOLL", Amount: 1030, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, driveToll, []string{d3.ID}, ""); err != nil {
		t.Fatal(err)
	}
	groups, _ = repo.ListTripGroups(ctx, v.ID)
	if groups[0].TollsTotal != 4030 || groups[0].ExpensesTotal != 3000 || groups[0].ExpenseCount != 1 {
		t.Fatalf("expected 40.30 tolls of which 30.00 attached to the group, got %+v", groups[0])
	}
	if err := repo.DeleteDriveExpense(ctx, v.ID, driveToll.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateTripGroup(ctx, tg, []string{}); !errors.As(err, &vErr) {
		t.Fatalf("an empty trip group must be rejected, got %v", err)
	}
	if err := repo.DeleteTripGroup(ctx, v.ID, tg.ID, false); err != nil {
		t.Fatal(err)
	}
	expenses, _ := repo.ListDriveExpenses(ctx, v.ID, "en")
	if len(expenses) != 1 || expenses[0].TripGroupID != nil {
		t.Fatalf("the expense must be kept unlinked, got %+v", expenses)
	}
	sumTCO, err := NewTCOService(db.Pool, "UTC").ComputeVehicleTCO(ctx, v.ID)
	if err != nil || sumTCO.TollsCost != 3000 {
		t.Fatalf("the unlinked toll must still count in the TCO, got %v (err %v)", sumTCO.TollsCost, err)
	}
}

func TestIntegrationCarpoolLegsAndStops(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "carpool@example.com")
	other := mustVehicle(t, repo, "other-carpool@example.com")
	svc := NewCarpoolService(db.Pool, repo)
	base := time.Now().UTC().AddDate(0, 0, -2)

	// Annecy → Chambéry (50 km) → Grenoble (60 km) → Valence (90 km); a 40 € toll over the whole trip group
	addr := func(s string) *string { return &s }
	var drives []*models.Drive
	for i, leg := range []struct {
		from, to string
		km       float64
	}{{"Annecy, France", "Chambéry, France", 50}, {"Chambéry, France", "Grenoble, France", 60}, {"Grenoble, France", "Valence, France", 90}} {
		d := mustDrive(t, repo, v.ID, i+1, base.Add(time.Duration(i)*2*time.Hour), 10000+float64(i)*100, leg.km)
		if _, err := db.Pool.Exec(ctx, `UPDATE drives SET start_address = $1, end_address = $2 WHERE id = $3`, leg.from, leg.to, d.ID); err != nil {
			t.Fatal(err)
		}
		d.StartAddress, d.EndAddress = addr(leg.from), addr(leg.to)
		drives = append(drives, d)
	}
	toll := &models.DriveExpense{VehicleID: v.ID, Type: "TOLL", Amount: 4000, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, toll, []string{drives[0].ID, drives[1].ID, drives[2].ID}, "Alpes"); err != nil {
		t.Fatal(err)
	}

	est, err := svc.EstimateCosts(ctx, v.ID, nil, nil, []string{drives[2].ID, drives[0].ID, drives[1].ID}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(est.Legs) != 3 || *est.Legs[0].StartLabel != "Annecy" || *est.Legs[2].EndLabel != "Valence" {
		t.Fatalf("expected 3 chronological legs with place labels, got %+v", est.Legs)
	}
	if est.StartDate == nil || !est.StartDate.Truncate(time.Microsecond).Equal(base.Truncate(time.Microsecond)) {
		t.Fatalf("expected est.StartDate %v, got %v", base, est.StartDate)
	}
	if est.Legs[0].TollsCost != 1000 || est.Legs[1].TollsCost != 1200 || est.Legs[2].TollsCost != 1800 || est.TollsCost != 4000 {
		t.Fatalf("expected the group toll split 10/12/18 € across legs, got %d %d %d", est.Legs[0].TollsCost, est.Legs[1].TollsCost, est.Legs[2].TollsCost)
	}

	// Anna rides Annecy → Valence, Bruno boards at Grenoble (stop 2)
	trip := &models.CarpoolTrip{VehicleID: v.ID, Title: "Annecy → Valence", Date: base}
	passengers := []models.CarpoolPassenger{
		{PassengerName: "Anna", Seats: 1, AmountPaid: 2500, BoardStopIndex: 0, AlightStopIndex: 3},
		{PassengerName: "Bruno", Seats: 1, AmountPaid: 1000, BoardStopIndex: 2, AlightStopIndex: 3},
	}
	if err := repo.CreateCarpoolTrip(ctx, trip, est.Legs, passengers); err != nil {
		t.Fatal(err)
	}
	if trip.TotalCost != est.TotalCost || trip.DistanceKm != 200 || trip.TotalRevenue != 3500 {
		t.Fatalf("trip totals must be the sum of its legs: %+v", trip)
	}

	trips, err := repo.ListCarpoolTrips(ctx, v.ID)
	if err != nil || len(trips) != 1 || len(trips[0].Legs) != 3 || len(trips[0].Passengers) != 2 {
		t.Fatalf("expected the trip with 3 legs and 2 passengers, got %+v (err %v)", trips, err)
	}
	loaded := trips[0]
	driver, shared := AllocateCarpoolCosts(loaded.Legs, loaded.Passengers)
	if driver+shared != loaded.TotalCost {
		t.Fatalf("shares must add up to the trip cost")
	}
	// Bruno only pays a third of the last leg; Anna half of the first two legs and a third of the last one
	last := loaded.Legs[2].Total()
	if loaded.Passengers[1].CostShare != money.Split(last, 3)[0] {
		t.Fatalf("Bruno's share must be a third of the last leg (%s), got %s", last, loaded.Passengers[1].CostShare)
	}
	if loaded.Legs[0].PassengerSeats != 1 || loaded.Legs[2].PassengerSeats != 2 {
		t.Fatalf("unexpected occupancy: %d / %d", loaded.Legs[0].PassengerSeats, loaded.Legs[2].PassengerSeats)
	}

	// Recalculate carpool trips: add an extra toll and recalculate
	extraToll := &models.DriveExpense{VehicleID: v.ID, Type: "TOLL", Amount: 2000, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, extraToll, []string{drives[0].ID}, "Tunnel"); err != nil {
		t.Fatal(err)
	}
	recalculated, err := svc.RecalculateTrip(ctx, v.ID, loaded.ID)
	if err != nil {
		t.Fatalf("failed to recalculate trip: %v", err)
	}
	// Initial total toll was 4000, now 4000 + 2000 = 6000
	if recalculated.TollsCost != 6000 {
		t.Fatalf("expected recalculated tolls cost to be 6000, got %d", recalculated.TollsCost)
	}
	if recalculated.TotalRevenue != 3500 {
		t.Fatalf("passenger revenue must remain untouched, got %d", recalculated.TotalRevenue)
	}

	// Invalid stops and foreign drives are rejected
	var vErr *apierror.Error
	bad := []models.CarpoolPassenger{{PassengerName: "Zoé", Seats: 1, BoardStopIndex: 2, AlightStopIndex: 4}}
	if err := repo.UpdateCarpoolTrip(ctx, trip, est.Legs, bad); !errors.As(err, &vErr) {
		t.Fatalf("expected invalid stops to be rejected, got %v", err)
	}
	foreign := &models.CarpoolTrip{VehicleID: other.ID, Title: "x", Date: base}
	if err := repo.CreateCarpoolTrip(ctx, foreign, est.Legs, nil); !errors.Is(err, database.ErrForeignReference) {
		t.Fatalf("expected foreign drives to be rejected, got %v", err)
	}
}

func TestCarpoolElectricityRecentCharges5Days(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "elec_test@example.com")
	svc := NewCarpoolService(db.Pool, repo)

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	cost1 := money.Cents(1000) // 10 EUR
	cost2 := money.Cents(2000) // 20 EUR

	// Charge 1: 1 day before 'now', 50 kWh, 10 EUR (0.20 €/kWh)
	c1 := &models.ChargeLog{
		VehicleID: v.ID,
		Date:      now.Add(-24 * time.Hour),
		KwhAdded:  50,
		Cost:      &cost1,
		Currency:  "EUR",
	}
	if err := repo.CreateManualCharge(ctx, c1); err != nil {
		t.Fatal(err)
	}

	// Charge 2: 3 days before Charge 1 (within 5-day window), 50 kWh, 20 EUR (0.40 €/kWh)
	c2 := &models.ChargeLog{
		VehicleID: v.ID,
		Date:      c1.Date.Add(-3 * 24 * time.Hour),
		KwhAdded:  50,
		Cost:      &cost2,
		Currency:  "EUR",
	}
	if err := repo.CreateManualCharge(ctx, c2); err != nil {
		t.Fatal(err)
	}

	// 1. Both charges are within 5 days of each other: weighted average (10 + 20) / (50 + 50) = 0.30 €/kWh
	rates, err := svc.GetVehicleUnitRatesAt(ctx, v.ID, &now)
	if err != nil {
		t.Fatal(err)
	}
	if rates.ElectricitySource != RateSourceRecentCharges {
		t.Fatalf("expected RECENT_CHARGES, got %s", rates.ElectricitySource)
	}
	if rates.ElectricityPerKwh != 0.30 {
		t.Fatalf("expected 0.30 €/kWh weighted average, got %f", rates.ElectricityPerKwh)
	}

	// 2. Now move Charge 2 to 7 days before Charge 1 (> 5 days apart)
	if _, err := db.Pool.Exec(ctx, `UPDATE charge_logs SET date = $1 WHERE id = $2`, c1.Date.Add(-7*24*time.Hour), c2.ID); err != nil {
		t.Fatal(err)
	}
	rates2, err := svc.GetVehicleUnitRatesAt(ctx, v.ID, &now)
	if err != nil {
		t.Fatal(err)
	}
	if rates2.ElectricitySource != RateSourceRecentCharges {
		t.Fatalf("expected RECENT_CHARGES, got %s", rates2.ElectricitySource)
	}
	// Only Charge 1 should be used: 10 EUR / 50 kWh = 0.20 €/kWh
	if rates2.ElectricityPerKwh != 0.20 {
		t.Fatalf("expected 0.20 €/kWh (only latest charge), got %f", rates2.ElectricityPerKwh)
	}
}

func TestCarpoolMonthlyInsuranceAllocation(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "ins_test@example.com")
	svc := NewCarpoolService(db.Pool, repo)

	// 1. Annual insurance of 365.25 EUR (36525 cents) -> Monthly insurance: 30.44 EUR (3044 cents)
	day1 := time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	interval := 12
	if err := repo.CreateMaintenanceExpense(ctx, &models.MaintenanceExpense{
		VehicleID:                v.ID,
		Category:                 "INSURANCE",
		Amount:                   36525,
		Currency:                 "EUR",
		Date:                     day1.AddDate(0, -1, 0),
		IsRecurring:              true,
		RecurrenceIntervalMonths: &interval,
		Description:              "Assurance annuelle",
	}); err != nil {
		t.Fatal(err)
	}

	// June 2026 drives (past completed month):
	// d1: carpool leg 1 (40 km)
	// d2: carpool leg 2 (60 km)
	// d3: personal commute (100 km)
	// d4: next day drive (50 km)
	// Total June km = 250 km
	d1 := mustDrive(t, repo, v.ID, 201, day1, 10000, 40)
	d2 := mustDrive(t, repo, v.ID, 202, day1.Add(4*time.Hour), 10040, 60)
	mustDrive(t, repo, v.ID, 203, day1.Add(8*time.Hour), 10100, 100)
	d4 := mustDrive(t, repo, v.ID, 204, day2, 10200, 50)

	// 2. Estimate Day 1 carpool with [d1, d2]
	est1, err := svc.EstimateCosts(ctx, v.ID, nil, nil, []string{d1.ID, d2.ID}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if est1.MonthlyInsuranceCost == nil || *est1.MonthlyInsuranceCost != 3044 {
		t.Fatalf("expected MonthlyInsuranceCost to be 3044 cents (~30.44 EUR), got %v", est1.MonthlyInsuranceCost)
	}
	if len(est1.Legs) != 2 {
		t.Fatalf("expected 2 legs, got %d", len(est1.Legs))
	}
	// Monthly insurance: 3044 cents allocated across 250 km
	// Leg 1: 40 km / 250 km * 30.44 EUR = 4.87 EUR = 487 cents
	if est1.Legs[0].InsuranceCost != 487 {
		t.Fatalf("expected Leg 1 insurance 487 cents, got %d", est1.Legs[0].InsuranceCost)
	}
	// Leg 2: 60 km / 250 km * 30.44 EUR = 7.31 EUR = 731 cents
	if est1.Legs[1].InsuranceCost != 731 {
		t.Fatalf("expected Leg 2 insurance 731 cents, got %d", est1.Legs[1].InsuranceCost)
	}
	// Total insurance for trip = 487 + 731 = 1218 cents (12.18 EUR)
	if est1.InsuranceCost != 1218 {
		t.Fatalf("expected Total trip insurance 1218 cents, got %d", est1.InsuranceCost)
	}
	// Effective insurance rate: 12.18 EUR / 100 km = 0.122 EUR/km
	if est1.InsuranceRatePerKm != 0.122 {
		t.Fatalf("expected InsuranceRatePerKm 0.122, got %f", est1.InsuranceRatePerKm)
	}

	// 3. Estimate Day 2 carpool with [d4] (50 km)
	est2, err := svc.EstimateCosts(ctx, v.ID, nil, nil, []string{d4.ID}, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Leg 1: 50 km / 250 km * 30.44 EUR = 6.09 EUR = 609 cents
	if est2.Legs[0].InsuranceCost != 609 || est2.InsuranceCost != 609 {
		t.Fatalf("expected Day 2 insurance 609 cents, got %d (leg: %d)", est2.InsuranceCost, est2.Legs[0].InsuranceCost)
	}

	// 4. Multi-day carpool trip [d1 (day 1), d4 (day 2)]
	estMulti, err := svc.EstimateCosts(ctx, v.ID, nil, nil, []string{d1.ID, d4.ID}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if estMulti.Legs[0].InsuranceCost != 487 {
		t.Fatalf("expected multi-day leg 1 insurance 487 cents, got %d", estMulti.Legs[0].InsuranceCost)
	}
	if estMulti.Legs[1].InsuranceCost != 609 {
		t.Fatalf("expected multi-day leg 2 insurance 609 cents, got %d", estMulti.Legs[1].InsuranceCost)
	}
	if estMulti.InsuranceCost != 1096 {
		t.Fatalf("expected multi-day total insurance 1096 cents, got %d", estMulti.InsuranceCost)
	}

	// 5. Create trip and verify RecalculateTrip after driving more personal km in June 2026
	trip := &models.CarpoolTrip{
		VehicleID: v.ID,
		Title:     "Covoit Day 1",
		Date:      day1,
	}
	if err := repo.CreateCarpoolTrip(ctx, trip, est1.Legs, nil); err != nil {
		t.Fatal(err)
	}

	// Personal drive of 200 km added in June: total June km is now 450 km
	mustDrive(t, repo, v.ID, 205, day1.Add(10*time.Hour), 10300, 200)

	recalculated, err := svc.RecalculateTrip(ctx, v.ID, trip.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Leg 1: 40 km / 450 km * 30.44 EUR = 2.71 EUR = 271 cents
	if recalculated.Legs[0].InsuranceCost != 271 {
		t.Fatalf("expected recalculated leg 1 insurance 271 cents, got %d", recalculated.Legs[0].InsuranceCost)
	}
	// Leg 2: 60 km / 450 km * 30.44 EUR = 4.06 EUR = 406 cents
	if recalculated.Legs[1].InsuranceCost != 406 {
		t.Fatalf("expected recalculated leg 2 insurance 406 cents, got %d", recalculated.Legs[1].InsuranceCost)
	}
	if recalculated.InsuranceCost != 677 {
		t.Fatalf("expected recalculated total insurance 677 cents, got %d", recalculated.InsuranceCost)
	}

	// 6. Lease with insurance included
	leaseV := mustVehicle(t, repo, "lease_ins@example.com")
	leaseStart := day1.AddDate(-1, 0, 0)
	price := money.Cents(4000000)
	leaseDuration := 36
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{
		VehicleID:              leaseV.ID,
		AcquisitionType:        models.AcquisitionLLD,
		StartDate:              leaseStart,
		PurchasePrice:          &price,
		LeaseDurationMonths:    &leaseDuration,
		LeaseIncludesInsurance: true,
	}); err != nil {
		t.Fatal(err)
	}
	leaseDrive := mustDrive(t, repo, leaseV.ID, 301, day1, 1000, 100)
	leaseEst, err := svc.EstimateCosts(ctx, leaseV.ID, nil, nil, []string{leaseDrive.ID}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if leaseEst.InsuranceSource != RateSourceIncluded {
		t.Fatalf("expected INCLUDED_IN_LEASE, got %s", leaseEst.InsuranceSource)
	}
	if leaseEst.InsuranceCost != 0 || leaseEst.Legs[0].InsuranceCost != 0 {
		t.Fatalf("expected 0 insurance cost when included in lease, got %d", leaseEst.InsuranceCost)
	}
}

func TestIntegrationMaintenanceAmortizationAndOdometer(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "maint_test@example.com")
	loc := time.UTC
	base := time.Date(2024, time.March, 1, 10, 0, 0, 0, loc)

	// 1. Odometer resolution tests
	mustDrive(t, repo, v.ID, 101, base.Add(-10*24*time.Hour), 48000, 50)
	mustDrive(t, repo, v.ID, 102, base, 48500, 100)

	stored, err := repo.ListOdometerAnchors(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	anchors := make([]OdometerAnchor, len(stored))
	for i, a := range stored {
		anchors[i] = OdometerAnchor{Date: a.Date, Km: a.Km, Origin: a.Origin}
	}
	odo1, _, _ := EstimateOdometerAt(anchors, base.Add(-5*24*time.Hour))
	if odo1 <= 48050 || odo1 >= 48400 {
		t.Fatalf("expected an odometer between the two drives, got %v", odo1)
	}
	odo2, _, _ := EstimateOdometerAt(anchors, base.Add(2*time.Hour))
	if odo2 != 48600 {
		t.Fatalf("expected 48600 odometer after second drive, got %v", odo2)
	}

	// 2. Create maintenance expenses:
	// m1: 50,000 km revision (500 €) amortized over 50,000 km
	covKm := 50000.0
	covMonths := 24
	m1 := &models.MaintenanceExpense{
		VehicleID:        v.ID,
		Category:         "MAINTENANCE",
		Amount:           50000, // 500.00 €
		Currency:         "EUR",
		Date:             base,
		Odometer:         &odo2,
		AmortizationMode: "DISTANCE",
		CoverageKm:       &covKm,
		Description:      "Grande Révision 50k",
	}
	if err := repo.CreateMaintenanceExpense(ctx, m1); err != nil {
		t.Fatal(err)
	}

	// m2: Wiper blades (40 €) immediate (NONE)
	m2 := &models.MaintenanceExpense{
		VehicleID:        v.ID,
		Category:         "MAINTENANCE",
		Amount:           4000, // 40.00 €
		Currency:         "EUR",
		Date:             base.Add(35 * 24 * time.Hour), // April 2024
		AmortizationMode: "NONE",
		Description:      "Balais essuie-glaces",
	}
	if err := repo.CreateMaintenanceExpense(ctx, m2); err != nil {
		t.Fatal(err)
	}

	// m3: Revision 80,000 km (600 €) amortized over 50,000 km which closes m1
	m3 := &models.MaintenanceExpense{
		VehicleID:           v.ID,
		Category:            "MAINTENANCE",
		Amount:              60000, // 600.00 €
		Currency:            "EUR",
		Date:                base.Add(180 * 24 * time.Hour), // September 2024
		AmortizationMode:    "DISTANCE",
		CoverageKm:          &covKm,
		CoverageMonths:      &covMonths,
		ClosesMaintenanceID: &m1.ID,
		Description:         "Révision 80k",
	}
	if err := repo.CreateMaintenanceExpense(ctx, m3); err != nil {
		t.Fatal(err)
	}

	list, err := repo.ListMaintenanceExpenses(ctx, v.ID)
	if err != nil || len(list) != 3 {
		t.Fatalf("expected 3 maintenance expenses, got %d (err %v)", len(list), err)
	}

	tcoSvc := NewTCOService(db.Pool, "UTC")
	monthlyDistances := map[string]float64{
		"2024-03": 1000,
		"2024-04": 2000,
		"2024-05": 2000,
		"2024-09": 1500,
	}
	now := base.Add(200 * 24 * time.Hour)
	maintMap, err := tcoSvc.computeMonthlyMaintenanceAmortization(ctx, v.ID, monthlyDistances, now)
	if err != nil {
		t.Fatalf("computeMonthlyMaintenanceAmortization failed: %v", err)
	}

	// Only maintenance and repairs are smoothed: an insurance or accessory row carrying a mode must not feed it.
	insurance := &models.MaintenanceExpense{
		VehicleID: v.ID, Category: "INSURANCE", Amount: 600, Currency: "EUR", Date: base,
		AmortizationMode: "DISTANCE", CoverageKm: &covKm, CoverageMonths: &covMonths,
	}
	if err := repo.CreateMaintenanceExpense(ctx, insurance); err != nil {
		t.Fatal(err)
	}
	accessory := &models.MaintenanceExpense{
		VehicleID: v.ID, Category: "ACCESSORY", Amount: 90, Currency: "EUR", Date: base,
		AmortizationMode: "NONE",
	}
	if err := repo.CreateMaintenanceExpense(ctx, accessory); err != nil {
		t.Fatal(err)
	}
	maintMap, err = tcoSvc.computeMonthlyMaintenanceAmortization(ctx, v.ID, monthlyDistances, now)
	if err != nil {
		t.Fatal(err)
	}

	// March 2024: 1000 km * (50000 / 50000) = 1000 cents (10.00 €)
	if maintMap["2024-03"] != 1000 {
		t.Fatalf("expected 1000 cents in 2024-03, got %d", maintMap["2024-03"])
	}

	// April 2024: 2000 km * 1.00 cent/km = 2000 cents + m2 (4000 cents) = 6000 cents (60.00 €)
	if maintMap["2024-04"] != 6000 {
		t.Fatalf("expected 6000 cents in 2024-04, got %d", maintMap["2024-04"])
	}

	// September 2024: m1 was closed by m3, so m1 stops! m3 starts: 1500 km * (60000 / 50000 = 1.20) = 1800 cents
	if maintMap["2024-09"] != 1800 {
		t.Fatalf("expected 1800 cents in 2024-09, got %d", maintMap["2024-09"])
	}
}

func TestExpenseDocumentsIntegration(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	_ = db
	v := mustVehicle(t, repo, "user_doc@example.com")

	// 1. Create a document (metadata only; binary stored on volume)
	pdfData := []byte("%PDF-1.4 test document content for invoice")
	desc := "Facture Révision Tesla Chambourcy"
	doc := &models.ExpenseDocument{
		UserID:      v.UserID,
		VehicleID:   v.ID,
		Filename:    "facture_chambourcy.pdf",
		MimeType:    "application/pdf",
		FileSize:    int64(len(pdfData)),
		Description: &desc,
	}
	if err := repo.SaveExpenseDocument(ctx, doc); err != nil {
		t.Fatalf("SaveExpenseDocument failed: %v", err)
	}
	if doc.ID == "" {
		t.Fatal("expected generated doc ID")
	}

	// Simulate writing to volume and persisting the storage path.
	fakeStoragePath := v.ID + "/" + doc.ID
	if err := repo.UpdateDocumentStoragePath(ctx, doc.ID, fakeStoragePath); err != nil {
		t.Fatalf("UpdateDocumentStoragePath failed: %v", err)
	}

	// 2. Fetch document by ID — verify metadata and storage path
	fetched, err := repo.GetExpenseDocumentByID(ctx, doc.ID, v.ID, v.UserID)
	if err != nil {
		t.Fatalf("GetExpenseDocumentByID failed: %v", err)
	}
	if fetched.Filename != "facture_chambourcy.pdf" {
		t.Fatalf("fetched doc filename mismatch: %s", fetched.Filename)
	}
	if fetched.StoragePath == nil || *fetched.StoragePath != fakeStoragePath {
		t.Fatalf("fetched doc StoragePath mismatch: %v", fetched.StoragePath)
	}

	// 3. Link document to 2 drive expenses (many-to-one)
	de1 := &models.DriveExpense{
		VehicleID:  v.ID,
		Type:       "TOLL",
		Amount:     1250,
		Currency:   "EUR",
		Date:       time.Now(),
		DocumentID: &doc.ID,
	}
	if err := repo.SaveDriveExpense(ctx, de1, nil, ""); err != nil {
		t.Fatalf("SaveDriveExpense de1 failed: %v", err)
	}
	if de1.DocumentFilename == nil || *de1.DocumentFilename != "facture_chambourcy.pdf" {
		t.Fatalf("expected DocumentFilename to be populated, got %v", de1.DocumentFilename)
	}

	de2 := &models.DriveExpense{
		VehicleID:  v.ID,
		Type:       "TOLL",
		Amount:     850,
		Currency:   "EUR",
		Date:       time.Now(),
		DocumentID: &doc.ID,
	}
	if err := repo.SaveDriveExpense(ctx, de2, nil, ""); err != nil {
		t.Fatalf("SaveDriveExpense de2 failed: %v", err)
	}

	// 4. Link document to 1 maintenance expense
	m := &models.MaintenanceExpense{
		VehicleID:        v.ID,
		Category:         "MAINTENANCE",
		Amount:           35000,
		Currency:         "EUR",
		Date:             time.Now(),
		Description:      "Révision complète",
		AmortizationMode: "DISTANCE",
		DocumentID:       &doc.ID,
	}
	if err := repo.CreateMaintenanceExpense(ctx, m); err != nil {
		t.Fatalf("CreateMaintenanceExpense failed: %v", err)
	}
	if m.DocumentFilename == nil || *m.DocumentFilename != "facture_chambourcy.pdf" {
		t.Fatalf("expected DocumentFilename to be populated, got %v", m.DocumentFilename)
	}

	// 5. Link document to 1 charge log
	chargeCost := money.Cents(1850)
	c := &models.ChargeLog{
		VehicleID:  v.ID,
		Date:       time.Now(),
		KwhAdded:   45.2,
		Cost:       &chargeCost,
		Currency:   "EUR",
		DocumentID: &doc.ID,
	}
	if err := repo.CreateManualCharge(ctx, c); err != nil {
		t.Fatalf("CreateManualCharge failed: %v", err)
	}
	if c.DocumentFilename == nil || *c.DocumentFilename != "facture_chambourcy.pdf" {
		t.Fatalf("expected DocumentFilename to be populated, got %v", c.DocumentFilename)
	}

	// 6. List documents - verify linked expenses count = 4
	docs, err := repo.ListExpenseDocuments(ctx, v.ID, v.UserID)
	if err != nil {
		t.Fatalf("ListExpenseDocuments failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if docs[0].LinkedExpensesCount != 4 {
		t.Fatalf("expected LinkedExpensesCount=4 (2 tolls + 1 maint + 1 charge), got %d", docs[0].LinkedExpensesCount)
	}

	// 7. Verify listing includes DocumentID and DocumentFilename
	tolls, err := repo.ListDriveExpenses(ctx, v.ID, "en")
	if err != nil || len(tolls) != 2 {
		t.Fatalf("expected 2 tolls, got %d (err %v)", len(tolls), err)
	}
	for _, toll := range tolls {
		if toll.DocumentID == nil || *toll.DocumentID != doc.ID {
			t.Fatalf("expected toll DocumentID %s, got %v", doc.ID, toll.DocumentID)
		}
		if toll.DocumentFilename == nil || *toll.DocumentFilename != "facture_chambourcy.pdf" {
			t.Fatalf("expected toll DocumentFilename 'facture_chambourcy.pdf', got %v", toll.DocumentFilename)
		}
	}

	maints, err := repo.ListMaintenanceExpenses(ctx, v.ID)
	if err != nil || len(maints) != 1 {
		t.Fatalf("expected 1 maintenance, got %d (err %v)", len(maints), err)
	}
	if maints[0].DocumentID == nil || *maints[0].DocumentID != doc.ID {
		t.Fatalf("expected maintenance DocumentID %s, got %v", doc.ID, maints[0].DocumentID)
	}

	charges, _, err := repo.ListCharges(ctx, v.ID, false, 50, 0)
	if err != nil || len(charges) != 1 {
		t.Fatalf("expected 1 charge, got %d (err %v)", len(charges), err)
	}
	if charges[0].DocumentID == nil || *charges[0].DocumentID != doc.ID {
		t.Fatalf("expected charge DocumentID %s, got %v", doc.ID, charges[0].DocumentID)
	}

	// 8. Delete document and verify ON DELETE SET NULL on all linked expenses
	if err := repo.DeleteExpenseDocument(ctx, doc.ID, v.ID, v.UserID); err != nil {
		t.Fatalf("DeleteExpenseDocument failed: %v", err)
	}

	tollsAfter, err := repo.ListDriveExpenses(ctx, v.ID, "en")
	if err != nil || len(tollsAfter) != 2 {
		t.Fatalf("expected tolls to be preserved, got %d (err %v)", len(tollsAfter), err)
	}
	for _, toll := range tollsAfter {
		if toll.DocumentID != nil {
			t.Fatalf("expected toll DocumentID to be cleared to nil, got %v", *toll.DocumentID)
		}
	}

	maintsAfter, err := repo.ListMaintenanceExpenses(ctx, v.ID)
	if err != nil || len(maintsAfter) != 1 {
		t.Fatalf("expected maintenance to be preserved, got %d (err %v)", len(maintsAfter), err)
	}
	if maintsAfter[0].DocumentID != nil {
		t.Fatalf("expected maintenance DocumentID to be cleared to nil, got %v", *maintsAfter[0].DocumentID)
	}

	chargesAfter, _, err := repo.ListCharges(ctx, v.ID, false, 50, 0)
	if err != nil || len(chargesAfter) != 1 {
		t.Fatalf("expected charge to be preserved, got %d (err %v)", len(chargesAfter), err)
	}
	if chargesAfter[0].DocumentID != nil {
		t.Fatalf("expected charge DocumentID to be cleared to nil, got %v", *chargesAfter[0].DocumentID)
	}
}

func TestMigrateLegacyDocumentsIntegration(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "legacy_user@example.com")

	legacyBytes := []byte("Legacy PDF binary content stored directly in PostgreSQL BYTEA")

	// Insert legacy document with data BYTEA and storage_path = NULL
	var docID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO expense_documents (user_id, vehicle_id, filename, mime_type, file_size, data, storage_path)
		VALUES ($1, $2, 'facture_legacy.pdf', 'application/pdf', $3, $4, NULL)
		RETURNING id::text;
	`, v.UserID, v.ID, len(legacyBytes), legacyBytes).Scan(&docID)
	if err != nil {
		t.Fatalf("failed to insert legacy document: %v", err)
	}

	// Verify GetExpenseDocumentByID retrieves Data even when storage_path is NULL
	fetched, err := repo.GetExpenseDocumentByID(ctx, docID, v.ID, v.UserID)
	if err != nil {
		t.Fatalf("GetExpenseDocumentByID failed: %v", err)
	}
	if fetched.StoragePath != nil {
		t.Fatalf("expected StoragePath to be nil, got %v", *fetched.StoragePath)
	}
	if string(fetched.Data) != string(legacyBytes) {
		t.Fatalf("expected legacy Data %q, got %q", string(legacyBytes), string(fetched.Data))
	}

	// Run migration
	savedCalls := make(map[string][]byte)
	count, err := repo.MigrateLegacyDocuments(ctx, func(vehicleID, id string, data []byte) (string, error) {
		savedCalls[id] = data
		return vehicleID + "/" + id, nil
	})
	if err != nil {
		t.Fatalf("MigrateLegacyDocuments failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 document migrated, got %d", count)
	}
	if string(savedCalls[docID]) != string(legacyBytes) {
		t.Fatalf("saved bytes mismatch: got %q", string(savedCalls[docID]))
	}

	// Verify document now has storage_path in database
	fetchedAfter, err := repo.GetExpenseDocumentByID(ctx, docID, v.ID, v.UserID)
	if err != nil {
		t.Fatalf("GetExpenseDocumentByID failed after migration: %v", err)
	}
	expectedPath := v.ID + "/" + docID
	if fetchedAfter.StoragePath == nil || *fetchedAfter.StoragePath != expectedPath {
		t.Fatalf("expected storage path %q, got %v", expectedPath, fetchedAfter.StoragePath)
	}

	// Migration is idempotent
	countSecond, err := repo.MigrateLegacyDocuments(ctx, func(vehicleID, id string, data []byte) (string, error) {
		return vehicleID + "/" + id, nil
	})
	if err != nil {
		t.Fatalf("MigrateLegacyDocuments second run failed: %v", err)
	}
	if countSecond != 0 {
		t.Fatalf("expected 0 documents migrated on second run, got %d", countSecond)
	}
}

func TestRecordSyncFailureAlertsOnlyOnTransitionToOpen(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	v := mustVehicle(t, repo, "circuit-alert@example.com")

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := repo.UpsertVehicleWebhook(context.Background(), &models.VehicleWebhook{
		VehicleID: v.ID,
		URL:       server.URL,
		Type:      "GENERIC",
		Enabled:   true,
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewSyncService(repo, nil)
	svc.SetNotificationService(NewNotificationService(repo))
	cb := NewCircuitBreaker(CircuitBreakerConfig{FailureThreshold: 2, Cooldown: time.Hour})
	svc.SetCircuitBreaker(v.ID, cb)

	svc.recordSyncFailure(*v, cb, errors.New("boom 1")) // below threshold: no alert yet
	svc.recordSyncFailure(*v, cb, errors.New("boom 2")) // trips the breaker to OPEN: alert fires once
	svc.recordSyncFailure(*v, cb, errors.New("boom 3")) // already OPEN: no additional alert

	// The alert is dispatched from a background goroutine (best-effort); give it a moment.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&callCount) == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("expected exactly 1 webhook call when the circuit breaker opens, got %d", got)
	}
	if cb.State() != CircuitOpen {
		t.Fatalf("expected circuit breaker to be OPEN, got %s", cb.State())
	}
}

func TestIntegrationTollSourceAndHasTollFilter(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "toll@example.com")

	base := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	manual := mustDrive(t, repo, v.ID, 1, base, 1000, 100)
	auto := mustDrive(t, repo, v.ID, 2, base.Add(3*time.Hour), 1100, 100)
	grouped1 := mustDrive(t, repo, v.ID, 3, base.Add(6*time.Hour), 1200, 100)
	grouped2 := mustDrive(t, repo, v.ID, 4, base.Add(9*time.Hour), 1300, 100)
	none := mustDrive(t, repo, v.ID, 5, base.Add(12*time.Hour), 1400, 100)

	// Expenses saved without a source default to MANUAL.
	m := &models.DriveExpense{VehicleID: v.ID, DriveID: &manual.ID, Type: "TOLL", Amount: 1000, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, m, nil, ""); err != nil {
		t.Fatal(err)
	}
	a := &models.DriveExpense{VehicleID: v.ID, DriveID: &auto.ID, Type: "TOLL", Amount: 2000, Currency: "EUR", Date: base, Source: models.ExpenseSourceAutoToll}
	if err := repo.SaveDriveExpense(ctx, a, nil, ""); err != nil {
		t.Fatal(err)
	}
	g := &models.DriveExpense{VehicleID: v.ID, Type: "TOLL", Amount: 3000, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, g, []string{grouped1.ID, grouped2.ID}, "Voyage"); err != nil {
		t.Fatal(err)
	}
	parking := &models.DriveExpense{VehicleID: v.ID, DriveID: &none.ID, Type: "PARKING", Amount: 500, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, parking, nil, ""); err != nil {
		t.Fatal(err)
	}

	if m.Source != models.ExpenseSourceManual {
		t.Fatalf("expected default source MANUAL, got %q", m.Source)
	}
	perDrive, err := repo.GetDriveExpensesByDriveID(ctx, v.ID, auto.ID, "en")
	if err != nil || len(perDrive) != 1 || perDrive[0].Source != models.ExpenseSourceAutoToll {
		t.Fatalf("expected the auto expense to expose its source, got %+v (err %v)", perDrive, err)
	}

	ids := func(f database.DriveFilter) map[string]bool {
		list, _, err := repo.ListDrives(ctx, v.ID, f, 50, 0)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, d := range list {
			out[d.ID] = true
		}
		return out
	}

	all := ids(database.DriveFilter{HasToll: true})
	if len(all) != 4 || !all[manual.ID] || !all[auto.ID] || !all[grouped1.ID] || !all[grouped2.ID] || all[none.ID] {
		t.Fatalf("has-toll filter must match direct and trip-group tolls only (not parking), got %v", all)
	}
	onlyAuto := ids(database.DriveFilter{HasToll: true, TollSource: models.ExpenseSourceAutoToll})
	if len(onlyAuto) != 1 || !onlyAuto[auto.ID] {
		t.Fatalf("expected only the auto-toll drive, got %v", onlyAuto)
	}
	onlyManual := ids(database.DriveFilter{HasToll: true, TollSource: models.ExpenseSourceManual})
	if len(onlyManual) != 3 || onlyManual[auto.ID] {
		t.Fatalf("expected manual toll drives (direct + group), got %v", onlyManual)
	}

	// Editing an auto expense through the manual path hands ownership to the user.
	a.Source = ""
	a.Amount = 2500
	if err := repo.SaveDriveExpense(ctx, a, nil, ""); err != nil {
		t.Fatal(err)
	}
	if a.Source != models.ExpenseSourceManual {
		t.Fatalf("expected a manual edit to make the expense MANUAL, got %q", a.Source)
	}
}

func TestIntegrationComparisonScenarios(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "compare@example.com")
	svc := NewComparisonService(NewTCOService(db.Pool, "Europe/Paris"))

	now := time.Now().UTC()
	mustDrive(t, repo, v.ID, 1, now.AddDate(0, -4, 0), 10000, 3000)
	mustDrive(t, repo, v.ID, 2, now.AddDate(0, -2, 0), 13000, 3000)
	cost := money.Cents(60000) // 600 EUR for 2400 kWh -> 0.25 EUR/kWh
	tm := 1
	if _, err := repo.UpsertTeslaMateCharge(ctx, &models.ChargeLog{VehicleID: v.ID, TeslaMateChargeID: &tm, Date: now.AddDate(0, -1, 0), KwhAdded: 2400, Cost: &cost, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}

	sc := &models.ComparisonScenario{
		UserID: v.UserID, VehicleID: &v.ID, Name: "Vs SUV essence", Mode: models.ComparisonModeRetrospective,
		AnnualKm: 15000, Years: 5,
		ICE: models.ICEInputs{FuelType: "SP95_E10", LPer100Km: 7, FuelPrice: 1.8,
			PurchasePrice: money.FromFloat(30000), ResaleValue: money.FromFloat(12000),
			MaintenanceYearly: money.FromFloat(700), InsuranceYearly: money.FromFloat(650)},
		Options: models.ScenarioOptions{FuelInflationPct: 2},
	}
	if err := repo.CreateComparisonScenario(ctx, sc); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetComparisonScenario(ctx, v.UserID, sc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tracked != nil || got.Options.FuelInflationPct != 2 || got.ICE.FuelPrice != 1.8 || got.ICE.PurchasePrice != money.FromFloat(30000) {
		t.Fatalf("round trip mismatch: %+v", got)
	}

	// The comparison reads the real TCO and writes nothing.
	before, err := NewTCOService(db.Pool, "Europe/Paris").ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Compare(ctx, got)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := NewTCOService(db.Pool, "Europe/Paris").ComputeVehicleTCO(ctx, v.ID)
	if before.TotalCost != after.TotalCost {
		t.Fatalf("comparison changed the TCO: %v -> %v", before.TotalCost, after.TotalCost)
	}
	// EV energy: 0.10 EUR/km (600 EUR / 6000 km) * 15000 km * 5 years = 7500 EUR.
	if res.Tracked.Energy != money.FromFloat(7500) {
		t.Errorf("EV energy = %v, want 7500 from the real charge cost", res.Tracked.Energy)
	}
	if res.ICE.Energy <= money.FromFloat(7*1.8/100*15000*5) { // inflation makes it higher than the flat figure
		t.Errorf("ICE energy = %v, expected fuel inflation to apply", res.ICE.Energy)
	}

	d, err := svc.Defaults(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.TrackedEurPerKwh == nil || *d.TrackedEurPerKwh != 0.25 || len(d.ICE) == 0 {
		t.Errorf("unexpected defaults: %+v", d)
	}

	// PROJECTION scenarios need EV inputs and no vehicle; other users cannot read them.
	proj := &models.ComparisonScenario{
		UserID: v.UserID, Name: "Projection", Mode: models.ComparisonModeProjection, AnnualKm: 12000, Years: 4,
		ICE: sc.ICE, Tracked: &models.TrackedInputs{KwhPer100Km: 16, EurPerKwh: 0.2, PurchasePrice: money.FromFloat(38000), ResaleValue: money.FromFloat(18000)},
	}
	if err := repo.CreateComparisonScenario(ctx, proj); err != nil {
		t.Fatal(err)
	}
	other, err := repo.CreateUser(ctx, "other@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetComparisonScenario(ctx, other.ID, proj.ID); !errors.Is(err, database.ErrNotFound) {
		t.Errorf("foreign user read = %v, want ErrNotFound", err)
	}
	if list, _ := repo.ListComparisonScenarios(ctx, v.UserID); len(list) != 2 {
		t.Errorf("list = %d scenarios, want 2", len(list))
	}

	// The DB constraint rejects a RETROSPECTIVE scenario without vehicle.
	bad := *sc
	bad.VehicleID = nil
	if err := repo.CreateComparisonScenario(ctx, &bad); err == nil {
		t.Error("expected constraint violation for retrospective scenario without vehicle")
	}

	proj.Name = "Projection renommée"
	if err := repo.UpdateComparisonScenario(ctx, proj); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteComparisonScenario(ctx, v.UserID, proj.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteComparisonScenario(ctx, v.UserID, proj.ID); !errors.Is(err, database.ErrNotFound) {
		t.Errorf("second delete = %v, want ErrNotFound", err)
	}

	// Deleting the vehicle removes its RETROSPECTIVE scenarios.
	if _, err := db.Pool.Exec(ctx, `DELETE FROM vehicles WHERE id = $1`, v.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := repo.ListComparisonScenarios(ctx, v.UserID); len(list) != 0 {
		t.Errorf("scenarios must cascade with the vehicle, got %d", len(list))
	}
}

func TestIntegrationICEVehicleFuelLogs(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "ice@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	ice := &models.Vehicle{UserID: u.ID, Name: "Clio", Powertrain: models.PowertrainICE, TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 10000}
	if err := repo.CreateVehicle(ctx, ice); err != nil {
		t.Fatal(err)
	}
	ev := mustVehicle(t, repo, "ev-default@example.com")
	if got, err := repo.GetVehicleByID(ctx, ev.ID, ev.UserID); err != nil || got.Powertrain != models.PowertrainEV {
		t.Fatalf("vehicles default to EV, got %+v (%v)", got, err)
	}
	tco := NewTCOService(db.Pool, "Europe/Paris")

	now := time.Now().UTC()
	l := func(v float64) *float64 { return &v }
	fills := []models.FuelLog{
		{Date: now.AddDate(0, -3, 0), Odometer: l(10000), Amount: money.FromFloat(80), Liters: l(50), IsFullTank: true},
		{Date: now.AddDate(0, -2, 0), Odometer: l(10500), Amount: money.FromFloat(40), Liters: l(25), IsFullTank: true},
		{Date: now.AddDate(0, -1, -15), Odometer: l(10800), Amount: money.FromFloat(20), Liters: l(12), IsFullTank: false},
		{Date: now.AddDate(0, -1, 0), Odometer: l(11100), Amount: money.FromFloat(40), Liters: l(24), IsFullTank: true},
	}
	for i := range fills {
		fills[i].VehicleID = ice.ID
		if err := repo.CreateFuelLog(ctx, &fills[i]); err != nil {
			t.Fatal(err)
		}
	}

	var odo float64
	if err := db.Pool.QueryRow(ctx, `SELECT current_odometer FROM vehicles WHERE id = $1`, ice.ID).Scan(&odo); err != nil || odo != 11100 {
		t.Fatalf("current odometer = %v (%v), want 11100", odo, err)
	}

	sum, err := tco.ComputeVehicleTCO(ctx, ice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Powertrain != models.PowertrainICE || sum.EnergyCost != money.FromFloat(180) {
		t.Errorf("powertrain/energy = %s / %v, want ICE / 180", sum.Powertrain, sum.EnergyCost)
	}
	if sum.DistanceBasisKm != 1100 {
		t.Errorf("distance basis = %v, want the 1100 km odometer span of the fill-ups", sum.DistanceBasisKm)
	}
	if sum.TotalLiters != 111 || sum.FuelFillUps != 4 || sum.ConsumptionL100km == nil || *sum.ConsumptionL100km != 5.545 { // 61 L over 1100 km between full tanks
		t.Errorf("fuel figures: liters=%v fills=%d consumption=%v", sum.TotalLiters, sum.FuelFillUps, sum.ConsumptionL100km)
	}
	if sum.EstimatedEnergyDistanceKm != 0 {
		t.Errorf("a combustion vehicle has no estimated energy distance, got %v", sum.EstimatedEnergyDistanceKm)
	}
	if sum.EnergyCostPerKm <= 0 {
		t.Errorf("energy cost per km = %v, want > 0 without any drive", sum.EnergyCostPerKm)
	}
	var monthlyKm, monthlyEnergy float64
	for _, m := range sum.MonthlyCosts {
		monthlyKm += m.DistanceKm
		monthlyEnergy += m.Energy.Float()
	}
	if monthlyKm < 1099 || monthlyKm > 1101 || monthlyEnergy != 180 {
		t.Errorf("monthly totals: %.1f km, %.2f EUR; want 1100 km and 180 EUR", monthlyKm, monthlyEnergy)
	}
	for _, w := range sum.Completeness.Warnings {
		if strings.Contains(w, "trajets") || strings.Contains(w, "recharge") {
			t.Errorf("unexpected EV warning on a combustion vehicle: %q", w)
		}
	}
	if sum.Completeness.ScorePct < 75 {
		t.Errorf("completeness score = %d, unreasonably low for a vehicle with fill-ups", sum.Completeness.ScorePct)
	}

	// The comparison is built on an electric reference; the combustion vehicle only prefills the ICE side.
	svc := NewComparisonService(tco)
	if _, err := svc.Compare(ctx, &models.ComparisonScenario{Mode: models.ComparisonModeRetrospective, VehicleID: &ice.ID, AnnualKm: 10000, Years: 3}); !errors.Is(err, ErrComparisonNeedsEV) {
		t.Errorf("retrospective on ICE = %v, want ErrComparisonNeedsEV", err)
	}
	d, err := svc.Defaults(ctx, ice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.ICELPer100Km == nil || *d.ICELPer100Km != 5.545 || d.ICEFuelPrice == nil || d.TrackedKwhPer100Km != nil {
		t.Errorf("ICE defaults: %+v", d)
	}

	// Editing, odometer never lowered by a delete, cascade with the vehicle.
	fills[3].Odometer = l(11200)
	if err := repo.UpdateFuelLog(ctx, &fills[3]); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteFuelLog(ctx, ice.ID, fills[3].ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteFuelLog(ctx, ice.ID, fills[3].ID); !errors.Is(err, database.ErrNotFound) {
		t.Errorf("second delete = %v, want ErrNotFound", err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT current_odometer FROM vehicles WHERE id = $1`, ice.ID).Scan(&odo); err != nil || odo != 11200 {
		t.Errorf("current odometer = %v (%v), want 11200 (never lowered)", odo, err)
	}
	if logs, _ := repo.ListFuelLogs(ctx, ice.ID); len(logs) != 3 {
		t.Errorf("fill-ups after delete = %d, want 3", len(logs))
	}
	if _, err := db.Pool.Exec(ctx, `DELETE FROM vehicles WHERE id = $1`, ice.ID); err != nil {
		t.Fatal(err)
	}
	var left int
	_ = db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM fuel_logs`).Scan(&left)
	if left != 0 {
		t.Errorf("fuel logs must cascade with the vehicle, %d left", left)
	}
}

func TestIntegrationManualReadingsAndFillUpsAreIndependent(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "manual@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	ice := &models.Vehicle{UserID: u.ID, Name: "Clio", Powertrain: models.PowertrainICE, TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, ice); err != nil {
		t.Fatal(err)
	}
	ev := &models.Vehicle{UserID: u.ID, Name: "Model 3", TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 20000}
	if err := repo.CreateVehicle(ctx, ev); err != nil {
		t.Fatal(err)
	}
	tco := NewTCOService(db.Pool, "Europe/Paris")

	day := time.Now().UTC().Truncate(24 * time.Hour)
	at := func(daysAgo int) time.Time { return day.AddDate(0, 0, -daysAgo) }
	l := func(v float64) *float64 { return &v }

	// Odometer readings alone (no fill-up): they define the distance and raise the current odometer.
	for _, r := range []models.OdometerCheckpoint{
		{VehicleID: ice.ID, Date: at(60), Odometer: 10000},
		{VehicleID: ice.ID, Date: at(20), Odometer: 11000}, // 25 km/day
	} {
		if err := repo.CreateOdometerCheckpoint(ctx, &r); err != nil {
			t.Fatal(err)
		}
	}
	var odo float64
	if err := db.Pool.QueryRow(ctx, `SELECT current_odometer FROM vehicles WHERE id = $1`, ice.ID).Scan(&odo); err != nil || odo != 11000 {
		t.Fatalf("current odometer = %v (%v), want 11000 from the readings", odo, err)
	}
	sum, err := tco.ComputeVehicleTCO(ctx, ice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.DistanceBasisKm != 1000 {
		t.Errorf("distance basis = %v, want the 1000 km covered by the readings", sum.DistanceBasisKm)
	}

	// Fill-ups without mileage, framed by the readings: mileage and consumption are estimated.
	fills := []models.FuelLog{
		{Date: at(50), Amount: money.FromFloat(80), Liters: l(50), IsFullTank: true},
		{Date: at(40), Amount: money.FromFloat(32), Liters: l(20), IsFullTank: true},
		{Date: at(30), Amount: money.FromFloat(24), Liters: l(15), IsFullTank: true},
	}
	for i := range fills {
		fills[i].VehicleID = ice.ID
		if err := repo.CreateFuelLog(ctx, &fills[i]); err != nil {
			t.Fatal(err)
		}
		if fills[i].Odometer != nil {
			t.Fatal("a fill-up entered without mileage must stay without mileage")
		}
	}
	logs, err := repo.ListFuelLogs(ctx, ice.ID)
	if err != nil {
		t.Fatal(err)
	}
	readings, _ := repo.ListOdometerCheckpoints(ctx, ice.ID)
	st := ComputeFuelStats(logs, BuildOdometerRefs(readings, nil))
	if st.NoMileageCount != 3 || st.MeasurableCount != 2 || st.EstimatedCount != 2 {
		t.Fatalf("stats: %+v", st)
	}
	if got := st.Logs[1].ConsumptionL100; got == nil || *got != 8 { // 20 L over 250 km
		t.Errorf("second consumption = %v, want 8", got)
	}
	if st.ConsumptionL100 == nil || *st.ConsumptionL100 != 7 { // 35 L over 500 km
		t.Errorf("overall consumption = %v, want 7", st.ConsumptionL100)
	}

	// The TCO still rests on the readings only, and the fill-ups add their cost.
	sum, err = tco.ComputeVehicleTCO(ctx, ice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.DistanceBasisKm != 1000 || sum.EnergyCost != money.FromFloat(136) {
		t.Errorf("basis/energy = %v / %v, want 1000 km / 136 EUR", sum.DistanceBasisKm, sum.EnergyCost)
	}

	// A fill-up with a mileage joins the readings as a cross-checked point.
	pts, err := repo.ListManualOdometerPoints(ctx, ice.ID)
	if err != nil || len(pts) != 2 {
		t.Fatalf("manual points = %d (%v), want the 2 readings only", len(pts), err)
	}
	withKm := models.FuelLog{VehicleID: ice.ID, Date: at(10), Odometer: l(11200), Amount: money.FromFloat(40), IsFullTank: true}
	if err := repo.CreateFuelLog(ctx, &withKm); err != nil {
		t.Fatal(err)
	}
	if pts, _ = repo.ListManualOdometerPoints(ctx, ice.ID); len(pts) != 3 || pts[2].Kind != models.OdometerPointFuel {
		t.Errorf("manual points = %+v", pts)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT current_odometer FROM vehicles WHERE id = $1`, ice.ID).Scan(&odo); err != nil || odo != 11200 {
		t.Errorf("current odometer = %v (%v), want 11200", odo, err)
	}

	// A reading on an electric vehicle never touches its odometer (TeslaMate owns it).
	evReading := models.OdometerCheckpoint{VehicleID: ev.ID, Date: at(5), Odometer: 25000}
	if err := repo.CreateOdometerCheckpoint(ctx, &evReading); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT current_odometer FROM vehicles WHERE id = $1`, ev.ID).Scan(&odo); err != nil || odo != 20000 {
		t.Errorf("EV current odometer = %v (%v), want 20000 untouched", odo, err)
	}
}

func TestIntegrationHighwayPredicateMatchesGoHeuristic(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "highway@example.com")

	// A grid around every threshold of the heuristic: SQL and Go must agree on each drive.
	type sample struct {
		km, avg float64
		max     int
	}
	var samples []sample
	for _, km := range []float64{5, 7.9, 8, 15, 19.9, 20, 39.9, 40, 80} {
		for _, avg := range []float64{40, 69.9, 70, 95} {
			for _, max := range []int{90, 104, 105, 109, 110, 125, 126, 132} {
				samples = append(samples, sample{km, avg, max})
			}
		}
	}
	start := time.Now().UTC().AddDate(-1, 0, 0)
	ids := map[int]*models.Drive{}
	for i, sm := range samples {
		tmID := i + 1
		avg, max := sm.avg, sm.max
		d := &models.Drive{VehicleID: v.ID, TeslaMateDriveID: &tmID, StartTime: start.Add(time.Duration(i) * time.Hour), EndTime: start.Add(time.Duration(i)*time.Hour + 30*time.Minute),
			DistanceKm: sm.km, SpeedAvg: &avg, SpeedMax: &max, Tags: []string{}}
		if _, err := repo.UpsertTeslaMateDrive(ctx, d); err != nil {
			t.Fatal(err)
		}
		ids[tmID] = d
	}

	rows, err := db.Pool.Query(ctx, `SELECT teslamate_drive_id, `+database.HighwayDrivePredicate+` FROM drives WHERE vehicle_id = $1`, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	checked := 0
	for rows.Next() {
		var tmID int
		var sqlSays bool
		if err := rows.Scan(&tmID, &sqlSays); err != nil {
			t.Fatal(err)
		}
		d := ids[tmID]
		if goSays := d.IsHighway(); goSays != sqlSays {
			t.Errorf("drive %.1f km avg %.1f max %d: Go=%v SQL=%v", d.DistanceKm, *d.SpeedAvg, *d.SpeedMax, goSays, sqlSays)
		}
		checked++
	}
	rows.Close()
	if checked != len(samples) {
		t.Fatalf("checked %d drives, want %d", checked, len(samples))
	}

	// A short, slow-looking drive is highway as soon as its GPS detection found a toll segment; an empty detection is not.
	slow := &models.Drive{VehicleID: v.ID, TeslaMateDriveID: intPtr(9001), StartTime: start.Add(1000 * time.Hour), EndTime: start.Add(1001 * time.Hour),
		DistanceKm: 6, SpeedAvg: floatPtr(50), SpeedMax: intPtr(90), Tags: []string{}}
	if _, err := repo.UpsertTeslaMateDrive(ctx, slow); err != nil {
		t.Fatal(err)
	}
	isHighway := func() bool {
		var b bool
		if err := db.Pool.QueryRow(ctx, `SELECT `+database.HighwayDrivePredicate+` FROM drives WHERE id = $1`, slow.ID).Scan(&b); err != nil {
			t.Fatal(err)
		}
		return b
	}
	if isHighway() {
		t.Fatal("a 6 km drive at 90 km/h is not highway before any detection")
	}
	empty := &models.TollDetection{DriveID: slow.ID, VehicleID: v.ID, Segments: nil}
	if err := repo.UpsertTollDetection(ctx, empty); err != nil {
		t.Fatal(err)
	}
	if isHighway() {
		t.Error("a detection without segment (null or empty) must not make the drive highway")
	}
	found := &models.TollDetection{DriveID: slow.ID, VehicleID: v.ID, Segments: []models.TollSegment{{Type: "open", Entry: "Péage A"}}}
	if err := repo.UpsertTollDetection(ctx, found); err != nil {
		t.Fatal(err)
	}
	if !isHighway() {
		t.Error("a drive with a detected toll segment must be highway")
	}
}

func TestIntegrationVehicleGrafanaURL(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "grafana@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	url := "http://192.168.1.50:3000"
	v := &models.Vehicle{UserID: u.ID, Name: "Model 3", TeslaMateAuthType: models.AuthModeNone, TeslaMateGrafanaURL: &url}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetVehicleByID(ctx, v.ID, u.ID)
	if err != nil || got.TeslaMateGrafanaURL == nil || *got.TeslaMateGrafanaURL != url {
		t.Fatalf("GetVehicleByID = %+v (%v)", got, err)
	}
	list, err := repo.ListVehiclesByUserID(ctx, u.ID)
	if err != nil || len(list) != 1 || list[0].TeslaMateGrafanaURL == nil || *list[0].TeslaMateGrafanaURL != url {
		t.Fatalf("ListVehiclesByUserID = %+v (%v)", list, err)
	}
	// Cleared on update
	got.TeslaMateGrafanaURL = nil
	if err := repo.UpdateVehicle(ctx, got); err != nil {
		t.Fatal(err)
	}
	if again, _ := repo.GetVehicleByID(ctx, v.ID, u.ID); again.TeslaMateGrafanaURL != nil {
		t.Errorf("Grafana URL must be cleared, got %q", *again.TeslaMateGrafanaURL)
	}
}

// A vehicle without any TeslaMate record has its hand-typed charges as its only energy data: the energy estimate
// covers the distance before the first charge, not the whole odometer span on top of the charges.
func TestIntegrationEstimatedEnergyStartsWithFirstManualCharge(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	tco := NewTCOService(db.Pool, "Europe/Paris")

	day := time.Now().UTC().Truncate(24 * time.Hour)
	at := func(daysAgo int) time.Time { return day.AddDate(0, 0, -daysAgo) }
	kwh100, price := 15.0, 0.20

	newVehicle := func(email string) *models.Vehicle {
		u, err := repo.CreateUser(ctx, email, "hash")
		if err != nil {
			t.Fatal(err)
		}
		v := &models.Vehicle{UserID: u.ID, Name: "EV", TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 11000,
			EstimatedKwh100km: &kwh100, EstimatedPricePerKwh: &price}
		if err := repo.CreateVehicle(ctx, v); err != nil {
			t.Fatal(err)
		}
		for _, r := range []models.OdometerCheckpoint{
			{VehicleID: v.ID, Date: at(100), Odometer: 10000},
			{VehicleID: v.ID, Date: at(20), Odometer: 11000},
		} {
			if err := repo.CreateOdometerCheckpoint(ctx, &r); err != nil {
				t.Fatal(err)
			}
		}
		return v
	}

	// No charge at all: the whole 1000 km is estimated.
	none := newVehicle("est-none@example.com")
	sum, err := tco.ComputeVehicleTCO(ctx, none.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.EstimatedEnergyDistanceKm < 999 || sum.EstimatedEnergyDistanceKm > 1001 {
		t.Errorf("without charges the estimate covers %v km, want 1000", sum.EstimatedEnergyDistanceKm)
	}

	// First manual charge 60 days ago, halfway through the interval: only the first half is estimated.
	charged := newVehicle("est-charged@example.com")
	cost := money.Cents(1000)
	if err := repo.CreateManualCharge(ctx, &models.ChargeLog{VehicleID: charged.ID, Date: at(60), KwhAdded: 50, Cost: &cost, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	sum, err = tco.ComputeVehicleTCO(ctx, charged.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.EstimatedEnergyDistanceKm < 499 || sum.EstimatedEnergyDistanceKm > 501 {
		t.Errorf("with a charge halfway the estimate covers %v km, want 500", sum.EstimatedEnergyDistanceKm)
	}

	manualDrive := func(v *models.Vehicle, daysAgo int, startOdo, km float64) {
		endOdo := startOdo + km
		d := &models.Drive{VehicleID: v.ID, StartTime: at(daysAgo), EndTime: at(daysAgo).Add(3 * time.Hour),
			StartOdometer: &startOdo, EndOdometer: &endOdo, DistanceKm: km, DurationMin: 180}
		if err := repo.CreateManualDrive(ctx, d); err != nil {
			t.Fatal(err)
		}
	}

	// A drive typed by hand carries no energy cost: it neither starts tracking nor removes its distance from the estimate.
	drove := newVehicle("est-manual-drive@example.com")
	manualDrive(drove, 80, 10100, 200)
	sum, err = tco.ComputeVehicleTCO(ctx, drove.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.EstimatedEnergyDistanceKm < 999 || sum.EstimatedEnergyDistanceKm > 1001 {
		t.Errorf("with a manual drive and no charge the estimate covers %v km, want 1000", sum.EstimatedEnergyDistanceKm)
	}
	if total := sum.TotalDistanceKm + sum.SmoothedDistanceKm; total < 999 || total > 1001 {
		t.Errorf("the manual drive is counted once in the distance: got %v km, want 1000", total)
	}

	// Manual drive before the first manual charge: tracking still starts with the charge, the drive's distance is estimated.
	both := newVehicle("est-manual-drive-charge@example.com")
	manualDrive(both, 80, 10100, 200)
	if err := repo.CreateManualCharge(ctx, &models.ChargeLog{VehicleID: both.ID, Date: at(60), KwhAdded: 50, Cost: &cost, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	sum, err = tco.ComputeVehicleTCO(ctx, both.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.EstimatedEnergyDistanceKm < 499 || sum.EstimatedEnergyDistanceKm > 501 {
		t.Errorf("with a manual drive then a charge halfway the estimate covers %v km, want 500", sum.EstimatedEnergyDistanceKm)
	}
}

func TestIntegrationMultiLegCarpoolBecomesATrip(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "carpool-trip@example.com")
	base := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	d1 := mustDrive(t, repo, v.ID, 1, base, 10000, 120)
	d2 := mustDrive(t, repo, v.ID, 2, base.Add(3*time.Hour), 10120, 80)
	d3 := mustDrive(t, repo, v.ID, 3, base.Add(9*time.Hour), 10200, 30)

	legs := func(ids ...string) []models.CarpoolLeg {
		out := make([]models.CarpoolLeg, len(ids))
		for i := range ids {
			id := ids[i]
			out[i] = models.CarpoolLeg{DriveID: &id, DistanceKm: 50}
		}
		return out
	}

	single := &models.CarpoolTrip{VehicleID: v.ID, Title: "Course seule", Date: base}
	if err := repo.CreateCarpoolTrip(ctx, single, legs(d3.ID), nil); err != nil {
		t.Fatal(err)
	}
	if single.TripGroupID != nil {
		t.Fatalf("a single-drive carpool is not a trip, got group %v", *single.TripGroupID)
	}

	multi := &models.CarpoolTrip{VehicleID: v.ID, Title: "Annecy → Lyon → Valence", Date: base}
	if err := repo.CreateCarpoolTrip(ctx, multi, legs(d1.ID, d2.ID), nil); err != nil {
		t.Fatal(err)
	}
	if multi.TripGroupID == nil {
		t.Fatal("a carpool made of several drives must be attached to a trip group")
	}
	groups, err := repo.ListTripGroups(ctx, v.ID)
	if err != nil || len(groups) != 1 {
		t.Fatalf("expected exactly one trip group, got %+v (err %v)", groups, err)
	}
	if g := groups[0]; g.ID != *multi.TripGroupID || g.Name != multi.Title || len(g.DriveIDs) != 2 || g.CarpoolCount != 1 || g.DistanceKm != 200 {
		t.Fatalf("unexpected trip group %+v", g)
	}

	// Saving it again keeps the same group instead of creating another one.
	if err := repo.UpdateCarpoolTrip(ctx, multi, legs(d1.ID, d2.ID), nil); err != nil {
		t.Fatal(err)
	}
	if groups, _ = repo.ListTripGroups(ctx, v.ID); len(groups) != 1 {
		t.Fatalf("updating the carpool must not create a second group, got %d", len(groups))
	}
}

func TestIntegrationListDrivesByID(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "drive-by-id@example.com")
	base := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	d1 := mustDrive(t, repo, v.ID, 1, base, 10000, 120)
	mustDrive(t, repo, v.ID, 2, base.Add(3*time.Hour), 10120, 80)

	got, total, err := repo.ListDrives(ctx, v.ID, database.DriveFilter{DriveID: d1.ID}, 10, 0)
	if err != nil || total != 1 || len(got) != 1 || got[0].ID != d1.ID {
		t.Fatalf("expected only the requested drive, got %+v (total %d, err %v)", got, total, err)
	}
}

func TestIntegrationDismissedTripSuggestionIsNotProposedAgain(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "dismiss-suggestion@example.com")
	other := mustVehicle(t, repo, "dismiss-suggestion-other@example.com")
	base := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	// mustDrive lasts one hour: the second drive starts 3 minutes after the first one ends.
	a := mustDrive(t, repo, v.ID, 1, base, 10000, 30)
	b := mustDrive(t, repo, v.ID, 2, base.Add(63*time.Minute), 10030, 30)
	since := base.Add(-time.Hour)

	suggestions := func() int {
		drives, err := repo.ListTripCandidateDrives(ctx, v.ID, since)
		if err != nil {
			t.Fatal(err)
		}
		return len(DetectTripSuggestions(drives, nil))
	}
	if n := suggestions(); n != 1 {
		t.Fatalf("two drives 3 minutes apart are suggested as one trip, got %d suggestions", n)
	}

	if err := repo.DismissTripSuggestion(ctx, other.ID, []string{a.ID, b.ID}); !errors.Is(err, database.ErrForeignReference) {
		t.Fatalf("another vehicle cannot dismiss these drives, got %v", err)
	}
	if n := suggestions(); n != 1 {
		t.Fatalf("a rejected dismissal must change nothing, got %d", n)
	}

	if err := repo.DismissTripSuggestion(ctx, v.ID, []string{a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	if n := suggestions(); n != 0 {
		t.Fatalf("a dismissed suggestion is not proposed again, got %d", n)
	}
}

func TestIntegrationTCOCountsOdometerReadingsAndAsksForTheStartOdometer(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "readings@example.com")
	tco := NewTCOService(db.Pool, "Europe/Paris")

	day := func(daysAgo int) time.Time {
		n := time.Now().UTC().AddDate(0, 0, -daysAgo)
		return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	}
	if err := repo.RecordIntegrationOdometer(ctx, v.ID, day(20), 37000); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordIntegrationOdometer(ctx, v.ID, day(1), 37500); err != nil {
		t.Fatal(err)
	}

	sum, err := tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.DistanceBasisKm != 500 {
		t.Fatalf("expected the span of the readings as distance basis, got %.0f", sum.DistanceBasisKm)
	}
	if !sum.Completeness.StartOdometerMissing {
		t.Fatal("expected the missing start odometer to be reported")
	}

	odo := 0.0
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{
		VehicleID: v.ID, AcquisitionType: models.AcquisitionCash, StartDate: day(400), StartOdometer: &odo,
	}); err != nil {
		t.Fatal(err)
	}
	sum, err = tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Completeness.StartOdometerMissing {
		t.Fatal("expected no start odometer warning once it is entered")
	}
	if sum.DistanceBasisKm != 37500 {
		t.Fatalf("expected the distance from the start odometer to the last reading, got %.0f", sum.DistanceBasisKm)
	}
}

func TestIntegrationHybridVehicleCombinesEnergies(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "phev@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	newHybrid := func(name string) *models.Vehicle {
		v := &models.Vehicle{UserID: u.ID, Name: name, Powertrain: models.PowertrainPHEV, TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 10000}
		if err := repo.CreateVehicle(ctx, v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	tco := NewTCOService(db.Pool, "Europe/Paris")
	now := time.Now().UTC()
	l := func(v float64) *float64 { return &v }
	addFuel := func(v *models.Vehicle) {
		for i, odo := range []float64{10000, 10600} {
			f := &models.FuelLog{VehicleID: v.ID, Date: now.AddDate(0, -2+i, 0), Odometer: l(odo), Amount: money.FromFloat(60), Liters: l(35), IsFullTank: true}
			if err := repo.CreateFuelLog(ctx, f); err != nil {
				t.Fatal(err)
			}
		}
	}
	addCharge := func(v *models.Vehicle) {
		cost := money.FromFloat(10)
		c := &models.ChargeLog{VehicleID: v.ID, Date: now.AddDate(0, -1, 0), KwhAdded: 50, Cost: &cost, Currency: "EUR"}
		if err := repo.CreateManualCharge(ctx, c); err != nil {
			t.Fatal(err)
		}
	}

	both := newHybrid("both")
	addFuel(both)
	addCharge(both)
	sum, err := tco.ComputeVehicleTCO(ctx, both.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.EnergyCost != money.FromFloat(130) {
		t.Errorf("energy cost = %v, want 120 of fuel + 10 of electricity", sum.EnergyCost)
	}
	if sum.FuelFillUps != 2 || sum.TotalLiters != 70 || sum.TotalKwhAdded != 50 {
		t.Errorf("volumes: fills=%d liters=%v kwh=%v", sum.FuelFillUps, sum.TotalLiters, sum.TotalKwhAdded)
	}
	if sum.EstimatedEnergyDistanceKm != 0 || sum.EnergyCostPerKm <= 0 {
		t.Errorf("estimated distance=%v cost per km=%v", sum.EstimatedEnergyDistanceKm, sum.EnergyCostPerKm)
	}
	for _, w := range sum.Completeness.Warnings {
		if strings.Contains(w, "fill-up recorded") || strings.Contains(w, "without a cost") {
			t.Errorf("unexpected warning on a hybrid with both energies: %q", w)
		}
	}

	fuelOnly := newHybrid("fuel only")
	addFuel(fuelOnly)
	if sum, err = tco.ComputeVehicleTCO(ctx, fuelOnly.ID); err != nil || sum.EnergyCost != money.FromFloat(120) || sum.TotalKwhAdded != 0 {
		t.Errorf("fuel-only hybrid: %+v (%v)", sum, err)
	}
	for _, w := range sum.Completeness.Warnings {
		if strings.Contains(w, "fill-up recorded") {
			t.Errorf("a hybrid with fill-ups must not warn about missing fuel: %q", w)
		}
	}

	chargeOnly := newHybrid("charges only")
	addCharge(chargeOnly)
	if sum, err = tco.ComputeVehicleTCO(ctx, chargeOnly.ID); err != nil || sum.EnergyCost != money.FromFloat(10) || sum.FuelFillUps != 0 {
		t.Errorf("charge-only hybrid: %+v (%v)", sum, err)
	}
	for _, w := range sum.Completeness.Warnings {
		if strings.Contains(w, "No fill-up recorded") {
			t.Errorf("a hybrid that charges must not warn about missing fuel: %q", w)
		}
	}

	rangeExtender := newHybrid("range extender")
	rangeExtender.Powertrain = models.PowertrainREEV
	if err := repo.UpdateVehicle(ctx, rangeExtender); err != nil {
		t.Fatal(err)
	}
	addFuel(rangeExtender)
	addCharge(rangeExtender)

	svc := NewComparisonService(tco)
	for _, v := range []*models.Vehicle{both, fuelOnly, chargeOnly, rangeExtender} {
		snapshot, err := tco.ComputeVehicleTCO(ctx, v.ID)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.DistanceBasisKm == 0 {
			checkpoint := &models.OdometerCheckpoint{VehicleID: v.ID, Date: now.AddDate(0, -2, 0), Odometer: 9000}
			if err := repo.CreateOdometerCheckpoint(ctx, checkpoint); err != nil {
				t.Fatal(err)
			}
			snapshot, err = tco.ComputeVehicleTCO(ctx, v.ID)
			if err != nil {
				t.Fatal(err)
			}
		}
		scenario := &models.ComparisonScenario{Mode: models.ComparisonModeRetrospective, VehicleID: &v.ID, AnnualKm: snapshot.DistanceBasisKm, Years: 2, ICE: models.ICEInputs{LPer100Km: 8, FuelPrice: 2}, Options: models.ScenarioOptions{FuelInflationPct: 10, ElectricityInflationPct: 20}}
		result, err := svc.Compare(ctx, scenario)
		if err != nil {
			t.Fatalf("compare %s: %v", v.Name, err)
		}
		fuel := snapshot.FuelEnergyCost.Float()
		electricity := (snapshot.EnergyCost - snapshot.FuelEnergyCost).Float()
		want := money.FromFloat(fuel*2.1 + electricity*2.2)
		if result.Tracked.Energy != want {
			t.Errorf("%s energy = %v, want %v", v.Name, result.Tracked.Energy, want)
		}
	}
	d, err := svc.Defaults(ctx, both.ID)
	if err != nil || d.TrackedKwhPer100Km != nil || d.ICELPer100Km != nil {
		t.Errorf("hybrid defaults must not mix energies: %+v (%v)", d, err)
	}
	// Both references start from the prices the hybrid paid: 120 over 70 L, 10 over 50 kWh.
	if d.ICEFuelPrice == nil || math.Abs(*d.ICEFuelPrice-120.0/70) > 0.001 || d.TrackedEurPerKwh == nil || *d.TrackedEurPerKwh != 0.2 {
		t.Errorf("hybrid price defaults: fuel=%v electricity=%v", d.ICEFuelPrice, d.TrackedEurPerKwh)
	}
	if d, err = svc.Defaults(ctx, fuelOnly.ID); err != nil || d.ICEFuelPrice == nil || d.TrackedEurPerKwh != nil {
		t.Errorf("fuel-only hybrid must prefill the fuel price only: %+v (%v)", d, err)
	}
	if d, err = svc.Defaults(ctx, chargeOnly.ID); err != nil || d.ICEFuelPrice != nil || d.TrackedEurPerKwh == nil || *d.TrackedEurPerKwh != 0.2 {
		t.Errorf("charge-only hybrid must prefill the electricity price only: %+v (%v)", d, err)
	}
}

func TestIntegrationOdometerEstimateFromOwnershipOrigin(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "origin@example.com")
	v.CurrentOdometer = 30000
	if err := repo.UpdateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	rent, months, startOdo := money.Cents(20000), 36, 0.0
	start := time.Now().UTC().AddDate(-2, 0, 0)
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{VehicleID: v.ID, AcquisitionType: models.AcquisitionLLD,
		StartDate: start, StartOdometer: &startOdo, LeaseMonthlyRent: &rent, LeaseDurationMonths: &months}); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.ListOdometerAnchors(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	anchors := make([]OdometerAnchor, len(stored))
	for i, a := range stored {
		anchors[i] = OdometerAnchor{Date: a.Date, Km: a.Km, Origin: a.Origin}
	}
	km, source, ok := EstimateOdometerAt(anchors, start.AddDate(1, 0, 0))
	if !ok || source != OdometerEstimateInterpolated || km < 14000 || km > 16000 {
		t.Fatalf("expected about 15,000 km one year into a two-year ownership at 30,000 km, got %v %q %v", km, source, ok)
	}
}

func TestIntegrationUnmountedTireWithoutSessionCounts(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	tco := NewTCOService(db.Pool, "UTC")
	v := mustVehicle(t, repo, "winter@example.com")
	v.CurrentOdometer = 20000
	if err := repo.UpdateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	tire := &models.Tire{VehicleID: &v.ID, Brand: "Nokian", Model: "WR", Dimension: "205/55 R16", Season: models.TireSeasonWinter,
		PurchaseDate: time.Now().UTC().AddDate(0, -2, 0), PurchasePrice: 40000, CurrentPosition: models.TirePosStorage,
		InitialDepthMm: 8, MinLegalDepthMm: 1.6, EstimatedLifespanKm: 40000}
	if err := repo.CreateTire(ctx, tire); err != nil {
		t.Fatal(err)
	}
	sum, err := tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	var booked money.Cents
	for _, m := range sum.MonthlyCosts {
		booked += m.TiresAmortized
	}
	if sum.TiresAmortizedCost != 40000 || booked != 40000 {
		t.Fatalf("expected the paid, never mounted tire counted in full, got total %s and monthly %s", sum.TiresAmortizedCost, booked)
	}
}

func TestIntegrationHistoricalSessionWithoutWheelIsAmortizedMonthly(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	tco := NewTCOService(db.Pool, "UTC")
	v := mustVehicle(t, repo, "historical@example.com")
	v.CurrentOdometer = 20000
	if err := repo.UpdateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	tire := &models.Tire{VehicleID: &v.ID, Brand: "B", Model: "Winter", Dimension: "205/55 R16", Season: models.TireSeasonWinter,
		PurchaseDate: time.Now().UTC().AddDate(-1, 0, 0), PurchasePrice: 40000, CurrentPosition: models.TirePosStorage,
		InitialDepthMm: 8, MinLegalDepthMm: 1.6, EstimatedLifespanKm: 40000}
	if err := repo.CreateTire(ctx, tire); err != nil {
		t.Fatal(err)
	}
	mounted := time.Now().UTC().AddDate(0, -6, 0)
	dismounted := time.Now().UTC().AddDate(0, -3, 0)
	end := 10000.0
	if err := repo.CreateTireMountSession(ctx, &models.TireMountSession{TireID: tire.ID, VehicleID: v.ID, Position: models.TirePosStorage,
		MountedDate: mounted, MountedOdometer: 4000, DismountedDate: &dismounted, DismountedOdometer: &end, DistanceKm: 6000}); err != nil {
		t.Fatal(err)
	}
	sum, err := tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	var booked money.Cents
	for _, m := range sum.MonthlyCosts {
		booked += m.TiresAmortized
	}
	if booked != 6000 {
		t.Fatalf("expected the 6,000 km driven on a wheel-less historical session amortized monthly, got %s", booked)
	}
}

func TestIntegrationOpenTireSessionIsReplaced(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "opensession@example.com")
	odo := 100.0
	newTire := func(model string, pos models.TirePosition) *models.Tire {
		tire := &models.Tire{MountedOdometer: &odo, VehicleID: &v.ID, Brand: "B", Model: model, Dimension: "205/55 R16", Season: models.TireSeasonSummer,
			PurchaseDate: time.Now().UTC().AddDate(-1, 0, 0), PurchasePrice: 10000, CurrentPosition: pos,
			InitialDepthMm: 8, MinLegalDepthMm: 1.6, EstimatedLifespanKm: 40000}
		if err := repo.CreateTire(ctx, tire); err != nil {
			t.Fatal(err)
		}
		return tire
	}
	openCount := func(tireID string) int {
		list, err := repo.ListTireMountSessions(ctx, tireID)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, s := range list {
			if s.DismountedDate == nil {
				n++
			}
		}
		return n
	}
	a, b := newTire("A", models.TirePosFL), newTire("B", models.TirePosFR)
	day := time.Now().UTC().AddDate(0, -6, 0)
	for _, pos := range []models.TirePosition{models.TirePosFL, models.TirePosFR} {
		if err := repo.CreateTireMountSession(ctx, &models.TireMountSession{TireID: a.ID, VehicleID: v.ID, Position: pos, MountedDate: day, MountedOdometer: 100}); err != nil {
			t.Fatal(err)
		}
	}
	if n := openCount(a.ID); n != 1 {
		t.Fatalf("a new open session must replace the previous one, got %d open", n)
	}
	if err := repo.CreateTireMountSession(ctx, &models.TireMountSession{TireID: b.ID, VehicleID: v.ID, Position: models.TirePosFR, MountedDate: day.AddDate(0, -3, 0), MountedOdometer: 50}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CopyTireHistory(ctx, v.ID, a.ID, []string{b.ID}, true, false, false); err != nil {
		t.Fatal(err)
	}
	if n := openCount(b.ID); n != 1 {
		t.Fatalf("a copied open session must replace the target's open session, got %d open", n)
	}
	list, err := repo.ListTireMountSessions(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.DismountedDate == nil && s.Position != models.TirePosFR {
			t.Fatalf("the copied open session must take the position where the target is fitted, got %s", s.Position)
		}
	}
}

func TestIntegrationCurrentMonthCountsFinancingDueLater(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	now := time.Now().UTC()
	tomorrow := now.AddDate(0, 0, 1)
	if tomorrow.Month() != now.Month() || tomorrow.Day() > 28 {
		t.Skip("needs a day later in the current month and a start day that exists in every month")
	}
	car := mustVehicle(t, repo, "due-later@example.com")
	rent, months := money.Cents(45000), 36
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{
		VehicleID: car.ID, AcquisitionType: models.AcquisitionLOA, StartDate: tomorrow.AddDate(0, -2, 0),
		LeaseMonthlyRent: &rent, LeaseDurationMonths: &months,
	}); err != nil {
		t.Fatal(err)
	}
	sum, err := NewTCOService(db.Pool, "UTC").ComputeVehicleTCO(ctx, car.ID)
	if err != nil {
		t.Fatal(err)
	}
	current := now.Format("2006-01")
	for _, m := range sum.MonthlyCosts {
		if m.Month == current {
			if m.Financing != rent {
				t.Fatalf("expected the rent due tomorrow in the current month, got %s", m.Financing)
			}
			if sum.FinancingCost != 2*rent {
				t.Fatalf("cumulative cash must keep only the payments already due, got %s", sum.FinancingCost)
			}
			return
		}
	}
	t.Fatalf("current month %s missing from %+v", current, sum.MonthlyCosts)
}
