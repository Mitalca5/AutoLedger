package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
)

type queryExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ensureMemberPerson returns the person of a member of the vehicle, creating it from the account's name.
func ensureMemberPerson(ctx context.Context, q queryExecer, vehicleID, userID string) (string, error) {
	var id string
	err := q.QueryRow(ctx, `SELECT id FROM vehicle_people WHERE vehicle_id::text = $1 AND user_id::text = $2`, vehicleID, userID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = q.QueryRow(ctx, `
		INSERT INTO vehicle_people (vehicle_id, name, user_id)
		SELECT $1, COALESCE(NULLIF(BTRIM(u.display_name), ''), u.email), u.id FROM users u WHERE u.id::text = $2
		RETURNING id`, vehicleID, userID).Scan(&id)
	return id, err
}

// attachAccountToPerson gives an account to a person of the vehicle that has none. The account's own
// person (created when it joined) is merged into it, so the history of both stays together.
func attachAccountToPerson(ctx context.Context, q queryExecer, vehicleID, personID, userID string) error {
	var linked *string
	err := q.QueryRow(ctx, `SELECT user_id::text FROM vehicle_people WHERE id::text = $1 AND vehicle_id::text = $2`, personID, vehicleID).Scan(&linked)
	if errors.Is(err, pgx.ErrNoRows) {
		return apierror.New("person.not_found", "Driver not found")
	}
	if err != nil {
		return err
	}
	if linked != nil {
		if *linked == userID {
			return nil
		}
		return apierror.New("person.already_linked", "This person is already linked to an account")
	}
	var existing *string
	err = q.QueryRow(ctx, `SELECT id::text FROM vehicle_people WHERE vehicle_id::text = $1 AND user_id::text = $2`, vehicleID, userID).Scan(&existing)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if existing != nil {
		if _, err := q.Exec(ctx, `UPDATE drives SET driver_id = $1 WHERE driver_id::text = $2 AND vehicle_id::text = $3`, personID, *existing, vehicleID); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `UPDATE vehicles SET default_driver_id = $1 WHERE default_driver_id::text = $2 AND id::text = $3`, personID, *existing, vehicleID); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `DELETE FROM vehicle_people WHERE id::text = $1`, *existing); err != nil {
			return err
		}
	}
	_, err = q.Exec(ctx, `UPDATE vehicle_people SET user_id = $1, updated_at = NOW() WHERE id::text = $2`, userID, personID)
	return err
}

const vehiclePersonColumns = `p.id, p.vehicle_id, p.name, p.user_id::text, u.email, (v.default_driver_id = p.id), p.created_at, p.updated_at`

func scanVehiclePerson(row pgx.Row, p *models.VehiclePerson) error {
	var isDefault *bool
	if err := row.Scan(&p.ID, &p.VehicleID, &p.Name, &p.UserID, &p.UserEmail, &isDefault, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return err
	}
	p.IsDefault = isDefault != nil && *isDefault
	return nil
}

const vehiclePersonFrom = `FROM vehicle_people p JOIN vehicles v ON v.id = p.vehicle_id LEFT JOIN users u ON u.id = p.user_id`

// ListVehiclePeople returns the people of a vehicle, the default driver first.
func (r *Repository) ListVehiclePeople(ctx context.Context, vehicleID string) ([]models.VehiclePerson, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+vehiclePersonColumns+` `+vehiclePersonFrom+`
		WHERE p.vehicle_id::text = $1
		ORDER BY (v.default_driver_id = p.id) DESC NULLS LAST, p.created_at ASC, p.name ASC`, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("failed to list vehicle people: %w", err)
	}
	defer rows.Close()
	people := []models.VehiclePerson{}
	for rows.Next() {
		var p models.VehiclePerson
		if err := scanVehiclePerson(rows, &p); err != nil {
			return nil, err
		}
		people = append(people, p)
	}
	return people, rows.Err()
}

func (r *Repository) getVehiclePerson(ctx context.Context, vehicleID, personID string) (*models.VehiclePerson, error) {
	var p models.VehiclePerson
	err := scanVehiclePerson(r.pool.QueryRow(ctx, `SELECT `+vehiclePersonColumns+` `+vehiclePersonFrom+`
		WHERE p.vehicle_id::text = $1 AND p.id::text = $2`, vehicleID, personID), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func cleanPersonName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", apierror.New("person.name_required", "The name is required")
	}
	if len([]rune(name)) > 80 {
		return "", apierror.New("person.name_too_long", "The name must not exceed 80 characters")
	}
	return name, nil
}

// CreateVehiclePerson adds a person without an account to a vehicle.
func (r *Repository) CreateVehiclePerson(ctx context.Context, vehicleID, name string) (*models.VehiclePerson, error) {
	name, err := cleanPersonName(name)
	if err != nil {
		return nil, err
	}
	var id string
	if err := r.pool.QueryRow(ctx, `INSERT INTO vehicle_people (vehicle_id, name) VALUES ($1, $2) RETURNING id`, vehicleID, name).Scan(&id); err != nil {
		return nil, fmt.Errorf("failed to create person: %w", err)
	}
	return r.getVehiclePerson(ctx, vehicleID, id)
}

// RenameVehiclePerson changes the name of a person.
func (r *Repository) RenameVehiclePerson(ctx context.Context, vehicleID, personID, name string) (*models.VehiclePerson, error) {
	name, err := cleanPersonName(name)
	if err != nil {
		return nil, err
	}
	tag, err := r.pool.Exec(ctx, `UPDATE vehicle_people SET name = $1, updated_at = NOW() WHERE id::text = $2 AND vehicle_id::text = $3`, name, personID, vehicleID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.getVehiclePerson(ctx, vehicleID, personID)
}

// LinkVehiclePersonToMember gives the account of a vehicle member to a person, merging the member's own person into it.
func (r *Repository) LinkVehiclePersonToMember(ctx context.Context, vehicleID, personID, userID string) (*models.VehiclePerson, error) {
	var isMember bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vehicle_members WHERE vehicle_id::text = $1 AND user_id::text = $2)`, vehicleID, userID).Scan(&isMember); err != nil {
		return nil, err
	}
	if !isMember {
		return nil, apierror.New("person.link_requires_member", "Invite this account to the vehicle first")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := attachAccountToPerson(ctx, tx, vehicleID, personID, userID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.getVehiclePerson(ctx, vehicleID, personID)
}

// DeleteVehiclePerson removes a person: their drives become unassigned (so they count for the default driver).
// The default driver itself cannot be removed.
func (r *Repository) DeleteVehiclePerson(ctx context.Context, vehicleID, personID string) error {
	p, err := r.getVehiclePerson(ctx, vehicleID, personID)
	if err != nil {
		return err
	}
	if p.IsDefault {
		return apierror.New("person.default_delete", "The default driver cannot be removed: choose another one first")
	}
	if p.UserID != nil {
		return apierror.New("person.linked_delete", "A person linked to an account cannot be removed: remove the member's access instead")
	}
	_, err = r.pool.Exec(ctx, `DELETE FROM vehicle_people WHERE id::text = $1 AND vehicle_id::text = $2`, personID, vehicleID)
	return err
}

// SetVehicleDefaultDriver chooses the person credited with drives that have no driver of their own.
func (r *Repository) SetVehicleDefaultDriver(ctx context.Context, vehicleID, personID string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE vehicles SET default_driver_id = p.id, updated_at = NOW()
		FROM vehicle_people p
		WHERE vehicles.id::text = $1 AND p.id::text = $2 AND p.vehicle_id = vehicles.id`, vehicleID, personID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// VehiclePersonExists reports whether the person belongs to the vehicle.
func (r *Repository) VehiclePersonExists(ctx context.Context, vehicleID, personID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vehicle_people WHERE id::text = $1 AND vehicle_id::text = $2)`, personID, vehicleID).Scan(&ok)
	return ok, err
}
