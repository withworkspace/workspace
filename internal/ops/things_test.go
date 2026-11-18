package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
)

// installed puts a plugin in the workspace as one that's there (its
// plugin.toml), and keeps how its things are named, as the daemon does
// when it starts one.
func installed(t *testing.T, root, name string, refs ...protocol.Ref) {
	t.Helper()
	app := filepath.Join(plugin.Dir(root), name, "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "name = \"" + name + "\"\nversion = \"0.1.0\"\ncommand = [\"./ws-" + name + "\"]\n"
	if err := os.WriteFile(filepath.Join(app, "plugin.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	registered(t, root, name, plugin.Registration{Refs: refs})
	if refused, err := plugin.KeepRefs(root, name, refs); err != nil || len(refused) != 0 {
		t.Fatalf("keeping %s's refs: %v, %v", name, refused, err)
	}
}

var jiraIssue = protocol.Ref{Kind: "issue", Title: "Jira issue", Prefixes: []string{"RAM-", "RAP-"}, Address: "https://acme.atlassian.net/browse/{ref}", Path: "/issue/{ref}"}

// A plugin's things are named from what ws keeps of how it names them:
// in what there is to name, by a name, by ws: and by where one is.
func TestAPluginsThingsAreNamed(t *testing.T) {
	w, _ := fixture(t, "")
	root := w.WS().Root
	if _, _, ok := w.ThingNamed("RAM-12"); ok {
		t.Error("RAM-12 names something with no plugins")
	}
	installed(t, root, "jira", jiraIssue)

	k := w.RefsKnown(t.Context(), false)
	if len(k.Things) != 1 || k.Things[0].Plugin != "jira" || k.Things[0].What != "issue" {
		t.Fatalf("things to name: %+v", k.Things)
	}
	want := ref.Ref{Kind: ref.Thing, Plugin: "jira", What: "issue", ID: "RAM-12", URL: "https://acme.atlassian.net/browse/RAM-12"}
	for _, said := range []string{"RAM-12", " RAM-12 ", "ws:RAM-12", "https://acme.atlassian.net/browse/RAM-12", "https://acme.atlassian.net/browse/RAM-12?focusedCommentId=4"} {
		got, kept, ok := w.ThingNamed(said)
		if !ok || got != want || kept.Plugin != "jira" || kept.Title != "Jira issue" {
			t.Errorf("ThingNamed(%q) = %+v, %+v, %v", said, got, kept, ok)
		}
	}
	for _, said := range []string{"", "cache", "RAM-12 and more", "UTF-8", "ws:cache", "https://other.example/browse/RAM-12"} {
		if got, _, ok := w.ThingNamed(said); ok {
			t.Errorf("ThingNamed(%q) = %+v, want nothing", said, got)
		}
	}

	// In a document, a name is linked to where its plugin said one is.
	w.WS().Org = "acme"
	if got, want := w.LinkNames("Fixes RAM-12 for cache.\n"), "Fixes [RAM-12](https://acme.atlassian.net/browse/RAM-12) for [cache](ws:cache).\n"; got != want {
		t.Errorf("LinkNames:\n got  %q\n want %q", got, want)
	}

	// Agents are told how its things are named, whatever their role.
	if err := os.WriteFile(filepath.Join(root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	note := PluginsNote(root, RoleOrchestrator)
	for _, want := range []string{"The jira plugin:", "Jira issue (`RAM-123`, `RAP-123`)", "`ws show RAM-123`"} {
		if !strings.Contains(note, want) {
			t.Errorf("the note lacks %q:\n%s", want, note)
		}
	}

	// Forgotten with its plugin, its names are words again.
	if err := plugin.Remove(root, "jira", false); err != nil {
		t.Fatal(err)
	}
	if err := plugin.Forget(root, "jira"); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := w.ThingNamed("RAM-12"); ok {
		t.Error("RAM-12 names something once jira's removed")
	}
	if k := w.RefsKnown(t.Context(), false); len(k.Things) != 0 {
		t.Errorf("things to name once jira's removed: %+v", k.Things)
	}
}

// A plugin that's gone without being removed through ws names nothing,
// though what ws kept of it is still there.
func TestAPluginThatsGoneNamesNothing(t *testing.T) {
	w, _ := fixture(t, "")
	root := w.WS().Root
	installed(t, root, "jira", jiraIssue)
	installed(t, root, "linear", protocol.Ref{Kind: "issue", Prefixes: []string{"ENG-"}})
	if err := os.RemoveAll(filepath.Join(plugin.Dir(root), "jira")); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := w.ThingNamed("RAM-12"); ok {
		t.Error("RAM-12 names something with jira's folder gone")
	}
	if got, _, ok := w.ThingNamed("ENG-3"); !ok || got.Plugin != "linear" || got.URL != "" {
		t.Errorf("ENG-3 = %+v, %v", got, ok)
	}
}

// What's said in a thread keeps the plugin's thing it named, and the
// graph has it: what's been said of RAM-12 is found from its name.
func TestATurnThatNamesAPluginsThingIsSaidOfIt(t *testing.T) {
	w, _ := fixture(t, "")
	groundRepo(t, w)
	installed(t, w.WS().Root, "jira", jiraIssue)

	sc := doc.OfMission("cache")
	text := []byte("# Plan\n\nCache per user.\n")
	c, err := w.OpenThread(sc, "frontend:plan.md", text, conversation.DocThread{Version: doc.Version(text), Line: 3, Quote: conversation.Quote{Exact: "per user"}, Kind: conversation.Ask, Body: "Why?"}, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	c, err = w.Reply(sc, c.ID, text, "It's what RAM-12 asks for.", "", AsCrew("c1"))
	if err != nil {
		t.Fatal(err)
	}
	refs := c.Turns[len(c.Turns)-1].Refs
	want := ref.Pinned{Said: "RAM-12", Ref: ref.Ref{Kind: ref.Thing, Plugin: "jira", What: "issue", ID: "RAM-12", URL: "https://acme.atlassian.net/browse/RAM-12"}}
	if len(refs) != 1 || refs[0] != want {
		t.Fatalf("the turn's refs = %+v, want %+v", refs, want)
	}
	said, err := w.SaidOf(store.Key{Name: "RAM-12"})
	if err != nil || len(said) != 1 || said[0].How != store.Mentions || said[0].Scope != "cache" || said[0].Ref != c.ID {
		t.Errorf("said of RAM-12: %+v, %v", said, err)
	}
}

// A thing a plugin's told ws of is found by what it says, as the plugin's:
// what one's called, whose it is, where it opens, and how it's shown.
func TestAPluginsThingsAreFound(t *testing.T) {
	w, _ := fixture(t, "")
	root := w.WS().Root
	installed(t, root, "jira", jiraIssue)
	// Told as the plugin tells it: through what ws answers a plugin.
	params, _ := json.Marshal(protocol.ThingPutParams{Things: []protocol.Thing{
		{Kind: "issue", Ref: "RAM-12", Title: "Token refresh is flaky", State: "In Progress", Text: "The clock should be injected."},
		{Kind: "issue", Ref: "ram-13", Title: "Not one of its names"},
	}})
	answer, err := w.AnswerPlugin(t.Context(), "jira", protocol.MethodThingPut, params)
	if put, ok := answer.(protocol.ThingPutResult); err != nil || !ok || len(put.Refused) != 1 || put.Refused[0].Ref != "ram-13" {
		t.Fatalf("thing/put: %+v, %v", answer, err)
	}
	if answer, err := w.AnswerPlugin(t.Context(), "jira", protocol.MethodThingList, json.RawMessage(`{"kind":"issue"}`)); err != nil || len(answer.(protocol.ThingListResult).Refs) != 1 {
		t.Errorf("thing/list: %+v, %v", answer, err)
	}

	for _, typed := range []string{"flaky token", "RAM-12", "clock injected"} {
		found, _, err := w.Find(t.Context(), FindQuery{Words: typed})
		if err != nil || len(found) != 1 {
			t.Fatalf("finding %q: %+v, %v", typed, found, err)
		}
		f := found[0]
		if f.Ref != "RAM-12" || f.Scope != "" || f.Show != "ws show RAM-12" || f.Title() != "Token refresh is flaky" || f.Thing.State != "In Progress" {
			t.Errorf("finding %q: %+v", typed, f)
		}
		if f.Named == nil || f.Named.Plugin != "jira" || f.Named.What != "issue" || f.Named.URL != "https://acme.atlassian.net/browse/RAM-12" || f.Called != "Jira issue" || f.Opens != "/x/jira/issue/RAM-12" {
			t.Errorf("finding %q: named %+v, called %q, opens %q", typed, f.Named, f.Called, f.Opens)
		}
		if f.What() != "Jira issue" || f.Where() != "the jira plugin" {
			t.Errorf("finding %q: %q, %q", typed, f.What(), f.Where())
		}
	}
	// Kept to a mission, it isn't that mission's.
	if found, _, _ := w.Find(t.Context(), FindQuery{Words: "flaky token", Mission: "cache"}); len(found) != 0 {
		t.Errorf("found in a mission: %+v", found)
	}

	// Removed with its plugin, it's found no more.
	if _, err := w.AnswerPlugin(t.Context(), "jira", protocol.MethodThingRemove, json.RawMessage(`{"kind":"issue","refs":["RAM-12"]}`)); err != nil {
		t.Fatal(err)
	}
	if found, _, _ := w.Find(t.Context(), FindQuery{Words: "flaky token"}); len(found) != 0 {
		t.Errorf("found once it's removed: %+v", found)
	}
}
