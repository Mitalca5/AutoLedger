package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

type fakeExportStore struct {
	charges     []models.ChargeLog
	fuel        []models.FuelLog
	expenses    []models.DriveExpense
	drivesCalls []database.DriveFilter
}

func (f *fakeExportStore) ListCharges(context.Context, string, bool, int, int) ([]models.ChargeLog, int, error) {
	return f.charges, len(f.charges), nil
}

func (f *fakeExportStore) ListDrives(_ context.Context, _ string, filter database.DriveFilter, _, _ int) ([]models.Drive, int, error) {
	f.drivesCalls = append(f.drivesCalls, filter)
	return nil, 0, nil
}

func (f *fakeExportStore) ListFuelLogs(context.Context, string) ([]models.FuelLog, error) {
	return f.fuel, nil
}

func (f *fakeExportStore) ListOdometerCheckpoints(context.Context, string) ([]models.OdometerCheckpoint, error) {
	return nil, nil
}

func (f *fakeExportStore) ListDriveExpenses(context.Context, string, string) ([]models.DriveExpense, error) {
	return f.expenses, nil
}

func (f *fakeExportStore) ListMaintenanceExpenses(context.Context, string) ([]models.MaintenanceExpense, error) {
	return nil, nil
}

func exportLines(t *testing.T, res *ExportResult) []string {
	t.Helper()
	body := strings.TrimPrefix(string(res.Body), "\xEF\xBB\xBF")
	return strings.Split(strings.TrimSpace(body), "\n")
}

func TestExportRejectsUnknownTypeAndFormat(t *testing.T) {
	svc := NewExportService(&fakeExportStore{})
	for _, opts := range []ExportOptions{{Type: "nope", Format: ExportCSV}, {Type: ExportFuel, Format: "xml"}, {Type: ExportMileage, Format: ExportCSV}} {
		_, err := svc.Export(context.Background(), &models.Vehicle{ID: "v"}, opts)
		if e, ok := err.(*apierror.Error); !ok || !strings.HasPrefix(e.Code, "export.invalid_") {
			t.Fatalf("expected an export.invalid_* error for %+v, got %v", opts, err)
		}
	}
}

func TestExportFuelSortsFiltersAndConvertsTheDistance(t *testing.T) {
	liters, odo := 40.0, 16093.44
	note := "=SUM(A1)"
	day := func(d int) time.Time { return time.Date(2026, 5, d, 8, 0, 0, 0, time.UTC) }
	store := &fakeExportStore{fuel: []models.FuelLog{
		{Date: day(20), Amount: 7200, Liters: &liters, Odometer: &odo, IsFullTank: true, Notes: &note},
		{Date: day(2), Amount: 5000},
		{Date: day(10), Amount: 6000},
	}}
	from, to := day(5), day(25)
	res, err := NewExportService(store).Export(context.Background(), &models.Vehicle{ID: "v"}, ExportOptions{Type: ExportFuel, Format: ExportCSV, From: &from, To: &to, Unit: "mi"})
	if err != nil {
		t.Fatal(err)
	}
	lines := exportLines(t, res)
	if len(lines) != 3 || lines[0] != "date,liters,price_per_liter,amount,fuel_type,is_full_tank,odometer_mi,notes" {
		t.Fatalf("unexpected header or row count: %q", lines)
	}
	if !strings.HasPrefix(lines[1], "2026-05-10T08:00:00Z,") {
		t.Fatalf("rows must be oldest first and the period applied: %q", lines)
	}
	if !strings.Contains(lines[2], ",40,,72") || !strings.Contains(lines[2], ",true,10000,'=SUM(A1)") {
		t.Fatalf("expected miles and a neutralised formula, got %q", lines[2])
	}
	if res.Extension != "csv" || !strings.HasPrefix(res.ContentType, "text/csv") {
		t.Fatalf("unexpected result %+v", res)
	}
}

func TestExportChargesAreOldestFirstAndJSONUsesNullForBlanks(t *testing.T) {
	cost := money.Cents(980)
	store := &fakeExportStore{charges: []models.ChargeLog{
		{Date: time.Date(2026, 5, 2, 8, 0, 0, 0, time.UTC), KwhAdded: 10, Currency: "EUR"},
		{Date: time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC), KwhAdded: 37.5, Cost: &cost, Currency: "EUR"},
	}}
	res, err := NewExportService(store).Export(context.Background(), &models.Vehicle{ID: "v"}, ExportOptions{Type: ExportCharges, Format: ExportJSON})
	if err != nil {
		t.Fatal(err)
	}
	body := string(res.Body)
	first := strings.Index(body, `"kwh":37.5`)
	second := strings.Index(body, `"kwh":10`)
	if first < 0 || second < first || !strings.Contains(body, `"cost":null`) || !strings.Contains(body, `"odometer_km":null`) {
		t.Fatalf("unexpected JSON %s", body)
	}
	if res.ContentType != "application/json" {
		t.Fatalf("unexpected content type %q", res.ContentType)
	}
}

func TestExportExpensesNameTheTripAndPassTheFiltersToDrives(t *testing.T) {
	group, title := "Alps", "Home to work"
	store := &fakeExportStore{expenses: []models.DriveExpense{
		{Date: time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC), Type: "TOLL", Amount: 450, Currency: "EUR", Source: "MANUAL", TripGroupName: &group, DriveTitle: &title},
		{Date: time.Date(2026, 5, 2, 8, 0, 0, 0, time.UTC), Type: "PARKING", Amount: 300, Currency: "EUR", Source: "MANUAL", DriveTitle: &title},
	}}
	svc := NewExportService(store)
	res, err := svc.Export(context.Background(), &models.Vehicle{ID: "v"}, ExportOptions{Type: ExportExpenses, Format: ExportCSV})
	if err != nil {
		t.Fatal(err)
	}
	lines := exportLines(t, res)
	if !strings.Contains(lines[1], ",Alps,") || !strings.Contains(lines[2], ",Home to work,") {
		t.Fatalf("a trip group name wins over the drive title: %q", lines)
	}
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.Export(context.Background(), &models.Vehicle{ID: "v"}, ExportOptions{Type: ExportDrives, Format: ExportCSV, Tag: "work", From: &from}); err != nil {
		t.Fatal(err)
	}
	if len(store.drivesCalls) != 1 || store.drivesCalls[0].Tag != "work" || store.drivesCalls[0].From == nil {
		t.Fatalf("the tag and period must reach the repository, got %+v", store.drivesCalls)
	}
}
