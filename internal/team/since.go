package team

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Since reads where a period starts, as it's said: today, yesterday, a
// weekday (the latest one, today included), week (this week's Monday), a
// number of days or weeks (7d, 2w) or a date (2026-11-03). It's the start
// of that day.
func Since(s string, now time.Time) (time.Time, error) {
	day := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()) }
	today := day(now)
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "today":
		return today, nil
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	case "week":
		return today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7)), nil
	}
	for d := time.Sunday; d <= time.Saturday; d++ {
		if name := strings.ToLower(d.String()); s == name || s == name[:3] {
			return today.AddDate(0, 0, -((int(today.Weekday()) - int(d) + 7) % 7)), nil
		}
	}
	if n := len(s); n > 1 && (s[n-1] == 'd' || s[n-1] == 'w') {
		if count, err := strconv.Atoi(s[:n-1]); err == nil && count >= 0 {
			if s[n-1] == 'w' {
				count *= 7
			}
			return today.AddDate(0, 0, -count), nil
		}
	}
	if t, err := time.ParseInLocation("2006-01-02", s, now.Location()); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("%q isn't a time to start from: say today, yesterday, a weekday, week, 7d, 2w or a date like 2026-11-03", s)
}
