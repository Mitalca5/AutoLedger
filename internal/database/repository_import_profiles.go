package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/teslacost/teslacost/internal/models"
)

// ListImportProfiles returns the user's saved CSV mappings, by name.
func (r *Repository) ListImportProfiles(ctx context.Context, userID string) ([]models.ImportProfile, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, name, import_type, columns, date_order, decimal_separator, created_at
		FROM import_profiles WHERE user_id = $1 ORDER BY lower(name)`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list import profiles: %w", err)
	}
	defer rows.Close()
	profiles := []models.ImportProfile{}
	for rows.Next() {
		var p models.ImportProfile
		var raw []byte
		if err := rows.Scan(&p.ID, &p.Name, &p.ImportType, &raw, &p.DateOrder, &p.DecimalSeparator, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to read an import profile: %w", err)
		}
		if err := json.Unmarshal(raw, &p.Columns); err != nil {
			return nil, fmt.Errorf("failed to decode an import profile: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// SaveImportProfile creates the profile, or replaces the user's profile of the same name.
func (r *Repository) SaveImportProfile(ctx context.Context, userID string, p *models.ImportProfile) error {
	cols, err := json.Marshal(p.Columns)
	if err != nil {
		return fmt.Errorf("failed to encode the import profile: %w", err)
	}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO import_profiles (user_id, name, import_type, columns, date_order, decimal_separator)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, name) DO UPDATE
		SET import_type = EXCLUDED.import_type, columns = EXCLUDED.columns,
		    date_order = EXCLUDED.date_order, decimal_separator = EXCLUDED.decimal_separator
		RETURNING id::text, created_at`,
		userID, p.Name, p.ImportType, cols, p.DateOrder, p.DecimalSeparator).Scan(&p.ID, &p.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to save the import profile: %w", err)
	}
	return nil
}

// DeleteImportProfile removes one of the user's profiles; ErrNotFound when it is not theirs.
func (r *Repository) DeleteImportProfile(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM import_profiles WHERE id::text = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete the import profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
