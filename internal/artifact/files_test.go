package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sheet = "h1 { color: tomato }\n"

func site(extra ...File) Files {
	return append(Files{
		{Path: IndexFile, Data: []byte("<!doctype html>\n<title>Glow</title>\n<link rel=\"stylesheet\" href=\"css/glow.css\">\n<script type=\"module\" src=\"app.js\"></script>\n<img src=\"img/dot.png\">\n")},
		{Path: "css/glow.css", Data: []byte(sheet)},
		{Path: "app.js", Data: []byte("import './lib/util.js';\n")},
		{Path: "lib/util.js", Data: []byte("export {};\n")},
		{Path: "img/dot.png", Data: []byte("\x89PNG\r\n\x1a\n\x00\x01\x02")},
	}, extra...)
}

// A version is its page and the files beside it, each a real file at its
// path, kept as a commit in the artifact's own repo: read back whole or
// one at a time, listed, and checked out to a folder to be worked on.
func TestFiles(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	e, err := s.Add(ctx, New{Title: "Glow", Files: site(), At: day, Version: Version{By: "you", Note: "With files."}})
	if err != nil {
		t.Fatal(err)
	}
	if v := e.Versions[0]; v.Files != 5 || v.Size != site().Size() {
		t.Errorf("v1 says %d files, %d bytes", v.Files, v.Size)
	}
	if !s.Repo(e).Exists() || !s.Repo(e).Has(ctx, 1) {
		t.Fatalf("no repo with v1 in %s", s.Path(e))
	}
	// Nothing's left in the folder but the record and the repo.
	entries, _ := os.ReadDir(s.Path(e))
	var names []string
	for _, d := range entries {
		names = append(names, d.Name())
	}
	if strings.Join(names, " ") != "artifact.toml files.git" {
		t.Errorf("the folder holds %v", names)
	}
	data, blob, _, n, err := s.ReadFile(ctx, "a1", 0, "css/glow.css")
	if err != nil || n != 1 || string(data) != sheet || len(blob) < 40 {
		t.Errorf("css/glow.css: %q %q v%d %v", data, blob, n, err)
	}
	png, _, _, _, err := s.ReadFile(ctx, "a1", 1, "img/dot.png")
	if err != nil || string(png) != "\x89PNG\r\n\x1a\n\x00\x01\x02" {
		t.Errorf("a file that isn't text comes back changed: %q %v", png, err)
	}
	for _, p := range []string{"nope.css", "css", "../artifact.toml", ".git/HEAD", ""} {
		if _, _, _, _, err := s.ReadFile(ctx, "a1", 1, p); !errors.Is(err, ErrNoFile) {
			t.Errorf("reading %q: %v", p, err)
		}
	}
	list, _, _, err := s.VersionFiles(ctx, "a1", 1)
	if err != nil || len(list) != 5 || list[0].Path != "app.js" || list[1].Path != "css/glow.css" || list[1].Size != len(sheet) {
		t.Errorf("listed %+v %v", list, err)
	}

	// A new version changes one file and adds another: the rest are the
	// same files, and the two versions differ by just those.
	next := site(File{Path: "data.json", Data: []byte(`{"n":1}`)})
	next[1].Data = []byte("h1 { color: teal }\n")
	e, n, err = s.AddVersion(ctx, "a1", Revision{Files: next, At: day.Add(time.Hour), Version: Version{By: "c1", From: "glow", Note: "Teal."}})
	if err != nil || n != 2 || e.Versions[1].Files != 6 {
		t.Fatalf("v2: %d %+v %v", n, e.Versions, err)
	}
	was, _ := s.Repo(e).All(ctx, 1)
	is, _ := s.Repo(e).All(ctx, 2)
	diff := Differ(was, is)
	if len(diff) != 2 || diff[0] != (Changed{"css/glow.css", "changed"}) || diff[1] != (Changed{"data.json", "added"}) {
		t.Errorf("v1 to v2 differ by %+v", diff)
	}
	if back := Differ(is, was); len(back) != 2 || back[1] != (Changed{"data.json", "removed"}) {
		t.Errorf("v2 to v1 differ by %+v", back)
	}
	// v2's commit stands on v1's.
	parent, err := s.Repo(e).out(ctx, nil, "rev-parse", "refs/tags/v2^")
	v1, _ := s.Repo(e).out(ctx, nil, "rev-parse", "refs/tags/v1")
	if err != nil || string(parent) != string(v1) {
		t.Errorf("v2's parent is %s, v1 is %s (%v)", parent, v1, err)
	}

	// A name with a space, a quote or an accent in it is a name like any
	// other.
	odd := site(File{Path: "img/my \"big\" logo.svg", Data: []byte("<svg/>")}, File{Path: "données/août.json", Data: []byte("{}")})
	if _, n, err := s.AddVersion(ctx, "a1", Revision{Files: odd, At: day.Add(90 * time.Minute), Version: Version{By: "you"}}); err != nil || n != 3 {
		t.Fatalf("a version with odd names: v%d %v", n, err)
	}
	for _, p := range []string{"img/my \"big\" logo.svg", "données/août.json"} {
		if _, _, _, _, err := s.ReadFile(ctx, "a1", 3, p); err != nil {
			t.Errorf("reading %q: %v", p, err)
		}
	}
	if list, _, _, _ := s.VersionFiles(ctx, "a1", 3); len(list) != 7 || list[2].Path != "données/août.json" {
		t.Errorf("listed %+v", list)
	}

	// Checked out, it's the files as they are, to work on.
	dir := filepath.Join(t.TempDir(), "work")
	if _, n, files, err := s.Checkout(ctx, "a1", 2, dir); err != nil || n != 2 || len(files) != 6 {
		t.Fatalf("checkout: v%d %d files %v", n, len(files), err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "css", "glow.css")); string(got) != "h1 { color: teal }\n" {
		t.Errorf("checked out css/glow.css: %q", got)
	}
	if _, _, _, err := s.Checkout(ctx, "a1", 1, dir); err == nil || !strings.Contains(err.Error(), "has files in it already") {
		t.Errorf("checking out over a folder with files: %v", err)
	}
	// And the folder, worked on, is the next version: hidden files left out.
	os.WriteFile(filepath.Join(dir, "css", "glow.css"), []byte("h1 { color: navy }\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("x"), 0o644)
	worked, err := ReadFolder(dir)
	if err != nil || len(worked) != 6 {
		t.Fatalf("read the folder: %d files %v", len(worked), err)
	}
	if _, n, err := s.AddVersion(ctx, "a1", Revision{Files: worked, At: day.Add(2 * time.Hour), Version: Version{By: "you"}}); err != nil || n != 4 {
		t.Fatalf("v4: %d %v", n, err)
	}
	if got, _, _, _, _ := s.ReadFile(ctx, "a1", 4, "css/glow.css"); string(got) != "h1 { color: navy }\n" {
		t.Errorf("v4's css/glow.css: %q", got)
	}
	if got, _, _, _, _ := s.ReadFile(ctx, "a1", 1, "css/glow.css"); string(got) != sheet {
		t.Errorf("v1's css/glow.css changed: %q", got)
	}
}

// What a version can't keep is refused before anything's written.
func TestFilesRefused(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	index := File{Path: IndexFile, Data: []byte(page)}
	for _, tc := range []struct {
		files Files
		want  string
	}{
		{Files{{Path: "page.html", Data: []byte(page)}}, "has no index.html"},
		{Files{index, {Path: "../out.css"}}, "leads out"},
		{Files{index, {Path: "/etc/passwd"}}, "starts at the root"},
		{Files{index, {Path: ".git/config"}}, "hidden"},
		{Files{index, {Path: "a\\b.css"}}, "forward slashes"},
		{Files{index, {Path: "artifact.toml"}}, "record's name"},
		{Files{index, {Path: "Logo.png"}, {Path: "logo.png"}}, "but for their case"},
		{Files{index, {Path: "img"}, {Path: "img/a.png"}}, "a folder too"},
		{Files{index, {Path: "big.bin", Data: make([]byte, MaxSize)}}, "at most 10 MB"},
	} {
		if _, err := s.Add(ctx, New{Title: "No", Files: tc.files, At: day}); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("adding %v: %v, want %q", tc.files.Infos(), err, tc.want)
		}
	}
	if list, _ := s.List(ctx); len(list) != 0 {
		t.Errorf("something was added: %+v", list)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, IndexFile), []byte(page), 0o644)
	os.Symlink("/etc/hosts", filepath.Join(dir, "hosts"))
	if _, err := ReadFolder(dir); err == nil || !strings.Contains(err.Error(), "is a link") {
		t.Errorf("a folder with a link in: %v", err)
	}
}

// Checking says what a page loads that isn't among its files, and what
// the other files reach for that the frame won't give.
func TestCheckFiles(t *testing.T) {
	c, err := Check(site())
	if err != nil || len(c.Warnings) != 0 || c.Title != "Glow" || len(c.Files) != 5 {
		t.Fatalf("a page with its files: %+v %v", c.Warnings, err)
	}
	c, err = Check(Files{
		{Path: IndexFile, Data: []byte("<!doctype html><title>T</title>\n<link rel=stylesheet href=\"style.css?v=2\">\n<script src='./js/app.js'></script>\n<img src=\"/logo.png\">\n<img data-src=\"lazy.png\" src=\"data:image/png;base64,AA\">\n<a href=\"about.html\">a link loads nothing</a>\n<img src=\"../up.png\">")},
		{Path: "js/app.js", Data: []byte("fetch('https://api.example.com/x'); new WebSocket('wss://x');")},
		{Path: "theme.css", Data: []byte("@import url(https://fonts.example.com/a.css);")},
		{Path: "about.html", Data: []byte("<!doctype html><script src=\"gone.js\"></script>")},
	})
	if err != nil {
		t.Fatal(err)
	}
	said := strings.Join(c.Warnings, "\n")
	for _, want := range []string{
		"its <link> loads style.css?v=2, which is missing from the artifact's files",
		"its <img> loads /logo.png, an absolute path",
		"its <img> loads ../up.png, which is outside the artifact",
		"js/app.js fetches https://api.example.com/x",
		"js/app.js calls WebSocket",
		"theme.css loads https://fonts.example.com/a.css",
		"about.html's <script> loads gone.js, which is missing from the artifact's files",
	} {
		if !strings.Contains(said, want) {
			t.Errorf("warnings don't say %q:\n%s", want, said)
		}
	}
	if len(c.Warnings) != 7 || strings.Contains(said, "lazy.png") || strings.Contains(said, "js/app.js,") {
		t.Errorf("warnings:\n%s", said)
	}
}

// An artifact from when a version was a file in its folder is read as it
// is, and put in a repo of its own by the sweep, or by its next version.
func TestConvert(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	old := func(name, title string) string {
		dir := filepath.Join(s.Dir(), name)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, VersionFile(1)), []byte(page), 0o644)
		os.WriteFile(filepath.Join(dir, VersionFile(2)), []byte(page+"<p>2</p>"), 0o644)
		os.WriteFile(filepath.Join(dir, RecordFile), []byte("title = \""+title+"\"\nmade = 2026-11-01T10:00:00Z\n\n[[versions]]\nn = 1\nat = 2026-11-01T10:00:00Z\nby = \"you\"\n\n[[versions]]\nn = 2\nbased_on = 1\nat = 2026-11-02T10:00:00Z\nby = \"c1\"\nnote = \"Two.\"\n"), 0o644)
		return dir
	}
	one, two := old("2026-11-01-one", "One"), old("2026-11-01-two", "Two")
	// Read as it is, before anything's converted.
	if text, _, n, err := s.ReadVersion(ctx, "a1", 0); err != nil || n != 2 || !strings.Contains(string(text), "<p>2</p>") {
		t.Fatalf("an old one's latest: v%d %q %v", n, text, err)
	}
	e, _ := s.Get(ctx, "a1")
	if files, size, ok := s.VersionSize(e, 1); !ok || files != 1 || size != len(page) {
		t.Errorf("an old one's v1: %d files, %d bytes, %v", files, size, ok)
	}
	// Its next version converts it first.
	_, n, err := s.AddVersion(ctx, "a1", Revision{Files: site(), At: day, Version: Version{By: "you"}})
	if err != nil || n != 3 {
		t.Fatalf("v3 of an old one: %d %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(one, VersionFile(1))); !os.IsNotExist(err) {
		t.Errorf("v1.html is still there: %v", err)
	}
	if text, _, _, err := s.ReadVersion(ctx, "a1", 1); err != nil || string(text) != page {
		t.Errorf("v1 after: %q %v", text, err)
	}
	// The sweep converts the rest, and says what each version is.
	done, err := s.Convert(ctx)
	if err != nil || len(done) != 1 || done[0].ID != "a2" {
		t.Fatalf("converted %+v %v", done, err)
	}
	if v := done[0].Versions[1]; v.Files != 1 || v.Size != len(page+"<p>2</p>") {
		t.Errorf("a2's v2 says %d files, %d bytes", v.Files, v.Size)
	}
	if text, _, _, err := s.ReadVersion(ctx, "a2", 2); err != nil || !strings.Contains(string(text), "<p>2</p>") {
		t.Errorf("a2's v2 after: %q %v", text, err)
	}
	if _, err := os.Stat(filepath.Join(two, VersionFile(2))); !os.IsNotExist(err) {
		t.Errorf("a2's v2.html is still there: %v", err)
	}
	if again, err := s.Convert(ctx); err != nil || len(again) != 0 {
		t.Errorf("converted again: %+v %v", again, err)
	}
}
