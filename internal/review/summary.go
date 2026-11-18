package review

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/withworkspace/workspace/internal/git"
)

// branchPoint is where HEAD left baseRef, or "" when it can't be told (no
// such branch fetched, no shared history).
func branchPoint(dir, baseRef string) string {
	if _, err := git.ReadLine(context.Background(), dir, "rev-parse", "--verify", "--quiet", baseRef+"^{commit}"); err != nil {
		return ""
	}
	base, err := git.ReadLine(context.Background(), dir, "merge-base", baseRef, "HEAD")
	if err != nil {
		return ""
	}
	return base
}

// Changed is one file that differs from the branch point, committed or
// not, without its lines: what the mission's Work tab lists, and opens the
// review at.
type Changed struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"` // renamed or copied from
	// Status is added, deleted, modified, renamed, copied or type, as a
	// diff's file has it.
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	// Skim is why it needn't be read closely, as a diff's file has it.
	Skim string `json:"skim,omitempty"`
	// Uncommitted: it has changes that aren't committed yet (or it's new
	// and untracked), which a review doesn't show.
	Uncommitted bool `json:"uncommitted,omitempty"`
}

// Summary is a capsule in brief: what's changed since its branch point
// (committed), its latest commits, how many files aren't committed, and
// the files that differ from the branch point, committed or not.
type Summary struct {
	BaseRef     string   `json:"baseRef"`
	Base        string   `json:"base,omitempty"`
	Commits     []Commit `json:"commits"` // the latest, newest first
	CommitCount int      `json:"commitCount"`
	Files       int      `json:"files"`
	Additions   int      `json:"additions"`
	Deletions   int      `json:"deletions"`
	Uncommitted int      `json:"uncommitted"`
	// Changed are the files that differ from the branch point as the
	// capsule is now, so what crew haven't committed yet is among them.
	// ChangedMore is how many were left out, past ChangedMax.
	Changed     []Changed `json:"changed"`
	ChangedMore int       `json:"changedMore,omitempty"`
}

// SummaryCommits is how many of the latest commits a summary has, and
// ChangedMax how many changed files.
const (
	SummaryCommits = 50
	ChangedMax     = 400
)

// Summarize is dir in brief.
func Summarize(dir, baseRef string) (*Summary, error) {
	s := &Summary{BaseRef: baseRef, Commits: []Commit{}, Changed: []Changed{}}
	loose, err := uncommitted(dir)
	if err == nil {
		s.Uncommitted = len(loose)
	}
	s.Base = branchPoint(dir, baseRef)
	if s.Base == "" {
		return s, nil
	}
	all, err := commits(dir, s.Base, "HEAD")
	if err != nil {
		return nil, err
	}
	s.CommitCount = len(all)
	for i := len(all) - 1; i >= 0 && len(s.Commits) < SummaryCommits; i-- {
		s.Commits = append(s.Commits, all[i])
	}
	out, err := git.Read(context.Background(), dir, "diff", "--numstat", "-z", "-M", s.Base, "HEAD")
	if err != nil {
		return nil, err
	}
	// numstat -z: "adds\tdels\tpath\0", or for a rename "adds\tdels\t\0from\0to\0".
	fields := strings.Split(string(out), "\x00")
	for i := 0; i < len(fields); i++ {
		parts := strings.SplitN(fields[i], "\t", 3)
		if len(parts) < 3 {
			continue
		}
		s.Files++
		a, _ := strconv.Atoi(parts[0]) // "-" for a binary file: 0
		d, _ := strconv.Atoi(parts[1])
		s.Additions += a
		s.Deletions += d
		if parts[2] == "" {
			i += 2 // a rename's two paths follow
		}
	}
	files, err := changed(dir, s.Base, loose)
	if err != nil {
		return nil, err
	}
	if len(files) > ChangedMax {
		s.ChangedMore, files = len(files)-ChangedMax, files[:ChangedMax]
	}
	s.Changed = files
	return s, nil
}

// changed is the working tree against base: what git names and counts
// for tracked files, then the untracked ones (loose has them), which are
// new since the branch point too.
func changed(dir, base string, loose []string) ([]Changed, error) {
	names, err := git.Read(context.Background(), dir, "diff", "-z", "--name-status", "-M", base)
	if err != nil {
		return nil, err
	}
	list := []Changed{}
	at := map[string]int{}
	fields := strings.Split(strings.TrimSuffix(string(names), "\x00"), "\x00")
	for i := 0; i+1 < len(fields); i += 2 {
		code := fields[i]
		if code == "" {
			break
		}
		f := Changed{Status: statuses[code[0]], Path: fields[i+1]}
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
		at[f.Path] = len(list)
		list = append(list, f)
	}
	counts, err := git.Read(context.Background(), dir, "diff", "--numstat", "-z", "-M", base)
	if err != nil {
		return nil, err
	}
	// numstat -z: "adds\tdels\tpath\0", or for a rename "adds\tdels\t\0from\0to\0".
	fields = strings.Split(string(counts), "\x00")
	for i := 0; i < len(fields); i++ {
		parts := strings.SplitN(fields[i], "\t", 3)
		if len(parts) < 3 {
			continue
		}
		path := parts[2]
		if path == "" {
			if i+2 >= len(fields) {
				break
			}
			path = fields[i+2]
			i += 2
		}
		if j, ok := at[path]; ok {
			list[j].Additions, _ = strconv.Atoi(parts[0]) // "-" for a binary file: 0
			list[j].Deletions, _ = strconv.Atoi(parts[1])
		}
	}
	for _, p := range loose {
		j, ok := at[p]
		if !ok {
			// Untracked: every line of it is new.
			j, at[p] = len(list), len(list)
			list = append(list, Changed{Path: p, Status: "added", Additions: countLines(filepath.Join(dir, p))})
		}
		list[j].Uncommitted = true
	}
	for i := range list {
		// skimOf reads a diff's file: a rename is one to skim when nothing
		// else changed in it, which there is said by its hunks.
		f := File{Path: list[i].Path, Status: list[i].Status}
		if list[i].Additions+list[i].Deletions > 0 {
			f.Hunks = []Hunk{{}}
		}
		list[i].Skim = skimOf(f, nil)
	}
	return list, nil
}

// countLines is how many lines a file has, or 0 for one that's large,
// binary or can't be read: enough for an untracked file's count.
func countLines(path string) int {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > 1<<20 {
		return 0
	}
	b, err := os.ReadFile(path)
	if err != nil || bytes.IndexByte(b, 0) >= 0 {
		return 0
	}
	n := bytes.Count(b, []byte{'\n'})
	if len(b) > 0 && b[len(b)-1] != '\n' {
		n++
	}
	return n
}
