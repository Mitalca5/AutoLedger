package models

import "time"

// AddCalendarMonths keeps a monthly interval in its target month, clamping an unavailable day.
func AddCalendarMonths(base time.Time, months int) time.Time {
	target := time.Date(base.Year(), base.Month()+time.Month(months), 1, 0, 0, 0, 0, base.Location())
	lastDay := time.Date(target.Year(), target.Month()+1, 0, 0, 0, 0, 0, base.Location()).Day()
	day := base.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(target.Year(), target.Month(), day, base.Hour(), base.Minute(), base.Second(), base.Nanosecond(), base.Location())
}
