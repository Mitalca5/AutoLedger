package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/servertext"
)

// SaveDriveExpense creates (exp.ID empty) or updates a drive expense in a single transaction.
// When groupDriveIDs contains several drives, the expense is attached to a trip group: the group
// previously dedicated to this expense is resynchronized, otherwise a new group named groupName is created.
func (r *Repository) SaveDriveExpense(ctx context.Context, exp *models.DriveExpense, groupDriveIDs []string, groupName string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var previousGroupID *string
	if exp.ID != "" {
		err := tx.QueryRow(ctx, `
			SELECT trip_group_id FROM drive_expenses WHERE id::text = $1 AND vehicle_id = $2 FOR UPDATE;
		`, exp.ID, exp.VehicleID).Scan(&previousGroupID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
	}

	ids := uniqueStrings(groupDriveIDs)
	switch {
	case len(ids) > 1:
		if err := ensureDrivesOwned(ctx, tx, exp.VehicleID, ids); err != nil {
			return err
		}
		groupID, err := r.reusableExpenseGroup(ctx, tx, exp, previousGroupID)
		if err != nil {
			return err
		}
		if groupID != "" {
			if _, err := tx.Exec(ctx, `DELETE FROM trip_group_drives WHERE trip_group_id = $1;`, groupID); err != nil {
				return err
			}
			if err := linkTripGroupDrives(ctx, tx, groupID, ids); err != nil {
				return err
			}
		} else {
			tg := &models.TripGroup{VehicleID: exp.VehicleID, Name: groupName, Notes: exp.Notes}
			if err := insertTripGroup(ctx, tx, tg, ids); err != nil {
				return err
			}
			groupID = tg.ID
		}
		exp.TripGroupID = &groupID
		exp.DriveID = nil
	case len(ids) == 1:
		exp.DriveID = &ids[0]
		exp.TripGroupID = nil
	}

	if exp.DriveID != nil && *exp.DriveID == "" {
		exp.DriveID = nil
	}
	if exp.TripGroupID != nil && *exp.TripGroupID == "" {
		exp.TripGroupID = nil
	}
	if exp.TripGroupID != nil {
		exp.DriveID = nil
		if err := ensureTripGroupOwned(ctx, tx, exp.VehicleID, *exp.TripGroupID); err != nil {
			return err
		}
	}
	if exp.DriveID != nil {
		if err := ensureDrivesOwned(ctx, tx, exp.VehicleID, []string{*exp.DriveID}); err != nil {
			return err
		}
	}

	if exp.Source == "" {
		exp.Source = models.ExpenseSourceManual
	}

	if exp.ID == "" {
		err = tx.QueryRow(ctx, `
			INSERT INTO drive_expenses (vehicle_id, trip_group_id, drive_id, type, amount, currency, fx_rate, date, notes, document_id, source)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id, created_at;
		`, exp.VehicleID, exp.TripGroupID, exp.DriveID, exp.Type,
			exp.Amount, exp.Currency, exp.FxRate, exp.Date, exp.Notes, exp.DocumentID, exp.Source,
		).Scan(&exp.ID, &exp.CreatedAt)
	} else {
		err = tx.QueryRow(ctx, `
			UPDATE drive_expenses
			SET trip_group_id = $1, drive_id = $2, type = $3, amount = $4,
			    currency = $5, fx_rate = $6, date = $7, notes = $8, document_id = $9, source = $10
			WHERE id::text = $11 AND vehicle_id = $12
			RETURNING created_at;
		`, exp.TripGroupID, exp.DriveID, exp.Type, exp.Amount,
			exp.Currency, exp.FxRate, exp.Date, exp.Notes, exp.DocumentID, exp.Source,
			exp.ID, exp.VehicleID,
		).Scan(&exp.CreatedAt)
	}
	if err != nil {
		return err
	}

	if exp.DocumentID != nil {
		_ = tx.QueryRow(ctx, `SELECT filename FROM expense_documents WHERE id = $1;`, *exp.DocumentID).Scan(&exp.DocumentFilename)
	}

	return tx.Commit(ctx)
}

// reusableExpenseGroup returns the expense's current trip group when no other expense or carpool uses it.
func (r *Repository) reusableExpenseGroup(ctx context.Context, tx pgx.Tx, exp *models.DriveExpense, previousGroupID *string) (string, error) {
	if previousGroupID == nil {
		return "", nil
	}
	var shared bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM drive_expenses WHERE trip_group_id = $1 AND id::text <> $2)
		    OR EXISTS(SELECT 1 FROM carpool_trips WHERE trip_group_id = $1);
	`, *previousGroupID, exp.ID).Scan(&shared)
	if err != nil {
		return "", err
	}
	if shared {
		return "", nil
	}
	return *previousGroupID, nil
}

// tripLabelWords is the fallback wording for a drive missing a reverse-geocoded address, in the
// acting user's language: the CASE below returns a display string built at read time, not a
// code, so it is translated here rather than by the frontend.
func tripLabelWords(lang string) (start, end string) {
	return servertext.Text(lang, "trip.departure"), servertext.Text(lang, "trip.arrival")
}

func (r *Repository) ListDriveExpenses(ctx context.Context, vehicleID, lang string) ([]models.DriveExpense, error) {
	start, end := tripLabelWords(lang)
	query := `
		SELECT
			e.id, e.vehicle_id, e.trip_group_id, tg.name,
			ARRAY(SELECT tgd.drive_id::text FROM trip_group_drives tgd WHERE tgd.trip_group_id = e.trip_group_id ORDER BY tgd.order_index),
			e.drive_id,
			CASE
				WHEN d.id IS NOT NULL THEN COALESCE(NULLIF(d.start_address, ''), $2) || ' → ' || COALESCE(NULLIF(d.end_address, ''), $3)
				ELSE NULL
			END,
			e.type, e.amount, e.currency, e.fx_rate, e.date, e.notes,
			e.document_id, doc.filename,
			e.source, e.created_at
		FROM drive_expenses e
		LEFT JOIN drives d ON e.drive_id = d.id AND d.vehicle_id = e.vehicle_id
		LEFT JOIN trip_groups tg ON e.trip_group_id = tg.id AND tg.vehicle_id = e.vehicle_id
		LEFT JOIN expense_documents doc ON e.document_id = doc.id
		WHERE e.vehicle_id = $1
		ORDER BY e.date DESC;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.DriveExpense
	for rows.Next() {
		var e models.DriveExpense
		if err := rows.Scan(
			&e.ID, &e.VehicleID, &e.TripGroupID, &e.TripGroupName, &e.TripGroupDriveIDs,
			&e.DriveID, &e.DriveTitle, &e.Type,
			&e.Amount, &e.Currency, &e.FxRate, &e.Date, &e.Notes,
			&e.DocumentID, &e.DocumentFilename,
			&e.Source, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

func (r *Repository) DeleteDriveExpense(ctx context.Context, vehicleID, expenseID string) error {
	query := `DELETE FROM drive_expenses WHERE id::text = $1 AND vehicle_id = $2;`
	cmdTag, err := r.pool.Exec(ctx, query, expenseID, vehicleID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
