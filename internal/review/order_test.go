package review

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOrder(t *testing.T) {
	paths := []string{
		"go.sum",
		"README.md",
		"src/a-b/z.ts",
		"src/a/b/deep.ts",
		"src/a/cache.test.ts",
		"src/a/cache.ts",
		"src/a/__snapshots__/cache.test.ts.snap",
		"src/a/api.pb.go",
		"internal/x/store_test.go",
		"internal/x/store.go",
		"internal/x/storekey.go",
		"py/test_load.py",
		"py/load.py",
		"moved/new.go",
	}
	var files []File
	for _, p := range paths {
		f := File{Path: p, Status: "modified", Hunks: []Hunk{{}}}
		if p == "moved/new.go" {
			f.Status, f.OldPath, f.Hunks = "renamed", "old.go", []Hunk{}
		}
		files = append(files, f)
	}
	// No repo to ask about attributes: names alone.
	order(t.TempDir(), "HEAD", files)
	var got []string
	skim := map[string]string{}
	for _, f := range files {
		got = append(got, f.Path)
		if f.Skim != "" {
			skim[f.Path] = f.Skim
		}
	}
	want := []string{
		"README.md",
		"internal/x/store.go",
		"internal/x/store_test.go",
		"internal/x/storekey.go",
		"py/load.py",
		"py/test_load.py",
		"src/a/cache.ts",
		"src/a/cache.test.ts",
		"src/a/b/deep.ts",
		"src/a-b/z.ts",
		// To skim, last.
		"go.sum",
		"moved/new.go",
		"src/a/api.pb.go",
		"src/a/__snapshots__/cache.test.ts.snap",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order:\n got %q\nwant %q", got, want)
	}
	wantSkim := map[string]string{
		"go.sum":                                 SkimLockfile,
		"moved/new.go":                           SkimRenamed,
		"src/a/api.pb.go":                        SkimGenerated,
		"src/a/__snapshots__/cache.test.ts.snap": SkimSnapshot,
	}
	if !reflect.DeepEqual(skim, wantSkim) {
		t.Errorf("skim: got %v, want %v", skim, wantSkim)
	}
}

func TestTestOf(t *testing.T) {
	for _, c := range []struct {
		base, of string
		test     bool
	}{
		{"cache_test.go", "cache.go", true},
		{"x.test.tsx", "x.tsx", true},
		{"x.spec.ts", "x.ts", true},
		{"test_load.py", "load.py", true},
		{"FooTest.java", "Foo.java", true},
		{"cache.go", "cache.go", false},
		{"_test.go", "_test.go", false},
		{"Test.java", "Test.java", false},
	} {
		if of, test := testOf(c.base); of != c.of || test != c.test {
			t.Errorf("testOf(%q) = %q, %v; want %q, %v", c.base, of, test, c.of, c.test)
		}
	}
}

// A repo says which of its files are generated in .gitattributes, read as
// it stands at the head compared: no working tree needed.
func TestOrderReadsAttributes(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "-q", "--initial-branch=main")
	run(t, dir, "config", "user.name", "Test")
	run(t, dir, "config", "user.email", "test@example.com")
	write(t, dir, "a.go", "package a\n")
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "base")
	base := run(t, dir, "rev-parse", "HEAD")
	write(t, dir, ".gitattributes", "client/** linguist-generated\n")
	write(t, dir, "client/api.ts", "export const a = 1;\n")
	write(t, dir, "z.go", "package a\n")
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "change")
	head := run(t, dir, "rev-parse", "HEAD")
	// The working tree goes back: only the commit says so.
	run(t, dir, "checkout", "-q", base)

	d, err := CompareCommits(dir, "main", base, head)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range d.Files {
		got = append(got, f.Path+":"+f.Skim)
	}
	want := []string{".gitattributes:", "z.go:", "client/api.ts:generated"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The files a change touches that the repo says are generated, and the
// diff an agent is pointed to, which leaves them out: from a bare clone,
// with no working tree to read attributes from.
func TestGeneratedIn(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "-q", "--initial-branch=main")
	run(t, dir, "config", "user.name", "Test")
	run(t, dir, "config", "user.email", "test@example.com")
	write(t, dir, "a.go", "package a\n")
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "base")
	base := run(t, dir, "rev-parse", "HEAD")
	write(t, dir, ".gitattributes", "client/** linguist-generated\nweb/*.gen.ts linguist-generated\n")
	write(t, dir, "client/api.ts", "export const a = 1;\n")
	write(t, dir, "client/b/types.ts", "export type B = 1;\n")
	write(t, dir, "web/api.gen.ts", "export const g = 1;\n")
	write(t, dir, "web/hand.ts", "export const h = 1;\n")
	write(t, dir, "z.go", "package a\n")
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "change")
	head := run(t, dir, "rev-parse", "HEAD")
	bare := filepath.Join(t.TempDir(), "bare.git")
	run(t, dir, "clone", "-q", "--bare", dir, bare)

	g := GeneratedIn(bare, base, head)
	if want := []string{"client/api.ts", "client/b/types.ts", "web/api.gen.ts"}; !reflect.DeepEqual(g.Files, want) {
		t.Errorf("Files = %q, want %q", g.Files, want)
	}
	// client/ is generated whole; web/ isn't, so its file is named.
	if want := []string{":(exclude)client/", ":(exclude)web/api.gen.ts"}; !reflect.DeepEqual(g.Pathspecs, want) {
		t.Errorf("Pathspecs = %q, want %q", g.Pathspecs, want)
	}
	shown := strings.Fields(run(t, bare, append([]string{"diff", "--name-only", base, head, "--", "."}, g.Pathspecs...)...))
	if want := []string{".gitattributes", "web/hand.ts", "z.go"}; !reflect.DeepEqual(shown, want) {
		t.Errorf("the diff past what's generated shows %q, want %q", shown, want)
	}
	if got := GeneratedIn(bare, base, base); len(got.Files)+len(got.Pathspecs) != 0 {
		t.Errorf("no change, but GeneratedIn = %+v", got)
	}
}
