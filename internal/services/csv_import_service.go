package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
)

type ImportType string

const (
	ImportTypeCharges  ImportType = "CHARGES"
	ImportTypeDrives   ImportType = "DRIVES"
	ImportTypeFuel     ImportType = "FUEL"
	ImportTypeOdometer ImportType = "ODOMETER"
	ImportTypeUnknown  ImportType = "UNKNOWN"
)

// maxListedRowErrors bounds the row errors returned to the client; ErrorCount keeps the real total.
const maxListedRowErrors = 50

// CSVImportOptions are the choices of one import: the preview and the execution run the same pipeline with them.
type CSVImportOptions struct {
	Type           ImportType
	SkipDuplicates bool
	// DistanceUnit ("km" or "mi") is the unit of the distance columns that carry no _km / _mi suffix.
	DistanceUnit string
}

type CSVPreviewResult struct {
	Type            ImportType        `json:"type"`
	TotalRows       int               `json:"total_rows"`
	ValidRows       int               `json:"valid_rows"`
	InvalidRows     int               `json:"invalid_rows"`
	DuplicateRows   int               `json:"duplicate_rows"`
	Headers         []string          `json:"headers"`
	SampleRows      []map[string]any  `json:"sample_rows"`
	Errors          []*apierror.Error `json:"errors,omitempty"`
	ErrorsTruncated bool              `json:"errors_truncated,omitempty"`
}

type CSVExecuteResult struct {
	Type          ImportType `json:"type"`
	TotalRows     int        `json:"total_rows"`
	ImportedCount int        `json:"imported_count"`
	SkippedCount  int        `json:"skipped_count"`
	ErrorCount    int        `json:"error_count"`
	// Committed is false when nothing was written: an invalid row or a database error cancels the whole file.
	Committed       bool              `json:"committed"`
	Errors          []*apierror.Error `json:"errors,omitempty"`
	ErrorsTruncated bool              `json:"errors_truncated,omitempty"`
}

type CSVImportService struct {
	repo *database.Repository
	loc  *time.Location
}

// NewCSVImportService reads the dates without an explicit offset in the given IANA timezone (APP_TIMEZONE).
func NewCSVImportService(repo *database.Repository, timezone string) *CSVImportService {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		slog.Warn("unknown timezone for CSV imports, using UTC", "timezone", timezone, "error", err)
		loc = time.UTC
	}
	return &CSVImportService{repo: repo, loc: loc}
}

// detectDelimiter inspects the first chunk of data to find the most probable delimiter.
func detectDelimiter(data []byte) rune {
	commaCount := bytes.Count(data, []byte{','})
	semicolonCount := bytes.Count(data, []byte{';'})
	tabCount := bytes.Count(data, []byte{'\t'})

	if semicolonCount > commaCount && semicolonCount > tabCount {
		return ';'
	}
	if tabCount > commaCount && tabCount > semicolonCount {
		return '\t'
	}
	return ','
}

// parseFlexibleTime tries common date and time layouts; a date without an offset is read in loc.
func parseFlexibleTime(raw string, loc *time.Location) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"02/01/2006 15:04:05",
		"02/01/2006 15:04",
		"02/01/2006",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, raw, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unrecognized date")
}

// parseFlexibleFloat parses numbers allowing both '.' and ',' decimal separators.
func parseFlexibleFloat(raw string) (float64, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, ",", ".")
	return strconv.ParseFloat(cleaned, 64)
}

func normalizeHeader(h string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))
}

func detectTypeFromHeaders(headers []string) ImportType {
	has := make(map[string]bool, len(headers))
	for _, h := range headers {
		has[normalizeHeader(h)] = true
	}
	any := func(names ...string) bool {
		for _, n := range names {
			if has[n] {
				return true
			}
		}
		return false
	}

	switch {
	case any("distance", "distance_km", "distance_mi", "dist", "duration", "duration_min"):
		return ImportTypeDrives
	case any("liters", "litres", "fuel_type", "price_per_liter", "price_per_litre", "volume"):
		return ImportTypeFuel
	case any("kwh", "kwh_added", "cost", "amount", "station"):
		return ImportTypeCharges
	case any("odometer", "odo", "odometer_km", "odometer_mi"):
		return ImportTypeOdometer
	}
	return ImportTypeUnknown
}

// readCSVRecords parses the file and returns, next to each record, its line number in the file.
func readCSVRecords(content []byte) (records [][]string, lines []int, err error) {
	content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))
	reader := csv.NewReader(bytes.NewReader(content))
	reader.Comma = detectDelimiter(content)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, apierror.Newf("import.invalid_csv", "Invalid CSV file: %v", err)
		}
		line, _ := reader.FieldPos(0)
		records = append(records, record)
		lines = append(lines, line)
	}
	if len(records) < 2 {
		return nil, nil, apierror.New("import.no_rows", "The CSV file must contain a header and at least one row")
	}
	return records, lines, nil
}

type plannedRow struct {
	insert func(ctx context.Context, tx *database.Repository) error
	line   int
}

// csvPlan is the outcome of validating a whole file against a vehicle, before anything is written.
type csvPlan struct {
	importType ImportType
	headers    []string
	total      int
	sample     []map[string]any
	rows       []plannedRow
	duplicates int
	invalid    int
	errors     []*apierror.Error
}

func (p *csvPlan) addError(e *apierror.Error) {
	p.invalid++
	if len(p.errors) < maxListedRowErrors {
		p.errors = append(p.errors, e)
	}
}

// plan validates every row and looks up the duplicates without writing anything: Preview reports it, Execute
// writes it.
func (s *CSVImportService) plan(ctx context.Context, vehicle *models.Vehicle, content []byte, opts CSVImportOptions) (*csvPlan, error) {
	records, lines, err := readCSVRecords(content)
	if err != nil {
		return nil, err
	}

	headers := records[0]
	importType := opts.Type
	if importType == ImportTypeUnknown || importType == "" {
		importType = detectTypeFromHeaders(headers)
	}
	parse, ok := rowParsers[importType]
	if !ok {
		return nil, apierror.Newf("import.unsupported_type", "Unrecognized or unsupported import type %q", string(importType))
	}
	if importType == ImportTypeCharges && vehicle.Powertrain == models.PowertrainICE {
		return nil, apierror.New("import.ice_charges", "A combustion vehicle has no charges to import: import its fill-ups instead")
	}

	columns := make(map[string]int, len(headers))
	for i, h := range headers {
		if _, taken := columns[normalizeHeader(h)]; !taken {
			columns[normalizeHeader(h)] = i
		}
	}
	unit := opts.DistanceUnit
	if unit != "mi" {
		unit = "km"
	}
	rc := &rowContext{columns: columns, vehicle: vehicle, loc: s.loc, unit: unit}

	p := &csvPlan{importType: importType, headers: headers}
	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
			continue
		}
		p.total++
		if len(p.sample) < 5 {
			p.sample = append(p.sample, sampleRow(headers, row))
		}

		parsed, rowErr := parse(rc, row, lines[i])
		if rowErr != nil {
			p.addError(rowErr)
			continue
		}
		if opts.SkipDuplicates {
			duplicate, err := parsed.isDuplicate(ctx, s.repo)
			if err != nil {
				return nil, err
			}
			if duplicate {
				p.duplicates++
				continue
			}
		}
		p.rows = append(p.rows, plannedRow{insert: parsed.insert, line: lines[i]})
	}
	return p, nil
}

func sampleRow(headers, row []string) map[string]any {
	m := make(map[string]any, len(headers))
	for j, val := range row {
		if j < len(headers) {
			m[headers[j]] = strings.TrimSpace(val)
		}
	}
	return m
}

// Preview runs the import pipeline without writing: the rows it counts valid are the ones Execute imports.
func (s *CSVImportService) Preview(ctx context.Context, vehicle *models.Vehicle, content []byte, opts CSVImportOptions) (*CSVPreviewResult, error) {
	p, err := s.plan(ctx, vehicle, content, opts)
	if err != nil {
		return nil, err
	}
	return &CSVPreviewResult{
		Type:            p.importType,
		TotalRows:       p.total,
		ValidRows:       len(p.rows),
		InvalidRows:     p.invalid,
		DuplicateRows:   p.duplicates,
		Headers:         p.headers,
		SampleRows:      p.sample,
		Errors:          p.errors,
		ErrorsTruncated: p.invalid > len(p.errors),
	}, nil
}

// Execute imports the file all or nothing: one invalid row, or one database error, leaves the vehicle untouched.
func (s *CSVImportService) Execute(ctx context.Context, vehicle *models.Vehicle, content []byte, opts CSVImportOptions) (*CSVExecuteResult, error) {
	p, err := s.plan(ctx, vehicle, content, opts)
	if err != nil {
		return nil, err
	}

	res := &CSVExecuteResult{
		Type:            p.importType,
		TotalRows:       p.total,
		SkippedCount:    p.duplicates,
		ErrorCount:      p.invalid,
		Errors:          p.errors,
		ErrorsTruncated: p.invalid > len(p.errors),
	}
	if p.invalid > 0 {
		return res, nil
	}

	var failedLine int
	err = s.repo.WithTx(ctx, func(tx *database.Repository) error {
		for _, row := range p.rows {
			if err := row.insert(ctx, tx); err != nil {
				failedLine = row.line
				return err
			}
		}
		return nil
	})
	if err != nil {
		if failedLine == 0 {
			return nil, err
		}
		slog.Error("CSV import rolled back", "line", failedLine, "error", err)
		res.ErrorCount = 1
		res.Errors = []*apierror.Error{apierror.Newf("import.row.database_error", "Line %d: the row could not be saved, nothing was imported", failedLine)}
		return res, nil
	}
	res.Committed = true
	res.ImportedCount = len(p.rows)
	return res, nil
}
