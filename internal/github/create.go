package github

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	gh "github.com/cli/go-gh/v2"
)

// NewPR is a pull request to open: from a branch, onto a base, with what it
// says.
type NewPR struct {
	Head, Base  string
	Title, Body string
	Draft       bool
}

// CreatePR opens a pull request with gh pr create, and says which.
func CreatePR(org, repo string, n NewPR) (*PR, error) {
	args := []string{"pr", "create", "--repo", repoSlug(org, repo), "--head", n.Head, "--base", n.Base, "--title", n.Title, "--body", n.Body}
	if n.Draft {
		args = append(args, "--draft")
	}
	stdOut, stdErr, err := gh.Exec(args...)
	if err != nil {
		return nil, fmt.Errorf("opening a pull request on GitHub: %s", ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	url := lastLine(stdOut.String())
	number, err := strconv.Atoi(url[strings.LastIndex(url, "/")+1:])
	if err != nil {
		return nil, fmt.Errorf("gh opened a pull request, but said %q, not its address", url)
	}
	return &PR{Number: number, Title: n.Title, HeadRefName: n.Head, BaseRefName: n.Base, State: "OPEN", URL: url, IsDraft: n.Draft}, nil
}

// lastLine is the last line of s that says something.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func (LiveClient) CreatePR(org, repo string, n NewPR) (*PR, error) { return CreatePR(org, repo, n) }

// CreatePR opens the pull request on the fixture's GitHub, by you, as the
// next number in its repo; refused when one is open from its branch
// already.
func (c FixtureClient) CreatePR(org, repo string, n NewPR) (*PR, error) {
	var out PR
	at := c.now().UTC().Format(time.RFC3339)
	err := c.change(func(f *Fixture) error {
		if f.Repos == nil {
			f.Repos = map[string]*FixtureRepo{}
		}
		r := f.Repos[repo]
		if r == nil {
			r = &FixtureRepo{}
			f.Repos[repo] = r
		}
		for _, pr := range r.PRs {
			if pr.HeadRefName == n.Head {
				return fmt.Errorf("a pull request for branch %q into branch %q already exists: %s#%d", n.Head, n.Base, repo, pr.Number)
			}
		}
		number := 0
		for _, pr := range append(append(append([]PR{}, r.PRs...), r.Merged...), r.Closed...) {
			number = max(number, pr.Number)
		}
		out = PR{Number: number + 1, Title: n.Title, HeadRefName: n.Head, BaseRefName: n.Base, State: "OPEN", IsDraft: n.Draft,
			URL: fmt.Sprintf("https://github.com/%s/%s/pull/%d", org, repo, number+1), Author: f.Viewer, CreatedAt: at, UpdatedAt: at}
		r.PRs = append(r.PRs, out)
		if strings.TrimSpace(n.Body) != "" {
			if r.Bodies == nil {
				r.Bodies = map[string]string{}
			}
			r.Bodies[strconv.Itoa(out.Number)] = n.Body
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Stub) CreatePR(org, repo string, n NewPR) (*PR, error) {
	if s.CreatePRFn != nil {
		return s.CreatePRFn(org, repo, n)
	}
	return nil, fmt.Errorf("no GitHub here to open a pull request on")
}

// MarkReady marks a draft pull request ready for review, with gh pr ready.
func MarkReady(org, repo string, number int) error {
	stdOut, stdErr, err := gh.Exec("pr", "ready", strconv.Itoa(number), "--repo", repoSlug(org, repo))
	if err != nil {
		return fmt.Errorf("marking %s#%d ready on GitHub: %s", repo, number, ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	return nil
}

func (LiveClient) MarkReady(org, repo string, number int) error { return MarkReady(org, repo, number) }

// MarkReady marks the fixture's pull request ready: no longer a draft.
func (c FixtureClient) MarkReady(org, repo string, number int) error {
	at := c.now().UTC().Format(time.RFC3339)
	return c.change(func(f *Fixture) error {
		if r := f.Repos[repo]; r != nil {
			for i := range r.PRs {
				if r.PRs[i].Number == number {
					r.PRs[i].IsDraft, r.PRs[i].UpdatedAt = false, at
					return nil
				}
			}
		}
		return fmt.Errorf("no open PR #%d in %s/%s", number, org, repo)
	})
}

func (s *Stub) MarkReady(org, repo string, number int) error {
	if s.MarkReadyFn != nil {
		return s.MarkReadyFn(org, repo, number)
	}
	return fmt.Errorf("no GitHub here to mark a pull request ready on")
}
