package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

func (r *Repository) ListReminderTemplates(ctx context.Context, userID string) ([]models.ReminderTemplate, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, items FROM reminder_templates WHERE user_id = $1 ORDER BY name;`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list reminder templates: %w", err)
	}
	defer rows.Close()
	list := []models.ReminderTemplate{}
	for rows.Next() {
		var t models.ReminderTemplate
		var raw []byte
		if err := rows.Scan(&t.ID, &t.Name, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &t.Items); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *Repository) GetReminderTemplate(ctx context.Context, id, userID string) (*models.ReminderTemplate, error) {
	var t models.ReminderTemplate
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT id, name, items FROM reminder_templates WHERE id::text = $1 AND user_id = $2;`, id, userID).Scan(&t.ID, &t.Name, &raw)
	if err != nil {
		return nil, ErrNotFound
	}
	if err := json.Unmarshal(raw, &t.Items); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repository) CreateReminderTemplate(ctx context.Context, t *models.ReminderTemplate) error {
	raw, err := json.Marshal(t.Items)
	if err != nil {
		return err
	}
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO reminder_templates (user_id, name, items) VALUES ($1, $2, $3) RETURNING id;
	`, t.UserID, t.Name, raw).Scan(&t.ID); err != nil {
		return fmt.Errorf("failed to create reminder template: %w", err)
	}
	return nil
}

func (r *Repository) DeleteReminderTemplate(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM reminder_templates WHERE id::text = $1 AND user_id = $2;`, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete reminder template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ReminderObservedInterval is what a reminder's completion history says about the interval the user follows.
type ReminderObservedInterval struct {
	Completions int
	AvgKm       *float64
	AvgDays     *float64
}

// ObservedReminderIntervals averages the gaps between consecutive completions of each reminder of a vehicle.
func (r *Repository) ObservedReminderIntervals(ctx context.Context, vehicleID string) (map[string]ReminderObservedInterval, error) {
	rows, err := r.pool.Query(ctx, `
		WITH gaps AS (
			SELECT reminder_id,
			       odometer - LAG(odometer) OVER w AS dkm,
			       completed_on - LAG(completed_on) OVER w AS ddays
			FROM reminder_completions
			WHERE vehicle_id = $1
			WINDOW w AS (PARTITION BY reminder_id ORDER BY completed_on, odometer)
		)
		SELECT reminder_id::text, COUNT(*) + 1,
		       AVG(dkm) FILTER (WHERE dkm > 0)::float8,
		       AVG(ddays) FILTER (WHERE ddays > 0)::float8
		FROM gaps WHERE dkm IS NOT NULL
		GROUP BY reminder_id;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]ReminderObservedInterval{}
	for rows.Next() {
		var id string
		var o ReminderObservedInterval
		if err := rows.Scan(&id, &o.Completions, &o.AvgKm, &o.AvgDays); err != nil {
			return nil, err
		}
		out[id] = o
	}
	return out, rows.Err()
}

func (r *Repository) recordReminderCompletion(ctx context.Context, vehicleID, reminderID string, on time.Time, odo float64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO reminder_completions (reminder_id, vehicle_id, completed_on, odometer) VALUES ($1::uuid, $2, $3::date, $4);
	`, reminderID, vehicleID, on, odo)
	return err
}
