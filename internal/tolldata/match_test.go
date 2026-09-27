package tolldata

import (
	"math"
	"testing"

	"github.com/teslacost/teslacost/internal/money"
)

// buildStraightTrace returns points along the equator (lat=0) from lon=0 to lon=stepDeg*(n-1),
// where distances are easy to reason about (1 degree ≈ 111km at the equator, both axes).
func buildStraightTrace(n int, stepDeg float64) []LatLon {
	trace := make([]LatLon, n)
	for i := 0; i < n; i++ {
		trace[i] = LatLon{Lat: 0, Lon: stepDeg * float64(i)}
	}
	return trace
}

func TestHaversineMeters_KnownDistance(t *testing.T) {
	a := LatLon{Lat: 0, Lon: 0}
	b := LatLon{Lat: 0, Lon: 1} // 1 degree of longitude at the equator ≈ 111.19 km
	d := haversineMeters(a, b)
	if math.Abs(d-111195) > 500 {
		t.Errorf("expected ~111195m, got %f", d)
	}
}

func TestPointToSegmentMeters_OnSegment(t *testing.T) {
	a := LatLon{Lat: 0, Lon: 0}
	b := LatLon{Lat: 0, Lon: 0.01}
	p := LatLon{Lat: 0, Lon: 0.005} // exactly between a and b
	d := pointToSegmentMeters(p, a, b)
	if d > 1 {
		t.Errorf("expected ~0m for a point on the segment, got %f", d)
	}
}

func TestPointToSegmentMeters_OffToTheSide(t *testing.T) {
	a := LatLon{Lat: 0, Lon: 0}
	b := LatLon{Lat: 0, Lon: 0.01}
	p := LatLon{Lat: 0.001, Lon: 0.005} // ~111m north of the segment's midpoint
	d := pointToSegmentMeters(p, a, b)
	if math.Abs(d-111) > 10 {
		t.Errorf("expected ~111m, got %f", d)
	}
}

func TestMergeCloseVisits_MergesJitter(t *testing.T) {
	trace := buildStraightTrace(5, 0.001) // ~111m steps
	st := Station{Name: "Z"}
	visits := []MatchedStation{
		{Station: st, SegmentIndex: 0},
		{Station: st, SegmentIndex: 1}, // ~111m away, well under MinRevisitGapMeters
	}
	merged := mergeCloseVisits(visits, trace, MinRevisitGapMeters)
	if len(merged) != 1 {
		t.Fatalf("expected jittery visits to merge into 1, got %d", len(merged))
	}
}

func TestMergeCloseVisits_KeepsRealRevisit(t *testing.T) {
	trace := buildStraightTrace(10, 0.005) // ~555m steps
	st := Station{Name: "Z"}
	visits := []MatchedStation{
		{Station: st, SegmentIndex: 0},
		{Station: st, SegmentIndex: 5}, // ~2775m away, well over MinRevisitGapMeters
	}
	merged := mergeCloseVisits(visits, trace, MinRevisitGapMeters)
	if len(merged) != 2 {
		t.Fatalf("expected distinct revisits to stay separate, got %d", len(merged))
	}
}

func TestDetectCrossings_MultipleSegmentsOnOneDrive(t *testing.T) {
	// 21 points, ~111m apart, spanning ~2.2km along the equator.
	trace := buildStraightTrace(21, 0.001)

	stations := []Station{
		{Name: "OPEN1", Type: "open", Operator: "OP1", Lat: 0, Lon: 0.002},
		{Name: "S1", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.004},
		{Name: "S2", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.006},
		{Name: "S3", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.008},
		{Name: "S4", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.010},
		{Name: "S5", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.012},
		{Name: "S6", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.014},
		{Name: "LONE_CLOSE", Type: "close", Operator: "OP2", Lat: 0, Lon: 0.020},
		{Name: "FAR_STATION", Type: "open", Operator: "OP3", Lat: 5, Lon: 5},
	}
	networkByStation := map[string]string{
		"S1": "component_1", "S2": "component_1", "S3": "component_1",
		"S4": "component_1", "S5": "component_1", "S6": "component_1",
		"LONE_CLOSE": "component_2",
	}
	ds := &Dataset{
		NetworkByStation: networkByStation,
		OpenPrice:        map[string]money.Cents{"OPEN1": money.FromFloat(1.10)},
		ClosedPrice: map[string]map[string]money.Cents{
			"S1": {"S6": money.FromFloat(12.30)},
		},
	}

	matches := DetectCrossings(trace, nil, stations, DefaultThresholdMeters)

	if len(matches) != 8 {
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Station.Name
		}
		t.Fatalf("expected 8 matches (FAR_STATION excluded), got %d: %v", len(matches), names)
	}

	wantOrder := []string{"OPEN1", "S1", "S2", "S3", "S4", "S5", "S6", "LONE_CLOSE"}
	for i, want := range wantOrder {
		if matches[i].Station.Name != want {
			t.Errorf("match[%d]: expected %q, got %q", i, want, matches[i].Station.Name)
		}
	}

	// Regression test: 6 consecutive closed stations on the same network must collapse into a
	// single S1->S6 segment, not 3 segments paired two-by-two (the originally reported bug).
	segments := ds.BuildSegments(matches)
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d: %+v", len(segments), segments)
	}

	if segments[0].Type != "open" || segments[0].Entry != "OPEN1" || segments[0].Exit != nil {
		t.Errorf("segment[0]: expected open barrier OPEN1, got %+v", segments[0])
	}
	if segments[0].EstimatedPrice == nil || *segments[0].EstimatedPrice != money.FromFloat(1.10) {
		t.Errorf("segment[0]: expected price 1.10, got %+v", segments[0].EstimatedPrice)
	}

	if segments[1].Type != "close" || segments[1].Entry != "S1" ||
		segments[1].Exit == nil || *segments[1].Exit != "S6" || segments[1].Network != "component_1" {
		t.Errorf("segment[1]: expected closed pair S1 -> S6 on component_1, got %+v", segments[1])
	}
	if segments[1].EstimatedPrice == nil || *segments[1].EstimatedPrice != money.FromFloat(12.30) {
		t.Errorf("segment[1]: expected price 12.30, got %+v", segments[1].EstimatedPrice)
	}

	if segments[2].Type != "close" || segments[2].Entry != "LONE_CLOSE" || segments[2].Exit != nil {
		t.Errorf("segment[2]: expected incomplete closed entry LONE_CLOSE with no exit, got %+v", segments[2])
	}
	if segments[2].EstimatedPrice != nil {
		t.Errorf("segment[2]: expected no price for an incomplete segment, got %+v", segments[2].EstimatedPrice)
	}
}

func TestDetectCrossings_NoStationsNearby(t *testing.T) {
	trace := buildStraightTrace(5, 0.002)
	stations := []Station{
		{Name: "FAR", Type: "open", Lat: 10, Lon: 10},
	}
	matches := DetectCrossings(trace, nil, stations, DefaultThresholdMeters)
	if len(matches) != 0 {
		t.Errorf("expected no matches, got %d", len(matches))
	}
}

func TestDetectCrossings_ShortTraceIsIgnored(t *testing.T) {
	matches := DetectCrossings([]LatLon{{Lat: 0, Lon: 0}}, nil, []Station{{Name: "X", Lat: 0, Lon: 0}}, DefaultThresholdMeters)
	if matches != nil {
		t.Errorf("expected nil for a trace with fewer than 2 points, got %v", matches)
	}
}

func TestBuildSegments_ExitAndReentryAtSameGate(t *testing.T) {
	// A drive enters at W, exits at X (e.g. to drop someone off), takes a > 300m detour off the
	// highway, then re-enters through the very same gate X, and finally exits at Y. Stations are
	// spaced ~1km+ apart so the detour and revisit don't spuriously graze W or Y's thresholds.
	trace := []LatLon{
		{Lat: 0, Lon: 0.0000}, // near W
		{Lat: 0, Lon: 0.0050},
		{Lat: 0, Lon: 0.0100}, // near X (visit 1: exit)
		{Lat: 0.01, Lon: 0.0100},
		{Lat: 0.01, Lon: 0.0110}, // detour, ~1.1km away from X
		{Lat: 0.005, Lon: 0.0100},
		{Lat: 0, Lon: 0.0101}, // near X again (visit 2: re-entry)
		{Lat: 0, Lon: 0.0150},
		{Lat: 0, Lon: 0.0200},
		{Lat: 0, Lon: 0.0300}, // near Y (final exit)
	}
	stations := []Station{
		{Name: "W", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.0000},
		{Name: "X", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.0100},
		{Name: "Y", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.0300},
	}
	networkByStation := map[string]string{"W": "component_1", "X": "component_1", "Y": "component_1"}
	ds := &Dataset{NetworkByStation: networkByStation}

	matches := DetectCrossings(trace, nil, stations, DefaultThresholdMeters)

	var xVisits int
	for _, m := range matches {
		if m.Station.Name == "X" {
			xVisits++
		}
	}
	if xVisits != 2 {
		t.Fatalf("expected 2 distinct visits of X (exit then re-entry), got %d in %+v", xVisits, matches)
	}

	segments := ds.BuildSegments(matches)
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments (W->X, X->Y), got %d: %+v", len(segments), segments)
	}
	if segments[0].Entry != "W" || segments[0].Exit == nil || *segments[0].Exit != "X" {
		t.Errorf("segment[0]: expected W -> X, got %+v", segments[0])
	}
	if segments[1].Entry != "X" || segments[1].Exit == nil || *segments[1].Exit != "Y" {
		t.Errorf("segment[1]: expected X -> Y, got %+v", segments[1])
	}
}

func TestBuildSegments_NetworkChangeSplitsSegments(t *testing.T) {
	matches := []MatchedStation{
		{Station: Station{Name: "A", Type: "close", Operator: "OP1"}, SegmentIndex: 0},
		{Station: Station{Name: "B", Type: "close", Operator: "OP2"}, SegmentIndex: 1},
	}
	ds := &Dataset{NetworkByStation: map[string]string{"A": "component_1", "B": "component_2"}}

	segments := ds.BuildSegments(matches)
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments (different networks), got %d: %+v", len(segments), segments)
	}
	if segments[0].Entry != "A" || segments[0].Exit != nil {
		t.Errorf("segment[0]: expected incomplete entry A, got %+v", segments[0])
	}
	if segments[1].Entry != "B" || segments[1].Exit != nil {
		t.Errorf("segment[1]: expected incomplete entry B, got %+v", segments[1])
	}
}

func TestPriceSegment_MissingClosedPriceLeavesNil(t *testing.T) {
	exit := "B"
	seg := Segment{Type: "close", Entry: "A", Exit: &exit}
	ds := &Dataset{ClosedPrice: map[string]map[string]money.Cents{}}
	ds.priceSegment(&seg)
	if seg.EstimatedPrice != nil {
		t.Errorf("expected no price when the pair is absent from ClosedPrice, got %+v", seg.EstimatedPrice)
	}
}

func TestDefaultThresholdIsFiftyMeters(t *testing.T) {
	// 0.00036° of latitude is about 40 m, 0.00072° about 80 m
	trace := []LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.01}}
	stations := []Station{
		{Name: "NEAR", Lat: 0.00036, Lon: 0.005},
		{Name: "FAR", Lat: 0.00072, Lon: 0.005},
	}
	matches := DetectCrossings(trace, nil, stations, DefaultThresholdMeters)
	if len(matches) != 1 || matches[0].Station.Name != "NEAR" {
		t.Fatalf("a station 40 m from the trace is crossed, one 80 m away is not: %+v", matches)
	}
}

// slowNear returns speeds for trace: slowKmh within 0.002° (~220 m) of one of the given
// longitudes (a gate the car goes through), motorwayKmh elsewhere.
func slowNear(trace []LatLon, slowKmh, motorwayKmh float64, lons ...float64) []float64 {
	speeds := make([]float64, len(trace))
	for i, p := range trace {
		speeds[i] = motorwayKmh
		for _, lon := range lons {
			if math.Abs(p.Lon-lon) <= 0.002 {
				speeds[i] = slowKmh
			}
		}
	}
	return speeds
}

func closedStations(network string, byName map[string]float64) ([]Station, map[string]string) {
	var stations []Station
	networkByStation := map[string]string{}
	for name, lon := range byName {
		stations = append(stations, Station{Name: name, Type: "close", Operator: "OP1", Lat: 0, Lon: lon})
		networkByStation[name] = network
	}
	return stations, networkByStation
}

func segmentNames(segments []Segment) []string {
	out := make([]string, len(segments))
	for i, s := range segments {
		out[i] = s.Entry + "->"
		if s.Exit != nil {
			out[i] += *s.Exit
		}
	}
	return out
}

func assertSegments(t *testing.T, segments []Segment, want ...string) {
	t.Helper()
	got := segmentNames(segments)
	if len(got) != len(want) {
		t.Fatalf("expected segments %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected segments %v, got %v", want, got)
		}
	}
}

func TestBuildSegments_ExitThenReentryElsewhereOnTheSameNetwork(t *testing.T) {
	// Annecy -> Les Abrets on the motorway, the road from Les Abrets to Voiron, then Voiron ->
	// Chatuzange on the motorway. The motorway passes two other gates of the network without
	// going through them. All four stations are on one network and every pair is priced.
	trace := buildStraightTrace(61, 0.001)
	stations, networkByStation := closedStations("component_1", map[string]float64{
		"ANNECY": 0.000, "CHAMBERY": 0.010, "LES ABRETS": 0.020, "VOIRON": 0.040, "TULLINS": 0.050, "CHATUZANGE": 0.060,
	})
	price := money.FromFloat(10)
	ds := &Dataset{
		NetworkByStation: networkByStation,
		ClosedPrice: map[string]map[string]money.Cents{
			"ANNECY": {"LES ABRETS": price, "CHATUZANGE": price, "VOIRON": price},
			"VOIRON": {"CHATUZANGE": price},
		},
	}
	speeds := slowNear(trace, 20, 115, 0.000, 0.020, 0.040, 0.060)

	matches := DetectCrossings(trace, speeds, stations, DefaultThresholdMeters)
	if len(matches) != 6 {
		t.Fatalf("expected the 6 stations to be matched, got %d", len(matches))
	}
	assertSegments(t, ds.BuildSegments(matches), "ANNECY->LES ABRETS", "VOIRON->CHATUZANGE")
}

func TestBuildSegments_StationsPassedAtMotorwaySpeedAreNotCrossed(t *testing.T) {
	trace := buildStraightTrace(41, 0.001)
	stations, networkByStation := closedStations("component_1", map[string]float64{
		"S1": 0.002, "S2": 0.010, "S3": 0.018, "S4": 0.026, "S5": 0.034,
	})
	ds := &Dataset{NetworkByStation: networkByStation}
	speeds := slowNear(trace, 30, 90, 0.002, 0.034)

	assertSegments(t, ds.BuildSegments(DetectCrossings(trace, speeds, stations, DefaultThresholdMeters)), "S1->S5")
}

func TestBuildSegments_SpeedJustAboveTheGateLimitIsADriveBy(t *testing.T) {
	trace := buildStraightTrace(21, 0.001)
	stations, networkByStation := closedStations("component_1", map[string]float64{"IN": 0.002, "PAST": 0.010, "OUT": 0.018})
	ds := &Dataset{NetworkByStation: networkByStation}

	atLimit := slowNear(trace, MaxGateSpeedKmh, MaxGateSpeedKmh+1, 0.002, 0.018)
	assertSegments(t, ds.BuildSegments(DetectCrossings(trace, atLimit, stations, DefaultThresholdMeters)), "IN->OUT")

	allAtLimit := slowNear(trace, MaxGateSpeedKmh, MaxGateSpeedKmh)
	assertSegments(t, ds.BuildSegments(DetectCrossings(trace, allAtLimit, stations, DefaultThresholdMeters)), "IN->PAST", "OUT->")
}

func TestBuildSegments_OpenBarrierPassedAtSpeedIsKept(t *testing.T) {
	// Free-flow gantries are crossed at full speed: only closed-network gates are filtered.
	trace := buildStraightTrace(11, 0.001)
	stations := []Station{{Name: "FREE FLOW", Type: "open", Operator: "OP1", Lat: 0, Lon: 0.005}}
	ds := &Dataset{OpenPrice: map[string]money.Cents{"FREE FLOW": money.FromFloat(2)}}
	speeds := slowNear(trace, 0, 130)

	assertSegments(t, ds.BuildSegments(DetectCrossings(trace, speeds, stations, DefaultThresholdMeters)), "FREE FLOW->")
}

func TestBuildSegments_CoLocatedGatesAreOnePassage(t *testing.T) {
	// A barrier and its slip road ~17 m apart are both matched when the car goes through one of
	// them: they are one exit, the priced one.
	trace := buildStraightTrace(31, 0.001)
	stations, networkByStation := closedStations("component_1", map[string]float64{
		"ENTRY": 0.002, "BARRIERE": 0.020, "BRETELLE": 0.02015,
	})
	ds := &Dataset{
		NetworkByStation: networkByStation,
		ClosedPrice:      map[string]map[string]money.Cents{"ENTRY": {"BARRIERE": money.FromFloat(4)}},
	}
	speeds := slowNear(trace, 10, 120, 0.002, 0.020)

	segments := ds.BuildSegments(DetectCrossings(trace, speeds, stations, DefaultThresholdMeters))
	assertSegments(t, segments, "ENTRY->BARRIERE")
	if segments[0].EstimatedPrice == nil || *segments[0].EstimatedPrice != money.FromFloat(4) {
		t.Errorf("expected the priced gate's price, got %+v", segments[0].EstimatedPrice)
	}
}

func TestBuildSegments_MainlineBarrierClosesOneTicketAndOpensTheNext(t *testing.T) {
	trace := buildStraightTrace(31, 0.001)
	stations, networkByStation := closedStations("component_1", map[string]float64{"A": 0.002, "BARRIER": 0.015, "C": 0.028})
	price := money.FromFloat(3)
	ds := &Dataset{
		NetworkByStation: networkByStation,
		ClosedPrice: map[string]map[string]money.Cents{
			"A": {"BARRIER": price, "C": price}, "BARRIER": {"C": price},
		},
	}
	speeds := slowNear(trace, 0, 110, 0.002, 0.015, 0.028)

	assertSegments(t, ds.BuildSegments(DetectCrossings(trace, speeds, stations, DefaultThresholdMeters)), "A->BARRIER", "BARRIER->C")
}

func TestBuildSegments_UnpricedCrossingsPairInOrder(t *testing.T) {
	trace := buildStraightTrace(31, 0.001)
	stations, networkByStation := closedStations("component_1", map[string]float64{"A": 0.002, "B": 0.015, "C": 0.028})
	ds := &Dataset{NetworkByStation: networkByStation}
	speeds := slowNear(trace, 0, 110, 0.002, 0.015, 0.028)

	assertSegments(t, ds.BuildSegments(DetectCrossings(trace, speeds, stations, DefaultThresholdMeters)), "A->B", "C->")
}

func TestDetectCrossings_SpeedsOfAnotherLengthAreIgnored(t *testing.T) {
	trace := buildStraightTrace(5, 0.001)
	stations := []Station{{Name: "X", Type: "close", Lat: 0, Lon: 0.002}}
	matches := DetectCrossings(trace, []float64{1, 2}, stations, DefaultThresholdMeters)
	if len(matches) != 1 || matches[0].MinSpeedKmh != nil {
		t.Fatalf("expected one match with an unknown speed, got %+v", matches)
	}
}
