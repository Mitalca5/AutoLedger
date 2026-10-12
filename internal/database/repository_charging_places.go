package database

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/teslacost/teslacost/internal/models"
)

// ErrPlaceNameTaken is returned when the account already has a place of that name.
var ErrPlaceNameTaken = errors.New("a charging place of that name already exists")

// ErrPlaceTextEmpty is returned when the reported text to link is blank.
var ErrPlaceTextEmpty = errors.New("the reported text is empty")

// placeAliasSQL normalises a reported location the way an alias is stored: lowercase, whitespace collapsed.
func placeAliasSQL(column string) string {
	return `lower(regexp_replace(btrim(` + column + `), '\s+', ' ', 'g'))`
}

// ListChargingPlaces lists the places of an account with their aliases.
func (r *Repository) ListChargingPlaces(ctx context.Context, ownerID string) ([]models.ChargingPlace, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id::text, p.name, p.category,
		       COALESCE(array_agg(a.alias ORDER BY a.alias) FILTER (WHERE a.alias IS NOT NULL), '{}')
		FROM charging_places p
		LEFT JOIN charging_place_aliases a ON a.place_id = p.id
		WHERE p.user_id = $1
		GROUP BY p.id
		ORDER BY lower(p.name);
	`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	places := []models.ChargingPlace{}
	for rows.Next() {
		var p models.ChargingPlace
		if err := rows.Scan(&p.ID, &p.Name, &p.Category, &p.Aliases); err != nil {
			return nil, err
		}
		places = append(places, p)
	}
	return places, rows.Err()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// CreateChargingPlace adds a place to an account.
func (r *Repository) CreateChargingPlace(ctx context.Context, ownerID, name, category string) (*models.ChargingPlace, error) {
	p := models.ChargingPlace{Name: strings.TrimSpace(name), Category: category, Aliases: []string{}}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO charging_places (user_id, name, category) VALUES ($1, $2, $3) RETURNING id::text;
	`, ownerID, p.Name, p.Category).Scan(&p.ID)
	if isUniqueViolation(err) {
		return nil, ErrPlaceNameTaken
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateChargingPlace renames a place or changes its category.
func (r *Repository) UpdateChargingPlace(ctx context.Context, ownerID, placeID, name, category string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE charging_places SET name = $3, category = $4 WHERE id::text = $2 AND user_id = $1;
	`, ownerID, placeID, strings.TrimSpace(name), category)
	if isUniqueViolation(err) {
		return ErrPlaceNameTaken
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteChargingPlace removes a place: its aliases go with it and the charges that pointed at it become unclassified.
func (r *Repository) DeleteChargingPlace(ctx context.Context, ownerID, placeID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM charging_places WHERE id::text = $2 AND user_id = $1;`, ownerID, placeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ChargingPlaceOwned reports whether the place belongs to the account.
func (r *Repository) ChargingPlaceOwned(ctx context.Context, ownerID, placeID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM charging_places WHERE id::text = $2 AND user_id = $1);`, ownerID, placeID).Scan(&ok)
	return ok, err
}

// SetChargingPlaceAlias links the text charges report as their location to a place; a nil place unlinks it.
func (r *Repository) SetChargingPlaceAlias(ctx context.Context, ownerID, text string, placeID *string) error {
	if strings.TrimSpace(text) == "" {
		return ErrPlaceTextEmpty
	}
	if placeID == nil {
		_, err := r.pool.Exec(ctx, `DELETE FROM charging_place_aliases WHERE user_id = $1 AND alias = `+placeAliasSQL("$2")+`;`, ownerID, text)
		return err
	}
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO charging_place_aliases (user_id, alias, place_id)
		SELECT $1, `+placeAliasSQL("$2")+`, p.id FROM charging_places p WHERE p.id::text = $3 AND p.user_id = $1
		ON CONFLICT (user_id, alias) DO UPDATE SET place_id = EXCLUDED.place_id;
	`, ownerID, text, *placeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListChargingLocations lists the distinct location texts of a vehicle's charges with the place each one is linked to.
func (r *Repository) ListChargingLocations(ctx context.Context, vehicleID, ownerID string) ([]models.ChargingLocation, error) {
	norm := placeAliasSQL("c.address")
	rows, err := r.pool.Query(ctx, `
		SELECT MIN(btrim(c.address)), COUNT(*), COALESCE(SUM(c.kwh_added), 0)::float8, a.place_id::text
		FROM charge_logs c
		LEFT JOIN charging_place_aliases a ON a.user_id = $2 AND a.alias = `+norm+`
		WHERE c.vehicle_id = $1 AND c.deleted_upstream_at IS NULL AND btrim(COALESCE(c.address, '')) <> ''
		GROUP BY `+norm+`, a.place_id
		ORDER BY COUNT(*) DESC, 1;
	`, vehicleID, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ChargingLocation{}
	for rows.Next() {
		var l models.ChargingLocation
		if err := rows.Scan(&l.Text, &l.Sessions, &l.KwhAdded, &l.PlaceID); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
