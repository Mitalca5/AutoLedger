package services

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// TariffService computes charging costs for time-of-use (HP/HC) and public charging schemes.
type TariffService struct{}

func NewTariffService() *TariffService {
	return &TariffService{}
}

// parseTimeToMinutesOfDay converts "HH:MM" into minutes from 00:00 (0..1439).
func parseTimeToMinutesOfDay(timeStr string) (int, error) {
	parts := strings.Split(strings.TrimSpace(timeStr), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time format %q, expected HH:MM", timeStr)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("invalid hour %q", parts[0])
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid minute %q", parts[1])
	}
	return h*60 + m, nil
}

// isMinuteInWindow returns true if a minute of the day falls within a time window.
func isMinuteInWindow(mod int, startMin, endMin int) bool {
	if startMin <= endMin {
		return mod >= startMin && mod < endMin
	}
	// Window crosses midnight (e.g. 22:00 -> 06:00)
	return mod >= startMin || mod < endMin
}

// CalculateSessionCost splits charging energy between peak and off-peak windows and returns total cost in cents.
func (s *TariffService) CalculateSessionCost(plan *models.TariffPlan, startTime, endTime time.Time, kwh float64) (money.Cents, error) {
	if plan == nil || kwh <= 0 {
		return 0, nil
	}

	if plan.PlanType == models.TariffTypeFlat || len(plan.TimeWindows) == 0 {
		if plan.FlatRateCents != nil {
			return money.Cents(math.Round(float64(*plan.FlatRateCents) * kwh)), nil
		}
		if plan.OffpeakRateCents != nil {
			return money.Cents(math.Round(float64(*plan.OffpeakRateCents) * kwh)), nil
		}
		return 0, nil
	}

	// Time of Use calculation
	if endTime.Before(startTime) || endTime.Equal(startTime) {
		endTime = startTime.Add(1 * time.Minute)
	}

	totalDuration := endTime.Sub(startTime)
	totalMinutes := int(math.Max(1, math.Round(totalDuration.Minutes())))

	// Parse configured windows
	type parsedWindow struct {
		startMin int
		endMin   int
		kind     string
	}
	windows := make([]parsedWindow, 0, len(plan.TimeWindows))
	for _, w := range plan.TimeWindows {
		sMin, err1 := parseTimeToMinutesOfDay(w.Start)
		eMin, err2 := parseTimeToMinutesOfDay(w.End)
		if err1 == nil && err2 == nil {
			windows = append(windows, parsedWindow{
				startMin: sMin,
				endMin:   eMin,
				kind:     strings.ToUpper(strings.TrimSpace(w.Kind)),
			})
		}
	}

	offpeakMinutes := 0
	peakMinutes := 0

	// Step minute by minute through session (capped to 2880 mins / 48h to prevent excessive loops)
	stepMinutes := 1
	if totalMinutes > 1440 {
		stepMinutes = 5 // optimize for very long sessions
	}

	for m := 0; m < totalMinutes; m += stepMinutes {
		curr := startTime.Add(time.Duration(m) * time.Minute)
		mod := curr.Hour()*60 + curr.Minute()

		matchedKind := models.TimeWindowPeak
		for _, w := range windows {
			if isMinuteInWindow(mod, w.startMin, w.endMin) {
				matchedKind = w.kind
				break
			}
		}

		if matchedKind == models.TimeWindowOffPeak {
			offpeakMinutes += stepMinutes
		} else {
			peakMinutes += stepMinutes
		}
	}

	evaluatedMinutes := offpeakMinutes + peakMinutes
	if evaluatedMinutes == 0 {
		evaluatedMinutes = 1
		peakMinutes = 1
	}

	peakRatio := float64(peakMinutes) / float64(evaluatedMinutes)
	offpeakRatio := float64(offpeakMinutes) / float64(evaluatedMinutes)

	peakKwh := kwh * peakRatio
	offpeakKwh := kwh * offpeakRatio

	peakRate := int64(0)
	if plan.PeakRateCents != nil {
		peakRate = int64(*plan.PeakRateCents)
	}
	offpeakRate := int64(0)
	if plan.OffpeakRateCents != nil {
		offpeakRate = int64(*plan.OffpeakRateCents)
	}

	cost := math.Round(peakKwh*float64(peakRate) + offpeakKwh*float64(offpeakRate))
	return money.Cents(cost), nil
}

// CalculatePublicCharging computes decomposed public charging fees (connection + energy + time + idle).
func (s *TariffService) CalculatePublicCharging(req models.PublicChargingCalculationRequest) models.PublicChargingBreakdown {
	energyCost := money.Cents(math.Round(req.Kwh * float64(req.PricePerKwh)))
	durationCost := money.Cents(int64(req.ChargingMinutes) * int64(req.PricePerMinute))

	totalPlugged := req.TotalPluggedMinutes
	if totalPlugged < req.ChargingMinutes {
		totalPlugged = req.ChargingMinutes
	}
	idleMinutes := totalPlugged - req.ChargingMinutes
	billableIdle := 0
	if idleMinutes > req.IdleGraceMinutes {
		billableIdle = idleMinutes - req.IdleGraceMinutes
	}
	idleCost := money.Cents(int64(billableIdle) * int64(req.IdleFeePerMinute))

	totalCost := req.ConnectionFee + energyCost + durationCost + idleCost

	return models.PublicChargingBreakdown{
		ConnectionCost: req.ConnectionFee,
		EnergyCost:     energyCost,
		DurationCost:   durationCost,
		IdleMinutes:    idleMinutes,
		IdleCost:       idleCost,
		TotalCost:      totalCost,
	}
}
