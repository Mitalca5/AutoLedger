package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
)

// CreatePendingCharge stores an unassigned charge from a shared charger or webhook.
func (r *Repository) CreatePendingCharge(ctx context.Context, c *models.PendingCharge) error {
	rawJSON, err := json.Marshal(c.RawData)
	if err != nil {
		rawJSON = []byte("{}")
	}

	query := `
		INSERT INTO pending_charges (
			user_id, source, charger_name, start_time, end_time, energy_kwh, location, raw_data
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at;
	`
	return r.pool.QueryRow(ctx, query,
		c.UserID, c.Source, c.ChargerName, c.StartTime, c.EndTime,
		c.EnergyKwh, c.Location, rawJSON,
	).Scan(&c.ID, &c.CreatedAt)
}

// ListPendingCharges retrieves all unassigned charges for a user.
func (r *Repository) ListPendingCharges(ctx context.Context, userID string) ([]models.PendingCharge, error) {
	query := `
		SELECT id, user_id, source, charger_name, start_time, end_time, energy_kwh, location, raw_data, created_at
		FROM pending_charges
		WHERE user_id = $1
		ORDER BY start_time DESC;
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending charges: %w", err)
	}
	defer rows.Close()

	var list []models.PendingCharge
	for rows.Next() {
		var c models.PendingCharge
		var rawJSON []byte
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.Source, &c.ChargerName, &c.StartTime, &c.EndTime,
			&c.EnergyKwh, &c.Location, &rawJSON, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		if len(rawJSON) > 0 {
			_ = json.Unmarshal(rawJSON, &c.RawData)
		}
		list = append(list, c)
	}
	if list == nil {
		list = []models.PendingCharge{}
	}
	return list, rows.Err()
}

// GetPendingChargeByID retrieves a single pending charge.
func (r *Repository) GetPendingChargeByID(ctx context.Context, id, userID string) (*models.PendingCharge, error) {
	query := `
		SELECT id, user_id, source, charger_name, start_time, end_time, energy_kwh, location, raw_data, created_at
		FROM pending_charges
		WHERE id = $1 AND user_id = $2;
	`
	var c models.PendingCharge
	var rawJSON []byte
	err := r.pool.QueryRow(ctx, query, id, userID).Scan(
		&c.ID, &c.UserID, &c.Source, &c.ChargerName, &c.StartTime, &c.EndTime,
		&c.EnergyKwh, &c.Location, &rawJSON, &c.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(rawJSON) > 0 {
		_ = json.Unmarshal(rawJSON, &c.RawData)
	}
	return &c, nil
}

// DeletePendingCharge deletes an unassigned charge once assigned or dismissed.
func (r *Repository) DeletePendingCharge(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM pending_charges WHERE id = $1 AND user_id = $2;`, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete pending charge: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
