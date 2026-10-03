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

const tariffPlanColumns = `id, user_id, name, plan_type, currency, flat_rate_cents,
	peak_rate_cents, offpeak_rate_cents, time_windows, bands, rules, default_band,
	standing_charge_cents, to_char(valid_from, 'YYYY-MM-DD'), to_char(valid_to, 'YYYY-MM-DD'),
	is_default, created_at, updated_at`

func scanTariffPlan(row pgx.Row) (*models.TariffPlan, error) {
	var p models.TariffPlan
	var windowsJSON, bandsJSON, rulesJSON []byte
	if err := row.Scan(
		&p.ID, &p.UserID, &p.Name, &p.PlanType, &p.Currency, &p.FlatRateCents,
		&p.PeakRateCents, &p.OffpeakRateCents, &windowsJSON, &bandsJSON, &rulesJSON, &p.DefaultBand,
		&p.StandingChargeCents, &p.ValidFrom, &p.ValidTo,
		&p.IsDefault, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(windowsJSON, &p.TimeWindows)
	_ = json.Unmarshal(bandsJSON, &p.Bands)
	_ = json.Unmarshal(rulesJSON, &p.Rules)
	if p.TimeWindows == nil {
		p.TimeWindows = []models.TimeWindow{}
	}
	if p.Bands == nil {
		p.Bands = []models.TariffBand{}
	}
	if p.Rules == nil {
		p.Rules = []models.TariffRule{}
	}
	return &p, nil
}

func marshalTariffJSON(p *models.TariffPlan) (windows, bands, rules []byte) {
	encode := func(v any) []byte {
		out, err := json.Marshal(v)
		if err != nil {
			return []byte("[]")
		}
		return out
	}
	return encode(p.TimeWindows), encode(p.Bands), encode(p.Rules)
}

func (r *Repository) CreateTariffPlan(ctx context.Context, p *models.TariffPlan) error {
	windowsJSON, bandsJSON, rulesJSON := marshalTariffJSON(p)

	if p.IsDefault {
		// Reset other defaults for this user
		_, _ = r.pool.Exec(ctx, `UPDATE tariff_plans SET is_default = FALSE WHERE user_id = $1;`, p.UserID)
	}

	query := `
		INSERT INTO tariff_plans (
			user_id, name, plan_type, currency, flat_rate_cents,
			peak_rate_cents, offpeak_rate_cents, time_windows, bands, rules, default_band,
			standing_charge_cents, valid_from, valid_to, is_default
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::date, $14::date, $15)
		RETURNING id, created_at, updated_at;
	`
	return r.pool.QueryRow(ctx, query,
		p.UserID, p.Name, p.PlanType, p.Currency, p.FlatRateCents,
		p.PeakRateCents, p.OffpeakRateCents, windowsJSON, bandsJSON, rulesJSON, p.DefaultBand,
		p.StandingChargeCents, p.ValidFrom, p.ValidTo, p.IsDefault,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *Repository) GetTariffPlanByID(ctx context.Context, id, userID string) (*models.TariffPlan, error) {
	p, err := scanTariffPlan(r.pool.QueryRow(ctx,
		`SELECT `+tariffPlanColumns+` FROM tariff_plans WHERE id = $1 AND user_id = $2;`, id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// GetVehicleTariffPlan returns the plan assigned to the vehicle, or the default plan of its owner.
func (r *Repository) GetVehicleTariffPlan(ctx context.Context, vehicleID string) (*models.TariffPlan, error) {
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

// GetVehicleTariffPlanAt returns the version of the vehicle's tariff that covers day (YYYY-MM-DD): the plans of the
// owner sharing the name of the assigned plan are its versions. When none covers the day, the assigned plan is returned.
func (r *Repository) GetVehicleTariffPlanAt(ctx context.Context, vehicleID, day string) (*models.TariffPlan, error) {
	plan, err := r.GetVehicleTariffPlan(ctx, vehicleID)
	if err != nil || plan == nil {
		return plan, err
	}
	version, err := scanTariffPlan(r.pool.QueryRow(ctx, `
		SELECT `+tariffPlanColumns+` FROM tariff_plans
		WHERE user_id = $1 AND lower(name) = lower($2)
		  AND (valid_from IS NULL OR valid_from <= $3::date)
		  AND (valid_to IS NULL OR valid_to >= $3::date)
		ORDER BY valid_from DESC NULLS LAST, created_at DESC
		LIMIT 1;`, plan.UserID, plan.Name, day))
	if errors.Is(err, pgx.ErrNoRows) {
		return plan, nil
	}
	if err != nil {
		return nil, err
	}
	return version, nil
}

func (r *Repository) GetDefaultTariffPlan(ctx context.Context, userID string) (*models.TariffPlan, error) {
	p, err := scanTariffPlan(r.pool.QueryRow(ctx,
		`SELECT `+tariffPlanColumns+` FROM tariff_plans WHERE user_id = $1 ORDER BY is_default DESC, created_at ASC LIMIT 1;`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *Repository) ListTariffPlans(ctx context.Context, userID string) ([]models.TariffPlan, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+tariffPlanColumns+` FROM tariff_plans WHERE user_id = $1 ORDER BY is_default DESC, name ASC, valid_from DESC NULLS LAST;`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tariff plans: %w", err)
	}
	defer rows.Close()

	list := []models.TariffPlan{}
	for rows.Next() {
		p, err := scanTariffPlan(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *p)
	}
	return list, rows.Err()
}

func (r *Repository) UpdateTariffPlan(ctx context.Context, p *models.TariffPlan) error {
	windowsJSON, bandsJSON, rulesJSON := marshalTariffJSON(p)

	if p.IsDefault {
		_, _ = r.pool.Exec(ctx, `UPDATE tariff_plans SET is_default = FALSE WHERE user_id = $1 AND id <> $2;`, p.UserID, p.ID)
	}

	query := `
		UPDATE tariff_plans
		SET name = $1, plan_type = $2, currency = $3, flat_rate_cents = $4,
		    peak_rate_cents = $5, offpeak_rate_cents = $6, time_windows = $7,
		    bands = $8, rules = $9, default_band = $10, standing_charge_cents = $11,
		    valid_from = $12::date, valid_to = $13::date,
		    is_default = $14, updated_at = NOW()
		WHERE id = $15 AND user_id = $16;
	`
	tag, err := r.pool.Exec(ctx, query,
		p.Name, p.PlanType, p.Currency, p.FlatRateCents,
		p.PeakRateCents, p.OffpeakRateCents, windowsJSON,
		bandsJSON, rulesJSON, p.DefaultBand, p.StandingChargeCents,
		p.ValidFrom, p.ValidTo,
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
