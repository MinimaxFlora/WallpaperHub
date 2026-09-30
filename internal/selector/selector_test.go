package selector

import (
	"testing"
	"time"
)

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load location %s: %v", name, err)
	}
	return loc
}

func TestTodayTrimsToMidnightInLocation(t *testing.T) {
	shanghai := mustLoad(t, "Asia/Shanghai")
	now := time.Date(2026, 9, 30, 23, 45, 10, 0, time.UTC) // 2026-10-01 07:45 in Shanghai
	got := Today(now, shanghai)
	want := time.Date(2026, 10, 1, 0, 0, 0, 0, shanghai)
	if !got.Equal(want) {
		t.Fatalf("Today() = %v, want %v", got, want)
	}
}

func TestOffsetDateMovesBackwards(t *testing.T) {
	loc := time.UTC
	today := time.Date(2026, 3, 1, 0, 0, 0, 0, loc)
	got := OffsetDate(today, 1)
	want := time.Date(2026, 2, 28, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("OffsetDate() = %v, want %v", got, want)
	}
}

func TestDayNumberIsStableAcrossTimeZones(t *testing.T) {
	shanghai := mustLoad(t, "Asia/Shanghai")
	newYork := mustLoad(t, "America/New_York")
	// The same civil date in two different zones must yield the same day number.
	shDate := time.Date(2026, 9, 30, 0, 0, 0, 0, shanghai)
	nyDate := time.Date(2026, 9, 30, 0, 0, 0, 0, newYork)
	if a, b := DayNumber(shDate, shanghai), DayNumber(nyDate, newYork); a != b {
		t.Fatalf("DayNumber differs per zone: %d vs %d", a, b)
	}
	if got, want := DayNumber(shDate, shanghai), int64(20726); got != want {
		t.Fatalf("DayNumber() = %d, want %d", got, want)
	}
}

func TestDayNumberIncrementsByOnePerDay(t *testing.T) {
	loc := mustLoad(t, "Asia/Shanghai")
	day1 := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	day2 := day1.AddDate(0, 0, 1)
	if b, a := DayNumber(day2, loc), DayNumber(day1, loc); b-a != 1 {
		t.Fatalf("day numbers are not consecutive: %d, %d", a, b)
	}
}

func TestPickIndexTable(t *testing.T) {
	loc := time.UTC
	epoch := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		day  int
		n    int
		want int
	}{
		{"zero images", 100, 0, 0},
		{"negative n", 100, -3, 0},
		{"single image", 100, 1, 0},
		{"exact multiple", 100, 10, 0},
		{"remainder", 103, 10, 3},
		{"epoch", 0, 7, 0},
		{"negative day number", -1, 7, 6},
		{"large day number", 1 << 20, 577, int(int64(1<<20) % 577)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			date := epoch.AddDate(0, 0, tc.day)
			if got := PickIndex(date, loc, tc.n); got != tc.want {
				t.Fatalf("PickIndex(%d, %d) = %d, want %d", tc.day, tc.n, got, tc.want)
			}
		})
	}
}

func TestPickIndexIsStableWithinSameDay(t *testing.T) {
	loc := mustLoad(t, "Asia/Shanghai")
	morning := time.Date(2026, 9, 30, 1, 0, 0, 0, loc)
	evening := time.Date(2026, 9, 30, 23, 0, 0, 0, loc)
	if a, b := PickIndex(morning, loc, 5), PickIndex(evening, loc, 5); a != b {
		t.Fatalf("same day produced different picks: %d vs %d", a, b)
	}
}

func TestPickIndexAdvancesByOnePerDay(t *testing.T) {
	loc := time.UTC
	n := 7
	day1 := time.Date(2026, 5, 10, 0, 0, 0, 0, loc)
	day2 := day1.AddDate(0, 0, 1)
	want := (PickIndex(day1, loc, n) + 1) % n
	if got := PickIndex(day2, loc, n); got != want {
		t.Fatalf("PickIndex advanced to %d, want %d", got, want)
	}
}
