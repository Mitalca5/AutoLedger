package services

import (
	"context"
	"log/slog"
	"math"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
)

// TireWearStats provides detailed wear analytics for a tire.
type TireWearStats struct {
	Tire                 models.Tire               `json:"tire"`
	CurrentDepthMm       float64                   `json:"current_depth_mm"`
	InitialDepthMm       float64                   `json:"initial_depth_mm"`
	MinLegalDepthMm      float64                   `json:"min_legal_depth_mm"`
	UsableDepthMm        float64                   `json:"usable_depth_mm"`
	RemainingDepthMm     float64                   `json:"remaining_depth_mm"`
	WearPercentage       float64                   `json:"wear_percentage"`
	DistanceTraveledKm   float64                   `json:"distance_traveled_km"`
	TotalDistanceKm      float64                   `json:"total_distance_km"`
	CurrentRunKm         float64                   `json:"current_run_km"` // Driven since the open mount session started
	EstimatedLifespanKm  int                       `json:"estimated_lifespan_km"`
	LifeProgressPct      float64                   `json:"life_progress_pct"`
	CostPerKm            float64                   `json:"cost_per_km"`
	WearRatePer10kKm     float64                   `json:"wear_rate_per_10k_km"`
	EstimatedRemainingKm float64                   `json:"estimated_remaining_km"`
	Condition            string                    `json:"condition"` // "GOOD", "WARNING", "CRITICAL"
	LogsCount            int                       `json:"logs_count"`
	Sessions             []models.TireMountSession `json:"sessions"`

	// Dynamic TeslaMate driving telemetry analytics
	DrivesCount            int               `json:"drives_count"`
	DrivingStressIndex     float64           `json:"driving_stress_index"`
	DrivingStyle           string            `json:"driving_style"` // "ECO", "BALANCED", "SPORT"
	AvgPowerMaxKw          float64           `json:"avg_power_max_kw"`
	AvgPowerMinKw          float64           `json:"avg_power_min_kw"`
	AvgConsumptionKwh100km float64           `json:"avg_consumption_kwh_100km"`
	DynamicLifespanKm      int               `json:"dynamic_lifespan_km"`
	DynamicRemainingKm     float64           `json:"dynamic_remaining_km"`
	WearExplanation        *apierror.Message `json:"wear_explanation,omitempty"`
}

// tireWearStore is the narrow slice of *database.Repository that TireWearService actually
// needs. Consumer-defined so tests can supply a fake without a real database (mirrors the
// syncStore interface in sync_service.go).
type tireWearStore interface {
	ListTireLogs(ctx context.Context, tireID string) ([]models.TireLog, error)
	ListTireMountSessions(ctx context.Context, tireID string) ([]models.TireMountSession, error)
	GetDrivingTelemetryStats(ctx context.Context, vehicleID string, ranges []database.OdometerRange) (avgPowerMax, avgPowerMin, avgConsumption float64, count int, err error)
}

// TireWearService calculates wear projections and stats for tires.
type TireWearService struct {
	repo tireWearStore
}

// NewTireWearService creates a new TireWearService.
func NewTireWearService(repo tireWearStore) *TireWearService {
	return &TireWearService{repo: repo}
}

// TireDistanceAtOdometer returns the distance driven by a tire (mount sessions only) when the vehicle
// odometer read odometer. Periods in storage do not count.
func TireDistanceAtOdometer(sessions []models.TireMountSession, odometer float64) float64 {
	total := 0.0
	for _, s := range sessions {
		end := odometer
		if s.DismountedOdometer != nil && *s.DismountedOdometer < end {
			end = *s.DismountedOdometer
		}
		if end > s.MountedOdometer {
			total += end - s.MountedOdometer
		}
	}
	return total
}

// CalculateTireWear computes wear metrics based on depth logs, mount sessions, and TeslaMate power telemetry.
func (s *TireWearService) CalculateTireWear(ctx context.Context, tire *models.Tire, vehicleCurrentOdometer float64) (*TireWearStats, error) {
	logs, err := s.repo.ListTireLogs(ctx, tire.ID)
	if err != nil {
		return nil, err
	}

	sessions, err := s.repo.ListTireMountSessions(ctx, tire.ID)
	if err != nil {
		return nil, err
	}

	initialDepth := tire.InitialDepthMm
	if initialDepth <= 0 {
		initialDepth = 8.0 // standard default for passenger car tire
	}

	minLegal := tire.MinLegalDepthMm
	if minLegal <= 0 {
		minLegal = 1.6 // EU legal minimum
	}

	usableDepth := math.Max(0.1, initialDepth-minLegal)
	currentDepth := initialDepth
	distanceTraveled := 0.0
	measuredWear := 0.0

	if len(logs) > 0 {
		// Latest log is first due to ORDER BY date DESC
		latestLog := logs[0]
		currentDepth = latestLog.DepthMm

		// Oldest log is last. Wear is measured against the distance driven by this tire only
		// (vehicle odometer readings converted through mount sessions, storage periods excluded).
		oldestLog := logs[len(logs)-1]
		if len(logs) >= 2 {
			distanceTraveled = math.Max(0, TireDistanceAtOdometer(sessions, latestLog.Odometer)-TireDistanceAtOdometer(sessions, oldestLog.Odometer))
			measuredWear = math.Max(0, oldestLog.DepthMm-latestLog.DepthMm)
		} else {
			// Single measure: wear since new over the tire's whole life at that reading.
			distanceTraveled = tire.InitialDistanceKm + TireDistanceAtOdometer(sessions, latestLog.Odometer)
			measuredWear = math.Max(0, initialDepth-latestLog.DepthMm)
		}
	}

	// Calculate lifetime distance
	currentRunKm := 0.0
	isMounted := tire.CurrentPosition != models.TirePosStorage && tire.CurrentPosition != models.TirePosDisposed
	if isMounted && tire.MountedOdometer != nil && vehicleCurrentOdometer > *tire.MountedOdometer {
		currentRunKm = vehicleCurrentOdometer - *tire.MountedOdometer
	}
	totalDistance := tire.AccumulatedDistanceKm + currentRunKm

	// Update active session distance in-memory for display
	for i := range sessions {
		if sessions[i].DismountedDate == nil && isMounted {
			sessions[i].DistanceKm = math.Round(currentRunKm*10) / 10
		}
	}

	lifespan := tire.EstimatedLifespanKm
	if lifespan <= 0 {
		lifespan = 40000
	}
	lifeProgressPct := math.Min(100.0, math.Round((totalDistance/float64(lifespan))*1000)/10)
	costPerKm := math.Round((tire.PurchasePrice.Float()/float64(lifespan))*10000) / 10000

	wornDepth := math.Max(0, initialDepth-currentDepth)
	remainingDepth := math.Max(0, currentDepth-minLegal)
	wearPct := math.Min(100.0, (wornDepth/usableDepth)*100.0)

	wearRatePer10k := 0.0
	estimatedRemainingKm := 0.0

	if distanceTraveled > 500 && measuredWear > 0.05 {
		wearRatePer10k = (measuredWear / distanceTraveled) * 10000.0
		if wearRatePer10k > 0 {
			estimatedRemainingKm = (remainingDepth / wearRatePer10k) * 10000.0
		}
	} else {
		// Fallback estimation using EV average wear rate (approx 1.2 mm / 10,000 km)
		wearRatePer10k = 1.2
		estimatedRemainingKm = (remainingDepth / 1.2) * 10000.0
	}

	// TeslaMate dynamic telemetry & power stress calculation.
	// Only drives performed while the tire was physically mounted are considered.
	// Each mount session provides an odometer window [MountedOdometer, DismountedOdometer].
	var avgPowerMax, avgPowerMin, avgConsumption float64
	var drivesCount int
	if tire.VehicleID != nil && *tire.VehicleID != "" {
		var ranges []database.OdometerRange
		if len(sessions) > 0 {
			ranges = make([]database.OdometerRange, 0, len(sessions))
			for _, sess := range sessions {
				ranges = append(ranges, database.OdometerRange{
					Min: sess.MountedOdometer,
					Max: sess.DismountedOdometer,
				})
			}
		} else if isMounted && tire.MountedOdometer != nil {
			ranges = []database.OdometerRange{
				{
					Min: *tire.MountedOdometer,
					Max: nil,
				},
			}
		}

		if len(ranges) > 0 {
			var errTelemetry error
			avgPowerMax, avgPowerMin, avgConsumption, drivesCount, errTelemetry = s.repo.GetDrivingTelemetryStats(ctx, *tire.VehicleID, ranges)
			if errTelemetry != nil {
				slog.Error("failed to get driving telemetry stats", "component", "tire-wear", "tire_id", tire.ID, "error", errTelemetry)
			}
		}
	}

	accelFactor := 1.0
	if avgPowerMax > 80 {
		accelFactor = 1.0 + (avgPowerMax-80.0)*0.002
	} else if avgPowerMax > 0 {
		accelFactor = 0.90 + (avgPowerMax/80.0)*0.10
	}

	absPowerMin := math.Abs(avgPowerMin)
	regenFactor := 1.0
	if absPowerMin > 35 {
		regenFactor = 1.0 + (absPowerMin-35.0)*0.003
	} else if absPowerMin > 0 {
		regenFactor = 0.95 + (absPowerMin/35.0)*0.05
	}

	consumptionFactor := 1.0
	if avgConsumption > 0 {
		consumptionFactor = math.Max(0.85, math.Min(1.35, avgConsumption/16.0))
	}

	positionWeight := 1.0
	if tire.CurrentPosition == models.TirePosRL || tire.CurrentPosition == models.TirePosRR {
		positionWeight = 1.15 // Rear axle takes major acceleration torque and regen torque on Tesla
	} else if tire.CurrentPosition == models.TirePosFL || tire.CurrentPosition == models.TirePosFR {
		positionWeight = 0.92 // Front axle takes steering and braking transfer
	}

	var stressIndex float64
	var drivingStyle string
	var dynamicLifespan = lifespan
	var dynamicRemainingKm = estimatedRemainingKm
	var wearExplanation *apierror.Message

	if drivesCount > 0 {
		stressIndex = (0.45*accelFactor + 0.30*regenFactor + 0.25*consumptionFactor) * positionWeight
		stressIndex = math.Max(0.75, math.Min(1.60, stressIndex))
		stressIndex = math.Round(stressIndex*100) / 100

		if stressIndex <= 0.93 {
			drivingStyle = "ECO"
		} else if stressIndex >= 1.12 {
			drivingStyle = "SPORT"
		} else {
			drivingStyle = "BALANCED"
		}

		dynamicLifespan = int(math.Round(float64(lifespan) / stressIndex))
		dynamicWearRatePer10k := wearRatePer10k * stressIndex
		if dynamicWearRatePer10k > 0 {
			dynamicRemainingKm = math.Max(0, (remainingDepth/dynamicWearRatePer10k)*10000.0)
		}

		style := "kw:balanced"
		switch drivingStyle {
		case "ECO":
			style = "kw:eco"
		case "SPORT":
			style = "kw:sport"
		}

		axle := "kw:any_axle"
		if positionWeight > 1.0 {
			axle = "kw:rear_axle"
		} else if positionWeight < 1.0 {
			axle = "kw:front_axle"
		}

		// Parameters: style and axle are keywords the front end translates.
		wearExplanation = apierror.NewMessagef("tire.wear_explanation",
			"%s driving, %s axle: average power peaks of +%.0f kW and %.0f kW in regeneration, consumption %.1f kWh/100km. Stress index: x%.2f (estimated lifespan adjusted to ~%.0f km).",
			style, axle, avgPowerMax, avgPowerMin, apierror.PerKm(avgConsumption), stressIndex, apierror.Km(dynamicLifespan),
		)
	}

	condition := "GOOD"
	if currentDepth <= 2.5 {
		condition = "CRITICAL"
	} else if currentDepth <= 4.0 {
		condition = "WARNING"
	}

	return &TireWearStats{
		Tire:                   *tire,
		CurrentDepthMm:         math.Round(currentDepth*10) / 10,
		InitialDepthMm:         initialDepth,
		MinLegalDepthMm:        minLegal,
		UsableDepthMm:          math.Round(usableDepth*10) / 10,
		RemainingDepthMm:       math.Round(remainingDepth*10) / 10,
		WearPercentage:         math.Round(wearPct*10) / 10,
		DistanceTraveledKm:     math.Round(distanceTraveled),
		TotalDistanceKm:        math.Round(totalDistance*10) / 10,
		CurrentRunKm:           math.Round(currentRunKm*10) / 10,
		EstimatedLifespanKm:    lifespan,
		LifeProgressPct:        lifeProgressPct,
		CostPerKm:              costPerKm,
		WearRatePer10kKm:       math.Round(wearRatePer10k*100) / 100,
		EstimatedRemainingKm:   math.Round(estimatedRemainingKm),
		Condition:              condition,
		LogsCount:              len(logs),
		Sessions:               sessions,
		DrivesCount:            drivesCount,
		DrivingStressIndex:     stressIndex,
		DrivingStyle:           drivingStyle,
		AvgPowerMaxKw:          math.Round(avgPowerMax),
		AvgPowerMinKw:          math.Round(avgPowerMin),
		AvgConsumptionKwh100km: math.Round(avgConsumption*10) / 10,
		DynamicLifespanKm:      dynamicLifespan,
		DynamicRemainingKm:     math.Round(dynamicRemainingKm),
		WearExplanation:        wearExplanation,
	}, nil
}
