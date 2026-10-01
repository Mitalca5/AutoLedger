package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
)

// Tariff Plans

func (r *Repository) CreateTariffPlan(ctx context.Context, p *models.TariffPlan) error {
	windowsJSON, err := json.Marshal(p.TimeWindows)
	if err != nil {
		windowsJSON = []byte("[]")
	}

	if p.IsDefault {
		// Reset other defaults for this user
		_, _ = r.pool.Exec(ctx, `UPDATE tariff_plans SET is_default = FALSE WHERE user_id = $1;`, p.UserID)
	}

	query := `
		INSERT INTO tariff_plans (
			user_id, name, plan_type, currency, flat_rate_cents,
			peak_rate_cents, offpeak_rate_cents, time_windows, is_default
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at;
	`
	return r.pool.QueryRow(ctx, query,
		p.UserID, p.Name, p.PlanType, p.Currency, p.FlatRateCents,
		p.PeakRateCents, p.OffpeakRateCents, windowsJSON, p.IsDefault,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *Repository) GetTariffPlanByID(ctx context.Context, id, userID string) (*models.TariffPlan, error) {
	query := `
		SELECT id, user_id, name, plan_type, currency, flat_rate_cents,
		       peak_rate_cents, offpeak_rate_cents, time_windows, is_default,
		       created_at, updated_at
		FROM tariff_plans
		WHERE id = $1 AND user_id = $2;
	`
	var p models.TariffPlan
	var windowsJSON []byte
	err := r.pool.QueryRow(ctx, query, id, userID).Scan(
		&p.ID, &p.UserID, &p.Name, &p.PlanType, &p.Currency, &p.FlatRateCents,
		&p.PeakRateCents, &p.OffpeakRateCents, &windowsJSON, &p.IsDefault,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(windowsJSON) > 0 {
		_ = json.Unmarshal(windowsJSON, &p.TimeWindows)
	}
	if p.TimeWindows == nil {
		p.TimeWindows = []models.TimeWindow{}
	}
	return &p, nil
}

func (r *Repository) GetVehicleTariffPlan(ctx context.Context, vehicleID string) (*models.TariffPlan, error) {
	// First check if the vehicle has an assigned tariff_plan_id
	var tariffID *string
	var userID string
	err := r.pool.QueryRow(ctx, `SELECT tariff_plan_id, user_id FROM vehicles WHERE id = $1;`, vehicleID).Scan(&tariffID, &userID)
	if err != nil {
		return nil, err
	}
	if tariffID != nil {
		p, err := r.GetTariffPlanByID(ctx, *tariffID, userID)
		if err == nil {
			return p, nil
		}
	}

	// Fallback to user default plan
	return r.GetDefaultTariffPlan(ctx, userID)
}

func (r *Repository) GetDefaultTariffPlan(ctx context.Context, userID string) (*models.TariffPlan, error) {
	query := `
		SELECT id, user_id, name, plan_type, currency, flat_rate_cents,
		       peak_rate_cents, offpeak_rate_cents, time_windows, is_default,
		       created_at, updated_at
		FROM tariff_plans
		WHERE user_id = $1
		ORDER BY is_default DESC, created_at ASC
		LIMIT 1;
	`
	var p models.TariffPlan
	var windowsJSON []byte
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&p.ID, &p.UserID, &p.Name, &p.PlanType, &p.Currency, &p.FlatRateCents,
		&p.PeakRateCents, &p.OffpeakRateCents, &windowsJSON, &p.IsDefault,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(windowsJSON) > 0 {
		_ = json.Unmarshal(windowsJSON, &p.TimeWindows)
	}
	if p.TimeWindows == nil {
		p.TimeWindows = []models.TimeWindow{}
	}
	return &p, nil
}

func (r *Repository) ListTariffPlans(ctx context.Context, userID string) ([]models.TariffPlan, error) {
	query := `
		SELECT id, user_id, name, plan_type, currency, flat_rate_cents,
		       peak_rate_cents, offpeak_rate_cents, time_windows, is_default,
		       created_at, updated_at
		FROM tariff_plans
		WHERE user_id = $1
		ORDER BY is_default DESC, name ASC;
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tariff plans: %w", err)
	}
	defer rows.Close()

	var list []models.TariffPlan
	for rows.Next() {
		var p models.TariffPlan
		var windowsJSON []byte
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Name, &p.PlanType, &p.Currency, &p.FlatRateCents,
			&p.PeakRateCents, &p.OffpeakRateCents, &windowsJSON, &p.IsDefault,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if len(windowsJSON) > 0 {
			_ = json.Unmarshal(windowsJSON, &p.TimeWindows)
		}
		if p.TimeWindows == nil {
			p.TimeWindows = []models.TimeWindow{}
		}
		list = append(list, p)
	}
	if list == nil {
		list = []models.TariffPlan{}
	}
	return list, rows.Err()
}

func (r *Repository) UpdateTariffPlan(ctx context.Context, p *models.TariffPlan) error {
	windowsJSON, err := json.Marshal(p.TimeWindows)
	if err != nil {
		windowsJSON = []byte("[]")
	}

	if p.IsDefault {
		_, _ = r.pool.Exec(ctx, `UPDATE tariff_plans SET is_default = FALSE WHERE user_id = $1 AND id <> $2;`, p.UserID, p.ID)
	}

	query := `
		UPDATE tariff_plans
		SET name = $1, plan_type = $2, currency = $3, flat_rate_cents = $4,
		    peak_rate_cents = $5, offpeak_rate_cents = $6, time_windows = $7,
		    is_default = $8, updated_at = NOW()
		WHERE id = $9 AND user_id = $10;
	`
	tag, err := r.pool.Exec(ctx, query,
		p.Name, p.PlanType, p.Currency, p.FlatRateCents,
		p.PeakRateCents, p.OffpeakRateCents, windowsJSON,
		p.IsDefault, p.ID, p.UserID,
	)
	if err != nil {
		return fmt.Errorf("failed to update tariff plan: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteTariffPlan(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tariff_plans WHERE id = $1 AND user_id = $2;`, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete tariff plan: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Public Charging Presets

func (r *Repository) CreatePublicChargingPreset(ctx context.Context, p *models.PublicChargingPreset) error {
	query := `
		INSERT INTO public_charging_presets (
			user_id, name, connection_fee_cents, price_per_kwh_cents,
			price_per_minute_cents, idle_fee_per_minute_cents, idle_grace_minutes, currency
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at;
	`
	return r.pool.QueryRow(ctx, query,
		p.UserID, p.Name, p.ConnectionFee, p.PricePerKwh,
		p.PricePerMinute, p.IdleFeePerMinute, p.IdleGraceMinutes, p.Currency,
	).Scan(&p.ID, &p.CreatedAt)
}

func (r *Repository) ListPublicChargingPresets(ctx context.Context, userID string) ([]models.PublicChargingPreset, error) {
	query := `
		SELECT id, user_id, name, connection_fee_cents, price_per_kwh_cents,
		       price_per_minute_cents, idle_fee_per_minute_cents, idle_grace_minutes,
		       currency, created_at
		FROM public_charging_presets
		WHERE user_id = $1
		ORDER BY name ASC;
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list public charging presets: %w", err)
	}
	defer rows.Close()

	var list []models.PublicChargingPreset
	for rows.Next() {
		var p models.PublicChargingPreset
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Name, &p.ConnectionFee, &p.PricePerKwh,
			&p.PricePerMinute, &p.IdleFeePerMinute, &p.IdleGraceMinutes,
			&p.Currency, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	if list == nil {
		list = []models.PublicChargingPreset{}
	}
	return list, rows.Err()
}

func (r *Repository) DeletePublicChargingPreset(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM public_charging_presets WHERE id = $1 AND user_id = $2;`, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete preset: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
