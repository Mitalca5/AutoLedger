package services

import "math"

// TireWearComparison contrasts the lifespan implied by the observed tread-loss rate with the owner's declared
// lifespan, or with the average wear rate when none was entered. The delta is on the lifespan, negative when the
// tire wears out sooner than expected. It does not replace the blended rate used by replacement forecasts.
type TireWearComparison struct {
	MeasuredLifespanKm float64 `json:"measured_lifespan_km"`
	ExpectedLifespanKm float64 `json:"expected_lifespan_km"`
	DeltaPct           float64 `json:"delta_pct"`
	Reference          string  `json:"reference"` // "declared" or "average"
}

const (
	tireWearReferenceDeclared = "declared"
	tireWearReferenceAverage  = "average"
)

func compareMeasuredTireWear(measuredRate, usableDepth float64, declaredLifespan int) *TireWearComparison {
	if measuredRate <= 0 || usableDepth <= 0 || math.IsNaN(measuredRate) || math.IsInf(measuredRate, 0) {
		return nil
	}
	expectedRate := defaultWearRatePer10k
	reference := tireWearReferenceAverage
	if declaredLifespan > 0 {
		expectedRate = usableDepth * 10000 / float64(declaredLifespan)
		reference = tireWearReferenceDeclared
	}
	measured := usableDepth / measuredRate * 10000
	expected := usableDepth / expectedRate * 10000
	return &TireWearComparison{
		MeasuredLifespanKm: math.Round(measured),
		ExpectedLifespanKm: math.Round(expected),
		DeltaPct:           math.Round((measured/expected-1)*1000) / 10,
		Reference:          reference,
	}
}
