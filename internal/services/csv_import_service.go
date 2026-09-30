package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

type ImportType string

const (
	ImportTypeCharges ImportType = "CHARGES"
	ImportTypeDrives  ImportType = "DRIVES"
	ImportTypeUnknown ImportType = "UNKNOWN"
)

type CSVPreviewResult struct {
	Type           ImportType       `json:"type"`
	TotalRows      int              `json:"total_rows"`
	ValidRows      int              `json:"valid_rows"`
	InvalidRows    int              `json:"invalid_rows"`
	Headers        []string         `json:"headers"`
	SampleRows     []map[string]any `json:"sample_rows"`
	ValidationErrors []string       `json:"validation_errors,omitempty"`
}

type CSVExecuteResult struct {
	Type          ImportType `json:"type"`
	TotalRows     int        `json:"total_rows"`
	ImportedCount int        `json:"imported_count"`
	SkippedCount  int        `json:"skipped_count"`
	ErrorCount    int        `json:"error_count"`
	Errors        []string   `json:"errors,omitempty"`
}

type CSVImportService struct {
	repo *database.Repository
}

func NewCSVImportService(repo *database.Repository) *CSVImportService {
	return &CSVImportService{repo: repo}
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

// parseFlexibleTime tries common date and time layouts.
func parseFlexibleTime(raw string) (time.Time, error) {
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
		if t, err := time.Parse(l, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date %q", raw)
}

// parseFlexibleFloat parses numbers allowing both '.' and ',' decimal separators.
func parseFlexibleFloat(raw string) (float64, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, ",", ".")
	return strconv.ParseFloat(cleaned, 64)
}

func detectTypeFromHeaders(headers []string) ImportType {
	headerMap := make(map[string]bool)
	for _, h := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = true
	}

	if headerMap["distance"] || headerMap["distance_km"] || headerMap["dist"] || headerMap["duration"] || headerMap["duration_min"] {
		return ImportTypeDrives
	}
	if headerMap["kwh"] || headerMap["kwh_added"] || headerMap["cost"] || headerMap["amount"] || headerMap["station"] {
		return ImportTypeCharges
	}
	return ImportTypeUnknown
}

func (s *CSVImportService) Preview(rawContent []byte) (*CSVPreviewResult, error) {
	delimiter := detectDelimiter(rawContent)
	reader := csv.NewReader(bytes.NewReader(rawContent))
	reader.Comma = delimiter
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV content: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file must contain an en-tête and at least one row")
	}

	headers := records[0]
	importType := detectTypeFromHeaders(headers)

	headerIndex := make(map[string]int)
	for i, h := range headers {
		norm := strings.ToLower(strings.TrimSpace(h))
		headerIndex[norm] = i
	}

	result := &CSVPreviewResult{
		Type:       importType,
		TotalRows:  len(records) - 1,
		Headers:    headers,
		SampleRows: make([]map[string]any, 0),
	}

	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
			continue
		}

		rowMap := make(map[string]any)
		for j, val := range row {
			if j < len(headers) {
				rowMap[headers[j]] = strings.TrimSpace(val)
			}
		}

		if len(result.SampleRows) < 5 {
			result.SampleRows = append(result.SampleRows, rowMap)
		}
		result.ValidRows++
	}

	return result, nil
}

func (s *CSVImportService) Execute(ctx context.Context, vehicle *models.Vehicle, rawContent []byte, targetType ImportType, skipDuplicates bool) (*CSVExecuteResult, error) {
	delimiter := detectDelimiter(rawContent)
	reader := csv.NewReader(bytes.NewReader(rawContent))
	reader.Comma = delimiter
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV has no data rows")
	}

	headers := records[0]
	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	if targetType == ImportTypeUnknown || targetType == "" {
		targetType = detectTypeFromHeaders(headers)
	}

	res := &CSVExecuteResult{
		Type:      targetType,
		TotalRows: len(records) - 1,
	}

	getCol := func(row []string, names ...string) string {
		for _, n := range names {
			if idx, ok := headerMap[n]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
		}
		return ""
	}

	switch targetType {
	case ImportTypeCharges:
		for i := 1; i < len(records); i++ {
			row := records[i]
			if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
				continue
			}

			dateStr := getCol(row, "date", "time", "start_time", "datetime")
			kwhStr := getCol(row, "kwh", "kwh_added", "energy", "energy_kwh")
			costStr := getCol(row, "cost", "amount", "price", "total_cost")
			odoStr := getCol(row, "odometer", "odo", "km")
			addressStr := getCol(row, "location", "address", "station", "place")
			currencyStr := getCol(row, "currency", "curr")

			t, err := parseFlexibleTime(dateStr)
			if err != nil {
				res.ErrorCount++
				res.Errors = append(res.Errors, fmt.Sprintf("row %d: invalid date %q", i+1, dateStr))
				continue
			}

			kwh, err := parseFlexibleFloat(kwhStr)
			if err != nil || kwh <= 0 {
				res.ErrorCount++
				res.Errors = append(res.Errors, fmt.Sprintf("row %d: invalid kWh %q", i+1, kwhStr))
				continue
			}

			if skipDuplicates {
				if hasDup, _ := s.repo.HasDuplicateCharge(ctx, vehicle.ID, t, kwh); hasDup {
					res.SkippedCount++
					continue
				}
			}

			var costCents *money.Cents
			if costStr != "" {
				if costFloat, err := parseFlexibleFloat(costStr); err == nil && costFloat >= 0 {
					cents := money.FromFloat(costFloat)
					costCents = &cents
				}
			}

			var odo *float64
			if odoStr != "" {
				if odoFloat, err := parseFlexibleFloat(odoStr); err == nil && odoFloat >= 0 {
					odo = &odoFloat
				}
			}

			curr := vehicle.Currency
			if currencyStr != "" && len(currencyStr) == 3 {
				curr = strings.ToUpper(currencyStr)
			}

			charge := &models.ChargeLog{
				VehicleID: vehicle.ID,
				Date:      t,
				KwhAdded:  kwh,
				Cost:      costCents,
				Currency:  curr,
				Odometer:  odo,
				Address:   &addressStr,
				IsManual:  true,
			}

			if err := s.repo.CreateManualCharge(ctx, charge); err != nil {
				res.ErrorCount++
				res.Errors = append(res.Errors, fmt.Sprintf("row %d: database error: %v", i+1, err))
			} else {
				res.ImportedCount++
			}
		}

	case ImportTypeDrives:
		for i := 1; i < len(records); i++ {
			row := records[i]
			if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
				continue
			}

			startStr := getCol(row, "start_time", "date", "start", "datetime")
			endStr := getCol(row, "end_time", "end")
			distStr := getCol(row, "distance_km", "distance", "dist", "km")
			kwhStr := getCol(row, "kwh", "energy", "energy_consumed_kwh")
			startAddr := getCol(row, "start_address", "start_location", "origin")
			endAddr := getCol(row, "end_address", "end_location", "destination")
			tagStr := getCol(row, "tag", "tags", "purpose")

			startTime, err := parseFlexibleTime(startStr)
			if err != nil {
				res.ErrorCount++
				res.Errors = append(res.Errors, fmt.Sprintf("row %d: invalid start time %q", i+1, startStr))
				continue
			}

			dist, err := parseFlexibleFloat(distStr)
			if err != nil || dist <= 0 || dist > 3000 {
				res.ErrorCount++
				res.Errors = append(res.Errors, fmt.Sprintf("row %d: invalid distance %q", i+1, distStr))
				continue
			}

			if skipDuplicates {
				if hasDup, _ := s.repo.HasDuplicateDrive(ctx, vehicle.ID, startTime, dist); hasDup {
					res.SkippedCount++
					continue
				}
			}

			endTime := startTime
			if endStr != "" {
				if et, err := parseFlexibleTime(endStr); err == nil && et.After(startTime) {
					endTime = et
				}
			}
			if endTime.Equal(startTime) {
				duration := int(math.Max(1, math.Round((dist / 50.0) * 60)))
				endTime = startTime.Add(time.Duration(duration) * time.Minute)
			}
			durationMin := int(math.Max(1, math.Round(endTime.Sub(startTime).Minutes())))

			var energy float64
			var cons100 float64
			if kwhStr != "" {
				if parsedKwh, err := parseFlexibleFloat(kwhStr); err == nil && parsedKwh > 0 {
					energy = parsedKwh
					cons100 = (energy / dist) * 100
				}
			}
			if energy == 0 {
				if vehicle.EstimatedKwh100km != nil && *vehicle.EstimatedKwh100km > 0 {
					cons100 = *vehicle.EstimatedKwh100km
				} else {
					cons100 = 16.0
				}
				energy = (cons100 * dist) / 100
			}

			tags := []string{}
			if tagStr != "" {
				tags = append(tags, tagStr)
			}

			drive := &models.Drive{
				VehicleID:           vehicle.ID,
				StartTime:           startTime,
				EndTime:             endTime,
				DistanceKm:          dist,
				DurationMin:         durationMin,
				StartAddress:        &startAddr,
				EndAddress:          &endAddr,
				EnergyConsumedKwh:   &energy,
				ConsumptionKwh100km: &cons100,
				Tags:                tags,
				IsManual:            true,
			}

			if err := s.repo.CreateManualDrive(ctx, drive); err != nil {
				res.ErrorCount++
				res.Errors = append(res.Errors, fmt.Sprintf("row %d: database error: %v", i+1, err))
			} else {
				res.ImportedCount++
			}
		}

	default:
		return nil, fmt.Errorf("unrecognized or unsupported import type %q", targetType)
	}

	return res, nil
}
