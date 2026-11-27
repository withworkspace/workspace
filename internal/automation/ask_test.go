package automation

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/events"
)

const flaky = `# When CI fails on a PR, ask whether it's a flake.
when = event("pr.checks_failed")
may = ["ask", "notify"]

def run(e):
    a = ask("Is this failure a flake?\n" + e.text, answer = {"flaky": "bool", "why": "text", "next": ["rerun", "fix"]}, about = [e.pr])
    if a == None:
        notify("no answer for " + e.pr)
    elif a.flaky:
        notify("flake: %s (%s)" % (a.why, a.next))
    else:
        notify("real: " + a.why)
`

var checksFailed = events.Event{Kind: events.PRChecksFailed, Origin: events.Heard, By: events.ByPerson("alice"), PR: "api#212", Text: "TestQueue timed out"}

// An ask parks its run while ws has an agent answer it, beside the pass:
// the answer wakes the run, which is given it in the shape it asked for.
// The agent's asked once, with the question, what ws shows of what it's
// about, and how to answer; what's spent counts against the budget.
func TestAsk_ParksAndAnswers(t *testing.T) {
	e, got := engine(t)
	var prompts []string
	var since time.Time
	e.Hooks.Spent = func(name string, from time.Time) (int64, error) { since = from; return 1000, nil }
	e.Hooks.Ask = func(a *Automation, prompt string) (string, error) {
		prompts = append(prompts, a.Name+": "+prompt)
		return "Here it is:\n```json\n{\"flaky\": true, \"why\": \"setup timed out\", \"next\": \"Rerun\"}\n```", nil
	}
	if _, err := e.Add("flaky", []byte(flaky)); err != nil {
		t.Fatal(err)
	}
	emit(t, e.Root, checksFailed)
	e.RunDue()
	if runs, _ := Runs(e.Root, "flaky", 1); len(runs) != 1 || runs[0].State != Waiting {
		t.Fatalf("not parked on its ask: %+v", runs)
	}
	e.Quiet()
	e.RunDue()
	if strings.Join(got.notified, "|") != "flaky: flake: setup timed out (rerun) " {
		t.Fatalf("notified %q", got.notified)
	}
	e.RunDue()
	if len(prompts) != 1 {
		t.Fatalf("asked %d times", len(prompts))
	}
	for _, want := range []string{"flaky: An automation", "Is this failure a flake?\nTestQueue timed out", "api#212:", "Idempotency keys", "- flaky: true or false", `- next: one of "rerun", "fix"`, "- why: a string"} {
		if !strings.Contains(prompts[0], want) {
			t.Errorf("prompt has no %q:\n%s", want, prompts[0])
		}
	}
	if !since.Equal(dayStart(time.Now())) {
		t.Errorf("budget counted from %s", since)
	}
	if runs, _ := Runs(e.Root, "flaky", 1); runs[0].State != Ran {
		t.Errorf("run %+v", runs[0])
	}
}

// An answer that isn't in the shape asked for is None: the script says
// what happens then, and the run's log says why.
func TestAsk_NoAnswer(t *testing.T) {
	e, got := engine(t)
	e.Hooks.Ask = func(*Automation, string) (string, error) {
		return `{"flaky": "maybe", "why": "hm", "next": "fix"}`, nil
	}
	e.Add("flaky", []byte(flaky))
	emit(t, e.Root, checksFailed)
	e.RunDue()
	e.Quiet()
	e.RunDue()
	if strings.Join(got.notified, "|") != "flaky: no answer for api#212 " {
		t.Fatalf("notified %q", got.notified)
	}
	runs, _ := Runs(e.Root, "flaky", 1)
	if runs[0].State != Ran || !strings.Contains(runs[0].Log, "no answer: answer's flaky is not true or false") {
		t.Errorf("run %+v", runs[0])
	}
}

// An automation that's spent its tokens for the day asks no one: its run
// fails, saying so, and you're told.
func TestAsk_OverBudget(t *testing.T) {
	e, got := engine(t)
	e.AskBudget = 50_000
	asked := 0
	e.Hooks.Ask = func(*Automation, string) (string, error) { asked++; return "", nil }
	e.Hooks.Spent = func(string, time.Time) (int64, error) { return 61_200, nil }
	e.Add("flaky", []byte(flaky))
	emit(t, e.Root, checksFailed)
	e.RunDue()
	e.Quiet()
	e.RunDue()
	runs, _ := Runs(e.Root, "flaky", 1)
	if asked != 0 || runs[0].State != Failed || !strings.Contains(runs[0].Error, "over its budget: spent 61.2k of 50k tokens today") {
		t.Fatalf("asked %d, run %+v", asked, runs[0])
	}
	if len(got.notified) != 1 || !strings.Contains(got.notified[0], "could not run") {
		t.Errorf("told %q", got.notified)
	}
}

// An ask ws stopped before was answered is asked again: one it gave up
// on itself (the daemon's life ended), and one left running by a ws
// that's gone.
func TestAsk_AskedAgain(t *testing.T) {
	e, got := engine(t)
	asked := 0
	e.Hooks.Ask = func(*Automation, string) (string, error) {
		asked++
		if asked == 1 {
			return "", ErrAskAgain
		}
		return `{"flaky": false, "why": "a real race", "next": "fix"}`, nil
	}
	e.Add("flaky", []byte(flaky))
	emit(t, e.Root, checksFailed)
	e.RunDue()
	e.Quiet()
	if asks, _ := Asks(e.Root, TaskQueued); len(asks) != 1 {
		t.Fatalf("not queued again: %+v", asks)
	}
	// Left running by a ws that's gone: this one asks it again.
	asks, _ := Asks(e.Root, TaskQueued)
	_ = setAsk(e.Root, asks[0].ID, TaskRunning, time.Now())
	fresh := &Engine{Root: e.Root, Hooks: e.Hooks}
	fresh.RunDue()
	fresh.Quiet()
	fresh.RunDue()
	if asked != 2 || strings.Join(got.notified, "|") != "flaky: real: a real race " {
		t.Fatalf("asked %d, notified %q", asked, got.notified)
	}
}

// Its agent failing is no answer, not the run's failure.
func TestAsk_AgentFails(t *testing.T) {
	e, got := engine(t)
	e.Hooks.Ask = func(*Automation, string) (string, error) { return "", errors.New("exit 1: rate limited") }
	e.Add("flaky", []byte(flaky))
	emit(t, e.Root, checksFailed)
	e.RunDue()
	e.Quiet()
	e.RunDue()
	if strings.Join(got.notified, "|") != "flaky: no answer for api#212 " {
		t.Fatalf("notified %q", got.notified)
	}
}

// A dry run asks no one: it says what it would ask, and the ask is None.
func TestAsk_DryRun(t *testing.T) {
	e, _ := engine(t)
	e.Hooks.Ask = func(*Automation, string) (string, error) { t.Fatal("a dry run asked"); return "", nil }
	emit(t, e.Root, checksFailed)
	a, err := Check("flaky", []byte(flaky))
	if err != nil {
		t.Fatal(err)
	}
	dr, err := e.DryRun(a, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Ran != 1 || strings.Join(dr.Lines[0].Did, "|") != "ask: Is this failure a flake? TestQueue timed out|notify: no answer for api#212" {
		t.Fatalf("%+v", dr.Lines)
	}
	if asks, _ := Asks(e.Root, ""); len(asks) != 0 {
		t.Errorf("a dry run queued %+v", asks)
	}
}

func TestAsk_Shapes(t *testing.T) {
	shape := Shape{"n": "int", "x": "number", "ok": "bool", "said": "text", "verdict": []string{"fix", "flake"}}
	got, err := readAnswer(`{"n": 3, "x": 2.5, "ok": false, "said": "hi", "verdict": "Flake", "more": 1}`, shape)
	if err != nil {
		t.Fatal(err)
	}
	m := got.(map[string]any)
	if m["n"] != 3.0 || m["x"] != 2.5 || m["ok"] != false || m["said"] != "hi" || m["verdict"] != "flake" || m["more"] != nil {
		t.Errorf("read %+v", m)
	}
	for said, want := range map[string]string{
		`{"n": 3.5, "x": 1, "ok": true, "said": "", "verdict": "fix"}`: "n is not a whole number",
		`{"x": 1, "ok": true, "said": "", "verdict": "fix"}`:           "has no n",
		`{"n": 1, "x": 1, "ok": true, "said": "", "verdict": "later"}`: `verdict is not one of "fix", "flake"`,
		`I think it's a flake.`: "did not answer with a JSON object",
		``:                      "said nothing",
	} {
		if _, err := readAnswer(said, shape); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", said, err, want)
		}
	}
	if got, _ := readAnswer("  Words.\n", nil); got != "Words." {
		t.Errorf("words: %q", got)
	}
}

func TestAsk_Refused(t *testing.T) {
	e, _ := engine(t)
	for script, want := range map[string]string{
		`when = event("pr.merged")
may = ["notify"]
def run(e):
    ask("why?")
`: `may = ["ask"]`,
		`when = event("pr.merged")
may = ["ask"]
def run(e):
    ask("why?", answer = {"ok": "boolean"})
`: `ok is "boolean"`,
		`when = event("pr.merged")
may = ["ask"]
def run(e):
    ask("why?", answer = {"ok": ["one"]})
`: "two or more choices",
	} {
		name := "refused"
		if _, err := e.Add(name, []byte(script)); err != nil {
			t.Fatal(err)
		}
		ev := emit(t, e.Root, events.Event{Kind: events.PRMerged, PR: "api#1"})
		e.RunDue()
		runs, _ := Runs(e.Root, name, 1)
		if len(runs) != 1 || runs[0].Event != ev.N || runs[0].State != Failed || !strings.Contains(runs[0].Error, want) {
			t.Errorf("want %q: %+v", want, runs)
		}
	}
}
