package services

import (
	"context"
	"sort"

	"github.com/teslacost/teslacost/internal/money"
)

// tagBreakdown splits the distance, the measured energy and the cost of the drives by tag. A drive carrying several
// tags counts in each of them (shared_drives says how many do); a drive with no tag counts under the empty tag. The
// cost of a drive is the one the drive list shows (ComputeDriveCost), so the total of a tag is the sum of the drives
// listed for it. Costs are only computed once a drive is tagged: the untagged-only split is never displayed.
func (s *TCOService) tagBreakdown(ctx context.Context, vehicleID string, totalDistance float64) ([]TagCostBreakdown, error) {
	drives, err := s.repo.ListDriveCostInputs(ctx, vehicleID)
	if err != nil {
		return nil, err
	}

	anyTagged := false
	ids := make([]string, len(drives))
	for i, d := range drives {
		ids[i] = d.ID
		if len(d.Tags) > 0 {
			anyTagged = true
		}
	}
	tolls, err := s.repo.GetTollExpensesForDrives(ctx, vehicleID, ids)
	if err != nil {
		return nil, err
	}

	var rates *UnitRates
	var insurance map[string]money.Cents
	if anyTagged {
		if rates, err = s.carpool.GetVehicleUnitRates(ctx, vehicleID); err != nil {
			return nil, err
		}
		if insurance, err = s.carpool.AllocateInsuranceCosts(ctx, vehicleID, drives, rates); err != nil {
			return nil, err
		}
	}

	byTag := map[string]*TagCostBreakdown{}
	for _, d := range drives {
		tags := d.Tags
		if len(tags) == 0 {
			tags = []string{""}
		}
		var cost money.Cents
		if rates != nil {
			cost = ComputeDriveCost(rates, d, insurance[d.ID], tolls[d.ID]).TotalCost
		}
		for _, tag := range tags {
			tb := byTag[tag]
			if tb == nil {
				tb = &TagCostBreakdown{Tag: tag}
				byTag[tag] = tb
			}
			tb.DistanceKm += d.DistanceKm
			if d.EnergyConsumedKwh != nil {
				tb.EnergyKwh += *d.EnergyConsumedKwh
			}
			tb.TollsAmount += tolls[d.ID]
			tb.Cost += cost
			if len(d.Tags) > 1 {
				tb.SharedDrives++
			}
		}
	}

	list := make([]TagCostBreakdown, 0, len(byTag))
	for _, tb := range byTag {
		if totalDistance > 0 {
			tb.Percentage = round1(tb.DistanceKm / totalDistance * 100)
		}
		if tb.DistanceKm > 0 && tb.Cost > 0 {
			tb.CostPerKm = round3(tb.Cost.Float() / tb.DistanceKm)
		}
		tb.DistanceKm = round1(tb.DistanceKm)
		tb.EnergyKwh = round1(tb.EnergyKwh)
		list = append(list, *tb)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].DistanceKm != list[j].DistanceKm {
			return list[i].DistanceKm > list[j].DistanceKm
		}
		return list[i].Tag < list[j].Tag
	})
	return list, nil
}
