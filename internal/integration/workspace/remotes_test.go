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
	Remote: &protocol.Remote{Path: "sketches/{slug}/", Main: "{slug}.excalidraw", Reads: true, Exports: true},
}

// reader stands in for the plugins that read a thing's main file: it
// says what's on a sketch is what's in its file, and keeps what it was
// asked.
type reader struct {
	asked []string
	down  bool // nobody's there to ask
	// The plugin's own sketches, by its names for them, and what ws
	// asked for and told it of them.
	own      map[string]*ownSketch
	exported []string
}

// ownSketch is one of the stand-in plugin's own sketches: what it's
// called, what's on it, and where it lives in a repo, if it does, with
// the version of it there that the plugin last had.
type ownSketch struct{ title, scene, remote, base string }

func (r *reader) ExportRemote(_ context.Context, _ string, in protocol.RemoteExportParams) (protocol.RemoteExported, error) {
	own, ok := r.own[in.Ref]
	if !ok {
		return protocol.RemoteExported{}, plugin.ErrNoThing
	}
	r.exported = append(r.exported, in.Ref)
	if in.Current != nil { // changed there since: what's there is drawn in beside its own
		r.exported[len(r.exported)-1] += " with " + string(in.Current.Content)
		own.scene, own.base = own.scene+" + "+string(in.Current.Content), in.Current.Version
	}
	return protocol.RemoteExported{Name: own.remote, Base: own.base,
		Title: own.title, Main: []byte(own.scene),
		Files: []protocol.RemoteFile{{Name: "{slug}.svg", Content: []byte("<svg>" + own.scene + "</svg>"), Picture: true}}}, nil
}

func (r *reader) SavedRemote(_ context.Context, _ string, in protocol.RemoteSavedParams) error {
	r.own[in.Ref].remote, r.own[in.Ref].base = in.Thing.Name, in.Thing.Version
	return nil
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
	r := &reader{own: map[string]*ownSketch{}}
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

// ws puts one of a plugin's own things in a repo, and saves it there, for
// you: it asks the plugin for the thing, names its folder, commits and
// lands it, and tells the plugin where it is. Where it's changed there
// since the plugin last had it, the plugin's given what's there.
func TestAThingIsPromotedAndSaved(t *testing.T) {
	w, r := sketching(t)
	r.own["sketch-12"] = &ownSketch{title: "Release  flow", scene: "brief it, plan, build under a lantern"}
	r.own["sketch-13"] = &ownSketch{title: "Architecture", scene: "layers"}
	r.own["sketch-14"] = &ownSketch{title: "Release flow", scene: "another by the same name"}
	ctx := context.Background()
	answer := cli.AnswerPlugin(w.Root)
	ask := func(method, params string) (protocol.RemoteSaved, error) {
		t.Helper()
		out, err := answer(ctx, "sketch", method, []byte(params))
		if err != nil {
			return protocol.RemoteSaved{}, err
		}
		return out.(protocol.RemoteSaved), nil
	}
	onMain := func(rel string) string {
		t.Helper()
		return testutil.GitOut(t, w.Sources["records"], "show", "main:"+rel)
	}
	subjects := func() string {
		t.Helper()
		return testutil.GitOut(t, w.Sources["records"], "log", "--format=%s", "-3", "main")
	}

	// Promoted: a folder named for its title, in records, in one commit
	// landed on main, with ws's thing.toml in it.
	done, err := ask(protocol.MethodRemotePromote, `{"kind":"sketch","ref":"sketch-12"}`)
	if err != nil {
		t.Fatal(err)
	}
	const name = "records/sketches/release-flow/"
	flow := done.Thing
	if flow.Name != name || flow.Title != "Release flow" || flow.Main != "release-flow.excalidraw" || flow.Picture != "release-flow.svg" || !done.Changed || done.Capsule != "" || !strings.Contains(done.Said, "landed on main") {
		t.Fatalf("promoted: %+v", done)
	}
	if got := onMain("sketches/release-flow/release-flow.excalidraw"); got != "brief it, plan, build under a lantern" {
		t.Errorf("its main file on main: %q", got)
	}
	if got := onMain("sketches/release-flow/release-flow.svg"); got != "<svg>brief it, plan, build under a lantern</svg>" {
		t.Errorf("its picture on main: %q", got)
	}
	if got := onMain("sketches/release-flow/thing.toml"); !strings.Contains(got, `kind = "sketch"`) || !strings.Contains(got, `title = "Release flow"`) || !strings.Contains(got, `main = "release-flow.excalidraw"`) || !strings.Contains(got, `picture = "release-flow.svg"`) {
		t.Errorf("its thing.toml on main:\n%s", got)
	}
	if got := subjects(); !strings.HasPrefix(got, "Add sketch: Release flow") {
		t.Errorf("the commit: %s", got)
	}
	// The plugin's told where it is, at what version, and its own name
	// for it is the thing's now.
	if own := r.own["sketch-12"]; own.remote != name || own.base == "" || own.base != flow.Version {
		t.Errorf("what the plugin was told: %+v (the thing's at %s)", own, flow.Version)
	}
	if at, ok, _ := plugin.AliasOf(w.Root, "sketch", "sketch", "sketch-12"); !ok || at != name {
		t.Errorf("what sketch-12 names now: %q, %v", at, ok)
	}
	// ws has found it there, and had it read.
	if res := testutil.RunCommand(t, w.Root, nil, "find", "lantern"); !strings.Contains(res.Stdout, name) {
		t.Errorf("found once it's promoted:\n%s", res.Stdout)
	}

	// It lives there now: it's saved, not promoted again. Another of the
	// same title has a folder of its own, and one put where you say is
	// named after that folder.
	if _, err := ask(protocol.MethodRemotePromote, `{"kind":"sketch","ref":"sketch-12"}`); err == nil || !strings.Contains(err.Error(), "lives at "+name+" already") {
		t.Errorf("promoted twice: %v", err)
	}
	if again, err := ask(protocol.MethodRemotePromote, `{"kind":"sketch","ref":"sketch-14"}`); err != nil || again.Thing.Name != "records/sketches/release-flow-2/" || again.Thing.Main != "release-flow-2.excalidraw" {
		t.Errorf("another of the same title: %+v, %v", again.Thing, err)
	}
	if there, err := ask(protocol.MethodRemotePromote, `{"kind":"sketch","ref":"sketch-13","to":"records:design/arch/"}`); err != nil || there.Thing.Name != "records/design/arch/" || there.Thing.Main != "arch.excalidraw" || onMain("design/arch/arch.svg") != "<svg>layers</svg>" {
		t.Errorf("one put where you said: %+v, %v", there.Thing, err)
	}
	// What ws won't do is said: somewhere taken, somewhere ws wouldn't
	// find it, a thing the plugin hasn't, a kind that isn't its own.
	r.own["sketch-15"] = &ownSketch{title: "Clash", scene: "x"}
	for _, c := range [][2]string{
		{`{"kind":"sketch","ref":"sketch-15","to":"records:design/arch/"}`, "there's something at records:design/arch/ already"},
		{`{"kind":"sketch","ref":"sketch-15","to":"nowhere:a/b/"}`, "isn't one of the workspace's repos"},
		{`{"kind":"sketch","ref":"sketch-15","to":"records:../out/"}`, "leads out of"},
		{`{"kind":"sketch","ref":"sketch-99"}`, "nothing by that name"},
		{`{"kind":"issue","ref":"sketch-15"}`, "has no kind"},
	} {
		if _, err := ask(protocol.MethodRemotePromote, c[0]); err == nil || !strings.Contains(err.Error(), c[1]) {
			t.Errorf("promote %s: %v, want it to say %q", c[0], err, c[1])
		}
	}
	if _, err := ask(protocol.MethodRemoteSave, `{"kind":"sketch","ref":"sketch-15"}`); err == nil || !strings.Contains(err.Error(), "isn't in a repo: promote it first") {
		t.Errorf("one that isn't in a repo, saved: %v", err)
	}

	// Saved as it is: nothing's committed. Saved once it's changed, and
	// renamed: one commit, the folder and its files named as they were.
	before := subjects()
	if same, err := ask(protocol.MethodRemoteSave, `{"kind":"sketch","ref":"sketch-12"}`); err != nil || same.Changed || same.Merged || same.Thing.Version != flow.Version || subjects() != before {
		t.Errorf("saved as it was: %+v, %v", same, err)
	}
	r.own["sketch-12"].scene, r.own["sketch-12"].title = "ship it by candle", "How a release goes"
	saved, err := ask(protocol.MethodRemoteSave, `{"kind":"sketch","ref":"sketch-12"}`)
	if err != nil || !saved.Changed || saved.Merged || saved.Thing.Name != name || saved.Thing.Title != "How a release goes" || saved.Thing.Version == flow.Version {
		t.Fatalf("saved: %+v, %v", saved, err)
	}
	if got := onMain("sketches/release-flow/release-flow.excalidraw"); got != "ship it by candle" {
		t.Errorf("its main file on main, saved: %q", got)
	}
	if got := subjects(); !strings.HasPrefix(got, "Amend sketch release-flow") {
		t.Errorf("the commit: %s", got)
	}
	if r.own["sketch-12"].base != saved.Thing.Version {
		t.Errorf("the plugin wasn't told what it saved at")
	}
	if res := testutil.RunCommand(t, w.Root, nil, "find", "candle"); !strings.Contains(res.Stdout, name) || !strings.Contains(res.Stdout, "How a release goes") {
		t.Errorf("found by what's on it now:\n%s", res.Stdout)
	}

	// Someone else saves it meanwhile, on the remote: the plugin's asked
	// again with what's there, and what it gives then is what's saved.
	testutil.CommitIn(t, w.Sources["records"], "sketches/release-flow/release-flow.excalidraw", "their circles", "Someone else's change")
	r.own["sketch-12"].scene = "my triangles"
	r.exported = nil
	merged, err := ask(protocol.MethodRemoteSave, `{"kind":"sketch","ref":"sketch-12"}`)
	if err != nil || !merged.Changed || !merged.Merged {
		t.Fatalf("saved over a change there: %+v, %v", merged, err)
	}
	if want := []string{"sketch-12", "sketch-12 with their circles"}; !slices.Equal(r.exported, want) {
		t.Errorf("the plugin was asked %q, want %q", r.exported, want)
	}
	if got := onMain("sketches/release-flow/release-flow.excalidraw"); got != "my triangles + their circles" {
		t.Errorf("what's on main after: %q", got)
	}
	if got := onMain("sketches/release-flow/release-flow.svg"); got != "<svg>my triangles + their circles</svg>" {
		t.Errorf("its picture after: %q", got)
	}
}

// Where a kind's things go: the places ws.toml names for it, the first
// where one goes unless you say, and all of them offered; with none
// named, the folder the kind's plugin names, in the records repo and then
// in each other space.
func TestWhereAKindsThingsGo(t *testing.T) {
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{Org: "test-org", DefaultBranch: "main", Repos: []testutil.RepoOpts{
		{Name: "records", Landing: "direct"}, {Name: "docs", Landing: "direct"}, {Name: "api"},
	}})
	said := func(text string) {
		t.Helper()
		f, err := os.OpenFile(filepath.Join(w.Root, "ws.toml"), os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.WriteString(text); err != nil {
			t.Fatal(err)
		}
	}
	said("\n[missions]\nrecords = \"records\"\n")
	registerPlugin(t, w.Root, "sketch", plugin.Registration{Commands: &plugin.Commands{}, Refs: []protocol.Ref{sketchKind}})
	if refused, err := plugin.KeepRefs(w.Root, "sketch", []protocol.Ref{sketchKind}); err != nil || len(refused) != 0 {
		t.Fatalf("keeping sketch's refs: %v, %v", refused, err)
	}
	r := &reader{own: map[string]*ownSketch{"sketch-1": {title: "Release flow", scene: "boxes"}, "sketch-2": {title: "Architecture", scene: "layers"}}}
	was := ops.RemoteReaders
	ops.RemoteReaders = func(string) (ops.RemoteReader, error) { return r, nil }
	t.Cleanup(func() { ops.RemoteReaders = was })
	offered := func() string {
		t.Helper()
		offers, err := cli.OpenOps(w.Root, nil).ThingOffered("sketch", "sketch", "Release flow")
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, o := range offers {
			out = append(out, o.Where)
		}
		return strings.Join(out, " ")
	}
	promote := func(params string) protocol.RemoteSaved {
		t.Helper()
		out, err := cli.AnswerPlugin(w.Root)(context.Background(), "sketch", protocol.MethodRemotePromote, []byte(params))
		if err != nil {
			t.Fatal(err)
		}
		return out.(protocol.RemoteSaved)
	}

	// With nothing said: the kind's own folder, in the one space there is.
	if got := offered(); got != "records:sketches/release-flow/" {
		t.Fatalf("offered, with nothing said: %s", got)
	}
	// Another repo that's a space is another place, after records.
	toml, err := os.ReadFile(filepath.Join(w.Root, "ws.toml"))
	if err != nil || !strings.Contains(string(toml), "[repos.docs]\n") {
		t.Fatalf("the workspace's ws.toml has no [repos.docs]: %v\n%s", err, toml)
	}
	if err := os.WriteFile(filepath.Join(w.Root, "ws.toml"), []byte(strings.Replace(string(toml), "[repos.docs]\n", "[repos.docs]\ndocuments = true\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := offered(); got != "records:sketches/release-flow/ docs:sketches/release-flow/" {
		t.Fatalf("offered, with two spaces: %s", got)
	}

	// Named in ws.toml: those, in their order, and no others. One ws
	// can't use isn't offered, and is said.
	said("\n[things.sketch]\npaths = [\"records:plans/sketches/{slug}/\", \"docs:architecture/{slug}/\", \"api:sketches/{slug}/\", \"docs:flat/\"]\n\n[things.note]\npaths = [\"records:notes/{slug}/\"]\n")
	if got := offered(); got != "records:plans/sketches/release-flow/ docs:architecture/release-flow/" {
		t.Fatalf("offered, as ws.toml names them: %s", got)
	}
	// A workspace with settings of yours laid over the team's has them still.
	if err := os.WriteFile(filepath.Join(w.Root, "ws.local.toml"), []byte("[web]\nport = 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := offered(); got != "records:plans/sketches/release-flow/ docs:architecture/release-flow/" {
		t.Fatalf("offered, with a ws.local.toml beside ws.toml: %s", got)
	}
	problems := strings.Join(cli.OpenOps(w.Root, nil).ThingPlaceProblems(), "\n")
	for _, want := range []string{`[things.note]: no plugin here has a kind "note"`, `[things.sketch] "api:sketches/{slug}/": api isn't a space of documents`, `[things.sketch] "docs:flat/" is a folder for each one`} {
		if !strings.Contains(problems, want) {
			t.Errorf("what's wrong with the places lacks %q:\n%s", want, problems)
		}
	}

	// Promoted with nowhere said, it goes to the first; where you say,
	// there. What's taken there is offered under another name.
	if done := promote(`{"kind":"sketch","ref":"sketch-1"}`); done.Thing.Name != "records/plans/sketches/release-flow/" {
		t.Errorf("promoted with nowhere said: %+v", done.Thing)
	}
	if done := promote(`{"kind":"sketch","ref":"sketch-2","to":"docs:architecture/architecture/"}`); done.Thing.Name != "docs/architecture/architecture/" || done.Thing.Main != "architecture.excalidraw" {
		t.Errorf("promoted to the second place: %+v", done.Thing)
	}
	if got := offered(); got != "records:plans/sketches/release-flow-2/ docs:architecture/release-flow/" {
		t.Fatalf("offered, once one's there: %s", got)
	}
}

// A kind whose things may be grouped ([things.<kind>] subfolders) has one
// put in a folder of the place it goes, named as it's promoted: its own
// folder is named for its title there. ws finds it as it finds any, the
// folders in use are offered the next time, and nothing's put inside
// another thing's folder.
func TestAKindsThingsAreGroupedInFolders(t *testing.T) {
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{Org: "test-org", DefaultBranch: "main", Repos: []testutil.RepoOpts{
		{Name: "records", Landing: "direct"}, {Name: "docs", Landing: "direct"},
	}})
	toml, err := os.ReadFile(filepath.Join(w.Root, "ws.toml"))
	if err != nil || !strings.Contains(string(toml), "[repos.docs]\n") {
		t.Fatalf("the workspace's ws.toml has no [repos.docs]: %v\n%s", err, toml)
	}
	base := strings.Replace(string(toml), "[repos.docs]\n", "[repos.docs]\ndocuments = true\n", 1) + "\n[missions]\nrecords = \"records\"\n\n[things.sketch]\npaths = [\"records:sketches/{slug}/\", \"docs:architecture/{slug}/\"]\n"
	configure := func(more string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(w.Root, "ws.toml"), []byte(base+more), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	configure("")
	registerPlugin(t, w.Root, "sketch", plugin.Registration{Commands: &plugin.Commands{}, Refs: []protocol.Ref{sketchKind}})
	if refused, err := plugin.KeepRefs(w.Root, "sketch", []protocol.Ref{sketchKind}); err != nil || len(refused) != 0 {
		t.Fatalf("keeping sketch's refs: %v, %v", refused, err)
	}
	r := &reader{own: map[string]*ownSketch{}}
	for _, name := range []string{"sketch-1", "sketch-2", "sketch-3", "sketch-4", "sketch-5"} {
		r.own[name] = &ownSketch{title: "Architecture", scene: "layers of " + name}
	}
	r.own["sketch-2"].title = "Roadmap"
	was := ops.RemoteReaders
	ops.RemoteReaders = func(string) (ops.RemoteReader, error) { return r, nil }
	t.Cleanup(func() { ops.RemoteReaders = was })
	ctx := context.Background()
	promote := func(name string, where ops.ThingWhere) (protocol.RemoteSaved, error) {
		return cli.OpenOps(w.Root, nil).PromoteThing(ctx, "sketch", "sketch", name, where)
	}
	offers := func() []ops.ThingOffer {
		t.Helper()
		out, err := cli.OpenOps(w.Root, nil).ThingOffered("sketch", "sketch", "Architecture")
		if err != nil || len(out) != 2 {
			t.Fatalf("offered: %+v, %v", out, err)
		}
		return out
	}

	// Not said in ws.toml: one goes straight in its place, and a folder
	// to group it in is refused, saying what would let it.
	if o := offers(); o[0].Subfolders || o[0].Place != "records:sketches/{slug}/" || o[1].Place != "docs:architecture/{slug}/" {
		t.Fatalf("offered, ungrouped: %+v", o)
	}
	var invalid *ops.Invalid
	if _, err := promote("sketch-1", ops.ThingWhere{Under: "cross-stack"}); !errors.As(err, &invalid) || !strings.Contains(err.Error(), "[things.sketch] subfolders = true") {
		t.Fatalf("grouped where things aren't: %v", err)
	}

	configure("subfolders = true\n")
	if o := offers(); !o[0].Subfolders || len(o[0].Groups) != 0 {
		t.Fatalf("offered, with nothing grouped yet: %+v", o)
	}
	// What isn't a folder's name, or leads out of the place, isn't taken.
	for _, bad := range []string{"../elsewhere", "a//b", ".hidden", "two words", "a/b/c/d/e/f/g"} {
		if _, err := promote("sketch-1", ops.ThingWhere{Under: bad}); !errors.As(err, &invalid) {
			t.Errorf("grouped in %q: %v", bad, err)
		}
	}
	if _, err := promote("sketch-1", ops.ThingWhere{Place: "records:elsewhere/{slug}/"}); !errors.As(err, &invalid) || !strings.Contains(err.Error(), "isn't one of the places") {
		t.Errorf("a place that isn't one of the kind's: %v", err)
	}

	// In a folder of the first place, and of the second, some folders deep.
	done, err := promote("sketch-1", ops.ThingWhere{Under: "/cross-stack/"})
	if err != nil || done.Thing.Name != "records/sketches/cross-stack/architecture/" || done.Thing.Main != "architecture.excalidraw" {
		t.Fatalf("grouped in the first place: %+v, %v", done.Thing, err)
	}
	done, err = promote("sketch-2", ops.ThingWhere{Place: "docs:architecture/{slug}/", Under: "plans/q4"})
	if err != nil || done.Thing.Name != "docs/architecture/plans/q4/roadmap/" {
		t.Fatalf("grouped in the second place: %+v, %v", done.Thing, err)
	}
	// The plugin's told where each is among its kind's things, to list
	// them as they're organised: the folder it's grouped in, and what ws
	// calls its repo.
	if done.Thing.Group != "plans/q4" || done.Thing.RepoLabel == "" {
		t.Errorf("where the second is, as the plugin's told: group %q, in %q", done.Thing.Group, done.Thing.RepoLabel)
	}
	// ws finds each where it is, as it finds any.
	for _, name := range []string{"records/sketches/cross-stack/architecture/", "docs/architecture/plans/q4/roadmap/"} {
		if rec, ok, err := plugin.ThingOf(w.Root, "sketch", "sketch", name); err != nil || !ok || rec.Found == "" {
			t.Errorf("%s, as ws keeps it: %+v, %v, %v", name, rec, ok, err)
		}
	}
	// The folders in use are offered, each place its own, with those they're in.
	o := offers()
	if got := strings.Join(o[0].Groups, " "); got != "cross-stack" {
		t.Errorf("the first place's folders: %q", got)
	}
	if got := strings.Join(o[1].Groups, " "); got != "plans plans/q4" {
		t.Errorf("the second place's folders: %q", got)
	}
	// Straight in the place, the name's free still; in the folder, it's taken.
	if o[0].Where != "records:sketches/architecture/" {
		t.Errorf("offered straight in the place: %s", o[0].Where)
	}
	if done, err := promote("sketch-3", ops.ThingWhere{Under: "cross-stack"}); err != nil || done.Thing.Name != "records/sketches/cross-stack/architecture-2/" {
		t.Errorf("a second of that title in the folder: %+v, %v", done.Thing, err)
	}

	// A thing's folder is the thing's: nothing's put in it, grouped there or named there.
	if _, err := promote("sketch-4", ops.ThingWhere{Under: "cross-stack/architecture"}); !errors.As(err, &invalid) || !strings.Contains(err.Error(), "is a thing itself") {
		t.Errorf("grouped in a thing's own folder: %v", err)
	}
	if _, err := promote("sketch-4", ops.ThingWhere{To: "records:sketches/cross-stack/architecture/inside/"}); !errors.As(err, &invalid) || !strings.Contains(err.Error(), "is a thing itself") {
		t.Errorf("put in a thing's own folder: %v", err)
	}
	// Where you say is still anywhere in a space, however deep.
	if done, err := promote("sketch-5", ops.ThingWhere{To: "records:sketches/cross-stack/deeper/arch/"}); err != nil || done.Thing.Name != "records/sketches/cross-stack/deeper/arch/" {
		t.Errorf("put where you said: %+v, %v", done.Thing, err)
	}
	if got := strings.Join(offers()[0].Groups, " "); got != "cross-stack cross-stack/deeper" {
		t.Errorf("the first place's folders, with one put deeper by hand: %q", got)
	}

	// One straight in its place, and one in none of the kind's places: the
	// plugin's list says each one's group, the last by the folders it's in.
	pack(t, w, "flat", "Flat", "one box")
	ground := testutil.CapsuleDir(w, "records", ".ground")
	if err := os.MkdirAll(filepath.Join(ground, "notes", "drawn", "aside"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ground, "notes", "drawn", "aside", "thing.toml"), []byte("kind = \"sketch\"\ntitle = \"Aside\"\nmain = \"aside.excalidraw\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.CommitIn(t, ground, "notes/drawn/aside/aside.excalidraw", "a doodle", "A sketch beside a note")
	listed, err := cli.AnswerPlugin(w.Root)(ctx, "sketch", protocol.MethodRemoteList, []byte(`{"kind":"sketch"}`))
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string]string{}
	for _, thing := range listed.(protocol.RemoteListResult).Things {
		groups[thing.Name] = thing.Group + " in " + thing.RepoLabel
	}
	label := cli.OpenOps(w.Root, nil).WS().DisplayNameFor
	for name, want := range map[string]string{
		"records/sketches/flat/":                     " in " + label("records"),
		"records/sketches/cross-stack/architecture/": "cross-stack in " + label("records"),
		"records/sketches/cross-stack/deeper/arch/":  "cross-stack/deeper in " + label("records"),
		"docs/architecture/plans/q4/roadmap/":        "plans/q4 in " + label("docs"),
		"records/notes/drawn/aside/":                 "notes/drawn in " + label("records"),
	} {
		if groups[name] != want {
			t.Errorf("%s is listed as %q, want %q", name, groups[name], want)
		}
	}
}
