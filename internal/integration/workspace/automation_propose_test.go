package workspace_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/automation"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

const proposedDrawing = `# Notify me if Brendan pushes a new drawing.
when = event("thing.added", kind = "sketch", by = "brendan")
may = ["notify"]

def run(e):
    notify("%s drew %s" % (e.by.short, e.title), open = e.thing)
`

// A conversation's agent proposes an automation (ws automation add
// --propose): ws checks it, dry-runs it over what's happened lately, and
// keeps that on the proposal for its card; told to add one itself, it's
// refused. Accepted, it's added, on.
func TestAutomation_ProposedInATalkAndAccepted(t *testing.T) {
	w := testutil.MissionWorkspace(t)
	talks := conversation.ForWorkspace(w.Root)
	talks.OpenTalk(conversation.You, "Tell me when Brendan draws something", time.Now())
	for _, by := range []string{"brendan-ash", "ana-ortiz", "brendan-ash"} {
		if _, err := events.Emit(w.Root, events.Event{Kind: events.ThingAdded, Origin: events.Heard, By: events.ByPerson(by), Thing: "records/sketches/x/", ThingKind: "sketch", Title: "Flow"}); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(conversation.EnvConversation, "w1")
	t.Setenv(mission.EnvCrew, conversation.Agent)

	guide := testutil.Run(t, w, nil, "automation", "add", "--propose")
	for _, want := range []string{"when = event(", "- thing.added:", "wait(event(", "ws automation add --propose --name"} {
		if !strings.Contains(guide.Stdout, want) {
			t.Errorf("the guide lacks %q", want)
		}
	}
	path := t.TempDir() + "/drawing.star"
	os.WriteFile(path, []byte(proposedDrawing), 0o644)
	if r := testutil.RunCommand(t, w.Root, nil, "automation", "add", path); r.Err == nil || !strings.Contains(r.Err.Error(), "propose an automation with --propose") {
		t.Errorf("added by the agent itself: %v", r.Err)
	}
	testutil.WithStdin(t, `when = event("pr.mergd")`)
	if r := testutil.RunCommand(t, w.Root, nil, "automation", "add", "--propose", "--name", "bad", "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "not proposed: bad.star:1:") || !strings.Contains(r.Err.Error(), `"pr.mergd" isn't a kind of event`) {
		t.Errorf("a bad one: %v", r.Err)
	}
	testutil.WithStdin(t, proposedDrawing)
	r := testutil.Run(t, w, nil, "automation", "add", "--propose", "--name", "drawing", "-")
	if !strings.Contains(r.Stderr, "proposed in w1 (proposal 1)") || !strings.Contains(r.Stderr, "it would have run 2 times") {
		t.Errorf("proposing:\n%s", r.Stderr)
	}
	c, _ := talks.Load("w1")
	p := c.Proposal(1)
	if p == nil || p.Kind != conversation.ProposeAutomation || p.Automation == nil || p.Automation.Name != "drawing" ||
		p.Automation.About != "Notify me if Brendan pushes a new drawing." || p.Automation.DryRun.Ran != 2 ||
		p.Automation.DryRun.Lines[0].Did[0] != "notify: Brendan drew Flow" {
		t.Fatalf("proposal: %+v", p)
	}
	if _, err := os.Stat(automation.Path(w.Root, "drawing")); err == nil {
		t.Error("proposing added it")
	}

	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")
	testutil.Run(t, w, nil, "conversation", "accept", "w1", "1")
	states, _ := automation.States(w.Root)
	if st, ok := states["drawing"]; !ok || !st.On {
		t.Errorf("accepted, not added on: %+v", states)
	}
	c, _ = talks.Load("w1")
	if p := c.Proposal(1); p.State != conversation.ProposalAccepted || p.Result != "drawing" {
		t.Errorf("the proposal isn't settled: %s %q", p.State, p.Result)
	}
}
