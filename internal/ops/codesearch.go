package ops

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"unicode/utf8"

	"github.com/withworkspace/workspace/internal/find"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Code search, as the web UI asks it: Search over what's worth a hit, with
// caps, its matches ready for a page.
//
// Across the workspace (or one repo, its tag) it searches each repo's
// .ground and, of its capsules, only the files they've changed since they
// branched: nobody is taken to a capsule's unchanged files, which are
// main's. A file a capsule has (added or modified) takes the place of
// .ground's copy, so a path isn't listed twice; one it removed leaves
// .ground's. In one worktree (the explorer's) it searches all of it.

// Caps on what a code search answers, so a broad pattern can't flood a
// page: the defaults, and the most a request may ask for.
const (
	CodeSearchMatches    = 200
	CodeSearchMaxMatches = 1000
	CodeSearchBytes      = 256 << 10
	CodeSearchMaxBytes   = 1 << 20
	// codeLineBytes is the most of a line a hit carries: a longer one is
	// cut to a window around its first match.
	codeLineBytes = 400
)

// What a code search was capped by.
const (
	CappedMatches = "matches"
	CappedBytes   = "bytes"
)

// CodeSearchRequest is a code search: the patterns (any of which matches),
// where, and how they're read.
type CodeSearchRequest struct {
	Patterns []string
	// Repo narrows it to one repo: its .ground and its capsules' changes.
	Repo string
	// Worktree, with Repo, is a search of the whole of that worktree,
	// unchanged files too (the explorer's, in a capsule).
	Worktree string
	// Regex reads the patterns as regular expressions; otherwise they're
	// literal text.
	Regex bool
	// Case is "smart" (the default: insensitive unless a pattern has a
	// capital), "sensitive" or "ignore".
	Case string
	// MaxMatches and MaxBytes cap it (0 for the defaults, at most
	// CodeSearchMaxMatches and CodeSearchMaxBytes). Bytes are of the hits'
	// paths and text.
	MaxMatches, MaxBytes int
}

// CodeHit is a line a code search matched.
type CodeHit struct {
	Repo     string `json:"repo"`
	Worktree string `json:"worktree"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	// Column is the first match's 1-based byte column, as ws rg --column
	// says it.
	Column int `json:"column"`
	// Text is the line, as valid UTF-8 (a byte that isn't is U+FFFD), or
	// a window of it around its first match when it's long (Cut).
	Text string `json:"text"`
	Cut  bool   `json:"cut,omitempty"`
	// Ranges are where in Text the patterns matched, in UTF-16 code units,
	// as JavaScript indexes a string: [start, end).
	Ranges [][2]int `json:"ranges"`
}

// CodeSearchResult is how a code search ended: how many hits it answered
// and their bytes, and what it was capped by ("" for nothing: it answered
// every match).
type CodeSearchResult struct {
	Matches int    `json:"matches"`
	Bytes   int    `json:"bytes"`
	Capped  string `json:"capped,omitempty"`
}

var errCapped = errors.New("capped")

// CodeSearch searches as the request says, and calls emit with each hit as
// ripgrep finds it, until ripgrep's done, a cap is reached (which stops
// it), or ctx ends (which stops it too, and is its error). Ripgrep's own
// error (a bad pattern) is an *RgExit, after the hits it found.
func (w *Workspace) CodeSearch(ctx context.Context, req CodeSearchRequest, emit func(CodeHit) error) (CodeSearchResult, error) {
	var res CodeSearchResult
	if len(req.Patterns) == 0 {
		return res, invalid("nothing to search for")
	}
	if slices.Contains(req.Patterns, "") {
		return res, invalid("pattern is empty")
	}
	maxMatches := capOf(req.MaxMatches, CodeSearchMatches, CodeSearchMaxMatches)
	maxBytes := capOf(req.MaxBytes, CodeSearchBytes, CodeSearchMaxBytes)

	roots, replaced, err := w.codeSearchRoots(ctx, req)
	if err != nil {
		return res, err
	}
	if len(roots) == 0 {
		return res, nil
	}

	var options []string
	if !req.Regex {
		options = append(options, "--fixed-strings")
	}
	switch req.Case {
	case "", "smart":
		options = append(options, "--smart-case")
	case "sensitive":
		options = append(options, "--case-sensitive")
	case "ignore":
		options = append(options, "--ignore-case")
	default:
		return res, invalid("case is smart, sensitive or ignore, not %q", req.Case)
	}

	emitMatch := func(m Match) error {
		if m.Root.Worktree == workspace.GroundDir && replaced[m.Root.Repo][m.Path] {
			return nil
		}
		if res.Matches >= maxMatches {
			res.Capped = CappedMatches
			return errCapped
		}
		hit := codeHit(m)
		size := len(hit.Path) + len(hit.Text)
		if res.Bytes+size > maxBytes {
			res.Capped = CappedBytes
			return errCapped
		}
		if err := emit(hit); err != nil {
			return err
		}
		res.Matches++
		res.Bytes += size
		return nil
	}
	for _, batch := range batchRoots(roots, codeArgBytes) {
		err := w.Search(ctx, SearchRequest{Roots: batch, Patterns: req.Patterns, Options: options}, emitMatch)
		if errors.Is(err, errCapped) {
			return res, nil
		}
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

// codeArgBytes is about the most of ripgrep's paths one run is handed: a
// capsule that changes thousands of files is searched in several runs,
// not one whose arguments are too long for the system.
const codeArgBytes = 128 << 10

// batchRoots splits roots into runs whose paths come to about limit
// bytes, splitting a root's files where one alone is more. Whole roots
// count as their path.
func batchRoots(roots []SearchRoot, limit int) [][]SearchRoot {
	var out [][]SearchRoot
	var cur []SearchRoot
	size := 0
	flush := func() {
		if len(cur) > 0 {
			out = append(out, cur)
			cur, size = nil, 0
		}
	}
	for _, r := range roots {
		if r.Files == nil {
			if size+len(r.Path) > limit {
				flush()
			}
			cur = append(cur, r)
			size += len(r.Path) + 1
			continue
		}
		part := SearchRoot{SearchRoot: r.SearchRoot, Files: []string{}}
		for _, f := range r.Files {
			n := len(r.Path) + len(f) + 2
			if size+n > limit {
				if len(part.Files) > 0 {
					cur = append(cur, part)
					part = SearchRoot{SearchRoot: r.SearchRoot, Files: []string{}}
				}
				flush()
			}
			part.Files = append(part.Files, f)
			size += n
		}
		if len(part.Files) > 0 {
			cur = append(cur, part)
		}
	}
	flush()
	return out
}

func capOf(asked, def, most int) int {
	switch {
	case asked <= 0:
		return def
	case asked > most:
		return most
	}
	return asked
}

// codeSearchRoots is what a code search searches, and, by repo, the paths
// whose .ground copy a capsule's takes the place of.
func (w *Workspace) codeSearchRoots(ctx context.Context, req CodeSearchRequest) ([]SearchRoot, map[string]map[string]bool, error) {
	ws := w.WS()
	if req.Repo != "" && !slices.Contains(ws.RepoNames, req.Repo) {
		return nil, nil, ErrNotFound
	}
	if req.Worktree != "" {
		if req.Repo == "" {
			return nil, nil, invalid("name a repo to search a capsule")
		}
		dir, _, err := w.Worktree(req.Repo, req.Worktree)
		if err != nil {
			return nil, nil, err
		}
		root := workspace.SearchRoot{Repo: req.Repo, Worktree: req.Worktree, Path: dir}
		return []SearchRoot{{SearchRoot: root}}, nil, nil
	}

	layout, err := w.FindIndex().Layout(ctx)
	if err != nil {
		return nil, nil, err
	}
	return codeRoots(layout, req.Repo)
}

// codeRoots is the roots of a search across the workspace's layout (or
// one repo of it), in repo order, .ground first.
func codeRoots(l find.Layout, repo string) ([]SearchRoot, map[string]map[string]bool, error) {
	var roots []SearchRoot
	replaced := map[string]map[string]bool{}
	for _, g := range l.Grounds {
		if repo != "" && g.Repo != repo {
			continue
		}
		roots = append(roots, SearchRoot{Repo: g.Repo, Worktree: workspace.GroundDir, Path: g.Dir})
	}
	for _, c := range l.Capsules {
		if repo != "" && c.Repo != repo {
			continue
		}
		var files []string
		for p, st := range c.Files {
			if st == find.Removed {
				continue
			}
			files = append(files, p)
			if replaced[c.Repo] == nil {
				replaced[c.Repo] = map[string]bool{}
			}
			replaced[c.Repo][filepath.ToSlash(p)] = true
		}
		if len(files) == 0 {
			continue
		}
		slices.Sort(files)
		roots = append(roots, SearchRoot{Repo: c.Repo, Worktree: c.Name, Path: c.Dir, Files: files})
	}
	slices.SortStableFunc(roots, func(a, b SearchRoot) int {
		switch {
		case a.Repo < b.Repo:
			return -1
		case a.Repo > b.Repo:
			return 1
		case a.Worktree == b.Worktree:
			return 0
		case a.Worktree == workspace.GroundDir:
			return -1
		case b.Worktree == workspace.GroundDir:
			return 1
		case a.Worktree < b.Worktree:
			return -1
		}
		return 1
	})
	return roots, replaced, nil
}

// codeHit is a match as a page shows it: its text valid UTF-8, cut to a
// window when it's long, and its ranges in UTF-16 code units.
func codeHit(m Match) CodeHit {
	hit := CodeHit{Repo: m.Root.Repo, Worktree: m.Root.Worktree, Path: m.Path, Line: m.Line, Column: m.Column, Ranges: [][2]int{}}
	text, from := m.Text, 0
	if len(text) > codeLineBytes {
		first := 0
		if len(m.Submatches) > 0 {
			first = m.Submatches[0].Start
		}
		from = runeStart(text, max(0, min(first-codeLineBytes/4, len(text)-codeLineBytes)))
		to := runeStart(text, min(len(text), from+codeLineBytes))
		text, hit.Cut = text[from:to], true
	}
	// at[i] is byte i of text in UTF-16 units of the hit's text.
	var b []rune
	at := make([]int, len(text)+1)
	units := 0
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		for j := range size {
			at[i+j] = units
		}
		b = append(b, r) // RuneError for a byte that isn't UTF-8
		if r >= 0x10000 {
			units += 2
		} else {
			units++
		}
		i += size
	}
	at[len(text)] = units
	hit.Text = string(b)
	for _, s := range m.Submatches {
		start, end := s.Start-from, s.End-from
		if end <= 0 || start >= len(text) {
			continue // outside the window
		}
		start, end = max(start, 0), min(end, len(text))
		hit.Ranges = append(hit.Ranges, [2]int{at[start], at[end]})
	}
	return hit
}

// runeStart is i, or the start of the rune it's inside.
func runeStart(s string, i int) int {
	for i > 0 && i < len(s) && !utf8.RuneStart(s[i]) {
		i--
	}
	return i
}
