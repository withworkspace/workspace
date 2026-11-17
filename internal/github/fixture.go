package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// FixtureClient answers for GitHub from a JSON file, for trying ws where
// there's no GitHub (the test workspace: [github] fixture in
// ws.local.toml) and for tests. It's read on every call, so editing the
// file changes what ws sees; a review submitted, a reply or a resolve is
// written back into it, as GitHub would remember it.
type FixtureClient struct {
	Path string
	// Now is the clock (tests set it).
	Now func() time.Time
}

// Fixture is the file's shape.
type Fixture struct {
	// Viewer is who you are.
	Viewer string                  `json:"viewer"`
	Repos  map[string]*FixtureRepo `json:"repos"`
	// Requested are PRs that ask for your review through a team (those that
	// ask you by name say so in their own requested list).
	Requested []PRRef `json:"requested,omitempty"`
	// Submitted are the reviews posted, newest last.
	Submitted []FixtureReview `json:"submitted,omitempty"`
}

// FixtureRepo is one repo on the fixture's GitHub.
type FixtureRepo struct {
	PRs    []PR `json:"prs"`
	Merged []PR `json:"merged,omitempty"`
	// Closed are PRs closed without merging.
	Closed []PR `json:"closed,omitempty"`
	// Stacks are the repo's stacks; without the key, stacks aren't turned
	// on there.
	Stacks []Stack `json:"stacks,omitempty"`
	// Bodies are PRs' descriptions, and Threads their review threads, by
	// number.
	Bodies  map[string]string         `json:"bodies,omitempty"`
	Threads map[string][]ReviewThread `json:"threads,omitempty"`
	// Said is what's been said on PRs themselves (reviews and comments), by
	// number, oldest first.
	Said map[string][]Said `json:"said,omitempty"`
}

// FixtureReview is a review that was posted.
type FixtureReview struct {
	Repo   string           `json:"repo"`
	Number int              `json:"number"`
	At     time.Time        `json:"at"`
	Review ReviewSubmission `json:"review"`
}

var fixtureMu sync.Mutex

func (c FixtureClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c FixtureClient) read() (*Fixture, error) {
	data, err := os.ReadFile(c.Path)
	if err != nil {
		return nil, fmt.Errorf("github fixture: %w", err)
	}
	var f Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("github fixture %s: %w", c.Path, err)
	}
	if f.Viewer == "" {
		f.Viewer = "you"
	}
	return &f, nil
}

// change reads the file, lets fn change it, and writes it back.
func (c FixtureClient) change(fn func(f *Fixture) error) error {
	fixtureMu.Lock()
	defer fixtureMu.Unlock()
	f, err := c.read()
	if err != nil {
		return err
	}
	if err := fn(f); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.Path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.Path)
}

func (c FixtureClient) repo(name string) (*Fixture, *FixtureRepo, error) {
	fixtureMu.Lock()
	defer fixtureMu.Unlock()
	f, err := c.read()
	if err != nil {
		return nil, nil, err
	}
	r := f.Repos[name]
	if r == nil {
		r = &FixtureRepo{}
	}
	return f, r, nil
}

func (c FixtureClient) PRsForRepo(org, repo string) ([]PR, error) {
	_, r, err := c.repo(repo)
	if err != nil {
		return nil, err
	}
	for i := range r.PRs {
		if r.PRs[i].State == "" {
			r.PRs[i].State = "OPEN"
		}
	}
	return r.PRs, nil
}

func (c FixtureClient) OpenPRs(org, repo string) ([]PR, error) { return c.PRsForRepo(org, repo) }

func (c FixtureClient) MergedPRsForRepo(org, repo string) ([]PR, error) {
	_, r, err := c.repo(repo)
	if err != nil {
		return nil, err
	}
	return r.Merged, nil
}

func (c FixtureClient) PRFromNumber(org, repo string, number int) (*PR, error) {
	_, r, err := c.repo(repo)
	if err != nil {
		return nil, err
	}
	for _, list := range [][]PR{r.PRs, r.Merged, r.Closed} {
		for i := range list {
			if list[i].Number == number {
				pr := list[i]
				if pr.State == "" {
					pr.State = "OPEN"
				}
				return &pr, nil
			}
		}
	}
	return nil, fmt.Errorf("no PR #%d in %s/%s", number, org, repo)
}

func (c FixtureClient) PRDetail(org, repo string, number int) (PRDetailResult, error) {
	_, r, err := c.repo(repo)
	if err != nil {
		return PRDetailResult{}, err
	}
	pr, err := c.PRFromNumber(org, repo, number)
	if err != nil {
		return PRDetailResult{}, err
	}
	return PRDetailResult{Title: pr.Title, Body: r.Bodies[strconv.Itoa(number)]}, nil
}

func (c FixtureClient) WorkflowRuns(org, repo, branch string, limit int) ([]WorkflowRun, error) {
	return nil, nil
}

func (c FixtureClient) Viewer() (string, error) {
	fixtureMu.Lock()
	defer fixtureMu.Unlock()
	f, err := c.read()
	if err != nil {
		return "", err
	}
	return f.Viewer, nil
}

// ReviewRequested is the PRs asking for your review: by name, and those
// the fixture lists as asked of a team of yours.
func (c FixtureClient) ReviewRequested(org string) ([]PRRef, error) {
	fixtureMu.Lock()
	defer fixtureMu.Unlock()
	f, err := c.read()
	if err != nil {
		return nil, err
	}
	out := append([]PRRef(nil), f.Requested...)
	for name, r := range f.Repos {
		for _, pr := range r.PRs {
			for _, q := range pr.Requested {
				if q.Login == f.Viewer {
					out = append(out, PRRef{Repo: name, Number: pr.Number})
				}
			}
		}
	}
	return out, nil
}

func (c FixtureClient) StacksForRepo(org, repo string) ([]Stack, error) {
	_, r, err := c.repo(repo)
	if err != nil {
		return nil, err
	}
	return r.Stacks, nil
}

// SubmitReview remembers the review as GitHub would: it's your latest on
// the PR, you're no longer asked to review it, and its comments are
// threads.
func (c FixtureClient) SubmitReview(org, repo string, number int, sub ReviewSubmission) (string, error) {
	at := c.now()
	err := c.change(func(f *Fixture) error {
		r := f.Repos[repo]
		var pr *PR
		if r != nil {
			for i := range r.PRs {
				if r.PRs[i].Number == number {
					pr = &r.PRs[i]
				}
			}
		}
		if pr == nil {
			return fmt.Errorf("no open PR #%d in %s/%s", number, org, repo)
		}
		if pr.Author == f.Viewer && sub.Event != EventComment {
			return errors.New("Unprocessable Entity: Can not approve your own pull request")
		}
		state := map[string]string{EventApprove: "APPROVED", EventRequestChanges: "CHANGES_REQUESTED", EventComment: "COMMENTED"}[sub.Event]
		if state == "" {
			return fmt.Errorf("Unprocessable Entity: event %q", sub.Event)
		}
		kept := pr.Latest[:0]
		for _, l := range pr.Latest {
			if l.Author != f.Viewer {
				kept = append(kept, l)
			}
		}
		pr.Latest = append(kept, LatestReview{Author: f.Viewer, State: state, SubmittedAt: at.UTC().Format(time.RFC3339), Commit: sub.Commit})
		asked := pr.Requested[:0]
		for _, q := range pr.Requested {
			if q.Login != f.Viewer {
				asked = append(asked, q)
			}
		}
		pr.Requested = asked
		req := f.Requested[:0]
		for _, q := range f.Requested {
			if q.Repo != repo || q.Number != number {
				req = append(req, q)
			}
		}
		f.Requested = req
		pr.UpdatedAt = at.UTC().Format(time.RFC3339)
		if r.Threads == nil {
			r.Threads = map[string][]ReviewThread{}
		}
		key := strconv.Itoa(number)
		for _, cm := range sub.Comments {
			t := ReviewThread{ID: fmt.Sprintf("T_%s_%d_%d", repo, number, len(r.Threads[key])+1), Path: cm.Path, Line: cm.Line, StartLine: cm.StartLine, Side: cm.Side,
				OriginalLine: cm.Line, OriginalStartLine: cm.StartLine,
				Comments: []ThreadComment{{Author: f.Viewer, Body: cm.Body, At: at.UTC().Format(time.RFC3339), Commit: sub.Commit, OriginalCommit: sub.Commit}}}
			r.Threads[key] = append(r.Threads[key], t)
		}
		if r.Said == nil {
			r.Said = map[string][]Said{}
		}
		r.Said[key] = append(r.Said[key], Said{ID: fmt.Sprintf("R_%s_%d_%d", repo, number, len(r.Said[key])+1), Author: f.Viewer, State: state, Body: sub.Body,
			At: at.UTC().Format(time.RFC3339), Commit: sub.Commit})
		f.Submitted = append(f.Submitted, FixtureReview{Repo: repo, Number: number, At: at, Review: sub})
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://github.com/%s/%s/pull/%d#pullrequestreview-fixture", org, repo, number), nil
}

func (c FixtureClient) ReviewThreads(org, repo string, number int) ([]ReviewThread, error) {
	_, r, err := c.repo(repo)
	if err != nil {
		return nil, err
	}
	return r.Threads[strconv.Itoa(number)], nil
}

func (c FixtureClient) thread(repo string, number int, id string, fn func(f *Fixture, t *ReviewThread)) error {
	return c.change(func(f *Fixture) error {
		if r := f.Repos[repo]; r != nil {
			list := r.Threads[strconv.Itoa(number)]
			for i := range list {
				if list[i].ID == id {
					fn(f, &list[i])
					return nil
				}
			}
		}
		return fmt.Errorf("no thread %s on %s#%d", id, repo, number)
	})
}

func (c FixtureClient) ReplyToThread(org, repo string, number int, threadID, body string) error {
	at := c.now().UTC().Format(time.RFC3339)
	return c.thread(repo, number, threadID, func(f *Fixture, t *ReviewThread) {
		t.Comments = append(t.Comments, ThreadComment{Author: f.Viewer, Body: body, At: at})
	})
}

func (c FixtureClient) ResolveThread(org, repo string, number int, threadID string, resolved bool) error {
	return c.thread(repo, number, threadID, func(f *Fixture, t *ReviewThread) { t.Resolved = resolved })
}

func (c FixtureClient) Discussion(org, repo string, number int) ([]Said, error) {
	_, r, err := c.repo(repo)
	if err != nil {
		return nil, err
	}
	return r.Said[strconv.Itoa(number)], nil
}

// Comment remembers the comment as GitHub would: the last thing said on
// the PR, by you.
func (c FixtureClient) Comment(org, repo string, number int, body string) error {
	at := c.now().UTC().Format(time.RFC3339)
	return c.change(func(f *Fixture) error {
		r := f.Repos[repo]
		if r == nil {
			return fmt.Errorf("no PR #%d in %s/%s", number, org, repo)
		}
		if r.Said == nil {
			r.Said = map[string][]Said{}
		}
		key := strconv.Itoa(number)
		r.Said[key] = append(r.Said[key], Said{ID: fmt.Sprintf("C_%s_%d_%d", repo, number, len(r.Said[key])+1), Author: f.Viewer, Body: body, At: at})
		for i := range r.PRs {
			if r.PRs[i].Number == number {
				r.PRs[i].UpdatedAt = at
			}
		}
		return nil
	})
}

// FixturePath is where a workspace's GitHub fixture is, given as [github]
// fixture says it: a path, relative to the workspace's root.
func FixturePath(root, fixture string) string {
	if filepath.IsAbs(fixture) {
		return fixture
	}
	return filepath.Join(root, fixture)
}

// ChangedPRs is the repo's open and merged PRs that have changed since:
// each one's latest reviews stand for all its reviews.
func (c FixtureClient) ChangedPRs(org, repo string, since time.Time) ([]PRActivity, error) {
	_, r, err := c.repo(repo)
	if err != nil {
		return nil, err
	}
	var out []PRActivity
	add := func(pr PR, state string) {
		at := pr.UpdatedAt
		if at == "" {
			at = pr.MergedAt
		}
		if !changedSince(at, since) {
			return
		}
		if pr.State == "" {
			pr.State = state
		}
		a := PRActivity{PR: pr, Body: r.Bodies[strconv.Itoa(pr.Number)]}
		for _, l := range pr.Latest {
			a.Reviews = append(a.Reviews, PRReview{Author: l.Author, State: l.State, SubmittedAt: l.SubmittedAt})
		}
		out = append(out, a)
	}
	for _, pr := range r.PRs {
		add(pr, "OPEN")
	}
	for _, pr := range r.Merged {
		add(pr, "MERGED")
	}
	return out, nil
}
