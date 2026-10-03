package services

import (
	"math"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestAllowanceForSlicePricesOnlyTheDistanceInEachSlice(t *testing.T) {
	to5000 := 5000
	to20000 := 20000
	scale := []models.MileageRate{
		{FromKm: 0, ToKm: &to5000, RatePerKm: 0.5},
		{FromKm: 5000, ToKm: &to20000, RatePerKm: 0.3},
		{FromKm: 20000, RatePerKm: 0.1},
	}
	cases := []struct {
		name       string
		start, end float64
		want       float64
	}{
		{"inside the first slice", 0, 1000, 500},
		{"across two slices", 4000, 6000, 1000*0.5 + 1000*0.3},
		{"across all three", 4000, 21000, 1000*0.5 + 15000*0.3 + 1000*0.1},
		{"beyond the open-ended slice", 30000, 31000, 100},
		{"empty range", 100, 100, 0},
	}
	for _, c := range cases {
		if got := AllowanceForSlice(scale, c.start, c.end); math.Abs(got-c.want) > 1e-6 {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if got := AllowanceForSlice(nil, 0, 1000); got != 0 {
		t.Errorf("no scale should give no allowance, got %v", got)
	}
}
