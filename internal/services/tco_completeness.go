package services

import (
	"math"
)

// completenessInputs gathers the ratios used by the completeness score.
type completenessInputs struct {
	kwhAdded, kwhPriced float64
	highwayDrives       int
	unqualifiedDrives   int
	trackedKm, basisKm  float64
	insurancePresent    bool
	acquisitionComplete bool
	pricedEntries       int
	unconvertedEntries  int
	drivesWithOdometer  int
	odometerAnomalies   int
	fuelOnly            bool // Combustion vehicle: energy and distance come from fuel fill-ups
	hybrid              bool // Records both charges and fill-ups
	fillUps             int
	telemetryMode       string // CONNECTED, SEMI_AUTO, MANUAL
	daysSinceOdometer   int    // Days since last odometer checkpoint
	hasCheckpoints      bool   // Vehicle has odometer checkpoints recorded
	// depreciationUnknown is true when a bought vehicle has no depreciation (price, resale value or holding period
	// missing): its full cost leaves out the largest item, so the score is capped.
	depreciationUnknown bool
}

// depreciationMissingCap is the highest score of a vehicle whose depreciation is unknown: it stays under the
// threshold from which a vehicle is ranked, whatever the other dimensions say.
const depreciationMissingCap = fleetComparableMinCompleteness - 1

// completenessHasData reports whether the vehicle has any distance or expense to judge: before that a score would
// only describe an empty account.
func completenessHasData(in completenessInputs) bool {
	return in.basisKm > 0 || in.kwhAdded > 0 || in.fillUps > 0 || in.pricedEntries+in.unconvertedEntries > 0
}

// energyScore is the share of the charged energy that has a cost; a vehicle with no charge has nothing priced.
func energyScore(in completenessInputs) float64 {
	if in.kwhAdded <= 0 {
		return 0
	}
	return ratio(in.kwhPriced, in.kwhAdded)
}

func ratio(part, total float64) float64 {
	if total <= 0 {
		return 1
	}
	return math.Max(0, math.Min(1, part/total))
}

// completion is the share of items without a problem; with no item at all there is nothing missing.
func completion(problems, total float64) float64 {
	if total <= 0 {
		return 1
	}
	return 1 - ratio(problems, total)
}

func boolScore(ok bool) float64 {
	if ok {
		return 1
	}
	return 0
}

// completenessScore weights how much of the TCO rests on complete data.
func completenessScore(in completenessInputs) (int, []CompletenessDimension) {
	distance := 0.0
	isManual := in.telemetryMode == "MANUAL" || in.telemetryMode == "SEMI_AUTO"

	if in.basisKm > 0 {
		if isManual && in.hasCheckpoints {
			// For manual tracking, regular odometer checkpoints provide distance validity.
			if in.daysSinceOdometer <= 45 {
				distance = 1.0
			} else if in.daysSinceOdometer <= 90 {
				distance = math.Max(0.5, 1.0-float64(in.daysSinceOdometer-45)/90.0)
			} else {
				distance = 0.5
			}
			if in.trackedKm > 0 {
				distance = math.Max(distance, ratio(in.trackedKm, in.basisKm))
			}
		} else {
			distance = ratio(in.trackedKm, in.basisKm)
		}
	}
	energyLabel, energyValue, energyKey := "Charges with a cost (kWh)", energyScore(in), "energy"
	distanceLabel, distanceKey := "Kilometres covered by drives", "distance"
	switch {
	case in.fuelOnly:
		energyLabel, energyValue, energyKey = "Fuel fill-ups recorded", boolScore(in.fillUps > 0), "energy_fuel"
		distanceLabel, distanceKey = "Kilometres covered by readings and fill-ups", "distance_fuel"
	case in.hybrid:
		energyLabel, energyValue, energyKey = "Charges with a cost and fill-ups recorded", hybridEnergyScore(in), "energy_hybrid"
		distanceLabel, distanceKey = "Kilometres covered by readings, drives and fill-ups", "distance_hybrid"
	case isManual:
		distanceLabel, distanceKey = "Kilometres covered by readings and drives", "distance_manual"
	}
	// A dimension with nothing to evaluate (no motorway drive to qualify, no odometer reading to compare, no priced
	// expense to convert) is not applicable: it is shown as such, never as complete.
	dims := []struct {
		key, labelKey, label string
		weight               float64
		score                float64
		neutral              bool
	}{
		{"energy", energyKey, energyLabel, 0.30, energyValue, false},
		{"distance", distanceKey, distanceLabel, 0.20, distance, false},
		{"tolls", "tolls", "Motorway drives qualified", 0.15, completion(float64(in.unqualifiedDrives), float64(in.highwayDrives)), in.highwayDrives == 0},
		{"insurance", "insurance", "Insurance entered", 0.10, boolScore(in.insurancePresent), false},
		{"acquisition", "acquisition", "Acquisition and depreciation entered", 0.10, boolScore(in.acquisitionComplete), false},
		{"odometer", "odometer", "Odometer continuity", 0.10, completion(float64(in.odometerAnomalies), float64(in.drivesWithOdometer)), in.drivesWithOdometer == 0},
		{"currency", "currency", "Expenses converted to the vehicle currency", 0.05, completion(float64(in.unconvertedEntries), float64(in.pricedEntries+in.unconvertedEntries)), in.pricedEntries+in.unconvertedEntries == 0},
	}
	// A dimension that is not applicable is left out and the others share its weight, so it neither inflates nor
	// deflates the score.
	var applicable float64
	for _, d := range dims {
		if !d.neutral {
			applicable += d.weight
		}
	}
	var total float64
	out := make([]CompletenessDimension, 0, len(dims))
	for _, d := range dims {
		weight := 0.0
		if !d.neutral {
			weight = d.weight / applicable
			total += weight * d.score
		}
		out = append(out, CompletenessDimension{Key: d.key, LabelKey: d.labelKey, Label: d.label, ScorePct: int(math.Round(d.score * 100)), Weight: weight, Applicable: !d.neutral})
	}
	score := int(math.Round(total * 100))
	if in.depreciationUnknown && score > depreciationMissingCap {
		score = depreciationMissingCap
	}
	return score, out
}

// hybridEnergyScore averages the energies the vehicle actually recorded: the priced share of its charges and
// the presence of fill-ups. A hybrid that only charges, or only refuels, is not penalised for the other energy.
func hybridEnergyScore(in completenessInputs) float64 {
	var sum float64
	var parts int
	if in.kwhAdded > 0 {
		sum += ratio(in.kwhPriced, in.kwhAdded)
		parts++
	}
	if in.fillUps > 0 {
		sum++
		parts++
	}
	if parts == 0 {
		return 0
	}
	return sum / float64(parts)
}
