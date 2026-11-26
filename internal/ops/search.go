package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/workspace"
)

// Searching the workspace's files with ripgrep: which worktree of each repo
// is searched, globs anchored to each of them, and ripgrep run over them.
// ws rg runs it with ripgrep's own output; the web UI has its matches as
// data (Search). Both choose roots and spell ripgrep's arguments here, so a
// match means the same in both.

// SearchScope is which worktrees a search covers: the repos or worktrees
// named (none for the workspace's default repos), how the worktree of each
// is chosen, and where the asker stands, which ScopeHere goes by.
type SearchScope struct {
	Cwd       string
	Selectors []workspace.RepoSelector
	Mode      workspace.ScopeMode
}

// SearchRoots is the worktrees a scope covers, in repo order with .ground
// first in each (see workspace.SearchRoots). None is refused: there's
// nothing to search.
func (w *Workspace) SearchRoots(s SearchScope) ([]workspace.SearchRoot, error) {
	roots, err := w.WS().SearchRoots(s.Cwd, s.Selectors, s.Mode)
	if err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		return nil, refused("nothing-to-search", "nothing to search: no worktrees found")
	}
	return roots, nil
}

// SearchRoot is one worktree to search, whole, or only some of its files.
type SearchRoot struct {
	workspace.SearchRoot
	// Files, when not nil, are the only files of the root searched: paths
	// relative to it, slash-separated. Empty searches none of it. Ripgrep
	// searches a file it's handed whatever the globs, ignore files and
	// hidden-file rules say.
	Files []string
}

// Rg is ripgrep set up to search roots: where it runs, and its arguments.
type Rg struct {
	Bin  string
	Dir  string
	Args []string
}

// ErrNoRg is ripgrep not found on PATH.
var ErrNoRg = errors.New("ripgrep (rg) not found on PATH; install it from https://github.com/BurntSushi/ripgrep")

// RgExit is ripgrep's exit status when it's not 0: 1 for no match, 2 for
// an error (with what it said of it, when it was kept).
type RgExit struct {
	Code   int
	Stderr string
}

func (e *RgExit) Error() string {
	if msg := strings.TrimSpace(e.Stderr); msg != "" {
		return fmt.Sprintf("ripgrep exited %d: %s", e.Code, msg)
	}
	return fmt.Sprintf("ripgrep exited %d", e.Code)
}

// Rg is ripgrep over roots with args (its own flags, patterns and paths,
// as ws rg is given them), not yet run. See RgArgs.
func (w *Workspace) Rg(roots []SearchRoot, args []string) (*Rg, error) {
	bin, err := exec.LookPath("rg")
	if err != nil {
		return nil, ErrNoRg
	}
	dir, argv := RgArgs(w.WS().Root, roots, args)
	return &Rg{Bin: bin, Dir: dir, Args: argv}, nil
}

// RgArgs spells ripgrep's run over roots: the directory it runs in, and
// args with every slash glob anchored to each root (AnchorGlobs, once per
// worktree, none for a root none of whose files are searched), then the
// roots (or their files) as paths.
//
// It runs from the workspace root: ripgrep matches slash-containing globs
// against paths relative to its cwd, so anchored globs only line up from
// there. Roots are absolute, so the paths ripgrep prints are.
//
// Both are spelled physically. ripgrep finds its cwd with getcwd, which
// resolves symlinks, and only strips that exact prefix from a root before
// matching globs: a workspace reached through a symlink (/tmp or /var on
// macOS, a linked home folder) otherwise matched no slash glob at all.
func RgArgs(wsRoot string, roots []SearchRoot, args []string) (dir string, argv []string) {
	var prefixes []string
	for _, r := range roots {
		if r.Files != nil && len(r.Files) == 0 {
			continue
		}
		if p := r.RelPath(); !slices.Contains(prefixes, p) {
			prefixes = append(prefixes, p)
		}
	}
	argv = AnchorGlobs(args, prefixes)
	for _, r := range roots {
		argv = append(argv, rootPaths(r)...)
	}
	return physicalPath(wsRoot), argv
}

// rootPaths is what ripgrep is handed for a root: its directory, or its
// files under it.
func rootPaths(r SearchRoot) []string {
	base := physicalPath(r.Path)
	if r.Files == nil {
		return []string{base}
	}
	out := make([]string, 0, len(r.Files))
	for _, f := range r.Files {
		out = append(out, filepath.Join(base, filepath.FromSlash(f)))
	}
	return out
}

// Run runs ripgrep with stdio as given, until it ends or ctx is done. Not
// 0 is an *RgExit (its stderr went to stderr, so it's not kept).
func (r *Rg) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	c := exec.CommandContext(ctx, r.Bin, r.Args...)
	c.Dir = r.Dir
	c.Stdin = stdin
	c.Stdout = stdout
	c.Stderr = stderr
	if err := c.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && ctx.Err() == nil {
			return &RgExit{Code: exitErr.ExitCode()}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("running rg: %w", err)
	}
	return nil
}

// AnchorGlobs expands ripgrep -g/--glob/--iglob values that contain a slash
// into one glob per search root, prefixed with the root's workspace-relative
// path. Ripgrep matches such globs against the printed path relative to its
// cwd, not relative to each root, so without this "-g 'src/*.ts'" would match
// nothing. Basename globs (no slash) are left alone. Rewriting stops at "--".
func AnchorGlobs(args []string, prefixes []string) []string {
	out := make([]string, 0, len(args))
	emit := func(flag, glob string) {
		for _, g := range anchorGlob(glob, prefixes) {
			if flag == "" {
				out = append(out, g)
			} else {
				out = append(out, flag, g)
			}
		}
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			out = append(out, args[i:]...)
			return out
		case arg == "-g" || arg == "--glob" || arg == "--iglob":
			if i+1 < len(args) {
				emit(arg, args[i+1])
				i++
			} else {
				out = append(out, arg)
			}
		case strings.HasPrefix(arg, "--glob=") || strings.HasPrefix(arg, "--iglob="):
			flag, glob, _ := strings.Cut(arg, "=")
			for _, g := range anchorGlob(glob, prefixes) {
				out = append(out, flag+"="+g)
			}
		case strings.HasPrefix(arg, "-g") && len(arg) > 2 && !strings.HasPrefix(arg, "--"):
			for _, g := range anchorGlob(arg[2:], prefixes) {
				out = append(out, "-g"+g)
			}
		default:
			out = append(out, arg)
		}
	}
	return out
}

func anchorGlob(glob string, prefixes []string) []string {
	neg := ""
	body := glob
	if strings.HasPrefix(body, "!") {
		neg, body = "!", body[1:]
	}
	if !strings.Contains(body, "/") {
		return []string{glob}
	}
	body = strings.TrimPrefix(body, "/")
	out := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		out = append(out, neg+p+"/"+body)
	}
	return out
}

// SearchRequest is a search: the roots, the patterns (any of which
// matches, each given to ripgrep with -e), and ripgrep's other options
// (flags such as -i, -F, -t go, -g 'src/**'; no patterns or paths, and none
// that change what it prints, such as -l or -c).
type SearchRequest struct {
	Roots    []SearchRoot
	Patterns []string
	Options  []string
}

// Match is one line ripgrep matched.
type Match struct {
	Root workspace.SearchRoot
	Path string // relative to the root, slash-separated
	Line int    // 1-based
	// Column is the first submatch's 1-based byte column, as rg --column
	// prints it.
	Column int
	// Text is the line, without its line ending (lines, with -U).
	Text string
	// Submatches are where in Text the patterns matched, in bytes.
	Submatches []Span
}

// Span is a half-open byte range [Start, End) of a match's text.
type Span struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Search runs ripgrep over the request's roots, as ws rg would, and calls
// emit with each match as ripgrep prints it (rg --json), as it does. A
// root's matches come together a file at a time; files across roots
// interleave. An error from emit stops ripgrep, and Search returns it.
//
// No match is not an error. Ripgrep ending in error (status 2: a bad
// pattern, an unreadable file) is an *RgExit, after the matches it did
// find were emitted.
func (w *Workspace) Search(ctx context.Context, req SearchRequest, emit func(Match) error) error {
	if len(req.Patterns) == 0 {
		return invalid("no pattern to search for")
	}
	if len(req.Roots) == 0 {
		return refused("nothing-to-search", "nothing to search: no roots")
	}
	searched := false
	for _, r := range req.Roots {
		searched = searched || r.Files == nil || len(r.Files) > 0
	}
	if !searched {
		// No paths would have ripgrep search its cwd, the workspace.
		return nil
	}
	args := append([]string{"--json"}, req.Options...)
	for _, p := range req.Patterns {
		args = append(args, "-e", p)
	}
	args = append(args, "--")
	rg, err := w.Rg(req.Roots, args)
	if err != nil {
		return err
	}
	return rg.search(ctx, req.Roots, emit)
}

// search runs ripgrep, whose args ask for --json, and emits its matches.
func (r *Rg) search(ctx context.Context, roots []SearchRoot, emit func(Match) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c := exec.CommandContext(ctx, r.Bin, r.Args...)
	c.Dir = r.Dir
	var stderr limitedBuffer
	stderr.max = 4 << 10
	c.Stderr = &stderr
	out, err := c.StdoutPipe()
	if err != nil {
		return err
	}
	if err := c.Start(); err != nil {
		return fmt.Errorf("running rg: %w", err)
	}

	attr := newRootAttributor(roots)
	readErr := parseRgJSON(out, func(m rgMatch) error {
		root, rel, ok := attr.attribute(m.path)
		if !ok {
			return nil
		}
		match := m.Match
		match.Root, match.Path = root, rel
		return emit(match)
	})
	if readErr != nil {
		cancel()
		io.Copy(io.Discard, out)
	}
	waitErr := c.Wait()

	switch {
	case readErr != nil:
		return readErr
	case ctx.Err() != nil:
		return ctx.Err()
	case waitErr == nil:
		return nil
	}
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		return fmt.Errorf("running rg: %w", waitErr)
	}
	if exitErr.ExitCode() == 1 {
		return nil
	}
	return &RgExit{Code: exitErr.ExitCode(), Stderr: stderr.String()}
}

// rgMatch is a match as ripgrep printed it, its path as printed.
type rgMatch struct {
	Match
	path string
}

// rgText is a string ripgrep prints: text when it's UTF-8, base64 bytes
// when it isn't.
type rgText struct {
	Text  *string `json:"text"`
	Bytes []byte  `json:"bytes"`
}

func (t rgText) String() string {
	if t.Text != nil {
		return *t.Text
	}
	return string(t.Bytes)
}

type rgMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type rgMatchData struct {
	Path       rgText `json:"path"`
	Lines      rgText `json:"lines"`
	LineNumber int    `json:"line_number"`
	Submatches []struct {
		Start int `json:"start"`
		End   int `json:"end"`
	} `json:"submatches"`
}

// parseRgJSON reads ripgrep's --json messages and calls emit with each
// match; the rest (begin, end, context, summary) are skipped. An error
// from emit ends it, returned. Output that isn't ripgrep's JSON (an option
// that changed what it prints) is an error.
func parseRgJSON(r io.Reader, emit func(rgMatch) error) error {
	dec := json.NewDecoder(r)
	for {
		var msg rgMessage
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("reading ripgrep's output: not its JSON (an option such as -l or -c changes what it prints): %w", err)
		}
		if msg.Type != "match" {
			continue
		}
		var d rgMatchData
		if err := json.Unmarshal(msg.Data, &d); err != nil {
			return fmt.Errorf("reading ripgrep's match: %w", err)
		}
		m := rgMatch{path: d.Path.String(),
			Line: d.LineNumber,
			Text: trimLineEnding(d.Lines.String())}
		for _, s := range d.Submatches {
			m.Submatches = append(m.Submatches, Span{Start: s.Start, End: s.End})
		}
		if len(m.Submatches) > 0 {
			m.Column = m.Submatches[0].Start + 1
		}
		if err := emit(m); err != nil {
			return err
		}
	}
}

func trimLineEnding(s string) string {
	s = strings.TrimSuffix(s, "\n")
	return strings.TrimSuffix(s, "\r")
}

// rootAttributor says which root a path ripgrep printed is under.
type rootAttributor struct {
	roots []workspace.SearchRoot
	bases []string // each root's physical path, with a trailing separator
}

func newRootAttributor(roots []SearchRoot) rootAttributor {
	a := rootAttributor{}
	for _, r := range roots {
		a.roots = append(a.roots, r.SearchRoot)
		a.bases = append(a.bases, physicalPath(r.Path)+string(filepath.Separator))
	}
	return a
}

// attribute is the root a path is under, and its path relative to it. The
// longest base wins, should one root be inside another.
func (a rootAttributor) attribute(p string) (workspace.SearchRoot, string, bool) {
	best := -1
	for i, base := range a.bases {
		if strings.HasPrefix(p, base) && (best < 0 || len(base) > len(a.bases[best])) {
			best = i
		}
	}
	if best < 0 {
		return workspace.SearchRoot{}, "", false
	}
	return a.roots[best], filepath.ToSlash(p[len(a.bases[best]):]), true
}

// limitedBuffer keeps the first max bytes written to it.
type limitedBuffer struct {
	bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.Len(); room > 0 {
		if len(p) > room {
			b.Buffer.Write(p[:room])
		} else {
			b.Buffer.Write(p)
		}
	}
	return len(p), nil
}
