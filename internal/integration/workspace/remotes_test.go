package workspace_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/testutil"
)

// sketchKind is a kind whose things can live in a repo: a sketch.
var sketchKind = protocol.Ref{
	Kind: "sketch", Title: "Sketch", Prefixes: []string{"sketch-"}, Path: "/s/{ref}",
	Fields: []protocol.Field{{Name: "title", Type: protocol.FieldTitle}, {Name: "text", Type: protocol.FieldMarkdown, Label: "On it"}},
	Remote: &protocol.Remote{Path: "sketches/{slug}/", Main: "{slug}.excalidraw", Reads: true},
}

// reader stands in for the plugins that read a thing's main file: it
// says what's on a sketch is what's in its file, and keeps what it was
// asked.
type reader struct {
	asked []string
	down  bool // nobody's there to ask
}

func (r *reader) ReadRemote(_ context.Context, name string, in protocol.RemoteReadParams) (map[string]string, error) {
	r.asked = append(r.asked, name+" "+in.Kind+" "+in.Name+" "+in.File+" "+in.Title)
	switch {
	case r.down:
		return nil, errors.New("the plugin's stopped")
	case strings.Contains(string(in.Content), "broken"):
		return nil, plugin.ErrCantRead
	}
	return map[string]string{"text": string(in.Content), "title": "not its manifest's"}, nil
}
func (r *reader) Close() {}

// sketching is a workspace whose records repo is a space, with the sketch
// plugin installed, its kind's things able to live in a repo, and the
// plugin stood in for.
func sketching(t *testing.T) (*testutil.Workspace, *reader) {
	t.Helper()
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{Org: "test-org", DefaultBranch: "main", Repos: []testutil.RepoOpts{{Name: "records", Landing: "direct"}}})
	f, err := os.OpenFile(filepath.Join(w.Root, "ws.toml"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n[missions]\nrecords = \"records\"\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	registerPlugin(t, w.Root, "sketch", plugin.Registration{Commands: &plugin.Commands{}, Refs: []protocol.Ref{sketchKind}})
	if refused, err := plugin.KeepRefs(w.Root, "sketch", []protocol.Ref{sketchKind}); err != nil || len(refused) != 0 {
		t.Fatalf("keeping sketch's refs: %v, %v", refused, err)
	}
	r := &reader{}
	was := ops.RemoteReaders
	ops.RemoteReaders = func(string) (ops.RemoteReader, error) { return r, nil }
	t.Cleanup(func() { ops.RemoteReaders = was })
	return w, r
}

// pack commits a sketch's package to records' default branch.
func pack(t *testing.T, w *testutil.Workspace, slug, title, scene string) {
	t.Helper()
	ground := testutil.CapsuleDir(w, "records", ".ground")
	folder := filepath.Join(ground, "sketches", slug)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "kind = \"sketch\"\ntitle = \"" + title + "\"\nmain = \"" + slug + ".excalidraw\"\npicture = \"" + slug + ".svg\"\n"
	if err := os.WriteFile(filepath.Join(folder, "thing.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.CommitIn(t, ground, "sketches/"+slug+"/"+slug+".excalidraw", scene, "A sketch: "+title)
}

// A sketch committed to a space is one of the sketch plugin's things:
// found by ws, read by the plugin once for each version of its file,
// named by its folder, and found by what the plugin said is on it.
func TestAThingInASpaceIsFoundAndRead(t *testing.T) {
	w, r := sketching(t)
	pack(t, w, "release-flow", "Release flow", "brief it, plan, build under a lantern")
	const name = "records/sketches/release-flow/"

	find := func(words string) string {
		t.Helper()
		res := testutil.RunCommand(t, w.Root, nil, "find", words)
		if res.Err != nil {
			t.Fatalf("find %s: %v\n%s", words, res.Err, res.Stderr)
		}
		return res.Stdout
	}
	// Found by what's on it, and by what its manifest calls it.
	if out := find("lantern"); !strings.Contains(out, name) || !strings.Contains(out, "Release flow") {
		t.Fatalf("found by what's on it:\n%s", out)
	}
	if out := find("release flow"); !strings.Contains(out, name) {
		t.Fatalf("found by its title:\n%s", out)
	}
	if want := []string{"sketch sketch " + name + " release-flow.excalidraw Release flow"}; !slices.Equal(r.asked, want) {
		t.Fatalf("the plugin was asked %q, want %q", r.asked, want)
	}
	// As ws keeps it: under the plugin, called what its manifest says.
	rec, ok, err := plugin.ThingOf(w.Root, "sketch", "sketch", name)
	if err != nil || !ok || rec.Found == "" || rec.Fields["title"] != "Release flow" || rec.Fields["text"] != "brief it, plan, build under a lantern" {
		t.Fatalf("as kept: %+v, %v, %v", rec, ok, err)
	}
	// It's ws's, not the plugin's: the plugin doesn't list it, and can't
	// have it forgotten.
	if names, _ := plugin.ListThings(w.Root, "sketch", "sketch"); len(names) != 0 {
		t.Errorf("listed to the plugin among what it told ws: %q", names)
	}
	if err := plugin.RemoveThings(w.Root, "sketch", "sketch", []string{name}); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := plugin.ThingOf(w.Root, "sketch", "sketch", name); !ok {
		t.Error("the plugin had a thing ws found forgotten")
	}

	// ws show, by its folder as a document's ref is written, or as ws
	// names it: what's kept of it, with nobody asked.
	for _, said := range []string{"records:sketches/release-flow/", name} {
		res := testutil.RunCommand(t, w.Root, nil, "show", said, "-f", "llm")
		if res.Err != nil {
			t.Fatalf("show %s: %v\n%s", said, res.Err, res.Stderr)
		}
		for _, want := range []string{name + ": Release flow\n", "Sketch, from the sketch plugin.", "As it is in its repo, on the default branch", "brief it, plan, build under a lantern"} {
			if !strings.Contains(res.Stdout, want) {
				t.Errorf("show %s lacks %q:\n%s", said, want, res.Stdout)
			}
		}
	}

	// A mission can be about it, by its folder, and is found from it.
	if res := testutil.RunCommand(t, w.Root, nil, "mission", "new", "Tidy the release flow", "--about", "records:sketches/release-flow/"); res.Err != nil {
		t.Fatalf("a mission about it: %v\n%s", res.Err, res.Stderr)
	}
	if res := testutil.RunCommand(t, w.Root, nil, "show", name, "-f", "llm"); res.Err != nil || !strings.Contains(res.Stdout, "tidy-the-release-flow") {
		t.Errorf("shown with the mission about it: %v\n%s", res.Err, res.Stdout)
	}

	// Looked for again with nothing changed: nobody's asked.
	find("lantern")
	if len(r.asked) != 1 {
		t.Fatalf("asked again with nothing changed: %q", r.asked)
	}

	// Its file changes: read again, and found by what's on it now.
	pack(t, w, "release-flow", "Release flow", "ship it by candle")
	if out := find("candle"); !strings.Contains(out, name) {
		t.Fatalf("found by what's on it now:\n%s", out)
	}
	if out := find("lantern"); strings.Contains(out, name) {
		t.Fatalf("still found by what was on it:\n%s", out)
	}
	if len(r.asked) != 2 {
		t.Fatalf("asked %d times after one change: %q", len(r.asked), r.asked)
	}

	// One the plugin can't read is kept by its title; one of a kind no
	// plugin has isn't kept, and neither is asked about twice.
	pack(t, w, "torn", "Torn page", "broken")
	ground := testutil.CapsuleDir(w, "records", ".ground")
	testutil.CommitIn(t, ground, "notes/a/thing.toml", "kind = \"note\"\ntitle = \"A note\"\nmain = \"a.txt\"\n", "A note's manifest")
	testutil.CommitIn(t, ground, "notes/a/a.txt", "candle", "A note")
	if out := find("torn page"); !strings.Contains(out, "records/sketches/torn/") {
		t.Fatalf("one the plugin can't read, by its title:\n%s", out)
	}
	if out := find("candle"); strings.Contains(out, "records/notes/a/") {
		t.Fatalf("a thing of a kind no plugin has was kept:\n%s", out)
	}
	asked := len(r.asked)
	find("candle")
	if len(r.asked) != asked {
		t.Fatalf("asked again of one it couldn't read: %q", r.asked[asked:])
	}

	// A plugin that isn't there to ask: what's new waits for it.
	r.down = true
	pack(t, w, "later", "Later", "by torchlight")
	if out := find("torchlight"); strings.Contains(out, "records/sketches/later/") {
		t.Fatalf("kept with nobody to read it:\n%s", out)
	}
	r.down = false
	if out := find("torchlight"); !strings.Contains(out, "records/sketches/later/") {
		t.Fatalf("read once the plugin's back:\n%s", out)
	}

	// Gone from the repo, gone from ws.
	testutil.GitCmd(t, ground, "rm", "-rq", "sketches/release-flow")
	testutil.GitCmd(t, ground, "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "-qm", "Drop the release flow")
	if out := find("candle"); strings.Contains(out, name) {
		t.Fatalf("found after it went:\n%s", out)
	}
	if _, ok, _ := plugin.ThingOf(w.Root, "sketch", "sketch", name); ok {
		t.Error("kept after it went")
	}

	// The plugin registers its kind afresh: what's there is read again,
	// as the kind now is. With no remote, nothing of the kind is kept.
	asked = len(r.asked)
	again := sketchKind
	again.Title = "Drawing"
	if _, err := plugin.KeepRefs(w.Root, "sketch", []protocol.Ref{again}); err != nil {
		t.Fatal(err)
	}
	find("torchlight")
	if got := len(r.asked) - asked; got != 2 {
		t.Fatalf("asked of %d after its kind was registered afresh, want the 2 there are: %q", got, r.asked[asked:])
	}
	local := again
	local.Remote = nil
	if _, err := plugin.KeepRefs(w.Root, "sketch", []protocol.Ref{local}); err != nil {
		t.Fatal(err)
	}
	if out := find("torchlight"); strings.Contains(out, "records/sketches/later/") {
		t.Fatalf("kept for a kind whose things don't live in a repo:\n%s", out)
	}
	if found, _ := plugin.Findings(w.Root); len(found) != 0 {
		t.Fatalf("kept for a kind whose things don't live in a repo: %+v", found)
	}
}

// The plugin asks ws which of its things are in the repos, and for one's
// file: ws has them, so the plugin needn't.
func TestAPluginAsksWSForItsThingsInASpace(t *testing.T) {
	w, _ := sketching(t)
	pack(t, w, "release-flow", "Release flow", "brief it, plan, build")
	pack(t, w, "arch", "Architecture", "boxes")
	ctx := context.Background()
	answer := cli.AnswerPlugin(w.Root)
	ask := func(who, method string, params string, into any) error {
		t.Helper()
		out, err := answer(ctx, who, method, []byte(params))
		if err != nil {
			return err
		}
		data, err := json.Marshal(out) // as it goes to the plugin
		if err != nil {
			t.Fatal(err)
		}
		return json.Unmarshal(data, into)
	}
	var list protocol.RemoteListResult
	if err := ask("sketch", protocol.MethodRemoteList, `{"kind":"sketch"}`, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Things) != 2 || list.Things[0].Name != "records/sketches/arch/" || list.Things[1].Title != "Release flow" {
		t.Fatalf("listed: %+v", list.Things)
	}
	flow := list.Things[1]
	if flow.Repo != "records" || flow.Path != "sketches/release-flow/" || flow.Main != "release-flow.excalidraw" || flow.Picture != "release-flow.svg" || flow.Version == "" || flow.Changed.IsZero() || flow.ChangedBy != "Test" {
		t.Errorf("the release flow, as listed: %+v", flow)
	}
	var opened protocol.RemoteOpened
	if err := ask("sketch", protocol.MethodRemoteOpen, `{"kind":"sketch","name":"records/sketches/release-flow/"}`, &opened); err != nil {
		t.Fatal(err)
	}
	if opened.RemoteThing != flow || string(opened.Content) != "brief it, plan, build" {
		t.Errorf("opened: %+v %q", opened.RemoteThing, opened.Content)
	}
	// Nothing by that name, a kind that isn't its own, and another
	// plugin's: each is said.
	for params, want := range map[string]string{
		`{"kind":"sketch","name":"records/sketches/gone/"}`:   "ws has no sketch at",
		`{"kind":"sketch","name":"records/sketches/arch"}`:    "ws has no sketch at",
		`{"kind":"issue","name":"records/sketches/arch/"}`:    "has no kind",
		`{"kind":"sketch","name":"../records/sketches/arch"}`: "ws has no sketch at",
	} {
		if err := ask("sketch", protocol.MethodRemoteOpen, params, &opened); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("open %s: %v, want it to say %q", params, err, want)
		}
	}
	if err := ask("jira", protocol.MethodRemoteList, `{"kind":"sketch"}`, &list); err == nil || !strings.Contains(err.Error(), "has no kind") {
		t.Errorf("another plugin's kind: %v", err)
	}
}
