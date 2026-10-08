package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // reporting timezone available even in minimal container images

	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/config"
	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/handlers"
	appMiddleware "github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/services"
)

// AppVersion is the application version, injected at build time via -ldflags "-X main.AppVersion=...".
var AppVersion = "1.35.0"

// dbConnectTimeout is how long the server waits for PostgreSQL to become reachable on startup before giving up.
// It covers a slow crash recovery after an unclean shutdown, not just a normal container boot race.
const dbConnectTimeout = 3 * time.Minute

// insecureDefaultsError refuses a production configuration that still holds a secret published in the repository.
// Other environments only get the warnings: the defaults are convenient for local development.
func insecureDefaultsError(cfg *config.Config) error {
	warnings := cfg.InsecureDefaults()
	if len(warnings) == 0 {
		return nil
	}
	if !strings.EqualFold(cfg.Environment, "production") {
		for _, w := range warnings {
			slog.Warn("insecure default secret", "component", "security", "detail", w)
		}
		return nil
	}
	return fmt.Errorf("ENVIRONMENT=production with default secrets: %s. Generate real values (openssl rand -hex 32) "+
		"and set them in .env, or set ENVIRONMENT=development for a throwaway local instance", strings.Join(warnings, "; "))
}

func main() {
	// 1. Load configuration
	cfg := config.Load()
	configureLogging(cfg)

	slog.Info("Starting AutoLedger full-stack server...")

	// A production deployment must not run with a secret published in the repository: anyone could forge
	// sessions or decrypt the stored TeslaMate credentials.
	if err := insecureDefaultsError(cfg); err != nil {
		slog.Error("refusing to start", "component", "security", "reason", err)
		os.Exit(1)
	}

	trustedProxies, err := appMiddleware.ParseTrustedProxies(cfg.TrustedProxies)
	if err != nil {
		slog.Error("invalid TRUSTED_PROXIES", "component", "security", "error", err)
		os.Exit(1)
	}

	// 2. Initialize encryption module
	encryptor, err := crypto.NewEncryptor(cfg.AppEncryptionKey)
	if err != nil {
		slog.Error("failed to initialize crypto module", "error", err)
		os.Exit(1)
	}

	// 3. Connect to PostgreSQL. A container restarted after a host reboot can come up before PostgreSQL has
	// finished crash recovery: retry for up to dbConnectTimeout rather than fail immediately. If PostgreSQL is
	// still unreachable after that, exit non-zero so restart: unless-stopped (docker-compose.yml) retries the
	// whole startup, instead of quietly running with no database and no API routes until someone notices and
	// restarts the container by hand.
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), dbConnectTimeout)
	dbPool, err := database.Connect(connectCtx, cfg.DatabaseURL)
	cancelConnect()
	if err != nil {
		slog.Error("giving up on PostgreSQL, exiting so the container restarts and retries", "error", err, "hint", database.UnreachableHint)
		os.Exit(1)
	}
	defer dbPool.Close()
	dbPool.WarnIfServerOutdated(context.Background())

	// A fresh, short-lived context for the rest of startup: it must not inherit whatever is left of the (possibly
	// long) connection wait above.
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	if migErr := dbPool.Migrate(ctx); migErr != nil {
		slog.Error("database migration failed", "error", migErr)
		os.Exit(1)
	}
	repo := database.NewRepository(dbPool.Pool)

	seedInitialAdmin(ctx, cfg, repo)

	notificationService := services.NewNotificationService(repo)
	syncService := services.NewSyncService(repo, encryptor)
	syncService.SetNotificationService(notificationService)
	tcoService := services.NewTCOService(dbPool.Pool, cfg.ReportingTimezone)
	svc := appServices{
		sync:         syncService,
		tireWear:     services.NewTireWearService(repo),
		tco:          tcoService,
		energyStats:  services.NewEnergyStatsService(dbPool.Pool, cfg.ReportingTimezone),
		comparison:   services.NewComparisonService(tcoService),
		carpool:      services.NewCarpoolService(dbPool.Pool, repo),
		notification: notificationService,
	}

	// 4. Setup Chi router
	r := newRouter(cfg, trustedProxies)
	registerSystemRoutes(r, dbPool.Pool.Ping)

	api, err := newAPIHandlers(cfg, repo, encryptor, svc)
	if err != nil {
		slog.Error("failed to initialize the API handlers", "error", err)
		os.Exit(1)
	}
	registerAPIRoutes(r, cfg.JWTSecret, repo, handlers.Idempotency(repo), api)
	registerSPA(r)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 75 * time.Second, // above the 60s request timeout so long syncs can still respond
		IdleTimeout:  60 * time.Second,
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	bgCtx, cancelBg := context.WithCancel(context.Background())
	defer cancelBg()
	startWorkers(bgCtx, cfg, repo, syncService)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered panic in HTTP server goroutine", "error", r)
				os.Exit(1)
			}
		}()
		slog.Info("AutoLedger API & web listening", "port", cfg.Port, "base_url", cfg.AppBaseURL)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-stopChan
	slog.Info("Shutting down server...")
	cancelBg()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("AutoLedger server stopped cleanly.")
}

// seedInitialAdmin creates the configured initial admin account when it does not exist yet.
func seedInitialAdmin(ctx context.Context, cfg *config.Config, repo *database.Repository) {
	if cfg.InitialAdminEmail == "" || cfg.InitialAdminPassword == "" {
		return
	}
	_, err := repo.GetUserByEmail(ctx, cfg.InitialAdminEmail)
	if err == nil || !errors.Is(err, database.ErrNotFound) {
		return
	}
	hash, err := auth.HashPassword(cfg.InitialAdminPassword)
	if err != nil {
		return
	}
	adminUser, err := repo.CreateUser(ctx, cfg.InitialAdminEmail, hash)
	if err != nil {
		slog.Error("failed to create initial admin account", "component", "auth", "error", err)
		return
	}
	slog.Info("initial admin account created successfully", "component", "auth", "email", adminUser.Email)
}
