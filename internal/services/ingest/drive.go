// Package ingest holds the rules every source of drives and charges shares: limits, normalization and the
// tolerances that decide whether a record already exists. It depends on nothing else in the application, so
// the manual forms, the CSV import and the integrations apply the same rules.
package ingest

import (
	"math"
	"time"
)

const (
	MaxDriveKm = 3000
	// AssumedAvgKmh gives a drive without an end time the duration it would take at this average speed.
	AssumedAvgKmh = 50.0
	// DefaultKwh100km is the consumption assumed for a drive when the vehicle has no estimate of its own.
	DefaultKwh100km = 16.0
)

// DriveInput is what a source knows about a drive; nil fields were not provided.
type DriveInput struct {
	Start           time.Time
	End             *time.Time
	DurationMin     *int
	DistanceKm      float64
	EnergyKwh       *float64
	VehicleKwh100km *float64
}

// DriveTimings is the normalized result of a DriveInput.
type DriveTimings struct {
	End         time.Time
	DurationMin int
	// EnergyKnown is false for a drive without distance and without measured energy: there is nothing to estimate from.
	EnergyKnown bool
	EnergyKwh   float64
	Kwh100km    float64
	// EnergyEstimated is true when the energy was derived from the vehicle's average consumption.
	EnergyEstimated bool
}

// ValidDriveDistance reports whether a distance in km is accepted for a drive.
func ValidDriveDistance(km float64) bool {
	return km > 0 && km <= MaxDriveKm
}

// DeriveDistance gives the distance of a drive in km: the one reported, else the difference of the two odometer
// readings. A distance of 0 means unknown (a source with no odometer). ok is false when a reported distance is
// out of range.
func DeriveDistance(reportedKm, startOdometerKm, endOdometerKm *float64) (km float64, ok bool) {
	if reportedKm != nil {
		return *reportedKm, ValidDriveDistance(*reportedKm)
	}
	if startOdometerKm != nil && endOdometerKm != nil {
		if d := *endOdometerKm - *startOdometerKm; ValidDriveDistance(d) {
			return d, true
		}
	}
	return 0, true
}

// NormalizeDrive completes a drive: end time (given end, then given duration, then the assumed average speed),
// duration in minutes, and energy (given, or estimated from the vehicle's consumption). The distance must
// already be valid, or 0 when unknown.
func NormalizeDrive(in DriveInput) DriveTimings {
	var out DriveTimings
	switch {
	case in.End != nil && in.End.After(in.Start):
		out.End = *in.End
	case in.DurationMin != nil && *in.DurationMin > 0:
		out.End = in.Start.Add(time.Duration(*in.DurationMin) * time.Minute)
	default:
		minutes := int(math.Max(1, math.Round((in.DistanceKm/AssumedAvgKmh)*60)))
		out.End = in.Start.Add(time.Duration(minutes) * time.Minute)
	}
	out.DurationMin = DurationMinutes(in.Start, out.End)

	hasEnergy := in.EnergyKwh != nil && *in.EnergyKwh > 0
	switch {
	case hasEnergy:
		out.EnergyKnown = true
		out.EnergyKwh = *in.EnergyKwh
		if in.DistanceKm > 0 {
			out.Kwh100km = Consumption100km(out.EnergyKwh, in.DistanceKm)
		}
	case in.DistanceKm > 0:
		out.EnergyKnown = true
		out.EnergyEstimated = true
		out.EnergyKwh, out.Kwh100km = EstimateDriveEnergy(in.VehicleKwh100km, in.DistanceKm)
	}
	return out
}

// DurationMinutes is the length of a drive in whole minutes, at least one.
func DurationMinutes(start, end time.Time) int {
	return int(math.Max(1, math.Round(end.Sub(start).Minutes())))
}

// Consumption100km converts an energy over a distance into kWh per 100 km.
func Consumption100km(energyKwh, distanceKm float64) float64 {
	return (energyKwh / distanceKm) * 100
}

// EstimateDriveEnergy derives a drive's energy from the vehicle's average consumption (kWh/100 km),
// falling back to DefaultKwh100km when the vehicle has none. It returns the energy and the consumption used.
func EstimateDriveEnergy(vehicleKwh100km *float64, distanceKm float64) (energyKwh, kwh100km float64) {
	kwh100km = DefaultKwh100km
	if vehicleKwh100km != nil && *vehicleKwh100km > 0 {
		kwh100km = *vehicleKwh100km
	}
	return kwh100km * distanceKm / 100, kwh100km
}

// CompleteOdometers derives the missing end of a start/end odometer pair from the distance driven.
func CompleteOdometers(start, end *float64, distanceKm float64) (*float64, *float64) {
	switch {
	case start != nil && end == nil:
		e := *start + distanceKm
		return start, &e
	case end != nil && start == nil:
		s := *end - distanceKm
		return &s, end
	}
	return start, end
}
