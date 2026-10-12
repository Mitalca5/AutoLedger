package services

import (
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestForecastUsesZeroOwnershipOrigin(t *testing.T) {
	now := d(2026, 2, 1)
	in := TireForecastInput{Now: now, RemainingKm: 5000, Season: models.TireSeasonAllSeason, Anchors: []OdometerAnchor{{Date: d(2026, 1, 1), Km: 0, Origin: true}, {Date: now, Km: 1000}}}
	got := ForecastTireReplacement(in)
	if got == nil || got.MonthlyKm != 1000 {
		t.Fatalf("expected forecast at 1000 km/month: %+v", got)
	}
	in.Anchors[0].Km = 1
	in.Anchors[1].Km = 1001
	other := ForecastTireReplacement(in)
	if other == nil || got.ReplacementDate != other.ReplacementDate {
		t.Fatalf("zero origin changed forecast: %+v %+v", got, other)
	}
}
