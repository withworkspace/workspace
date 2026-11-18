package ops

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
)

// stopHost holds every crew it's asked about, and remembers whom it was
// asked to stop.
type stopHost struct {
	fakeHost
	stopped []string
}

func (h *stopHost) Stop(key string) error { h.stopped = append(h.stopped, key); return nil }

// crewAt puts crew on the cache mission, each as given, waiting since at
// unless it says otherwise.
func crewAt(t *testing.T, w *Workspace, at time.Time, crew ...mission.Crew) {
	t.Helper()
	if _, err := w.Missions.Update("cache", func(m *mission.Mission) error {
		for _, c := range crew {
			c.Started = at.Add(-time.Hour)
			if c.ActivityAt == nil && c.Activity != "" {
				c.ActivityAt = &at
			}
			m.Crew = append(m.Crew, c)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Crew who reported done are closed once their turn has been over a while,
// and only while nothing ws runs after a done is looking at the mission.
func TestCloseDone(t *testing.T) {
	w, _ := fixture(t, "")
	h := &stopHost{}
	w.opts.Host = h
	turnEnded := time.Now()
	gone := turnEnded.Add(-time.Minute)
	crewAt(t, w, turnEnded,
		mission.Crew{ID: "c1", Status: mission.CrewDone, Activity: mission.ActivityWaiting},               // done, waiting
		mission.Crew{ID: "c2", Activity: mission.ActivityWaiting},                                         // waiting, not done
		mission.Crew{ID: "c3", Status: mission.CrewDone, Activity: mission.ActivityWorking},               // reported done mid-turn
		mission.Crew{ID: "c4", Status: mission.CrewDone, Activity: mission.ActivityWaiting, Ended: &gone}, // closed already
		mission.Crew{ID: "c5", Status: mission.CrewDone, Activity: mission.ActivityWaiting, Pending: &mission.Pending{}},
		mission.Crew{ID: "c6", Status: mission.CrewDone, Activity: mission.ActivityWaiting, Role: mission.RoleBriefing},
	)

	if said := w.CloseDone(turnEnded.Add(CloseDoneAfter - time.Second)); len(said) > 0 || len(h.stopped) > 0 {
		t.Fatalf("closed before its time: %v %v", said, h.stopped)
	}

	w.Missions.StartOrchestrating("cache", "c1 (done)", time.Minute)
	later := turnEnded.Add(CloseDoneAfter)
	if w.CloseDone(later); len(h.stopped) > 0 {
		t.Fatalf("closed while the orchestrator looks: %v", h.stopped)
	}
	w.Missions.Update("cache", func(m *mission.Mission) error { m.Orchestrating = nil; return nil })

	said := w.CloseDone(later)
	if !slices.Equal(h.stopped, []string{"cache/c1"}) {
		t.Errorf("stopped %v, want only cache/c1", h.stopped)
	}
	if len(said) != 1 {
		t.Errorf("said %v", said)
	}
	log, _ := w.Missions.ReadLog("cache", 0)
	if !slices.ContainsFunc(log, func(e mission.Event) bool {
		return e.Kind == mission.EventNote && strings.Contains(e.Text, "closed c1's session")
	}) {
		t.Errorf("the close isn't logged: %v", log)
	}
}

// A mission that ends closes its crew: done is as far as they'd go.
func TestEndClosesCrew(t *testing.T) {
	w, _ := fixture(t, "## Goal\nx\n")
	h := &stopHost{}
	w.opts.Host = h
	now := time.Now()
	crewAt(t, w, now,
		mission.Crew{ID: "c1", Status: mission.CrewDone, Activity: mission.ActivityWaiting},
		mission.Crew{ID: "c2", Status: mission.CrewDone, Ended: &now},
	)
	if _, err := w.End("cache", &EndAs{Ending: "dropped", Text: "x"}, AsYou); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(h.stopped, []string{"cache/c1"}) {
		t.Errorf("stopped %v, want cache/c1", h.stopped)
	}
}

// Archiving a mission closes its crew too, before their capsules go.
func TestArchiveClosesCrew(t *testing.T) {
	w, _ := fixture(t, "")
	h := &stopHost{}
	w.opts.Host = h
	crewAt(t, w, time.Now(), mission.Crew{ID: "c1", Status: mission.CrewDone, Activity: mission.ActivityWaiting})
	if _, err := w.Archive("cache", false, AsYou); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(h.stopped, []string{"cache/c1"}) {
		t.Errorf("stopped %v, want cache/c1", h.stopped)
	}
}
