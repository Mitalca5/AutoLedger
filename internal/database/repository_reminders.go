package database

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
)

// Maintenance reminders and their per-vehicle notification webhooks.

func (r *Repository) ListMaintenanceReminders(ctx context.Context, vehicleID string, currentOdo float64) ([]models.MaintenanceReminder, error) {
	query := `
		SELECT id, vehicle_id, title, category, interval_km, interval_months, scheduled_date, repeat_yearly,
		       last_service_odometer, last_service_date, lead_km, lead_days,
		       webhook_enabled, last_notified_at, last_notified_odometer,
		       created_at, updated_at
		FROM maintenance_reminders
		WHERE vehicle_id = $1
		ORDER BY created_at ASC;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	var list []models.MaintenanceReminder
	for rows.Next() {
		var rem models.MaintenanceReminder
		if err := rows.Scan(
			&rem.ID, &rem.VehicleID, &rem.Title, &rem.Category, &rem.IntervalKm, &rem.IntervalMonths, &rem.ScheduledDate, &rem.RepeatYearly,
			&rem.LastServiceOdometer, &rem.LastServiceDate, &rem.LeadKm, &rem.LeadDays,
			&rem.WebhookEnabled, &rem.LastNotifiedAt, &rem.LastNotifiedOdometer,
			&rem.CreatedAt, &rem.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rem.ComputeStatus(currentOdo, now)
		list = append(list, rem)
	}

	// Sort by urgency: OVERDUE first, then DUE_SOON, then OK
	sort.Slice(list, func(i, j int) bool {
		priority := func(s string) int {
			switch s {
			case "OVERDUE":
				return 0
			case "DUE_SOON":
				return 1
			default:
				return 2
			}
		}
		pI := priority(list[i].Status)
		pJ := priority(list[j].Status)
		if pI != pJ {
			return pI < pJ
		}
		remI := 99999999.0
		if list[i].RemainingKm != nil {
			remI = *list[i].RemainingKm
		}
		remJ := 99999999.0
		if list[j].RemainingKm != nil {
			remJ = *list[j].RemainingKm
		}
		return remI < remJ
	})

	return list, rows.Err()
}

func (r *Repository) GetMaintenanceReminderByID(ctx context.Context, vehicleID, reminderID string, currentOdo float64) (*models.MaintenanceReminder, error) {
	query := `
		SELECT id, vehicle_id, title, category, interval_km, interval_months, scheduled_date, repeat_yearly,
		       last_service_odometer, last_service_date, lead_km, lead_days,
		       webhook_enabled, last_notified_at, last_notified_odometer,
		       created_at, updated_at
		FROM maintenance_reminders
		WHERE vehicle_id = $1 AND id::text = $2;
	`
	var rem models.MaintenanceReminder
	err := r.pool.QueryRow(ctx, query, vehicleID, reminderID).Scan(
		&rem.ID, &rem.VehicleID, &rem.Title, &rem.Category, &rem.IntervalKm, &rem.IntervalMonths, &rem.ScheduledDate, &rem.RepeatYearly,
		&rem.LastServiceOdometer, &rem.LastServiceDate, &rem.LeadKm, &rem.LeadDays,
		&rem.WebhookEnabled, &rem.LastNotifiedAt, &rem.LastNotifiedOdometer,
		&rem.CreatedAt, &rem.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rem.ComputeStatus(currentOdo, time.Now())
	return &rem, nil
}

func (r *Repository) CreateMaintenanceReminder(ctx context.Context, rem *models.MaintenanceReminder) error {
	query := `
		INSERT INTO maintenance_reminders (
			vehicle_id, title, category, interval_km, interval_months,
			last_service_odometer, last_service_date, lead_km, lead_days,
			webhook_enabled, scheduled_date, repeat_yearly
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at;
	`
	return r.pool.QueryRow(ctx, query,
		rem.VehicleID, rem.Title, rem.Category, rem.IntervalKm, rem.IntervalMonths,
		rem.LastServiceOdometer, rem.LastServiceDate, rem.LeadKm, rem.LeadDays,
		rem.WebhookEnabled, rem.ScheduledDate, rem.RepeatYearly,
	).Scan(&rem.ID, &rem.CreatedAt, &rem.UpdatedAt)
}

func (r *Repository) UpdateMaintenanceReminder(ctx context.Context, rem *models.MaintenanceReminder) error {
	query := `
		UPDATE maintenance_reminders
		SET title = $1, category = $2, interval_km = $3, interval_months = $4,
		    last_service_odometer = $5, last_service_date = $6, lead_km = $7, lead_days = $8,
		    webhook_enabled = $9, scheduled_date = $10, repeat_yearly = $11, updated_at = NOW()
		WHERE id::text = $12 AND vehicle_id = $13;
	`
	cmdTag, err := r.pool.Exec(ctx, query,
		rem.Title, rem.Category, rem.IntervalKm, rem.IntervalMonths,
		rem.LastServiceOdometer, rem.LastServiceDate, rem.LeadKm, rem.LeadDays,
		rem.WebhookEnabled, rem.ScheduledDate, rem.RepeatYearly, rem.ID, rem.VehicleID,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CompleteMaintenanceReminder(ctx context.Context, vehicleID, reminderID string, completedDate time.Time, completedOdo float64) error {
	query := `
		UPDATE maintenance_reminders
		SET last_service_date = $1, last_service_odometer = $2, last_notified_at = NULL, last_notified_odometer = NULL, updated_at = NOW()
		WHERE id::text = $3 AND vehicle_id = $4;
	`
	cmdTag, err := r.pool.Exec(ctx, query, completedDate, completedOdo, reminderID, vehicleID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return r.recordReminderCompletion(ctx, vehicleID, reminderID, completedDate, completedOdo)
}

func (r *Repository) DeleteMaintenanceReminder(ctx context.Context, vehicleID, reminderID string) error {
	query := `DELETE FROM maintenance_reminders WHERE id::text = $1 AND vehicle_id = $2;`
	cmdTag, err := r.pool.Exec(ctx, query, reminderID, vehicleID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) MarkReminderNotified(ctx context.Context, reminderID string, notifiedAt time.Time, notifiedOdo float64) error {
	query := `
		UPDATE maintenance_reminders
		SET last_notified_at = $1, last_notified_odometer = $2
		WHERE id::text = $3;
	`
	_, err := r.pool.Exec(ctx, query, notifiedAt, notifiedOdo, reminderID)
	return err
}

func (r *Repository) GetVehicleWebhook(ctx context.Context, vehicleID string) (*models.VehicleWebhook, error) {
	query := `
		SELECT id, vehicle_id, url, type, enabled, created_at, updated_at
		FROM vehicle_webhooks
		WHERE vehicle_id = $1;
	`
	var w models.VehicleWebhook
	err := r.pool.QueryRow(ctx, query, vehicleID).Scan(
		&w.ID, &w.VehicleID, &w.URL, &w.Type, &w.Enabled, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No webhook configured yet
		}
		return nil, err
	}
	return &w, nil
}

func (r *Repository) UpsertVehicleWebhook(ctx context.Context, w *models.VehicleWebhook) error {
	query := `
		INSERT INTO vehicle_webhooks (vehicle_id, url, type, enabled)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (vehicle_id) DO UPDATE
		SET url = EXCLUDED.url, type = EXCLUDED.type, enabled = EXCLUDED.enabled, updated_at = NOW()
		RETURNING id, created_at, updated_at;
	`
	return r.pool.QueryRow(ctx, query, w.VehicleID, w.URL, w.Type, w.Enabled).Scan(&w.ID, &w.CreatedAt, &w.UpdatedAt)
}

func (r *Repository) DeleteVehicleWebhook(ctx context.Context, vehicleID string) error {
	query := `DELETE FROM vehicle_webhooks WHERE vehicle_id = $1;`
	_, err := r.pool.Exec(ctx, query, vehicleID)
	return err
}
