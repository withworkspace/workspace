package nav

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// refsAt is who uses the definition named at a line of a fixture's file,
// as path:line, a maybe with a ?.
func refsAt(t *testing.T, s *Service, dir, rev, p string, line int, name string) []string {
	t.Helper()
	tree, _ := s.Tree(dir, rev)
	src, err := tree.Read(p)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Split(string(src), "\n")[line-1]
	col := strings.Index(text, name)
	if col < 0 {
		t.Fatalf("no %s on %s:%d", name, p, line)
	}
	r, err := s.References(dir, rev, p, line, utf16Len([]byte(text[:col])))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, u := range r.Uses {
		m := ""
		if u.Maybe {
			m = "?"
		}
		out = append(out, fmt.Sprintf("%s:%d%s", u.Path, u.Line, m))
	}
	sort.Strings(out)
	return out
}

func TestReferences_UsesThatLeadToTheDefinition(t *testing.T) {
	dir, rev := commitFolder(t, filepath.Join("testdata", "go", "members"))
	s := New("")
	cases := []struct {
		path string
		line int
		name string
		want string
	}{
		// Hello is Base's: called on a Server (embedded) and on a Base.
		{"types.go", 7, "Hello", "use.go:25 use.go:7"},
		// Start: on a value from NewServer, and on NewServer's result.
		{"types.go", 25, "Start", "use.go:14 use.go:16"},
		// The Store interface's Get: through a field and a method's result.
		{"types.go", 11, "Get", "use.go:22 use.go:4"},
		// A field, set in a literal and read.
		{"types.go", 18, "name", "types.go:22 use.go:18 use.go:20"},
		// From a use, not the definition: Start's uses all the same.
		{"use.go", 14, "Start", "use.go:14 use.go:16"},
	}
	for _, c := range cases {
		got := strings.Join(refsAt(t, s, dir, rev, c.path, c.line, c.name), " ")
		if got != c.want {
			t.Errorf("%s:%d %s: got %q, want %q", c.path, c.line, c.name, got, c.want)
		}
	}
}

func TestReferences_AnInterfacesMethodIsAMaybe(t *testing.T) {
	dir, rev := repoWith(t, map[string]string{
		"go.mod": "module example.com/x\n\ngo 1.23\n",
		"a.go": `package x

type Getter interface{ Get() string }

type Disk struct{}

func (Disk) Get() string { return "disk" }

func read(g Getter) string { return g.Get() }

func direct(d Disk) string { return d.Get() }
`,
	})
	s := New("")
	got := strings.Join(refsAt(t, s, dir, rev, "a.go", 7, "Get"), " ")
	if got != "a.go:11 a.go:9?" {
		t.Errorf("Disk.Get: %q", got)
	}
}
