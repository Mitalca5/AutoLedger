package services

import (
	"sort"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

// Sources of the months a tire is expected on the car.
const (
	TireForecastMonthsLearned = "learned"  // read from the past mount sessions of tires of the same season
	TireForecastMonthsDefault = "default"  // typical season windows, no usable history
	TireForecastMonthsAllYear = "all_year" // no tire of another season to swap with: the set stays on the car
)

const (
	tireForecastHorizonYears = 10
	tireForecastWindowMonths = 12
	tireForecastMinWindow    = 30 * 24 * time.Hour
)

// TireForecast predicts when a tire reaches its legal wear limit.
type TireForecast struct {
	// ReplacementDate is the first day the tire is expected to be at its limit (YYYY-MM-DD); empty when it cannot be predicted.
	ReplacementDate string `json:"replacement_date,omitempty"`
	// MonthlyKm is the vehicle's average distance per month over the months the tire is on the car.
	MonthlyKm float64 `json:"monthly_km"`
	// MountedMonths lists the calendar months (1-12) the tire is expected on the car.
	MountedMonths []int `json:"mounted_months"`
	// MonthsSource tells whether MountedMonths was learned from history or is the default for the season.
	MonthsSource string `json:"months_source"`
	// WearBasis tells where the wear rate behind RemainingKm comes from: "measured" (depth logs) or "default" (average EV wear).
	WearBasis string `json:"wear_basis,omitempty"`
}

// TireForecastInput is everything the projection needs; it holds no database access so it stays unit-testable.
type TireForecastInput struct {
	Now         time.Time
	RemainingKm float64
	Season      models.TireSeason
	// SeasonSessions are the mount sessions of every tire of this season on the vehicle, the tire's own included.
	SeasonSessions []models.TireMountSession
	// KeptAllYear is set when the vehicle has no tire to swap this one with (see KeptOnAllYear).
	KeptAllYear bool
	Anchors     []OdometerAnchor
}

// KeptOnAllYear tells whether a seasonal set is presumably never swapped: the vehicle has no other seasonal set
// (winter for a summer one and the reverse) and no all-season tire. present lists the seasons of its tires in use.
func KeptOnAllYear(season models.TireSeason, present []models.TireSeason) bool {
	if season != models.TireSeasonSummer && season != models.TireSeasonWinter {
		return false
	}
	for _, p := range present {
		if p == models.TireSeasonAllSeason || (p != season && (p == models.TireSeasonSummer || p == models.TireSeasonWinter)) {
			return false
		}
	}
	return true
}

// defaultTireMonths gives the typical months a tire of a season is on the car in the northern hemisphere.
func defaultTireMonths(season models.TireSeason) [12]bool {
	var m [12]bool
	for i := range m {
		switch season {
		case models.TireSeasonWinter:
			m[i] = i >= 10 || i <= 2 // November to March
		case models.TireSeasonSummer:
			m[i] = i >= 3 && i <= 9 // April to October
		default:
			m[i] = true
		}
	}
	return m
}

// coveredMonths marks the calendar months between two dates (twelve at most).
func coveredMonths(from, to time.Time, months *[12]bool) {
	d := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 12 && !d.After(end); i++ {
		months[d.Month()-1] = true
		d = d.AddDate(0, 1, 0)
	}
}

// learnTireMonths reads the calendar months a set is on the car. Finished sessions give the season window (ok is
// false without any); a session still open adds the months it has already covered, so a set kept on through the
// year (no winter tires required in some regions) is recognised as it goes.
func learnTireMonths(sessions []models.TireMountSession, now time.Time) (closed, open [12]bool, ok bool) {
	for _, s := range sessions {
		switch {
		case s.DismountedDate == nil:
			if s.MountedDate.Before(now) {
				coveredMonths(s.MountedDate, now, &open)
			}
		case !s.DismountedDate.Before(s.MountedDate):
			ok = true
			coveredMonths(s.MountedDate, *s.DismountedDate, &closed)
		}
	}
	return closed, open, ok
}

// averageMonthlyKm is the distance driven per month over the last year, or over the history when it is shorter.
func averageMonthlyKm(anchors []OdometerAnchor, now time.Time) float64 {
	if len(anchors) == 0 {
		return 0
	}
	first := now
	for _, a := range anchors {
		if (a.Km > 0 || a.Origin) && a.Date.Before(first) {
			first = a.Date
		}
	}
	from := models.AddCalendarMonths(now, -tireForecastWindowMonths)
	if from.Before(first) {
		from = first
	}
	if now.Sub(from) < tireForecastMinWindow {
		return 0
	}
	end, _, okEnd := EstimateOdometerAt(anchors, now)
	start, _, okStart := EstimateOdometerAt(anchors, from)
	if !okEnd || !okStart || end <= start {
		return 0
	}
	months := now.Sub(from).Hours() / 24 / (365.0 / 12)
	return (end - start) / months
}

// monthlyKmProfile is the distance driven in each calendar month, averaged over the complete months of the last two
// years that the history covers. observed marks the months that have at least one.
func monthlyKmProfile(anchors []OdometerAnchor, now time.Time) (km [12]float64, observed [12]bool) {
	first := now
	for _, a := range anchors {
		if (a.Km > 0 || a.Origin) && a.Date.Before(first) {
			first = a.Date
		}
	}
	var sum [12]float64
	var n [12]int
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	for i := 1; i <= 24; i++ {
		start := thisMonth.AddDate(0, -i, 0)
		if start.Before(first) {
			break
		}
		a, _, okA := EstimateOdometerAt(anchors, start)
		b, _, okB := EstimateOdometerAt(anchors, start.AddDate(0, 1, 0))
		if !okA || !okB || b < a {
			continue
		}
		sum[start.Month()-1] += b - a
		n[start.Month()-1]++
	}
	for i := range km {
		if n[i] > 0 {
			km[i], observed[i] = sum[i]/float64(n[i]), true
		}
	}
	return km, observed
}

// monthlyKmWhileMounted gives the distance driven per calendar month, from the months the tire is on the car only
// (the mileage differs between winter and summer). A month never observed takes the average of the observed mounted
// months; with none of those, the average of the vehicle's last year. ok is false without any usable mileage.
func monthlyKmWhileMounted(anchors []OdometerAnchor, now time.Time, mounted [12]bool) (km [12]float64, avg float64, ok bool) {
	profile, observed := monthlyKmProfile(anchors, now)
	var sum, all float64
	var n, nAll int
	for i := range profile {
		if !observed[i] {
			continue
		}
		all += profile[i]
		nAll++
		if mounted[i] {
			sum += profile[i]
			n++
		}
	}
	switch {
	case n > 0:
		avg = sum / float64(n)
	case nAll > 0:
		avg = all / float64(nAll)
	default:
		avg = averageMonthlyKm(anchors, now)
	}
	if avg <= 0 {
		return km, 0, false
	}
	for i := range km {
		if observed[i] {
			km[i] = profile[i]
		} else {
			km[i] = avg
		}
	}
	return km, avg, true
}

// ForecastTireReplacement spreads the remaining distance of a tire over the months it is on the car, at the mileage
// the vehicle drives in each of those calendar months. Returns nil when the vehicle has no usable mileage history.
func ForecastTireReplacement(in TireForecastInput) *TireForecast {
	months, source := defaultTireMonths(in.Season), TireForecastMonthsDefault
	if in.Season != models.TireSeasonAllSeason {
		closed, open, hasClosed := learnTireMonths(in.SeasonSessions, in.Now)
		base := months
		switch {
		case hasClosed:
			base, source = closed, TireForecastMonthsLearned
		case in.KeptAllYear:
			base, source = defaultTireMonths(models.TireSeasonAllSeason), TireForecastMonthsAllYear
		}
		for i := range months {
			months[i] = base[i] || open[i]
			if open[i] && !base[i] {
				source = TireForecastMonthsLearned
			}
		}
	}
	perMonth, monthly, ok := monthlyKmWhileMounted(in.Anchors, in.Now, months)
	if !ok {
		return nil
	}
	f := &TireForecast{MonthlyKm: monthly, MonthsSource: source}
	for i, on := range months {
		if on {
			f.MountedMonths = append(f.MountedMonths, i+1)
		}
	}
	sort.Ints(f.MountedMonths)

	remaining := in.RemainingKm
	if remaining <= 0 {
		f.ReplacementDate = in.Now.Format("2006-01-02")
		return f
	}
	day := time.Date(in.Now.Year(), in.Now.Month(), in.Now.Day(), 0, 0, 0, 0, time.UTC)
	limit := day.AddDate(tireForecastHorizonYears, 0, 0)
	for ; day.Before(limit); day = day.AddDate(0, 0, 1) {
		if !months[day.Month()-1] {
			continue
		}
		daysInMonth := time.Date(day.Year(), day.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		remaining -= perMonth[day.Month()-1] / float64(daysInMonth)
		if remaining <= 0 {
			f.ReplacementDate = day.Format("2006-01-02")
			return f
		}
	}
	return f
}
