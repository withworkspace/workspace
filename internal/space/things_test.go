package space

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/workspace"
)

// A folder with a thing.toml in it is a thing in its space: found on the
// default branch wherever the folder is, read again only when its
// manifest or its main file changes, and gone with either.
func TestRefresh_FindsTheThingsInASpace(t *testing.T) {
	root := t.TempDir()
	db, err := store.Own(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ws := &workspace.Workspace{Root: root, RepoNames: []string{"records"}, Records: "records"}
	rec := ws.MainWorktree("records")
	os.MkdirAll(rec, 0o755)
	gitIn(t, rec, "init", "-q", "-b", "main")
	gitIn(t, rec, "config", "user.name", "Dana")
	gitIn(t, rec, "config", "user.email", "dana@example.com")

	write(t, rec, "docs/plan.md", "# Plan\n")
	write(t, rec, "sketches/release-flow/thing.toml", "kind = \"sketch\"\ntitle = \"Release  flow\"\nmain = \"release-flow.excalidraw\"\npicture = \"release-flow.svg\"\n")
	write(t, rec, "sketches/release-flow/release-flow.excalidraw", `{"elements":[]}`)
	write(t, rec, "sketches/release-flow/release-flow.svg", "<svg/>")
	// Not things: a file of the kind in no folder of its own, a manifest
	// at the space's top, one whose main file isn't there, one that names
	// a file outside its folder, and one with no kind.
	write(t, rec, "sketches/lone.excalidraw", `{"elements":[]}`)
	write(t, rec, "thing.toml", "kind = \"sketch\"\nmain = \"docs/plan.md\"\n")
	write(t, rec, "sketches/no-main/thing.toml", "kind = \"sketch\"\nmain = \"gone.excalidraw\"\n")
	write(t, rec, "sketches/outside/thing.toml", "kind = \"sketch\"\nmain = \"../lone.excalidraw\"\n")
	write(t, rec, "sketches/no-kind/thing.toml", "title = \"No kind\"\nmain = \"a.txt\"\n")
	write(t, rec, "sketches/no-kind/a.txt", "a")
	// One with no title is called by its folder, anywhere in the space.
	write(t, rec, "deep/down/arch/thing.toml", "kind = \"sketch\"\nmain = \"scene.excalidraw\"\n")
	write(t, rec, "deep/down/arch/scene.excalidraw", `{"elements":[1]}`)
	gitIn(t, rec, "add", ".")
	gitIn(t, rec, "commit", "-q", "-m", "first", "--date=2026-11-20T10:00:00Z")

	said := func() string {
		t.Helper()
		things, err := Things(bg, db, "")
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, th := range things {
			out = append(out, fmt.Sprintf("%s %s %q main=%s picture=%s by=%s", th.Name(), th.Kind, th.Title, th.Main, th.Picture, th.ChangedBy))
		}
		return strings.Join(out, "\n")
	}
	st, err := Refresh(bg, db, ws, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.Read != 1 || st.Things != 2 {
		t.Fatalf("first: %+v", st)
	}
	want := `records/deep/down/arch/ sketch "arch" main=scene.excalidraw picture= by=Dana
records/sketches/release-flow/ sketch "Release flow" main=release-flow.excalidraw picture=release-flow.svg by=Dana`
	if got := said(); got != want {
		t.Fatalf("the things found:\n%s\nwant:\n%s", got, want)
	}
	flow, ok, err := ThingAt(bg, db, "records", "sketches/release-flow")
	if err != nil || !ok || flow.Changed == "" {
		t.Fatalf("the release flow: %+v, %v, %v", flow, ok, err)
	}
	if text, err := MainFile(bg, ws, flow); err != nil || string(text) != `{"elements":[]}` {
		t.Fatalf("its main file: %q, %v", text, err)
	}
	if things, _ := Things(bg, db, "note"); len(things) != 0 {
		t.Fatalf("things of a kind there are none of: %+v", things)
	}

	// Nothing's moved: nothing's read.
	if st, err := Refresh(bg, db, ws, nil); err != nil || st.Moved != 0 || st.Things != 0 {
		t.Fatalf("again: %+v, %v", st, err)
	}

	// A commit that changes one's main file reads that one, and its
	// version is another; a picture's change isn't the thing's.
	write(t, rec, "sketches/release-flow/release-flow.excalidraw", `{"elements":[2]}`)
	write(t, rec, "deep/down/arch/arch.svg", "<svg/>")
	gitIn(t, rec, "-c", "user.name=Sam", "commit", "-q", "-am", "second", "--date=2026-11-20T11:00:00Z")
	gitIn(t, rec, "add", ".")
	gitIn(t, rec, "commit", "-q", "-m", "a picture", "--date=2026-11-20T12:00:00Z")
	if st, err := Refresh(bg, db, ws, nil); err != nil || st.Things != 1 {
		t.Fatalf("after a change: %+v, %v", st, err)
	}
	moved, _, _ := ThingAt(bg, db, "records", "sketches/release-flow/")
	if moved.Version() == flow.Version() || moved.Manifest != flow.Manifest || moved.ChangedBy != "Sam" {
		t.Fatalf("the release flow, changed: %+v (was %+v)", moved, flow)
	}
	if text, _ := MainFile(bg, ws, moved); string(text) != `{"elements":[2]}` {
		t.Fatalf("its main file, changed: %q", text)
	}
	// What was read before is still what its version says.
	if text, _ := MainFile(bg, ws, flow); string(text) != `{"elements":[]}` {
		t.Fatalf("its main file as it was: %q", text)
	}

	// Renamed in its manifest, and another's manifest gone: one read, one gone.
	write(t, rec, "sketches/release-flow/thing.toml", "kind = \"sketch\"\ntitle = \"How a release goes\"\nmain = \"release-flow.excalidraw\"\n")
	gitIn(t, rec, "rm", "-q", "deep/down/arch/thing.toml")
	gitIn(t, rec, "commit", "-q", "-am", "third", "--date=2026-11-20T13:00:00Z")
	if st, err := Refresh(bg, db, ws, nil); err != nil || st.Things != 2 {
		t.Fatalf("after a rename and a removal: %+v, %v", st, err)
	}
	if got, want := said(), `records/sketches/release-flow/ sketch "How a release goes" main=release-flow.excalidraw picture= by=Dana`; got != want {
		t.Fatalf("the things left:\n%s\nwant:\n%s", got, want)
	}

	// A repo that's no longer a space has none.
	ws.RepoNames, ws.Records = nil, ""
	if _, err := Refresh(bg, db, ws, nil); err != nil {
		t.Fatal(err)
	}
	if got := said(); got != "" {
		t.Fatalf("things of a repo that isn't a space: %s", got)
	}
}

// A thing.toml as ws writes one is one ws reads, whatever's in its title,
// and a file's id is git's.
func TestManifest_WrittenAsItsRead(t *testing.T) {
	text, err := Manifest("sketch", "  The \"release\"\tflow: é → ü  ", "release-flow.excalidraw", "release-flow.svg")
	if err != nil {
		t.Fatal(err)
	}
	kind, title, main, picture, ok := ReadManifest(text)
	if !ok || kind != "sketch" || title != `The "release" flow: é → ü` || main != "release-flow.excalidraw" || picture != "release-flow.svg" {
		t.Fatalf("read back: %q %q %q %q %v\n%s", kind, title, main, picture, ok, text)
	}
	if text, _ := Manifest("sketch", "Untitled", "a.excalidraw", ""); strings.Contains(string(text), "picture") {
		t.Errorf("one with no picture says one:\n%s", text)
	}
	for _, bad := range [][2]string{{"Sketch", "a.txt"}, {"sketch", "../a.txt"}, {"sketch", "thing.toml"}, {"", "a.txt"}} {
		if _, err := Manifest(bad[0], "A", bad[1], ""); err == nil {
			t.Errorf("a manifest of kind %q and main %q was written", bad[0], bad[1])
		}
	}
	if got := BlobID([]byte("hello\n")); got != "ce013625030ba8dba906f756967f9e9ca394464a" {
		t.Errorf("git's id for hello: %s", got)
	}
}
