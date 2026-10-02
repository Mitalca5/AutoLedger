package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// TCOService computes TCO metrics from the cost ledger.
type TCOService struct {
	pool     *pgxpool.Pool
	repo     *database.Repository
	timezone string
}

// NewTCOService creates a new TCOService; timezone (IANA name) is used for monthly buckets.
func NewTCOService(pool *pgxpool.Pool, timezone string) *TCOService {
	if timezone == "" {
		timezone = "UTC"
	}
	return &TCOService{pool: pool, repo: database.NewRepository(pool), timezone: timezone}
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func perKm(amount money.Cents, km float64) float64 {
	if km <= 0 {
		return 0
	}
	return round3(amount.Float() / km)
}

// ComputeVehicleTCO calculates complete TCO for a given vehicle.
func (s *TCOService) ComputeVehicleTCO(ctx context.Context, vehicleID string) (*TCOSummary, error) {
	sum := &TCOSummary{}
	comp := &sum.Completeness
	now := time.Now()

	// 1. Ownership contract and odometer
	ownership, err := s.repo.GetVehicleOwnership(ctx, vehicleID)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("ownership: %w", err)
	}
	var currentOdometer float64
	var estKwh100km, estPricePerKwh *float64
	var powertrain, telemetryMode string
	if err := s.pool.QueryRow(ctx, `SELECT current_odometer, estimated_kwh_100km, estimated_price_per_kwh, powertrain, telemetry_mode FROM vehicles WHERE id = $1;`, vehicleID).Scan(&currentOdometer, &estKwh100km, &estPricePerKwh, &powertrain, &telemetryMode); err != nil {
		return nil, fmt.Errorf("vehicle: %w", err)
	}
	isICE := powertrain == models.PowertrainICE
	isManual := telemetryMode == models.TelemetryManual || telemetryMode == models.TelemetrySemiAuto
	sum.Powertrain = powertrain
	sum.EstimatedKwh100km = estKwh100km
	sum.EstimatedPricePerKwh = estPricePerKwh

	// 2. Distance: tracked drives, odometer span, distance since the start of the contract
	var trackedKm, odometerSpan, trackedSinceStart float64
	var contractStart *time.Time
	if ownership != nil {
		contractStart = &ownership.StartDate
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(distance_km), 0),
		       COALESCE(MAX(end_odometer) FILTER (WHERE end_odometer > 0) - MIN(start_odometer) FILTER (WHERE start_odometer > 0), 0),
		       COALESCE(SUM(distance_km) FILTER (WHERE $2::timestamptz IS NULL OR start_time >= $2), 0)
		FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL;
	`, vehicleID, contractStart).Scan(&trackedKm, &odometerSpan, &trackedSinceStart); err != nil {
		return nil, fmt.Errorf("distance: %w", err)
	}
	basisKm := math.Max(trackedKm, odometerSpan)
	var fuelStats FuelStats
	var fuelSpanKm float64
	readings, err := s.repo.ListOdometerCheckpoints(ctx, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("odometer readings: %w", err)
	}

	// Mileage covered by odometer readings (manual or integration), the start of the ownership and, for a
	// combustion vehicle, fill-ups that carry a mileage
	minOdo, maxOdo, known := 0.0, 0.0, false
	note := func(odo float64) {
		if !known || odo < minOdo {
			minOdo = odo
		}
		if !known || odo > maxOdo {
			maxOdo = odo
		}
		known = true
	}
	for _, r := range readings {
		note(r.Odometer)
	}
	if ownership != nil && ownership.StartOdometer != nil {
		note(*ownership.StartOdometer)
	}
	if isICE {
		fuelLogs, err := s.repo.ListFuelLogs(ctx, vehicleID)
		if err != nil {
			return nil, fmt.Errorf("fuel logs: %w", err)
		}
		fuelStats = ComputeFuelStats(fuelLogs, BuildOdometerRefs(readings, ownership))
		for _, f := range fuelLogs {
			if f.Odometer != nil {
				note(*f.Odometer)
			}
		}
	}
	if known {
		fuelSpanKm = maxOdo - minOdo
		basisKm = math.Max(basisKm, fuelSpanKm)
	}
	comp.StartOdometerMissing = ownership == nil || ownership.StartOdometer == nil
	if comp.StartOdometerMissing && known {
		comp.Warnings = append(comp.Warnings, "The odometer at the start of ownership is not entered: the mileage before the first reading is not counted")
	}
	kmSinceStart := trackedSinceStart
	if ownership != nil && ownership.StartOdometer != nil && currentOdometer > *ownership.StartOdometer {
		kmSinceStart = math.Max(kmSinceStart, currentOdometer-*ownership.StartOdometer)
		basisKm = math.Max(basisKm, kmSinceStart)
	}
	if untracked := basisKm - trackedKm; !isICE && untracked > 50 && untracked > 0.01*basisKm {
		comp.UntrackedDistanceKm = round1(untracked)
	}
	owned := ComputeOwnershipCosts(ownership, now, kmSinceStart)

	// 3. Ledger totals per category
	byCategory := map[string]money.Cents{}
	rows, err := s.pool.Query(ctx, `
		SELECT category, COALESCE(SUM(amount_eur), 0) FROM cost_ledger WHERE vehicle_id = $1 GROUP BY category;
	`, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("ledger: %w", err)
	}
	for rows.Next() {
		var category string
		var amount money.Cents
		if err := rows.Scan(&category, &amount); err != nil {
			rows.Close()
			return nil, err
		}
		byCategory[category] = amount
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 4. Energy volume and gaps
	var kwhAdded, kwhPriced float64
	var unconvertedCharges int
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(kwh_added), 0),
		       COALESCE(SUM(kwh_added) FILTER (WHERE cost IS NOT NULL AND (currency = (SELECT currency FROM vehicles WHERE id = $1) OR fx_rate IS NOT NULL)), 0),
		       COUNT(*) FILTER (WHERE cost IS NULL),
		       COALESCE(SUM(kwh_added) FILTER (WHERE cost IS NULL), 0),
		       COUNT(*) FILTER (WHERE cost IS NOT NULL AND currency <> (SELECT currency FROM vehicles WHERE id = $1) AND fx_rate IS NULL)
		FROM charge_logs
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL;
	`, vehicleID).Scan(&kwhAdded, &kwhPriced, &comp.ChargesWithoutCost, &comp.KwhWithoutCost, &unconvertedCharges); err != nil {
		return nil, fmt.Errorf("energy: %w", err)
	}
	if comp.ChargesWithoutCost > 0 {
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%d charge(s) without a cost (%.0f kWh): energy cost underestimated", comp.ChargesWithoutCost, comp.KwhWithoutCost))
	}

	if isICE {
		sum.FuelFillUps = fuelStats.FillUps
		sum.TotalLiters = fuelStats.TotalLiters
		sum.AvgCostPerLiter = fuelStats.AvgPricePerLiter
		sum.ConsumptionL100km = fuelStats.ConsumptionL100
		if fuelStats.FillUps == 0 {
			comp.Warnings = append(comp.Warnings, "No fill-up recorded: fuel cost unknown")
		}
	}

	// 5. Unconverted foreign amounts, insurance expenses, carpool revenue
	var unconvertedOther, insuranceEntries int
	if err := s.pool.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM drive_expenses WHERE vehicle_id = $1 AND currency <> (SELECT currency FROM vehicles WHERE id = $1) AND fx_rate IS NULL)
		     + (SELECT COUNT(*) FROM maintenance_expenses WHERE vehicle_id = $1 AND currency <> (SELECT currency FROM vehicles WHERE id = $1) AND fx_rate IS NULL),
		       (SELECT COUNT(*) FROM maintenance_expenses WHERE vehicle_id = $1 AND category = 'INSURANCE'),
		       (SELECT COALESCE(SUM(total_revenue), 0) FROM carpool_trips WHERE vehicle_id = $1);
	`, vehicleID).Scan(&unconvertedOther, &insuranceEntries, &sum.CarpoolRevenue); err != nil {
		return nil, fmt.Errorf("completeness: %w", err)
	}
	comp.UnconvertedExpenses = unconvertedCharges + unconvertedOther
	if comp.UnconvertedExpenses > 0 {
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%d expense(s) in a foreign currency without a conversion rate: excluded from the totals", comp.UnconvertedExpenses))
	}
	switch {
	case insuranceEntries > 0:
		sum.InsuranceSource = InsuranceSourceRecordedExpenses
	case owned.IncludesInsurance:
		sum.InsuranceSource = InsuranceSourceIncluded
	default:
		sum.InsuranceSource = InsuranceSourceNone
		comp.InsuranceMissing = true
		comp.Warnings = append(comp.Warnings, "No insurance premium recorded (recurring “Insurance” expense)")
	}

	// 6. Tires amortized by kilometers actually driven on each tire
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(
		           CASE
		               WHEN t.current_position = 'DISPOSED' OR t.is_archived THEN t.purchase_price
		               ELSE t.purchase_price * LEAST(1.0, GREATEST(0,
		                   (t.accumulated_distance_km - t.initial_distance_km
		                    + CASE WHEN t.current_position IN ('FL', 'FR', 'RL', 'RR') AND t.mounted_odometer IS NOT NULL
		                           THEN GREATEST(v.current_odometer - t.mounted_odometer, 0) ELSE 0 END)
		                   / GREATEST(t.estimated_lifespan_km - t.initial_distance_km, 1)))
		           END
		       ), 0)
		FROM tires t
		JOIN vehicles v ON v.id = t.vehicle_id
		WHERE t.vehicle_id = $1;
	`, vehicleID).Scan(&sum.TiresAmortizedCost); err != nil {
		return nil, fmt.Errorf("tires: %w", err)
	}

	// 7. Acquisition, financing and depreciation
	if ownership != nil {
		sum.AcquisitionType = ownership.AcquisitionType
		sum.ContractStartDate = &ownership.StartDate
		sum.ContractEndDate = owned.ContractEndDate
		sum.OwnershipEndDate = ownership.EndDate
		if ownership.IsLease() {
			sum.ContractDurationMonths = ownership.LeaseDurationMonths
			sum.LeaseKmAllowancePerYear = ownership.LeaseKmAllowancePerYear
			if ownership.LeaseKmAllowancePerYear != nil && ownership.LeaseDurationMonths != nil && *ownership.LeaseDurationMonths > 0 {
				totalAllowance := round1(*ownership.LeaseKmAllowancePerYear * float64(*ownership.LeaseDurationMonths) / 12)
				sum.LeaseKmAllowanceTotal = &totalAllowance
			}
			sum.LeaseExcessKmPrice = ownership.LeaseExcessKmPrice
			sum.LeaseMonthlyRent = ownership.LeaseMonthlyRent
			sum.LeaseDownPayment = ownership.LeaseDownPayment
			sum.LeasePurchaseOptionPrice = ownership.LeasePurchaseOptionPrice
			sum.OptionExercisedDate = ownership.OptionExercisedDate
			sum.LeaseIncludesMaintenance = ownership.LeaseIncludesMaintenance
			sum.LeaseIncludesInsurance = ownership.LeaseIncludesInsurance
			sum.LeaseIncludesTires = ownership.LeaseIncludesTires
		} else if ownership.AcquisitionType == models.AcquisitionLoan {
			sum.ContractDurationMonths = ownership.LoanDurationMonths
		} else if ownership.ExpectedHoldingMonths != nil {
			sum.ContractDurationMonths = ownership.ExpectedHoldingMonths
		}
	}
	sum.AcquisitionCost = byCategory[LedgerAcquisition]
	sum.DepreciationCost = owned.Depreciation
	sum.LeaseExcessKmCost = owned.LeaseExcessKm
	sum.LeaseExcessKmProjected = owned.LeaseExcessKmProjected
	sum.LeaseKmDriven = round1(owned.LeaseKmDriven)
	sum.LeaseKmAllowanceToDate = round1(owned.LeaseKmAllowanceToDate)
	if len(owned.Missing) > 0 {
		comp.AcquisitionMissing = true
		comp.Warnings = append(comp.Warnings, owned.Missing...)
	}
	comp.Warnings = append(comp.Warnings, owned.LeaseWarnings(now)...)

	// 8. Toll qualification backlog
	var highwayDrives, drivesWithOdometer, ledgerEntries int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE `+database.UnqualifiedDrivePredicate+`),
		       COUNT(*) FILTER (WHERE `+database.HighwayDrivePredicate+`),
		       COUNT(*) FILTER (WHERE start_odometer > 0 AND end_odometer > 0),
		       (SELECT COUNT(*) FROM cost_ledger WHERE vehicle_id = $1)
		FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL;
	`, vehicleID).Scan(&comp.UnqualifiedDrives, &highwayDrives, &drivesWithOdometer, &ledgerEntries); err != nil {
		return nil, fmt.Errorf("unqualified drives: %w", err)
	}
	if comp.UnqualifiedDrives > 0 {
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%d motorway-type drive(s) with no toll entered and not qualified as “no toll”", comp.UnqualifiedDrives))
	}
	if basisKm <= 0 {
		comp.Warnings = append(comp.Warnings, "No mileage recorded: the cost per kilometre cannot be calculated")
	}

	// 9. Odometer continuity
	var gapKm float64
	if err := s.pool.QueryRow(ctx, database.OdometerContinuitySummarySQL, vehicleID).Scan(&comp.OdometerGaps, &gapKm, &comp.OdometerAnomalies); err != nil {
		return nil, fmt.Errorf("odometer continuity: %w", err)
	}

	var checkpointCount int
	var daysSinceCheckpoint int
	_ = s.pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(EXTRACT(DAY FROM (NOW() - MAX(date)))::int, 999)
		FROM odometer_checkpoints
		WHERE vehicle_id = $1;
	`, vehicleID).Scan(&checkpointCount, &daysSinceCheckpoint)

	if isManual {
		// Manual tracking does not expect continuous drives; gaps between manual entries are normal.
		if comp.OdometerAnomalies > 0 {
			comp.Warnings = append(comp.Warnings, fmt.Sprintf(
				"%d odometer inconsistency(ies) (odometer going backwards or distance differing from reading)", comp.OdometerAnomalies))
		}
	} else {
		if comp.OdometerGaps > 0 {
			comp.Warnings = append(comp.Warnings, fmt.Sprintf(
				"%d odometer gap(s) between consecutive drives (%.0f km with no recorded drive)", comp.OdometerGaps, gapKm))
		}
		if comp.OdometerAnomalies > 0 {
			comp.Warnings = append(comp.Warnings, fmt.Sprintf(
				"%d odometer inconsistency(ies) (odometer going backwards or distance differing from the reading) to check in TeslaMate", comp.OdometerAnomalies))
		}
	}

	comp.IsComplete = len(comp.Warnings) == 0
	if comp.Warnings == nil {
		comp.Warnings = []string{}
	}
	comp.ScorePct, comp.Dimensions = completenessScore(completenessInputs{
		kwhAdded:            kwhAdded,
		kwhPriced:           kwhPriced,
		highwayDrives:       highwayDrives,
		unqualifiedDrives:   comp.UnqualifiedDrives,
		trackedKm:           math.Max(trackedKm, fuelSpanKm),
		basisKm:             basisKm,
		insurancePresent:    !comp.InsuranceMissing,
		acquisitionComplete: !comp.AcquisitionMissing,
		pricedEntries:       ledgerEntries,
		unconvertedEntries:  comp.UnconvertedExpenses,
		drivesWithOdometer:  drivesWithOdometer,
		odometerAnomalies:   comp.OdometerAnomalies + comp.OdometerGaps,
		ice:                 isICE,
		iceFillUps:          fuelStats.FillUps,
		telemetryMode:       telemetryMode,
		daysSinceOdometer:   daysSinceCheckpoint,
		hasCheckpoints:      checkpointCount > 0,
	})

	// 10. Aggregates
	energy := byCategory[LedgerEnergy]
	travel := byCategory[LedgerToll] + byCategory[LedgerParking] + byCategory[LedgerTravelOther]
	tires := byCategory[LedgerTires]
	maintenance := byCategory[LedgerMaintenance]
	repair := byCategory[LedgerRepair]
	insurance := byCategory[LedgerInsurance]
	financing := byCategory[LedgerFinancing]
	subscription, tax := byCategory[LedgerSubscr], byCategory[LedgerTax]
	var other money.Cents
	for category, amount := range byCategory {
		switch category {
		case LedgerEnergy, LedgerToll, LedgerParking, LedgerTravelOther, LedgerTires, LedgerMaintenance, LedgerRepair,
			LedgerInsurance, LedgerFinancing, LedgerTax, LedgerSubscr, LedgerAcquisition:
		default:
			other += amount
		}
	}
	otherTotal := subscription + tax + other

	running := energy + travel + maintenance + repair + insurance + financing + otherTotal
	sum.TotalCost = running + tires
	// Economic financing cost: prepaid lease amounts spread over the contract, return fees and excess mileage accrued
	sum.FinancingFullCost = financing + owned.LeasePrepaidAdjustment + owned.LeaseEndFeesAdjustment + owned.LeaseExcessKm
	sum.FullCost = running - financing + sum.FinancingFullCost + sum.TiresAmortizedCost + sum.DepreciationCost
	sum.FullCostNet = sum.FullCost - sum.CarpoolRevenue

	sum.TotalDistanceKm = round1(trackedKm)
	sum.OdometerDistanceKm = round1(odometerSpan)
	sum.DistanceBasisKm = round1(basisKm)
	sum.TotalCostPerKm = perKm(sum.TotalCost, basisKm)
	sum.UsageCostPerKm = perKm(energy+travel, basisKm)
	sum.FullCostPerKm = perKm(sum.FullCost, basisKm)
	sum.FullCostNetPerKm = perKm(sum.FullCostNet, basisKm)
	sum.DepreciationCostPerKm = perKm(sum.DepreciationCost, basisKm)
	sum.EnergyCost = energy
	sum.EnergyCostPerKm = perKm(energy, basisKm)
	sum.TotalKwhAdded = round1(kwhAdded)
	if kwhPriced > 0 {
		sum.AvgCostPerKwh = round3(energy.Float() / kwhPriced)
	}
	sum.TollsCost = travel
	sum.TollsCostPerKm = perKm(travel, basisKm)
	sum.TiresCost = tires
	sum.TiresCostPerKm = perKm(tires, basisKm)
	sum.TiresAmortizedCostPerKm = perKm(sum.TiresAmortizedCost, basisKm)
	sum.MaintenanceCost = maintenance
	sum.MaintenanceCostPerKm = perKm(maintenance, basisKm)
	sum.RepairCost = repair
	sum.RepairCostPerKm = perKm(repair, basisKm)
	sum.InsuranceCost = insurance
	sum.InsuranceCostPerKm = perKm(insurance, basisKm)
	sum.FinancingCost = financing
	sum.FinancingCostPerKm = perKm(sum.FinancingFullCost, basisKm)
	sum.SubscriptionCost = subscription
	sum.TaxCost = tax
	sum.OtherCost = other
	sum.OtherCostPerKm = perKm(otherTotal, basisKm)

	if sum.TagBreakdown, err = s.tagBreakdown(ctx, vehicleID, trackedKm); err != nil {
		return nil, fmt.Errorf("tag breakdown: %w", err)
	}
	var totalSmoothedKm, totalEstimatedEnergyKm float64
	if sum.MonthlyCosts, totalSmoothedKm, totalEstimatedEnergyKm, err = s.monthlyCosts(ctx, vehicleID, ownership, currentOdometer, estKwh100km, estPricePerKwh, now); err != nil {
		return nil, fmt.Errorf("monthly costs: %w", err)
	}
	sum.SmoothedDistanceKm = totalSmoothedKm
	sum.EstimatedEnergyDistanceKm = totalEstimatedEnergyKm

	if estKwh100km != nil && *estKwh100km > 0 && estPricePerKwh != nil && *estPricePerKwh > 0 && totalEstimatedEnergyKm > 0 {
		sum.EstimatedEnergyKwh = round1(totalEstimatedEnergyKm * (*estKwh100km / 100.0))
		sum.EstimatedEnergyCost = money.FromFloat(sum.EstimatedEnergyKwh * *estPricePerKwh)
		sum.TotalKwhAdded = round1(sum.TotalKwhAdded + sum.EstimatedEnergyKwh)
		energy += sum.EstimatedEnergyCost
		sum.EnergyCost = energy
		sum.TotalCost += sum.EstimatedEnergyCost
		sum.FullCost += sum.EstimatedEnergyCost
		sum.FullCostNet += sum.EstimatedEnergyCost
		if sum.TotalKwhAdded > 0 {
			sum.AvgCostPerKwh = round3(sum.EnergyCost.Float() / sum.TotalKwhAdded)
		}
	}
	if comp.UntrackedDistanceKm > 0 {
		if sum.EstimatedEnergyCost > 0 {
			comp.Warnings = append(comp.Warnings, fmt.Sprintf(
				"%.0f km driven before tracking started: charges estimated and completed (%.1f kWh/100km at %.3f/kWh)", sum.EstimatedEnergyDistanceKm, *estKwh100km, *estPricePerKwh))
		} else {
			comp.Warnings = append(comp.Warnings, fmt.Sprintf(
				"%.0f km driven appear in no drive (before TeslaMate or TeslaMate offline): the cost per km uses the odometer distance", comp.UntrackedDistanceKm))
		}
	}
	if sum.SmoothedDistanceKm > 0 {
		if (trackedKm + sum.SmoothedDistanceKm) > basisKm+0.5 {
			basisKm = trackedKm + sum.SmoothedDistanceKm
		}
		sum.DistanceBasisKm = round1(basisKm)
		sum.TotalCostPerKm = perKm(sum.TotalCost, basisKm)
		sum.UsageCostPerKm = perKm(energy+travel, basisKm)
		sum.FullCostPerKm = perKm(sum.FullCost, basisKm)
		sum.FullCostNetPerKm = perKm(sum.FullCostNet, basisKm)
		sum.DepreciationCostPerKm = perKm(sum.DepreciationCost, basisKm)
		sum.EnergyCostPerKm = perKm(energy, basisKm)
		sum.TollsCostPerKm = perKm(travel, basisKm)
		sum.TiresCostPerKm = perKm(tires, basisKm)
		sum.TiresAmortizedCostPerKm = perKm(sum.TiresAmortizedCost, basisKm)
		sum.MaintenanceCostPerKm = perKm(maintenance, basisKm)
		sum.RepairCostPerKm = perKm(repair, basisKm)
		sum.InsuranceCostPerKm = perKm(insurance, basisKm)
		sum.FinancingCostPerKm = perKm(sum.FinancingFullCost, basisKm)
		sum.OtherCostPerKm = perKm(otherTotal, basisKm)
	}
	return sum, nil
}

func (s *TCOService) tagBreakdown(ctx context.Context, vehicleID string, totalDistance float64) ([]TagCostBreakdown, error) {
	rows, err := s.pool.Query(ctx, database.DriveTollAllocationCTE+`,
		per_drive AS (
			SELECT drive_id, SUM(allocated) AS amount FROM allocations GROUP BY drive_id
		)
		SELECT sub.tag,
		       COALESCE(SUM(sub.distance_km), 0),
		       COALESCE(SUM(sub.energy_consumed_kwh), 0),
		       COALESCE(SUM(pd.amount), 0)
		FROM (
			SELECT d.id,
			       unnest(CASE WHEN d.tags = '{}' OR d.tags IS NULL THEN ARRAY[''] ELSE d.tags END) AS tag,
			       d.distance_km, d.energy_consumed_kwh
			FROM drives d
			WHERE d.vehicle_id = $1 AND d.deleted_upstream_at IS NULL
		) sub
		LEFT JOIN per_drive pd ON pd.drive_id = sub.id
		GROUP BY sub.tag
		ORDER BY 2 DESC;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []TagCostBreakdown{}
	for rows.Next() {
		var tb TagCostBreakdown
		if err := rows.Scan(&tb.Tag, &tb.DistanceKm, &tb.EnergyKwh, &tb.TollsAmount); err != nil {
			return nil, err
		}
		if totalDistance > 0 {
			tb.Percentage = round1(tb.DistanceKm / totalDistance * 100)
		}
		tb.DistanceKm = round1(tb.DistanceKm)
		tb.EnergyKwh = round1(tb.EnergyKwh)
		list = append(list, tb)
	}
	return list, rows.Err()
}
