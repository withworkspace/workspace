package automation

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/events"
)

// A dry run says what an automation would have done over the events kept
// lately, doing none of it: its notifications, its waits (what met them,
// and how long), what its pace held back or settled.
func TestDryRun(t *testing.T) {
	e, got := engine(t)
	base := time.Now().Add(-10 * 24 * time.Hour)
	at := func(h float64) time.Time { return base.Add(time.Duration(h * float64(time.Hour))) }
	emitAt := func(ev events.Event, h float64) { ev.At = at(h); emit(t, e.Root, ev) }
	merged := func(pr string) events.Event {
		return events.Event{Kind: events.PRMerged, Origin: events.Heard, By: events.ByPerson("alice"), PR: pr, Title: pr}
	}
	emitAt(merged("api#1"), 0)
	emitAt(events.Event{Kind: events.RunEnded, Run: "deploy.ground.api", Title: "deploy", Status: "passed"}, 0.25)
	emitAt(merged("api#2"), 24)
	emitAt(merged("api#3"), 48)
	emitAt(events.Event{Kind: events.RunEnded, Run: "deploy.ground.api", Title: "deploy", Status: "failed (exit 1)"}, 49)

	a, err := Check("deploy", []byte(deploy))
	if err != nil {
		t.Fatal(err)
	}
	dr, err := e.DryRun(a, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Matched != 3 || dr.Ran != 3 || len(dr.Lines) != 3 || len(got.notified) != 0 {
		t.Fatalf("dry run %+v, notified %q", dr, got.notified)
	}
	want := []struct{ did, waited string }{
		{"notify: deployed api#1 (Idempotency keys)", "waited 15m for e2"},
		{"notify: not deployed api#2", "then timed out"},
		{"notify: not deployed api#3", "waited 1h for e5"},
	}
	for i, w := range want {
		l := dr.Lines[i]
		if len(l.Did) != 1 || l.Did[0] != w.did || len(l.Waited) != 1 || !strings.Contains(l.Waited[0], w.waited) {
			t.Errorf("line %d: %+v, want %s · %s", i, l, w.did, w.waited)
		}
	}
	if runs, _ := Runs(e.Root, "", 10); len(runs) != 0 {
		t.Errorf("a dry run kept runs: %+v", runs)
	}

	// Pace: at_most holds back, settle batches.
	for i := range 3 {
		emitAt(events.Event{Kind: events.PRChecksFailed, Origin: events.Heard, By: events.ByPerson("bob"), PR: "api#9"}, 50+float64(i)*0.1)
	}
	red, _ := Check("red", []byte(`when = event("pr.checks_failed", at_most = "1/h", per = "pr")`+"\n"))
	if dr, _ = e.DryRun(red, 30*24*time.Hour); dr.Ran != 1 || dr.Held != 2 || !strings.Contains(dr.Lines[1].Held, "at most 1 in 1h for api#9") {
		t.Errorf("at_most: %+v", dr)
	}
	for i := range 4 {
		emitAt(events.Event{Kind: events.ThingChanged, Origin: events.Heard, By: events.ByPerson("ana"), Thing: "flow", ThingKind: "sketch"}, 60+float64(i)*0.05)
	}
	emitAt(events.Event{Kind: events.ThingChanged, Origin: events.Heard, By: events.ByPerson("ana"), Thing: "flow", ThingKind: "sketch"}, 70)
	saves, _ := Check("saves", []byte(`when = event("thing.changed", settle = "10m", per = "thing")
may = ["notify"]
def run(e):
    notify("%d saves" % len(e.batch))
`))
	dr, _ = e.DryRun(saves, 30*24*time.Hour)
	if dr.Matched != 5 || dr.Ran != 2 || dr.Lines[0].Did[0] != "notify: 4 saves" || dr.Lines[1].Did[0] != "notify: 1 saves" {
		t.Errorf("settle: %+v", dr)
	}
}

// The guide says every kind of event there is.
func TestGuideSaysEveryKind(t *testing.T) {
	g := Guide()
	for _, k := range events.Kinds {
		if !strings.Contains(g, "- "+k+": ") {
			t.Errorf("the guide doesn't say %s", k)
		}
	}
}
