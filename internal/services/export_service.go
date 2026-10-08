package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
)

type ExportType string

const (
	ExportCharges     ExportType = "charges"
	ExportDrives      ExportType = "drives"
	ExportFuel        ExportType = "fuel"
	ExportOdometer    ExportType = "odometer"
	ExportExpenses    ExportType = "expenses"
	ExportMaintenance ExportType = "maintenance"
	ExportMileage     ExportType = "mileage"
)

type ExportFormat string

const (
	ExportCSV  ExportFormat = "csv"
	ExportJSON ExportFormat = "json"
)

// ExportOptions selects what to export. Unit is the account's distance unit ("km" or "mi").
type ExportOptions struct {
	Type   ExportType
	Format ExportFormat
	From   *time.Time
	To     *time.Time
	Tag    string
	Unit   string
	Lang   string
	// UserID and RateLabel feed the mileage export: whose scale to apply (none when the label is empty).
	UserID    string
	RateLabel string
}

// exportStore is the slice of *database.Repository that ExportService reads.
type exportStore interface {
	ListCharges(ctx context.Context, vehicleID string, missingCostOnly bool, limit, offset int) ([]models.ChargeLog, int, error)
	ListDrives(ctx context.Context, vehicleID string, filter database.DriveFilter, limit, offset int) ([]models.Drive, int, error)
	ListFuelLogs(ctx context.Context, vehicleID string) ([]models.FuelLog, error)
	ListOdometerCheckpoints(ctx context.Context, vehicleID string) ([]models.OdometerCheckpoint, error)
	ListDriveExpenses(ctx context.Context, vehicleID, lang string) ([]models.DriveExpense, error)
	ListMaintenanceExpenses(ctx context.Context, vehicleID string) ([]models.MaintenanceExpense, error)
}

type ExportService struct {
	repo    exportStore
	mileage *MileageService
}

func NewExportService(repo exportStore) *ExportService {
	return &ExportService{repo: repo}
}

// WithMileage enables the mileage export.
func (s *ExportService) WithMileage(m *MileageService) *ExportService {
	s.mileage = m
	return s
}

type exportTable struct {
	headers []string
	rows    [][]any
}

// ExportResult is the rendered file.
type ExportResult struct {
	Body        []byte
	ContentType string
	Extension   string
}

func ValidExportType(t ExportType) bool {
	switch t {
	case ExportCharges, ExportDrives, ExportFuel, ExportOdometer, ExportExpenses, ExportMaintenance, ExportMileage:
		return true
	}
	return false
}

// Export renders one kind of record of a vehicle. Column names are those of the CSV import, so charges, drives,
// fill-ups and odometer readings can be imported back; amounts are in the currency stored with each record.
func (s *ExportService) Export(ctx context.Context, vehicle *models.Vehicle, opts ExportOptions) (*ExportResult, error) {
	if !ValidExportType(opts.Type) {
		return nil, apierror.New("export.invalid_type", "Unknown export type")
	}
	if opts.Format != ExportCSV && opts.Format != ExportJSON {
		return nil, apierror.New("export.invalid_format", "The format must be csv or json")
	}
	if opts.Unit != "mi" {
		opts.Unit = "km"
	}

	var table *exportTable
	var err error
	switch opts.Type {
	case ExportCharges:
		table, err = s.charges(ctx, vehicle, opts)
	case ExportDrives:
		table, err = s.drives(ctx, vehicle, opts)
	case ExportFuel:
		table, err = s.fuel(ctx, vehicle, opts)
	case ExportOdometer:
		table, err = s.odometer(ctx, vehicle, opts)
	case ExportExpenses:
		table, err = s.expenses(ctx, vehicle, opts)
	case ExportMaintenance:
		table, err = s.maintenance(ctx, vehicle, opts)
	case ExportMileage:
		table, err = s.mileageTable(ctx, vehicle, opts)
	}
	if err != nil {
		return nil, err
	}
	if opts.Format == ExportJSON {
		return renderExportJSON(table)
	}
	return renderExportCSV(table)
}

func inPeriod(t time.Time, opts ExportOptions) bool {
	return (opts.From == nil || !t.Before(*opts.From)) && (opts.To == nil || !t.After(*opts.To))
}

func exportTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func exportDistance(km float64, unit string) float64 {
	if unit == "mi" {
		km /= kmPerMile
	}
	return math.Round(km*1000) / 1000
}

func distanceHeader(base, unit string) string { return base + "_" + unit }

func optFloat(v *float64, f func(float64) any) any {
	if v == nil {
		return ""
	}
	return f(*v)
}

func optString(v *string) any {
	if v == nil {
		return ""
	}
	return *v
}

func (s *ExportService) charges(ctx context.Context, v *models.Vehicle, o ExportOptions) (*exportTable, error) {
	list, _, err := s.repo.ListCharges(ctx, v.ID, false, math.MaxInt32, 0)
	if err != nil {
		return nil, err
	}
	t := &exportTable{headers: []string{"date", "kwh", "cost", "currency", "fx_rate", distanceHeader("odometer", o.Unit), "location"}}
	for i := len(list) - 1; i >= 0; i-- {
		c := list[i]
		if !inPeriod(c.Date, o) {
			continue
		}
		var cost any = ""
		if c.Cost != nil {
			cost = c.Cost.String()
		}
		t.rows = append(t.rows, []any{
			exportTime(c.Date), c.KwhAdded, cost, c.Currency,
			optFloat(c.FxRate, func(f float64) any { return f }),
			optFloat(c.Odometer, func(f float64) any { return exportDistance(f, o.Unit) }),
			optString(c.Address),
		})
	}
	return t, nil
}

func (s *ExportService) drives(ctx context.Context, v *models.Vehicle, o ExportOptions) (*exportTable, error) {
	filter := database.DriveFilter{Tag: o.Tag, From: o.From, To: o.To}
	list, _, err := s.repo.ListDrives(ctx, v.ID, filter, math.MaxInt32, 0)
	if err != nil {
		return nil, err
	}
	t := &exportTable{headers: []string{"start_time", "end_time", distanceHeader("distance", o.Unit), "kwh", "start_address", "end_address", "tag"}}
	for i := len(list) - 1; i >= 0; i-- {
		d := list[i]
		t.rows = append(t.rows, []any{
			exportTime(d.StartTime), exportTime(d.EndTime), exportDistance(d.DistanceKm, o.Unit),
			optFloat(d.EnergyConsumedKwh, func(f float64) any { return f }),
			optString(d.StartAddress), optString(d.EndAddress), strings.Join(d.Tags, ";"),
		})
	}
	return t, nil
}

func (s *ExportService) fuel(ctx context.Context, v *models.Vehicle, o ExportOptions) (*exportTable, error) {
	list, err := s.repo.ListFuelLogs(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	t := &exportTable{headers: []string{"date", "liters", "price_per_liter", "amount", "fuel_type", "is_full_tank", distanceHeader("odometer", o.Unit), "notes"}}
	sorted := make([]models.FuelLog, len(list))
	copy(sorted, list)
	sortByDate(sorted, func(f models.FuelLog) time.Time { return f.Date })
	for _, f := range sorted {
		if !inPeriod(f.Date, o) {
			continue
		}
		t.rows = append(t.rows, []any{
			exportTime(f.Date),
			optFloat(f.Liters, func(x float64) any { return x }),
			optFloat(f.PricePerLiter, func(x float64) any { return x }),
			f.Amount.String(), optString(f.FuelType), f.IsFullTank,
			optFloat(f.Odometer, func(x float64) any { return exportDistance(x, o.Unit) }),
			optString(f.Notes),
		})
	}
	return t, nil
}

func (s *ExportService) odometer(ctx context.Context, v *models.Vehicle, o ExportOptions) (*exportTable, error) {
	list, err := s.repo.ListOdometerCheckpoints(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	t := &exportTable{headers: []string{"date", distanceHeader("odometer", o.Unit), "notes"}}
	sorted := make([]models.OdometerCheckpoint, len(list))
	copy(sorted, list)
	sortByDate(sorted, func(c models.OdometerCheckpoint) time.Time { return c.Date })
	for _, c := range sorted {
		if !inPeriod(c.Date, o) {
			continue
		}
		t.rows = append(t.rows, []any{exportTime(c.Date), exportDistance(c.Odometer, o.Unit), optString(c.Notes)})
	}
	return t, nil
}

func (s *ExportService) expenses(ctx context.Context, v *models.Vehicle, o ExportOptions) (*exportTable, error) {
	list, err := s.repo.ListDriveExpenses(ctx, v.ID, o.Lang)
	if err != nil {
		return nil, err
	}
	t := &exportTable{headers: []string{"date", "type", "amount", "currency", "fx_rate", "trip", "notes", "source"}}
	sorted := make([]models.DriveExpense, len(list))
	copy(sorted, list)
	sortByDate(sorted, func(e models.DriveExpense) time.Time { return e.Date })
	for _, e := range sorted {
		if !inPeriod(e.Date, o) {
			continue
		}
		trip := ""
		switch {
		case e.TripGroupName != nil:
			trip = *e.TripGroupName
		case e.DriveTitle != nil:
			trip = *e.DriveTitle
		}
		t.rows = append(t.rows, []any{
			exportTime(e.Date), e.Type, e.Amount.String(), e.Currency,
			optFloat(e.FxRate, func(f float64) any { return f }), trip, optString(e.Notes), e.Source,
		})
	}
	return t, nil
}

func (s *ExportService) maintenance(ctx context.Context, v *models.Vehicle, o ExportOptions) (*exportTable, error) {
	list, err := s.repo.ListMaintenanceExpenses(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	t := &exportTable{headers: []string{"date", "category", "description", "amount", "currency", "fx_rate", distanceHeader("odometer", o.Unit), "amortization_mode", "coverage_months"}}
	sorted := make([]models.MaintenanceExpense, len(list))
	copy(sorted, list)
	sortByDate(sorted, func(m models.MaintenanceExpense) time.Time { return m.Date })
	for _, m := range sorted {
		if !inPeriod(m.Date, o) {
			continue
		}
		var months any = ""
		if m.CoverageMonths != nil {
			months = *m.CoverageMonths
		}
		t.rows = append(t.rows, []any{
			exportTime(m.Date), m.Category, m.Description, m.Amount.String(), m.Currency,
			optFloat(m.FxRate, func(f float64) any { return f }),
			optFloat(m.Odometer, func(f float64) any { return exportDistance(f, o.Unit) }),
			m.AmortizationMode, months,
		})
	}
	return t, nil
}

func sortByDate[T any](list []T, at func(T) time.Time) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && at(list[j]).Before(at(list[j-1])); j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

// csvSafe stops a spreadsheet from running a text cell as a formula.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func csvCell(v any) string {
	switch x := v.(type) {
	case string:
		return csvSafe(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case bool:
		return strconv.FormatBool(x)
	}
	return ""
}

func renderExportCSV(t *exportTable) (*ExportResult, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	if err := w.Write(t.headers); err != nil {
		return nil, err
	}
	for _, row := range t.rows {
		cells := make([]string, len(row))
		for i, v := range row {
			cells[i] = csvCell(v)
		}
		if err := w.Write(cells); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return &ExportResult{Body: buf.Bytes(), ContentType: "text/csv; charset=utf-8", Extension: "csv"}, nil
}

func renderExportJSON(t *exportTable) (*ExportResult, error) {
	var buf bytes.Buffer
	buf.WriteString("[")
	for i, row := range t.rows {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString("{")
		for j, v := range row {
			if j > 0 {
				buf.WriteString(",")
			}
			key, _ := json.Marshal(t.headers[j])
			var val []byte
			if s, ok := v.(string); ok && s == "" {
				val = []byte("null")
			} else {
				var err error
				if val, err = json.Marshal(v); err != nil {
					return nil, err
				}
			}
			buf.Write(key)
			buf.WriteString(":")
			buf.Write(val)
		}
		buf.WriteString("}")
	}
	buf.WriteString("]")
	return &ExportResult{Body: buf.Bytes(), ContentType: "application/json", Extension: "json"}, nil
}

func (s *ExportService) mileageTable(ctx context.Context, v *models.Vehicle, o ExportOptions) (*exportTable, error) {
	if s.mileage == nil {
		return nil, apierror.New("export.invalid_type", "Unknown export type")
	}
	report, err := s.mileage.Report(ctx, v, o.UserID, MileageOptions{From: o.From, To: o.To, Tag: o.Tag, RateLabel: o.RateLabel})
	if err != nil {
		return nil, err
	}
	t := &exportTable{headers: []string{"tag", "trips", distanceHeader("distance", o.Unit), "tolls", "allowance", "currency"}}
	for _, row := range report.Tags {
		var allowance any = ""
		if row.Allowance != nil {
			allowance = row.Allowance.String()
		}
		t.rows = append(t.rows, []any{row.Tag, row.Trips, exportDistance(row.DistanceKm, o.Unit), row.Tolls.String(), allowance, report.Currency})
	}
	return t, nil
}
