package database

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
)

const comparisonColumns = `id, user_id, vehicle_id, name, mode, annual_km, years,
	ice_fuel_type, ice_l_100km, ice_fuel_price, ice_purchase_price, ice_resale_value,
	ice_maintenance_yearly, ice_insurance_yearly, ice_tax_yearly, tracked_inputs, options, created_at, updated_at`

func scanComparison(row pgx.Row) (*models.ComparisonScenario, error) {
	var s models.ComparisonScenario
	var evJSON, optJSON []byte
	if err := row.Scan(&s.ID, &s.UserID, &s.VehicleID, &s.Name, &s.Mode, &s.AnnualKm, &s.Years,
		&s.ICE.FuelType, &s.ICE.LPer100Km, &s.ICE.FuelPrice, &s.ICE.PurchasePrice, &s.ICE.ResaleValue,
		&s.ICE.MaintenanceYearly, &s.ICE.InsuranceYearly, &s.ICE.TaxYearly, &evJSON, &optJSON,
		&s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}
	if len(evJSON) > 0 {
		s.Tracked = &models.TrackedInputs{}
		if err := json.Unmarshal(evJSON, s.Tracked); err != nil {
			return nil, err
		}
	}
	if len(optJSON) > 0 {
		if err := json.Unmarshal(optJSON, &s.Options); err != nil {
			return nil, err
		}
	}
	return &s, nil
}

// marshalScenarioJSON encodes the JSONB columns of a scenario (tracked_inputs is NULL when absent).
func marshalScenarioJSON(s *models.ComparisonScenario) (ev []byte, opts []byte, err error) {
	if s.Tracked != nil {
		if ev, err = json.Marshal(s.Tracked); err != nil {
			return nil, nil, err
		}
	}
	opts, err = json.Marshal(s.Options)
	return ev, opts, err
}

// ListComparisonScenarios lists the scenarios of a user, newest first.
func (r *Repository) ListComparisonScenarios(ctx context.Context, userID string) ([]models.ComparisonScenario, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+comparisonColumns+` FROM comparison_scenarios WHERE user_id = $1 ORDER BY created_at DESC;`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []models.ComparisonScenario{}
	for rows.Next() {
		s, err := scanComparison(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *s)
	}
	return list, rows.Err()
}

// GetComparisonScenario returns a scenario owned by the user.
func (r *Repository) GetComparisonScenario(ctx context.Context, userID, scenarioID string) (*models.ComparisonScenario, error) {
	s, err := scanComparison(r.pool.QueryRow(ctx, `SELECT `+comparisonColumns+` FROM comparison_scenarios WHERE id = $1 AND user_id = $2;`, scenarioID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// CreateComparisonScenario stores a new scenario.
func (r *Repository) CreateComparisonScenario(ctx context.Context, s *models.ComparisonScenario) error {
	ev, opts, err := marshalScenarioJSON(s)
	if err != nil {
		return err
	}
	return r.pool.QueryRow(ctx, `
		INSERT INTO comparison_scenarios (user_id, vehicle_id, name, mode, annual_km, years,
			ice_fuel_type, ice_l_100km, ice_fuel_price, ice_purchase_price, ice_resale_value,
			ice_maintenance_yearly, ice_insurance_yearly, ice_tax_yearly, tracked_inputs, options)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, created_at, updated_at;
	`, s.UserID, s.VehicleID, s.Name, s.Mode, s.AnnualKm, s.Years,
		s.ICE.FuelType, s.ICE.LPer100Km, s.ICE.FuelPrice, s.ICE.PurchasePrice, s.ICE.ResaleValue,
		s.ICE.MaintenanceYearly, s.ICE.InsuranceYearly, s.ICE.TaxYearly, ev, opts,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

// UpdateComparisonScenario replaces the editable fields of a scenario owned by the user.
func (r *Repository) UpdateComparisonScenario(ctx context.Context, s *models.ComparisonScenario) error {
	ev, opts, err := marshalScenarioJSON(s)
	if err != nil {
		return err
	}
	err = r.pool.QueryRow(ctx, `
		UPDATE comparison_scenarios
		SET vehicle_id = $3, name = $4, mode = $5, annual_km = $6, years = $7,
			ice_fuel_type = $8, ice_l_100km = $9, ice_fuel_price = $10, ice_purchase_price = $11, ice_resale_value = $12,
			ice_maintenance_yearly = $13, ice_insurance_yearly = $14, ice_tax_yearly = $15,
			tracked_inputs = $16, options = $17, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING user_id, created_at, updated_at;
	`, s.ID, s.UserID, s.VehicleID, s.Name, s.Mode, s.AnnualKm, s.Years,
		s.ICE.FuelType, s.ICE.LPer100Km, s.ICE.FuelPrice, s.ICE.PurchasePrice, s.ICE.ResaleValue,
		s.ICE.MaintenanceYearly, s.ICE.InsuranceYearly, s.ICE.TaxYearly, ev, opts,
	).Scan(&s.UserID, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// DeleteComparisonScenario deletes a scenario owned by the user.
func (r *Repository) DeleteComparisonScenario(ctx context.Context, userID, scenarioID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM comparison_scenarios WHERE id = $1 AND user_id = $2;`, scenarioID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
