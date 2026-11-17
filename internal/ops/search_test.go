package ops

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/workspace"
)

// searchFixture is a workspace of two repos, frontend (alias fe) with a
// capsule login-fix, and backend (alias api), as plain folders: a file in
// each worktree with "needle" in it, and .silo and .bare that must never
// be searched unless named.
func searchFixture(t *testing.T) *Workspace {
	t.Helper()
	root := t.TempDir()
	ws := &workspace.Workspace{
		Root:      root,
		RepoNames: []string{"backend", "frontend"},
		AliasMap:  map[string]string{"fe": "frontend", "api": "backend"},
	}
	write := func(rel, content string) {
		p := filepath.Join(root, "repos", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("frontend/.ground/src/app.ts", "const a = 1\nneedle in frontend ground\n")
	write("frontend/.ground/lib/src/deep.ts", "needle in frontend lib/src\n")
	write("frontend/login-fix/src/app.ts", "needle in frontend capsule\n")
	write("frontend/login-fix/src/other.ts", "needle in an unchanged capsule file\n")
	write("frontend/.silo/src/app.ts", "needle in silo\n")
	write("frontend/.bare/hooks/note.txt", "needle in bare\n")
	write("backend/.ground/src/main.go", "package main // needle and needle\n")
	return New(&config.Config{}, ws, nil, Options{})
}

func relPaths(roots []workspace.SearchRoot) []string {
	out := make([]string, len(roots))
	for i, r := range roots {
		out[i] = r.RelPath()
	}
	return out
}

func wholeRoots(roots []workspace.SearchRoot) []SearchRoot {
	out := make([]SearchRoot, len(roots))
	for i, r := range roots {
		out[i] = SearchRoot{SearchRoot: r}
	}
	return out
}

// Scope: where you stand, what you name, and the mode pick each repo's
// worktree; .silo and .bare only when named; none is refused.
func TestSearchRoots(t *testing.T) {
	w := searchFixture(t)
	root := w.WS().Root
	capsule := filepath.Join(root, "repos", "frontend", "login-fix", "src")
	tests := []struct {
		name  string
		scope SearchScope
		want  []string
	}{
		{"outside any worktree: ground everywhere",
			SearchScope{Cwd: root},
			[]string{"repos/backend/.ground", "repos/frontend/.ground"}},
		{"in a capsule: it for its repo, ground for the rest",
			SearchScope{Cwd: capsule},
			[]string{"repos/backend/.ground", "repos/frontend/login-fix"}},
		{"ground mode ignores where you stand",
			SearchScope{Cwd: capsule, Mode: workspace.ScopeGround},
			[]string{"repos/backend/.ground", "repos/frontend/.ground"}},
		{"all: ground then capsules",
			SearchScope{Cwd: root, Mode: workspace.ScopeAll, Selectors: []workspace.RepoSelector{{Repo: "frontend"}}},
			[]string{"repos/frontend/.ground", "repos/frontend/login-fix"}},
		{"a named worktree, even silo",
			SearchScope{Cwd: root, Selectors: []workspace.RepoSelector{{Repo: "frontend", Worktree: ".silo"}}},
			[]string{"repos/frontend/.silo"}},
		{"in silo counts as nowhere",
			SearchScope{Cwd: filepath.Join(root, "repos", "frontend", ".silo")},
			[]string{"repos/backend/.ground", "repos/frontend/.ground"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roots, err := w.SearchRoots(tt.scope)
			if err != nil {
				t.Fatal(err)
			}
			if got := relPaths(roots); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("roots = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("no worktrees is refused", func(t *testing.T) {
		empty := New(&config.Config{}, &workspace.Workspace{Root: t.TempDir(), RepoNames: []string{"x"}}, nil, Options{})
		_, err := empty.SearchRoots(SearchScope{Cwd: empty.WS().Root})
		refusedWith(t, err, "nothing-to-search")
		if err == nil || err.Error() != "nothing to search: no worktrees found" {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("unknown repo", func(t *testing.T) {
		_, err := w.SearchRoots(SearchScope{Selectors: []workspace.RepoSelector{{Repo: "nope"}}})
		if err == nil || !strings.Contains(err.Error(), `unknown repo "nope"`) {
			t.Errorf("err = %v", err)
		}
	})
}

// Ripgrep's arguments: anchored globs, then each root (or its files),
// physically spelled, run from the workspace root.
func TestRgArgs(t *testing.T) {
	wsRoot := t.TempDir()
	real, _ := filepath.EvalSymlinks(wsRoot)
	ground := workspace.SearchRoot{Repo: "fe", Worktree: ".ground", Path: filepath.Join(wsRoot, "repos", "fe", ".ground")}
	capsule := workspace.SearchRoot{Repo: "fe", Worktree: "fix", Path: filepath.Join(wsRoot, "repos", "fe", "fix")}
	other := workspace.SearchRoot{Repo: "be", Worktree: ".ground", Path: filepath.Join(wsRoot, "repos", "be", ".ground")}
	for _, r := range []workspace.SearchRoot{ground, capsule, other} {
		os.MkdirAll(r.Path, 0o755)
	}
	dir, argv := RgArgs(wsRoot, []SearchRoot{
		{SearchRoot: ground},
		{SearchRoot: capsule, Files: []string{"src/a.ts", "b.ts"}},
		{SearchRoot: other, Files: []string{}},
	}, []string{"-g", "src/*.ts", "needle"})
	if dir != real {
		t.Errorf("dir = %q, want %q", dir, real)
	}
	want := []string{
		"-g", "repos/fe/.ground/src/*.ts", "-g", "repos/fe/fix/src/*.ts", "needle",
		filepath.Join(real, "repos", "fe", ".ground"),
		filepath.Join(real, "repos", "fe", "fix", "src", "a.ts"),
		filepath.Join(real, "repos", "fe", "fix", "b.ts"),
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("argv\n got %q\nwant %q", argv, want)
	}
}

// Glob anchoring per root: a slash glob becomes one per root, the rest
// stay as they are.
func TestAnchorGlobs(t *testing.T) {
	prefixes := []string{"repos/fe/.ground", "repos/api/.ground"}
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "basename glob untouched",
			args: []string{"-g", "*.ts", "needle"},
			want: []string{"-g", "*.ts", "needle"},
		},
		{
			name: "negated basename glob untouched",
			args: []string{"-g", "!*.snap", "needle"},
			want: []string{"-g", "!*.snap", "needle"},
		},
		{
			name: "slash glob expanded per root",
			args: []string{"-g", "src/*.ts", "needle"},
			want: []string{"-g", "repos/fe/.ground/src/*.ts", "-g", "repos/api/.ground/src/*.ts", "needle"},
		},
		{
			name: "negated slash glob keeps negation",
			args: []string{"-g", "!src/**", "needle"},
			want: []string{"-g", "!repos/fe/.ground/src/**", "-g", "!repos/api/.ground/src/**", "needle"},
		},
		{
			name: "leading slash anchor is absorbed",
			args: []string{"--glob", "/src/*.ts", "needle"},
			want: []string{"--glob", "repos/fe/.ground/src/*.ts", "--glob", "repos/api/.ground/src/*.ts", "needle"},
		},
		{
			name: "doublestar prefix preserved",
			args: []string{"-g", "**/src/*.ts"},
			want: []string{"-g", "repos/fe/.ground/**/src/*.ts", "-g", "repos/api/.ground/**/src/*.ts"},
		},
		{
			name: "attached short form",
			args: []string{"-gsrc/*.ts", "needle"},
			want: []string{"-grepos/fe/.ground/src/*.ts", "-grepos/api/.ground/src/*.ts", "needle"},
		},
		{
			name: "equals long forms",
			args: []string{"--glob=src/*.ts", "--iglob=LIB/*.TS"},
			want: []string{
				"--glob=repos/fe/.ground/src/*.ts", "--glob=repos/api/.ground/src/*.ts",
				"--iglob=repos/fe/.ground/LIB/*.TS", "--iglob=repos/api/.ground/LIB/*.TS",
			},
		},
		{
			name: "iglob separate value",
			args: []string{"--iglob", "SRC/*", "x"},
			want: []string{"--iglob", "repos/fe/.ground/SRC/*", "--iglob", "repos/api/.ground/SRC/*", "x"},
		},
		{
			name: "nothing rewritten after double dash",
			args: []string{"-g", "src/*", "--", "-g", "src/*"},
			want: []string{"-g", "repos/fe/.ground/src/*", "-g", "repos/api/.ground/src/*", "--", "-g", "src/*"},
		},
		{
			name: "dangling -g left alone",
			args: []string{"needle", "-g"},
			want: []string{"needle", "-g"},
		},
		{
			name: "other flags untouched",
			args: []string{"-i", "-t", "go", "-C", "2", "--json", "needle"},
			want: []string{"-i", "-t", "go", "-C", "2", "--json", "needle"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnchorGlobs(tt.args, prefixes)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AnchorGlobs(%q)\n got %q\nwant %q", tt.args, got, tt.want)
			}
		})
	}
}

// Parsing ripgrep's JSON: path, line, column, text and submatch ranges,
// from text or base64 bytes; the other messages skipped.
func TestParseRgJSON(t *testing.T) {
	out := strings.Join([]string{
		`{"type":"begin","data":{"path":{"text":"/w/repos/fe/.ground/src/a.ts"}}}`,
		`{"type":"context","data":{"path":{"text":"/w/repos/fe/.ground/src/a.ts"},"lines":{"text":"before\n"},"line_number":1,"absolute_offset":0,"submatches":[]}}`,
		`{"type":"match","data":{"path":{"text":"/w/repos/fe/.ground/src/a.ts"},"lines":{"text":"needle é needle\r\n"},"line_number":2,"absolute_offset":7,"submatches":[{"match":{"text":"needle"},"start":0,"end":6},{"match":{"text":"needle"},"start":10,"end":16}]}}`,
		`{"type":"end","data":{"path":{"text":"/w/repos/fe/.ground/src/a.ts"},"binary_offset":null,"stats":{}}}`,
		// "caf\xe9 x\n" and a path that isn't UTF-8 either: ripgrep gives bytes.
		`{"type":"match","data":{"path":{"bytes":"L3cvcmVwb3MvZmUvLmdyb3VuZC/pLnR4dA=="},"lines":{"bytes":"Y2Fm6SB4Cg=="},"line_number":9,"absolute_offset":0,"submatches":[{"match":{"text":"x"},"start":5,"end":6}]}}`,
		`{"type":"summary","data":{"stats":{}}}`,
	}, "\n")
	var got []rgMatch
	if err := parseRgJSON(strings.NewReader(out), func(m rgMatch) error { got = append(got, m); return nil }); err != nil {
		t.Fatal(err)
	}
	want := []rgMatch{
		{path: "/w/repos/fe/.ground/src/a.ts", Match: Match{Line: 2, Column: 1, Text: "needle é needle", Submatches: []Span{{0, 6}, {10, 16}}}},
		{path: "/w/repos/fe/.ground/\xe9.txt", Match: Match{Line: 9, Column: 6, Text: "caf\xe9 x", Submatches: []Span{{5, 6}}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}

	t.Run("not JSON says so", func(t *testing.T) {
		err := parseRgJSON(strings.NewReader("/w/repos/fe/.ground/src/a.ts\n"), func(rgMatch) error { return nil })
		if err == nil || !strings.Contains(err.Error(), "-l or -c") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("emit's error ends it", func(t *testing.T) {
		stop := errors.New("stop")
		n := 0
		err := parseRgJSON(strings.NewReader(out), func(rgMatch) error { n++; return stop })
		if err != stop || n != 1 {
			t.Errorf("err = %v after %d, want stop after 1", err, n)
		}
	})
}

// A match's root is the one its path is under, the deepest if one root
// is inside another; its path is relative to that root.
func TestRootAttributor(t *testing.T) {
	sep := string(filepath.Separator)
	outer := workspace.SearchRoot{Repo: "a", Worktree: ".ground", Path: sep + filepath.Join("w", "repos", "a", ".ground")}
	inner := workspace.SearchRoot{Repo: "a", Worktree: "nested", Path: filepath.Join(outer.Path, "nested")}
	a := newRootAttributor([]SearchRoot{{SearchRoot: outer}, {SearchRoot: inner}})
	for _, tt := range []struct {
		path, wantWT, wantRel string
		ok                    bool
	}{
		{filepath.Join(outer.Path, "src", "x.go"), ".ground", "src/x.go", true},
		{filepath.Join(inner.Path, "y.go"), "nested", "y.go", true},
		{outer.Path + "-other" + sep + "z.go", "", "", false},
		{sep + filepath.Join("elsewhere", "z.go"), "", "", false},
	} {
		root, rel, ok := a.attribute(tt.path)
		if ok != tt.ok || root.Worktree != tt.wantWT || rel != tt.wantRel {
			t.Errorf("attribute(%q) = %q %q %v, want %q %q %v", tt.path, root.Worktree, rel, ok, tt.wantWT, tt.wantRel, tt.ok)
		}
	}
}

func requireRg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep not installed")
	}
}

func searchAll(t *testing.T, w *Workspace, req SearchRequest) ([]Match, error) {
	t.Helper()
	var got []Match
	err := w.Search(context.Background(), req, func(m Match) error { got = append(got, m); return nil })
	slices.SortFunc(got, func(a, b Match) int {
		return strings.Compare(a.Root.RelPath()+"/"+a.Path, b.Root.RelPath()+"/"+b.Path)
	})
	return got, err
}

func matchKeys(ms []Match) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Root.Repo + "/" + m.Root.Worktree + ":" + m.Path
	}
	return out
}

// Search runs ripgrep over the roots and streams each match, attributed to
// its root, with slash globs anchored per root.
func TestSearch(t *testing.T) {
	requireRg(t)
	w := searchFixture(t)
	root := w.WS().Root
	scoped, err := w.SearchRoots(SearchScope{Cwd: filepath.Join(root, "repos", "frontend", "login-fix")})
	if err != nil {
		t.Fatal(err)
	}
	roots := wholeRoots(scoped)

	t.Run("one tree per repo, structured", func(t *testing.T) {
		got, err := searchAll(t, w, SearchRequest{Roots: roots, Patterns: []string{"needle"}})
		if err != nil {
			t.Fatal(err)
		}
		if keys := matchKeys(got); !reflect.DeepEqual(keys, []string{
			"backend/.ground:src/main.go", "frontend/login-fix:src/app.ts", "frontend/login-fix:src/other.ts",
		}) {
			t.Fatalf("matches = %q", keys)
		}
		m := got[0]
		if m.Root.Path != filepath.Join(root, "repos", "backend", ".ground") || m.Line != 1 || m.Column != 17 ||
			m.Text != "package main // needle and needle" || !reflect.DeepEqual(m.Submatches, []Span{{16, 22}, {27, 33}}) {
			t.Errorf("match = %+v", m)
		}
	})

	t.Run("slash glob anchored per root", func(t *testing.T) {
		all := wholeRoots([]workspace.SearchRoot{scoped[1], {Repo: "frontend", Worktree: ".ground", Path: filepath.Join(root, "repos", "frontend", ".ground")}})
		got, err := searchAll(t, w, SearchRequest{Roots: all, Patterns: []string{"needle"}, Options: []string{"-g", "src/*.ts"}})
		if err != nil {
			t.Fatal(err)
		}
		// src/ at the top of each root: not ground's lib/src/deep.ts.
		if keys := matchKeys(got); !reflect.DeepEqual(keys, []string{
			"frontend/.ground:src/app.ts", "frontend/login-fix:src/app.ts", "frontend/login-fix:src/other.ts",
		}) {
			t.Fatalf("matches = %q", keys)
		}
		if got[0].Line != 2 {
			t.Errorf("line = %d, want 2", got[0].Line)
		}
	})

	t.Run("only a root's files", func(t *testing.T) {
		some := []SearchRoot{{SearchRoot: scoped[1], Files: []string{"src/app.ts"}}, {SearchRoot: scoped[0], Files: []string{}}}
		got, err := searchAll(t, w, SearchRequest{Roots: some, Patterns: []string{"needle"}})
		if err != nil {
			t.Fatal(err)
		}
		if keys := matchKeys(got); !reflect.DeepEqual(keys, []string{"frontend/login-fix:src/app.ts"}) {
			t.Errorf("matches = %q", keys)
		}
	})

	t.Run("roots none of whose files are searched", func(t *testing.T) {
		none := []SearchRoot{{SearchRoot: scoped[0], Files: []string{}}, {SearchRoot: scoped[1], Files: []string{}}}
		got, err := searchAll(t, w, SearchRequest{Roots: none, Patterns: []string{"needle"}})
		if err != nil || len(got) != 0 {
			t.Errorf("got %q, err %v; want nothing (not the workspace root)", matchKeys(got), err)
		}
	})

	t.Run("several patterns and options", func(t *testing.T) {
		got, err := searchAll(t, w, SearchRequest{Roots: roots, Patterns: []string{"PACKAGE", "-nope-"}, Options: []string{"-i"}})
		if err != nil {
			t.Fatal(err)
		}
		if keys := matchKeys(got); !reflect.DeepEqual(keys, []string{"backend/.ground:src/main.go"}) {
			t.Errorf("matches = %q", keys)
		}
	})

	t.Run("no match is no error", func(t *testing.T) {
		got, err := searchAll(t, w, SearchRequest{Roots: roots, Patterns: []string{"definitely-not-here-zzz"}})
		if err != nil || len(got) != 0 {
			t.Errorf("got %d matches, err %v", len(got), err)
		}
	})

	t.Run("a bad pattern is ripgrep's error", func(t *testing.T) {
		_, err := searchAll(t, w, SearchRequest{Roots: roots, Patterns: []string{"("}})
		var exit *RgExit
		if !errors.As(err, &exit) || exit.Code != 2 || !strings.Contains(exit.Stderr, "regex") {
			t.Errorf("err = %v, want RgExit 2 saying why", err)
		}
	})

	t.Run("emit's error stops it", func(t *testing.T) {
		stop := errors.New("enough")
		n := 0
		err := w.Search(context.Background(), SearchRequest{Roots: roots, Patterns: []string{"needle"}}, func(Match) error { n++; return stop })
		if err != stop || n != 1 {
			t.Errorf("err = %v after %d, want enough after 1", err, n)
		}
	})

	t.Run("a cancelled context stops it", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := w.Search(ctx, SearchRequest{Roots: roots, Patterns: []string{"needle"}}, func(Match) error { return nil })
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want canceled", err)
		}
	})

	t.Run("no pattern", func(t *testing.T) {
		err := w.Search(context.Background(), SearchRequest{Roots: roots}, func(Match) error { return nil })
		var inv *Invalid
		if !errors.As(err, &inv) {
			t.Errorf("err = %v, want invalid", err)
		}
	})
}
