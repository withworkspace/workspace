package automation

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

const drawing = `# Notify me if Brendan pushes a new drawing.
when = event("thing.added", kind = "sketch", by = "brendan")
may = ["notify"]

def run(e):
    notify("%s drew %s" % (e.by.short, e.title), open = e.thing)
`

// A script is read for its when, may and run, and one that can't be used
// is refused, saying why.
func TestCheck(t *testing.T) {
	a, err := Check("drawing", []byte(drawing))
	if err != nil {
		t.Fatal(err)
	}
	if a.About != "Notify me if Brendan pushes a new drawing." || a.WhenSaid() != "thing.added kind=sketch by=brendan" || !a.Allows(MayNotify) || a.Allows(MayConverse) {
		t.Errorf("read as %+v", a)
	}
	for src, want := range map[string]string{
		`may = ["notify"]`:                                  "it says no when",
		`when = event("pr.mergd")`:                          `"pr.mergd" isn't a kind of event`,
		`when = event("pr.merged", colour = "red")`:         `"colour" isn't a field`,
		`when = "pr.merged"`:                                "when is a string",
		"when = event(\"pr.merged\")\nmay = [\"merge\"]":    `"merge" isn't something an automation may do yet`,
		"when = event(\"pr.merged\")\ndef run(): pass":      "run takes 0 arguments",
		"when = event(\"pr.merged\")\nmay = [\"converse\"]": "has no run to",
		"when = event(\"pr.merged\")\nwhile True: pass":     "while",
	} {
		if _, err := Check("x", []byte(src)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want it to say %q", src, err, want)
		}
	}
	if _, err := Check("Bad Name", []byte(drawing)); err == nil {
		t.Error("a name with spaces")
	}
}

type told struct {
	notified  []string
	conversed []string
}

func engine(t *testing.T) (*Engine, *told) {
	t.Helper()
	root := t.TempDir()
	storetest.Own(t, root)
	got := &told{}
	e := &Engine{Root: root, Hooks: Hooks{
		Notify: func(a *Automation, text string, refs []string) error {
			got.notified = append(got.notified, a.Name+": "+text+" "+strings.Join(refs, ","))
			return nil
		},
		Converse: func(a *Automation, text string, about []string) (string, error) {
			got.conversed = append(got.conversed, text)
			return "w9", nil
		},
		Show: func(ref string) (map[string]any, error) {
			return map[string]any{"head": "Idempotency keys", "links": []any{map[string]any{"ref": "RAM-881", "what": "about"}}}, nil
		},
	}}
	return e, got
}

func emit(t *testing.T, root string, e events.Event) events.Event {
	t.Helper()
	got, err := events.Emit(root, e)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

var sketch = events.Event{Kind: events.ThingAdded, Origin: events.Heard, By: events.ByPerson("brendan-ash"), Repo: "records", Thing: "records/sketches/queues/", ThingKind: "sketch", Title: "Queues"}

// An automation runs on each event it's for after it's added, once: not
// on what came before it, and not on what it isn't for.
func TestEngine_RunsOnWhatItsFor(t *testing.T) {
	e, got := engine(t)
	emit(t, e.Root, sketch) // before it was added
	if _, err := e.Add("drawing", []byte(drawing)); err != nil {
		t.Fatal(err)
	}
	other := sketch
	other.By = events.ByPerson("ana-ortiz")
	emit(t, e.Root, other)
	emit(t, e.Root, sketch)
	said := e.RunDue()
	if len(said) != 1 || !strings.HasSuffix(said[0], ": ran") {
		t.Errorf("said %q", said)
	}
	if strings.Join(got.notified, "\n") != "drawing: Brendan drew Queues records/sketches/queues/" {
		t.Errorf("notified %q", got.notified)
	}
	if e.RunDue(); len(got.notified) != 1 {
		t.Errorf("run twice: %q", got.notified)
	}
	runs, _ := Runs(e.Root, "drawing", 10)
	if len(runs) != 1 || runs[0].State != Ran {
		t.Errorf("runs %+v", runs)
	}
}

// A run that stopped part way through is run again without doing again
// what it did; a step started and never said done stops it, unsure.
func TestEngine_StepsAreDoneOnce(t *testing.T) {
	e, got := engine(t)
	e.Add("drawing", []byte(drawing))
	ev := emit(t, e.Root, sketch)
	// As though the daemon stopped once notify had been done, before the
	// run was said to have ended.
	startRun(e.Root, "drawing", ev.N, e.now())
	keepStep(e.Root, "drawing", ev.N, 1, "notify", "done", "")
	e.RunDue()
	if len(got.notified) != 0 {
		t.Errorf("a step done was done again: %q", got.notified)
	}
	if runs, _ := Runs(e.Root, "drawing", 1); runs[0].State != Ran {
		t.Errorf("run %+v", runs[0])
	}

	ev = emit(t, e.Root, sketch)
	startRun(e.Root, "drawing", ev.N, e.now())
	keepStep(e.Root, "drawing", ev.N, 1, "notify", "started", "")
	e.RunDue()
	runs, _ := Runs(e.Root, "drawing", 1)
	if runs[0].State != Unsure || len(got.notified) != 1 || !strings.Contains(got.notified[0], "couldn't run") {
		t.Errorf("unsure: %+v, told %q", runs[0], got.notified)
	}
}

// Without a run, the event's notified as it says it; a failure is told
// you once, not at every event; and what an automation started isn't run
// on.
func TestEngine_DefaultsFailuresAndItsOwn(t *testing.T) {
	e, got := engine(t)
	e.Add("merges", []byte("when = event(\"pr.merged\")\n"))
	e.Add("broken", []byte("when = event(\"pr.merged\")\nmay = [\"notify\"]\ndef run(e):\n    notify(e.nope)\n"))
	e.Add("talk", []byte("when = event(\"conversation.answered\")\nmay = [\"converse\"]\ndef run(e):\n    converse(\"again\")\n"))
	emit(t, e.Root, events.Event{Kind: events.PRMerged, Origin: events.Heard, By: events.ByPerson("alice"), PR: "api#212", Title: "Idempotency"})
	emit(t, e.Root, events.Event{Kind: events.PRMerged, Origin: events.Heard, By: events.ByPerson("alice"), PR: "api#213", Title: "Retry"})
	emit(t, e.Root, events.Event{Kind: events.ConversationAnswered, By: events.ByWS, Conversation: "w9"})
	emit(t, e.Root, events.Event{Kind: events.ConversationAnswered, By: events.ByWS, Conversation: "w2"})
	e.RunDue()
	var merges, failed int
	for _, n := range got.notified {
		switch {
		case strings.HasPrefix(n, "merges: Idempotency api#212"), strings.HasPrefix(n, "merges: Retry api#213"):
			merges++
		case strings.Contains(n, "broken couldn't run"):
			failed++
		}
	}
	if merges != 2 || failed != 1 {
		t.Errorf("notified %q", got.notified)
	}
	// converse started w9 once it'd run on w2: only w2 is run on (w9's was
	// before, and is run on: what it starts from now on isn't).
	if len(got.conversed) != 2 {
		t.Errorf("conversed %q", got.conversed)
	}
	emit(t, e.Root, events.Event{Kind: events.ConversationAnswered, By: events.ByWS, Conversation: "w9"})
	e.RunDue()
	if len(got.conversed) != 2 {
		t.Errorf("ran on its own conversation: %q", got.conversed)
	}
}

// show and events read what ws has from a run.
func TestEngine_ReadsWhatWSHas(t *testing.T) {
	e, got := engine(t)
	e.Add("reads", []byte(`when = event("pr.merged")
may = ["notify"]
def run(e):
    pr = show(e.pr)
    merged = events("pr.merged", since = "1d")
    notify("%s, %d merged, about %s" % (pr.head, len(merged), pr.links[0].ref))
`))
	emit(t, e.Root, events.Event{Kind: events.PRMerged, Origin: events.Heard, By: events.ByPerson("alice"), PR: "api#212"})
	e.RunDue()
	if len(got.notified) != 1 || got.notified[0] != "reads: Idempotency keys, 1 merged, about RAM-881 " {
		t.Errorf("notified %q", got.notified)
	}
}
