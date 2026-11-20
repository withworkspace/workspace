package github

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gh "github.com/cli/go-gh/v2"
)

// PR represents a pull request associated with a branch.
type PR struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	HeadRefName    string `json:"headRefName"`
	State          string `json:"state"`
	ReviewDecision string `json:"reviewDecision"`
	StatusRollup   string `json:"statusRollup"` // derived from statusCheckRollup
	URL            string `json:"url"`
	Author         string `json:"authorLogin"` // GitHub login; extracted from nested author.login via extractAuthor(); tag avoids collision with gh's "author" object
	MergedAt       string `json:"mergedAt"`
	// Mergeable is GitHub's word on whether it merges cleanly: MERGEABLE,
	// CONFLICTING, or UNKNOWN while it's working that out.
	Mergeable string `json:"mergeable,omitempty"`

	// What the Reviews page needs besides: OpenPRs fills these in, with a
	// query of its own (reviews.go).
	IsDraft           bool   `json:"isDraft,omitempty"`
	BaseRefName       string `json:"baseRefName,omitempty"`
	HeadRefOid        string `json:"headRefOid,omitempty"`
	Additions         int    `json:"additions,omitempty"`
	Deletions         int    `json:"deletions,omitempty"`
	ChangedFiles      int    `json:"changedFiles,omitempty"`
	CreatedAt         string `json:"createdAt,omitempty"`
	UpdatedAt         string `json:"updatedAt,omitempty"`
	IsCrossRepository bool   `json:"isCrossRepository,omitempty"` // its branch is in a fork
	// Bot: a bot opened it (Dependabot and the like).
	Bot bool `json:"authorIsBot,omitempty"`
	// Requested are who's asked to review it: people by login, teams by
	// slug.
	Requested []ReviewRequest `json:"requested,omitempty"`
	// Latest is the latest review by each person.
	Latest []LatestReview `json:"latest,omitempty"`
}

// ReviewRequest is someone asked to review a PR: a person, or a team.
type ReviewRequest struct {
	Login string `json:"login,omitempty"`
	Team  string `json:"team,omitempty"` // its slug
}

// LatestReview is a person's latest review of a PR.
type LatestReview struct {
	Author string `json:"author"`
	// State is APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED or PENDING.
	State       string `json:"state"`
	SubmittedAt string `json:"submittedAt,omitempty"`
	Commit      string `json:"commit,omitempty"` // the commit reviewed
}

// WorkflowRun represents a GitHub Actions workflow run.
type WorkflowRun struct {
	Name       string `json:"name"`
	Status     string `json:"status"`     // completed, in_progress, queued
	Conclusion string `json:"conclusion"` // success, failure, cancelled, etc.
	CreatedAt  string `json:"createdAt"`
}

// CheckRun represents a single CI check.
type CheckRun struct {
	Name       string `json:"name"`
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
}

func repoSlug(org, repo string) string {
	return org + "/" + repo
}

// PRsForRepo returns open PRs for a given org/repo.
func PRsForRepo(org, repo string) ([]PR, error) {
	fullRepo := repoSlug(org, repo)
	stdOut, _, err := gh.Exec(
		"pr", "list",
		"--repo", fullRepo,
		"--state", "open",
		"--json", "number,title,headRefName,state,reviewDecision,url,statusCheckRollup,author,isDraft,mergeable,headRefOid",
	)
	if err != nil {
		return nil, fmt.Errorf("gh pr list for %s: %w", fullRepo, err)
	}

	var raw []json.RawMessage
	if err := json.Unmarshal(stdOut.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}

	var prs []PR
	for _, r := range raw {
		var pr PR
		if err := json.Unmarshal(r, &pr); err != nil {
			continue
		}
		pr.StatusRollup = deriveStatus(r)
		pr.Author = extractAuthor(r)
		prs = append(prs, pr)
	}
	return prs, nil
}

// MergedPRsForRepo returns recently merged PRs for a given org/repo.
func MergedPRsForRepo(org, repo string) ([]PR, error) {
	fullRepo := repoSlug(org, repo)
	stdOut, _, err := gh.Exec(
		"pr", "list",
		"--repo", fullRepo,
		"--state", "merged",
		// headRefOid is the commit it merged at: archiving a mission burns a
		// capsule still there, and keeps one that's moved since.
		"--json", "number,title,headRefName,state,reviewDecision,url,statusCheckRollup,author,mergedAt,headRefOid",
	)
	if err != nil {
		return nil, fmt.Errorf("gh pr list --state merged for %s: %w", fullRepo, err)
	}

	var raw []json.RawMessage
	if err := json.Unmarshal(stdOut.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}

	var prs []PR
	for _, r := range raw {
		var pr PR
		if err := json.Unmarshal(r, &pr); err != nil {
			continue
		}
		pr.StatusRollup = deriveStatus(r)
		pr.Author = extractAuthor(r)
		prs = append(prs, pr)
	}
	return prs, nil
}

// WorkflowRuns returns recent workflow runs for a repo/branch.
func WorkflowRuns(org, repo, branch string, limit int) ([]WorkflowRun, error) {
	fullRepo := repoSlug(org, repo)
	stdOut, _, err := gh.Exec(
		"run", "list",
		"--repo", fullRepo,
		"--branch", branch,
		"--limit", fmt.Sprintf("%d", limit),
		"--json", "name,status,conclusion,createdAt",
	)
	if err != nil {
		return nil, fmt.Errorf("gh run list for %s: %w", fullRepo, err)
	}

	var runs []WorkflowRun
	if err := json.Unmarshal(stdOut.Bytes(), &runs); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}
	return runs, nil
}

// PRFromNumber fetches a specific PR by number.
func PRFromNumber(org, repo string, number int) (*PR, error) {
	fullRepo := repoSlug(org, repo)
	stdOut, _, err := gh.Exec(
		"pr", "view",
		fmt.Sprintf("%d", number),
		"--repo", fullRepo,
		"--json", "number,title,headRefName,state,reviewDecision,url,statusCheckRollup,author,isDraft,mergeable,headRefOid,mergedAt",
	)
	if err != nil {
		return nil, fmt.Errorf("gh pr view %d for %s: %w", number, fullRepo, err)
	}

	var pr PR
	raw := stdOut.Bytes()
	if err := json.Unmarshal(raw, &pr); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}
	pr.StatusRollup = deriveStatus(json.RawMessage(raw))
	pr.Author = extractAuthor(json.RawMessage(raw))
	return &pr, nil
}

// PRDetailResult holds all data returned by PRDetail.
type PRDetailResult struct {
	Title   string
	Body    string
	Checks  []CheckRun
	Commits []string
}

// PRDetail fetches the body, check runs, and recent commits for a PR.
func PRDetail(org, repo string, number int) (PRDetailResult, error) {
	fullRepo := repoSlug(org, repo)
	stdOut, _, err := gh.Exec(
		"pr", "view",
		fmt.Sprintf("%d", number),
		"--repo", fullRepo,
		"--json", "title,body,statusCheckRollup,commits",
	)
	if err != nil {
		return PRDetailResult{}, fmt.Errorf("gh pr view %d for %s: %w", number, fullRepo, err)
	}

	var data struct {
		Title             string     `json:"title"`
		Body              string     `json:"body"`
		StatusCheckRollup []CheckRun `json:"statusCheckRollup"`
		Commits           []struct {
			MessageHeadline string `json:"messageHeadline"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(stdOut.Bytes(), &data); err != nil {
		return PRDetailResult{}, fmt.Errorf("parsing gh output: %w", err)
	}

	commits := make([]string, len(data.Commits))
	for i, c := range data.Commits {
		commits[i] = c.MessageHeadline
	}

	return PRDetailResult{
		Title:   data.Title,
		Body:    data.Body,
		Checks:  data.StatusCheckRollup,
		Commits: commits,
	}, nil
}

// CacheDir returns the directory used for PR cache files.
func CacheDir() string {
	return filepath.Join(os.TempDir(), "ws-pr-cache")
}

// WritePRCache writes PR data to a cache file atomically.
// Errors are silently swallowed (best-effort).
func WritePRCache(cacheDir, org, repo string, prs []PR) {
	data, err := json.Marshal(prs)
	if err != nil {
		return
	}
	_ = os.MkdirAll(cacheDir, 0o755)
	tmp := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-prs.tmp", org, repo))
	target := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-prs.json", org, repo))
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, target)
}

// ReadPRCache reads cached PR data for a repo.
// Returns nil, nil on miss or corruption.
func ReadPRCache(cacheDir, org, repo string) ([]PR, error) {
	path := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-prs.json", org, repo))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	var prs []PR
	if err := json.Unmarshal(data, &prs); err != nil {
		return nil, nil
	}
	return prs, nil
}

// WriteBranchCache writes worktree-to-branch mappings to a cache file.
func WriteBranchCache(cacheDir, org, repo string, branches map[string]string) {
	data, err := json.Marshal(branches)
	if err != nil {
		return
	}
	_ = os.MkdirAll(cacheDir, 0o755)
	tmp := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-branches.tmp", org, repo))
	target := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-branches.json", org, repo))
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, target)
}

// ReadBranchCache reads cached worktree-to-branch mappings.
// Returns nil on miss or corruption.
func ReadBranchCache(cacheDir, org, repo string) map[string]string {
	path := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-branches.json", org, repo))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var branches map[string]string
	if err := json.Unmarshal(data, &branches); err != nil {
		return nil
	}
	return branches
}

// WriteUserCache writes the GitHub username to a cache file.
func WriteUserCache(cacheDir, login string) {
	_ = os.MkdirAll(cacheDir, 0o755)
	path := filepath.Join(cacheDir, "gh-user.txt")
	_ = os.WriteFile(path, []byte(login), 0o644)
}

// ReadUserCache reads the cached GitHub username. Returns "" on miss.
func ReadUserCache(cacheDir string) string {
	path := filepath.Join(cacheDir, "gh-user.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// extractAuthor extracts the author login from raw PR JSON.
func extractAuthor(raw json.RawMessage) string {
	var data struct {
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return ""
	}
	return data.Author.Login
}

// rollupRun is one entry of gh's statusCheckRollup: a check's run, or a
// commit status (which has a context where a run has a name).
type rollupRun struct {
	Name         string `json:"name"`
	WorkflowName string `json:"workflowName"`
	Context      string `json:"context"`
	Conclusion   string `json:"conclusion"`
	Status       string `json:"status"`
	StartedAt    string `json:"startedAt"`
}

// check is which check a run is of: its workflow and name, or a status's
// context. "" when it says neither, and can't be told from another.
func (r rollupRun) check() string {
	switch {
	case r.Name != "":
		return r.WorkflowName + "\x00" + r.Name
	case r.Context != "":
		return "status\x00" + r.Context
	}
	return ""
}

// latestRuns is each check's latest run, in the order the checks first
// come. GitHub lists every run made on the head commit, not the latest of
// each check: one whose workflow ran again (the pull request was edited,
// say, and its title checked a second time) is there twice. A run that a
// later one replaced isn't the check's result any more, which is how gh
// pr checks and GitHub's own page have it. Runs started at the same time
// keep the one listed last.
func latestRuns(runs []rollupRun) []rollupRun {
	at := map[string]int{}
	var out []rollupRun
	for _, r := range runs {
		key := r.check()
		i, seen := at[key]
		switch {
		case key == "" || !seen:
			if key != "" {
				at[key] = len(out)
			}
			out = append(out, r)
		case r.StartedAt >= out[i].StartedAt:
			out[i] = r
		}
	}
	return out
}

// deriveStatus extracts an overall status from statusCheckRollup JSON:
// failure where a check's latest run failed, pending where one's still
// running, success otherwise, and "" with no checks. A failure that a
// later run of the same check replaced doesn't count: it would hold the
// pull request at failing for as long as that commit is its head.
func deriveStatus(raw json.RawMessage) string {
	var data struct {
		StatusCheckRollup []rollupRun `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal(raw, &data); err != nil || len(data.StatusCheckRollup) == 0 {
		return ""
	}

	hasFailure := false
	hasPending := false
	for _, check := range latestRuns(data.StatusCheckRollup) {
		switch check.Conclusion {
		case "FAILURE", "ERROR", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED":
			hasFailure = true
		case "":
			if check.Status == "IN_PROGRESS" || check.Status == "QUEUED" || check.Status == "PENDING" {
				hasPending = true
			}
		}
	}

	if hasFailure {
		return "failure"
	}
	if hasPending {
		return "pending"
	}
	return "success"
}
