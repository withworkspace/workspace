package web

import (
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store/storetest"
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

// A list of crew says "Starting" from the mark the mission's headline
// reads, and from nothing else: through a start, the two agree. Crew with
// ws's hooks in their session are starting until their first report,
// though ws has handed over their prompt; crew without, until it has.
func TestCrewOf_StartingAsTheHeadlineHasIt(t *testing.T) {
	store := mission.Store{Root: storetest.Own(t, t.TempDir()).Root()}
	m, err := store.Create("Cache tokens")
	if err != nil {
		t.Fatal(err)
	}
	slug := m.Slug
	// agree is the crew's place in a list and the mission's headline, as
	// they stand now: both starting, or neither.
	agree := func(when, id string, starting bool) {
		t.Helper()
		m, err := store.Load(slug)
		if err != nil {
			t.Fatal(err)
		}
		c, _ := m.CrewByID(id)
		h := mission.Resolve(mission.Snapshot{M: m, State: mission.Active, Now: time.Now()})[0]
		banner := h.Kind == mission.KindLaunching
		if listed := crewOf(c).Starting; listed != starting || banner != starting {
			t.Errorf("%s: the list has %s starting %v, the headline %q (%s); want starting %v in both", when, id, listed, h.Text, h.Kind, starting)
		}
		if !starting && (h.Kind != mission.KindWorking || h.By != id) {
			t.Errorf("%s: the headline is %q (%s), want %s working", when, h.Text, h.Kind, id)
		}
	}
	launch := func(hooks bool) string {
		t.Helper()
		store.StartLaunching(slug, "")
		c, err := store.StartCrew(slug, "claude", "")
		if err != nil {
			t.Fatal(err)
		}
		store.SetCrewHooks(slug, c.ID, hooks)
		return c.ID
	}

	id := launch(true)
	agree("hooks, its session coming up", id, true)
	store.CrewHandedPrompt(slug, id)
	agree("hooks, its prompt handed over and nothing reported", id, true)
	store.CrewHook(slug, id, mission.Hook{Conversation: "abc"})
	agree("hooks, its first report", id, false)
	store.CrewHook(slug, id, mission.Hook{Prompted: true})
	agree("hooks, its first turn", id, false)
	store.EndCrew(slug, id, "exit 0")

	id = launch(false)
	agree("no hooks, its session coming up", id, true)
	store.CrewHandedPrompt(slug, id)
	agree("no hooks, its prompt handed over", id, false)
}
