package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
)

func (r *Repository) CreateTripGroup(ctx context.Context, tg *models.TripGroup, driveIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := insertTripGroup(ctx, tx, tg, driveIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertTripGroup(ctx context.Context, tx pgx.Tx, tg *models.TripGroup, driveIDs []string) error {
	if err := ensureDrivesOwned(ctx, tx, tg.VehicleID, driveIDs); err != nil {
		return err
	}

	query := `
		INSERT INTO trip_groups (vehicle_id, name, notes)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at;
	`
	if err := tx.QueryRow(ctx, query, tg.VehicleID, tg.Name, tg.Notes).Scan(&tg.ID, &tg.CreatedAt, &tg.UpdatedAt); err != nil {
		return err
	}
	return linkTripGroupDrives(ctx, tx, tg.ID, driveIDs)
}

// linkTripGroupDrives links drives to a trip group, ordered chronologically.
func linkTripGroupDrives(ctx context.Context, tx pgx.Tx, tripGroupID string, driveIDs []string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO trip_group_drives (trip_group_id, drive_id, order_index)
		SELECT $1, d.id, ROW_NUMBER() OVER (ORDER BY d.start_time) - 1
		FROM drives d
		WHERE d.id::text = ANY($2::text[]) AND d.deleted_upstream_at IS NULL;
	`, tripGroupID, uniqueStrings(driveIDs))
	if err != nil {
		return fmt.Errorf("failed to link drives to trip group: %w", err)
	}
	return nil
}

func (r *Repository) ListTripGroups(ctx context.Context, vehicleID string) ([]models.TripGroup, error) {
	rows, err := r.pool.Query(ctx, DriveTollAllocationCTE+`
		SELECT tg.id, tg.vehicle_id, tg.name, tg.notes, tg.created_at, tg.updated_at,
		       ARRAY(SELECT tgd.drive_id::text FROM trip_group_drives tgd
		             JOIN drives d ON d.id = tgd.drive_id AND d.deleted_upstream_at IS NULL
		             WHERE tgd.trip_group_id = tg.id ORDER BY d.start_time),
		       COALESCE(stats.km, 0), stats.first_start, stats.last_end,
		       COALESCE((SELECT SUM(`+amountInVehicleCurrencyExpr("e.vehicle_id")+`) FROM drive_expenses e WHERE e.trip_group_id = tg.id), 0),
		       (SELECT COUNT(*) FROM drive_expenses e WHERE e.trip_group_id = tg.id),
		       (SELECT COUNT(*) FROM carpool_trips c WHERE c.trip_group_id = tg.id),
		       COALESCE((SELECT SUM(a.allocated) FROM allocations a
		                 WHERE a.drive_id IN (SELECT tgd.drive_id FROM trip_group_drives tgd WHERE tgd.trip_group_id = tg.id)), 0)
		FROM trip_groups tg
		LEFT JOIN LATERAL (
			SELECT SUM(d.distance_km) AS km, MIN(d.start_time) AS first_start, MAX(d.end_time) AS last_end
			FROM trip_group_drives tgd
			JOIN drives d ON d.id = tgd.drive_id AND d.deleted_upstream_at IS NULL
			WHERE tgd.trip_group_id = tg.id
		) stats ON TRUE
		WHERE tg.vehicle_id = $1
		ORDER BY COALESCE(stats.first_start, tg.created_at) DESC;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.TripGroup
	for rows.Next() {
		var tg models.TripGroup
		if err := rows.Scan(&tg.ID, &tg.VehicleID, &tg.Name, &tg.Notes, &tg.CreatedAt, &tg.UpdatedAt,
			&tg.DriveIDs, &tg.DistanceKm, &tg.StartTime, &tg.EndTime, &tg.ExpensesTotal, &tg.ExpenseCount, &tg.CarpoolCount, &tg.TollsTotal); err != nil {
			return nil, err
		}
		list = append(list, tg)
	}
	return list, rows.Err()
}

// UpdateTripGroup renames a trip group and, when driveIDs is not nil, replaces its drives.
func (r *Repository) UpdateTripGroup(ctx context.Context, tg *models.TripGroup, driveIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE trip_groups SET name = $1, notes = $2, updated_at = NOW()
		WHERE id::text = $3 AND vehicle_id = $4;
	`, tg.Name, tg.Notes, tg.ID, tg.VehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if driveIDs != nil {
		if len(uniqueStrings(driveIDs)) == 0 {
			return apierror.New("trip.needs_drive", "A trip must contain at least one drive")
		}
		if err := ensureDrivesOwned(ctx, tx, tg.VehicleID, driveIDs); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trip_group_drives WHERE trip_group_id::text = $1;`, tg.ID); err != nil {
			return err
		}
		if err := linkTripGroupDrives(ctx, tx, tg.ID, driveIDs); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// DeleteTripGroup deletes a trip group. Its expenses are kept (no longer linked to drives) unless deleteExpenses is set.
func (r *Repository) DeleteTripGroup(ctx context.Context, vehicleID, tripGroupID string, deleteExpenses bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := ensureTripGroupOwned(ctx, tx, vehicleID, tripGroupID); err != nil {
		return ErrNotFound
	}
	if deleteExpenses {
		if _, err := tx.Exec(ctx, `DELETE FROM drive_expenses WHERE trip_group_id::text = $1 AND vehicle_id = $2;`, tripGroupID, vehicleID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM trip_groups WHERE id::text = $1 AND vehicle_id = $2;`, tripGroupID, vehicleID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ============================================================================
// Drive Expenses (Tolls, Parking)
// ============================================================================
