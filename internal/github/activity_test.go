package github

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseChangedPRs(t *testing.T) {
	data := []byte(`{"data":{"repository":{"pullRequests":{"pageInfo":{"hasNextPage":true,"endCursor":"c1"},"nodes":[
	  {"number":12,"title":"feat: cache tokens","url":"https://github.com/acme/api/pull/12","state":"MERGED","isDraft":false,
	   "headRefName":"token-cache","baseRefName":"main","additions":40,"deletions":2,"changedFiles":3,
	   "createdAt":"2026-11-08T09:00:00Z","updatedAt":"2026-11-09T09:00:00Z","mergedAt":"2026-11-09T09:00:00Z","closedAt":"2026-11-09T09:00:00Z",
	   "body":"Caches tokens.","author":{"__typename":"User","login":"ana-acme","name":"Ana Diaz"},"mergedBy":{"login":"ben-acme"},"mergeCommit":{"oid":"abc123"},
	   "reviews":{"nodes":[{"author":{"__typename":"User","login":"ben-acme","name":"Ben Okoro"},"state":"APPROVED","submittedAt":"2026-11-08T15:00:00Z"},
	                        {"author":{"__typename":"Bot","login":"lint-bot"},"state":"COMMENTED","submittedAt":"2026-11-08T15:01:00Z"}]},
	   "timelineItems":{"nodes":[{"createdAt":"2026-11-08T10:00:00Z","actor":{"login":"ana-acme"}}]}},
	  {"number":13,"title":"chore: bump","url":"u","state":"OPEN","isDraft":true,"headRefName":"bump","baseRefName":"main",
	   "additions":1,"deletions":1,"changedFiles":1,"createdAt":"2026-11-07T09:00:00Z","updatedAt":"2026-11-07T09:00:00Z","mergedAt":null,"closedAt":null,
	   "author":{"__typename":"Bot","login":"dependabot"},"mergedBy":null,"mergeCommit":null,"reviews":{"nodes":[]},"timelineItems":{"nodes":[]}}
	]}}}}`)
	prs, next, err := parseChangedPRs(data)
	if err != nil || next != "c1" || len(prs) != 2 {
		t.Fatalf("parsed %d, next %q, %v", len(prs), next, err)
	}
	a := prs[0]
	if a.Number != 12 || a.Author != "ana-acme" || a.Bot || a.MergedBy != "ben-acme" || a.MergeCommit != "abc123" || a.MergedAt == "" || a.HeadRefName != "token-cache" || a.Additions != 40 {
		t.Errorf("merged = %+v", a)
	}
	if a.ReadyAt != "2026-11-08T10:00:00Z" || a.ReadyBy != "ana-acme" {
		t.Errorf("ready = %q by %q", a.ReadyAt, a.ReadyBy)
	}
	if a.Body != "Caches tokens." || a.AuthorName != "Ana Diaz" || a.Reviews[0].Name != "Ben Okoro" {
		t.Errorf("description and names = %q, %q, %q", a.Body, a.AuthorName, a.Reviews[0].Name)
	}
	if len(a.Reviews) != 2 || a.Reviews[0].State != "APPROVED" || a.Reviews[0].Author != "ben-acme" || !a.Reviews[1].Bot {
		t.Errorf("reviews = %+v", a.Reviews)
	}
	if b := prs[1]; !b.Bot || !b.IsDraft || b.MergedAt != "" || b.MergedBy != "" || b.ReadyAt != "" {
		t.Errorf("open = %+v", b)
	}
	if _, _, err := parseChangedPRs([]byte(`{"errors":[{"message":"nope"}]}`)); err == nil {
		t.Error("an error from GitHub wasn't one")
	}
}

func TestFixtureChangedPRs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "github.json")
	os.WriteFile(path, []byte(`{"repos":{"demo":{
	  "prs":[{"number":3,"title":"open","authorLogin":"ana","updatedAt":"2026-11-09T09:00:00Z","latest":[{"author":"ben","state":"APPROVED","submittedAt":"2026-11-09T08:00:00Z"}]},
	         {"number":2,"title":"old","authorLogin":"ana","updatedAt":"2026-10-01T09:00:00Z"}],
	  "merged":[{"number":1,"title":"merged","authorLogin":"cal","mergedAt":"2026-11-08T09:00:00Z"}]}}}`), 0o644)
	c := FixtureClient{Path: path}
	got, err := c.ChangedPRs("acme", "demo", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(got) != 2 {
		t.Fatalf("changed = %+v, %v", got, err)
	}
	if got[0].Number != 3 || got[0].State != "OPEN" || len(got[0].Reviews) != 1 || got[0].Reviews[0].Author != "ben" {
		t.Errorf("open = %+v", got[0])
	}
	if got[1].Number != 1 || got[1].State != "MERGED" {
		t.Errorf("merged = %+v", got[1])
	}
}
