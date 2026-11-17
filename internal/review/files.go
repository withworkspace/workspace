package review

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/withworkspace/workspace/internal/git"
)

// A capsule as files, for the web UI's file explorer: every file in its
// working tree with how it stands against where the capsule left its base
// branch (committed or not), a file's text with the lines that changed,
// and a summary of the whole for the mission's Work tab.

// TreeFile is one file in a capsule's working tree, or one gone from it
// since the branch point.
type TreeFile struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"` // renamed from
	// Status is how it differs from the branch point: added, modified,
	// deleted, renamed, copied, type, or "" for unchanged.
	Status string `json:"status,omitempty"`
	// Uncommitted: it has changes that aren't committed yet.
	Uncommitted bool `json:"uncommitted,omitempty"`
}

// Tree is a capsule's files.
type Tree struct {
	BaseRef string     `json:"baseRef"`
	Base    string     `json:"base,omitempty"` // the branch point: "" when BaseRef isn't here
	Head    string     `json:"head,omitempty"`
	Branch  string     `json:"branch,omitempty"`
	Files   []TreeFile `json:"files"`
	// Truncated: there were more than TreeMax files, and the rest are left
	// out.
	Truncated bool `json:"truncated,omitempty"`
}

// TreeMax is the most files a tree lists.
const TreeMax = 20000

// ReadTree lists dir's files: tracked and untracked (not ignored), with
// those changed since the branch point marked, deleted ones included.
func ReadTree(dir, baseRef string) (*Tree, error) {
	t := &Tree{BaseRef: baseRef, Files: []TreeFile{}}
	t.Head, _ = git.ReadLine(context.Background(), dir, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	t.Branch, _ = git.ReadLine(context.Background(), dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	t.Base = branchPoint(dir, baseRef)

	out, err := git.Read(context.Background(), dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	at := map[string]int{}
	for _, p := range strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if p == "" {
			continue
		}
		if _, seen := at[p]; seen {
			continue // listed twice: staged and changed
		}
		if len(t.Files) >= TreeMax {
			t.Truncated = true
			break
		}
		at[p] = len(t.Files)
		t.Files = append(t.Files, TreeFile{Path: p})
	}

	// Changed since the branch point, committed or not: the working tree
	// against the base.
	if t.Base != "" {
		names, err := git.Read(context.Background(), dir, "diff", "-z", "--name-status", "-M", t.Base)
		if err != nil {
			return nil, err
		}
		fields := strings.Split(strings.TrimSuffix(string(names), "\x00"), "\x00")
		for i := 0; i+1 < len(fields); i += 2 {
			code := fields[i]
			f := TreeFile{Status: statuses[code[0]], Path: fields[i+1]}
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
			if j, ok := at[f.Path]; ok {
				t.Files[j].Status, t.Files[j].OldPath = f.Status, f.OldPath
			} else if f.Status == "deleted" && len(t.Files) < TreeMax {
				at[f.Path] = len(t.Files)
				t.Files = append(t.Files, f)
			}
		}
	}
	// Uncommitted, untracked included: untracked files are new since the
	// branch point too.
	changed, err := uncommitted(dir)
	if err != nil {
		return nil, err
	}
	for _, p := range changed {
		if j, ok := at[p]; ok {
			t.Files[j].Uncommitted = true
			if t.Files[j].Status == "" && t.Base != "" {
				t.Files[j].Status = "added"
			}
		}
	}
	return t, nil
}

// FileMax is the largest file ReadFile shows as text.
const FileMax = 2 << 20

// Change is a stretch of a file that's changed since the branch point, in
// its lines now: added, modified, or (a point, Start only) deleted after.
type Change struct {
	Kind  string `json:"kind"` // added, modified, deleted
	Start int    `json:"start"`
	End   int    `json:"end,omitempty"`
}

// FileView is a file as it is in the capsule now.
type FileView struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Binary bool   `json:"binary,omitempty"`
	// TooLarge: bigger than FileMax, so not shown.
	TooLarge bool     `json:"tooLarge,omitempty"`
	Lines    []string `json:"lines"`
	// Deleted: gone from the working tree; Lines are as it was at the
	// branch point.
	Deleted bool     `json:"deleted,omitempty"`
	Changes []Change `json:"changes"`
}

// ErrOutside is ReadFile's answer for a path that leads out of the capsule.
var ErrOutside = errors.New("that path is outside the capsule")

// ResolvePath is path in dir, refusing anything that leads out of it (.., or
// a link to somewhere else).
func ResolvePath(dir, path string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrOutside
	}
	full := filepath.Join(dir, clean)
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(full)
	if err != nil {
		return full, err // not there (deleted, say): the caller decides
	}
	if rel, err := filepath.Rel(root, real); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrOutside
	}
	return real, nil
}

// ReadFile is path in dir as it is now, with what's changed since the
// branch point marked; a file deleted since is read as it was there.
func ReadFile(dir, baseRef, path string) (*FileView, error) {
	v := &FileView{Path: path, Lines: []string{}, Changes: []Change{}}
	full, err := ResolvePath(dir, path)
	if errors.Is(err, ErrOutside) {
		return nil, err
	}
	base := branchPoint(dir, baseRef)
	if err != nil {
		if !os.IsNotExist(err) && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if base == "" {
			return nil, fmt.Errorf("no %s in the capsule", path)
		}
		lines, berr := Blob(dir, base, path)
		if errors.Is(berr, ErrBinary) {
			v.Deleted, v.Binary = true, true
			return v, nil
		}
		if berr != nil {
			return nil, fmt.Errorf("no %s in the capsule", path)
		}
		v.Deleted, v.Lines = true, lines
		return v, nil
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a folder", path)
	}
	v.Size = info.Size()
	if v.Size > FileMax {
		v.TooLarge = true
		return v, nil
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		v.Binary = true
		return v, nil
	}
	if text := strings.TrimSuffix(string(data), "\n"); text != "" {
		v.Lines = strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	}
	if base != "" {
		v.Changes = changesSince(dir, base, path)
	}
	return v, nil
}

// changesSince is where path's lines differ from base, in its lines now
// (the working tree, committed or not). An untracked file is all new.
func changesSince(dir, base, path string) []Change {
	out, err := git.Read(context.Background(), dir, "diff", "--no-color", "--no-ext-diff", "-U0", base, "--", path)
	list := []Change{}
	if err != nil {
		return list
	}
	if len(out) == 0 {
		// Not in base and not tracked: new.
		if _, err := git.ReadLine(context.Background(), dir, "cat-file", "-e", base+":"+path); err != nil {
			if lines, err := os.ReadFile(filepath.Join(dir, path)); err == nil {
				n := strings.Count(strings.TrimSuffix(string(lines), "\n"), "\n") + 1
				list = append(list, Change{Kind: "added", Start: 1, End: n})
			}
		}
		return list
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}
		// @@ -a[,b] +c[,d] @@
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		_, oldN := hunkRange(parts[1])
		newStart, newN := hunkRange(parts[2])
		switch {
		case newN == 0:
			list = append(list, Change{Kind: "deleted", Start: newStart})
		case oldN == 0:
			list = append(list, Change{Kind: "added", Start: newStart, End: newStart + newN - 1})
		default:
			list = append(list, Change{Kind: "modified", Start: newStart, End: newStart + newN - 1})
		}
	}
	return list
}

// hunkRange reads "-12,3" or "+12" as start and count.
func hunkRange(s string) (int, int) {
	s = strings.TrimLeft(s, "-+")
	start, count, found := strings.Cut(s, ",")
	a, _ := strconv.Atoi(start)
	if !found {
		return a, 1
	}
	n, _ := strconv.Atoi(count)
	return a, n
}
