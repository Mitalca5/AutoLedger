package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
)

// setupHybridComparison records a drive for a hybrid in a disposable database.
func setupHybridComparison(t *testing.T, powertrain string, distanceKm float64) (*database.DB, *database.Repository, *models.Vehicle, time.Time) {
	t.Helper()
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "hybrid-comparison@example.com")
	v.Powertrain = powertrain
	if err := repo.UpdateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	mustDrive(t, repo, v.ID, 1, now.AddDate(0, 0, -10), 10000, distanceKm)
	return db, repo, v, now
}
