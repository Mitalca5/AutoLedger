package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/config"
	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/geocode"
	"github.com/teslacost/teslacost/internal/handlers"
	"github.com/teslacost/teslacost/internal/services"
	"github.com/teslacost/teslacost/internal/storage"
)

// appServices holds the long-lived services the handlers and the background workers share.
type appServices struct {
	sync         *services.SyncService
	tireWear     *services.TireWearService
	tco          *services.TCOService
	energyStats  *services.EnergyStatsService
	comparison   *services.ComparisonService
	carpool      *services.CarpoolService
	notification *services.NotificationService
}

// apiHandlers is every HTTP handler the router mounts. The routes_*.go files register them by domain.
type apiHandlers struct {
	auth           *handlers.AuthHandler
	vehicle        *handlers.VehicleHandler
	drive          *handlers.DriveHandler
	tire           *handlers.TireHandler
	expense        *handlers.ExpenseHandler
	tco            *handlers.TCOHandler
	energy         *handlers.EnergyHandler
	comparison     *handlers.ComparisonHandler
	carpool        *handlers.CarpoolHandler
	checkpoint     *handlers.CheckpointHandler
	fuel           *handlers.FuelHandler
	reminder       *handlers.ReminderHandler
	vehicleMember  *handlers.VehicleMemberHandler
	csvImport      *handlers.ImportHandler
	importProfile  *handlers.ImportProfileHandler
	mileage        *handlers.MileageHandler
	residual       *handlers.ResidualHandler
	serviceBook    *handlers.ServiceBookHandler
	export         *handlers.ExportHandler
	token          *handlers.TokenHandler
	tariff         *handlers.TariffHandler
	pendingCharges *handlers.PendingChargesHandler
	fleet          *handlers.FleetHandler
	ha             *handlers.HomeAssistantHandler
}

// newOIDCService returns the OIDC service when SSO is configured, nil otherwise.
func newOIDCService(cfg *config.Config) (*auth.OIDCService, error) {
	if !cfg.OIDCEnabled {
		slog.Info("OIDC not configured, using local JWT auth only", "component", "auth")
		return nil, nil
	}
	svc, err := auth.NewOIDCService(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("OIDC initialization failed: %w", err)
	}
	slog.Info("OIDC SSO enabled", "component", "auth", "issuer", cfg.OIDCIssuerURL, "provider", cfg.OIDCProviderName)
	return svc, nil
}

// newAPIHandlers wires every handler to its repository and services.
func newAPIHandlers(cfg *config.Config, repo *database.Repository, encryptor *crypto.Encryptor, svc appServices) (*apiHandlers, error) {
	oidcService, err := newOIDCService(cfg)
	if err != nil {
		return nil, err
	}

	tollDetectionService, err := services.NewTollDetectionService(repo, encryptor)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize toll detection service: %w", err)
	}
	storageService, err := storage.NewFileStorageService(cfg.StorageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize file storage service: %w", err)
	}
	migrateLegacyDocuments(repo, storageService)

	mileageService := services.NewMileageService(repo, cfg.ReportingTimezone)
	tariffService := services.NewTariffServiceIn(cfg.ReportingTimezone)

	h := &apiHandlers{
		auth:           handlers.NewAuthHandler(repo, cfg, oidcService),
		vehicle:        handlers.NewVehicleHandler(repo, encryptor, svc.sync),
		drive:          handlers.NewDriveHandler(repo, svc.carpool, tollDetectionService),
		tire:           handlers.NewTireHandler(repo, svc.tireWear),
		expense:        handlers.NewExpenseHandler(repo, storageService),
		tco:            handlers.NewTCOHandler(repo, svc.tco),
		energy:         handlers.NewEnergyHandler(repo, svc.energyStats),
		comparison:     handlers.NewComparisonHandler(repo, svc.comparison),
		carpool:        handlers.NewCarpoolHandler(repo, svc.carpool),
		checkpoint:     handlers.NewCheckpointHandler(repo),
		fuel:           handlers.NewFuelHandler(repo),
		reminder:       handlers.NewReminderHandler(repo, svc.notification),
		vehicleMember:  handlers.NewVehicleMemberHandler(repo),
		csvImport:      handlers.NewImportHandler(repo, services.NewCSVImportService(repo, cfg.ReportingTimezone)),
		importProfile:  handlers.NewImportProfileHandler(repo),
		mileage:        handlers.NewMileageHandler(repo, mileageService),
		residual:       handlers.NewResidualHandler(repo, services.NewResidualService(repo, svc.energyStats)),
		serviceBook:    handlers.NewServiceBookHandler(repo, services.NewServiceBookService(repo, storageService.Read)),
		export:         handlers.NewExportHandler(repo, services.NewExportService(repo).WithMileage(mileageService)),
		token:          handlers.NewTokenHandler(repo),
		tariff:         handlers.NewTariffHandler(repo, tariffService),
		pendingCharges: handlers.NewPendingChargesHandler(repo, tariffService),
		fleet:          handlers.NewFleetHandler(services.NewFleetService(repo, svc.tco)),
		ha:             handlers.NewHomeAssistantHandler(repo, tariffService),
	}
	h.ha.SetTimezone(cfg.ReportingTimezone)
	if cfg.GeocodingEnabled {
		geocoder := geocode.New(cfg.GeocodingURL, cfg.GeocodingUserAgent)
		h.ha.SetGeocoder(geocoder)
		h.drive.SetGeocoder(geocoder)
	}
	return h, nil
}

// migrateLegacyDocuments moves documents still stored in the PostgreSQL BYTEA column to the storage volume.
func migrateLegacyDocuments(repo *database.Repository, storageService *storage.FileStorageService) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	count, err := repo.MigrateLegacyDocuments(ctx, storageService.Save)
	if err != nil {
		slog.Warn("legacy document migration encountered an error", "component", "storage", "error", err)
	} else if count > 0 {
		slog.Info("migrated legacy documents from database to volume storage", "component", "storage", "count", count)
	}
}
