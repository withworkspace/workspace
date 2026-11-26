package ops

import (
	"strings"
	"testing"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/plugin"
)

// A Wait whose until is an event ws hears of is passed as soon as one's
// kept, saying which: not by an event it doesn't wait on, and not before.
func TestWaitUntilHeard(t *testing.T) {
	w, host, _ := atWait(t, "   - on: alice\n   - until: pr.merged pr=api#212\n")
	if said := w.CheckWaits(); len(said) != 0 {
		t.Fatalf("passed before it happened: %q", said)
	}
	st := stepOf(t, w, 2)
	if st.Accepted() || st.Watch().Summary() != "not yet" || !strings.Contains(st.Watch().Results[0].Evidence, "hasn't heard of it") {
		t.Fatalf("after a look: %+v", st.Watch())
	}
	last := w.LatestEvent()

	events.Emit(w.Root(), events.Event{Kind: events.PRMerged, Origin: events.Heard, By: events.ByPerson("alice"), Repo: "api", PR: "api#211"})
	said, last := w.CheckWaitsHeard(last)
	if len(said) != 0 || stepOf(t, w, 2).Accepted() {
		t.Fatalf("passed by another pull request merging: %q", said)
	}

	e, _ := events.Emit(w.Root(), events.Event{Kind: events.PRMerged, Origin: events.Heard, By: events.ByPerson("alice"), Repo: "api", PR: "api#212", Title: "Idempotency"})
	said, _ = w.CheckWaitsHeard(last)
	if len(said) != 1 || !strings.Contains(said[0], "step 2") {
		t.Fatalf("not passed when it merged: %q", said)
	}
	st = stepOf(t, w, 2)
	if !st.Accepted() || st.Record.AcceptedBy != "ws" || !strings.Contains(st.Record.Why, "`pr.merged pr=api#212` is met") {
		t.Errorf("step 2 = %+v", st.Record)
	}
	if r := st.Watch().Results; len(r) != 1 || !strings.HasPrefix(r[0].Evidence, e.Ref()+": pr.merged api#212 (Idempotency)") {
		t.Errorf("what showed it: %+v", r)
	}
	if len(host.specs) != 1 {
		t.Errorf("the step after it didn't start: %d crew", len(host.specs))
	}
}

// A thing that's as the until asks when the mission reaches the step
// passes it at its first look: it's so already.
func TestWaitUntilStandsAlready(t *testing.T) {
	w, _, _ := atWait(t, "   - on: product\n   - until: thing.changed kind=jira:issue thing=RAM-881 status=Done\n")
	issue := protocol.Ref{Kind: "issue", Prefixes: []string{"RAM-"}, Fields: []protocol.Field{{Name: "title", Type: protocol.FieldTitle}, {Name: "status", Type: protocol.FieldState}}}
	if _, err := plugin.KeepRefs(w.Root(), "jira", []protocol.Ref{issue}); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.PutThings(w.Root(), "jira", []protocol.Thing{{Kind: "issue", Ref: "RAM-881", Fields: map[string]string{"title": "Sign off the flow", "status": "Done"}}}); err != nil {
		t.Fatal(err)
	}
	said := w.CheckWaits()
	if len(said) != 1 || !stepOf(t, w, 2).Accepted() {
		t.Fatalf("not passed, though it's so: %q", said)
	}
	if r := stepOf(t, w, 2).Watch().Results; !strings.Contains(r[0].Evidence, "RAM-881's status Done already") {
		t.Errorf("what showed it: %+v", r)
	}
}

