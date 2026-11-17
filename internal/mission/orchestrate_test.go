package mission

import (
	"strings"
	"testing"
	"time"
)

func TestOrchestratorTakes(t *testing.T) {
	at := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	m := &Mission{Crew: []Crew{
		{ID: "c1", Started: at, Status: CrewDone, StatusAt: &at},
		{ID: "c2", Started: at, Role: RoleBriefing},
		{ID: "c3", Started: at, Role: RoleAnswer},
		{ID: "c4", Started: at, Activity: ActivityWaiting, ActivityAt: &at},
		{ID: "c5", Started: at, Activity: ActivityWaiting, ActivityAt: &at, Conversing: true},
		{ID: "c6", Started: at, Ended: &at},
	}}
	cases := []struct {
		e    OrchestratorEvent
		want bool
	}{
		{OrchestratorEvent{Trigger: "answer"}, true},
		{OrchestratorEvent{Trigger: "review"}, true},
		{OrchestratorEvent{Crew: "c1", Trigger: "done"}, true},
		{OrchestratorEvent{Crew: "c1", Trigger: "ended"}, false}, // its done was looked at
		{OrchestratorEvent{Crew: "c2", Trigger: "ended"}, false},
		{OrchestratorEvent{Crew: "c3", Trigger: "done"}, false},
		{OrchestratorEvent{Crew: "c4", Trigger: "quiet"}, true},
		{OrchestratorEvent{Crew: "c5", Trigger: "quiet"}, false}, // answering you
		{OrchestratorEvent{Crew: "c6", Trigger: "ended"}, true},
		{OrchestratorEvent{Crew: "c6", Trigger: "ended", Stopped: true}, false},
		{OrchestratorEvent{Crew: "c9", Trigger: "done"}, false},
	}
	for _, c := range cases {
		if got, why := OrchestratorTakes(m, c.e); got != c.want {
			t.Errorf("%+v: %v (%s)", c.e, got, why)
		}
	}
	m.Crew[0].Orchestrated = m.Crew[0].eventKey()
	if got, _ := OrchestratorTakes(m, OrchestratorEvent{Crew: "c1", Trigger: "done"}); got {
		t.Error("looked at the same event twice")
	}
}

// A quiet turn crew ended to wait on work they left running is held: while
// the harness will wake them for it, and while work nothing wakes them for
// runs. It's taken once that work has ended, and by the backstop: work
// that never ends, or that ended without the harness waking them. Done
// and ended crew are taken whatever's running.
func TestOrchestratorTakes_HeldOnOwnWork(t *testing.T) {
	at := time.Date(2026, 11, 12, 18, 21, 35, 0, time.UTC)
	waiting := func(id string, p *Pending) Crew {
		return Crew{ID: id, Started: at.Add(-time.Hour), Activity: ActivityWaiting, ActivityAt: &at, Pending: p}
	}
	m := &Mission{Crew: []Crew{
		waiting("c1", nil),                       // nothing running, or unknown
		waiting("c2", heldOn(at, suite)),         // the harness wakes it
		waiting("c3", heldOn(at, server)),        // nothing wakes it
		waiting("c4", heldOn(at, suite, server)), // both
		{ID: "c5", Started: at, Status: CrewDone, StatusAt: &at, Activity: ActivityWaiting, ActivityAt: &at, Pending: heldOn(at, server)},
		{ID: "c6", Started: at, Ended: &at, Pending: heldOn(at, suite)},
	}}
	soon := at.Add(5 * time.Second)
	takes := func(id, trigger string, when time.Time) (bool, string) {
		t.Helper()
		return OrchestratorTakes(m, OrchestratorEvent{Crew: id, Trigger: trigger, At: when})
	}

	if ok, why := takes("c1", "quiet", soon); !ok {
		t.Errorf("nothing running: not taken (%s)", why)
	}
	if ok, why := takes("c2", "quiet", soon); ok || !strings.Contains(why, "its harness wakes it for: just test") {
		t.Errorf("work its harness wakes it for: %v (%s)", ok, why)
	}
	if ok, why := takes("c3", "quiet", soon); ok || !strings.Contains(why, "ws takes its turn when that ends: nohup node server.js") {
		t.Errorf("work nothing wakes it for: %v (%s)", ok, why)
	}
	if ok, _ := takes("c4", "quiet", soon); ok {
		t.Error("taken while work runs")
	}
	if ok, why := takes("c5", "done", soon); !ok {
		t.Errorf("done crew: not taken (%s)", why)
	}
	if ok, why := takes("c6", "ended", soon); !ok {
		t.Errorf("ended crew: not taken (%s)", why)
	}

	// The work nothing wakes c3 for ends: its turn is taken, once.
	ended := at.Add(6 * time.Minute)
	m.Crew[2].Pending.Work[0].Ended = &ended
	if ok, why := takes("c3", "quiet", ended); !ok {
		t.Errorf("its work ended: not taken (%s)", why)
	}
	m.Crew[2].Orchestrated = m.Crew[2].eventKey()
	if ok, _ := takes("c3", "quiet", ended.Add(time.Minute)); ok {
		t.Error("taken twice")
	}
	// c4's ended too, but its harness's own task still runs.
	m.Crew[3].Pending.Work[1].Ended = &ended
	if ok, _ := takes("c4", "quiet", ended); ok {
		t.Error("taken while work its harness wakes it for runs")
	}

	// The backstop: work that never ends is taken after HoldMax.
	if ok, _ := takes("c2", "quiet", at.Add(HoldMax-time.Second)); ok {
		t.Error("taken before HoldMax")
	}
	if ok, why := takes("c2", "quiet", at.Add(HoldMax)); !ok {
		t.Errorf("still running after HoldMax: not taken (%s)", why)
	}
	// And work the harness said it would wake it for, ended, with no turn
	// started a WakeGrace later.
	m.Crew[1].Pending.Work[0].Ended = &ended
	if ok, why := takes("c2", "quiet", ended.Add(WakeGrace-time.Second)); ok || !strings.Contains(why, "its work has ended") {
		t.Errorf("within WakeGrace: %v (%s)", ok, why)
	}
	if ok, why := takes("c2", "quiet", ended.Add(WakeGrace)); !ok {
		t.Errorf("not woken: not taken (%s)", why)
	}
	// Ended alongside work nothing wakes it for, the harness still has the
	// minute to wake it.
	m.Crew[3].Pending.Work[0].Ended = &ended
	if ok, _ := takes("c4", "quiet", ended.Add(time.Second)); ok {
		t.Error("taken before its harness had the minute to wake it")
	}
	if ok, why := takes("c4", "quiet", ended.Add(WakeGrace)); !ok {
		t.Errorf("all ended, not woken: not taken (%s)", why)
	}
}
