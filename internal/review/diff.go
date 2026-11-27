// Package review is ws's review service: a mission capsule's committed
// changes, read as a diff, and (to come) the reviews written on them.
package review

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

// Diff is a capsule's committed changes against its base.
type Diff struct {
	BaseRef string `json:"baseRef"` // what it's compared with, such as origin/main
	Base    string `json:"base"`    // where it left BaseRef: their merge base, or Since
	// Since is set when the diff is only what's changed since a commit (the
	// last review's): then Base is that commit.
	Since string `json:"since,omitempty"`
	Head  string `json:"head"`
	// Commits are Base..Head, oldest first.
	Commits []Commit `json:"commits"`
	// Uncommitted are paths changed in the capsule but not committed, which
	// aren't in the diff: a review is of commits.
	Uncommitted []string `json:"uncommitted"`
	Files       []File   `json:"files"`
}

// Commit is one commit in a diff.
type Commit struct {
	SHA     string    `json:"sha"`
	Subject string    `json:"subject"`
	Author  string    `json:"author"`
	Time    time.Time `json:"time"`
}

// File is one file's changes.
type File struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"` // renamed or copied from
	// Status is added, deleted, modified, renamed, copied or type (changed
	// between file, link and submodule).
	Status string `json:"status"`
	// OldBlob and NewBlob are the file's git objects before and after: a
	// file is unchanged since you last looked if its NewBlob is.
	OldBlob   string `json:"oldBlob,omitempty"`
	NewBlob   string `json:"newBlob,omitempty"`
	Binary    bool   `json:"binary,omitempty"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	// Large files' hunks are left out (FileDiff has them), so one generated
	// file doesn't slow the whole review.
	Large bool   `json:"large,omitempty"`
	Hunks []Hunk `json:"hunks"`
	// Skim is why the file needn't be read closely (a lockfile, generated,
	// a snapshot, renamed and nothing else): such files come last, folded.
	Skim string `json:"skim,omitempty"`
}

// Hunk is one stretch of changes, with context.
type Hunk struct {
	OldStart int    `json:"oldStart"`
	OldLines int    `json:"oldLines"`
	NewStart int    `json:"newStart"`
	NewLines int    `json:"newLines"`
	Section  string `json:"section,omitempty"` // what git says it's in: the function, say
	Lines    []Line `json:"lines"`
}

// Line is one line of a hunk. Keys are short: a diff has many.
type Line struct {
	Kind  string `json:"k"`           // "c" context, "a" added, "d" deleted
	Old   int    `json:"o,omitempty"` // its number before (context and deleted)
	New   int    `json:"n,omitempty"` // its number after (context and added)
	Text  string `json:"t"`
	NoEOL bool   `json:"noeol,omitempty"` // the file ends here without a newline
}

// LargeLines is how many lines of diff make a file large.
const LargeLines = 3000

// Compare reads dir's committed changes since it left baseRef.
func Compare(dir, baseRef string) (*Diff, error) {
	head, err := git.ReadLine(context.Background(), dir, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, fmt.Errorf("no commits to review: %w", err)
	}
	if _, err := git.ReadLine(context.Background(), dir, "rev-parse", "--verify", "--quiet", baseRef+"^{commit}"); err != nil {
		return nil, fmt.Errorf("could not compare changes: %s not fetched", baseRef)
	}
	base, err := git.ReadLine(context.Background(), dir, "merge-base", baseRef, head)
	if err != nil {
		return nil, fmt.Errorf("%s and the capsule share no history: %w", baseRef, err)
	}
	d, err := between(dir, baseRef, base, head)
	if err != nil {
		return nil, err
	}
	if d.Uncommitted, err = uncommitted(dir); err != nil {
		return nil, err
	}
	return d, nil
}

// CompareSince is dir's committed changes since a commit: what's changed
// since the last review. since must still be in HEAD's history (a rebase
// can take it away).
func CompareSince(dir, baseRef, since string) (*Diff, error) {
	head, err := git.ReadLine(context.Background(), dir, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, fmt.Errorf("no commits to review: %w", err)
	}
	if _, err := git.ReadLine(context.Background(), dir, "merge-base", "--is-ancestor", since, head); err != nil {
		return nil, fmt.Errorf("%s is no longer in the capsule's history (rebased?): show all changes instead", short(since))
	}
	d, err := between(dir, baseRef, since, head)
	if err != nil {
		return nil, err
	}
	d.Since = since
	if d.Uncommitted, err = uncommitted(dir); err != nil {
		return nil, err
	}
	return d, nil
}

// CompareCommits is the changes from base to head, two commits in a clone
// (a bare one will do): a pull request's, read from what's been fetched.
// There's no working tree to it, so nothing is uncommitted. baseRef names
// what base is of, for the page.
func CompareCommits(dir, baseRef, base, head string) (*Diff, error) {
	for _, c := range []string{base, head} {
		if _, err := git.ReadLine(context.Background(), dir, "rev-parse", "--verify", "--quiet", c+"^{commit}"); err != nil {
			return nil, fmt.Errorf("commit %s isn't here (fetch it?)", short(c))
		}
	}
	return between(dir, baseRef, base, head)
}

// between is the changes from base to head, with large files' hunks left
// out.
func between(dir, baseRef, base, head string) (*Diff, error) {
	d := &Diff{BaseRef: baseRef, Base: base, Head: head, Commits: []Commit{}, Uncommitted: []string{}}
	var err error
	if d.Commits, err = commits(dir, base, head); err != nil {
		return nil, err
	}
	if d.Files, err = files(dir, base, head); err != nil {
		return nil, err
	}
	for i := range d.Files {
		if f := &d.Files[i]; f.Additions+f.Deletions > LargeLines {
			f.Large, f.Hunks = true, []Hunk{}
		}
	}
	order(dir, head, d.Files)
	return d, nil
}

// MergeBase is where head left tip: the commit a pull request's changes
// are from.
func MergeBase(dir, tip, head string) (string, error) {
	base, err := git.ReadLine(context.Background(), dir, "merge-base", tip, head)
	if err != nil {
		return "", fmt.Errorf("%s and %s share no history", short(tip), short(head))
	}
	return base, nil
}

// IsAncestor is whether commit a is in b's history (or is b).
func IsAncestor(dir, a, b string) bool {
	_, err := git.Read(context.Background(), dir, "merge-base", "--is-ancestor", a, b)
	return err == nil
}

// CommitOf is the commit a ref names in dir, or "" if it names none.
func CommitOf(dir, ref string) string {
	sha, err := git.ReadLine(context.Background(), dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return ""
	}
	return sha
}

// FileDiff is one file's changes between base and head in full, large or
// not. path is the file's path at head (or at base, if it was deleted).
func FileDiff(dir, base, head, path string) (*File, error) {
	fs, err := files(dir, base, head)
	if err != nil {
		return nil, err
	}
	for i := range fs {
		if fs[i].Path == path {
			return &fs[i], nil
		}
	}
	return nil, fmt.Errorf("%s is not changed between %s and %s", path, short(base), short(head))
}

// TextDiff is the changes from one text to another where neither is in a
// repo (two versions of an artifact), read as a file of a review is, in
// full, so the web UI shows it in the same diff view. name is what the
// file's called (its extension says its language); it's "modified", with
// no hunks when the two are the same. git diffs them, from copies in a
// folder of their own.
func TextDiff(ctx context.Context, name string, old, new []byte) (*File, error) {
	dir, err := os.MkdirTemp("", "ws-diff-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	for p, text := range map[string][]byte{"a": old, "b": new} {
		if err := os.WriteFile(filepath.Join(dir, p), text, 0o600); err != nil {
			return nil, err
		}
	}
	// --no-index says whether they differ in its exit code: 1 is that they
	// do, and it's printed them.
	patch, err := git.Cmd{Ctx: ctx, Dir: dir, Args: []string{"diff", "--no-index", "--no-color", "--no-ext-diff", "--no-renames", "-U3", "--", "a", "b"}}.Output()
	if err != nil && git.ExitCode(err) != 1 {
		return nil, err
	}
	f := &File{Path: name, Status: "modified", Hunks: []Hunk{}}
	sections := splitSections(string(patch))
	if len(sections) == 0 {
		return f, nil
	}
	if len(sections) > 1 {
		return nil, fmt.Errorf("could not compare versions: %d diffs for two texts", len(sections))
	}
	if err := parseSection(sections[0], f); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return f, nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func commits(dir, base, head string) ([]Commit, error) {
	out, err := git.Read(context.Background(), dir, "log", "--reverse", "--format=%H%x1f%s%x1f%an%x1f%aI%x1e", base+".."+head)
	if err != nil {
		return nil, err
	}
	list := []Commit{}
	for rec := range strings.SplitSeq(string(out), "\x1e") {
		f := strings.Split(strings.TrimSpace(rec), "\x1f")
		if len(f) != 4 {
			continue
		}
		t, _ := time.Parse(time.RFC3339, f[3])
		list = append(list, Commit{SHA: f[0], Subject: f[1], Author: f[2], Time: t})
	}
	return list, nil
}

// uncommitted lists paths with changes not committed, untracked files
// included.
func uncommitted(dir string) ([]string, error) {
	out, err := git.Read(context.Background(), dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	paths := []string{}
	fields := strings.Split(string(out), "\x00")
	for i := 0; i < len(fields); i++ {
		e := fields[i]
		if len(e) < 4 {
			continue
		}
		paths = append(paths, e[3:])
		if e[0] == 'R' || e[0] == 'C' {
			i++ // the path it came from
		}
	}
	return paths, nil
}

var statuses = map[byte]string{'A': "added", 'D': "deleted", 'M': "modified", 'R': "renamed", 'C': "copied", 'T': "type"}

// files reads the changes as git names them (exact paths, -z) and as it
// patches them, in the same order.
func files(dir, base, head string) ([]File, error) {
	names, err := git.Read(context.Background(), dir, "diff", "-z", "--name-status", "-M", base, head)
	if err != nil {
		return nil, err
	}
	list := []File{}
	fields := strings.Split(strings.TrimSuffix(string(names), "\x00"), "\x00")
	for i := 0; i+1 < len(fields); i += 2 {
		code := fields[i]
		f := File{Status: statuses[code[0]], Path: fields[i+1], Hunks: []Hunk{}}
		if f.Status == "" {
			f.Status = "modified"
		}
		if code[0] == 'R' || code[0] == 'C' {
			if i+2 >= len(fields) {
				break
			}
			f.OldPath, f.Path = fields[i+1], fields[i+2]
			i++
		}
		list = append(list, f)
	}
	patch, err := git.Read(context.Background(), dir, "diff", "--no-color", "--no-ext-diff", "--full-index", "-M", "-U3", base, head)
	if err != nil {
		return nil, err
	}
	sections := splitSections(string(patch))
	if len(sections) != len(list) {
		return nil, fmt.Errorf("could not read diff: %d changed files but %d patches", len(list), len(sections))
	}
	for i, s := range sections {
		if err := parseSection(s, &list[i]); err != nil {
			return nil, fmt.Errorf("%s: %w", list[i].Path, err)
		}
	}
	return list, nil
}

// splitSections splits a patch into one section per file.
func splitSections(patch string) [][]string {
	var sections [][]string
	for l := range strings.SplitSeq(strings.TrimSuffix(patch, "\n"), "\n") {
		if strings.HasPrefix(l, "diff --git ") {
			sections = append(sections, nil)
		}
		if n := len(sections); n > 0 {
			sections[n-1] = append(sections[n-1], l)
		}
	}
	return sections
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@ ?(.*)$`)

var indexLine = regexp.MustCompile(`^index ([0-9a-f]+)\.\.([0-9a-f]+)`)

// parseSection reads one file's patch: its blobs, whether it's binary, and
// its hunks. Within a hunk, lines are counted off against its header, so a
// deleted line reading "--- x" is a line, not a header.
func parseSection(lines []string, f *File) error {
	var h *Hunk
	oldLeft, newLeft, oldNo, newNo := 0, 0, 0, 0
	for _, l := range lines[1:] {
		if h != nil && (oldLeft > 0 || newLeft > 0) {
			var line Line
			switch {
			case strings.HasPrefix(l, " ") || l == "":
				line = Line{Kind: "c", Old: oldNo, New: newNo, Text: strings.TrimPrefix(l, " ")}
				oldNo, newNo, oldLeft, newLeft = oldNo+1, newNo+1, oldLeft-1, newLeft-1
			case strings.HasPrefix(l, "-"):
				line = Line{Kind: "d", Old: oldNo, Text: l[1:]}
				oldNo, oldLeft = oldNo+1, oldLeft-1
				f.Deletions++
			case strings.HasPrefix(l, "+"):
				line = Line{Kind: "a", New: newNo, Text: l[1:]}
				newNo, newLeft = newNo+1, newLeft-1
				f.Additions++
			case strings.HasPrefix(l, `\`):
				if n := len(h.Lines); n > 0 {
					h.Lines[n-1].NoEOL = true
				}
				continue
			default:
				return fmt.Errorf("unexpected line in a hunk: %q", l)
			}
			h.Lines = append(h.Lines, line)
			continue
		}
		switch {
		case strings.HasPrefix(l, `\`) && h != nil:
			if n := len(h.Lines); n > 0 {
				h.Lines[n-1].NoEOL = true
			}
		case strings.HasPrefix(l, "@@"):
			m := hunkHeader.FindStringSubmatch(l)
			if m == nil {
				return fmt.Errorf("bad hunk header %q", l)
			}
			f.Hunks = append(f.Hunks, Hunk{
				OldStart: atoi(m[1]), OldLines: count(m[2]), NewStart: atoi(m[3]), NewLines: count(m[4]),
				Section: m[5], Lines: []Line{},
			})
			h = &f.Hunks[len(f.Hunks)-1]
			oldLeft, newLeft, oldNo, newNo = h.OldLines, h.NewLines, h.OldStart, h.NewStart
		case h == nil && strings.HasPrefix(l, "index "):
			if m := indexLine.FindStringSubmatch(l); m != nil {
				f.OldBlob, f.NewBlob = zeroless(m[1]), zeroless(m[2])
			}
		case h == nil && (strings.HasPrefix(l, "Binary files ") || l == "GIT binary patch"):
			f.Binary = true
		}
	}
	return nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// count is a hunk header's line count, 1 when it's left out.
func count(s string) int {
	if s == "" {
		return 1
	}
	return atoi(s)
}

// zeroless is a blob id, or "" for git's all-zeros "no file".
func zeroless(id string) string {
	if strings.Trim(id, "0") == "" {
		return ""
	}
	return id
}

// BlobMax is the largest file Blob reads.
const BlobMax = 4 << 20

// ErrBinary is Blob's answer for a file that isn't text.
var ErrBinary = errors.New("binary file")

// Blob is a file's lines at rev, for showing more context around a hunk.
func Blob(dir, rev, path string) ([]string, error) {
	size, err := git.ReadLine(context.Background(), dir, "cat-file", "-s", rev+":"+path)
	if err != nil {
		return nil, fmt.Errorf("no %s at %s", path, short(rev))
	}
	if n, _ := strconv.Atoi(size); n > BlobMax {
		return nil, fmt.Errorf("%s is too large to show (%d bytes)", path, n)
	}
	out, err := git.Read(context.Background(), dir, "cat-file", "blob", rev+":"+path)
	if err != nil {
		return nil, err
	}
	if bytes.IndexByte(out, 0) >= 0 {
		return nil, ErrBinary
	}
	text := strings.TrimSuffix(string(out), "\n")
	if text == "" {
		return []string{}, nil
	}
	return strings.Split(text, "\n"), nil
}
