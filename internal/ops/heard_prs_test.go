package ops

import (
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

// What moved between one Reviews list and the next is heard: a pull
// request opened, your review asked, approved, its checks red or green,
// and one that's left merged or closed as GitHub says. A mission's own
// say what its log doesn't.
func TestHearPRs(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	gh := &github.Stub{PRFromNumberFn: func(org, repo string, n int) (*github.PR, error) {
		switch n {
		case 5:
			return &github.PR{Number: 5, State: "MERGED", MergedAt: "2026-11-26T10:00:00Z"}, nil
		case 6:
			return &github.PR{Number: 6, State: "CLOSED"}, nil
		}
		return &github.PR{Number: n, State: "OPEN"}, nil
	}}
	ws := &workspace.Workspace{Root: root, RepoNames: []string{"api"}, Org: "acme", DefaultBranch: "main"}
	w := New(&config.Config{}, ws, gh, Options{})
	list := func(prs ...pulls.PR) *pulls.List {
		l := &pulls.List{Me: "me"}
		sec := pulls.Section{Key: "others"}
		for _, pr := range prs {
			pr := pr
			sec.Rows = append(sec.Rows, pulls.Row{Key: prKey(pr), PR: &pr})
		}
		l.Sections = []pulls.Section{sec}
		return l
	}
	mission := &pulls.MissionRef{Slug: "cache"}
	prev := list(
		pulls.PR{Repo: "api", Number: 1, Title: "Idempotency", Author: "brendan", Checks: "pending"},
		pulls.PR{Repo: "api", Number: 2, Title: "Retry", Author: "ana", Decision: "review_required", Checks: "success"},
		pulls.PR{Repo: "api", Number: 3, Title: "Cache", Author: "me", Checks: "pending", Mission: mission},
		pulls.PR{Repo: "api", Number: 5, Title: "Merged one", Author: "ana"},
		pulls.PR{Repo: "api", Number: 6, Title: "Closed one", Author: "brendan"},
	)
	next := list(
		pulls.PR{Repo: "api", Number: 1, Title: "Idempotency", Author: "brendan", Checks: "failure", Asked: "you"},
		pulls.PR{Repo: "api", Number: 2, Title: "Retry", Author: "ana", Decision: "approved", Checks: "success"},
		pulls.PR{Repo: "api", Number: 3, Title: "Cache", Author: "me", Checks: "failure", Mission: mission},
		pulls.PR{Repo: "api", Number: 4, Title: "New one", Author: "ana", Checks: "pending"},
	)
	w.hearPRs(prev, next)
	got, err := events.List(root, events.Filter{Origin: events.Heard, Only: true})
	if err != nil {
		t.Fatal(err)
	}
	said := map[string]string{}
	for _, e := range got {
		said[e.Kind+" "+e.PR] = e.By
	}
	want := map[string]string{
		"pr.review_requested api#1": events.ByGitHub,
		"pr.checks_failed api#1":    "person:brendan",
		"pr.approved api#2":         events.ByGitHub,
		"pr.opened api#4":           "person:ana",
		"pr.merged api#5":           "person:ana",
		"pr.closed api#6":           "person:brendan",
	}
	for k, by := range want {
		if said[k] != by {
			t.Errorf("%s: by %q, want %q", k, said[k], by)
		}
	}
	if len(said) != len(want) {
		t.Errorf("heard %v, want only %v (a mission's checks failing is its log's)", said, want)
	}

	// The same list again: nothing moved.
	w.hearPRs(next, next)
	if again, _ := events.List(root, events.Filter{Origin: events.Heard, Only: true}); len(again) != len(got) {
		t.Errorf("heard %d, then %d with nothing moved", len(got), len(again))
	}
}
