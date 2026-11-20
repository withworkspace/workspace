package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

const page = "<!doctype html>\n<title>Glow</title>\n<style>body{background:radial-gradient(#fff3,#000)}</style>\n"

var day = time.Date(2026, 11, 12, 18, 46, 0, 0, time.Local)

func newStore(t *testing.T) Store {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".ws"), 0o755)
	storetest.Own(t, root)
	return Store{Root: root}
}

func add(t *testing.T, s Store, title, note string) Entry {
	t.Helper()
	e, err := s.Add(context.Background(), New{Title: title, HTML: []byte(page), At: day, Version: Version{By: "c1", From: "glow", FromTitle: "The glow", Note: note}})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// An artifact is a folder named from its day and title, with its record
// and its first version, and a row that numbers it; a version is a whole
// new file, based on the one it revises, and the record says who made
// each and why.
func TestAddAndRevise(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	e := add(t, s, "  Horizon glow on the web UI background ", "Accent at 8% in dark and 3% in light.")
	if e.ID != "a1" || e.Folder != "2026-11-12-horizon-glow-on-the-web-ui-background" || e.Title != "Horizon glow on the web UI background" {
		t.Fatalf("added %+v", e)
	}
	if got, _, _, err := s.ReadVersion(ctx, "a1", 1); err != nil || string(got) != page {
		t.Errorf("v1's page is %q (%v)", got, err)
	}
	if e.Versions[0].Files != 1 || e.Versions[0].Size != len(page) {
		t.Errorf("v1 says it's %d files, %d bytes", e.Versions[0].Files, e.Versions[0].Size)
	}
	e, n, err := s.AddVersion(ctx, "a1", Revision{HTML: []byte(page + "<p>2</p>"), At: day.Add(time.Hour), Version: Version{By: "you", Note: "No glow in light."}})
	if err != nil || n != 2 {
		t.Fatalf("v2: %d, %v", n, err)
	}
	// A revision of v1 after v2 is v3, based on v1.
	e, n, err = s.AddVersion(ctx, "a1", Revision{HTML: []byte(page + "<p>3</p>"), On: 1, At: day.Add(2 * time.Hour), Version: Version{By: "c2", From: "glow", Note: "Softer."}})
	if err != nil || n != 3 {
		t.Fatalf("v3: %d, %v", n, err)
	}
	data, _ := os.ReadFile(filepath.Join(s.Path(e), RecordFile))
	a, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Versions) != 3 || a.Versions[1].BasedOn != 1 || a.Versions[2].BasedOn != 1 || a.Versions[0].BasedOn != 0 {
		t.Errorf("versions: %+v", a.Versions)
	}
	if v := a.Versions[0]; v.By != "c1" || v.From != "glow" || v.FromTitle != "The glow" || v.Note != "Accent at 8% in dark and 3% in light." || !v.At.Equal(day) {
		t.Errorf("v1's provenance: %+v", v)
	}
	if !strings.Contains(string(data), "[[versions]]\nn = 1\nat = ") || !strings.Contains(string(data), `status = "live"`) || strings.Contains(string(data), "proposal") {
		t.Errorf("artifact.toml:\n%s", data)
	}
	if _, _, err := s.AddVersion(ctx, "a1", Revision{HTML: []byte(page), On: 7, At: day}); err == nil || !strings.Contains(err.Error(), "has no v7") {
		t.Errorf("based on a version it hasn't: %v", err)
	}
	text, _, which, err := s.ReadVersion(ctx, "a1", 0)
	if err != nil || which != 3 || !strings.Contains(string(text), "<p>3</p>") {
		t.Errorf("latest: v%d %q %v", which, text, err)
	}
	// The same title on the same day is a folder of its own, -2.
	if e2 := add(t, s, "Horizon glow on the web UI background", ""); e2.ID != "a2" || !strings.HasSuffix(e2.Folder, "-background-2") {
		t.Errorf("second of the same name: %+v", e2)
	}
}

// artifact.toml is the record: edited by hand, it wins over the row; a
// folder that turns up with one is given a ref; a folder that's gone is
// kept as gone, its number not used again.
func TestTheFileWins(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	e := add(t, s, "Glow", "First.")
	path := filepath.Join(s.Path(e), RecordFile)
	data, _ := os.ReadFile(path)
	os.WriteFile(path, []byte(strings.Replace(string(data), `title = "Glow"`, `title = "Glow, by hand"`, 1)), 0o644)

	other := filepath.Join(s.Dir(), "2026-11-01-brought-in")
	os.MkdirAll(other, 0o755)
	os.WriteFile(filepath.Join(other, VersionFile(1)), []byte(page), 0o644)
	os.WriteFile(filepath.Join(other, RecordFile), []byte("title = \"Brought in\"\nmade = 2026-11-01T10:00:00Z\n\n[[versions]]\nn = 1\nat = 2026-11-01T10:00:00Z\nby = \"you\"\n"), 0o644)

	list, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Entry{}
	for _, e := range list {
		got[e.ID] = e
	}
	if got["a1"].Title != "Glow, by hand" || got["a2"].Title != "Brought in" || got["a2"].Status != Live {
		t.Errorf("listed %+v", list)
	}
	if names, _ := found(t, s.Root, "brought"); len(names) != 1 || names[0] != "a2" {
		t.Errorf("found %v", names)
	}

	os.RemoveAll(other)
	e2, err := s.Get(ctx, "a2")
	if err != nil || !e2.Gone {
		t.Errorf("gone: %+v %v", e2, err)
	}
	if e3 := add(t, s, "Third", ""); e3.ID != "a3" {
		t.Errorf("a gone artifact's number was used again: %s", e3.ID)
	}
}

// Discarding moves the folder aside and keeps the row, out of the graph;
// a discarded artifact takes no versions, and its name isn't used again.
func TestDiscard(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	e := add(t, s, "Glow", "First.")
	d, err := s.Discard(ctx, "a1", Discard{At: day, By: "you", Reason: "  the second one won "})
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != Discarded || d.Discarded.Reason != "the second one won" || d.Where() != ".ws/artifacts/.discarded/"+e.Folder+"/" {
		t.Errorf("discarded %+v", d)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), e.Folder)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("its folder is still there")
	}
	data, _ := os.ReadFile(filepath.Join(s.DiscardedDir(), e.Folder, RecordFile))
	if !strings.Contains(string(data), `status = "discarded"`) || !strings.Contains(string(data), `reason = "the second one won"`) {
		t.Errorf("its record:\n%s", data)
	}
	if got, err := s.Get(ctx, "a1"); err != nil || got.Status != Discarded || got.Gone {
		t.Errorf("after: %+v %v", got, err)
	}
	if text, _, _, err := s.ReadVersion(ctx, "a1", 1); err != nil || string(text) != page {
		t.Errorf("its version is still read from .discarded: %q %v", text, err)
	}
	if names, _ := found(t, s.Root, "glow"); len(names) != 0 {
		t.Errorf("a discarded artifact is found: %v", names)
	}
	if _, _, err := s.AddVersion(ctx, "a1", Revision{HTML: []byte(page), At: day}); !errors.Is(err, ErrDiscarded) {
		t.Errorf("a version of a discarded one: %v", err)
	}
	if _, err := s.Discard(ctx, "a1", Discard{At: day, By: "you"}); !errors.Is(err, ErrDiscarded) {
		t.Errorf("discarded twice: %v", err)
	}
	if again := add(t, s, "Glow", ""); again.Folder == e.Folder {
		t.Errorf("a discarded folder's name was used again: %s", again.Folder)
	}
}

// Litter is what belongs to no artifact: a folder with no record, a
// version its record doesn't name, a file beside the folders, a .tmp.
// What's young may be a write under way.
func TestLitter(t *testing.T) {
	s := newStore(t)
	e := add(t, s, "Glow", "")
	dir := s.Path(e)
	// A version committed that the record never named, and, from when a
	// version was a file, a file it doesn't name.
	if _, err := s.Repo(e).CommitVersion(context.Background(), 2, 1, Page([]byte(page)), "v2", day); err != nil {
		t.Fatal(err)
	}
	unnamed := string(s.Repo(e)) + " (v2, which artifact.toml doesn't name)"
	stray := filepath.Join(dir, VersionFile(4))
	os.WriteFile(stray, []byte(page), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("mine"), 0o644) // belongs to it
	noRecord := filepath.Join(s.Dir(), "2026-11-11-half")
	os.MkdirAll(noRecord, 0o755)
	beside := filepath.Join(s.Dir(), "loose.html")
	os.WriteFile(beside, []byte(page), 0o644)
	young, _ := s.Litter(context.Background())
	if len(young) != 0 {
		t.Errorf("young files are litter: %v", young)
	}
	past := time.Now().Add(-2 * time.Minute)
	for _, p := range []string{stray, noRecord, beside, filepath.Join(dir, "notes.md"), filepath.Join(dir, RecordFile)} {
		os.Chtimes(p, past, past)
	}
	got, err := s.Litter(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{stray: true, unnamed: true, noRecord: true, beside: true}
	if len(got) != len(want) {
		t.Errorf("litter %v", got)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("%s isn't litter", p)
		}
	}
	// A version past the unnamed one is numbered past it: a number is
	// never used for two.
	if _, n, err := s.AddVersion(context.Background(), e.ID, Revision{HTML: []byte(page), At: day}); err != nil || n != 3 {
		t.Errorf("past the stray: v%d %v", n, err)
	}
}

func TestCheck(t *testing.T) {
	for _, ok := range []string{page, "<!-- made by c1,\n for the glow -->\n" + page, "\xef\xbb\xbf  " + page} {
		c, err := Check(Page([]byte(ok)))
		if err != nil || c.Title != "Glow" || len(c.Warnings) != 0 {
			t.Errorf("a plain page (%.30q): %+v %v", ok, c, err)
		}
	}
	for _, tc := range []struct{ in, want string }{
		{"   \n", "nothing in it"},
		{"# Glow\n\nIt's markdown.", "isn't HTML"},
		{"<!doctype html>\x00\x01", "isn't text"},
		{"<!doctype html>" + strings.Repeat("x", MaxSize), "at most 10 MB"},
	} {
		if _, err := Check(Page([]byte(tc.in))); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Check(%.20q): %v, want %q", tc.in, err, tc.want)
		}
	}
	c, err := Check(Page([]byte(`<div>no doctype</div>
<script src="https://cdn.example.com/react.js"></script>
<link rel="stylesheet" href="//fonts.example.com/inter.css">
<style>@import url("https://example.com/a.css"); body { background: url(https://example.com/bg.png) }</style>
<a href="https://example.com">a link loads nothing</a>
<img src="data:image/png;base64,AAAA">
<script>fetch('/api/workspace'); fetch("data.json"); localStorage.setItem('x', 1)</script>`)))
	if err != nil {
		t.Fatal(err)
	}
	said := strings.Join(c.Warnings, "\n")
	for _, want := range []string{"<!doctype html>", "<script> loads https://cdn.example.com/react.js", "<link> loads //fonts.example.com/inter.css", "CSS loads https://example.com/a.css", "CSS loads https://example.com/bg.png", "fetches /api/workspace", "uses localStorage"} {
		if !strings.Contains(said, want) {
			t.Errorf("warnings don't say %q:\n%s", want, said)
		}
	}
	if strings.Contains(said, "example.com\n") || strings.Contains(said, "<a>") || len(c.Warnings) != 7 {
		t.Errorf("warnings:\n%s", said)
	}
}

func TestRefs(t *testing.T) {
	for _, tc := range []struct {
		in string
		id string
		v  int
		ok bool
	}{{"a3", "a3", 0, true}, {"a3.v2", "a3", 2, true}, {"a0", "", 0, false}, {"a3.v0", "", 0, false}, {"a3v2", "", 0, false}, {"p3", "", 0, false}} {
		id, v, ok := ParseRef(tc.in)
		if id != tc.id || v != tc.v || ok != tc.ok {
			t.Errorf("ParseRef(%q) = %q %d %v", tc.in, id, v, ok)
		}
	}
}

func found(t *testing.T, root, words string) (names, headings []string) {
	t.Helper()
	db, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := db.Find(context.Background(), store.Said{Words: words})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		names, headings = append(names, f.Thing.Key.Name), append(headings, f.Heading)
	}
	return names, headings
}
