package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/teslacost/teslacost/internal/config"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/services"
)

// startWorkers launches the background jobs; they all stop when ctx ends.
func startWorkers(ctx context.Context, cfg *config.Config, repo *database.Repository, syncService *services.SyncService, notifications *services.NotificationService) {
	if syncService != nil && cfg.SyncIntervalMinutes > 0 {
		go syncService.StartBackgroundWorker(ctx, cfg.SyncIntervalMinutes)
	}

	// Reminders are also checked on a schedule: the odometer of a Home Assistant, manual or combustion vehicle is
	// not fed by the TeslaMate sync, and a reminder reaching its due date needs no odometer change at all.
	if notifications != nil {
		go runReminderChecks(ctx, notifications, reminderCheckInterval)
	}

	// Every refresh adds a token row: the expired and revoked ones are purged once a day.
	if repo != nil {
		go purgeExpiredRefreshTokens(ctx, repo, 24*time.Hour)
	}
}

// reminderCheckInterval is how often the reminders of every vehicle with a webhook are evaluated. The 7-day
// anti-spam rule of CheckAndNotify keeps a reminder from being sent twice.
const reminderCheckInterval = time.Hour

// runReminderChecks evaluates the reminders at start-up and then every interval, until ctx ends.
func runReminderChecks(ctx context.Context, notifications *services.NotificationService, interval time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("recovered panic in the reminder check", "component", "notification", "error", r)
		}
	}()
	check := func() {
		if err := notifications.CheckAllVehicles(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("could not check the reminders", "component", "notification", "error", err)
		}
	}
	check()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}

// purgeExpiredRefreshTokens deletes the refresh tokens that can no longer be used, at start-up and then every
// interval, until ctx ends.
func purgeExpiredRefreshTokens(ctx context.Context, repo *database.Repository, interval time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("recovered panic in the token purge", "component", "auth", "error", r)
		}
	}()
	purge := func() {
		if n, err := repo.CleanupExpiredRefreshTokens(ctx); err != nil {
			slog.Warn("could not purge expired refresh tokens", "component", "auth", "error", err)
		} else if n > 0 {
			slog.Info("purged expired refresh tokens", "component", "auth", "count", n)
		}
	}
	purge()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			purge()
		}
	}
}
