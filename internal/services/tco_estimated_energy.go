package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/money"
)

// EstimatedEnergyMonth is the energy of the distance driven before tracking started in one month, priced at the
// vehicle's own average consumption and price. It is the figure the TCO adds to the recorded energy.
type EstimatedEnergyMonth struct {
	Month    string      `json:"month"` // YYYY-MM
	Km       float64     `json:"km"`
	KwhAdded float64     `json:"kwh_added"`
	Cost     money.Cents `json:"cost"`
}

// EstimatedEnergyByMonth lists, oldest first, the months whose distance has no recorded charge behind it and the
// energy estimated for them. It is empty until the vehicle has both an average consumption and a price per kWh.
func (s *TCOService) EstimatedEnergyByMonth(ctx context.Context, vehicleID string) ([]EstimatedEnergyMonth, error) {
	basics, err := s.loadVehicleBasics(ctx, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("vehicle: %w", err)
	}
	kwh100km, price := basics.estKwh100km, basics.estPricePerKwh
	if kwh100km == nil || *kwh100km <= 0 || price == nil || *price < 0 {
		return nil, nil
	}
	ownership, err := s.repo.GetVehicleOwnership(ctx, vehicleID)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("ownership: %w", err)
	}
	in, err := s.loadSmoothingInputs(ctx, vehicleID, basics.powertrain)
	if err != nil {
		return nil, fmt.Errorf("odometer readings: %w", err)
	}
	_, preTracking, err := s.computeMileageSmoothing(ctx, vehicleID, ownership, basics.currentOdometer, time.Now().In(s.location()), in)
	if err != nil {
		return nil, err
	}
	return estimateEnergyMonths(preTracking, *kwh100km, *price), nil
}

// estimateEnergyMonths prices the pre-tracking distance of each month, with the rounding of the TCO monthly rows.
func estimateEnergyMonths(kmByMonth map[string]float64, kwh100km, pricePerKwh float64) []EstimatedEnergyMonth {
	out := make([]EstimatedEnergyMonth, 0, len(kmByMonth))
	for m, km := range kmByMonth {
		km = round1(km)
		if km <= 0 {
			continue
		}
		kwh := round1(km * kwh100km / 100)
		out = append(out, EstimatedEnergyMonth{Month: m, Km: km, KwhAdded: kwh, Cost: money.FromFloat(kwh * pricePerKwh)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Month < out[j].Month })
	return out
}
