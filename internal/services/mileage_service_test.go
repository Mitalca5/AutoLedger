package services

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestAllowanceForSlicePricesOnlyTheDistanceInEachSlice(t *testing.T) {
	to5000 := 5000
	to20000 := 20000
	scale := []models.MileageRate{
		{FromKm: 0, ToKm: &to5000, RatePerKm: 0.5},
		{FromKm: 5000, ToKm: &to20000, RatePerKm: 0.3},
		{FromKm: 20000, RatePerKm: 0.1},
	}
	cases := []struct {
		name       string
		start, end float64
		want       float64
	}{
		{"inside the first slice", 0, 1000, 500},
		{"across two slices", 4000, 6000, 1000*0.5 + 1000*0.3},
		{"across all three", 4000, 21000, 1000*0.5 + 15000*0.3 + 1000*0.1},
		{"beyond the open-ended slice", 30000, 31000, 100},
		{"empty range", 100, 100, 0},
	}
	for _, c := range cases {
		if got := AllowanceForSlice(scale, c.start, c.end); math.Abs(got-c.want) > 1e-6 {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if got := AllowanceForSlice(nil, 0, 1000); got != 0 {
		t.Errorf("no scale should give no allowance, got %v", got)
	}
}

type fakeMileageStore struct {
	drives   []models.Drive
	rates    []models.MileageRate
	tolls    map[string]money.Cents
	rateErr  error
	tollIDs  []string
	filter   database.DriveFilter
	ratesFor string
}

func (f *fakeMileageStore) ListDrives(_ context.Context, _ string, filter database.DriveFilter, _, _ int) ([]models.Drive, int, error) {
	f.filter = filter
	return f.drives, len(f.drives), nil
}

func (f *fakeMileageStore) ListMileageRates(_ context.Context, userID string) ([]models.MileageRate, error) {
	f.ratesFor = userID
	return f.rates, f.rateErr
}

func (f *fakeMileageStore) GetTollExpensesForDrives(_ context.Context, _ string, ids []string) (map[string]money.Cents, error) {
	f.tollIDs = ids
	return f.tolls, nil
}

func TestMileageReportGroupsByTagAndAppliesTheScaleFromTheYearStart(t *testing.T) {
	to10000 := 10000
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 12, 0, 0, 0, time.UTC) }
	store := &fakeMileageStore{
		drives: []models.Drive{
			{ID: "before", StartTime: day(1, 10), DistanceKm: 9000, Tags: []string{"work"}},
			{ID: "a", StartTime: day(3, 5), DistanceKm: 2000, Tags: []string{"work"}},
			{ID: "b", StartTime: day(3, 6), DistanceKm: 300},
			{ID: "both", StartTime: day(3, 7), DistanceKm: 100, Tags: []string{"work", "club"}},
		},
		rates: []models.MileageRate{
			{Label: "scale", Year: 2026, FromKm: 0, ToKm: &to10000, RatePerKm: 0.5},
			{Label: "scale", Year: 2026, FromKm: 10000, RatePerKm: 0.25},
			{Label: "other", Year: 2026, FromKm: 0, RatePerKm: 9},
		},
		tolls: map[string]money.Cents{"a": 500, "both": 250},
	}
	svc := NewMileageService(store, "UTC")
	from := day(3, 1)
	report, err := svc.Report(context.Background(), &models.Vehicle{ID: "v", Currency: "EUR"}, "u1", MileageOptions{From: &from, RateLabel: "scale"})
	if err != nil {
		t.Fatal(err)
	}
	if store.ratesFor != "u1" || report.Currency != "EUR" || report.From != "2026-03-01" || report.To != "" {
		t.Fatalf("unexpected report header %+v (rates for %q)", report, store.ratesFor)
	}
	if want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC); !store.filter.From.Equal(want) {
		t.Fatalf("drives must be read from the start of the year, got %v", store.filter.From)
	}
	if len(store.tollIDs) != 3 {
		t.Fatalf("tolls must only be read for the trips of the period, got %v", store.tollIDs)
	}
	if len(report.Tags) != 3 {
		t.Fatalf("expected work, untagged and club, got %+v", report.Tags)
	}
	work, untagged, club := report.Tags[0], report.Tags[1], report.Tags[2]
	if work.Tag != "work" || work.Trips != 2 || work.DistanceKm != 2100 || work.Tolls != 750 {
		t.Fatalf("unexpected work total %+v", work)
	}
	// 9,000 km already driven before the period under this tag: 1,000 km at 0.50 then 1,100 km at 0.25.
	if work.Allowance == nil || *work.Allowance != 77500 {
		t.Fatalf("expected the scale to continue from the year's earlier distance, got %v", work.Allowance)
	}
	if untagged.Tag != "" || untagged.DistanceKm != 300 || club.Tag != "club" || club.DistanceKm != 100 {
		t.Fatalf("unexpected order or totals: %+v", report.Tags)
	}
}

func TestMileageReportWithoutScaleOrWithTagFilter(t *testing.T) {
	store := &fakeMileageStore{
		drives: []models.Drive{
			{ID: "a", StartTime: time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC), DistanceKm: 12.34, Tags: []string{"work"}},
			{ID: "b", StartTime: time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC), DistanceKm: 50},
		},
	}
	report, err := NewMileageService(store, "UTC").Report(context.Background(), &models.Vehicle{ID: "v"}, "u", MileageOptions{Tag: "work"})
	if err != nil {
		t.Fatal(err)
	}
	if store.ratesFor != "" {
		t.Fatal("the scales are only read when one is chosen")
	}
	if len(report.Tags) != 1 || report.Tags[0].Tag != "work" || report.Tags[0].Allowance != nil || report.Tags[0].DistanceKm != 12.3 {
		t.Fatalf("unexpected report %+v", report.Tags)
	}
}

func TestMileageReportReturnsStoreErrors(t *testing.T) {
	boom := errors.New("boom")
	store := &fakeMileageStore{rateErr: boom}
	if _, err := NewMileageService(store, "UTC").Report(context.Background(), &models.Vehicle{ID: "v"}, "u", MileageOptions{RateLabel: "x"}); !errors.Is(err, boom) {
		t.Fatalf("expected the store error, got %v", err)
	}
}
