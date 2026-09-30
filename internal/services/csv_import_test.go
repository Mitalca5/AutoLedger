package services

import (
	"testing"
)

func TestCSVImportPreview(t *testing.T) {
	service := NewCSVImportService(nil)

	t.Run("preview charges csv with comma", func(t *testing.T) {
		csvData := `date,kwh,cost,currency,location
2026-09-15 14:30:00,42.5,18.50,EUR,Ionity Aire
2026-09-16 10:00:00,20.0,8.00,EUR,Home Wallbox
`
		res, err := service.Preview([]byte(csvData))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Type != ImportTypeCharges {
			t.Errorf("expected type %v, got %v", ImportTypeCharges, res.Type)
		}
		if res.TotalRows != 2 {
			t.Errorf("expected 2 rows, got %d", res.TotalRows)
		}
		if len(res.SampleRows) != 2 {
			t.Errorf("expected 2 sample rows, got %d", len(res.SampleRows))
		}
	})

	t.Run("preview drives csv with semicolon", func(t *testing.T) {
		csvData := "start_time;end_time;distance_km;kwh;start_address;end_address;tag\n2026-09-15 08:00;2026-09-15 08:45;45,5;7,2;Nantes;Rennes;Pro\n"
		res, err := service.Preview([]byte(csvData))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Type != ImportTypeDrives {
			t.Errorf("expected type %v, got %v", ImportTypeDrives, res.Type)
		}
		if res.TotalRows != 1 {
			t.Errorf("expected 1 row, got %d", res.TotalRows)
		}
	})

	t.Run("preview empty csv returns error", func(t *testing.T) {
		_, err := service.Preview([]byte("   \n"))
		if err == nil {
			t.Errorf("expected error on empty CSV, got nil")
		}
	})
}

func TestFlexibleParsers(t *testing.T) {
	t.Run("parseFlexibleFloat", func(t *testing.T) {
		tests := []struct {
			input string
			want  float64
		}{
			{"12.34", 12.34},
			{"12,34", 12.34},
			{" 1 234,56 ", 1234.56},
		}
		for _, tc := range tests {
			got, err := parseFlexibleFloat(tc.input)
			if err != nil || got != tc.want {
				t.Errorf("parseFlexibleFloat(%q) = %v, err=%v; want %v", tc.input, got, err, tc.want)
			}
		}
	})

	t.Run("parseFlexibleTime", func(t *testing.T) {
		validDates := []string{
			"2026-09-30T14:00:00Z",
			"2026-09-30 14:00:00",
			"2026-09-30 14:00",
			"30/09/2026 14:00",
			"2026-09-30",
		}
		for _, vd := range validDates {
			if _, err := parseFlexibleTime(vd); err != nil {
				t.Errorf("parseFlexibleTime(%q) failed: %v", vd, err)
			}
		}
	})
}
