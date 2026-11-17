package github

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseOpenPRs(t *testing.T) {
	data := `{"data": {"repository": {"pullRequests": {
		"pageInfo": {"hasNextPage": true, "endCursor": "c2"},
		"nodes": [
			{"number": 89, "title": "Key by tenant", "url": "u", "state": "OPEN", "isDraft": true, "reviewDecision": "REVIEW_REQUIRED",
			 "headRefName": "tenant", "headRefOid": "abc", "baseRefName": "storage", "isCrossRepository": true,
			 "additions": 12, "deletions": 3, "changedFiles": 2, "createdAt": "2026-11-09T10:00:00Z", "updatedAt": "2026-11-10T10:00:00Z",
			 "author": {"__typename": "User", "login": "alice"},
			 "reviewRequests": {"nodes": [{"requestedReviewer": {"__typename": "User", "login": "me"}}, {"requestedReviewer": {"__typename": "Team", "slug": "backend"}}, {"requestedReviewer": null}]},
			 "latestReviews": {"nodes": [{"author": {"login": "carol"}, "state": "APPROVED", "submittedAt": "2026-11-10T09:00:00Z", "commit": {"oid": "abc"}}]},
			 "commits": {"nodes": [{"commit": {"statusCheckRollup": {"state": "FAILURE"}}}]}},
			{"number": 90, "title": "Bump", "author": {"__typename": "Bot", "login": "dependabot"}, "headRefName": "bump", "baseRefName": "main",
			 "reviewRequests": {"nodes": []}, "latestReviews": {"nodes": []}, "commits": {"nodes": [{"commit": {"statusCheckRollup": null}}]}}
		]}}}}`
	prs, next, err := parseOpenPRs([]byte(data))
	if err != nil || next != "c2" || len(prs) != 2 {
		t.Fatalf("prs = %+v, next %q, %v", prs, next, err)
	}
	pr := prs[0]
	if pr.Number != 89 || pr.Title != "Key by tenant" || pr.Author != "alice" || pr.Bot || !pr.IsDraft || pr.BaseRefName != "storage" || pr.HeadRefName != "tenant" || pr.HeadRefOid != "abc" ||
		pr.Additions != 12 || pr.Deletions != 3 || pr.ChangedFiles != 2 || !pr.IsCrossRepository || pr.ReviewDecision != "REVIEW_REQUIRED" || pr.StatusRollup != "failure" || pr.UpdatedAt != "2026-11-10T10:00:00Z" {
		t.Errorf("pr = %+v", pr)
	}
	if len(pr.Requested) != 2 || pr.Requested[0].Login != "me" || pr.Requested[1].Team != "backend" {
		t.Errorf("requested = %+v", pr.Requested)
	}
	if len(pr.Latest) != 1 || pr.Latest[0] != (LatestReview{Author: "carol", State: "APPROVED", SubmittedAt: "2026-11-10T09:00:00Z", Commit: "abc"}) {
		t.Errorf("latest = %+v", pr.Latest)
	}
	if bot := prs[1]; !bot.Bot || bot.Author != "dependabot" || bot.StatusRollup != "" {
		t.Errorf("the bot's = %+v", bot)
	}
	// It survives the PR cache and the fixture, which keep it as JSON.
	kept, _ := json.Marshal(pr)
	var back PR
	if err := json.Unmarshal(kept, &back); err != nil || len(back.Requested) != 2 || len(back.Latest) != 1 || back.Author != "alice" {
		t.Errorf("after the cache: %+v, %v", back, err)
	}
	if _, _, err := parseOpenPRs([]byte(`{"errors": [{"message": "Could not resolve to a Repository"}]}`)); err == nil {
		t.Error("an error from GitHub read as no PRs")
	}
}

func TestParseStacks(t *testing.T) {
	page := `[{"id": 1, "number": 3, "base": {"ref": "main"}, "open": true, "pull_requests": [
		{"number": 88, "state": "closed", "draft": false, "merged_at": "2026-11-09T10:00:00Z", "head": {"ref": "storage", "sha": "a1"}},
		{"number": 89, "state": "open", "draft": false, "merged_at": null, "head": {"ref": "tenant", "sha": "b2"}}]},
		{"id": 2, "number": 4, "base": {"ref": "main"}, "open": false, "pull_requests": []}]`
	stacks, err := parseStacks([]byte(page))
	if err != nil || len(stacks) != 2 {
		t.Fatalf("stacks = %+v, %v", stacks, err)
	}
	s := stacks[0]
	if s.Number != 3 || s.Base != "main" || !s.Open || len(s.PRs) != 2 || !s.PRs[0].Merged || s.PRs[1].Merged || s.PRs[1].HeadRef != "tenant" || s.PRs[1].HeadSHA != "b2" {
		t.Errorf("stack = %+v", s)
	}
	if stacks[1].Open {
		t.Errorf("a stack that's all merged: %+v", stacks[1])
	}
	if _, err := parseStacks([]byte(`{"message": "Not Found"}`)); err == nil {
		t.Error("something that isn't a list of stacks parsed")
	}
}

func TestParseReviewThreads(t *testing.T) {
	data := `{"data": {"repository": {"pullRequest": {"reviewThreads": {
		"pageInfo": {"hasNextPage": true, "endCursor": "c1"},
		"nodes": [{"id": "T1", "isResolved": true, "isOutdated": false, "path": "a.go", "line": 12, "startLine": 10, "originalLine": 12, "diffSide": "RIGHT",
			"comments": {"nodes": [{"author": {"login": "carol"}, "body": "question: why?", "createdAt": "2026-11-10T09:00:00Z", "url": "u", "diffHunk": "@@", "commit": {"oid": "c"}, "originalCommit": {"oid": "o"}}]}}]}}}}}`
	threads, next, err := parseReviewThreads([]byte(data))
	if err != nil || next != "c1" || len(threads) != 1 {
		t.Fatalf("threads = %+v, next %q, %v", threads, next, err)
	}
	th := threads[0]
	if th.ID != "T1" || !th.Resolved || th.Line != 12 || th.StartLine != 10 || th.Side != "RIGHT" || len(th.Comments) != 1 ||
		th.Comments[0] != (ThreadComment{Author: "carol", Body: "question: why?", At: "2026-11-10T09:00:00Z", URL: "u", DiffHunk: "@@", Commit: "c", OriginalCommit: "o"}) {
		t.Errorf("thread = %+v", th)
	}
}

func TestGHError(t *testing.T) {
	out := []byte(`{"message": "Unprocessable Entity", "errors": ["Can not approve your own pull request"]}`)
	if got := ghError(out, "gh: Unprocessable Entity (HTTP 422)", os.ErrInvalid); got != "Unprocessable Entity: Can not approve your own pull request" {
		t.Errorf("ghError = %q", got)
	}
	if got := ghError(nil, "gh: Not Found (HTTP 404)\n", os.ErrInvalid); got != "gh: Not Found (HTTP 404)" {
		t.Errorf("ghError = %q", got)
	}
}

// The fixture answers for GitHub from a file, and remembers what's
// submitted as GitHub would.
func TestFixtureClient(t *testing.T) {
	path := filepath.Join(t.TempDir(), "github.json")
	os.WriteFile(path, []byte(`{
		"viewer": "me",
		"requested": [{"repo": "api", "number": 2}],
		"repos": {"api": {
			"prs": [
				{"number": 1, "title": "One", "authorLogin": "alice", "headRefName": "one", "requested": [{"login": "me"}]},
				{"number": 2, "title": "Two", "authorLogin": "bob", "headRefName": "two"},
				{"number": 3, "title": "Mine", "authorLogin": "me", "headRefName": "three"}],
			"stacks": [{"number": 1, "base": "main", "open": true, "prs": [{"number": 1}, {"number": 2}]}],
			"bodies": {"1": "The first."}
		}}
	}`), 0o644)
	at := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	c := FixtureClient{Path: path, Now: func() time.Time { return at }}
	var _ Client = c

	if me, err := c.Viewer(); err != nil || me != "me" {
		t.Fatalf("viewer = %q, %v", me, err)
	}
	prs, err := c.PRsForRepo("acme", "api")
	if err != nil || len(prs) != 3 || prs[0].State != "OPEN" {
		t.Fatalf("prs = %+v, %v", prs, err)
	}
	if other, _ := c.PRsForRepo("acme", "web"); len(other) != 0 {
		t.Errorf("a repo the fixture doesn't have: %+v", other)
	}
	if asked, _ := c.ReviewRequested("acme"); len(asked) != 2 {
		t.Errorf("asked = %+v", asked)
	}
	if stacks, _ := c.StacksForRepo("acme", "api"); len(stacks) != 1 || len(stacks[0].PRs) != 2 {
		t.Errorf("stacks = %+v", stacks)
	}
	if d, err := c.PRDetail("acme", "api", 1); err != nil || d.Body != "The first." || d.Title != "One" {
		t.Errorf("detail = %+v, %v", d, err)
	}

	url, err := c.SubmitReview("acme", "api", 1, ReviewSubmission{Commit: "abc", Event: EventRequestChanges, Body: "Close.",
		Comments: []ReviewComment{{Path: "a.go", Body: "issue (blocking): no", Line: 3, Side: "RIGHT"}}})
	if err != nil || url == "" {
		t.Fatalf("submit: %q, %v", url, err)
	}
	prs, _ = c.PRsForRepo("acme", "api")
	if len(prs[0].Requested) != 0 || len(prs[0].Latest) != 1 || prs[0].Latest[0].State != "CHANGES_REQUESTED" || prs[0].Latest[0].Commit != "abc" {
		t.Errorf("after submitting: %+v", prs[0])
	}
	threads, _ := c.ReviewThreads("acme", "api", 1)
	if len(threads) != 1 || threads[0].Line != 3 || threads[0].Comments[0].Author != "me" {
		t.Fatalf("threads = %+v", threads)
	}
	if err := c.ReplyToThread("acme", "api", 1, threads[0].ID, "Fixed?"); err != nil {
		t.Fatal(err)
	}
	if err := c.ResolveThread("acme", "api", 1, threads[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if threads, _ = c.ReviewThreads("acme", "api", 1); len(threads[0].Comments) != 2 || !threads[0].Resolved {
		t.Errorf("after replying and resolving: %+v", threads[0])
	}
	if err := c.ResolveThread("acme", "api", 1, "nope", true); err == nil {
		t.Error("resolved a thread that isn't there")
	}
	// GitHub takes no verdict on your own PR.
	if _, err := c.SubmitReview("acme", "api", 3, ReviewSubmission{Event: EventApprove}); err == nil {
		t.Error("approved your own PR")
	}
	if _, err := c.SubmitReview("acme", "api", 9, ReviewSubmission{Event: EventApprove}); err == nil {
		t.Error("reviewed a PR that isn't open")
	}
}
