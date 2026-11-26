package github

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Who's asked you, by name, to review: what needs you of the pull requests
// outside a mission (ops.Needs). The daemon asks it whether or not a
// Reviews page is open, so it's one search of an owner and not a look at
// each repo; and you can take your name off one that isn't yours to review.

// AskedByName is the org's open PRs, drafts left out, that ask for your
// review by name: not those asked of a team you're in, which someone else
// can answer.
//
// VERIFIED (GitHub's search through gh, 2026-11-23): user-review-requested:@me
// is "pull requests that you have directly been asked to review". One a
// team's assignment gave to you is among them: it asks you by name.
func AskedByName(org string) ([]PRRef, error) {
	stdOut, stdErr, err := ghExec(
		"search", "prs",
		"--state=open",
		"--draft=false",
		"--owner="+org,
		"--limit", "200",
		"--json", "number,repository",
		"--", "user-review-requested:@me",
	)
	if err != nil {
		return nil, fmt.Errorf("gh search prs user-review-requested:@me: %s", ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	var found []struct {
		Number     int `json:"number"`
		Repository struct {
			Name          string `json:"name"`
			NameWithOwner string `json:"nameWithOwner"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(stdOut.Bytes(), &found); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}
	out := make([]PRRef, 0, len(found))
	for _, f := range found {
		owner, _, _ := strings.Cut(f.Repository.NameWithOwner, "/")
		out = append(out, PRRef{Owner: owner, Repo: f.Repository.Name, Number: f.Number})
	}
	return out, nil
}

func (LiveClient) AskedByName(org string) ([]PRRef, error) { return AskedByName(org) }

// AskedByName is the fixture's open PRs, drafts left out, whose own
// requests name you.
func (c FixtureClient) AskedByName(org string) ([]PRRef, error) {
	fixtureMu.Lock()
	defer fixtureMu.Unlock()
	f, err := c.read()
	if err != nil {
		return nil, err
	}
	var out []PRRef
	for name, r := range f.Repos {
		for _, pr := range r.PRs {
			if pr.IsDraft {
				continue
			}
			for _, q := range pr.Requested {
				if q.Login != "" && q.Login == f.Viewer {
					out = append(out, PRRef{Owner: org, Repo: name, Number: pr.Number})
				}
			}
		}
	}
	return out, nil
}

func (s *Stub) AskedByName(org string) ([]PRRef, error) {
	if s.AskedByNameFn != nil {
		return s.AskedByNameFn(org)
	}
	return nil, nil
}

// DropReviewRequest takes login off the people asked to review a PR.
func DropReviewRequest(org, repo string, number int, login string) error {
	stdOut, stdErr, err := ghExec("api", "--method", "DELETE",
		fmt.Sprintf("repos/%s/%s/pulls/%d/requested_reviewers", org, repo, number),
		"-f", "reviewers[]="+login)
	if err != nil {
		return fmt.Errorf("taking %s off %s#%d's reviewers on GitHub: %s", login, repo, number, ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	return nil
}

func (LiveClient) DropReviewRequest(org, repo string, number int, login string) error {
	return DropReviewRequest(org, repo, number, login)
}

// DropReviewRequest takes login off the fixture's pull request's
// reviewers.
func (c FixtureClient) DropReviewRequest(org, repo string, number int, login string) error {
	at := c.now().UTC().Format(time.RFC3339)
	return c.change(func(f *Fixture) error {
		if r := f.Repos[repo]; r != nil {
			for i := range r.PRs {
				if r.PRs[i].Number != number {
					continue
				}
				asked := r.PRs[i].Requested[:0]
				for _, q := range r.PRs[i].Requested {
					if q.Login != login {
						asked = append(asked, q)
					}
				}
				r.PRs[i].Requested, r.PRs[i].UpdatedAt = asked, at
				return nil
			}
		}
		return fmt.Errorf("no open PR #%d in %s/%s", number, org, repo)
	})
}

func (s *Stub) DropReviewRequest(org, repo string, number int, login string) error {
	if s.DropReviewRequestFn != nil {
		return s.DropReviewRequestFn(org, repo, number, login)
	}
	return fmt.Errorf("no GitHub here to take a reviewer off a pull request on")
}
