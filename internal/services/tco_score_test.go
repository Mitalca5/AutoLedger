package services

import "testing"

func TestCompletenessScore(t *testing.T) {
	full := completenessInputs{
		kwhAdded: 500, kwhPriced: 500, highwayDrives: 10, unqualifiedDrives: 0,
		trackedKm: 10000, basisKm: 10000, insurancePresent: true, acquisitionComplete: true,
		pricedEntries: 40, drivesWithOdometer: 200,
	}
	if score, dims := completenessScore(full); score != 100 || len(dims) != 7 {
		t.Fatalf("expected 100 %% over 7 dimensions, got %d (%d)", score, len(dims))
	}

	partial := full
	partial.kwhPriced = 250          // energy 50 % → −15 pts
	partial.unqualifiedDrives = 5    // tolls 50 % → −7.5 pts
	partial.insurancePresent = false // −10 pts
	score, dims := completenessScore(partial)
	if score != 68 {
		t.Fatalf("expected 67.5 rounded to 68, got %d", score)
	}
	if !dims[2].Applicable || dims[2].Weight != 0.15 {
		t.Fatalf("tolls apply with motorway drives and keep their weight, got %+v", dims[2])
	}
	if dims[0].Key != "energy" || dims[0].ScorePct != 50 {
		t.Fatalf("unexpected energy dimension: %+v", dims[0])
	}

	noDistance := full
	noDistance.basisKm, noDistance.trackedKm = 0, 0
	if score, _ := completenessScore(noDistance); score != 80 {
		t.Fatalf("no kilometers at all must cost the distance weight, got %d", score)
	}
}

func TestCompletenessScoreCombustionVehicle(t *testing.T) {
	in := completenessInputs{
		ice: true, iceFillUps: 5, trackedKm: 1000, basisKm: 1000, pricedEntries: 5,
		insurancePresent: true, acquisitionComplete: true,
	}
	score, dims := completenessScore(in)
	if score != 100 {
		t.Errorf("score = %d, want 100 for a fully documented combustion vehicle (dimensions %+v)", score, dims)
	}
	byKey := map[string]CompletenessDimension{}
	for _, d := range dims {
		byKey[d.Key] = d
	}
	if byKey["energy"].Label != "Fuel fill-ups recorded" || byKey["distance"].Label != "Kilometres covered by readings and fill-ups" {
		t.Errorf("combustion labels: %+v / %+v", byKey["energy"], byKey["distance"])
	}

	in.iceFillUps = 0
	if score, _ := completenessScore(in); score != 65 {
		t.Errorf("score without any fill-up = %d, want 65 (energy 0.30 of the 0.85 left without tolls)", score)
	}
}

func TestCompletenessScoreEmptyDenominators(t *testing.T) {
	// An electric vehicle with drives but no highway drive, no odometer-checked drive and no priced entry
	// has nothing to qualify, check or convert: those dimensions are complete, not empty.
	in := completenessInputs{
		kwhAdded: 100, kwhPriced: 100, trackedKm: 1000, basisKm: 1000,
		insurancePresent: true, acquisitionComplete: true,
	}
	score, dims := completenessScore(in)
	if score != 100 {
		t.Fatalf("score = %d, want 100 (dimensions %+v)", score, dims)
	}
	for _, d := range dims {
		if d.ScorePct != 100 {
			t.Errorf("dimension %s = %d%%, want 100%%", d.Key, d.ScorePct)
		}
	}

	// A real backlog still counts.
	in.highwayDrives, in.unqualifiedDrives = 4, 1
	if _, dims := completenessScore(in); dims[2].Key != "tolls" || dims[2].ScorePct != 75 {
		t.Errorf("tolls dimension = %+v, want 75%%", dims[2])
	}
}

func TestCompletenessScoreManualEV(t *testing.T) {
	// A manual EV without drives tracking, but with recent odometer checkpoints,
	// recorded charges with price, insurance and acquisition entered.
	in := completenessInputs{
		telemetryMode:       "MANUAL",
		hasCheckpoints:      true,
		daysSinceOdometer:   15,
		basisKm:             5000,
		trackedKm:           0, // No automated drives
		kwhAdded:            300,
		kwhPriced:           300,
		insurancePresent:    true,
		acquisitionComplete: true,
	}

	score, dims := completenessScore(in)
	if score != 100 {
		t.Fatalf("expected 100%% score for properly maintained manual EV, got %d%% (dims: %+v)", score, dims)
	}

	for _, d := range dims {
		if d.ScorePct != 100 {
			t.Errorf("expected dimension %s to be 100%%, got %d%%", d.Key, d.ScorePct)
		}
	}
}

func TestCompletenessScoreNeutralisesTollsWithoutMotorwayDrives(t *testing.T) {
	in := completenessInputs{
		kwhAdded: 100, kwhPriced: 100, trackedKm: 1000, basisKm: 1000,
		insurancePresent: false, acquisitionComplete: true,
	}
	score, dims := completenessScore(in)
	// 0.10 of the remaining 0.85 is missing: 1 - 0.10/0.85 = 88.2 %, not the 90 % the tolls weight would give.
	if score != 88 {
		t.Fatalf("score = %d, want 88 with the tolls weight shared by the other dimensions", score)
	}
	var sum float64
	for _, d := range dims {
		sum += d.Weight
		if d.Key == "tolls" && (d.Applicable || d.Weight != 0) {
			t.Errorf("tolls must be neutral without motorway drives, got %+v", d)
		}
	}
	if sum < 0.999 || sum > 1.001 {
		t.Errorf("weights must still add up to 1, got %v", sum)
	}

	in.highwayDrives, in.unqualifiedDrives = 2, 2
	score, _ = completenessScore(in)
	if score != 75 { // tolls 0 % (−15) and insurance missing (−10)
		t.Errorf("score with two unqualified motorway drives = %d, want 75", score)
	}
}
