package ingest

import "time"

// Two records of the same vehicle are the same event when they fall inside these tolerances. A source that
// gives its records an external id is matched on that id first; these only apply to records without one.
const (
	ChargeTimeWindow = 30 * time.Minute
	ChargeKwhDelta   = 0.5
	DriveTimeWindow  = 15 * time.Minute
	DriveKmDelta     = 1.0
	FuelTimeWindow   = 30 * time.Minute
	OdometerWindow   = 24 * time.Hour
	OdometerKmDelta  = 0.5
)
