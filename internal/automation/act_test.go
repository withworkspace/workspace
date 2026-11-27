package automation

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/events"
)

const dustAll = `# Dust what's added to the shelf; and dust one when I ask.
when = [event("thing.added", kind = "sketch"), action("Dust it", on = "frame-fixture:thing")]
may = ["frame-fixture:things"]

def run(e):
    p = act("frame-fixture:things", [{"thing": e.on or "THING-1", "note": "Dusted."}], why = "It was dusty")
    wait(p, timeout = "1d")
`

// act is done as the run may: on what happened, accepted at once where
// you let it (granted), else proposed; on an action you ran, put in front
// of you to confirm, grant or not. Either way, its handle is waited on as
// propose's is.
func TestAct_AsTheRunMay(t *testing.T) {
	e, _ := engine(t)
	var acted []string
	states := map[string][2]string{}
	e.Hooks.Act = func(a *Automation, p Proposing, how Acting, in string, chain []string) (string, error) {
		ref := "w" + string(rune('1'+len(acted))) + "#1"
		acted = append(acted, p.Kind+" accept="+map[bool]string{true: "yes", false: "no"}[how.Accept]+" confirm="+map[bool]string{true: "yes", false: "no"}[how.Confirm])
		states[ref] = [2]string{ProposalOpen, ""}
		if how.Accept {
			states[ref] = [2]string{"accepted", "THING-1"}
		}
		return ref, nil
	}
	e.Hooks.Proposal = func(ref string) (string, string, bool) { st, ok := states[ref]; return st[0], st[1], ok }
	if _, err := e.Add("dust", []byte(dustAll)); err != nil {
		t.Fatal(err)
	}
	emit(t, e.Root, sketch)
	e.RunDue()
	if err := Grant(e.Root, "dust", "frame-fixture:things", true, time.Now()); err != nil {
		t.Fatal(err)
	}
	emit(t, e.Root, sketch)
	e.RunDue()
	RunAction(e.Root, "dust", "Dust it", "THING-2", nil)
	e.RunDue()
	want := "frame-fixture:things accept=no confirm=no|frame-fixture:things accept=yes confirm=no|frame-fixture:things accept=no confirm=yes"
	if strings.Join(acted, "|") != want {
		t.Fatalf("acted %q", acted)
	}
	runs, _ := Runs(e.Root, "dust", 3)
	got := map[string]int{}
	for _, r := range runs {
		got[r.State]++
	}
	if got[Ran] != 1 || got[Waiting] != 2 {
		t.Errorf("runs: %+v", runs)
	}
	if g, _ := Grants(e.Root, "dust"); strings.Join(g, ",") != "frame-fixture:things" {
		t.Errorf("grants %q", g)
	}
}

func TestAct_Grants(t *testing.T) {
	e, _ := engine(t)
	e.Add("dust", []byte(dustAll))
	if err := Grant(e.Root, "dust", "jira:changes", true, time.Now()); err == nil || !strings.Contains(err.Error(), "may not act on jira:changes") {
		t.Errorf("granted what it may not: %v", err)
	}
	if err := Grant(e.Root, "nope", "frame-fixture:things", true, time.Now()); err != ErrNotFound {
		t.Errorf("an automation there isn't: %v", err)
	}
	Grant(e.Root, "dust", "frame-fixture:things", true, time.Now())
	// Revised to ask for something else, what it no longer asks for goes.
	e.Add("dust", []byte(strings.ReplaceAll(dustAll, "frame-fixture:things", "jira:changes")))
	if g, _ := Grants(e.Root, "dust"); len(g) != 0 {
		t.Errorf("kept %q", g)
	}
	e.Add("dust", []byte(dustAll))
	Grant(e.Root, "dust", "frame-fixture:things", true, time.Now())
	e.Remove("dust")
	if g, _ := Grants(e.Root, "dust"); len(g) != 0 {
		t.Errorf("removed, kept %q", g)
	}
}

func TestAct_Refused(t *testing.T) {
	for script, want := range map[string]string{
		"may = [\"frame-fixture:things\"]\ndef run(e):\n    act(\"mission\", {\"title\": \"x\"})\n": "is not a plugin's kind",
		"may = [\"notify\"]\ndef run(e):\n    act(\"frame-fixture:things\", [])\n":                  `may = ["frame-fixture:things"]`,
		"may = [\"frame-fixture:things\"]\ndef run(e):\n    act(\"frame-fixture:things\")\n":        "missing argument for input",
	} {
		e, _ := engine(t)
		e.Hooks.Act = func(*Automation, Proposing, Acting, string, []string) (string, error) { return "w1#1", nil }
		if _, err := e.Add("refused", []byte("when = event(\"pr.merged\")\n"+script)); err != nil {
			t.Fatal(err)
		}
		emit(t, e.Root, events.Event{Kind: events.PRMerged, PR: "api#1"})
		e.RunDue()
		runs, _ := Runs(e.Root, "refused", 1)
		if len(runs) != 1 || runs[0].State != Failed || !strings.Contains(runs[0].Error, want) {
			t.Errorf("want %q: %+v", want, runs)
		}
	}
	if _, err := Check("bad", []byte("when = event(\"pr.merged\")\nmay = [\"jira\"]\ndef run(e):\n    pass\n")); err == nil || !strings.Contains(err.Error(), "or a plugin's kind of proposal") {
		t.Errorf("a may that's neither: %v", err)
	}
}

// A dry run acts on nothing: it says what it'd do, and how.
func TestAct_DryRun(t *testing.T) {
	e, _ := engine(t)
	e.Hooks.Act = func(*Automation, Proposing, Acting, string, []string) (string, error) {
		t.Fatal("a dry run acted")
		return "", nil
	}
	emit(t, e.Root, sketch)
	a, _ := Check("dust", []byte(dustAll))
	dr, err := e.DryRun(a, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Ran != 1 || !strings.Contains(strings.Join(dr.Lines[0].Did, "|"), "act frame-fixture:things (proposed: not allowed without asking)") {
		t.Fatalf("%+v", dr.Lines)
	}
}
