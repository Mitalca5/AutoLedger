package services

import "math"

// TireWearComparison contrasts the observed tread-loss rate with the owner's
// declared lifespan, or the average wear rate when no lifespan was entered.
// It does not replace the blended rate used by replacement forecasts.
type TireWearComparison struct {
	MeasuredRatePer10kKm float64 `json:"measured_rate_per_10k_km"`
	ExpectedRatePer10kKm float64 `json:"expected_rate_per_10k_km"`
	MeasuredLifespanKm   float64 `json:"measured_lifespan_km"`
	ExpectedLifespanKm   float64 `json:"expected_lifespan_km"`
	DeltaPct             float64 `json:"delta_pct"`
	Reference            string  `json:"reference"` // "declared" or "average"
}

func compareMeasuredTireWear(measuredRate, usableDepth float64, declaredLifespan int) *TireWearComparison {
	if measuredRate <= 0 || usableDepth <= 0 || math.IsNaN(measuredRate) || math.IsInf(measuredRate, 0) {
		return nil
	}
	expectedRate := defaultWearRatePer10k
	reference := "average"
	if declaredLifespan > 0 {
		expectedRate = usableDepth * 10000 / float64(declaredLifespan)
		reference = "declared"
	}
	return &TireWearComparison{
		MeasuredRatePer10kKm: measuredRate,
		ExpectedRatePer10kKm: expectedRate,
		MeasuredLifespanKm:   math.Round(usableDepth / measuredRate * 10000),
		ExpectedLifespanKm:   math.Round(usableDepth / expectedRate * 10000),
		DeltaPct:             math.Round((measuredRate/expectedRate-1)*1000) / 10,
		Reference:            reference,
	}
}
