package github

import "time"

// Client abstracts GitHub API access for testability.
type Client interface {
	PRsForRepo(org, repo string) ([]PR, error)
	MergedPRsForRepo(org, repo string) ([]PR, error)
	PRFromNumber(org, repo string, number int) (*PR, error)
	PRDetail(org, repo string, number int) (PRDetailResult, error)
	WorkflowRuns(org, repo, branch string, limit int) ([]WorkflowRun, error)

	// The Reviews page's (reviews.go): who you are, the PRs asking for
	// your review, a repo's stacks, posting a review, and a PR's threads.
	Viewer() (string, error)
	OpenPRs(org, repo string) ([]PR, error)
	ReviewRequested(org string) ([]PRRef, error)
	StacksForRepo(org, repo string) ([]Stack, error)
	SubmitReview(org, repo string, number int, r ReviewSubmission) (string, error)
	ReviewThreads(org, repo string, number int) ([]ReviewThread, error)
	ReplyToThread(org, repo string, number int, threadID, body string) error
	ResolveThread(org, repo string, number int, threadID string, resolved bool) error
	// What's been said on a PR itself (its reviews and its comments), and
	// saying something there.
	Discussion(org, repo string, number int) ([]Said, error)
	Comment(org, repo string, number int, body string) error

	// The Team page's (activity.go): a repo's pull requests that have
	// changed since a time, each with what's happened to it.
	ChangedPRs(org, repo string, since time.Time) ([]PRActivity, error)

	// Opening a pull request, as a step of a mission's plan does through
	// ws (create.go).
	CreatePR(org, repo string, n NewPR) (*PR, error)
	// MarkReady marks a draft pull request ready for review, as a step's
	// pull request is once its checks pass (create.go).
	MarkReady(org, repo string, number int) error
}

// LiveClient calls the real gh CLI.
type LiveClient struct{}

func (LiveClient) PRsForRepo(org, repo string) ([]PR, error) {
	return PRsForRepo(org, repo)
}

func (LiveClient) MergedPRsForRepo(org, repo string) ([]PR, error) {
	return MergedPRsForRepo(org, repo)
}

func (LiveClient) PRFromNumber(org, repo string, number int) (*PR, error) {
	return PRFromNumber(org, repo, number)
}

func (LiveClient) PRDetail(org, repo string, number int) (PRDetailResult, error) {
	return PRDetail(org, repo, number)
}

func (LiveClient) WorkflowRuns(org, repo, branch string, limit int) ([]WorkflowRun, error) {
	return WorkflowRuns(org, repo, branch, limit)
}

func (LiveClient) Viewer() (string, error) {
	cacheDir := CacheDir()
	if login := ReadUserCache(cacheDir); login != "" {
		return login, nil
	}
	login, err := Viewer()
	if err == nil {
		WriteUserCache(cacheDir, login)
	}
	return login, err
}

func (LiveClient) OpenPRs(org, repo string) ([]PR, error) { return OpenPRs(org, repo) }

func (LiveClient) ReviewRequested(org string) ([]PRRef, error) { return ReviewRequested(org) }

func (LiveClient) StacksForRepo(org, repo string) ([]Stack, error) { return StacksForRepo(org, repo) }

func (LiveClient) SubmitReview(org, repo string, number int, r ReviewSubmission) (string, error) {
	return SubmitReview(org, repo, number, r)
}

func (LiveClient) ReviewThreads(org, repo string, number int) ([]ReviewThread, error) {
	return ReviewThreads(org, repo, number)
}

func (LiveClient) ReplyToThread(org, repo string, number int, threadID, body string) error {
	return ReplyToThread(threadID, body)
}

func (LiveClient) ResolveThread(org, repo string, number int, threadID string, resolved bool) error {
	return ResolveThread(threadID, resolved)
}

func (LiveClient) Discussion(org, repo string, number int) ([]Said, error) {
	return Discussion(org, repo, number)
}

func (LiveClient) Comment(org, repo string, number int, body string) error {
	return Comment(org, repo, number, body)
}

func (LiveClient) ChangedPRs(org, repo string, since time.Time) ([]PRActivity, error) {
	return ChangedPRs(org, repo, since)
}
