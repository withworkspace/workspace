package automation

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/events"
)

const sketchMission = `# A new sketch from Brendan becomes a mission, if I say so.
when = event("thing.added", kind = "sketch")
may = ["propose", "notify"]

def run(e):
    p = propose("mission", {"title": "Build " + e.title, "about": [e.thing]}, why = "Brendan drew " + e.title)
    propose("decision", {"what": "We build " + e.title + " next"})
    r = wait(p, timeout = "3d")
    if r and r.accepted:
        notify("started " + r.result)
    elif r:
        notify("you " + r.state + " it")
    else:
        notify("no word on " + p.ref)
`

// fakeProposals is a conversation store's proposals, as the hooks see
// them: what was proposed where, and how each stands.
type fakeProposals struct {
	made   []string
	states map[string][2]string
}

func (f *fakeProposals) hooks(e *Engine) {
	f.states = map[string][2]string{}
	convs := 0
	e.Hooks.Propose = func(a *Automation, p Proposing, in string, chain []string) (string, error) {
		if in == "" {
			convs++
			in = fmt.Sprintf("w%d", convs)
		}
		n := 1
		for _, m := range f.made {
			if strings.HasPrefix(m, in+"#") {
				n++
			}
		}
		ref := fmt.Sprintf("%s#%d", in, n)
		f.made = append(f.made, fmt.Sprintf("%s %s (%s; %s; %s)", ref, p.Says(), p.Why, strings.Join(chain, ">"), p.On))
		f.states[ref] = [2]string{ProposalOpen, ""}
		return ref, nil
	}
	e.Hooks.Proposal = func(ref string) (string, string, bool) {
		st, ok := f.states[ref]
		return st[0], st[1], ok
	}
}

// What a run proposes goes in one conversation, at once; a wait on it
// parks the run till you've said, and is given what you said and what
// accepting it made.
func TestPropose_WaitsForYou(t *testing.T) {
	e, got := engine(t)
	later := clock(e)
	f := &fakeProposals{}
	f.hooks(e)
	if _, err := e.Add("sketch-mission", []byte(sketchMission)); err != nil {
		t.Fatal(err)
	}
	ev := emit(t, e.Root, sketch)
	e.RunDue()
	if len(f.made) != 2 || f.made[0] != fmt.Sprintf("w1#1 a mission: Build Queues (Brendan drew Queues; sketch-mission; e%d: %s)", ev.N, ev.Says()) || !strings.HasPrefix(f.made[1], "w1#2 a decision: We build Queues next") {
		t.Fatalf("made %q", f.made)
	}
	if runs, _ := Runs(e.Root, "sketch-mission", 1); runs[0].State != Waiting || len(got.notified) != 0 {
		t.Fatalf("not parked: %+v %q", runs, got.notified)
	}
	f.states["w1#1"] = [2]string{"accepted", "build-queues"}
	e.RunDue()
	if strings.Join(got.notified, "|") != "sketch-mission: started build-queues " {
		t.Fatalf("notified %q", got.notified)
	}
	if len(f.made) != 2 {
		t.Errorf("proposed again: %q", f.made)
	}

	// Declined, and never said.
	emit(t, e.Root, sketch)
	e.RunDue()
	f.states["w2#1"] = [2]string{"declined", ""}
	e.RunDue()
	emit(t, e.Root, sketch)
	e.RunDue()
	later(4 * 24 * time.Hour)
	e.RunDue()
	if strings.Join(got.notified[1:], "|") != "sketch-mission: you declined it |sketch-mission: no word on w3#1 " {
		t.Errorf("notified %q", got.notified)
	}
}

func TestPropose_Refused(t *testing.T) {
	for input, want := range map[string]string{
		`"mission", {"brief": "x"}`:              "a mission needs a title",
		`"mission", {"title": "x", "slug": "y"}`: "a mission has no slug",
		`"decision", {"what": 3}`:                "a decision's what must be text",
		`"plan", {"plan": "x"}`:                  `"plan" is not something an automation can propose`,
		`"mission", "Fix it"`:                    "the proposal is a dict",
	} {
		e, _ := engine(t)
		(&fakeProposals{}).hooks(e)
		script := "when = event(\"pr.merged\")\nmay = [\"propose\"]\ndef run(e):\n    propose(" + input + ")\n"
		if _, err := e.Add("refused", []byte(script)); err != nil {
			t.Fatal(err)
		}
		emit(t, e.Root, events.Event{Kind: events.PRMerged, PR: "api#1"})
		e.RunDue()
		runs, _ := Runs(e.Root, "refused", 1)
		if len(runs) != 1 || runs[0].State != Failed || !strings.Contains(runs[0].Error, want) {
			t.Errorf("%s: want %q: %+v", input, want, runs)
		}
	}
}

// A plugin's kind is the plugin's to read: what the script gives is
// handed on as it is.
func TestPropose_PluginKind(t *testing.T) {
	e, _ := engine(t)
	var given []Proposing
	e.Hooks.Propose = func(a *Automation, p Proposing, in string, chain []string) (string, error) {
		given = append(given, p)
		return "w1#1", nil
	}
	script := `when = event("pr.merged")
may = ["propose"]
def run(e):
    propose("jira:changes", {"changes": [{"issue": "RAM-881", "comment": "Merged in " + e.pr, "done": True}]})
`
	e.Add("comment", []byte(script))
	emit(t, e.Root, events.Event{Kind: events.PRMerged, PR: "api#212"})
	e.RunDue()
	if len(given) != 1 || given[0].Kind != "jira:changes" {
		t.Fatalf("given %+v", given)
	}
	ch := given[0].Fields()["changes"].([]any)[0].(map[string]any)
	if ch["issue"] != "RAM-881" || ch["comment"] != "Merged in api#212" || ch["done"] != true {
		t.Errorf("input %+v", given[0].Input)
	}
}

// A dry run proposes nothing, and doesn't guess what you'd say to it.
func TestPropose_DryRun(t *testing.T) {
	e, _ := engine(t)
	e.Hooks.Propose = func(*Automation, Proposing, string, []string) (string, error) {
		t.Fatal("a dry run proposed")
		return "", nil
	}
	emit(t, e.Root, sketch)
	a, err := Check("sketch-mission", []byte(sketchMission))
	if err != nil {
		t.Fatal(err)
	}
	dr, err := e.DryRun(a, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	l := dr.Lines[0]
	if strings.Join(l.Did, "|") != "propose a mission: Build Queues|propose a decision: We build Queues next" || len(l.Waited) != 1 || !strings.Contains(l.Waited[0], "accept or decline") || dr.Waiting != 1 {
		t.Fatalf("%+v", dr)
	}
}

const askedAndAnswered = `# Ask a conversation, then tell me what it said.
when = event("pr.merged")
may = ["converse", "notify"]

def run(e):
    c = converse("What changed in " + e.pr + "?")
    a = wait(event("conversation.answered", conversation = c), timeout = "1h")
    notify(a.text if a else "no answer")
`

// A wait is met by what's said in a conversation the automation started:
// what it waits for there is an answer, not its own doing. Its own
// conversations still don't set it off.
func TestWorkflow_WaitsOnItsOwnConversation(t *testing.T) {
	e, got := engine(t)
	e.Add("asked", []byte(askedAndAnswered))
	emit(t, e.Root, events.Event{Kind: events.PRMerged, PR: "api#212"})
	e.RunDue()
	emit(t, e.Root, events.Event{Kind: events.ConversationAnswered, Conversation: "w9", By: events.ByWS, Text: "It adds idempotency keys."})
	e.RunDue()
	if strings.Join(got.notified, "|") != "asked: It adds idempotency keys. " {
		t.Fatalf("notified %q", got.notified)
	}
	if len(got.conversed) != 1 {
		t.Errorf("conversed %q", got.conversed)
	}
}
