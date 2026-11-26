package automation

import (
	"github.com/withworkspace/workspace/internal/store/storetest"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/events"
)

// clock is an engine's clock, moved by hand.
func clock(e *Engine) func(d time.Duration) {
	now := time.Now()
	e.Now = func() time.Time { return now }
	return func(d time.Duration) { now = now.Add(d) }
}

const deploy = `# After a merge, wait for the deploy, then say how it went.
when = event("pr.merged")
may = ["notify"]

def run(e):
    pr = show(e.pr)
    d = wait(event("run.ended", run = "deploy"), timeout = "2h")
    if d and d.status == "passed":
        notify("deployed %s (%s)" % (e.pr, pr.head))
    else:
        notify("not deployed " + e.pr)
`

// A run parks at a wait(), and is run again from the top once what it
// waits for comes, or its time's up: what it did and read before is as
// it was, not done again.
func TestWorkflow_Wait(t *testing.T) {
	e, got := engine(t)
	later := clock(e)
	shows := 0
	show := e.Hooks.Show
	e.Hooks.Show = func(ref string) (map[string]any, error) { shows++; return show(ref) }
	e.Add("deploy", []byte(deploy))
	emit(t, e.Root, events.Event{Kind: events.RunEnded, Run: "deploy.ground.api", Title: "deploy", Status: "passed"}) // before: not it
	emit(t, e.Root, events.Event{Kind: events.PRMerged, Origin: events.Heard, By: events.ByPerson("alice"), PR: "api#212"})
	e.RunDue()
	if runs, _ := Runs(e.Root, "deploy", 1); len(got.notified) != 0 || runs[0].State != Waiting {
		t.Fatalf("not parked: %q %+v", got.notified, runs)
	}
	emit(t, e.Root, events.Event{Kind: events.RunEnded, Run: "test.ground.api", Title: "test", Status: "passed"})
	if e.RunDue(); len(got.notified) != 0 {
		t.Fatalf("met by another run: %q", got.notified)
	}
	emit(t, e.Root, events.Event{Kind: events.RunEnded, Run: "deploy.ground.api", Title: "deploy", Status: "passed"})
	e.RunDue()
	if strings.Join(got.notified, "|") != "deploy: deployed api#212 (Idempotency keys) " {
		t.Fatalf("after the deploy: %q", got.notified)
	}
	if shows != 1 {
		t.Errorf("show was asked %d times: a run that's run again is given what it read", shows)
	}
	if runs, _ := Runs(e.Root, "deploy", 1); runs[0].State != Ran {
		t.Errorf("run %+v", runs[0])
	}

	// Its time's up: the wait is None.
	emit(t, e.Root, events.Event{Kind: events.PRMerged, Origin: events.Heard, By: events.ByPerson("alice"), PR: "api#213"})
	e.RunDue()
	later(3 * time.Hour)
	e.RunDue()
	if len(got.notified) != 2 || got.notified[1] != "deploy: not deployed api#213 " {
		t.Errorf("timed out: %q", got.notified)
	}
}

// A wait is for what comes after the event its run's on: one that came
// before the daemon got to the run (it was catching up) meets it, as a
// dry run would have had it.
func TestWorkflow_WaitCountsWhatCameWhileCatchingUp(t *testing.T) {
	e, got := engine(t)
	e.Add("deploy", []byte(deploy))
	emit(t, e.Root, events.Event{Kind: events.PRMerged, Origin: events.Heard, By: events.ByPerson("alice"), PR: "api#212"})
	emit(t, e.Root, events.Event{Kind: events.RunEnded, Run: "deploy.ground.api", Title: "deploy", Status: "passed"})
	e.RunDue() // parks at the wait, which is met at once
	e.RunDue()
	if strings.Join(got.notified, "|") != "deploy: deployed api#212 (Idempotency keys) " {
		t.Errorf("notified %q", got.notified)
	}
}

// settle runs once a key's events have been quiet, with all of them; per
// keeps the keys apart.
func TestWorkflow_Settle(t *testing.T) {
	e, got := engine(t)
	later := clock(e)
	e.Add("settled", []byte(`when = event("thing.changed", settle = "10m", per = "thing")
may = ["notify"]
def run(e):
    notify("%s: %d saves" % (e.thing, len(e.batch)))
`))
	change := func(thing string) {
		emit(t, e.Root, events.Event{Kind: events.ThingChanged, Origin: events.Heard, By: events.ByPerson("brendan"), Thing: thing, ThingKind: "sketch"})
	}
	change("flow")
	change("flow")
	change("queues")
	e.RunDue()
	later(5 * time.Minute)
	change("flow") // still at it: flow settles from now
	e.RunDue()
	later(6 * time.Minute)
	e.RunDue()
	if strings.Join(got.notified, "|") != "settled: queues: 1 saves " {
		t.Fatalf("after 11 minutes: %q", got.notified)
	}
	later(5 * time.Minute)
	e.RunDue()
	if len(got.notified) != 2 || got.notified[1] != "settled: flow: 3 saves " {
		t.Errorf("once flow's quiet: %q", got.notified)
	}
}

// at_most holds back what's past it, each key apart, and says so in its
// runs.
func TestWorkflow_AtMost(t *testing.T) {
	e, got := engine(t)
	later := clock(e)
	e.Add("red", []byte(`when = event("pr.checks_failed", at_most = "1/h", per = "pr")`+"\n"))
	red := func(pr string) {
		emit(t, e.Root, events.Event{Kind: events.PRChecksFailed, Origin: events.Heard, By: events.ByPerson("alice"), PR: pr, Title: "Red"})
	}
	red("api#1")
	red("api#1")
	red("api#2")
	red("api#1")
	e.RunDue()
	if len(got.notified) != 2 {
		t.Fatalf("notified %q", got.notified)
	}
	runs, _ := Runs(e.Root, "red", 10)
	held := 0
	for _, r := range runs {
		if r.State == Held {
			held++
			if !strings.Contains(r.Error, "at most 1 in 1h for api#1") {
				t.Errorf("held: %q", r.Error)
			}
		}
	}
	if held != 2 {
		t.Errorf("held %d, want 2: %+v", held, runs)
	}
	later(61 * time.Minute)
	red("api#1")
	if e.RunDue(); len(got.notified) != 3 {
		t.Errorf("an hour on: %q", got.notified)
	}
}

// A pace that can't be read is refused.
func TestWorkflow_PaceRefused(t *testing.T) {
	for src, want := range map[string]string{
		`when = event("pr.merged", at_most = "lots")`:                                             "at_most",
		`when = event("pr.merged", settle = "soon")`:                                              "settle",
		`when = event("pr.merged", per = "colour")`:                                               "per",
		"when = event(\"pr.merged\")\nmay = [\"notify\"]\ndef run(e):\n    wait(\"pr.closed\")\n": "",
	} {
		_, err := Check("x", []byte(src))
		if want == "" {
			if err != nil {
				t.Errorf("%q: %v", src, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want it to say %q", src, err, want)
		}
	}
	a, _ := Check("x", []byte(`when = event("pr.checks_failed", at_most = "1/h", per = "pr")`))
	if a.WhenSaid() != "pr.checks_failed (at_most=1/1h per=pr)" {
		t.Errorf("said as %q", a.WhenSaid())
	}
}

// A pass with nothing to do writes nothing: the daemon's loop is told of
// every write, its own too, so one that wrote would pass again, for good.
func TestWorkflow_IdlePassWritesNothing(t *testing.T) {
	root := t.TempDir()
	db := storetest.Own(t, root)
	e := &Engine{Root: root, Hooks: Hooks{Notify: func(*Automation, string, []string) error { return nil }}}
	for name, src := range map[string]string{
		"drawing": drawing,
		"deploy":  deploy,
		"settled": `when = event("thing.changed", kind = "sketch", settle = "10m", per = "thing")` + "\n",
	} {
		if _, err := e.Add(name, []byte(src)); err != nil {
			t.Fatal(err)
		}
	}
	emit(t, root, events.Event{Kind: events.PRMerged, Origin: events.Heard, By: events.ByPerson("alice"), PR: "api#212"})
	emit(t, root, events.Event{Kind: events.ThingChanged, Origin: events.Heard, By: events.ByPerson("ana-ortiz"), Thing: "records/sketches/q/", ThingKind: "sketch"})
	e.RunDue() // the deploy parks at its wait; the sketch settles
	told, stop := db.Watch()
	defer stop()
	for range 3 {
		e.RunDue()
	}
	select {
	case <-told:
		t.Error("a pass with nothing to do wrote")
	default:
	}
}
