package team

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

// What lands on a default branch is read from git, in the repo's clone:
// the branch is fetched, and its commits read along its first parents, so
// a merged branch's own commits aren't each counted as landing.

// Commit is a commit on a repo's default branch.
type Commit struct {
	SHA     string
	Name    string
	Email   string
	Subject string
	// Lead is the first line of its message after the subject: a merge
	// commit's is its pull request's title.
	Lead string
	// At is when it was committed, which is nearer to when it landed than
	// when it was written.
	At time.Time
	// Parents is how many it has: two for a merge commit.
	Parents   int
	Additions int
	Deletions int
	Files     int
	Paths     []string
}

// pathsKept is how many of a commit's paths are kept.
const pathsKept = 40

// Fetch brings a repo's default branch up to date in its clone.
func Fetch(bare, branch string) error {
	_, err := git.Output(bare, "fetch", "--quiet", "--no-tags", "origin", fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", branch, branch))
	return err
}

// Commits are the commits on a repo's default branch since a time, as its
// clone has it, newest first.
func Commits(bare, branch string, since time.Time) ([]Commit, error) {
	out, err := git.Output(bare, "log", "--first-parent", "--diff-merges=first-parent", "--numstat",
		"--since="+since.Format(time.RFC3339), "--format=%x1e%H%x1f%P%x1f%an%x1f%ae%x1f%cI%x1f%s%x1f%b%x1d", "refs/remotes/origin/"+branch, "--")
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for rec := range strings.SplitSeq(out, "\x1e") {
		head, stat, _ := strings.Cut(strings.TrimLeft(rec, "\n"), "\x1d")
		f := strings.Split(head, "\x1f")
		if len(f) < 7 {
			continue
		}
		c := Commit{SHA: f[0], Parents: len(strings.Fields(f[1])), Name: f[2], Email: f[3], Subject: f[5]}
		c.Lead, _, _ = strings.Cut(strings.TrimSpace(f[6]), "\n")
		c.At, _ = time.Parse(time.RFC3339, f[4])
		for line := range strings.SplitSeq(stat, "\n") {
			n := strings.SplitN(line, "\t", 3)
			if len(n) < 3 {
				continue
			}
			// A binary file's counts are "-".
			add, _ := strconv.Atoi(n[0])
			del, _ := strconv.Atoi(n[1])
			c.Additions += add
			c.Deletions += del
			c.Files++
			if len(c.Paths) < pathsKept {
				c.Paths = append(c.Paths, n[2])
			}
		}
		commits = append(commits, c)
	}
	return commits, nil
}
