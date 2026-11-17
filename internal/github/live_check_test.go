package github

import (
	"os"
	"testing"
	"time"
)

// TestChangedPRsLive asks GitHub itself, when WS_LIVE_GITHUB names a
// public repo (owner/name): the query is in GitHub's words, which only
// GitHub can check.
func TestChangedPRsLive(t *testing.T) {
	slug := os.Getenv("WS_LIVE_GITHUB")
	if slug == "" {
		t.Skip("WS_LIVE_GITHUB isn't set")
	}
	var owner, repo string
	for i := range slug {
		if slug[i] == '/' {
			owner, repo = slug[:i], slug[i+1:]
		}
	}
	prs, err := ChangedPRs(owner, repo, time.Now().AddDate(0, 0, -30))
	if err != nil {
		t.Fatal(err)
	}
	named := 0
	for _, pr := range prs {
		if pr.AuthorName != "" {
			named++
		}
	}
	t.Logf("%d pull requests changed in 30 days, %d with their author's name", len(prs), named)
}
