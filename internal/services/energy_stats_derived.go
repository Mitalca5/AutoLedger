package services

import (
	"context"
	"sort"
	"time"
)

// Basis of a section of the energy statistics.
const (
	BasisMeasured    = "measured"    // Read from drives or charges that report it
	BasisDerived     = "derived"     // Computed from odometer readings and charges
	BasisUnavailable = "unavailable" // Nothing to compute it from
)

// EnergyBasis tells, per section, whether its figures are measured, derived or unavailable.
type EnergyBasis struct {
	Consumption string `json:"consumption"`
	Cost        string `json:"cost"`
	Charging    string `json:"charging"`
	FullCharge  string `json:"full_charge"`
	Temperature string `json:"temperature"`
	Battery     string `json:"battery"`
}

type odometerPoint struct {
	date time.Time
	odo  float64
}

// monthlyOdometerDistance spreads the distance between consecutive odometer readings over the calendar months in
// between, day by day. Nothing is extrapolated before the first or after the last reading, and a reading that
// goes backwards is ignored.
func monthlyOdometerDistance(points []odometerPoint, loc *time.Location) map[string]float64 {
	sort.SliceStable(points, func(i, j int) bool {
		if points[i].date.Equal(points[j].date) {
			return points[i].odo < points[j].odo
		}
		return points[i].date.Before(points[j].date)
	})
	out := map[string]float64{}
	var prev *odometerPoint
	for i := range points {
		p := points[i]
		if prev != nil && p.odo < prev.odo {
			continue
		}
		if prev != nil && p.date.After(prev.date) && p.odo > prev.odo {
			start := prev.date.In(loc)
			end := p.date.In(loc)
			days := end.Sub(start).Hours() / 24
			perDay := (p.odo - prev.odo) / days
			cursor := start
			for cursor.Before(end) {
				next := time.Date(cursor.Year(), cursor.Month()+1, 1, 0, 0, 0, 0, loc)
				if next.After(end) {
					next = end
				}
				out[cursor.Format("2006-01")] += perDay * next.Sub(cursor).Hours() / 24
				cursor = next
			}
		}
		prev = &p
	}
	return out
}

// odometerPoints lists the dated mileage readings of a vehicle: manual and integration readings and the
// odometer at the start of the ownership.
func (s *EnergyStatsService) odometerPoints(ctx context.Context, vehicleID string) ([]odometerPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT date::timestamptz, odometer::float8 FROM odometer_checkpoints WHERE vehicle_id = $1
		UNION ALL
		SELECT start_date::timestamptz, start_odometer::float8 FROM vehicle_ownership
		WHERE vehicle_id = $1 AND start_odometer IS NOT NULL;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var points []odometerPoint
	for rows.Next() {
		var p odometerPoint
		if err := rows.Scan(&p.date, &p.odo); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}
