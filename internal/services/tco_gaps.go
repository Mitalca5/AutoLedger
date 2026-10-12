package services

import (
	"sort"

	"github.com/teslacost/teslacost/internal/money"
)

// CompletenessGap is one thing that is missing from the TCO and what it is worth. Amount is an order of magnitude
// drawn from the vehicle's own prices, nil when it cannot be estimated.
type CompletenessGap struct {
	// Key is "depreciation", "unpriced_charges", "untracked_distance" or "unconverted_expenses".
	Key    string       `json:"key"`
	Amount *money.Cents `json:"amount"`
	// Count is the number of records behind the gap (charges, expenses).
	Count int `json:"count"`
	// Quantity is the energy (kWh) or the distance (km) without a cost.
	Quantity float64 `json:"quantity"`
	// Target is where the gap is fixed: "charges" or "expenses" for the unconverted amounts.
	Target string `json:"target,omitempty"`
}

// completenessGaps lists the gaps of the TCO, the largest first: the missing depreciation (the largest item of a
// bought vehicle, never estimated), then the estimated amounts in descending order, then the ones with no amount.
func completenessGaps(sum *TCOSummary, unconvertedCharges, unconvertedOther int) []CompletenessGap {
	comp := sum.Completeness
	gaps := []CompletenessGap{}

	if comp.AcquisitionMissing && !sum.DepreciationKnown {
		gaps = append(gaps, CompletenessGap{Key: "depreciation"})
	}
	if comp.ChargesWithoutCost > 0 {
		g := CompletenessGap{Key: "unpriced_charges", Count: comp.ChargesWithoutCost, Quantity: comp.KwhWithoutCost}
		if sum.AvgCostPerKwh > 0 && comp.KwhWithoutCost > 0 {
			a := money.FromFloat(comp.KwhWithoutCost * sum.AvgCostPerKwh)
			g.Amount = &a
		}
		gaps = append(gaps, g)
	}
	if comp.UntrackedDistanceKm > 0 && sum.EstimatedEnergyCost == 0 {
		g := CompletenessGap{Key: "untracked_distance", Quantity: comp.UntrackedDistanceKm}
		if sum.EnergyCostPerKm != nil && *sum.EnergyCostPerKm > 0 {
			a := money.FromFloat(comp.UntrackedDistanceKm * *sum.EnergyCostPerKm)
			g.Amount = &a
		}
		gaps = append(gaps, g)
	}
	if unconvertedCharges+unconvertedOther > 0 {
		gaps = append(gaps, CompletenessGap{Key: "unconverted_expenses", Count: unconvertedCharges + unconvertedOther, Target: comp.UnconvertedTarget})
	}

	rank := func(g CompletenessGap) int {
		switch {
		case g.Key == "depreciation":
			return 0
		case g.Amount != nil:
			return 1
		}
		return 2
	}
	sort.SliceStable(gaps, func(i, j int) bool {
		ri, rj := rank(gaps[i]), rank(gaps[j])
		if ri != rj {
			return ri < rj
		}
		return ri == 1 && *gaps[i].Amount > *gaps[j].Amount
	})
	return gaps
}
