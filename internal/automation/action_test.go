package automation

import (
	"strings"
	"testing"
)

const nudge = `# Nudge a PR of mine that's waiting on review, with a note, or post the week's merges.
when = [action("Nudge its reviewers", on = "pr", state = "open", author = ["me", "brendan*"], input = {"note": "text", "loud": "bool", "how": ["comment", "ping"]}), action("Post the week's merges")]
may = ["notify"]

def run(e):
    if e.on:
        notify("%s %s on %s: %s (loud %s)" % (e.title, e.input.how, e.on, e.input.note, e.input.loud))
    else:
        notify(e.title)
`

// An action is run by you, on a thing of its kind or on nothing: its run
// is given what it was run on, and what was asked of you. Only its own
// automation's action runs.
func TestAction_Runs(t *testing.T) {
	e, got := engine(t)
	if _, err := e.Add("nudge", []byte(nudge)); err != nil {
		t.Fatal(err)
	}
	other := strings.Replace(nudge, "Nudge a PR", "Another", 1)
	if _, err := e.Add("other", []byte(other)); err != nil {
		t.Fatal(err)
	}
	a, _ := Load(e.Root, "nudge")
	x, ok := a.ActionNamed("Nudge its reviewers")
	if !ok {
		t.Fatalf("actions %+v", a.Actions())
	}
	in, err := x.ReadInput(map[string]any{"note": "a week now", "how": "Ping", "loud": "true"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RunAction(e.Root, "nudge", x.Title, "api#212", in); err != nil {
		t.Fatal(err)
	}
	if _, err := RunAction(e.Root, "nudge", "Post the week's merges", "", nil); err != nil {
		t.Fatal(err)
	}
	e.RunDue()
	if strings.Join(got.notified, "|") != "nudge: Nudge its reviewers ping on api#212: a week now (loud True) |nudge: Post the week's merges " {
		t.Errorf("notified %q", got.notified)
	}
	if w := a.WhenSaid(); w != `you run "Nudge its reviewers" on a pr author=me,brendan* state=open · you run "Post the week's merges"` {
		t.Errorf("when said %q", w)
	}
}

// Which things an action's offered on: of its kind, its fields as asked
// (a value, a pattern, any of a list); one with no kind on nothing.
func TestAction_Applies(t *testing.T) {
	a, err := Check("nudge", []byte(nudge))
	if err != nil {
		t.Fatal(err)
	}
	on, none := a.Actions()[0], a.Actions()[1]
	for _, c := range []struct {
		kind   string
		fields map[string]string
		on     bool
		none   bool
		why    string
	}{
		{"pr", map[string]string{"state": "open", "author": "me"}, true, false, ""},
		{"pr", map[string]string{"State": "open", "Author": "brendan-ash"}, true, false, ""},
		{"pull", map[string]string{"state": "open", "author": "me"}, true, false, ""},
		{"pr", map[string]string{"state": "merged", "author": "me"}, false, false, `only for those whose author is "me", not me or brendan*, state is "merged", not open`},
		{"pr", map[string]string{"author": "me"}, false, false, `state is ""`},
		{"mission", map[string]string{"state": "open"}, false, false, "runs on a pr, not a mission"},
		{"", nil, false, true, "runs on a pr"},
	} {
		if got := on.Applies(c.kind, c.fields); got != c.on {
			t.Errorf("%s %v: applies %v", c.kind, c.fields, got)
		}
		if got := none.Applies(c.kind, c.fields); got != c.none {
			t.Errorf("%s %v: one on nothing applies %v", c.kind, c.fields, got)
		}
		if why := on.WhyNot(c.kind, c.fields); !strings.Contains(why, c.why) || (c.why == "") != (why == "") {
			t.Errorf("%s %v: why not %q, want %q", c.kind, c.fields, why, c.why)
		}
	}
}

func TestAction_Input(t *testing.T) {
	a, _ := Check("nudge", []byte(nudge))
	x := a.Actions()[0]
	for in, want := range map[string]map[string]any{
		"missing note":         {"how": "ping"},
		"one of comment, ping": {"note": "x", "how": "shout"},
		"does not take":          {"note": "x", "how": "ping", "size": "L"},
		"yes or no":            {"note": "x", "how": "ping", "loud": "maybe"},
	} {
		_, err := x.ReadInput(want)
		if err == nil || !strings.Contains(err.Error(), strings.TrimPrefix(in, "missing ")) {
			t.Errorf("%s: %v", in, err)
		}
	}
	if got, err := x.ReadInput(map[string]any{"note": "x", "how": "comment"}); err != nil || got["loud"] != false {
		t.Errorf("a bool left out is no: %v %v", got, err)
	}
	if got, err := (InputField{Name: "n", Type: "int"}).read("3"); err != nil || got != 3.0 {
		t.Errorf("int from words: %v %v", got, err)
	}
}

func TestAction_Refused(t *testing.T) {
	for script, want := range map[string]string{
		`when = action("A", status = "open")`:                    "needs a kind",
		`when = [action("A"), action("A", on = "pr")]`:           `two actions called "A"`,
		`when = action("A", on = "pr", input = {"x": "date"})`:   `x is "date"`,
		`when = action(on = "pr")`:                               "needs a title",
		`when = action("A", on = "pr", input = {"x": ["only"]})`: "two or more choices",
		`when = action("A", on = "pr", status = 3)`:              "status is a value, or a list",
	} {
		if _, err := Check("refused", []byte(script)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", script, err, want)
		}
	}
}
