package nav

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/testgit"
)

func TestMain(m *testing.M) {
	cleanup := testgit.NoSigning()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// repoWith is a repo with one commit of files, and that commit.
func repoWith(t *testing.T, files map[string]string) (dir, rev string) {
	t.Helper()
	dir = t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	run(t, dir, "config", "user.email", "t@example.com")
	run(t, dir, "config", "user.name", "T")
	for p, text := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "files")
	return dir, run(t, dir, "rev-parse", "HEAD")
}

// named is a line's names as name:part, for comparing.
func named(src string, line int, names []Named) string {
	text := []rune(strings.Split(src, "\n")[line-1])
	var units []rune // UTF-16 units, as runes (none here are astral)
	units = append(units, text...)
	var out []string
	for _, n := range names {
		out = append(out, string(units[n.From:n.To])+":"+string(n.Part))
	}
	return strings.Join(out, " ")
}

const mainGo = `package main

import (
	"fmt"
	str "strings"
)

type Greeter struct{ name string }

func (g *Greeter) Greet() string {
	return "héllo " + str.ToUpper(g.name)
}

func main() {
	g := &Greeter{name: "ws"}
	fmt.Println(g.Greet())
}
`

func TestNames_EveryNameOnItsLineWithItsPart(t *testing.T) {
	dir, rev := repoWith(t, map[string]string{"main.go": mainGo, "README.md": "# hi\n"})
	s := New(t.TempDir())
	n, err := s.Names(dir, rev, "main.go")
	if err != nil {
		t.Fatal(err)
	}
	if n.Lang != "go" || n.Why != "" {
		t.Fatalf("lang %q, why %q", n.Lang, n.Why)
	}
	want := map[int]string{
		4:  "fmt:i",
		5:  "str:i strings:i",
		8:  "Greeter:d name:d string:r",
		10: "g:d Greeter:r Greet:d string:r",
		// After a multi-byte é, columns still count UTF-16 units.
		11: "str:r ToUpper:m g:r name:m",
		14: "main:d",
		15: "g:d Greeter:r name:m",
		16: "fmt:r Println:m g:r Greet:m",
	}
	for line, w := range want {
		if got := named(mainGo, line, n.Lines[line]); got != w {
			t.Errorf("line %d: got %q, want %q", line, got, w)
		}
	}
}

func TestNames_FoldsAreDeclarationsAndScopesOfMoreThanOneLine(t *testing.T) {
	dir, rev := repoWith(t, map[string]string{"main.go": mainGo, "one.go": "package main\n\nfunc one() int { return 1 }\n"})
	s := New(t.TempDir())
	n, err := s.Names(dir, rev, "main.go")
	if err != nil {
		t.Fatal(err)
	}
	// The imports aren't a scope, and the struct is on one line: the
	// method and main are what fold.
	if got, want := fmt.Sprint(n.Folds), "[[10 12] [14 17]]"; got != want {
		t.Errorf("folds: got %s, want %s", got, want)
	}
	// As the web UI reads them: pairs of lines.
	if b, _ := json.Marshal(n); !strings.Contains(string(b), `"folds":[[10,12],[14,17]]`) {
		t.Errorf("folds as JSON: %s", b)
	}
	one, err := s.Names(dir, rev, "one.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(one.Folds) != 0 {
		t.Errorf("a file of one-line declarations folds nowhere: got %v", one.Folds)
	}
}

func TestNames_AFileWithNoNavigationSaysWhy(t *testing.T) {
	dir, rev := repoWith(t, map[string]string{"main.go": mainGo, "README.md": "# hi\n"})
	n, err := New("").Names(dir, rev, "README.md")
	if err != nil {
		t.Fatal(err)
	}
	if n.Why != "No navigation for .md files" || len(n.Lines) != 0 {
		t.Fatalf("got %+v", n)
	}
}

func TestNames_ReadsABareCloneAtACommit(t *testing.T) {
	src, rev := repoWith(t, map[string]string{"main.go": mainGo})
	bare := filepath.Join(t.TempDir(), "bare.git")
	run(t, "", "clone", "-q", "--bare", src, bare)
	n, err := New("").Names(bare, rev, "main.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := named(mainGo, 14, n.Lines[14]); got != "main:d" {
		t.Fatalf("line 14: %q", got)
	}
	if _, err := New("").Names(bare, rev, "nope.go"); err == nil {
		t.Fatal("a file that isn't there: no error")
	}
}

func TestFacts_KeptOnDiskByBlobAndReadBack(t *testing.T) {
	dir, rev := repoWith(t, map[string]string{"a/main.go": mainGo})
	cache := t.TempDir()
	tree, err := New(cache).Tree(dir, rev)
	if err != nil {
		t.Fatal(err)
	}
	first, err := New(cache).Facts(tree, "a/main.go")
	if err != nil {
		t.Fatal(err)
	}
	// A new service, with a runtime that can't parse: what's read is
	// what was kept.
	again := New(cache)
	again.Runtime = keptOnly{}
	f, err := again.Facts(tree, "a/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Defs) != len(first.Defs) || f.Package != "main" {
		t.Fatalf("read back %d defs, package %q; made %d", len(f.Defs), f.Package, len(first.Defs))
	}
}

type keptOnly struct{}

func (keptOnly) Has(string) bool { return true }
func (keptOnly) Captures(string, string, []byte) ([]Match, bool, error) {
	return nil, false, os.ErrInvalid
}

func TestBuild_ScopesContainersAndWrittenTypes(t *testing.T) {
	src := []byte(mainGo)
	l := LangOf("main.go")
	m, broken, err := Builtin().Captures("go", l.Query("go"), src)
	if err != nil {
		t.Fatal(err)
	}
	f := Build("go", src, m, broken)
	def := func(name, kind string) Def {
		for _, d := range f.Defs {
			if d.Name == name && d.Kind == kind {
				return d
			}
		}
		t.Fatalf("no %s %s", kind, name)
		return Def{}
	}
	if d := def("name", "field"); d.Container < 0 || f.Defs[d.Container].Name != "Greeter" || d.Typed.Text != "string" {
		t.Errorf("field name: %+v", d)
	}
	if d := def("Greet", "method"); d.Receiver.Text != "*Greeter" || d.Typed.Text != "string" || d.Scope != 0 {
		t.Errorf("method Greet: %+v", d)
	}
	g := def("g", "var")
	if g.New.Text != "Greeter" || f.Scopes[g.Scope].Kind != "block" {
		t.Errorf("var g: %+v in %s", g, f.Scopes[g.Scope].Kind)
	}
	if p := def("g", "param"); p.Typed.Text != "*Greeter" || f.Scopes[p.Scope].Kind != "function" {
		t.Errorf("param g: %+v", p)
	}
	if len(f.Imports) != 2 || f.Imports[1].Alias != "str" || f.Imports[1].Path != "strings" {
		t.Errorf("imports: %+v", f.Imports)
	}
}
