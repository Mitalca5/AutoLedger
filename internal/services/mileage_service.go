package services

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// MileageService totals trips per tag and applies a mileage allowance scale the user typed.
type MileageService struct {
	repo *database.Repository
	loc  *time.Location
}

func NewMileageService(repo *database.Repository, timezone string) *MileageService {
	loc, err := time.LoadLocation(timezone)
	if err != nil || timezone == "" {
		loc = time.UTC
	}
	return &MileageService{repo: repo, loc: loc}
}

// MileageOptions selects the period (nil bounds are open), an optional tag and the scale label ("" for none).
type MileageOptions struct {
	From      *time.Time
	To        *time.Time
	Tag       string
	RateLabel string
}

// AllowanceForSlice prices the cumulative distance between startKm and endKm of one year with the scale's slices.
func AllowanceForSlice(slices []models.MileageRate, startKm, endKm float64) float64 {
	total := 0.0
	for _, s := range slices {
		lo := math.Max(float64(s.FromKm), startKm)
		hi := endKm
		if s.ToKm != nil {
			hi = math.Min(float64(*s.ToKm), endKm)
		}
		if hi > lo {
			total += (hi - lo) * s.RatePerKm
		}
	}
	return total
}

type tagYear struct {
	tag  string
	year int
}

func (s *MileageService) Report(ctx context.Context, vehicle *models.Vehicle, userID string, opts MileageOptions) (*models.MileageReport, error) {
	filter := database.DriveFilter{Tag: opts.Tag, To: opts.To}
	if opts.From != nil {
		start := time.Date(opts.From.In(s.loc).Year(), 1, 1, 0, 0, 0, 0, s.loc)
		filter.From = &start
	}
	drives, _, err := s.repo.ListDrives(ctx, vehicle.ID, filter, math.MaxInt32, 0)
	if err != nil {
		return nil, err
	}

	var slices map[int][]models.MileageRate
	if opts.RateLabel != "" {
		rates, err := s.repo.ListMileageRates(ctx, userID)
		if err != nil {
			return nil, err
		}
		slices = map[int][]models.MileageRate{}
		for _, r := range rates {
			if r.Label == opts.RateLabel {
				slices[r.Year] = append(slices[r.Year], r)
			}
		}
	}

	inPeriod := func(t time.Time) bool {
		return (opts.From == nil || !t.Before(*opts.From)) && (opts.To == nil || !t.After(*opts.To))
	}
	var periodIDs []string
	for _, d := range drives {
		if inPeriod(d.StartTime) {
			periodIDs = append(periodIDs, d.ID)
		}
	}
	tolls, err := s.repo.GetTollExpensesForDrives(ctx, vehicle.ID, periodIDs)
	if err != nil {
		return nil, err
	}

	type acc struct {
		trips          int
		km             float64
		before, within map[int]float64
		tollCents      money.Cents
	}
	byTag := map[string]*acc{}
	for _, d := range drives {
		year := d.StartTime.In(s.loc).Year()
		tags := d.Tags
		if len(tags) == 0 {
			tags = []string{""}
		}
		for _, tag := range tags {
			if opts.Tag != "" && tag != opts.Tag {
				continue
			}
			a := byTag[tag]
			if a == nil {
				a = &acc{before: map[int]float64{}, within: map[int]float64{}}
				byTag[tag] = a
			}
			if !inPeriod(d.StartTime) {
				a.before[year] += d.DistanceKm
				continue
			}
			a.trips++
			a.km += d.DistanceKm
			a.within[year] += d.DistanceKm
			a.tollCents += tolls[d.ID]
		}
	}

	report := &models.MileageReport{RateLabel: opts.RateLabel, Currency: vehicle.Currency, Tags: []models.MileageTagTotal{}}
	if opts.From != nil {
		report.From = opts.From.Format("2006-01-02")
	}
	if opts.To != nil {
		report.To = opts.To.Format("2006-01-02")
	}
	for tag, a := range byTag {
		if a.trips == 0 {
			continue
		}
		row := models.MileageTagTotal{Tag: tag, Trips: a.trips, DistanceKm: math.Round(a.km*10) / 10, Tolls: a.tollCents}
		if opts.RateLabel != "" {
			amount := 0.0
			for year, km := range a.within {
				start := a.before[year]
				amount += AllowanceForSlice(slices[year], start, start+km)
			}
			c := money.Cents(math.Round(amount * 100))
			row.Allowance = &c
		}
		report.Tags = append(report.Tags, row)
	}
	sort.Slice(report.Tags, func(i, j int) bool {
		if report.Tags[i].DistanceKm != report.Tags[j].DistanceKm {
			return report.Tags[i].DistanceKm > report.Tags[j].DistanceKm
		}
		return report.Tags[i].Tag < report.Tags[j].Tag
	})
	return report, nil
}
