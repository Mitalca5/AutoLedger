package services

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// TariffService computes charging costs for time-of-use (HP/HC) and public charging schemes.
type TariffService struct {
	loc *time.Location
}

func NewTariffService() *TariffService {
	return &TariffService{loc: time.UTC}
}

// NewTariffServiceIn reads the time of day of a session in the given IANA timezone (APP_TIMEZONE): off-peak hours
// are wall-clock hours.
func NewTariffServiceIn(timezone string) *TariffService {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		slog.Warn("unknown timezone for tariffs, using UTC", "timezone", timezone, "error", err)
		loc = time.UTC
	}
	return &TariffService{loc: loc}
}

// DayOf is the calendar day (YYYY-MM-DD) of a moment in the service timezone, used to pick the version of a tariff.
func (s *TariffService) DayOf(t time.Time) string {
	return t.In(s.loc).Format("2006-01-02")
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

// effectiveGrid reads any plan as bands, rules and a default band: a flat plan is one band, a peak / off-peak plan
// two bands with a rule per off-peak window. ok is false when the plan carries no usable price.
func effectiveGrid(plan *models.TariffPlan) (bands map[string]money.Rate, rules []models.TariffRule, defaultBand string, ok bool) {
	bands = map[string]money.Rate{}
	if plan.PlanType == models.TariffTypeBands {
		for _, b := range plan.Bands {
			bands[b.Name] = b.RateCents
		}
		defaultBand = plan.DefaultBand
		if _, found := bands[defaultBand]; !found && len(plan.Bands) > 0 {
			defaultBand = plan.Bands[0].Name
		}
		return bands, plan.Rules, defaultBand, len(plan.Bands) > 0
	}

	if plan.PlanType == models.TariffTypeFlat || len(plan.TimeWindows) == 0 {
		switch {
		case plan.FlatRateCents != nil:
			bands["FLAT"] = *plan.FlatRateCents
		case plan.OffpeakRateCents != nil:
			bands["FLAT"] = *plan.OffpeakRateCents
		default:
			return bands, nil, "", false
		}
		return bands, nil, "FLAT", true
	}

	bands[models.TimeWindowPeak] = 0
	bands[models.TimeWindowOffPeak] = 0
	if plan.PeakRateCents != nil {
		bands[models.TimeWindowPeak] = *plan.PeakRateCents
	}
	if plan.OffpeakRateCents != nil {
		bands[models.TimeWindowOffPeak] = *plan.OffpeakRateCents
	}
	for _, w := range plan.TimeWindows {
		band := models.TimeWindowPeak
		if strings.ToUpper(strings.TrimSpace(w.Kind)) == models.TimeWindowOffPeak {
			band = models.TimeWindowOffPeak
		}
		rules = append(rules, models.TariffRule{Start: w.Start, End: w.End, Band: band})
	}
	return bands, rules, models.TimeWindowPeak, true
}

type parsedRule struct {
	days     [7]bool
	startMin int
	endMin   int
	band     string
}

func parseRules(rules []models.TariffRule) []parsedRule {
	parsed := make([]parsedRule, 0, len(rules))
	for _, r := range rules {
		sMin, err1 := parseTimeToMinutesOfDay(r.Start)
		eMin, err2 := parseTimeToMinutesOfDay(r.End)
		if strings.TrimSpace(r.End) == "24:00" {
			eMin, err2 = 1440, nil
		}
		if err1 != nil || err2 != nil {
			continue
		}
		pr := parsedRule{startMin: sMin, endMin: eMin, band: r.Band}
		if len(r.Days) == 0 {
			for i := range pr.days {
				pr.days[i] = true
			}
		}
		for _, d := range r.Days {
			if d >= 0 && d <= 6 {
				pr.days[d] = true
			}
		}
		parsed = append(parsed, pr)
	}
	return parsed
}

// matches tells whether the rule covers the given weekday and minute of the day.
func (r parsedRule) matches(weekday time.Weekday, mod int) bool {
	if r.startMin == r.endMin {
		return r.days[weekday]
	}
	if r.startMin < r.endMin {
		return r.days[weekday] && mod >= r.startMin && mod < r.endMin
	}
	// Crosses midnight: the evening part belongs to the day it starts on, the morning part to the day before.
	previous := (int(weekday) + 6) % 7
	return (r.days[weekday] && mod >= r.startMin) || (r.days[previous] && mod < r.endMin)
}

// CalculateSessionCost splits the energy of a session between the bands in force while it ran, in proportion to
// the time spent in each, and returns the cost in cents. The clock is read in the service timezone, so a window
// follows the wall clock across daylight-saving changes. A plan whose validity range excludes the day of the
// session prices nothing.
func (s *TariffService) CalculateSessionCost(plan *models.TariffPlan, startTime, endTime time.Time, kwh float64) (money.Cents, error) {
	if plan == nil || kwh <= 0 {
		return 0, nil
	}
	startTime = startTime.In(s.loc)
	if !plan.AppliesOn(startTime.Format("2006-01-02")) {
		return 0, nil
	}

	bands, rules, defaultBand, ok := effectiveGrid(plan)
	if !ok {
		return 0, nil
	}
	if len(rules) == 0 {
		return bands[defaultBand].Cost(kwh), nil
	}
	parsed := parseRules(rules)

	if !endTime.After(startTime) {
		endTime = startTime.Add(1 * time.Minute)
	}
	totalMinutes := int(math.Max(1, math.Round(endTime.Sub(startTime).Minutes())))

	// Sessions longer than a day are sampled every 5 minutes to bound the loop
	step := 1
	if totalMinutes > 1440 {
		step = 5
	}

	minutesByBand := map[string]int{}
	evaluated := 0
	for m := 0; m < totalMinutes; m += step {
		curr := startTime.Add(time.Duration(m) * time.Minute)
		mod := curr.Hour()*60 + curr.Minute()
		band := defaultBand
		for _, r := range parsed {
			if r.matches(curr.Weekday(), mod) {
				band = r.band
				break
			}
		}
		minutesByBand[band] += step
		evaluated += step
	}

	cost := 0.0
	for band, minutes := range minutesByBand {
		cost += kwh * float64(minutes) / float64(evaluated) * bands[band].Float()
	}
	return money.FromFloat(cost), nil
}

// CalculatePublicCharging computes decomposed public charging fees (connection + energy + time + idle).
func (s *TariffService) CalculatePublicCharging(req models.PublicChargingCalculationRequest) models.PublicChargingBreakdown {
	energyCost := req.PricePerKwh.Cost(req.Kwh)
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
