package services

import (
	"math"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestMonthlyTireProfileIncludesZeroOwnershipOrigin(t *testing.T) {
	anchors := []OdometerAnchor{{Date: d(2026, 1, 1), Km: 0, Origin: true}, {Date: d(2026, 2, 1), Km: 1000}, {Date: d(2026, 3, 1), Km: 3000}}
	km, observed := monthlyKmProfile(anchors, d(2026, 3, 1))
	if !observed[0] || !observed[1] || km[0] != 1000 || km[1] != 2000 {
		t.Fatalf("monthly profile lost the origin: %v %v", km, observed)
	}
	for i := 2; i < 12; i++ {
		if observed[i] {
			t.Fatalf("unobserved month %d reported", i+1)
		}
	}
	var january [12]bool
	january[0] = true
	_, avg, ok := monthlyKmWhileMounted(anchors, d(2026, 3, 1), january)
	if !ok || avg != 1000 {
		t.Fatalf("mounted-season profile: %v %v", avg, ok)
	}
}

func TestTireLookbackClampsLeapDay(t *testing.T) {
	now := d(2024, 2, 29)
	from := models.AddCalendarMonths(now, -12)
	if !from.Equal(d(2023, 2, 28)) {
		t.Fatalf("lookback starts at %v", from)
	}
	anchors := []OdometerAnchor{{Date: d(2023, 2, 28), Km: 0, Origin: true}, {Date: d(2023, 3, 1), Km: 1000}, {Date: now, Km: 12000}}
	got := averageMonthlyKm(anchors, now)
	want := 12000.0 / (366.0 / (365.0 / 12))
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("leap-day mileage excluded: %v, want %v", got, want)
	}
}
