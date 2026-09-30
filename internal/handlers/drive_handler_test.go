package handlers

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseDriveFilter(t *testing.T) {
	t.Run("empty query params", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/vehicles/v1/drives", nil)
		f := parseDriveFilter(req)

		if f.Tag != "" {
			t.Errorf("expected empty tag, got %s", f.Tag)
		}
		if f.UnqualifiedOnly {
			t.Errorf("expected unqualifiedOnly false, got true")
		}
		if f.From != nil {
			t.Errorf("expected nil From, got %v", f.From)
		}
		if f.To != nil {
			t.Errorf("expected nil To, got %v", f.To)
		}
		if f.Query != "" {
			t.Errorf("expected empty Query, got %s", f.Query)
		}
	})

	t.Run("a single drive can be requested by id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/vehicles/v1/drives?drive_id=abc-123", nil)
		if f := parseDriveFilter(req); f.DriveID != "abc-123" {
			t.Errorf("expected DriveID abc-123, got %q", f.DriveID)
		}
	})

	t.Run("dates formatted as YYYY-MM-DD", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/vehicles/v1/drives?from=2026-08-01&to=2026-08-31&q=Paris&tag=Pro&unqualified=true", nil)
		f := parseDriveFilter(req)

		if f.Tag != "Pro" {
			t.Errorf("expected tag Pro, got %s", f.Tag)
		}
		if !f.UnqualifiedOnly {
			t.Errorf("expected unqualifiedOnly true, got false")
		}
		if f.Query != "Paris" {
			t.Errorf("expected Query Paris, got %s", f.Query)
		}
		if f.From == nil {
			t.Fatalf("expected From not nil")
		}
		expectedFrom := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
		if !f.From.Equal(expectedFrom) {
			t.Errorf("expected From %v, got %v", expectedFrom, f.From)
		}

		if f.To == nil {
			t.Fatalf("expected To not nil")
		}
		expectedTo := time.Date(2026, 8, 31, 23, 59, 59, 0, time.UTC)
		if !f.To.Equal(expectedTo) {
			t.Errorf("expected To %v, got %v", expectedTo, f.To)
		}
	})

	t.Run("dates formatted as RFC3339", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/vehicles/v1/drives?from=2026-08-01T10:00:00Z&to=2026-08-15T18:30:00Z", nil)
		f := parseDriveFilter(req)

		if f.From == nil || f.To == nil {
			t.Fatalf("expected non-nil dates")
		}
		if f.From.Format(time.RFC3339) != "2026-08-01T10:00:00Z" {
			t.Errorf("expected RFC3339 From, got %s", f.From.Format(time.RFC3339))
		}
		if f.To.Format(time.RFC3339) != "2026-08-15T18:30:00Z" {
			t.Errorf("expected RFC3339 To, got %s", f.To.Format(time.RFC3339))
		}
	})
}

func TestManualDriveValidation(t *testing.T) {
	t.Run("valid distance and times", func(t *testing.T) {
		req := SaveManualDriveRequest{
			StartTime:  time.Now().Add(-1 * time.Hour),
			DistanceKm: 25.5,
		}
		if req.DistanceKm <= 0 || req.DistanceKm > 3000 {
			t.Errorf("expected valid distance, got %f", req.DistanceKm)
		}
	})

	t.Run("invalid distance triggers check", func(t *testing.T) {
		req := SaveManualDriveRequest{
			StartTime:  time.Now(),
			DistanceKm: -5.0,
		}
		if !(req.DistanceKm <= 0 || req.DistanceKm > 3000) {
			t.Errorf("expected distance <= 0 to be invalid")
		}
	})
}
