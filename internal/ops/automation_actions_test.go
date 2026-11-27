package ops

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/automation"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

const missionActions = `# Things I do to missions, and a digest.
when = [action("Nudge its crew", on = "mission", state = ["active", "review"], input = {"note": "text"}), action("Post the digest")]
may = ["notify"]

def run(e):
    notify("%s %s %s" % (e.title, e.on, e.input.note if e.on else ""))
`

// An automation's actions are offered on what they're for, as it stands:
// one on nothing under Do, one on a mission only on a mission in a state
// it names. Run, the daemon runs it at once, and it's said how it went;
// one that isn't for that thing, or isn't given what it asks, is refused.
func TestAutomationActions(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	w := New(&config.Config{}, &workspace.Workspace{Root: root, RepoNames: []string{"api"}}, nil, Options{})
	m, err := w.Missions.Create("Cache keys")
	if err != nil {
		t.Fatal(err)
	}
	var told []string
	e := &automation.Engine{Root: root, Hooks: automation.Hooks{Notify: func(_ *automation.Automation, text string, _ []string) error {
		told = append(told, text)
		return nil
	}}}
	if _, err := e.Add("crew", []byte(missionActions)); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
				e.RunDue()
			}
		}
	}()

	global, _ := w.AutomationActions("")
	if len(global) != 1 || global[0].Title != "Post the digest" || global[0].Automation != "crew" {
		t.Errorf("on nothing: %+v", global)
	}
	if on, _ := w.AutomationActions("api"); len(on) != 0 {
		t.Errorf("on a repo: %+v", on)
	}
	parked, _ := w.Missions.Create("Parked one")
	if _, err := w.Missions.Update(parked.Slug, func(m *mission.Mission) error { m.State = mission.Parked; return nil }); err != nil {
		t.Fatal(err)
	}
	if on, _ := w.AutomationActions(parked.Slug); len(on) != 0 {
		t.Errorf("on a parked mission: %+v", on)
	}
	on, _ := w.AutomationActions(m.Slug)
	if len(on) != 1 || on[0].Title != "Nudge its crew" || len(on[0].Input) != 1 {
		t.Fatalf("on an active mission: %+v", on)
	}

	got, err := w.RunAutomationAction("crew", "Nudge its crew", m.Slug, map[string]any{"note": "look at the flake"})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != automation.Ran || len(got.Did) != 1 || got.Did[0].Text != "Nudge its crew "+m.Slug+" look at the flake" {
		t.Errorf("ran %+v", got)
	}
	if got, err := w.RunAutomationAction("crew", "Post the digest", "", nil); err != nil || got.State != automation.Ran {
		t.Errorf("the digest: %+v %v", got, err)
	}
	for _, c := range []struct {
		title, on string
		input     map[string]any
		want      string
	}{
		{"Nudge its crew", "", nil, "it's run on a mission"},
		{"Nudge its crew", m.Slug, nil, "asks for note"},
		{"Post the digest", m.Slug, nil, "on nothing in particular"},
		{"Nudge its crew", "nope-nope", map[string]any{"note": "x"}, "can't say what nope-nope is"},
		{"Fly", "", nil, `no action called "Fly"`},
	} {
		_, err := w.RunAutomationAction("crew", c.title, c.on, c.input)
		var invalid *Invalid
		if !errors.As(err, &invalid) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s on %q: %v, want %q", c.title, c.on, err, c.want)
		}
	}
	if len(told) != 2 {
		t.Errorf("told %q", told)
	}
	automation.SetOn(root, "crew", false)
	if list, _ := w.AutomationActions(""); len(list) != 0 {
		t.Errorf("off, it's offered: %+v", list)
	}
}
