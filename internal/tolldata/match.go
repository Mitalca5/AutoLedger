package tolldata

import (
	"math"
	"sort"

	"github.com/teslacost/teslacost/internal/money"
)

// DefaultThresholdMeters is the maximum distance between a drive's GPS trace and a toll
// station for that station to be considered crossed.
const DefaultThresholdMeters = 50.0

// MinRevisitGapMeters is the minimum trace distance required between two visits of the same
// station for them to be treated as two distinct passes (e.g. exiting and re-entering through
// the same gate) rather than GPS/threshold jitter around a single pass. Two different stations
// matched closer than this along the trace are the same passage (co-located gates).
const MinRevisitGapMeters = 300.0

// GateSpeedRadiusMeters is the radius around a station within which the trace's speeds are
// read to tell a gate crossing (the car slows down) from driving past it on the motorway.
const GateSpeedRadiusMeters = 300.0

// MaxGateSpeedKmh is the lowest speed near a closed-network station above which the car drove
// past it on the motorway instead of going through it: a closed-network gate is on the exit or
// entry slip road, and crossing it means slowing down. Open barriers are not filtered this way
// (free-flow gantries are passed at full speed).
const MaxGateSpeedKmh = 60.0

// MatchedStation is a toll station whose distance to the drive's trace fell under the threshold.
type MatchedStation struct {
	Station      Station
	SegmentIndex int      // index into the trace segment [i, i+1] where the minimum distance occurred
	TraceMeters  float64  // distance along the trace from its start to SegmentIndex
	MinSpeedKmh  *float64 // lowest speed near the station during this visit, nil when the trace has no speeds
}

// DetectCrossings returns the stations crossed by trace, in chronological order. A station may
// appear more than once if the trace approaches it, moves away by more than MinRevisitGapMeters,
// then approaches it again (e.g. exiting and re-entering through the same gate). speedsKmh gives
// the speed at each trace point; nil (or a length different from trace) means unknown speeds.
func DetectCrossings(trace []LatLon, speedsKmh []float64, stations []Station, thresholdMeters float64) []MatchedStation {
	if len(trace) < 2 {
		return nil
	}
	if len(speedsKmh) != len(trace) {
		speedsKmh = nil
	}

	minLat, maxLat, minLon, maxLon := boundingBox(trace, thresholdMeters)
	cumulative := cumulativeMeters(trace)

	var matches []MatchedStation
	for _, st := range stations {
		if st.Lat < minLat || st.Lat > maxLat || st.Lon < minLon || st.Lon > maxLon {
			continue // coarse pre-filter: station is nowhere near the trace's bounding box
		}

		visits := detectVisits(trace, st, thresholdMeters)
		for _, v := range mergeCloseVisits(visits, trace, MinRevisitGapMeters) {
			v.TraceMeters = cumulative[v.SegmentIndex]
			v.MinSpeedKmh = minSpeedNear(trace, speedsKmh, st, v.SegmentIndex)
			matches = append(matches, v)
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].SegmentIndex < matches[j].SegmentIndex
	})
	return matches
}

// cumulativeMeters returns, for each trace point, the haversine length of the trace up to it.
func cumulativeMeters(trace []LatLon) []float64 {
	out := make([]float64, len(trace))
	for i := 1; i < len(trace); i++ {
		out[i] = out[i-1] + haversineMeters(trace[i-1], trace[i])
	}
	return out
}

// minSpeedNear returns the lowest speed of the visit's segment [idx, idx+1] and of the
// neighboring trace points within GateSpeedRadiusMeters of the station, nil without speeds.
func minSpeedNear(trace []LatLon, speedsKmh []float64, st Station, idx int) *float64 {
	if speedsKmh == nil {
		return nil
	}
	p := LatLon{Lat: st.Lat, Lon: st.Lon}
	lowest := math.Min(speedsKmh[idx], speedsKmh[idx+1])
	for i := idx - 1; i >= 0 && haversineMeters(trace[i], p) <= GateSpeedRadiusMeters; i-- {
		lowest = math.Min(lowest, speedsKmh[i])
	}
	for i := idx + 2; i < len(trace) && haversineMeters(trace[i], p) <= GateSpeedRadiusMeters; i++ {
		lowest = math.Min(lowest, speedsKmh[i])
	}
	return &lowest
}

// detectVisits groups consecutive trace segments under the threshold into visits, keeping the
// closest point of each visit as its representative index.
func detectVisits(trace []LatLon, st Station, thresholdMeters float64) []MatchedStation {
	var visits []MatchedStation
	inVisit := false
	var bestDist float64
	bestIdx := -1

	for i := 0; i < len(trace)-1; i++ {
		d := pointToSegmentMeters(LatLon{Lat: st.Lat, Lon: st.Lon}, trace[i], trace[i+1])
		switch {
		case d <= thresholdMeters && !inVisit:
			inVisit, bestDist, bestIdx = true, d, i
		case d <= thresholdMeters && d < bestDist:
			bestDist, bestIdx = d, i
		case d > thresholdMeters && inVisit:
			visits = append(visits, MatchedStation{Station: st, SegmentIndex: bestIdx})
			inVisit = false
		}
	}
	if inVisit {
		visits = append(visits, MatchedStation{Station: st, SegmentIndex: bestIdx})
	}
	return visits
}

// mergeCloseVisits merges consecutive visits of the same station when the trace distance
// traveled between them is below minGapMeters, treating them as GPS/threshold jitter around a
// single real pass rather than two distinct crossings.
func mergeCloseVisits(visits []MatchedStation, trace []LatLon, minGapMeters float64) []MatchedStation {
	if len(visits) < 2 {
		return visits
	}

	merged := []MatchedStation{visits[0]}
	for _, v := range visits[1:] {
		last := &merged[len(merged)-1]
		if traceDistanceMeters(trace, last.SegmentIndex, v.SegmentIndex) < minGapMeters {
			continue // within jitter range of the previous visit: drop it
		}
		merged = append(merged, v)
	}
	return merged
}

// traceDistanceMeters sums the haversine length of the trace between segment indices from and to.
func traceDistanceMeters(trace []LatLon, from, to int) float64 {
	var total float64
	for i := from; i < to && i+1 < len(trace); i++ {
		total += haversineMeters(trace[i], trace[i+1])
	}
	return total
}

// boundingBox returns a lat/lon box around trace, expanded by marginMeters.
func boundingBox(trace []LatLon, marginMeters float64) (minLat, maxLat, minLon, maxLon float64) {
	minLat, maxLat = trace[0].Lat, trace[0].Lat
	minLon, maxLon = trace[0].Lon, trace[0].Lon
	for _, p := range trace[1:] {
		if p.Lat < minLat {
			minLat = p.Lat
		}
		if p.Lat > maxLat {
			maxLat = p.Lat
		}
		if p.Lon < minLon {
			minLon = p.Lon
		}
		if p.Lon > maxLon {
			maxLon = p.Lon
		}
	}
	// ~111km per degree of latitude; longitude degrees shrink with cos(lat), but a fixed
	// generous margin in degrees is simpler and safe (only used to cheaply discard far stations).
	marginDeg := marginMeters / 111000.0
	return minLat - marginDeg, maxLat + marginDeg, minLon - marginDeg, maxLon + marginDeg
}

// Segment is a detected toll crossing: either a closed-network entry/exit pair, a single open
// barrier, or an incomplete closed-network entry with no matching exit found on this trace.
type Segment struct {
	Network        string // OpenTollData network_name (closed networks only), empty for open barriers
	Operator       string
	Type           string // "open" or "close"
	Entry          string
	Exit           *string
	EstimatedPrice *money.Cents // class 1 (light vehicle) estimate, nil if unavailable
}

// BuildSegments groups chronologically-ordered matched stations into toll segments and prices
// each one against the dataset (class 1 / light vehicle only).
func (d *Dataset) BuildSegments(matches []MatchedStation) []Segment {
	matches = withoutDriveBys(matches)
	var segments []Segment

	i := 0
	for i < len(matches) {
		st := matches[i].Station

		if st.Type != "close" {
			seg := Segment{Type: st.Type, Operator: st.Operator, Entry: st.Name}
			d.priceSegment(&seg)
			segments = append(segments, seg)
			i++
			continue
		}

		network := d.NetworkByStation[st.Name]
		seen := map[string]bool{st.Name: true}
		j := i
		for j+1 < len(matches) {
			next := matches[j+1].Station
			if next.Type != "close" || network == "" || d.NetworkByStation[next.Name] != network || seen[next.Name] {
				break // different network/type, or a station already seen in this run (exit + re-entry)
			}
			seen[next.Name] = true
			j++
		}

		segments = append(segments, d.closedRunSegments(network, matches[i:j+1])...)
		i = j + 1
	}

	return segments
}

// withoutDriveBys drops the closed-network stations the car drove past at motorway speed.
func withoutDriveBys(matches []MatchedStation) []MatchedStation {
	kept := make([]MatchedStation, 0, len(matches))
	for _, m := range matches {
		if m.Station.Type == "close" && m.MinSpeedKmh != nil && *m.MinSpeedKmh > MaxGateSpeedKmh {
			continue
		}
		kept = append(kept, m)
	}
	return kept
}

// closedRunSegments turns a run of consecutive stations of one closed network into segments.
// Without speeds, a station on the run may have been driven past rather than crossed, so the
// run is a single ticket from its first to its last station. With speeds, every station left
// was crossed, so the run alternates entries and exits (see pairPassages).
func (d *Dataset) closedRunSegments(network string, run []MatchedStation) []Segment {
	if run[0].MinSpeedKmh == nil {
		var exit *Station
		if len(run) > 1 {
			exit = &run[len(run)-1].Station
		}
		return []Segment{d.closedSegment(network, run[0].Station, exit)}
	}
	return d.pairPassages(network, groupPassages(run))
}

// groupPassages groups stations matched within MinRevisitGapMeters of each other along the
// trace into one passage: co-located gates (a barrier and its slip road) crossed at once.
func groupPassages(run []MatchedStation) [][]Station {
	var passages [][]Station
	lastMeters := 0.0
	for k, m := range run {
		if k > 0 && m.TraceMeters-lastMeters < MinRevisitGapMeters {
			passages[len(passages)-1] = append(passages[len(passages)-1], m.Station)
		} else {
			passages = append(passages, []Station{m.Station})
		}
		lastMeters = m.TraceMeters
	}
	return passages
}

// Costs of the ways pairPassages can read a run of crossed passages, lowest wins.
const (
	costUnpriced = 2 // an entry/exit pair missing from the price table, or an entry with no exit
	costChain    = 1 // a passage read as the exit of one ticket and the entry of the next (a mainline barrier)
)

// pairPassages reads crossed passages as entry, exit, entry, exit... (leaving the motorway and
// coming back on it later is two tickets), choosing, when the price table says so, to read a
// passage as both an exit and the next entry (a barrier in the middle of the network) and
// which gate of a co-located passage was used.
func (d *Dataset) pairPassages(network string, passages [][]Station) []Segment {
	n := len(passages)
	type step struct {
		cost  int
		next  int // passage opening the next ticket
		entry Station
		exit  *Station
	}
	best := make([]step, n+1)
	for i := n - 1; i >= 0; i-- {
		if i == n-1 {
			best[i] = step{cost: costUnpriced, next: n, entry: passages[i][0]}
			continue
		}
		entry, exit, cost := d.bestPair(passages[i], passages[i+1])
		best[i] = step{cost: cost + best[i+2].cost, next: i + 2, entry: entry, exit: &exit}
		if i+1 < n-1 && cost+costChain+best[i+1].cost < best[i].cost {
			best[i] = step{cost: cost + costChain + best[i+1].cost, next: i + 1, entry: entry, exit: &exit}
		}
	}

	var segments []Segment
	for i := 0; i < n; i = best[i].next {
		segments = append(segments, d.closedSegment(network, best[i].entry, best[i].exit))
	}
	return segments
}

// bestPair picks the gates of two passages forming a priced ticket, else their first and last.
func (d *Dataset) bestPair(from, to []Station) (Station, Station, int) {
	for _, entry := range from {
		for _, exit := range to {
			if _, ok := d.ClosedPrice[entry.Name][exit.Name]; ok {
				return entry, exit, 0
			}
		}
	}
	return from[0], to[len(to)-1], costUnpriced
}

// closedSegment builds and prices a closed-network ticket, exit nil when none was found.
func (d *Dataset) closedSegment(network string, entry Station, exit *Station) Segment {
	seg := Segment{Network: network, Operator: entry.Operator, Type: "close", Entry: entry.Name}
	if exit != nil {
		name := exit.Name
		seg.Exit = &name
	}
	d.priceSegment(&seg)
	return seg
}

// priceSegment fills seg.EstimatedPrice from the dataset's class 1 price tables, if available.
func (d *Dataset) priceSegment(seg *Segment) {
	if seg.Type != "close" {
		if p, ok := d.OpenPrice[seg.Entry]; ok {
			seg.EstimatedPrice = &p
		}
		return
	}
	if seg.Exit == nil {
		return
	}
	if exits, ok := d.ClosedPrice[seg.Entry]; ok {
		if p, ok := exits[*seg.Exit]; ok {
			seg.EstimatedPrice = &p
		}
	}
}
