package automation

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/events"
)

// A schedule reads as how often, or a time of day on some days; what it
// can't be is said.
func TestSchedule_Parse(t *testing.T) {
	for in, want := range map[string]string{
		"1h": "every 1h0m0s", "30m": "every 30m0s", "day 03:00": "days 7 at 180", "Weekday 9:30": "days 5 at 570",
		"mon,thu 17:30": "days 2 at 1050", "fri": "days 1 at 540",
	} {
		s, err := ParseSchedule(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		got := "every " + s.Every.String()
		if s.Every == 0 {
			got = "days " + string(rune('0'+len(s.Days))) + " at " + itoa(s.At)
		}
		if got != want {
			t.Errorf("%q read as %s, want %s", in, got, want)
		}
	}
	for _, bad := range []string{"", "1m", "someday", "day 25:00", "mon 9"} {
		if _, err := ParseSchedule(bad); err == nil {
			t.Errorf("%q read", bad)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// Its next time is the first after a time: today's if it's still to come,
// else the next day it's on.
func TestSchedule_Next(t *testing.T) {
	s, _ := ParseSchedule("weekday 09:00")
	fri := time.Date(2026, 11, 27, 10, 0, 0, 0, time.Local) // a Friday, after 09:00
	if got := s.Next(fri); !got.Equal(time.Date(2026, 11, 30, 9, 0, 0, 0, time.Local)) {
		t.Errorf("after Friday's: %v, want Monday's", got)
	}
	if got := s.Next(fri.Add(-2 * time.Hour)); !got.Equal(time.Date(2026, 11, 27, 9, 0, 0, 0, time.Local)) {
		t.Errorf("before Friday's: %v", got)
	}
}

const daily = `# Once a day, say so.
when = every("day 03:00")
may = ["notify"]

def run(e):
    notify("it's " + e.title)
`

// Its time come, it runs, on a schedule.due of its own: once, however many
// times were missed while the daemon was away; not again till its next.
func TestSchedule_Runs(t *testing.T) {
	e, got := engine(t)
	later := clock(e)
	if _, err := e.Add("daily", []byte(daily)); err != nil {
		t.Fatal(err)
	}
	a, _ := Load(e.Root, "daily")
	if a.WhenSaid() != "every day 03:00" {
		t.Errorf("when said %q", a.WhenSaid())
	}
	e.RunDue()
	if len(got.notified) != 0 {
		t.Fatalf("ran before its time: %q", got.notified)
	}
	later(3 * 24 * time.Hour) // three missed
	e.RunDue()
	e.RunDue()
	if strings.Join(got.notified, "|") != "daily: it's day 03:00 " {
		t.Errorf("notified %q", got.notified)
	}
	due, _ := events.List(e.Root, events.Filter{Kinds: []string{events.ScheduleDue}})
	if len(due) != 1 || due[0].Automation != "daily" || due[0].Says() != "daily's time: day 03:00" {
		t.Errorf("due %+v", due)
	}
	later(24 * time.Hour)
	e.RunDue()
	if len(got.notified) != 2 {
		t.Errorf("the next day: %q", got.notified)
	}
}

// Another automation's time isn't its: a schedule.due is only for the one
// it names.
func TestSchedule_OnlyItsOwn(t *testing.T) {
	e, got := engine(t)
	later := clock(e)
	e.Add("daily", []byte(daily))
	e.Add("other", []byte(strings.Replace(daily, "day 03:00", "mon 09:00", 1)))
	later(25 * time.Hour)
	e.RunDue()
	for _, n := range got.notified {
		if strings.HasPrefix(n, "other:") && !strings.Contains(n, "mon") {
			t.Errorf("other ran on daily's time: %q", got.notified)
		}
	}
}

// A dry run runs it on each of its times over the time it looks back.
func TestSchedule_DryRun(t *testing.T) {
	e, _ := engine(t)
	a, err := Check("daily", []byte(daily))
	if err != nil {
		t.Fatal(err)
	}
	dr, err := e.DryRun(a, 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Ran < 6 || dr.Ran > 7 || len(dr.Lines) != dr.Ran || dr.Lines[0].Event != "" || dr.Lines[0].Did[0] != "notify: it's day 03:00" {
		t.Errorf("%+v", dr)
	}
}
