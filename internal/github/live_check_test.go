package github

import (
	"os"
	"strings"
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

// TestHeadsLive asks GitHub itself where WS_LIVE_GITHUB's main is, beside
// a repo that isn't there: the one answers, the other's left out.
func TestHeadsLive(t *testing.T) {
	slug := os.Getenv("WS_LIVE_GITHUB")
	if slug == "" {
		t.Skip("WS_LIVE_GITHUB isn't set")
	}
	owner, repo, _ := strings.Cut(slug, "/")
	heads, err := Heads(owner, []string{repo, repo + "-not-a-repo-of-theirs"}, "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(heads) != 1 || len(heads[repo]) != 40 {
		t.Errorf("heads = %v, want %s's main alone", heads, repo)
	}
}
