package demodata

import (
	"testing"
	"time"
)

func TestInspectionReminderClampsMonthEnd(t *testing.T) {
	for _, tc := range []struct{ key, day, want string }{
		{"ev", "2026-08-31", "2027-02-28"},
		{"ev", "2023-08-31", "2024-02-29"},
		{"ice", "2026-01-31", "2026-04-30"},
	} {
		at, err := time.Parse("2006-01-02", tc.day)
		if err != nil {
			t.Fatal(err)
		}
		dataset, err := Build(tc.key, at)
		if err != nil {
			t.Fatal(err)
		}
		if got := dataset.Reminders[2].Due.Format("2006-01-02"); got != tc.want {
			t.Errorf("%s at %s: got %s, want %s", tc.key, tc.day, got, tc.want)
		}
	}
}
