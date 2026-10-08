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
func startWorkers(ctx context.Context, cfg *config.Config, repo *database.Repository, syncService *services.SyncService) {
	if syncService != nil && cfg.SyncIntervalMinutes > 0 {
		go syncService.StartBackgroundWorker(ctx, cfg.SyncIntervalMinutes)
	}

	// Every refresh adds a token row: the expired and revoked ones are purged once a day.
	if repo != nil {
		go purgeExpiredRefreshTokens(ctx, repo, 24*time.Hour)
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
