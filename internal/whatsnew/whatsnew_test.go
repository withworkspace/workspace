package whatsnew

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/testgit"
)

func TestMain(m *testing.M) {
	done := testgit.NoSigning()
	code := m.Run()
	done()
	os.Exit(code)
}

func TestParse_ReadsHeadAndBody(t *testing.T) {
	e, err := Parse("plugins", []byte("---\ntitle: Plugins are in Settings\nlink: /settings/plugins\npicture: plugins.png\n---\nAdd them from **Settings**.\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := Entry{Slug: "plugins", Title: "Plugins are in Settings", Link: "/settings/plugins", Picture: "plugins.png", Body: "Add them from **Settings**."}
	if e != want {
		t.Fatalf("got %+v, want %+v", e, want)
	}
}

func TestParse_RefusesWhatIsntAnEntry(t *testing.T) {
	for name, text := range map[string]string{
		"no head":      "Just words.\n",
		"no end":       "---\ntitle: x\n",
		"no title":     "---\nlink: /x\n---\nWords.\n",
		"no body":      "---\ntitle: x\n---\n\n",
		"unknown key":  "---\ntitle: x\nversion: v1.0.0\n---\nWords.\n",
		"long title":   "---\ntitle: " + strings.Repeat("a", TitleMax+1) + "\n---\nWords.\n",
		"long body":    "---\ntitle: x\n---\n" + strings.Repeat("a", BodyMax+1) + "\n",
		"outside link": "---\ntitle: x\nlink: https://example.com\n---\nWords.\n",
		"picture path": "---\ntitle: x\npicture: ../x.png\n---\nWords.\n",
		"not picture":  "---\ntitle: x\npicture: x.svg\n---\nWords.\n",
	} {
		if _, err := Parse("e", []byte(text)); err == nil {
			t.Errorf("%s: read as an entry", name)
		}
	}
}

func entries(versions ...string) []Entry {
	var out []Entry
	for i, v := range versions {
		out = append(out, Entry{Slug: string(rune('a' + i)), Version: v, Title: "t", Body: "b"})
	}
	return out
}

func TestArrange_NewestFirstKeepingTheLatestReleases(t *testing.T) {
	ix := Arrange(entries("v0.1.0", "", "v0.3.0", "v0.10.0", "v0.2.0", "v0.3.0"), 2)
	var got []string
	for _, e := range ix.Entries {
		got = append(got, e.Slug+"@"+e.Version)
	}
	if want := "b@ d@v0.10.0 c@v0.3.0 f@v0.3.0"; strings.Join(got, " ") != want {
		t.Fatalf("got %v, want %s", got, want)
	}
	if ix.Before != "v0.2.0" {
		t.Fatalf("before is %q, want v0.2.0", ix.Before)
	}
	if !ix.Missing("v0.1.0") || ix.Missing("v0.2.0") || ix.Missing("v0.3.0") {
		t.Fatal("missing should be only for what's seen before v0.2.0")
	}
	if all := Arrange(entries("v0.1.0"), 2); all.Before != "" || all.Missing("v0.0.1") {
		t.Fatal("nothing left out, so nothing's missing")
	}
}

func TestAfter_IsBetweenSeenAndRunning(t *testing.T) {
	ix := Arrange(entries("v0.1.0", "v0.2.0", "v0.3.0-rc.1", "v0.3.0", "v0.4.0", ""), Keep)
	var got []string
	for _, e := range ix.After("v0.2.0", "v0.3.0") {
		got = append(got, e.Version)
	}
	if want := "v0.3.0 v0.3.0-rc.1"; strings.Join(got, " ") != want {
		t.Fatalf("got %v, want %s", got, want)
	}
	if n := len(ix.After("v0.4.0", "v0.4.0")); n != 0 {
		t.Fatalf("%d new where nothing is", n)
	}
	if n := len(ix.UpTo("v0.2.0")); n != 2 {
		t.Fatalf("up to v0.2.0 is %d, want 2", n)
	}
}

func TestNotes_AreTheReleasesEntries(t *testing.T) {
	es := []Entry{
		{Slug: "a", Version: "v1.0.0", Title: "A thing", Body: "It does.", Picture: "a.png"},
		{Slug: "b", Version: "v0.9.0", Title: "Old", Body: "Before."},
	}
	got := Notes(es, "v1.0.0", "https://github.com/o/r")
	want := "## What's new\n\n### A thing\n\nIt does.\n\n![A thing](https://github.com/o/r/releases/download/v1.0.0/whatsnew-a.png)\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if Notes(es, "v1.1.0", "") != "" {
		t.Fatal("a release with no entries has no notes of them")
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, root, name, title string) {
	t.Helper()
	path := filepath.Join(root, EntriesDir, name+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\ntitle: "+title+"\n---\nWords.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFind_AnEntryIsInTheFirstReleaseWithIt(t *testing.T) {
	root := t.TempDir()
	run(t, root, "git", "init", "-q", "-b", "main")
	run(t, root, "git", "config", "user.email", "t@example.com")
	run(t, root, "git", "config", "user.name", "t")
	commit := func(msg string) {
		run(t, root, "git", "add", "-A")
		run(t, root, "git", "commit", "-q", "-m", msg)
	}
	write(t, root, "one", "One")
	commit("one")
	run(t, root, "git", "tag", "v0.1.0")
	write(t, root, "two", "Two")
	commit("two")
	run(t, root, "git", "tag", "v0.2.0-rc.1")
	run(t, root, "git", "tag", "v0.2.0")
	write(t, root, "one", "One, said again") // changed since: still v0.1.0's
	write(t, root, "three", "Three")
	commit("three")
	write(t, root, "four", "Four") // not committed

	found, err := Find(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range found {
		got[e.Slug] = e.Version
	}
	want := map[string]string{"one": "v0.1.0", "two": "v0.2.0-rc.1", "three": "", "four": ""}
	for slug, v := range want {
		if got[slug] != v {
			t.Errorf("%s is in %q, want %q", slug, got[slug], v)
		}
	}
}

func TestFind_RefusesAnEntryThatDoesntRead(t *testing.T) {
	root := t.TempDir()
	run(t, root, "git", "init", "-q")
	write(t, root, "bad", "")
	if _, err := Find(root); err == nil {
		t.Fatal("an entry with no title was found")
	}
}

func TestTheRepoEntriesRead(t *testing.T) {
	root, _ := filepath.Abs("../..")
	if _, err := Find(root); err != nil {
		t.Fatal(err)
	}
}
