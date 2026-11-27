package automation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const ciRed = `# When CI fails on a pull request, find out why, and say when it's green.
when = event("pr.checks_failed", repo = "api", at_most = "1/h", per = "pr")
may = ["notify", "converse"]

def say(text):
    notify(text, open = e_pr)

def run(e):
    pr = show(e.pr)
    if pr == None:
        return
    elif e.title.startswith("WIP"):
        log("skipped a WIP")
    else:
        converse("Why did CI fail on %s (%s)?" % (e.pr, e.title), about = [e.pr])
    green = wait(event("pr.checks_passed", pr = e.pr), timeout = "2h")
    if green == None:
        notify("%s is still red after 2h" % e.pr)
    else:
        say("%s is green again" % e.pr)
    for c in e.commits:
        n = len(c)
    total = 0
`

// An outline is the script as what it does: its when, its reads, its
// ifs as branches (elif and else too), what it notifies and the
// conversations it starts with their words, a wait split into met and its
// time up, a for, a helper drawn where it's called, and code as written.
func TestOutline_DrawsTheScript(t *testing.T) {
	a, err := Check("ci-red", []byte(strings.Replace(ciRed, "e_pr", "\"api#1\"", 1)))
	if err != nil {
		t.Fatal(err)
	}
	o, err := OutlineOf(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(o.When) != 1 || o.When[0].Match != "pr.checks_failed repo=api" || o.When[0].Pace != "at_most=1/1h per=pr" {
		t.Errorf("when: %+v", o.When)
	}
	var kinds []string
	for _, s := range o.Steps {
		kinds = append(kinds, s.Kind)
	}
	if strings.Join(kinds, " ") != "read if wait for code" {
		b, _ := json.MarshalIndent(o.Steps, "", "  ")
		t.Fatalf("steps %v:\n%s", kinds, b)
	}
	read := o.Steps[0]
	if read.Text != "show(e.pr)" || read.Detail != "pr" || read.At != "9:14" {
		t.Errorf("the read: %+v", read)
	}
	branches := o.Steps[1].Branches
	if len(branches) != 3 || branches[0].Label != "if pr == None" || branches[0].Steps[0].Kind != "stop" ||
		branches[1].Label != `elif e.title.startswith("WIP")` || branches[1].Steps[0].Kind != "log" || branches[1].Steps[0].Text != "skipped a WIP" ||
		branches[2].Label != "else" || branches[2].Steps[0].Text != "Why did CI fail on {e.pr} ({e.title})?" || branches[2].Steps[0].Detail != "about [e.pr]" {
		t.Errorf("the if: %+v", branches)
	}
	wait := o.Steps[2]
	if wait.Text != "pr.checks_passed pr={e.pr}" || wait.Detail != "up to 2h" || len(wait.Branches) != 2 ||
		wait.Branches[0].Label != "met" || wait.Branches[0].Steps[0].Text != "{e.pr} is green again" ||
		wait.Branches[1].Label != "time's up" || wait.Branches[1].Steps[0].Text != "{e.pr} is still red after 2h" {
		t.Errorf("the wait: %+v", wait)
	}
	if f := o.Steps[3]; f.Branches[0].Label != "for each c in e.commits" || f.Branches[0].Steps[0].Text != "n = len(c)" {
		t.Errorf("the for: %+v", f)
	}
	if c := o.Steps[4]; c.Text != "total = 0" {
		t.Errorf("code: %+v", c)
	}
}

// With no run, the event's notified as it is.
func TestOutline_NoRun(t *testing.T) {
	a, err := Check("plain", []byte(`when = event("pr.merged")`))
	if err != nil {
		t.Fatal(err)
	}
	if o, _ := OutlineOf(a); !o.Default || len(o.Steps) != 0 {
		t.Errorf("%+v", o)
	}
}

// A run keeps where each step was called from as its outline has it, so
// what its runs did is counted on its outline's steps: a run's and a dry
// run's alike.
func TestOutline_CountsLandOnItsSteps(t *testing.T) {
	e, _ := engine(t)
	if _, err := e.Add("drawing", []byte(drawing)); err != nil {
		t.Fatal(err)
	}
	emit(t, e.Root, sketch)
	emit(t, e.Root, sketch)
	e.RunDue()
	a, _ := Load(e.Root, "drawing")
	o, err := OutlineOf(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Steps) != 1 || o.Steps[0].Kind != "notify" {
		t.Fatalf("outline %+v", o.Steps)
	}
	counts, _, err := Counts(e.Root, "drawing")
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 1 || counts[0].At != o.Steps[0].At || counts[0].Done != 2 {
		t.Errorf("counts %+v, the step at %s", counts, o.Steps[0].At)
	}
	dry, err := e.DryRun(a, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(dry.Steps) != 1 || dry.Steps[0].At != o.Steps[0].At || dry.Steps[0].Done != 2 {
		t.Errorf("dry run's counts %+v", dry.Steps)
	}
}

// A failed run is counted where it failed.
func TestOutline_FailedAt(t *testing.T) {
	if got := failedAt("ci-red", "ci-red.star:6:9: show: got int, want string"); got != "6:9" {
		t.Errorf("%q", got)
	}
	if got := failedAt("ci-red", "it took longer than 10s"); got != "" {
		t.Errorf("%q", got)
	}
}
