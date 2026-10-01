package services

import (
	"context"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
)

// FleetService coordinates aggregated multi-vehicle analytics for households.
type FleetService struct {
	repo *database.Repository
}

func NewFleetService(repo *database.Repository) *FleetService {
	return &FleetService{repo: repo}
}

// GetSummary returns consolidated KPIs, monthly breakdown and driver shares for all vehicles of a household.
func (s *FleetService) GetSummary(ctx context.Context, userID string) (*models.FleetSummaryResponse, error) {
	return s.repo.GetFleetSummary(ctx, userID)
}
