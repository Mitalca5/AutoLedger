package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
)

// Vehicles and their shared members (multi-user access roles).

func (r *Repository) CreateVehicle(ctx context.Context, v *models.Vehicle) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if v.Powertrain == "" {
		v.Powertrain = models.PowertrainEV
	}
	if v.Currency == "" {
		v.Currency = "EUR"
	}
	v.TelemetryMode = v.DerivedTelemetryMode()
	query := `
		INSERT INTO vehicles (
			user_id, name, vin, teslamate_car_id, current_odometer,
			teslamate_api_url, teslamate_auth_type, teslamate_api_key_encrypted,
			teslamate_basic_user, teslamate_basic_pass_encrypted,
			estimated_kwh_100km, estimated_price_per_kwh, currency, powertrain,
			telemetry_mode, make, model, teslamate_grafana_url
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id, created_at, updated_at;
	`
	err = tx.QueryRow(ctx, query,
		v.UserID, v.Name, v.Vin, v.TeslaMateCarID, v.CurrentOdometer,
		v.TeslaMateAPIURL, v.TeslaMateAuthType, v.TeslaMateAPIKeyEncrypted,
		v.TeslaMateBasicUser, v.TeslaMateBasicPassEnc,
		v.EstimatedKwh100km, v.EstimatedPricePerKwh, v.Currency, v.Powertrain,
		v.TelemetryMode, v.Make, v.Model, v.TeslaMateGrafanaURL,
	).Scan(&v.ID, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create vehicle: %w", err)
	}

	memberQuery := `
		INSERT INTO vehicle_members (vehicle_id, user_id, role)
		VALUES ($1, $2, 'OWNER')
		ON CONFLICT (vehicle_id, user_id) DO NOTHING;
	`
	if _, err := tx.Exec(ctx, memberQuery, v.ID, v.UserID); err != nil {
		return fmt.Errorf("failed to insert vehicle owner member: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit vehicle creation: %w", err)
	}
	v.Role = models.RoleOwner
	return nil
}

func (r *Repository) listVehicles(ctx context.Context, where string, args ...any) ([]models.Vehicle, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+vehicleColumns+` FROM vehicles WHERE `+where+` ORDER BY created_at ASC;`, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list vehicles: %w", err)
	}
	defer rows.Close()

	var list []models.Vehicle
	for rows.Next() {
		var v models.Vehicle
		if err := scanVehicle(rows, &v); err != nil {
			return nil, err
		}
		v.Role = models.RoleOwner
		list = append(list, v)
	}
	return list, rows.Err()
}

func (r *Repository) ListVehiclesByUserID(ctx context.Context, userID string) ([]models.Vehicle, error) {
	query := `
		SELECT v.id, v.user_id, v.name, v.vin, v.teslamate_car_id, v.current_odometer,
		       v.teslamate_api_url, v.teslamate_auth_type, v.teslamate_api_key_encrypted,
		       v.teslamate_basic_user, v.teslamate_basic_pass_encrypted,
		       v.estimated_kwh_100km, v.estimated_price_per_kwh, v.currency, v.powertrain,
		       v.telemetry_mode, v.make, v.model, v.teslamate_grafana_url,
		       v.created_at, v.updated_at,
		       COALESCE(vm.role, CASE WHEN v.user_id::text = $1 THEN 'OWNER' ELSE 'VIEWER' END) as role
		FROM vehicles v
		LEFT JOIN vehicle_members vm ON v.id = vm.vehicle_id AND vm.user_id::text = $1
		WHERE v.user_id::text = $1 OR vm.user_id IS NOT NULL
		ORDER BY v.created_at ASC;
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list vehicles: %w", err)
	}
	defer rows.Close()

	var list []models.Vehicle
	for rows.Next() {
		var v models.Vehicle
		var role string
		if err := rows.Scan(
			&v.ID, &v.UserID, &v.Name, &v.Vin, &v.TeslaMateCarID, &v.CurrentOdometer,
			&v.TeslaMateAPIURL, &v.TeslaMateAuthType, &v.TeslaMateAPIKeyEncrypted,
			&v.TeslaMateBasicUser, &v.TeslaMateBasicPassEnc,
			&v.EstimatedKwh100km, &v.EstimatedPricePerKwh, &v.Currency, &v.Powertrain,
			&v.TelemetryMode, &v.Make, &v.Model, &v.TeslaMateGrafanaURL,
			&v.CreatedAt, &v.UpdatedAt, &role,
		); err != nil {
			return nil, err
		}
		v.Role = models.VehicleRole(role)
		sanitizeVehicleForRole(&v)
		list = append(list, v)
	}
	return list, rows.Err()
}

func (r *Repository) ListAllVehiclesWithTeslaMate(ctx context.Context) ([]models.Vehicle, error) {
	return r.listVehicles(ctx, `teslamate_api_url IS NOT NULL AND teslamate_api_url != ''`)
}

func (r *Repository) GetVehicleByID(ctx context.Context, id, userID string) (*models.Vehicle, error) {
	var v models.Vehicle
	query := `
		SELECT v.id, v.user_id, v.name, v.vin, v.teslamate_car_id, v.current_odometer,
		       v.teslamate_api_url, v.teslamate_auth_type, v.teslamate_api_key_encrypted,
		       v.teslamate_basic_user, v.teslamate_basic_pass_encrypted,
		       v.estimated_kwh_100km, v.estimated_price_per_kwh, v.currency, v.powertrain,
		       v.telemetry_mode, v.make, v.model, v.teslamate_grafana_url,
		       v.created_at, v.updated_at,
		       COALESCE(vm.role, CASE WHEN v.user_id::text = $2 THEN 'OWNER' ELSE 'VIEWER' END) as role
		FROM vehicles v
		LEFT JOIN vehicle_members vm ON v.id = vm.vehicle_id AND vm.user_id::text = $2
		WHERE v.id::text = $1 AND (v.user_id::text = $2 OR vm.user_id IS NOT NULL);
	`
	var role string
	err := r.pool.QueryRow(ctx, query, id, userID).Scan(
		&v.ID, &v.UserID, &v.Name, &v.Vin, &v.TeslaMateCarID, &v.CurrentOdometer,
		&v.TeslaMateAPIURL, &v.TeslaMateAuthType, &v.TeslaMateAPIKeyEncrypted,
		&v.TeslaMateBasicUser, &v.TeslaMateBasicPassEnc,
		&v.EstimatedKwh100km, &v.EstimatedPricePerKwh, &v.Currency, &v.Powertrain,
		&v.TelemetryMode, &v.Make, &v.Model, &v.TeslaMateGrafanaURL,
		&v.CreatedAt, &v.UpdatedAt, &role,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get vehicle: %w", err)
	}
	v.Role = models.VehicleRole(role)
	sanitizeVehicleForRole(&v)
	return &v, nil
}

// GetVehicleByIDInternal returns vehicle without authentication or role filtering (used by background sync).
func (r *Repository) GetVehicleByIDInternal(ctx context.Context, id string) (*models.Vehicle, error) {
	var v models.Vehicle
	err := scanVehicle(r.pool.QueryRow(ctx, `SELECT `+vehicleColumns+` FROM vehicles WHERE id::text = $1;`, id), &v)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get vehicle: %w", err)
	}
	v.Role = models.RoleOwner
	return &v, nil
}

func (r *Repository) UpdateVehicle(ctx context.Context, v *models.Vehicle) error {
	query := `
		UPDATE vehicles
		SET name = $1, vin = $2, teslamate_car_id = $3, current_odometer = $4,
		    teslamate_api_url = $5, teslamate_auth_type = $6,
		    teslamate_api_key_encrypted = $7, teslamate_basic_user = $8,
		    teslamate_basic_pass_encrypted = $9,
		    estimated_kwh_100km = $10, estimated_price_per_kwh = $11,
		    powertrain = $12, telemetry_mode = $13, make = $14, model = $15,
		    teslamate_grafana_url = $16,
		    updated_at = NOW()
		WHERE id = $17;
	`
	v.TelemetryMode = v.DerivedTelemetryMode()
	tag, err := r.pool.Exec(ctx, query,
		v.Name, v.Vin, v.TeslaMateCarID, v.CurrentOdometer,
		v.TeslaMateAPIURL, v.TeslaMateAuthType, v.TeslaMateAPIKeyEncrypted,
		v.TeslaMateBasicUser, v.TeslaMateBasicPassEnc,
		v.EstimatedKwh100km, v.EstimatedPricePerKwh, v.Powertrain,
		v.TelemetryMode, v.Make, v.Model, v.TeslaMateGrafanaURL,
		v.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update vehicle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateVehicleEstimatedEnergy(ctx context.Context, vehicleID, userID string, kwh100km, eurPerKwh *float64) error {
	query := `
		UPDATE vehicles
		SET estimated_kwh_100km = $1, estimated_price_per_kwh = $2, updated_at = NOW()
		WHERE id::text = $3 AND (user_id::text = $4 OR EXISTS (
			SELECT 1 FROM vehicle_members vm WHERE vm.vehicle_id = vehicles.id AND vm.user_id::text = $4 AND vm.role IN ('OWNER', 'EDITOR')
		));
	`
	tag, err := r.pool.Exec(ctx, query, kwh100km, eurPerKwh, vehicleID, userID)
	if err != nil {
		return fmt.Errorf("failed to update estimated energy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateVehicleOdometer(ctx context.Context, vehicleID string, odometer float64) error {
	query := `
		UPDATE vehicles
		SET current_odometer = $1, updated_at = NOW()
		WHERE id = $2;
	`
	_, err := r.pool.Exec(ctx, query, odometer, vehicleID)
	return err
}

func (r *Repository) DeleteVehicle(ctx context.Context, id, userID string) error {
	query := `
		DELETE FROM vehicles 
		WHERE id::text = $1 AND (user_id::text = $2 OR EXISTS (
			SELECT 1 FROM vehicle_members vm WHERE vm.vehicle_id = vehicles.id AND vm.user_id::text = $2 AND vm.role = 'OWNER'
		));
	`
	tag, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete vehicle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListVehicleMembers returns all members who have access to the vehicle.
func (r *Repository) ListVehicleMembers(ctx context.Context, vehicleID string) ([]models.VehicleMember, error) {
	query := `
		SELECT vm.vehicle_id, vm.user_id, vm.role, u.email, u.display_name, vm.created_at, vm.updated_at
		FROM vehicle_members vm
		JOIN users u ON vm.user_id = u.id
		WHERE vm.vehicle_id::text = $1
		ORDER BY 
			CASE vm.role WHEN 'OWNER' THEN 1 WHEN 'EDITOR' THEN 2 ELSE 3 END,
			vm.created_at ASC;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("failed to list vehicle members: %w", err)
	}
	defer rows.Close()

	var members []models.VehicleMember
	for rows.Next() {
		var m models.VehicleMember
		var role string
		if err := rows.Scan(&m.VehicleID, &m.UserID, &role, &m.UserEmail, &m.DisplayName, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Role = models.VehicleRole(role)
		members = append(members, m)
	}
	return members, rows.Err()
}

// AddVehicleMember associates an existing user to a vehicle by email with a specified role.
func (r *Repository) AddVehicleMember(ctx context.Context, vehicleID, userEmail string, role models.VehicleRole) (*models.VehicleMember, error) {
	if !role.IsValid() {
		return nil, apierror.Newf("member.invalid_role", "Invalid role: %s", role)
	}
	var targetUser models.User
	err := r.pool.QueryRow(ctx, `SELECT id, email, display_name FROM users WHERE LOWER(email) = LOWER($1);`, userEmail).
		Scan(&targetUser.ID, &targetUser.Email, &targetUser.DisplayName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}

	query := `
		INSERT INTO vehicle_members (vehicle_id, user_id, role)
		VALUES ($1, $2, $3)
		RETURNING created_at, updated_at;
	`
	var createdAt, updatedAt time.Time
	err = r.pool.QueryRow(ctx, query, vehicleID, targetUser.ID, string(role)).Scan(&createdAt, &updatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "vehicle_members_pkey") {
			return nil, apierror.New("member.already_member", "This user already has access to this vehicle")
		}
		return nil, fmt.Errorf("failed to add vehicle member: %w", err)
	}

	return &models.VehicleMember{
		VehicleID:   vehicleID,
		UserID:      targetUser.ID,
		Role:        role,
		UserEmail:   targetUser.Email,
		DisplayName: targetUser.DisplayName,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// UpdateVehicleMemberRole updates the role of a vehicle member, preventing demotion of the last owner.
func (r *Repository) UpdateVehicleMemberRole(ctx context.Context, vehicleID, targetUserID string, newRole models.VehicleRole) error {
	if !newRole.IsValid() {
		return apierror.Newf("member.invalid_role", "Invalid role: %s", newRole)
	}

	var currentRole string
	err := r.pool.QueryRow(ctx, `SELECT role FROM vehicle_members WHERE vehicle_id::text = $1 AND user_id::text = $2;`, vehicleID, targetUserID).Scan(&currentRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if models.VehicleRole(currentRole) == models.RoleOwner && newRole != models.RoleOwner {
		var ownerCount int
		if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM vehicle_members WHERE vehicle_id::text = $1 AND role = 'OWNER';`, vehicleID).Scan(&ownerCount); err != nil {
			return err
		}
		if ownerCount <= 1 {
			return apierror.New("member.last_owner_demote", "The only owner of the vehicle cannot be demoted")
		}
	}

	query := `
		UPDATE vehicle_members
		SET role = $1, updated_at = NOW()
		WHERE vehicle_id::text = $2 AND user_id::text = $3;
	`
	tag, err := r.pool.Exec(ctx, query, string(newRole), vehicleID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to update vehicle member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveVehicleMember removes a member's access to a vehicle, preventing removal of the last owner.
func (r *Repository) RemoveVehicleMember(ctx context.Context, vehicleID, targetUserID string) error {
	var currentRole string
	err := r.pool.QueryRow(ctx, `SELECT role FROM vehicle_members WHERE vehicle_id::text = $1 AND user_id::text = $2;`, vehicleID, targetUserID).Scan(&currentRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if models.VehicleRole(currentRole) == models.RoleOwner {
		var ownerCount int
		if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM vehicle_members WHERE vehicle_id::text = $1 AND role = 'OWNER';`, vehicleID).Scan(&ownerCount); err != nil {
			return err
		}
		if ownerCount <= 1 {
			return apierror.New("member.last_owner_remove", "The only owner of the vehicle cannot be removed")
		}
	}

	tag, err := r.pool.Exec(ctx, `DELETE FROM vehicle_members WHERE vehicle_id::text = $1 AND user_id::text = $2;`, vehicleID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to remove vehicle member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetVehicleMemberRole returns the role of a user on a vehicle.
func (r *Repository) GetVehicleMemberRole(ctx context.Context, vehicleID, userID string) (models.VehicleRole, error) {
	var role string
	query := `
		SELECT COALESCE(vm.role, CASE WHEN v.user_id::text = $2 THEN 'OWNER' ELSE 'VIEWER' END)
		FROM vehicles v
		LEFT JOIN vehicle_members vm ON v.id = vm.vehicle_id AND vm.user_id::text = $2
		WHERE v.id::text = $1 AND (v.user_id::text = $2 OR vm.user_id IS NOT NULL);
	`
	err := r.pool.QueryRow(ctx, query, vehicleID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return models.VehicleRole(role), nil
}

// ============================================================================
// Drives & Trip Groups
// ============================================================================
