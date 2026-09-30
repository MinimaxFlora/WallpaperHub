// Package selector implements the deterministic date-to-image mapping.
package selector

import "time"

const secondsPerDay = 86400

// Today returns midnight (00:00:00) of the calendar date containing now in loc.
func Today(now time.Time, loc *time.Location) time.Time {
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// OffsetDate returns today shifted backwards by idx days.
func OffsetDate(today time.Time, idx int) time.Time {
	return today.AddDate(0, 0, -idx)
}

// DayNumber returns the civil day number of date, counted in days since the
// Unix epoch. The calendar fields are read in loc, while the number itself is
// computed in UTC so that the result is exact for every time zone.
func DayNumber(date time.Time, loc *time.Location) int64 {
	y, m, d := date.In(loc).Date()
	utcMidnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return utcMidnight.Unix() / secondsPerDay
}

// PickIndex maps date to an index in [0, n) using the day number modulo n.
// It returns 0 when n is not positive.
func PickIndex(date time.Time, loc *time.Location, n int) int {
	if n <= 0 {
		return 0
	}
	r := DayNumber(date, loc) % int64(n)
	if r < 0 {
		r += int64(n)
	}
	return int(r)
}
