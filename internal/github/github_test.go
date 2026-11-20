package github

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- deriveStatus tests ---

func TestDeriveStatus_Success(t *testing.T) {
	raw := json.RawMessage(`{
		"statusCheckRollup": [
			{"conclusion": "SUCCESS", "status": "COMPLETED"},
			{"conclusion": "SUCCESS", "status": "COMPLETED"}
		]
	}`)
	if got := deriveStatus(raw); got != "success" {
		t.Errorf("deriveStatus() = %q, want %q", got, "success")
	}
}

func TestDeriveStatus_Failure(t *testing.T) {
	raw := json.RawMessage(`{
		"statusCheckRollup": [
			{"conclusion": "SUCCESS", "status": "COMPLETED"},
			{"conclusion": "FAILURE", "status": "COMPLETED"}
		]
	}`)
	if got := deriveStatus(raw); got != "failure" {
		t.Errorf("deriveStatus() = %q, want %q", got, "failure")
	}
}

func TestDeriveStatus_Pending(t *testing.T) {
	raw := json.RawMessage(`{
		"statusCheckRollup": [
			{"conclusion": "SUCCESS", "status": "COMPLETED"},
			{"conclusion": "", "status": "IN_PROGRESS"}
		]
	}`)
	if got := deriveStatus(raw); got != "pending" {
		t.Errorf("deriveStatus() = %q, want %q", got, "pending")
	}
}

func TestDeriveStatus_FailureBeatsPending(t *testing.T) {
	raw := json.RawMessage(`{
		"statusCheckRollup": [
			{"conclusion": "FAILURE", "status": "COMPLETED"},
			{"conclusion": "", "status": "IN_PROGRESS"}
		]
	}`)
	if got := deriveStatus(raw); got != "failure" {
		t.Errorf("deriveStatus() = %q, want %q (failure should take priority over pending)", got, "failure")
	}
}

func TestDeriveStatus_Empty(t *testing.T) {
	raw := json.RawMessage(`{"statusCheckRollup": []}`)
	if got := deriveStatus(raw); got != "" {
		t.Errorf("deriveStatus() = %q, want empty", got)
	}
}

func TestDeriveStatus_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`not json`)
	if got := deriveStatus(raw); got != "" {
		t.Errorf("deriveStatus() = %q, want empty", got)
	}
}

func TestDeriveStatus_ErrorConclusion(t *testing.T) {
	raw := json.RawMessage(`{
		"statusCheckRollup": [
			{"conclusion": "ERROR", "status": "COMPLETED"}
		]
	}`)
	if got := deriveStatus(raw); got != "failure" {
		t.Errorf("deriveStatus() = %q, want %q", got, "failure")
	}
}

func TestDeriveStatus_CancelledConclusion(t *testing.T) {
	raw := json.RawMessage(`{
		"statusCheckRollup": [
			{"conclusion": "CANCELLED", "status": "COMPLETED"}
		]
	}`)
	if got := deriveStatus(raw); got != "failure" {
		t.Errorf("deriveStatus() = %q, want %q", got, "failure")
	}
}

func TestDeriveStatus_TimedOutConclusion(t *testing.T) {
	raw := json.RawMessage(`{
		"statusCheckRollup": [
			{"conclusion": "TIMED_OUT", "status": "COMPLETED"}
		]
	}`)
	if got := deriveStatus(raw); got != "failure" {
		t.Errorf("deriveStatus() = %q, want %q", got, "failure")
	}
}

func TestDeriveStatus_ActionRequiredConclusion(t *testing.T) {
	raw := json.RawMessage(`{
		"statusCheckRollup": [
			{"conclusion": "ACTION_REQUIRED", "status": "COMPLETED"}
		]
	}`)
	if got := deriveStatus(raw); got != "failure" {
		t.Errorf("deriveStatus() = %q, want %q", got, "failure")
	}
}

func TestDeriveStatus_QueuedPending(t *testing.T) {
	raw := json.RawMessage(`{
		"statusCheckRollup": [
			{"conclusion": "", "status": "QUEUED"}
		]
	}`)
	if got := deriveStatus(raw); got != "pending" {
		t.Errorf("deriveStatus() = %q, want %q", got, "pending")
	}
}

// GitHub lists every run made on the head commit. A pull request whose
// title failed its check, was edited and passed has both runs there: the
// check's result is the later one.
func TestDeriveStatus_ALaterRunReplacesAnEarlierOne(t *testing.T) {
	run := func(workflow, name, conclusion, status, started string) string {
		return `{"__typename": "CheckRun", "workflowName": "` + workflow + `", "name": "` + name + `", "conclusion": "` + conclusion + `", "status": "` + status + `", "startedAt": "` + started + `"}`
	}
	failedTitle := run("PR Title", "PR Title", "FAILURE", "COMPLETED", "2026-11-20T12:55:31Z")
	passedTitle := run("PR Title", "PR Title", "SUCCESS", "COMPLETED", "2026-11-20T12:56:40Z")
	runningTitle := run("PR Title", "PR Title", "", "IN_PROGRESS", "2026-11-20T12:56:40Z")
	passedTest := run("CI", "Test", "SUCCESS", "COMPLETED", "2026-11-20T12:55:33Z")
	for _, c := range []struct {
		name string
		runs []string
		want string
	}{
		{"a failure, then the same check passing", []string{failedTitle, passedTest, passedTitle}, "success"},
		{"listed the other way round", []string{passedTitle, passedTest, failedTitle}, "success"},
		{"a failure, and the same check running again", []string{failedTitle, passedTest, runningTitle}, "pending"},
		{"a pass, then the same check failing", []string{
			run("PR Title", "PR Title", "SUCCESS", "COMPLETED", "2026-11-20T12:55:31Z"), passedTest,
			run("PR Title", "PR Title", "FAILURE", "COMPLETED", "2026-11-20T12:56:40Z")}, "failure"},
		{"the same name in another workflow is another check", []string{
			run("Lint", "check", "FAILURE", "COMPLETED", "2026-11-20T12:55:31Z"),
			run("CI", "check", "SUCCESS", "COMPLETED", "2026-11-20T12:56:40Z")}, "failure"},
	} {
		raw := json.RawMessage(`{"statusCheckRollup": [` + strings.Join(c.runs, ",") + `]}`)
		if got := deriveStatus(raw); got != c.want {
			t.Errorf("%s: deriveStatus() = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDeriveStatus_PendingStatus(t *testing.T) {
	raw := json.RawMessage(`{
		"statusCheckRollup": [
			{"conclusion": "", "status": "PENDING"}
		]
	}`)
	if got := deriveStatus(raw); got != "pending" {
		t.Errorf("deriveStatus() = %q, want %q", got, "pending")
	}
}

// --- extractAuthor tests ---

func TestExtractAuthor_ValidLogin(t *testing.T) {
	raw := json.RawMessage(`{"author":{"login":"octocat"}}`)
	if got := extractAuthor(raw); got != "octocat" {
		t.Errorf("extractAuthor() = %q, want %q", got, "octocat")
	}
}

func TestExtractAuthor_MissingAuthor(t *testing.T) {
	raw := json.RawMessage(`{"title":"some PR"}`)
	if got := extractAuthor(raw); got != "" {
		t.Errorf("extractAuthor() = %q, want empty", got)
	}
}

func TestExtractAuthor_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`not json`)
	if got := extractAuthor(raw); got != "" {
		t.Errorf("extractAuthor() = %q, want empty", got)
	}
}

// --- PR cache function tests ---

func TestWritePRCache_And_ReadPRCache(t *testing.T) {
	cacheDir := t.TempDir()
	prs := []PR{
		{Number: 1, Title: "First PR", HeadRefName: "feature-a"},
		{Number: 2, Title: "Second PR", HeadRefName: "feature-b"},
	}

	WritePRCache(cacheDir, "my-org", "my-repo", prs)

	got, err := ReadPRCache(cacheDir, "my-org", "my-repo")
	if err != nil {
		t.Fatalf("ReadPRCache() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d PRs, want 2", len(got))
	}
	if got[0].Title != "First PR" {
		t.Errorf("PR[0].Title = %q, want %q", got[0].Title, "First PR")
	}
	if got[1].HeadRefName != "feature-b" {
		t.Errorf("PR[1].HeadRefName = %q, want %q", got[1].HeadRefName, "feature-b")
	}
}

func TestWritePRCache_PreservesAllFields(t *testing.T) {
	cacheDir := t.TempDir()
	prs := []PR{
		{
			Number:         42,
			Title:          "My PR",
			HeadRefName:    "my-branch",
			State:          "OPEN",
			ReviewDecision: "APPROVED",
			StatusRollup:   "success",
			URL:            "https://github.com/org/repo/pull/42",
			Author:         "octocat",
		},
	}

	WritePRCache(cacheDir, "org", "repo", prs)

	got, err := ReadPRCache(cacheDir, "org", "repo")
	if err != nil {
		t.Fatalf("ReadPRCache() error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d PRs, want 1", len(got))
	}
	pr := got[0]
	if pr.Number != 42 {
		t.Errorf("Number = %d, want 42", pr.Number)
	}
	if pr.State != "OPEN" {
		t.Errorf("State = %q, want %q", pr.State, "OPEN")
	}
	if pr.ReviewDecision != "APPROVED" {
		t.Errorf("ReviewDecision = %q, want %q", pr.ReviewDecision, "APPROVED")
	}
	if pr.StatusRollup != "success" {
		t.Errorf("StatusRollup = %q, want %q", pr.StatusRollup, "success")
	}
	if pr.URL != "https://github.com/org/repo/pull/42" {
		t.Errorf("URL = %q, want %q", pr.URL, "https://github.com/org/repo/pull/42")
	}
	if pr.Author != "octocat" {
		t.Errorf("Author = %q, want %q", pr.Author, "octocat")
	}
}

func TestReadPRCache_Miss(t *testing.T) {
	cacheDir := t.TempDir()
	got, err := ReadPRCache(cacheDir, "org", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil on cache miss, got %v", got)
	}
}

func TestReadPRCache_CorruptedFile(t *testing.T) {
	cacheDir := t.TempDir()
	path := filepath.Join(cacheDir, "org-repo-prs.json")
	os.WriteFile(path, []byte("not json"), 0644)

	got, err := ReadPRCache(cacheDir, "org", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil on corrupted cache, got %v", got)
	}
}

func TestWritePRCache_CreatesDirectory(t *testing.T) {
	baseDir := t.TempDir()
	cacheDir := filepath.Join(baseDir, "nested", "cache")

	prs := []PR{{Number: 1, Title: "Test"}}
	WritePRCache(cacheDir, "org", "repo", prs)

	got, err := ReadPRCache(cacheDir, "org", "repo")
	if err != nil {
		t.Fatalf("ReadPRCache() error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d PRs, want 1", len(got))
	}
}

func TestWritePRCache_EmptySlice(t *testing.T) {
	cacheDir := t.TempDir()
	WritePRCache(cacheDir, "org", "repo", []PR{})

	got, err := ReadPRCache(cacheDir, "org", "repo")
	if err != nil {
		t.Fatalf("ReadPRCache() error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d PRs, want 0", len(got))
	}
}

// --- User cache function tests ---

func TestWriteUserCache_And_ReadUserCache(t *testing.T) {
	cacheDir := t.TempDir()
	WriteUserCache(cacheDir, "octocat")

	got := ReadUserCache(cacheDir)
	if got != "octocat" {
		t.Errorf("ReadUserCache() = %q, want %q", got, "octocat")
	}
}

func TestReadUserCache_Miss(t *testing.T) {
	cacheDir := t.TempDir()
	got := ReadUserCache(cacheDir)
	if got != "" {
		t.Errorf("ReadUserCache() = %q, want empty on miss", got)
	}
}

func TestCacheDir(t *testing.T) {
	dir := CacheDir()
	if dir == "" {
		t.Error("CacheDir() should not be empty")
	}
	expected := filepath.Join(os.TempDir(), "ws-pr-cache")
	if dir != expected {
		t.Errorf("CacheDir() = %q, want %q", dir, expected)
	}
}

func TestFixtureCreatePR(t *testing.T) {
	path := filepath.Join(t.TempDir(), "github.json")
	os.WriteFile(path, []byte(`{"viewer":"me","repos":{"demo":{"prs":[{"number":4,"headRefName":"other"}],"merged":[{"number":7,"headRefName":"old"}]}}}`), 0o644)
	c := FixtureClient{Path: path}
	pr, err := c.CreatePR("acme", "demo", NewPR{Head: "fix", Base: "main", Title: "Fix it", Body: "Why", Draft: true})
	if err != nil {
		t.Fatal(err)
	}
	if pr.Number != 8 || pr.HeadRefName != "fix" || pr.URL != "https://github.com/acme/demo/pull/8" || pr.Author != "me" || !pr.IsDraft {
		t.Errorf("opened %+v", pr)
	}
	open, _ := c.PRsForRepo("acme", "demo")
	if len(open) != 2 || open[1].Number != 8 || open[1].State != "OPEN" {
		t.Errorf("open now = %+v", open)
	}
	if d, _ := c.PRDetail("acme", "demo", 8); d.Body != "Why" {
		t.Errorf("its body = %q", d.Body)
	}
	if _, err := c.CreatePR("acme", "demo", NewPR{Head: "fix", Base: "main", Title: "Again"}); err == nil || !strings.Contains(err.Error(), "already exists: demo#8") {
		t.Errorf("a second from the same branch: %v", err)
	}
	if pr, err := c.CreatePR("acme", "fresh", NewPR{Head: "x", Base: "main", Title: "First"}); err != nil || pr.Number != 1 {
		t.Errorf("in a repo it had no record of: %+v, %v", pr, err)
	}
}
