package services

import (
	"context"
	"fmt"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/teslamate"
	"github.com/teslacost/teslacost/internal/tolldata"
)

// ErrNoGPSTrace is returned when a drive has no TeslaMate GPS trace to detect tolls from
// (e.g. a manually-entered drive).
var ErrNoGPSTrace = apierror.New("toll.no_gps", "This drive has no TeslaMate GPS trace available")

// TollDetectionService matches a drive's GPS trace against the vendored OpenTollData
// toll station reference to detect which toll gates it crossed.
type TollDetectionService struct {
	repo      *database.Repository
	encryptor *crypto.Encryptor
	dataset   *tolldata.Dataset
}

// NewTollDetectionService loads the embedded toll dataset and returns a ready-to-use service.
func NewTollDetectionService(repo *database.Repository, encryptor *crypto.Encryptor) (*TollDetectionService, error) {
	dataset, err := tolldata.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load toll dataset: %w", err)
	}
	return &TollDetectionService{repo: repo, encryptor: encryptor, dataset: dataset}, nil
}

// DetectTolls fetches the drive's GPS trace from TeslaMateAPI, matches it against the toll
// station reference, and persists the result. vehicle must belong to the caller (checked by
// the handler before calling this).
func (s *TollDetectionService) DetectTolls(ctx context.Context, vehicle *models.Vehicle, driveID string) (*models.TollDetection, error) {
	drive, err := s.repo.GetDriveByID(ctx, driveID, vehicle.ID)
	if err != nil {
		return nil, err
	}
	if drive.TeslaMateDriveID == nil {
		return nil, ErrNoGPSTrace
	}
	if vehicle.TeslaMateCarID == nil {
		return nil, ErrNoGPSTrace
	}

	client, err := buildTeslaMateClient(vehicle, s.encryptor)
	if err != nil {
		return nil, fmt.Errorf("failed to build TeslaMate client: %w", err)
	}

	positions, units, err := client.GetDriveDetails(ctx, *vehicle.TeslaMateCarID, *drive.TeslaMateDriveID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch drive GPS trace: %w", err)
	}

	trace := make([]tolldata.LatLon, len(positions))
	for i, p := range positions {
		trace[i] = tolldata.LatLon{Lat: p.Latitude, Lon: p.Longitude}
	}

	matches := tolldata.DetectCrossings(trace, traceSpeedsKmh(positions, units.UnitOfLength), s.dataset.Stations, tolldata.DefaultThresholdMeters)
	segments := s.dataset.BuildSegments(matches)

	td := &models.TollDetection{
		DriveID:   drive.ID,
		VehicleID: vehicle.ID,
		Segments:  toModelSegments(segments),
	}
	if err := s.repo.UpsertTollDetection(ctx, td); err != nil {
		return nil, fmt.Errorf("failed to save toll detection: %w", err)
	}

	return td, nil
}

// traceSpeedsKmh returns the trace's speeds in km/h, nil when TeslaMate sent none (it reports a
// missing speed as 0, so a trace where every speed is 0 has no speed information).
func traceSpeedsKmh(positions []teslamate.DrivePosition, unitOfLength string) []float64 {
	speeds := make([]float64, len(positions))
	known := false
	for i, p := range positions {
		speeds[i] = teslamate.ConvertDistanceToKm(p.Speed, unitOfLength)
		known = known || p.Speed > 0
	}
	if !known {
		return nil
	}
	return speeds
}

func toModelSegments(segments []tolldata.Segment) []models.TollSegment {
	out := make([]models.TollSegment, len(segments))
	for i, s := range segments {
		out[i] = models.TollSegment{
			Network:        s.Network,
			Operator:       s.Operator,
			Type:           s.Type,
			Entry:          s.Entry,
			Exit:           s.Exit,
			EstimatedPrice: s.EstimatedPrice,
		}
	}
	return out
}
