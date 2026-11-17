package web

import (
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
)

// Crew that reported done and has been at work since (sent back to it)
// is working again; reported done and idle, or ended, it isn't.
func TestWorkingAgain(t *testing.T) {
	done := time.Date(2026, 11, 16, 15, 22, 29, 0, time.UTC)
	later, earlier := done.Add(4*time.Minute), done.Add(-time.Minute)
	crew := func(activity string, at time.Time, ended bool) mission.Crew {
		c := mission.Crew{ID: "c2", Status: mission.CrewDone, StatusAt: &done, Activity: activity, ActivityAt: &at}
		if ended {
			c.Ended = &later
		}
		return c
	}
	for _, tc := range []struct {
		name string
		c    mission.Crew
		want bool
	}{
		{"sent back, re-running the tests", crew(mission.ActivityWorking, later, false), true},
		{"done, and waiting", crew(mission.ActivityWaiting, later, false), false},
		{"working when it said done", crew(mission.ActivityWorking, earlier, false), false},
		{"ended", crew(mission.ActivityWorking, later, true), false},
	} {
		if got := workingAgain(tc.c); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}
