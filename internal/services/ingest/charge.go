package ingest

import "math"

// MaxChargeKwh is the largest energy accepted for one charging session.
const MaxChargeKwh = 1000

// ValidChargeEnergy reports whether an energy in kWh is accepted for a charging session.
func ValidChargeEnergy(kwh float64) bool {
	return !math.IsNaN(kwh) && kwh > 0 && kwh <= MaxChargeKwh
}
