package nav

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Fixtures: testdata/<lang>/<case>/ is a small repo. A comment line with a
// caret says what the name above the caret leads to:
//
//	head, err := git.ReadLine(ctx, dir)
//	//               ^ resolved internal/git/git.go:ReadLine "imported from internal/git"
//
// A target is path:Name (the first definition of Name in that file),
// :Name for the same file, or path for a module (its package line). A
// caret is counted from the comment's indent, as if it were the code's:
// under a line ending in {, gofmt indents it deeper. A name at the start
// of a line, where no caret fits, is pointed at with <-. The
// answer is resolved, candidates <n>, external <module>, builtin or
// unresolved; none is that there's no name there. The words after it in
// quotes, if any, are how.

var expectation = regexp.MustCompile(`^\s*(?://|#)\s*(\^|<-)\s+(resolved|candidates|external|builtin|unresolved|none)\b(.*)$`)

type expect struct {
	path    string
	line    int // of the name, from 1
	col     int // UTF-16 units
	answer  string
	args    []string
	how     string
	written string
}

func readExpectations(t *testing.T, root string) []expect {
	t.Helper()
	var out []expect
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || LangOf(p) == nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(src), "\n")
		code := 0 // the last line that isn't an expectation
		for i, line := range lines {
			m := expectation.FindStringSubmatchIndex(line)
			if m == nil {
				code = i
				continue
			}
			// A caret is counted from where its comment's indent ends, and
			// put that far from where the code's ends: gofmt indents a
			// comment after a { one deeper than the line it's under.
			indent := func(l string) int { return len(l) - len(strings.TrimLeft(l, " \t")) }
			caret := indent(lines[code]) + m[2] - indent(line)
			if line[m[2]:m[3]] == "<-" {
				caret = indent(lines[code]) // <-: where the line's code starts
			}
			rest := strings.TrimSpace(line[m[6]:m[7]])
			e := expect{path: rel, line: code + 1, answer: line[m[4]:m[5]], written: strings.TrimSpace(line)}
			if q := strings.Index(rest, `"`); q >= 0 {
				e.how = strings.Trim(rest[q:], `"`)
				rest = strings.TrimSpace(rest[:q])
			}
			e.args = strings.Fields(rest)
			if caret < 0 || caret > len(lines[code]) {
				t.Fatalf("%s:%d: the caret is outside line %d", rel, i+1, code+1)
			}
			e.col = utf16Len([]byte(lines[code][:caret]))
			out = append(out, e)
		}
		return nil
	})
	return out
}

// commitFolder makes a repo of a folder's files, and its commit.
func commitFolder(t *testing.T, folder string) (string, string) {
	t.Helper()
	files := map[string]string{}
	filepath.WalkDir(folder, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(folder, p)
		files[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	return repoWith(t, files)
}

func TestFixtures(t *testing.T) {
	langDirs, _ := os.ReadDir("testdata")
	for _, ld := range langDirs {
		if !ld.IsDir() {
			continue
		}
		cases, _ := os.ReadDir(filepath.Join("testdata", ld.Name()))
		for _, c := range cases {
			if !c.IsDir() {
				continue
			}
			folder := filepath.Join("testdata", ld.Name(), c.Name())
			t.Run(ld.Name()+"/"+c.Name(), func(t *testing.T) {
				exps := readExpectations(t, folder)
				if len(exps) == 0 {
					t.Fatal("no expectations")
				}
				dir, rev := commitFolder(t, folder)
				s := New("")
				for _, e := range exps {
					a, err := s.Definition(dir, rev, e.path, e.line, e.col)
					where := fmt.Sprintf("%s:%d:%d (%s)", e.path, e.line, e.col, e.written)
					if e.answer == "none" {
						if !errors.Is(err, ErrNoName) {
							t.Errorf("%s: a name, %v", where, err)
						}
						continue
					}
					if err != nil {
						t.Errorf("%s: %v", where, err)
						continue
					}
					if msg := check(t, s, dir, rev, e, a); msg != "" {
						t.Errorf("%s\n\t%s\n\tgot %s", where, msg, describeAnswer(a))
					}
				}
			})
		}
	}
}

func check(t *testing.T, s *Service, dir, rev string, e expect, a *Answer) string {
	if a.Answer != e.answer {
		return "answer is " + a.Answer
	}
	if e.how != "" && a.How != e.how {
		return fmt.Sprintf("how is %q", a.How)
	}
	switch e.answer {
	case "candidates":
		if len(e.args) > 0 {
			if n, _ := strconv.Atoi(e.args[0]); n != len(a.Targets) {
				return fmt.Sprintf("%d candidates", len(a.Targets))
			}
		}
		// The rest: the first candidates, in order.
		for i, want := range e.args[min(1, len(e.args)):] {
			if i >= len(a.Targets) || !isTarget(t, s, dir, rev, e.path, want, a.Targets[i]) {
				return "candidate " + strconv.Itoa(i+1) + " isn't " + want
			}
		}
	case "external":
		if len(e.args) > 0 && (a.External == nil || a.External.Module != e.args[0]) {
			return "not external " + e.args[0]
		}
	case "resolved":
		if len(e.args) != len(a.Targets) {
			return fmt.Sprintf("%d targets", len(a.Targets))
		}
		for _, want := range e.args {
			found := false
			for _, got := range a.Targets {
				found = found || isTarget(t, s, dir, rev, e.path, want, got)
			}
			if !found {
				return "not at " + want
			}
		}
	}
	return ""
}

// isTarget is whether a target is the one written: path:Name, :Name, or
// a path alone for a module.
func isTarget(t *testing.T, s *Service, dir, rev, from, want string, got Target) bool {
	p, name, hasName := strings.Cut(want, ":")
	if p == "" {
		p = from
	}
	if got.Path != p {
		return false
	}
	if !hasName {
		return got.Kind == "package" || got.Kind == "module"
	}
	tree, _ := s.Tree(dir, rev)
	f, err := s.Facts(tree, p)
	if err != nil {
		return false
	}
	for _, i := range f.Named(name) {
		line, _ := f.Line(f.Defs[i].At.Start)
		if line == got.Line {
			return true
		}
	}
	return false
}

func describeAnswer(a *Answer) string {
	var ts []string
	for _, t := range a.Targets {
		ts = append(ts, fmt.Sprintf("%s:%d(%s %s)", t.Path, t.Line, t.Kind, t.Container))
	}
	sort.Strings(ts)
	ext := ""
	if a.External != nil {
		ext = fmt.Sprintf(" external %+v", *a.External)
	}
	return fmt.Sprintf("%s %q [%s]%s", a.Answer, a.How, strings.Join(ts, " "), ext)
}
