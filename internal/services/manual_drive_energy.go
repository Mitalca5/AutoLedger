package services

// defaultKwh100km is the consumption assumed for a manual drive when the vehicle has no estimate of its own.
const defaultKwh100km = 16.0

// EstimateDriveEnergy derives a drive's energy from the vehicle's average consumption (kWh/100 km),
// falling back to defaultKwh100km when the vehicle has none. It returns the energy and the consumption used.
func EstimateDriveEnergy(vehicleKwh100km *float64, distanceKm float64) (energyKwh, kwh100km float64) {
	kwh100km = defaultKwh100km
	if vehicleKwh100km != nil && *vehicleKwh100km > 0 {
		kwh100km = *vehicleKwh100km
	}
	return kwh100km * distanceKm / 100, kwh100km
}
