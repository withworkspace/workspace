package review

import (
	"context"
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

// Summary is a capsule in brief: what's changed since its branch point
// (committed), its latest commits, and how many files aren't committed.
type Summary struct {
	BaseRef     string   `json:"baseRef"`
	Base        string   `json:"base,omitempty"`
	Commits     []Commit `json:"commits"` // the latest, newest first
	CommitCount int      `json:"commitCount"`
	Files       int      `json:"files"`
	Additions   int      `json:"additions"`
	Deletions   int      `json:"deletions"`
	Uncommitted int      `json:"uncommitted"`
}

// SummaryCommits is how many of the latest commits a summary has.
const SummaryCommits = 3

// Summarize is dir in brief.
func Summarize(dir, baseRef string) (*Summary, error) {
	s := &Summary{BaseRef: baseRef, Commits: []Commit{}}
	if changed, err := uncommitted(dir); err == nil {
		s.Uncommitted = len(changed)
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
	return s, nil
}
