package services

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

// computeMonthlyTireAmortization prorates each tire's purchase price across the months it was
// actually driven on, mirroring the lifetime formula used for TiresAmortizedCost (km used /
// estimated lifespan) but as a month-by-month delta of the cumulative amortized amount.
// Kilometers from TeslaMate drives as well as manual odometer checkpoints (smoothedByMonth)
// and session distances are attributed to mounted tires.
// For disposed tires with recorded distance, the purchase price is smoothly prorated across their
// actual operational months without artificial spikes at disposal.
func (s *TCOService) computeMonthlyTireAmortization(ctx context.Context, vehicleID string, now time.Time, smoothedByMonth map[string]float64) (map[string]money.Cents, error) {
	type tireInfo struct {
		purchasePrice     money.Cents
		lifespanKm        float64
		disposed          bool
		lastDismountMonth string
		purchaseMonth     string
		neverUsed         bool
	}
	tires := make(map[string]*tireInfo)

	rows, err := s.pool.Query(ctx, `
		SELECT id::text, purchase_price, GREATEST(estimated_lifespan_km - initial_distance_km, 1),
		       (current_position = 'DISPOSED' OR is_archived),
		       TO_CHAR(purchase_date AT TIME ZONE $2, 'YYYY-MM'),
		       (current_position NOT IN ('FL', 'FR', 'RL', 'RR') AND accumulated_distance_km - initial_distance_km <= 0
		        AND NOT EXISTS (SELECT 1 FROM tire_mount_sessions s WHERE s.tire_id = tires.id))
		FROM tires
		WHERE vehicle_id = $1 AND purchase_price > 0;
	`, vehicleID, s.timezone)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		info := &tireInfo{}
		if err := rows.Scan(&id, &info.purchasePrice, &info.lifespanKm, &info.disposed, &info.purchaseMonth, &info.neverUsed); err != nil {
			rows.Close()
			return nil, err
		}
		tires[id] = info
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(tires) == 0 {
		return nil, nil
	}

	kmByTireMonth := make(map[string]map[string]float64)
	months := map[string]bool{}

	// 1. Km driven by the vehicle in TeslaMate drives while each tire was mounted
	kmRows, err := s.pool.Query(ctx, `
		SELECT s.tire_id::text, TO_CHAR(d.start_time AT TIME ZONE $3, 'YYYY-MM') AS m, SUM(d.distance_km)
		FROM tire_mount_sessions s
		JOIN drives d ON d.vehicle_id = s.vehicle_id
		    AND d.deleted_upstream_at IS NULL
		    AND d.start_time >= s.mounted_date
		    AND d.start_time < COALESCE(s.dismounted_date, $2)
		WHERE s.vehicle_id = $1
		GROUP BY s.tire_id, m;
	`, vehicleID, now, s.timezone)
	if err != nil {
		return nil, err
	}
	for kmRows.Next() {
		var tireID, month string
		var km float64
		if err := kmRows.Scan(&tireID, &month, &km); err != nil {
			kmRows.Close()
			return nil, err
		}
		if kmByTireMonth[tireID] == nil {
			kmByTireMonth[tireID] = map[string]float64{}
		}
		kmByTireMonth[tireID][month] += km
		months[month] = true
	}
	kmRows.Close()
	if err := kmRows.Err(); err != nil {
		return nil, err
	}

	// 2. Fetch all mount sessions to incorporate smoothed mileage and manual session distances.
	// A finished session recorded without a wheel (position STORAGE) was still driven on.
	sessionRows, err := s.pool.Query(ctx, `
		SELECT s.tire_id::text, s.mounted_date, COALESCE(s.dismounted_date, $2), s.distance_km
		FROM tire_mount_sessions s
		WHERE s.vehicle_id = $1 AND (s.position IN ('FL', 'FR', 'RL', 'RR') OR s.dismounted_date IS NOT NULL);
	`, vehicleID, now)
	if err != nil {
		return nil, err
	}

	type sessionData struct {
		tireID         string
		mountedDate    time.Time
		dismountedDate time.Time
		distanceKm     float64
	}
	var sessions []sessionData
	for sessionRows.Next() {
		var sd sessionData
		if err := sessionRows.Scan(&sd.tireID, &sd.mountedDate, &sd.dismountedDate, &sd.distanceKm); err != nil {
			sessionRows.Close()
			return nil, err
		}
		sessions = append(sessions, sd)
	}
	sessionRows.Close()
	if err := sessionRows.Err(); err != nil {
		return nil, err
	}

	// Attribute vehicle smoothed kilometers (odometer checkpoints) to mounted tires
	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		loc = time.UTC
	}
	for m, smoothedKm := range smoothedByMonth {
		if smoothedKm <= 0 {
			continue
		}
		startMonth, err := time.ParseInLocation("2006-01", m, loc)
		if err != nil {
			continue
		}
		endMonth := startMonth.AddDate(0, 1, 0)
		monthHours := endMonth.Sub(startMonth).Hours()
		if monthHours <= 0 {
			continue
		}

		for _, sess := range sessions {
			if sess.mountedDate.Before(endMonth) && sess.dismountedDate.After(startMonth) {
				overlapStart := sess.mountedDate
				if overlapStart.Before(startMonth) {
					overlapStart = startMonth
				}
				overlapEnd := sess.dismountedDate
				if overlapEnd.After(endMonth) {
					overlapEnd = endMonth
				}
				overlapHours := overlapEnd.Sub(overlapStart).Hours()
				if overlapHours > 0 {
					ratio := overlapHours / monthHours
					if ratio > 1.0 {
						ratio = 1.0
					}
					if kmByTireMonth[sess.tireID] == nil {
						kmByTireMonth[sess.tireID] = map[string]float64{}
					}
					kmByTireMonth[sess.tireID][m] += smoothedKm * ratio
					months[m] = true
				}
			}
		}
	}

	// Ensure manual session distance_km is not lost if drives/smoothing did not cover it
	for _, sess := range sessions {
		if sess.distanceKm <= 0 || !sess.dismountedDate.After(sess.mountedDate) {
			continue
		}
		// Count current km in kmByTireMonth during this session
		var curSessionKm float64
		sessMonths := allocateMissingKmByMonth(sess.mountedDate, sess.dismountedDate, sess.distanceKm)
		for sm := range sessMonths {
			curSessionKm += kmByTireMonth[sess.tireID][sm]
		}
		if sess.distanceKm > curSessionKm+1.0 {
			missing := sess.distanceKm - curSessionKm
			missingByMonth := allocateMissingKmByMonth(sess.mountedDate, sess.dismountedDate, missing)
			for sm, km := range missingByMonth {
				if kmByTireMonth[sess.tireID] == nil {
					kmByTireMonth[sess.tireID] = map[string]float64{}
				}
				kmByTireMonth[sess.tireID][sm] += km
				months[sm] = true
			}
		}
	}

	// Last dismount month per tire, to book an unmounted disposed tire's balance if it drove 0 km
	dismountRows, err := s.pool.Query(ctx, `
		SELECT tire_id::text, TO_CHAR(MAX(dismounted_date) AT TIME ZONE $2, 'YYYY-MM')
		FROM tire_mount_sessions
		WHERE vehicle_id = $1 AND dismounted_date IS NOT NULL
		GROUP BY tire_id;
	`, vehicleID, s.timezone)
	if err != nil {
		return nil, err
	}
	for dismountRows.Next() {
		var tireID, month string
		if err := dismountRows.Scan(&tireID, &month); err != nil {
			dismountRows.Close()
			return nil, err
		}
		if info, ok := tires[tireID]; ok {
			info.lastDismountMonth = month
		}
	}
	dismountRows.Close()
	if err := dismountRows.Err(); err != nil {
		return nil, err
	}

	sortedMonths := make([]string, 0, len(months))
	for m := range months {
		sortedMonths = append(sortedMonths, m)
	}
	sort.Strings(sortedMonths)

	result := make(map[string]money.Cents)
	for tireID, info := range tires {
		var totalKm float64
		for _, m := range sortedMonths {
			totalKm += kmByTireMonth[tireID][m]
		}

		// For disposed tires that have actually driven, amortize over their actual total km
		// so that their full cost is smoothed across their real service life without end-of-life spikes.
		effectiveLifespan := info.lifespanKm
		if info.disposed && totalKm > 0 {
			effectiveLifespan = totalKm
		}

		var cumKm float64
		var cumAmortized money.Cents
		for _, m := range sortedMonths {
			km := kmByTireMonth[tireID][m]
			if km <= 0 {
				continue
			}
			cumKm += km
			fraction := math.Min(1.0, cumKm/effectiveLifespan)
			newCum := money.Cents(math.Round(float64(info.purchasePrice) * fraction))
			if delta := newCum - cumAmortized; delta > 0 {
				result[m] += delta
				cumAmortized = newCum
			}
		}

		// A tire bought and paid for but never mounted has no wear to spread: it counts when it was bought,
		// until a mount session says where it was driven.
		if info.neverUsed && !info.disposed && info.purchaseMonth != "" {
			result[info.purchaseMonth] += info.purchasePrice
			continue
		}

		// If a tire was disposed with 0 km driven, book purchase price to dismount/disposal month
		if info.disposed && cumAmortized < info.purchasePrice {
			month := info.lastDismountMonth
			if month == "" {
				month = now.Format("2006-01")
			}
			result[month] += info.purchasePrice - cumAmortized
		}
	}
	return result, nil
}
