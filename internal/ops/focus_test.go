package ops

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/plugin"
)

func focusNames(t *testing.T, w *Workspace) []string {
	t.Helper()
	items, err := w.Focus()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, it := range items {
		s := it.Name
		if it.Title != "" {
			s += " " + it.Kind + " " + it.Title
		}
		out = append(out, s)
	}
	return out
}

// You pin anything ws names, as ws writes its name: a mission, a pull
// request, a plugin's thing, a file. A word isn't anything, and a pin
// is yours: crew don't pin.
func TestFocus_YouPinWhatWSNames(t *testing.T) {
	w, _ := fixture(t, "")
	root := w.WS().Root
	installed(t, root, "jira", jiraIssue)
	params, _ := json.Marshal(protocol.ThingPutParams{Things: []protocol.Thing{{Kind: "issue", Ref: "RAM-12", Fields: map[string]string{"title": "Token refresh is flaky", "status": "In Progress"}}}})
	if _, err := w.AnswerPlugin(t.Context(), "jira", protocol.MethodThingPut, params); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"cache", "ws:RAM-12", "frontend#12", "frontend/src/app.ts:12-20"} {
		if got, err := w.Pin(AsYou, name); err != nil || got.Already {
			t.Fatalf("pin %s: %+v, %v", name, got, err)
		}
	}
	if got, err := w.Pin(AsYou, "RAM-12"); err != nil || !got.Already || got.String() != "RAM-12 is already pinned." {
		t.Errorf("pinning RAM-12 again: %+v, %v", got, err)
	}
	want := []string{"cache mission Cache", "RAM-12 thing Token refresh is flaky", "frontend#12", "frontend/src/app.ts"}
	if got := focusNames(t, w); !slices.Equal(got, want) {
		t.Fatalf("focus:\n got  %q\n want %q", got, want)
	}

	for _, name := range []string{"no-such-mission", "two words", "RAM-99"} {
		if _, err := w.Pin(AsYou, name); err == nil {
			t.Errorf("pinned %q", name)
		}
	}
	_, err := w.Pin(AsCrew("c1"), "cache")
	refusedWith(t, err, "not-yours")
	refusedWith(t, w.ArrangeFocus(AsOrchestrator, []string{"cache"}), "not-yours")

	if err := w.ArrangeFocus(AsYou, []string{"frontend#12", "RAM-12"}); err != nil {
		t.Fatal(err)
	}
	if got, err := w.Unpin(AsYou, "frontend/src/app.ts:3"); err != nil || !got.Was || got.String() != "Unpinned frontend/src/app.ts." {
		t.Errorf("unpin: %+v, %v", got, err)
	}
	want = []string{"frontend#12", "RAM-12 thing Token refresh is flaky", "cache mission Cache"}
	if got := focusNames(t, w); !slices.Equal(got, want) {
		t.Fatalf("arranged, one unpinned:\n got  %q\n want %q", got, want)
	}

	// The plugin's removed: what it had is no one's focus.
	if err := plugin.Forget(root, "jira"); err != nil {
		t.Fatal(err)
	}
	if got := focusNames(t, w); !slices.Equal(got, []string{"frontend#12", "cache mission Cache"}) {
		t.Errorf("once the plugin's removed: %q", got)
	}
}

// A mission that's ended leaves your focus by itself, and comes back with
// it, as its pin's kept.
func TestFocus_WhatsEndedLeavesIt(t *testing.T) {
	w, _ := fixture(t, "")
	if _, err := w.Pin(AsYou, "cache"); err != nil {
		t.Fatal(err)
	}
	set := func(s mission.State) {
		if _, err := w.Missions.Update("cache", func(m *mission.Mission) error { m.State = s; return nil }); err != nil {
			t.Fatal(err)
		}
	}
	set(mission.Archived)
	if got := focusNames(t, w); len(got) != 0 {
		t.Errorf("archived, focus = %q", got)
	}
	set(mission.Active)
	if got := focusNames(t, w); !slices.Equal(got, []string{"cache mission Cache"}) {
		t.Errorf("brought back, focus = %q", got)
	}
}
