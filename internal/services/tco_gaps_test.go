package services

import "testing"

func TestCompletenessGapsAreOrderedByWorth(t *testing.T) {
	perKm := 0.04
	sum := &TCOSummary{
		AvgCostPerKwh:   0.25,
		EnergyCostPerKm: &perKm,
		Completeness: TCOCompleteness{
			AcquisitionMissing: true, ChargesWithoutCost: 3, KwhWithoutCost: 100,
			UntrackedDistanceKm: 2000, UnconvertedTarget: "charges",
		},
	}
	gaps := completenessGaps(sum, 1, 0)
	keys := make([]string, len(gaps))
	for i, g := range gaps {
		keys[i] = g.Key
	}
	want := []string{"depreciation", "untracked_distance", "unpriced_charges", "unconverted_expenses"}
	if len(keys) != len(want) {
		t.Fatalf("gaps = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("gaps = %v, want %v (80 for the distance over 25 for the charges)", keys, want)
		}
	}
	if gaps[0].Amount != nil || gaps[3].Amount != nil {
		t.Error("depreciation and unconverted amounts are never estimated")
	}
	if got := gaps[2].Amount; got == nil || got.Float() != 25 {
		t.Errorf("unpriced charges = %v, want 100 kWh at 0.25", got)
	}
	if gaps[3].Target != "charges" {
		t.Errorf("unconverted target = %q, want charges", gaps[3].Target)
	}
}

func TestCompletenessGapsSkipWhatIsEstimatedOrKnown(t *testing.T) {
	sum := &TCOSummary{
		DepreciationKnown:   true,
		EstimatedEnergyCost: 500,
		Completeness:        TCOCompleteness{UntrackedDistanceKm: 3000},
	}
	if gaps := completenessGaps(sum, 0, 0); len(gaps) != 0 {
		t.Fatalf("untracked distance already estimated, got %+v", gaps)
	}
	sum.EstimatedEnergyCost = 0
	sum.Completeness.UnconvertedTarget = "expenses"
	gaps := completenessGaps(sum, 0, 2)
	if len(gaps) != 2 || gaps[0].Key != "untracked_distance" || gaps[0].Amount != nil || gaps[1].Target != "expenses" {
		t.Fatalf("gaps = %+v", gaps)
	}
}
