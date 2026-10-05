package handlers

import (
	"testing"
	"time"
)

func strp(s string) *string { return &s }

func TestBuildReminderScheduledDate(t *testing.T) {
	h := &ReminderHandler{}

	rem, err := h.buildReminder(ReminderPayload{Title: "Pneus hiver", ScheduledDate: strp("2026-11-01"), RepeatYearly: true}, "veh", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rem.ScheduledDate == nil || !rem.ScheduledDate.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) || !rem.RepeatYearly {
		t.Errorf("scheduled date or yearly flag lost: %+v", rem)
	}

	rem, err = h.buildReminder(ReminderPayload{Title: "CT", ScheduledDate: strp("2026-11-01T23:00:00.000Z")}, "veh", "")
	if err != nil || rem.ScheduledDate == nil || rem.ScheduledDate.Day() != 1 {
		t.Errorf("an RFC 3339 timestamp must keep its date part: %v %+v", err, rem)
	}

	rem, err = h.buildReminder(ReminderPayload{Title: "Vidange", RepeatYearly: true}, "veh", "")
	if err != nil || rem.RepeatYearly || rem.ScheduledDate != nil {
		t.Errorf("yearly without a date must be ignored: %v %+v", err, rem)
	}
}

func TestBuildReminderRejectsBadSchedules(t *testing.T) {
	h := &ReminderHandler{}
	months := 12

	if _, err := h.buildReminder(ReminderPayload{Title: "x", ScheduledDate: strp("not a date")}, "veh", ""); err == nil {
		t.Error("an invalid date must be rejected")
	}
	if _, err := h.buildReminder(ReminderPayload{Title: "x", ScheduledDate: strp("2026-11-01"), IntervalMonths: &months}, "veh", ""); err == nil {
		t.Error("a fixed date together with a months interval must be rejected")
	}
}
