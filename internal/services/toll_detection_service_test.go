package services

import (
	"math"
	"testing"

	"github.com/teslacost/teslacost/internal/teslamate"
)

func TestTraceSpeedsKmh_ConvertsMilesPerHour(t *testing.T) {
	speeds := traceSpeedsKmh([]teslamate.DrivePosition{{Speed: 0}, {Speed: 50}}, "mi")
	if len(speeds) != 2 || speeds[0] != 0 || math.Abs(speeds[1]-80.4672) > 1e-9 {
		t.Fatalf("expected [0 80.4672] km/h, got %v", speeds)
	}
}

func TestTraceSpeedsKmh_KeepsKilometersPerHour(t *testing.T) {
	speeds := traceSpeedsKmh([]teslamate.DrivePosition{{Speed: 130}}, "km")
	if len(speeds) != 1 || speeds[0] != 130 {
		t.Fatalf("expected [130], got %v", speeds)
	}
}

func TestTraceSpeedsKmh_AllZeroMeansNoSpeeds(t *testing.T) {
	if speeds := traceSpeedsKmh([]teslamate.DrivePosition{{Speed: 0}, {Speed: 0}}, "km"); speeds != nil {
		t.Fatalf("a trace without any speed has no speed information, got %v", speeds)
	}
}
