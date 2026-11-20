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
	return protocol.RemoteExported{Name: own.remote, Base: own.base, RemotePackage: protocol.RemotePackage{
		Title: own.title, Main: []byte(own.scene),
		Files: []protocol.RemoteFile{{Name: "{slug}.svg", Content: []byte("<svg>" + own.scene + "</svg>"), Picture: true}},
	}}, nil
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
