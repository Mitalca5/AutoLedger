package database

import (
	"context"
	"fmt"

	"github.com/teslacost/teslacost/internal/models"
)

func (r *Repository) ListMileageRates(ctx context.Context, userID string) ([]models.MileageRate, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, label, year, from_km, to_km, rate_per_km::float8
		FROM mileage_rates WHERE user_id = $1
		ORDER BY label, year DESC, from_km;
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list mileage rates: %w", err)
	}
	defer rows.Close()
	list := []models.MileageRate{}
	for rows.Next() {
		var m models.MileageRate
		if err := rows.Scan(&m.ID, &m.Label, &m.Year, &m.FromKm, &m.ToKm, &m.RatePerKm); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *Repository) CreateMileageRate(ctx context.Context, m *models.MileageRate) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO mileage_rates (user_id, label, year, from_km, to_km, rate_per_km)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id;
	`, m.UserID, m.Label, m.Year, m.FromKm, m.ToKm, m.RatePerKm).Scan(&m.ID)
	if err != nil {
		return fmt.Errorf("failed to create mileage rate: %w", err)
	}
	return nil
}

func (r *Repository) DeleteMileageRate(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM mileage_rates WHERE id = $1 AND user_id = $2;`, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete mileage rate: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
