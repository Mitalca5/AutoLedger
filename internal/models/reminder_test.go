package models

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func scheduled(date, last *time.Time, yearly bool) *MaintenanceReminder {
	return &MaintenanceReminder{ScheduledDate: date, LastServiceDate: last, RepeatYearly: yearly, LeadDays: 15}
}

func TestComputeStatusScheduledDate(t *testing.T) {
	nov1 := day(2026, time.November, 1)

	cases := []struct {
		name      string
		rem       *MaintenanceReminder
		now       time.Time
		status    string
		remaining *int
		due       *time.Time
	}{
		{"far ahead", scheduled(nov1, nil, false), time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC), "OK", intp(46), nov1},
		{"inside the lead window", scheduled(nov1, nil, false), time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC), "DUE_SOON", intp(12), nov1},
		{"the due day is not overdue yet", scheduled(nov1, nil, false), time.Date(2026, 11, 1, 18, 30, 0, 0, time.UTC), "DUE_SOON", intp(0), nov1},
		{"day after", scheduled(nov1, nil, false), time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC), "OVERDUE", intp(-1), nov1},
		{"one-off done within the lead window", scheduled(nov1, day(2026, time.October, 25), false), time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC), "OK", nil, nil},
		{"yearly done early moves to next year", scheduled(nov1, day(2026, time.October, 25), true), time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC), "OK", intp(363), day(2027, time.November, 1)},
		{"yearly done late moves to next year", scheduled(nov1, day(2026, time.November, 5), true), time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC), "OK", intp(360), day(2027, time.November, 1)},
		{"completion far before the window leaves it pending", scheduled(nov1, day(2026, time.June, 1), true), time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC), "OK", intp(46), nov1},
		{"yearly never done stays overdue", scheduled(nov1, nil, true), time.Date(2027, 2, 1, 9, 0, 0, 0, time.UTC), "OVERDUE", intp(-92), nov1},
		{"yearly started before the first date is waiting", scheduled(day(2027, time.April, 1), day(2026, time.November, 3), true), time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC), "OK", intp(132), day(2027, time.April, 1)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.rem.ComputeStatus(0, c.now)
			if c.rem.Status != c.status {
				t.Errorf("status = %s, want %s", c.rem.Status, c.status)
			}
			switch {
			case c.remaining == nil && c.rem.RemainingDays != nil:
				t.Errorf("remaining days = %d, want none", *c.rem.RemainingDays)
			case c.remaining != nil && (c.rem.RemainingDays == nil || *c.rem.RemainingDays != *c.remaining):
				t.Errorf("remaining days = %v, want %d", c.rem.RemainingDays, *c.remaining)
			}
			switch {
			case c.due == nil && c.rem.DueDate != nil:
				t.Errorf("due date = %v, want none", c.rem.DueDate)
			case c.due != nil && (c.rem.DueDate == nil || !c.rem.DueDate.Equal(*c.due)):
				t.Errorf("due date = %v, want %v", c.rem.DueDate, c.due)
			}
		})
	}
}

func TestComputeStatusScheduledDateWinsOverMonths(t *testing.T) {
	months := 12
	rem := &MaintenanceReminder{
		ScheduledDate:   day(2026, time.November, 1),
		IntervalMonths:  &months,
		LastServiceDate: day(2025, time.January, 1),
		CreatedAt:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		LeadDays:        15,
	}
	rem.ComputeStatus(0, time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC))
	if rem.Status != "OK" || rem.DueDate == nil || !rem.DueDate.Equal(*rem.ScheduledDate) {
		t.Errorf("expected the fixed date to drive the status, got %s %v", rem.Status, rem.DueDate)
	}
}

func TestComputeStatusScheduledDateWithKilometers(t *testing.T) {
	km, odo := 10000, 30000.0
	rem := &MaintenanceReminder{
		ScheduledDate:       day(2026, time.November, 1),
		IntervalKm:          &km,
		LastServiceOdometer: &odo,
		LeadKm:              1000,
		LeadDays:            15,
	}
	rem.ComputeStatus(40500, time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC))
	if rem.Status != "OVERDUE" {
		t.Errorf("kilometers past due must win over a distant date, got %s", rem.Status)
	}
}

func intp(v int) *int { return &v }
