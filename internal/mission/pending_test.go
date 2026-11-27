package mission

import (
	"strings"
	"testing"
	"time"
)

func heldOn(at time.Time, work ...Work) *Pending { return pendingAt(work, at) }

var (
	suite  = Work{What: "just test", From: "harness", Wakes: WakeYes}
	server = Work{What: "nohup node server.js", From: "processes", PID: 4242, Wakes: WakeNo}
)

// The end of a turn records what crew left running, as their harness said;
// unknown, or nothing, records nothing. The next prompt clears it. A
// prompt that's the harness waking them is no message: it leaves what they
// said in place. A message answers a question they'd asked, and leaves
// their done, which isn't a message's to take back.
func TestCrewHook_PendingAndWakeUps(t *testing.T) {
	fixedNow(t)
	s := owned(t)
	m, _ := s.Create("Cache")
	s.StartCrew(m.Slug, "claude", "chat-1")
	crew := func() Crew { m, _ := s.Load(m.Slug); return m.Crew[0] }

	s.CrewHook(m.Slug, "c1", Hook{Prompted: true})
	s.CrewHook(m.Slug, "c1", Hook{Activity: ActivityWaiting, Work: []Work{suite}})
	c := crew()
	if c.Pending == nil || !c.Pending.At.Equal(*c.ActivityAt) || len(c.Pending.Work) != 1 || c.Pending.Work[0] != suite {
		t.Fatalf("a stop with work: %+v", c.Pending)
	}
	// Notifications and replies leave it.
	s.CrewHook(m.Slug, "c1", Hook{Notice: "Claude is waiting for your input"})
	s.CrewHook(m.Slug, "c1", Hook{Reply: "Running the suite."})
	if crew().Pending == nil {
		t.Error("a notice cleared it")
	}

	// It reports done while the work runs; the work's end wakes it.
	s.SetCrewStatus(m.Slug, "c1", CrewDone, "Stage one is in.")
	s.CrewHook(m.Slug, "c1", Hook{Prompted: true, WakeUp: true})
	if c := crew(); c.Pending != nil || c.Activity != ActivityWorking || c.Status != CrewDone || c.Note != "Stage one is in." {
		t.Errorf("after a wake-up: pending %+v, activity %q, status %q (%q)", c.Pending, c.Activity, c.Status, c.Note)
	}
	s.CrewHook(m.Slug, "c1", Hook{Activity: ActivityWaiting})
	if c := crew(); c.Pending != nil || c.Status != CrewDone {
		t.Errorf("a stop with nothing running: pending %+v, status %q", c.Pending, c.Status)
	}

	// Your message leaves the done, and answers a question.
	s.CrewHook(m.Slug, "c1", Hook{Prompted: true})
	if c := crew(); c.Status != CrewDone || c.Note != "Stage one is in." {
		t.Errorf("your message took back the done: status %q (%q)", c.Status, c.Note)
	}
	s.SetCrewStatus(m.Slug, "c1", CrewAsking, "Which suite?")
	s.CrewHook(m.Slug, "c1", Hook{Prompted: true})
	if c := crew(); c.Status != "" || c.Note != "" {
		t.Errorf("your message left the question: status %q", c.Status)
	}

	// Work recorded ends; a later turn's isn't the watch's to end.
	s.CrewHook(m.Slug, "c1", Hook{Activity: ActivityWaiting, Work: []Work{suite, server}})
	turn := crew().Pending.At
	seen := turn.Add(3 * time.Minute)
	if changed, _ := s.EndWork(m.Slug, "c1", turn.Add(-time.Minute), seen, func(Work) bool { return true }); changed {
		t.Error("ended another turn's work")
	}
	if changed, _ := s.EndWork(m.Slug, "c1", turn, seen, func(w Work) bool { return w.PID == 4242 }); !changed {
		t.Error("ended nothing")
	}
	if p := crew().Pending; p.Work[0].Ended != nil || p.Work[1].Ended == nil || !p.Work[1].Ended.Equal(seen) {
		t.Errorf("after the server ended: %+v", p.Work)
	}
	if changed, _ := s.EndWork(m.Slug, "c1", turn, seen, func(w Work) bool { return w.PID == 4242 }); changed {
		t.Error("ended it twice")
	}

	// Resumed, its new session waits on nothing yet.
	s.EndCrew(m.Slug, "c1", "exit 0")
	s.ResumeCrew(m.Slug, "c1", "ws")
	if crew().Pending != nil {
		t.Error("a resumed crew kept its last session's work")
	}
}

// Crew waiting on their own work are in the agent's court, with what they
// wait on, not "waiting for you". Once the turn's no longer held, it's
// yours as any quiet turn is.
func TestResolve_CrewWaitingOnOwnWork(t *testing.T) {
	at := time.Date(2026, 11, 12, 18, 21, 35, 0, time.UTC)
	m := &Mission{Slug: "cache", Crew: []Crew{{ID: "c1", Started: at.Add(-time.Hour), Activity: ActivityWaiting, ActivityAt: &at,
		LastReply: "Rerunning the full suite; I'll commit once it passes.", Pending: heldOn(at, suite)}}}

	h := Resolve(Snapshot{M: m, State: Active, Now: at.Add(time.Minute)})[0]
	if h.Court != CourtAgent || h.Kind != KindOwnWork || h.Text != "c1 is waiting on background work: just test" || !h.Since.Equal(at) {
		t.Errorf("held: %+v", h)
	}
	if h.Action.Verb != VerbWatch || h.Action.Target != "c1" {
		t.Errorf("held: action %+v", h.Action)
	}

	h = Resolve(Snapshot{M: m, State: Active, Now: at.Add(HoldMax)})[0]
	if h.Court != CourtYou || !strings.HasPrefix(h.Text, "c1 is waiting for you") {
		t.Errorf("no longer held: %+v", h)
	}

	// You talked to it: it's answering you, whatever it left running.
	m.Crew[0].Conversing = true
	if h := Resolve(Snapshot{M: m, State: Active, Now: at.Add(time.Minute)})[0]; h.Kind != KindReplied {
		t.Errorf("conversing: %+v", h)
	}
}

func TestWorkWhat(t *testing.T) {
	long := strings.Repeat("x", 100)
	cases := []struct {
		work []Work
		want string
	}{
		{[]Work{suite}, "just test"},
		{[]Work{{What: "go test ./...\n  -run X"}}, "go test ./...…"},
		{[]Work{{What: "ls **/*.go"}}, "ls **/*.go"},
		{[]Work{{What: long}}, strings.Repeat("x", workWhatMax-1) + "…"},
		{[]Work{suite, server, suite, suite}, "just test, nohup node server.js, and 2 more"},
	}
	for _, c := range cases {
		if got := WorkWhat(c.work); got != c.want {
			t.Errorf("WorkWhat = %q, want %q", got, c.want)
		}
	}
}
