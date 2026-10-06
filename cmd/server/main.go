package main

import (
	"context"
	"encoding/json"
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

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/config"
	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/geocode"
	"github.com/teslacost/teslacost/internal/handlers"
	appMiddleware "github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/services"
	"github.com/teslacost/teslacost/internal/storage"
	"github.com/teslacost/teslacost/web"
)

// AppVersion is the application version, injected at build time via -ldflags "-X main.AppVersion=...".
var AppVersion = "1.30.1"

// requestIDHandler wraps a slog.Handler to attach the chi request ID (if any is present on the
// context) to every log record. This is what lets a "request_id" field emitted by a *Context
// slog call (e.g. slog.ErrorContext in writeRepoError) be correlated with the chi access log
// line for the same request.
type requestIDHandler struct {
	slog.Handler
}

func (h requestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if reqID := chiMiddleware.GetReqID(ctx); reqID != "" {
		r.AddAttrs(slog.String("request_id", reqID))
	}
	return h.Handler.Handle(ctx, r)
}

func (h requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return requestIDHandler{h.Handler.WithAttrs(attrs)}
}

func (h requestIDHandler) WithGroup(name string) slog.Handler {
	return requestIDHandler{h.Handler.WithGroup(name)}
}

// configureLogging sets the process-wide slog default: JSON output in production (log
// aggregators, jq-friendly), human-readable text otherwise. Level defaults to Info in
// production (never Debug) and Debug in development; LOG_LEVEL overrides either.
func configureLogging(cfg *config.Config) {
	level := slog.LevelInfo
	if !strings.EqualFold(cfg.Environment, "production") {
		level = slog.LevelDebug
	}
	if raw := strings.TrimSpace(os.Getenv("LOG_LEVEL")); raw != "" {
		var parsed slog.Level
		if err := parsed.UnmarshalText([]byte(strings.ToUpper(raw))); err == nil {
			level = parsed
		}
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	if strings.EqualFold(cfg.Environment, "production") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(requestIDHandler{handler}))
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

// maxJSONBodyBytes caps the body of API requests; uploads have their own limit in their handler.
const maxJSONBodyBytes = 1 << 20

// dbConnectTimeout is how long the server waits for PostgreSQL to become reachable on startup before giving up.
// It covers a slow crash recovery after an unclean shutdown, not just a normal container boot race.
const dbConnectTimeout = 3 * time.Minute

// contentSecurityPolicy is the policy sent with every response: the built-in one, a custom one from the
// environment, or none ("off").
func contentSecurityPolicy(cfg *config.Config) string {
	switch {
	case strings.EqualFold(cfg.ContentSecurityPolicy, "off"):
		return ""
	case cfg.ContentSecurityPolicy != "":
		return cfg.ContentSecurityPolicy
	default:
		return appMiddleware.DefaultCSP
	}
}

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
		slog.Error("giving up on PostgreSQL, exiting so the container restarts and retries", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	// A fresh, short-lived context for the rest of startup: it must not inherit whatever is left of the (possibly
	// long) connection wait above.
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	if migErr := dbPool.Migrate(ctx); migErr != nil {
		slog.Error("database migration failed", "error", migErr)
		os.Exit(1)
	}
	repo := database.NewRepository(dbPool.Pool)

	var syncService *services.SyncService
	var tireWearService *services.TireWearService
	var tcoService *services.TCOService
	var energyStatsService *services.EnergyStatsService
	var comparisonService *services.ComparisonService
	var carpoolService *services.CarpoolService
	var notificationService *services.NotificationService

	// Seed initial admin if configured and user does not exist
	if cfg.InitialAdminEmail != "" && cfg.InitialAdminPassword != "" {
		_, err := repo.GetUserByEmail(ctx, cfg.InitialAdminEmail)
		if err != nil && errors.Is(err, database.ErrNotFound) {
			hash, err := auth.HashPassword(cfg.InitialAdminPassword)
			if err == nil {
				adminUser, err := repo.CreateUser(ctx, cfg.InitialAdminEmail, hash)
				if err == nil {
					slog.Info("initial admin account created successfully", "component", "auth", "email", adminUser.Email)
				} else {
					slog.Error("failed to create initial admin account", "component", "auth", "error", err)
				}
			}
		}
	}

	notificationService = services.NewNotificationService(repo)
	syncService = services.NewSyncService(repo, encryptor)
	syncService.SetNotificationService(notificationService)
	tireWearService = services.NewTireWearService(repo)
	tcoService = services.NewTCOService(dbPool.Pool, cfg.ReportingTimezone)
	energyStatsService = services.NewEnergyStatsService(dbPool.Pool, cfg.ReportingTimezone)
	comparisonService = services.NewComparisonService(tcoService)
	carpoolService = services.NewCarpoolService(dbPool.Pool, repo)

	// 4. Setup Chi router
	r := chi.NewRouter()

	// Standard middlewares
	r.Use(chiMiddleware.RequestID)
	// The client address only comes from forwarding headers when the peer is a trusted proxy: rate limits and
	// session records depend on it.
	r.Use(appMiddleware.ClientIP(trustedProxies))
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(60 * time.Second))
	if cfg.SecurityHeaders {
		r.Use(appMiddleware.SecurityHeaders(contentSecurityPolicy(cfg)))
	}
	r.Use(appMiddleware.BodyLimit(maxJSONBodyBytes))
	if cfg.Demo {
		r.Use(appMiddleware.DemoReadOnly)
	}
	r.Use(appMiddleware.OriginCheck(append([]string{cfg.AppBaseURL}, cfg.AllowedOrigins...)))

	// CORS setup
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Public Health Check Endpoint
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "disconnected"
		if dbPool != nil {
			pingCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			if err := dbPool.Pool.Ping(pingCtx); err == nil {
				dbStatus = "connected"
			}
		}

		status := "healthy"
		httpStatus := http.StatusOK
		if dbStatus != "connected" {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(map[string]any{
			"status":    status,
			"database":  dbStatus,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   AppVersion,
		})
	}
	r.Get("/api/health", healthHandler)
	r.Get("/healthz", healthHandler)
	r.Get("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"version": AppVersion,
		})
	})

	// API Routes
	if repo != nil {
		// Initialize OIDC service if configured.
		var oidcService *auth.OIDCService
		if cfg.OIDCEnabled {
			svc, oidcErr := auth.NewOIDCService(context.Background(), cfg)
			if oidcErr != nil {
				slog.Error("OIDC initialization failed", "component", "auth", "error", oidcErr)
				os.Exit(1)
			}
			oidcService = svc
			slog.Info("OIDC SSO enabled", "component", "auth", "issuer", cfg.OIDCIssuerURL, "provider", cfg.OIDCProviderName)
		} else {
			slog.Info("OIDC not configured, using local JWT auth only", "component", "auth")
		}

		authHandler := handlers.NewAuthHandler(repo, cfg, oidcService)
		vehicleHandler := handlers.NewVehicleHandler(repo, encryptor, syncService)

		tollDetectionService, err := services.NewTollDetectionService(repo, encryptor)
		if err != nil {
			slog.Error("failed to initialize toll detection service", "error", err)
			os.Exit(1)
		}
		driveHandler := handlers.NewDriveHandler(repo, carpoolService, tollDetectionService)
		tireHandler := handlers.NewTireHandler(repo, tireWearService)

		storageService, err := storage.NewFileStorageService(cfg.StorageDir)
		if err != nil {
			slog.Error("failed to initialize file storage service", "error", err)
			os.Exit(1)
		}

		// Migrate any legacy unmigrated documents from PostgreSQL BYTEA column to the storage volume
		migCtx, migCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		if count, err := repo.MigrateLegacyDocuments(migCtx, storageService.Save); err != nil {
			slog.Warn("legacy document migration encountered an error", "component", "storage", "error", err)
		} else if count > 0 {
			slog.Info("migrated legacy documents from database to volume storage", "component", "storage", "count", count)
		}
		migCancel()

		expenseHandler := handlers.NewExpenseHandler(repo, storageService)
		tcoHandler := handlers.NewTCOHandler(repo, tcoService)
		energyHandler := handlers.NewEnergyHandler(repo, energyStatsService)
		comparisonHandler := handlers.NewComparisonHandler(repo, comparisonService)
		carpoolHandler := handlers.NewCarpoolHandler(repo, carpoolService)
		checkpointHandler := handlers.NewCheckpointHandler(repo)
		fuelHandler := handlers.NewFuelHandler(repo)
		reminderHandler := handlers.NewReminderHandler(repo, notificationService)
		vehicleMemberHandler := handlers.NewVehicleMemberHandler(repo)
		csvImportService := services.NewCSVImportService(repo, cfg.ReportingTimezone)
		importHandler := handlers.NewImportHandler(repo, csvImportService)
		importProfileHandler := handlers.NewImportProfileHandler(repo)
		mileageService := services.NewMileageService(repo, cfg.ReportingTimezone)
		mileageHandler := handlers.NewMileageHandler(repo, mileageService)
		residualHandler := handlers.NewResidualHandler(repo, services.NewResidualService(repo, energyStatsService))
		serviceBookHandler := handlers.NewServiceBookHandler(repo, services.NewServiceBookService(repo, storageService.Read))
		exportHandler := handlers.NewExportHandler(repo, services.NewExportService(repo).WithMileage(mileageService))

		tariffService := services.NewTariffServiceIn(cfg.ReportingTimezone)
		fleetService := services.NewFleetService(repo, tcoService)

		tokenHandler := handlers.NewTokenHandler(repo)
		tariffHandler := handlers.NewTariffHandler(repo, tariffService)
		pendingChargesHandler := handlers.NewPendingChargesHandler(repo, tariffService)
		fleetHandler := handlers.NewFleetHandler(fleetService)
		haHandler := handlers.NewHomeAssistantHandler(repo, tariffService)
		haHandler.SetTimezone(cfg.ReportingTimezone)
		if cfg.GeocodingEnabled {
			geocoder := geocode.New(cfg.GeocodingURL, cfg.GeocodingUserAgent)
			haHandler.SetGeocoder(geocoder)
			driveHandler.SetGeocoder(geocoder)
		}

		// Public Auth
		r.Route("/api/auth", func(r chi.Router) {
			r.Get("/config", authHandler.GetConfig)
			// Rate limited by IP: these are the credential-guessing surface (password brute
			// force, account enumeration via registration). 10 attempts/minute is generous for
			// a legitimate user retrying a typo but blocks automated guessing.
			r.With(httprate.LimitByIP(10, time.Minute)).Post("/register", authHandler.Register)
			r.With(httprate.LimitByIP(10, time.Minute)).Post("/login", authHandler.Login)
			// A refresh token is a credential too: guessing is pointless (256 random bits) but the route does a
			// database write, so it gets a generous limit rather than none.
			r.With(httprate.LimitByIP(30, time.Minute)).Post("/refresh", authHandler.RefreshToken)
			r.Post("/logout", authHandler.Logout)
			// OIDC Authorization Code Flow endpoints (public — no JWT required)
			r.With(httprate.LimitByIP(20, time.Minute)).Get("/oidc/login", authHandler.OIDCLogin)
			r.With(httprate.LimitByIP(20, time.Minute)).Get("/oidc/callback", authHandler.OIDCCallback)
		})

		// Integration routes: the only ones an API token opens (Home Assistant, scripts); a session works too.
		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.AuthenticateIntegration(cfg.JWTSecret, repo))
			r.Use(handlers.Idempotency(repo))

			r.Post("/api/integrations/homeassistant/event", haHandler.HandleEvent)
			r.Get("/api/integrations/homeassistant/vehicles", haHandler.ListVehicles)
			r.Get("/api/integrations/homeassistant/vehicles/{vehicleId}", haHandler.GetVehicle)
			r.Get("/api/integrations/homeassistant/vehicles/{vehicleId}/metrics", haHandler.GetVehicleMetrics)
		})

		// Protected Routes (session only)
		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.AuthenticateJWT(cfg.JWTSecret))
			r.Use(handlers.Idempotency(repo))

			r.Get("/api/auth/me", authHandler.Me)
			r.Get("/api/auth/sessions", authHandler.ListSessions)
			r.Delete("/api/auth/sessions/{sessionId}", authHandler.RevokeSession)
			r.Post("/api/auth/logout-all", authHandler.LogoutAll)
			r.With(httprate.LimitByIP(10, time.Minute)).Post("/api/auth/password", authHandler.ChangePassword)
			r.Put("/api/auth/language", authHandler.UpdateLanguage)
			r.Put("/api/auth/distance-unit", authHandler.UpdateDistanceUnit)

			// API Tokens (External Integrations / Home Assistant)
			r.Route("/api/auth/tokens", func(r chi.Router) {
				r.Get("/", tokenHandler.ListTokens)
				r.Post("/", tokenHandler.CreateToken)
				r.Delete("/{tokenId}", tokenHandler.RevokeToken)
			})

			// Tariffs & Public Charging Calculator
			r.Route("/api/mileage-rates", func(r chi.Router) {
				r.Get("/", mileageHandler.ListRates)
				r.Post("/", mileageHandler.CreateRate)
				r.Delete("/{id}", mileageHandler.DeleteRate)
			})
			// Reminder templates
			r.Route("/api/reminder-templates", func(r chi.Router) {
				r.Get("/", reminderHandler.ListTemplates)
				r.Post("/", reminderHandler.CreateTemplate)
				r.Delete("/{id}", reminderHandler.DeleteTemplate)
			})
			r.Route("/api/tariffs", func(r chi.Router) {
				r.Get("/plans", tariffHandler.List)
				r.Post("/plans", tariffHandler.Create)
				r.Put("/plans/{id}", tariffHandler.Update)
				r.Delete("/plans/{id}", tariffHandler.Delete)
				r.Post("/calculate-session", tariffHandler.Calculate)
				r.Get("/public-presets", tariffHandler.ListPublicPresets)
				r.Post("/public-presets", tariffHandler.CreatePublicPreset)
				r.Delete("/public-presets/{id}", tariffHandler.DeletePublicPreset)
				r.Post("/calculate-public", tariffHandler.CalculatePublic)
			})

			// Pending Charges ("Recharges à qualifier")
			r.Route("/api/pending-charges", func(r chi.Router) {
				r.Get("/", pendingChargesHandler.List)
				r.Post("/{id}/assign", pendingChargesHandler.Assign)
				r.Delete("/{id}", pendingChargesHandler.Delete)
			})

			// Saved CSV import mappings
			r.Route("/api/import-profiles", func(r chi.Router) {
				r.Get("/", importProfileHandler.List)
				r.Post("/", importProfileHandler.Save)
				r.Delete("/{profileId}", importProfileHandler.Delete)
			})

			// Household Fleet Dashboard
			r.Get("/api/fleet/summary", fleetHandler.GetSummary)
			r.Put("/api/fleet/budget", fleetHandler.SetBudget)

			// EV vs ICE cost comparison (informational)
			r.Route("/api/comparison-scenarios", func(r chi.Router) {
				r.Get("/", comparisonHandler.List)
				r.Post("/", comparisonHandler.Create)
				r.Get("/defaults", comparisonHandler.Defaults)
				r.Put("/{scenarioId}", comparisonHandler.Update)
				r.Delete("/{scenarioId}", comparisonHandler.Delete)
				r.Get("/{scenarioId}/result", comparisonHandler.Result)
			})

			// Vehicles
			r.Route("/api/vehicles", func(r chi.Router) {
				r.Get("/", vehicleHandler.List)
				r.Post("/", vehicleHandler.Create)
				r.Post("/test-connection", vehicleHandler.TestTeslaMateRaw)
				r.Get("/{id}", vehicleHandler.Get)
				r.Put("/{id}", vehicleHandler.Update)
				r.Delete("/{id}", vehicleHandler.Delete)
				r.Post("/{id}/teslamate/test", vehicleHandler.TestTeslaMate)
				r.Post("/{id}/sync", vehicleHandler.Sync)
				r.Get("/{id}/sync", vehicleHandler.GetSyncStatus)
				r.Get("/{id}/ownership", vehicleHandler.GetOwnership)
				r.Put("/{id}/ownership", vehicleHandler.SaveOwnership)
				r.Delete("/{id}/ownership", vehicleHandler.DeleteOwnership)
				r.Put("/{id}/estimated-energy", vehicleHandler.UpdateEstimatedEnergy)
				r.Get("/{id}/odometer-at", vehicleHandler.GetOdometerAtDate)
				r.Get("/{id}/odometer-estimate", vehicleHandler.GetOdometerEstimate)
				r.Get("/{id}/data-sources", vehicleHandler.GetDataSources)
				r.Get("/{vehicleId}/data-quality", tcoHandler.GetDataQuality)

				// Shared Vehicle Members
				r.Get("/{id}/members", vehicleMemberHandler.ListMembers)
				r.Post("/{id}/members", vehicleMemberHandler.AddMember)
				r.Put("/{id}/members/{memberId}", vehicleMemberHandler.UpdateMemberRole)
				r.Delete("/{id}/members/{memberId}", vehicleMemberHandler.RemoveMember)
				r.Get("/{id}/people", vehicleMemberHandler.ListPeople)
				r.Post("/{id}/people", vehicleMemberHandler.CreatePerson)
				r.Put("/{id}/people/{personId}", vehicleMemberHandler.UpdatePerson)
				r.Delete("/{id}/people/{personId}", vehicleMemberHandler.DeletePerson)
				r.Put("/{id}/people/{personId}/link", vehicleMemberHandler.LinkPerson)
				r.Put("/{id}/people/{personId}/default", vehicleMemberHandler.SetDefaultPerson)

				// Odometer Checkpoints
				r.Get("/{vehicleId}/odometer-checkpoints", checkpointHandler.List)
				r.Post("/{vehicleId}/odometer-checkpoints", checkpointHandler.Create)
				r.Put("/{vehicleId}/odometer-checkpoints/{checkpointId}", checkpointHandler.Update)
				r.Delete("/{vehicleId}/odometer-checkpoints/{checkpointId}", checkpointHandler.Delete)

				// Fuel fill-ups (combustion vehicles)
				r.Get("/{vehicleId}/fuel-logs", fuelHandler.List)
				r.Post("/{vehicleId}/fuel-logs", fuelHandler.Create)
				r.Put("/{vehicleId}/fuel-logs/{fuelLogId}", fuelHandler.Update)
				r.Delete("/{vehicleId}/fuel-logs/{fuelLogId}", fuelHandler.Delete)

				// Drives
				r.Get("/{vehicleId}/drives", driveHandler.List)
				r.Get("/{vehicleId}/drives/address-backfill", driveHandler.AddressBackfillStatus)
				r.Post("/{vehicleId}/drives/resolve-addresses", driveHandler.ResolveAddresses)
				r.Post("/{vehicleId}/drives", driveHandler.Create)
				r.Put("/{vehicleId}/drives/{driveId}", driveHandler.Update)
				r.Delete("/{vehicleId}/drives/{driveId}", driveHandler.Delete)
				r.Put("/{vehicleId}/drives/{driveId}/driver", driveHandler.UpdateDriver)
				r.Get("/{vehicleId}/drives/{driveId}/expenses", driveHandler.GetDriveExpenses)

				// Export (CSV / JSON)
				r.Get("/{vehicleId}/export", exportHandler.Export)
				r.Get("/{vehicleId}/service-book", serviceBookHandler.Download)
				r.Get("/{vehicleId}/mileage-report", mileageHandler.Report)
				r.Get("/{vehicleId}/battery-health", residualHandler.BatteryHealth)
				r.Post("/{vehicleId}/battery-health", residualHandler.SaveReading)
				r.Delete("/{vehicleId}/battery-health/{date}", residualHandler.DeleteReading)
				r.Get("/{vehicleId}/residual-value", residualHandler.Residual)

				// Import (CSV Charges & Drives)
				r.Post("/{vehicleId}/import/preview", importHandler.Preview)
				r.Post("/{vehicleId}/import/execute", importHandler.Execute)
				r.Get("/{vehicleId}/import/batches", importHandler.ListBatches)
				r.Delete("/{vehicleId}/import/batches/{batchId}", importHandler.UndoBatch)
				r.Patch("/{vehicleId}/drives/{driveId}/tags", driveHandler.UpdateTags)
				r.Patch("/{vehicleId}/drives/{driveId}/toll-review", driveHandler.SetTollReview)
				r.Get("/{vehicleId}/drives/{driveId}/toll-detection", driveHandler.GetTollDetection)
				r.Post("/{vehicleId}/drives/{driveId}/detect-tolls", driveHandler.DetectTolls)
				r.Post("/{vehicleId}/drives/{driveId}/apply-toll-estimate", driveHandler.ApplyTollEstimate)
				r.Post("/{vehicleId}/drives/apply-toll-estimates", driveHandler.ApplyTollEstimatesBulk)
				r.Post("/{vehicleId}/trip-groups", driveHandler.CreateTripGroup)
				r.Get("/{vehicleId}/trip-groups", driveHandler.ListTripGroups)
				r.Get("/{vehicleId}/trip-suggestions", driveHandler.TripSuggestions)
				r.Post("/{vehicleId}/trip-suggestions/dismiss", driveHandler.DismissTripSuggestion)
				r.Put("/{vehicleId}/trip-groups/{groupId}", driveHandler.UpdateTripGroup)
				r.Delete("/{vehicleId}/trip-groups/{groupId}", driveHandler.DeleteTripGroup)

				// Carpooling
				r.Get("/{vehicleId}/carpools", carpoolHandler.List)
				r.Post("/{vehicleId}/carpools", carpoolHandler.Create)
				r.Post("/{vehicleId}/carpools/recalculate", carpoolHandler.Recalculate)
				r.Get("/{vehicleId}/carpools/estimate", carpoolHandler.Estimate)
				r.Get("/{vehicleId}/carpools/{id}", carpoolHandler.Get)
				r.Put("/{vehicleId}/carpools/{id}", carpoolHandler.Update)
				r.Delete("/{vehicleId}/carpools/{id}", carpoolHandler.Delete)

				// Tires
				r.Get("/{vehicleId}/tires", tireHandler.List)
				r.Post("/{vehicleId}/tires", tireHandler.Create)
				r.Post("/{vehicleId}/tires/batch", tireHandler.BatchCreate)
				r.Patch("/{vehicleId}/tires/batch", tireHandler.BatchUpdate)
				r.Post("/{vehicleId}/tires/batch-dispose", tireHandler.BatchDispose)
				r.Post("/{vehicleId}/tires/quick-rotate", tireHandler.QuickRotate)
				r.Put("/{vehicleId}/tires/{tireId}", tireHandler.Update)
				r.Delete("/{vehicleId}/tires/{tireId}", tireHandler.Delete)
				r.Post("/{vehicleId}/tires/{tireId}/dispose", tireHandler.Dispose)
				r.Post("/{vehicleId}/tires/{tireId}/copy-history", tireHandler.CopyHistory)
				r.Get("/{vehicleId}/tires/{tireId}/history", tireHandler.GetHistory)
				r.Post("/{vehicleId}/tires/{tireId}/sessions", tireHandler.CreateSession)
				r.Put("/{vehicleId}/tires/{tireId}/sessions/{sessionId}", tireHandler.UpdateSession)
				r.Delete("/{vehicleId}/tires/{tireId}/sessions/{sessionId}", tireHandler.DeleteSession)
				r.Post("/{vehicleId}/tires/{tireId}/logs", tireHandler.AddLog)
				r.Put("/{vehicleId}/tires/{tireId}/logs/{logId}", tireHandler.UpdateLog)
				r.Delete("/{vehicleId}/tires/{tireId}/logs/{logId}", tireHandler.DeleteLog)
				r.Post("/{vehicleId}/tire-rotations", tireHandler.Rotate)

				// Expenses
				r.Get("/{vehicleId}/expenses", expenseHandler.ListDriveExpenses)
				r.Post("/{vehicleId}/expenses", expenseHandler.CreateDriveExpense)
				r.Put("/{vehicleId}/expenses/{expenseId}", expenseHandler.UpdateDriveExpense)
				r.Delete("/{vehicleId}/expenses/{expenseId}", expenseHandler.DeleteDriveExpense)
				r.Get("/{vehicleId}/maintenance", expenseHandler.ListMaintenance)
				r.Post("/{vehicleId}/maintenance", expenseHandler.CreateMaintenance)
				r.Put("/{vehicleId}/maintenance/{maintenanceId}", expenseHandler.UpdateMaintenance)
				r.Delete("/{vehicleId}/maintenance/{maintenanceId}", expenseHandler.DeleteMaintenance)
				r.Get("/{vehicleId}/charges", expenseHandler.ListCharges)
				r.Post("/{vehicleId}/charges", expenseHandler.CreateManualCharge)
				r.Put("/{vehicleId}/charges/{chargeId}", expenseHandler.UpdateCharge)
				r.Delete("/{vehicleId}/charges/{chargeId}", expenseHandler.DeleteManualCharge)

				// Documents & Invoices
				r.Get("/{vehicleId}/documents", expenseHandler.ListDocuments)
				r.Post("/{vehicleId}/documents", expenseHandler.UploadDocument)
				r.Get("/{vehicleId}/documents/{docId}", expenseHandler.DownloadDocument)
				r.Delete("/{vehicleId}/documents/{docId}", expenseHandler.DeleteDocument)

				// Maintenance Reminders & Webhooks
				r.Get("/{vehicleId}/reminders", reminderHandler.List)
				r.Post("/{vehicleId}/reminders", reminderHandler.Create)
				r.Post("/{vehicleId}/reminders/apply-template", reminderHandler.ApplyTemplate)
				r.Put("/{vehicleId}/reminders/{reminderId}", reminderHandler.Update)
				r.Delete("/{vehicleId}/reminders/{reminderId}", reminderHandler.Delete)
				r.Post("/{vehicleId}/reminders/{reminderId}/complete", reminderHandler.Complete)
				r.Get("/{vehicleId}/webhook", reminderHandler.GetWebhook)
				r.Put("/{vehicleId}/webhook", reminderHandler.SaveWebhook)
				r.Delete("/{vehicleId}/webhook", reminderHandler.DeleteWebhook)
				r.Post("/{vehicleId}/webhook/test", reminderHandler.TestWebhook)

				// TCO Analytics
				r.Get("/{vehicleId}/tco", tcoHandler.GetTCO)
				r.Get("/{vehicleId}/energy-stats", energyHandler.GetStats)
			})
		})
	}

	// Embedded Vue 3 Frontend SPA serving
	spaServer := handlers.NewSPAServer(web.GetDistFS())
	r.Handle("/*", spaServer)

	// Server setup
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 75 * time.Second, // above the 60s request timeout so long syncs can still respond
		IdleTimeout:  60 * time.Second,
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	// Background workers (Auto-sync)
	bgCtx, cancelBg := context.WithCancel(context.Background())
	defer cancelBg()

	if syncService != nil && cfg.SyncIntervalMinutes > 0 {
		go syncService.StartBackgroundWorker(bgCtx, cfg.SyncIntervalMinutes)
	}

	// Every refresh adds a token row: the expired and revoked ones are purged once a day.
	if repo != nil {
		go purgeExpiredRefreshTokens(bgCtx, repo, 24*time.Hour)
	}

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
