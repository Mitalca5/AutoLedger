package services

import (
	"testing"
	"time"
)

func TestEstimateOdometerAt(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	anchors := []OdometerAnchor{{day(11), 1100}, {day(1), 1000}, {day(21), 1500}}

	cases := []struct {
		name    string
		anchors []OdometerAnchor
		at      time.Time
		km      float64
		source  string
		ok      bool
	}{
		{"on a reading", anchors, day(11), 1100, OdometerEstimateExact, true},
		{"halfway between two readings", anchors, day(6), 1050, OdometerEstimateInterpolated, true},
		{"between the last two readings", anchors, day(16), 1300, OdometerEstimateInterpolated, true},
		{"before the first reading", anchors, day(1).AddDate(0, 0, -5), 1000, OdometerEstimateBounded, true},
		{"after the last reading", anchors, day(30), 1500, OdometerEstimateBounded, true},
		{"a lower reading later is ignored", []OdometerAnchor{{day(1), 1000}, {day(5), 900}, {day(11), 1100}}, day(6), 1050, OdometerEstimateInterpolated, true},
		{"zero readings are ignored", []OdometerAnchor{{day(1), 0}, {day(11), 1100}}, day(5), 1100, OdometerEstimateBounded, true},
		{"no reading", nil, day(5), 0, "", false},
	}
	for _, c := range cases {
		km, source, ok := EstimateOdometerAt(c.anchors, c.at)
		if ok != c.ok || source != c.source || km < c.km-0.001 || km > c.km+0.001 {
			t.Errorf("%s: got %v %q %v, want %v %q %v", c.name, km, source, ok, c.km, c.source, c.ok)
		}
	}
}
