package services

import (
	"context"
	"sort"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

// smoothingInputs are the vehicle records the mileage smoothing reads. The caller loads them once per computation: the
// TCO needs the same readings and fill-ups for its own distance figures.
type smoothingInputs struct {
	powertrain  string
	checkpoints []models.OdometerCheckpoint
	// fuelLogs is only read for a vehicle that refuels.
	fuelLogs []models.FuelLog
}

// loadSmoothingInputs reads the odometer readings and, for a vehicle that refuels, its fill-ups.
func (s *TCOService) loadSmoothingInputs(ctx context.Context, vehicleID, powertrain string) (smoothingInputs, error) {
	in := smoothingInputs{powertrain: powertrain}
	var err error
	if in.checkpoints, err = s.repo.ListOdometerCheckpoints(ctx, vehicleID); err != nil {
		return in, err
	}
	if models.PowertrainCanRefuel(powertrain) {
		if in.fuelLogs, err = s.repo.ListFuelLogs(ctx, vehicleID); err != nil {
			return in, err
		}
	}
	return in, nil
}

// driveDistanceIndex answers "how far was driven between two instants" over a vehicle's drives, which are listed once
// and sorted by start time instead of being queried for every odometer interval.
type driveDistanceIndex struct {
	starts []time.Time
	km     []float64
	synced []bool
}

// loadDriveDistanceIndex lists the distance of every drive that is not deleted upstream. A drive typed by hand is not
// synced: its distance counts, its energy does not.
func (s *TCOService) loadDriveDistanceIndex(ctx context.Context, vehicleID string) (*driveDistanceIndex, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT start_time, COALESCE(distance_km, 0), COALESCE(is_manual = FALSE, FALSE)
		FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL
		ORDER BY start_time;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	idx := &driveDistanceIndex{}
	for rows.Next() {
		var start time.Time
		var km float64
		var synced bool
		if err := rows.Scan(&start, &km, &synced); err != nil {
			return nil, err
		}
		idx.starts = append(idx.starts, start)
		idx.km = append(idx.km, km)
		idx.synced = append(idx.synced, synced)
	}
	return idx, rows.Err()
}

// between sums the drives starting in [from, to): every drive for the tracked distance, only the synced ones for the
// second figure. The starts are sorted, so the first drive of the interval is found by bisection and the intervals of
// one pass, which do not overlap, cost one scan of the drives in total.
func (d *driveDistanceIndex) between(from, to time.Time) (trackedKm, syncedKm float64) {
	i := sort.Search(len(d.starts), func(i int) bool { return !d.starts[i].Before(from) })
	for ; i < len(d.starts) && d.starts[i].Before(to); i++ {
		trackedKm += d.km[i]
		if d.synced[i] {
			syncedKm += d.km[i]
		}
	}
	return trackedKm, syncedKm
}

// computeMileageSmoothing interpolates missing mileage across months between known odometer checkpoints.
func (s *TCOService) computeMileageSmoothing(ctx context.Context, vehicleID string, ownership *models.VehicleOwnership, currentOdometer float64, now time.Time, in smoothingInputs) (map[string]float64, map[string]float64, error) {
	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		loc = time.UTC
	}

	checkpoints := in.checkpoints

	type odoPoint struct {
		date time.Time
		odo  float64
	}

	var points []odoPoint
	usesFuel := models.PowertrainCanRefuel(in.powertrain)
	if usesFuel {
		// Fill-ups are odometer readings of a vehicle that refuels, like manual checkpoints.
		for _, f := range in.fuelLogs {
			if f.Odometer != nil {
				points = append(points, odoPoint{date: f.Date.In(loc), odo: *f.Odometer})
			}
		}
	}
	if ownership != nil && ownership.StartOdometer != nil && *ownership.StartOdometer >= 0 {
		points = append(points, odoPoint{
			date: ownership.StartDate.In(loc),
			odo:  *ownership.StartOdometer,
		})
	}
	for _, cp := range checkpoints {
		points = append(points, odoPoint{
			date: cp.Date.In(loc),
			odo:  cp.Odometer,
		})
	}
	if len(points) == 0 {
		return nil, nil, nil
	}

	// Tracking starts with the first synchronized record: a drive typed by hand carries no energy cost.
	var firstDriveTime *time.Time
	var firstDriveOdo *float64
	_ = s.pool.QueryRow(ctx, `
		SELECT start_time, start_odometer
		FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL AND start_odometer > 0 AND is_manual = FALSE
		ORDER BY start_time ASC LIMIT 1;
	`, vehicleID).Scan(&firstDriveTime, &firstDriveOdo)

	var firstChargeTime *time.Time
	_ = s.pool.QueryRow(ctx, `
		SELECT MIN(date)
		FROM charge_logs
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL AND (cost_source = 'TESLAMATE' OR is_manual = FALSE);
	`, vehicleID).Scan(&firstChargeTime)

	var firstTrackingTime *time.Time
	if firstDriveTime != nil && firstChargeTime != nil {
		if firstChargeTime.Before(*firstDriveTime) {
			firstTrackingTime = firstChargeTime
		} else {
			firstTrackingTime = firstDriveTime
		}
	} else if firstDriveTime != nil {
		firstTrackingTime = firstDriveTime
	} else if firstChargeTime != nil {
		firstTrackingTime = firstChargeTime
	}

	if firstTrackingTime == nil {
		// No TeslaMate record: the charges typed by hand are the only energy data, so tracking starts with the first one.
		// Without this, the whole distance would be estimated on top of the energy already entered.
		_ = s.pool.QueryRow(ctx, `
			SELECT MIN(date) FROM charge_logs WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL;
		`, vehicleID).Scan(&firstTrackingTime)
	}

	if firstDriveTime != nil && firstDriveOdo != nil && *firstDriveOdo > 0 {
		hasPriorPoint := false
		for _, p := range points {
			if p.odo < *firstDriveOdo && !p.date.After(*firstDriveTime) {
				hasPriorPoint = true
				break
			}
		}
		if hasPriorPoint {
			points = append(points, odoPoint{
				date: firstDriveTime.In(loc),
				odo:  *firstDriveOdo,
			})
		}
	}
	if currentOdometer > 0 {
		points = append(points, odoPoint{
			date: now.In(loc),
			odo:  currentOdometer,
		})
	}

	if len(points) < 2 {
		return nil, nil, nil
	}

	// Sort chronologically by date
	sort.Slice(points, func(i, j int) bool {
		if points[i].date.Equal(points[j].date) {
			return points[i].odo < points[j].odo
		}
		return points[i].date.Before(points[j].date)
	})

	// Deduplicate by date (keep highest odometer on same day) and eliminate regressions
	var cleanPoints []odoPoint
	for _, p := range points {
		if len(cleanPoints) == 0 {
			cleanPoints = append(cleanPoints, p)
			continue
		}
		last := &cleanPoints[len(cleanPoints)-1]
		if last.date.Format("2006-01-02") == p.date.Format("2006-01-02") {
			if p.odo > last.odo {
				last.odo = p.odo
			}
			continue
		}
		if p.odo >= last.odo {
			cleanPoints = append(cleanPoints, p)
		}
	}

	if len(cleanPoints) < 2 {
		return nil, nil, nil
	}

	if usesFuel {
		// No TeslaMate history to precede: every interval is plain smoothing, never estimated energy.
		first := cleanPoints[0].date
		firstTrackingTime = &first
	}

	smoothedByMonth := make(map[string]float64)
	preTmByMonth := make(map[string]float64)

	drives, err := s.loadDriveDistanceIndex(ctx, vehicleID)
	if err != nil {
		return nil, nil, err
	}

	for i := 0; i < len(cleanPoints)-1; i++ {
		p1 := cleanPoints[i]
		p2 := cleanPoints[i+1]

		deltaOdo := p2.odo - p1.odo
		if deltaOdo <= 0 {
			continue
		}

		t1 := p1.date
		t2 := p2.date
		if !t2.After(t1) {
			continue
		}

		// Distance of the drives in [t1, t2]: every drive counts for the distance, only synchronized
		// ones for the energy (a manual drive's distance still needs its energy estimated before tracking).
		trackedKm, syncedKm := drives.between(t1, t2)

		if missingKm := deltaOdo - trackedKm; missingKm > 1.0 {
			intervalSmoothed, _ := allocateSmoothingForInterval(t1, t2, missingKm, firstTrackingTime)
			for month, km := range intervalSmoothed {
				smoothedByMonth[month] += km
			}
		}
		if unpricedKm := deltaOdo - syncedKm; unpricedKm > 1.0 {
			_, intervalPreTm := allocateSmoothingForInterval(t1, t2, unpricedKm, firstTrackingTime)
			for month, km := range intervalPreTm {
				preTmByMonth[month] += km
			}
		}
	}

	return smoothedByMonth, preTmByMonth, nil
}
