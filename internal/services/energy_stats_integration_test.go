package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/migrations"
)

// The queries feeding the statistics: months in the reporting timezone, EUR conversion of the cost, exclusion of
// what TeslaMate deleted, and the grid/stored energy reading.
func TestIntegrationEnergyStats(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "energy@example.com")
	svc := NewEnergyStatsService(db.Pool, "Europe/Paris")

	kwh := func(v float64) *float64 { return &v }
	drive := func(tmID int, start time.Time, km float64, energy *float64) *models.Drive {
		d := mustDrive(t, repo, v.ID, tmID, start, 10000+float64(tmID)*200, km)
		d.EnergyConsumedKwh = energy
		if _, err := repo.UpsertTeslaMateDrive(ctx, d); err != nil {
			t.Fatal(err)
		}
		return d
	}

	// 23:30 UTC on 31 January is 00:30 on 1 February in Paris: the drive belongs to February.
	drive(1, time.Date(2026, 1, 31, 23, 30, 0, 0, time.UTC), 100, kwh(15))
	drive(2, time.Date(2026, 2, 10, 9, 0, 0, 0, time.UTC), 200, nil) // no energy reading: distance only
	deleted := drive(3, time.Date(2026, 2, 12, 9, 0, 0, 0, time.UTC), 300, kwh(60))
	// A manual drive whose energy was derived from the average consumption: its distance counts, not its energy.
	estimated := &models.Drive{VehicleID: v.ID, StartTime: time.Date(2026, 2, 14, 9, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 2, 14, 10, 0, 0, 0, time.UTC),
		DistanceKm: 50, DurationMin: 60, EnergyConsumedKwh: kwh(40), ConsumptionKwh100km: kwh(80), EnergyEstimated: true}
	if err := repo.CreateManualDrive(ctx, estimated); err != nil {
		t.Fatal(err)
	}

	charge := func(tmID int, start time.Time, hours float64, added float64, used *float64, cost *money.Cents, currency string, fx *float64) {
		end := start.Add(time.Duration(hours * float64(time.Hour)))
		c := &models.ChargeLog{VehicleID: v.ID, TeslaMateChargeID: &tmID, Date: start, EndDate: &end, KwhAdded: added, KwhUsed: used, Cost: cost, Currency: currency, FxRate: fx}
		if _, err := repo.UpsertTeslaMateCharge(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	eur := func(v float64) *money.Cents { c := money.FromFloat(v); return &c }

	charge(1, time.Date(2026, 2, 3, 18, 0, 0, 0, time.UTC), 4, 30, kwh(33), eur(6), "EUR", nil)  // AC
	charge(2, time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC), 0.5, 40, nil, eur(20), "EUR", nil)   // DC
	charge(3, time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC), 5, 10, kwh(12), nil, "EUR", nil)     // no tariff
	charge(4, time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC), 5, 10, kwh(12), eur(10), "GBP", nil) // foreign cost without rate: unknown
	// Only manual entries carry a foreign currency with its rate.
	fx := 1.2
	manualEnd := time.Date(2026, 2, 8, 17, 0, 0, 0, time.UTC)
	manual := &models.ChargeLog{VehicleID: v.ID, Date: time.Date(2026, 2, 8, 12, 0, 0, 0, time.UTC), EndDate: &manualEnd, KwhAdded: 10, Cost: eur(5), Currency: "GBP", FxRate: &fx} // 5 GBP = 6 EUR
	if err := repo.CreateManualCharge(ctx, manual); err != nil {
		t.Fatal(err)
	}

	// A charge deleted in TeslaMate no longer counts.
	charge(6, time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC), 3, 99, kwh(100), eur(50), "EUR", nil)
	if res, err := repo.ReconcileTeslaMateRecords(ctx, "charges", v.ID, ptrTime(time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC)), nil); err != nil || res.Marked != 1 {
		t.Fatalf("expected the charge to be flagged deleted, got %+v (err %v)", res, err)
	}
	// ... and neither does a deleted drive.
	if res, err := repo.ReconcileTeslaMateRecords(ctx, "drives", v.ID, ptrTime(deleted.StartTime.Add(-time.Hour)), nil); err != nil || res.Marked != 1 {
		t.Fatalf("expected the drive to be flagged deleted, got %+v (err %v)", res, err)
	}

	got, err := svc.Compute(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Months) != 1 || got.Months[0].Month != "2026-02" {
		t.Fatalf("expected the 23:30 UTC drive in the Paris month of February, got %+v", got.Months)
	}
	m := got.Months[0]
	if m.DistanceKm != 350 {
		t.Errorf("distance: got %v, want 350 (deleted drive excluded, estimated manual drive included)", m.DistanceKm)
	}
	eq(t, "consumption", m.ConsumptionKwh100km, 15.0) // 15 kWh over the 100 km with a reading; the estimated drive stays out
	if m.EnergyCost != money.FromFloat(32) {          // 6 + 20 + 6 (GBP converted); no tariff and no rate stay out
		t.Errorf("energy cost: got %v, want 32.00", m.EnergyCost)
	}
	if m.KwhAdded != 100 { // 30 + 40 + 10 + 10 + 10, deleted session excluded
		t.Errorf("kWh added: got %v, want 100", m.KwhAdded)
	}
	if got.Summary.SessionsWithoutCost != 2 {
		t.Errorf("sessions without cost: got %d, want 2 (no tariff, GBP without rate)", got.Summary.SessionsWithoutCost)
	}
	// 50 kWh stored over 57 drawn: neither the DC session nor the manual entry has a grid reading
	eq(t, "efficiency", m.ChargeEfficiency, 0.877)
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestIntegrationBatteryTemperatureAndBackfill(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "battery@example.com")
	svc := NewEnergyStatsService(db.Pool, "Europe/Paris")

	// Drives with their temperature: two cold groups, one mild group.
	temp := func(c float64) *float64 { return &c }
	energy := func(k float64) *float64 { return &k }
	addDrive := func(tmID int, day int, km, kwh, celsius float64) {
		d := mustDrive(t, repo, v.ID, tmID, time.Date(2026, 1, day, 9, 0, 0, 0, time.UTC), 1000+float64(tmID)*100, km)
		d.EnergyConsumedKwh = energy(kwh)
		d.OutsideTempC = temp(celsius)
		if _, err := repo.UpsertTeslaMateDrive(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	addDrive(1, 3, 60, 12, -2.5)  // -5..0 bin
	addDrive(2, 4, 80, 15.2, 1.0) // 0..5 bin
	addDrive(3, 5, 200, 30, 17.0) // 15..20 bin
	addDrive(4, 6, 3, 2, 17.0)    // under 5 km: ignored
	// A drive without temperature or without energy does not count either.
	noTemp := mustDrive(t, repo, v.ID, 5, time.Date(2026, 1, 7, 9, 0, 0, 0, time.UTC), 2000, 50)
	noTemp.EnergyConsumedKwh = energy(9)
	if _, err := repo.UpsertTeslaMateDrive(ctx, noTemp); err != nil {
		t.Fatal(err)
	}

	// A charge keeps its state of charge and temperature through the upsert, and a later sync updates them.
	tmID := 1
	start := time.Date(2026, 1, 10, 18, 0, 0, 0, time.UTC)
	end := start.Add(4 * time.Hour)
	charge := &models.ChargeLog{VehicleID: v.ID, TeslaMateChargeID: &tmID, Date: start, EndDate: &end, KwhAdded: 40, Currency: "EUR"}
	if _, err := repo.UpsertTeslaMateCharge(ctx, charge); err != nil {
		t.Fatal(err)
	}
	s1, e1 := 20, 70
	charge.StartBatteryLevel, charge.EndBatteryLevel = &s1, &e1
	if _, err := repo.UpsertTeslaMateCharge(ctx, charge); err != nil {
		t.Fatal(err)
	}

	// Battery health: one snapshot a day, the latest reading of the day wins.
	day := time.Date(2026, 1, 10, 6, 0, 0, 0, time.UTC)
	if err := repo.UpsertBatterySnapshot(ctx, v.ID, day, models.BatterySnapshot{CurrentCapacityKwh: energy(75.0), MaxCapacityKwh: energy(78.4), HealthPercent: energy(95.66)}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertBatterySnapshot(ctx, v.ID, day.Add(10*time.Hour), models.BatterySnapshot{CurrentCapacityKwh: energy(75.2), MaxCapacityKwh: energy(78.4), HealthPercent: energy(95.9)}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertBatterySnapshot(ctx, v.ID, day.AddDate(0, 0, 30), models.BatterySnapshot{CurrentCapacityKwh: energy(74.9)}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.Compute(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.TemperatureBins) != 3 {
		t.Fatalf("temperature bins: got %+v, want 3 (short, temperature-less drives excluded)", got.TemperatureBins)
	}
	if b := got.TemperatureBins[0]; b.MinC != -5 || b.MaxC != 0 || b.Drives != 1 || b.ConsumptionKwh100km != 20 {
		t.Errorf("-5..0 bin: %+v", b)
	}
	if b := got.TemperatureBins[1]; b.MinC != 0 || b.ConsumptionKwh100km != 19 {
		t.Errorf("0..5 bin: %+v", b)
	}
	eq(t, "cold consumption", got.Temperature.ColdConsumptionKwh100km, 19.4) // 27.2 kWh over 140 km
	eq(t, "mild consumption", got.Temperature.MildConsumptionKwh100km, 15)

	eq(t, "capacity", got.Summary.EstimatedCapacityKwh, 80) // 40 kWh over 50 points
	if len(got.BatteryHealth) != 2 || got.BatteryHealth[0].Date != "2026-01-10" {
		t.Fatalf("battery health: %+v", got.BatteryHealth)
	}
	eq(t, "latest reading of the day wins", got.BatteryHealth[0].CurrentCapacityKwh, 75.2)
	if got.BatteryHealth[1].HealthPercent != nil {
		t.Errorf("a snapshot without health keeps it unknown, got %v", *got.BatteryHealth[1].HealthPercent)
	}

	// The migration makes the next synchronization re-read the whole history.
	if err := repo.MarkSyncSuccess(ctx, v.ID, "drives", true); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkSyncSuccess(ctx, v.ID, "charges", true); err != nil {
		t.Fatal(err)
	}
	if done, _ := repo.IsFullImportCompleted(ctx, v.ID, "drives"); !done {
		t.Fatal("precondition: the full import should be recorded")
	}
	up, err := migrations.FS.ReadFile("000026_battery_and_temperature.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, string(up)); err != nil { // idempotent: columns and table already exist
		t.Fatalf("migration 26 must be re-runnable: %v", err)
	}
	for _, resource := range []string{"drives", "charges"} {
		if done, _ := repo.IsFullImportCompleted(ctx, v.ID, resource); done {
			t.Errorf("%s: the full import flag must be reset so the history is re-read", resource)
		}
	}
	// Existing rows and costs entered by hand survive the reset.
	if list, total, _ := repo.ListCharges(ctx, v.ID, false, 10, 0); total != 1 || len(list) != 1 {
		t.Errorf("charges must be untouched by the migration, got %d", total)
	}
}
