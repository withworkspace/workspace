package ops

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/release"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/whatsnew"
	"github.com/withworkspace/workspace/internal/workspace"
)

// newsWorld is a workspace whose ws is at version, saying base is where
// it's released, with ix as what that ws says is new.
func newsWorld(t *testing.T, version, base string, ix whatsnew.Index) *Workspace {
	t.Helper()
	root := t.TempDir()
	storetest.Own(t, root)
	was := held
	held = func() whatsnew.Index { return ix }
	t.Cleanup(func() { held = was })
	return New(&config.Config{}, &workspace.Workspace{Root: root}, nil, Options{
		WS: release.Upgrader{Version: version, Release: !strings.Contains(version, "-0."), Source: release.Source{Base: base}},
	})
}

var news = whatsnew.Arrange([]whatsnew.Entry{
	{Slug: "old", Version: "v1.2.0", Title: "Old", Body: "b"},
	{Slug: "mid", Version: "v1.3.0", Title: "Mid", Body: "b", Picture: "mid.png"},
	{Slug: "new", Version: "v1.4.0", Title: "New", Body: "b"},
	{Slug: "next", Title: "Next", Body: "b"},
}, whatsnew.Keep)

func slugs(es []whatsnew.Entry) string {
	var out []string
	for _, e := range es {
		out = append(out, e.Slug)
	}
	return strings.Join(out, " ")
}

func TestWhatsNew_AWorkspaceThatsSeenNothingStartsFromWhatsRunning(t *testing.T) {
	w := newsWorld(t, "v1.4.0", "", news)
	w.NoteRunning(func() string { return "" })
	if got := w.WhatsNew(); len(got.Unseen) != 0 || slugs(got.Entries) != "new mid old" {
		t.Fatalf("a new workspace: unseen %q, entries %q", slugs(got.Unseen), slugs(got.Entries))
	}
}

func TestWhatsNew_TheFirstUpgradeStartsFromTheWSItReplaced(t *testing.T) {
	w := newsWorld(t, "v1.4.0", "", news)
	w.NoteRunning(func() string { return "v1.2.0" })
	if got := slugs(w.WhatsNew().Unseen); got != "new mid" {
		t.Fatalf("unseen %q, want new mid", got)
	}
	// A daemon started again isn't told twice, nor from further back.
	w.NoteRunning(func() string { return "v1.0.0" })
	if got := slugs(w.WhatsNew().Unseen); got != "new mid" {
		t.Fatalf("unseen %q after starting again", got)
	}
	if err := w.SeenWhatsNew(); err != nil {
		t.Fatal(err)
	}
	if got := w.WhatsNew(); len(got.Unseen) != 0 || got.Missing {
		t.Fatalf("seen, and still unseen %q", slugs(got.Unseen))
	}
}

func TestWhatsNew_AnUpgradeSaysWhatsNewSinceTheLastSeen(t *testing.T) {
	w := newsWorld(t, "v1.4.0", "", news)
	if err := writeSeen(w.Root(), "v1.3.0"); err != nil {
		t.Fatal(err)
	}
	w.NoteRunning(func() string { t.Fatal("asked for the ws replaced, with one seen"); return "" })
	if got := slugs(w.WhatsNew().Unseen); got != "new" {
		t.Fatalf("unseen %q, want new", got)
	}
}

func TestWhatsNew_PutBackSaysNothing(t *testing.T) {
	w := newsWorld(t, "v1.3.0", "", news)
	if err := writeSeen(w.Root(), "v1.4.0"); err != nil {
		t.Fatal(err)
	}
	w.NoteRunning(func() string { return "" })
	if got := w.WhatsNew(); len(got.Unseen) != 0 || slugs(got.Entries) != "mid old" {
		t.Fatalf("rolled back: unseen %q, entries %q", slugs(got.Unseen), slugs(got.Entries))
	}
	if readSeen(w.Root()) != "v1.3.0" {
		t.Fatal("seen should start from the ws put back, so the upgrade after says what's new")
	}
}

func TestWhatsNew_ABuildOfAClone(t *testing.T) {
	w := newsWorld(t, "v1.4.1-0.20261120000000-abcdefabcdef", "", news)
	w.NoteRunning(func() string { return "v1.0.0" })
	got := w.WhatsNew()
	if got.Release || len(got.Unseen) != 0 || slugs(got.Entries) != "next new mid old" {
		t.Fatalf("a dev build: %+v", got)
	}
	if _, err := os.Stat(seenPath(w.Root())); err == nil {
		t.Fatal("a dev build kept a release as seen")
	}
}

func TestWhatsNew_TooFarBackIsOnTheReleasesPage(t *testing.T) {
	ix := whatsnew.Arrange([]whatsnew.Entry{
		{Slug: "a", Version: "v1.1.0", Title: "A", Body: "b"},
		{Slug: "b", Version: "v1.2.0", Title: "B", Body: "b"},
		{Slug: "c", Version: "v1.3.0", Title: "C", Body: "b"},
	}, 2)
	w := newsWorld(t, "v1.3.0", "https://github.com/o/r", ix)
	if err := writeSeen(w.Root(), "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	got := w.WhatsNew()
	if slugs(got.Unseen) != "c b" || !got.Missing || got.Releases != "https://github.com/o/r/releases" {
		t.Fatalf("got %+v", got)
	}
}

func TestWhatsNewPicture_FetchedOnceAndKept(t *testing.T) {
	asked := 0
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		asked++
		if r.URL.Path != "/releases/download/v1.3.0/whatsnew-mid.png" {
			http.NotFound(rw, r)
			return
		}
		rw.Write([]byte("PNG")) //nolint:errcheck // a test's
	}))
	defer srv.Close()
	w := newsWorld(t, "v1.4.0", srv.URL, news)
	for range 2 {
		path, err := w.WhatsNewPicture(context.Background(), "mid.png")
		if err != nil {
			t.Fatal(err)
		}
		if data, _ := os.ReadFile(path); string(data) != "PNG" {
			t.Fatalf("kept %q", data)
		}
	}
	if asked != 1 {
		t.Fatalf("asked GitHub %d times, want once", asked)
	}
	if _, err := w.WhatsNewPicture(context.Background(), "../ws.db"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a picture no entry names: %v", err)
	}
	os.WriteFile(filepath.Join(w.Root(), config.LocalFileName), []byte("[updates]\ncheck = false\n"), 0o644) //nolint:errcheck // a test's
	os.RemoveAll(filepath.Join(w.Root(), ".ws", "cache", "whatsnew"))                                        //nolint:errcheck // a test's
	var refusedErr *Refused
	if _, err := w.WhatsNewPicture(context.Background(), "mid.png"); !errors.As(err, &refusedErr) {
		t.Fatalf("fetched with looking turned off: %v", err)
	}
	if w.WhatsNew().Pictures {
		t.Fatal("says pictures can be fetched with looking turned off")
	}
}

func TestWhatsNew_AnUpgradeWithNoEntriesSaysNothing(t *testing.T) {
	w := newsWorld(t, "v1.5.0", "", news)
	if err := writeSeen(w.Root(), "v1.4.0"); err != nil {
		t.Fatal(err)
	}
	w.NoteRunning(func() string { return "v1.4.0" })
	if got := w.WhatsNew(); len(got.Unseen) != 0 || got.Missing {
		t.Fatalf("v1.5.0 has no entries, and unseen is %q", slugs(got.Unseen))
	}
}

func TestWhatsNewPicture_ABuildOfACloneShowsTheClones(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "next.png"), []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	ix := whatsnew.Arrange([]whatsnew.Entry{
		{Slug: "next", Title: "Next", Body: "b", Picture: "next.png"},
		{Slug: "gone", Title: "Gone", Body: "b", Picture: "gone.png"},
	}, whatsnew.Keep)
	ix.Dir = dir
	w := newsWorld(t, "v1.4.1-0.20261120000000-abcdefabcdef", "https://github.com/o/r", ix)
	if got := w.WhatsNew(); !got.Local || got.Pictures {
		t.Fatalf("a clone's build: local %v, pictures %v", got.Local, got.Pictures)
	}
	path, err := w.WhatsNewPicture(context.Background(), "next.png")
	if err != nil || path != filepath.Join(dir, "next.png") {
		t.Fatalf("got %q, %v", path, err)
	}
	if _, err := w.WhatsNewPicture(context.Background(), "gone.png"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a picture that's not in the clone: %v", err)
	}
	if _, err := w.WhatsNewPicture(context.Background(), "../ws.db"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a picture no entry names: %v", err)
	}
	// A release's build doesn't look in a folder it was built from.
	rel := newsWorld(t, "v1.4.0", "", ix)
	if rel.WhatsNew().Local {
		t.Fatal("a release's build says it shows pictures from a clone")
	}
}
