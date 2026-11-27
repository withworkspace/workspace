package automation

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.starlark.net/starlark"

	"github.com/withworkspace/workspace/internal/events"
)

// A schedule is when = every(...): an automation run on a clock rather
// than on what happens. Its time come, the engine keeps a schedule.due
// event for it (and only it) and runs it on that, as on any event: so a
// run has an event to be on, its runs and waits are as any other's, and
// ws events says when it ran. The daemon runs on this machine: a time
// missed while it was asleep or stopped is run once when it's back, not
// once for each.

// Schedule is when an automation's run by the clock: every so long
// (Every), or at a time of day (At, minutes past midnight, here) on some
// days of the week (Days).
type Schedule struct {
	Every time.Duration
	Days  []time.Weekday
	At    int
	// Said is it as it's written: "day 03:00", "1h".
	Said string
}

// MinEvery is the least an every can be.
const MinEvery = 5 * time.Minute

var dayNames = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// ParseSchedule reads one: "1h", "30m" (every so long), "day 03:00",
// "weekday 09:00", "mon 09:00", "mon,thu 17:30". A time of day left out
// is 09:00.
func ParseSchedule(s string) (Schedule, error) {
	said := strings.Join(strings.Fields(strings.ToLower(s)), " ")
	out := Schedule{Said: said}
	if said == "" {
		return out, fmt.Errorf(`every says when: "1h", "day 03:00", "weekday 09:00", "mon 09:00"`)
	}
	if d, err := lengthOf(said); err == nil {
		if d < MinEvery {
			return out, fmt.Errorf("every %s is too often: at least every %s", said, shortDuration(MinEvery))
		}
		out.Every = d
		return out, nil
	}
	days, at, _ := strings.Cut(said, " ")
	switch days {
	case "day", "daily", "every day":
		out.Days = []time.Weekday{0, 1, 2, 3, 4, 5, 6}
	case "weekday", "weekdays":
		out.Days = []time.Weekday{1, 2, 3, 4, 5}
	default:
		for _, d := range strings.Split(days, ",") {
			wd, ok := dayNames[strings.TrimSpace(d)[:min(3, len(strings.TrimSpace(d)))]]
			if !ok {
				return out, fmt.Errorf(`every %q: say how often ("1h") or when ("day 03:00", "weekday 09:00", "mon,thu 17:30")`, s)
			}
			out.Days = append(out.Days, wd)
		}
	}
	out.At = 9 * 60
	if at = strings.TrimSpace(at); at != "" {
		h, m, ok := strings.Cut(at, ":")
		hh, err1 := strconv.Atoi(h)
		mm, err2 := strconv.Atoi(m)
		if !ok || err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
			return out, fmt.Errorf("every %q: the time of day is as 03:00 or 17:30", s)
		}
		out.At = hh*60 + mm
	}
	return out, nil
}

// Next is its first time after t, in t's zone.
func (s Schedule) Next(t time.Time) time.Time {
	if s.Every > 0 {
		return t.Add(s.Every)
	}
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	for i := 0; i < 8; i++ {
		d := day.AddDate(0, 0, i)
		at := d.Add(time.Duration(s.At) * time.Minute)
		if at.After(t) && containsDay(s.Days, d.Weekday()) {
			return at
		}
	}
	return t.Add(24 * time.Hour) // no day: not reached, as one's always read with some
}

func containsDay(days []time.Weekday, d time.Weekday) bool {
	for _, x := range days {
		if x == d {
			return true
		}
	}
	return false
}

// everyBuiltin is every(when): an automation's run by the clock. It's a
// match for its own schedule.due, the automation's name filled in once
// it's known (Check).
func everyBuiltin(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var when string
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &when); err != nil {
		return nil, fmt.Errorf(`every says when, as every("day 03:00"): %w`, err)
	}
	s, err := ParseSchedule(when)
	if err != nil {
		return nil, err
	}
	return matchValue{m: events.Match{Kind: events.ScheduleDue}, schedule: &s}, nil
}

// Schedules are a's: those of its when that are every()s.
func (a *Automation) Schedules() []Schedule {
	var out []Schedule
	for _, s := range a.schedules {
		if s != nil {
			out = append(out, *s)
		}
	}
	return out
}

// scheduleAt is When[i]'s schedule, if it's an every().
func (a *Automation) scheduleAt(i int) *Schedule {
	if i < len(a.schedules) {
		return a.schedules[i]
	}
	return nil
}

// dueAt is when a's next due after since (here, as a time of day is),
// the earliest of its schedules', and which: zero for none.
func (a *Automation) dueAt(since time.Time) (time.Time, Schedule) {
	var due time.Time
	var which Schedule
	for _, s := range a.Schedules() {
		if n := s.Next(since.In(time.Local)); due.IsZero() || n.Before(due) {
			due, which = n, s
		}
	}
	return due, which
}

// tick keeps a schedule.due for a when its time's come since its last
// (one, however many were missed), and moves its last to now.
func (e *Engine) tick(a *Automation, st *State) {
	if len(a.Schedules()) == 0 {
		return
	}
	since := st.Ticked
	if since.IsZero() {
		since = st.Added
	}
	due, which := a.dueAt(since)
	now := e.now()
	if due.IsZero() || now.Before(due) {
		return
	}
	if _, err := events.Emit(e.Root, events.Event{Kind: events.ScheduleDue, By: "ws", Automation: a.Name, Title: which.Said, At: now}); err != nil {
		return
	}
	_ = tick(e.Root, a.Name, now)
	st.Ticked = now
}

// slotsIn are a's times in [from, to], at most limit of them: what a dry
// run runs it on.
func (a *Automation) slotsIn(from, to time.Time, limit int) []events.Event {
	var out []events.Event
	for _, s := range a.Schedules() {
		for t := s.Next(from); !t.After(to) && len(out) < limit; t = s.Next(t) {
			out = append(out, events.Event{Kind: events.ScheduleDue, By: "ws", Automation: a.Name, Title: s.Said, At: t})
		}
	}
	return out
}

// ErrNoSchedule is the answer for running one now that isn't on a clock.
var ErrNoSchedule = errors.New("it runs on events, not on a schedule: it runs when the next one it's for comes")

// RunNow keeps a schedule.due for name, by you: its time come now, which
// the daemon runs it on as on any.
func RunNow(root, name string) (events.Event, error) {
	a, err := Load(root, name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return events.Event{}, ErrNotFound
		}
		return events.Event{}, err
	}
	if len(a.Schedules()) == 0 {
		return events.Event{}, ErrNoSchedule
	}
	return events.Emit(root, events.Event{Kind: events.ScheduleDue, By: events.ByYou, Automation: name, Title: "run now"})
}
