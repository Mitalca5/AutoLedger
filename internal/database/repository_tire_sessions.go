package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
)

// CopyTireHistory duplicates all sessions and/or tread logs from a source tire to one or more target tires.
func (r *Repository) CopyTireHistory(ctx context.Context, vehicleID, sourceTireID string, targetTireIDs []string, copySessions, copyLogs, adaptPosition bool) error {
	if len(targetTireIDs) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	current, err := lockVehicleTires(ctx, tx, vehicleID)
	if err != nil {
		return err
	}
	source, ok := current[sourceTireID]
	if !ok {
		return ErrNotFound
	}
	_ = source

	// Fetch sessions of source tire
	type sessionRow struct {
		position           models.TirePosition
		mountedDate        time.Time
		mountedOdometer    float64
		dismountedDate     *time.Time
		dismountedOdometer *float64
		distanceKm         float64
		notes              *string
	}
	var sourceSessions []sessionRow
	if copySessions {
		rows, err := tx.Query(ctx, `
			SELECT position, mounted_date, mounted_odometer, dismounted_date, dismounted_odometer, distance_km, notes
			FROM tire_mount_sessions
			WHERE tire_id = $1 AND vehicle_id = $2
			ORDER BY mounted_date ASC;
		`, sourceTireID, vehicleID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var s sessionRow
			if err := rows.Scan(&s.position, &s.mountedDate, &s.mountedOdometer, &s.dismountedDate, &s.dismountedOdometer, &s.distanceKm, &s.notes); err != nil {
				rows.Close()
				return err
			}
			sourceSessions = append(sourceSessions, s)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}

	// Fetch logs of source tire
	type logRow struct {
		date     time.Time
		odometer float64
		depthMm  float64
		notes    *string
	}
	var sourceLogs []logRow
	if copyLogs {
		rows, err := tx.Query(ctx, `
			SELECT date, odometer, depth_mm, notes
			FROM tire_logs
			WHERE tire_id = $1
			ORDER BY date ASC;
		`, sourceTireID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var l logRow
			if err := rows.Scan(&l.date, &l.odometer, &l.depthMm, &l.notes); err != nil {
				rows.Close()
				return err
			}
			sourceLogs = append(sourceLogs, l)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}

	for _, targetID := range targetTireIDs {
		if targetID == sourceTireID {
			continue
		}
		target, ok := current[targetID]
		if !ok {
			return ErrNotFound
		}

		if copySessions {
			for _, s := range sourceSessions {
				pos := s.position
				if adaptPosition && isMountedPosition(target.CurrentPosition) {
					pos = target.CurrentPosition
				}
				if s.dismountedDate == nil {
					// The session still open is where the target tire is fitted now, and replaces its own.
					if !isMountedPosition(target.CurrentPosition) {
						continue
					}
					pos = target.CurrentPosition
					if err := replaceOpenTireSessions(ctx, tx, targetID, ""); err != nil {
						return err
					}
				}
				if _, err := tx.Exec(ctx, `
					INSERT INTO tire_mount_sessions (
						tire_id, vehicle_id, position, mounted_date, mounted_odometer,
						dismounted_date, dismounted_odometer, distance_km, notes, created_at, updated_at
					) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW());
				`, targetID, vehicleID, pos, s.mountedDate, s.mountedOdometer, s.dismountedDate, s.dismountedOdometer, s.distanceKm, s.notes); err != nil {
					return err
				}
			}
		}

		if copyLogs {
			for _, l := range sourceLogs {
				if _, err := tx.Exec(ctx, `
					INSERT INTO tire_logs (tire_id, date, odometer, depth_mm, notes, created_at)
					VALUES ($1, $2, $3, $4, $5, NOW());
				`, targetID, l.date, l.odometer, l.depthMm, l.notes); err != nil {
					return err
				}
			}
		}

		if err := recalcTireDistance(ctx, tx, targetID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) ListTireMountSessions(ctx context.Context, tireID string) ([]models.TireMountSession, error) {
	query := `
		SELECT id, tire_id, vehicle_id, position, mounted_date, mounted_odometer,
		       dismounted_date, dismounted_odometer, distance_km, notes, created_at, updated_at
		FROM tire_mount_sessions
		WHERE tire_id::text = $1
		ORDER BY mounted_date DESC;
	`
	rows, err := r.pool.Query(ctx, query, tireID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.TireMountSession
	for rows.Next() {
		var s models.TireMountSession
		if err := rows.Scan(
			&s.ID, &s.TireID, &s.VehicleID, &s.Position, &s.MountedDate, &s.MountedOdometer,
			&s.DismountedDate, &s.DismountedOdometer, &s.DistanceKm, &s.Notes, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	if list == nil {
		list = []models.TireMountSession{}
	}
	return list, rows.Err()
}

func normalizeSessionDistance(s *models.TireMountSession) error {
	if s.DismountedOdometer != nil {
		if *s.DismountedOdometer < s.MountedOdometer {
			return apierror.New("tire.dismount_odometer_lower", "The removal odometer is lower than the fitting odometer")
		}
		// Odometers win over a manual distance; a manual distance is kept when odometers are unknown.
		if *s.DismountedOdometer > s.MountedOdometer {
			s.DistanceKm = *s.DismountedOdometer - s.MountedOdometer
		}
	} else if s.DismountedDate == nil {
		s.DistanceKm = 0
	}
	if s.DistanceKm < 0 {
		return apierror.New("tire.session_distance_negative", "A session distance cannot be negative")
	}
	return nil
}

func (r *Repository) CreateTireMountSession(ctx context.Context, s *models.TireMountSession) error {
	if err := normalizeSessionDistance(s); err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := ensureTiresOwned(ctx, tx, s.VehicleID, []string{s.TireID}); err != nil {
		return ErrNotFound
	}
	if s.DismountedDate == nil {
		if err := replaceOpenTireSessions(ctx, tx, s.TireID, ""); err != nil {
			return err
		}
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO tire_mount_sessions (
			tire_id, vehicle_id, position, mounted_date, mounted_odometer,
			dismounted_date, dismounted_odometer, distance_km, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at;
	`,
		s.TireID, s.VehicleID, s.Position, s.MountedDate, s.MountedOdometer,
		s.DismountedDate, s.DismountedOdometer, s.DistanceKm, s.Notes,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return err
	}
	if err := recalcTireDistance(ctx, tx, s.TireID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) UpdateTireMountSession(ctx context.Context, s *models.TireMountSession) error {
	if err := normalizeSessionDistance(s); err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE tire_mount_sessions s
		SET position = $1, mounted_date = $2, mounted_odometer = $3,
		    dismounted_date = $4, dismounted_odometer = $5, distance_km = $6, notes = $7,
		    updated_at = NOW()
		FROM tires t
		WHERE s.id::text = $8 AND s.tire_id = t.id AND t.id::text = $9 AND t.vehicle_id = $10;
	`,
		s.Position, s.MountedDate, s.MountedOdometer,
		s.DismountedDate, s.DismountedOdometer, s.DistanceKm, s.Notes,
		s.ID, s.TireID, s.VehicleID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if s.DismountedDate == nil {
		if err := replaceOpenTireSessions(ctx, tx, s.TireID, s.ID); err != nil {
			return err
		}
	}
	if err := recalcTireDistance(ctx, tx, s.TireID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) DeleteTireMountSession(ctx context.Context, vehicleID, sessionID, tireID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		DELETE FROM tire_mount_sessions s
		USING tires t
		WHERE s.id::text = $1 AND s.tire_id = t.id AND t.id::text = $2 AND t.vehicle_id = $3;
	`, sessionID, tireID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := recalcTireDistance(ctx, tx, tireID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// replaceOpenTireSessions removes the sessions of a tire that are still open (except keepID): a tire is fitted at one place at a time,
// so a new open session replaces the previous one.
func replaceOpenTireSessions(ctx context.Context, tx pgx.Tx, tireID, keepID string) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM tire_mount_sessions
		WHERE tire_id::text = $1 AND dismounted_date IS NULL AND ($2 = '' OR id::text <> $2);
	`, tireID, keepID)
	return err
}

// recalcTireDistance derives accumulated distance (initial + closed sessions) and the active mount odometer.
func recalcTireDistance(ctx context.Context, tx pgx.Tx, tireID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE tires t
		SET accumulated_distance_km = t.initial_distance_km + COALESCE((
		        SELECT SUM(s.distance_km) FROM tire_mount_sessions s
		        WHERE s.tire_id = t.id AND s.dismounted_date IS NOT NULL
		    ), 0),
		    mounted_odometer = (
		        SELECT s.mounted_odometer FROM tire_mount_sessions s
		        WHERE s.tire_id = t.id AND s.dismounted_date IS NULL
		        ORDER BY s.mounted_date DESC LIMIT 1
		    ),
		    updated_at = NOW()
		WHERE t.id::text = $1;
	`, tireID)
	if err != nil {
		return fmt.Errorf("failed to recalculate tire distance: %w", err)
	}
	return nil
}
