package github

import (
	"errors"
	"time"
)

// Stub is a test double for Client: each call answers from its function,
// or with nothing.
type Stub struct {
	PRsForRepoFn       func(org, repo string) ([]PR, error)
	MergedPRsForRepoFn func(org, repo string) ([]PR, error)
	PRFromNumberFn     func(org, repo string, number int) (*PR, error)
	PRDetailFn         func(org, repo string, number int) (PRDetailResult, error)
	WorkflowRunsFn     func(org, repo, branch string, limit int) ([]WorkflowRun, error)

	// The Reviews page's. Me is who Viewer answers ("" for "you").
	Me string
	// OpenPRsFn answers OpenPRs; without it, PRsForRepoFn does.
	OpenPRsFn         func(org, repo string) ([]PR, error)
	ReviewRequestedFn func(org string) ([]PRRef, error)
	StacksForRepoFn   func(org, repo string) ([]Stack, error)
	SubmitReviewFn    func(org, repo string, number int, r ReviewSubmission) (string, error)
	ReviewThreadsFn   func(org, repo string, number int) ([]ReviewThread, error)
	ReplyToThreadFn   func(org, repo string, number int, threadID, body string) error
	ResolveThreadFn   func(org, repo string, number int, threadID string, resolved bool) error
	DiscussionFn      func(org, repo string, number int) ([]Said, error)
	CommentFn         func(org, repo string, number int, body string) error

	// The Team page's.
	ChangedPRsFn func(org, repo string, since time.Time) ([]PRActivity, error)

	// CreatePRFn opens a pull request; without it, there's no GitHub to
	// open one on.
	CreatePRFn func(org, repo string, n NewPR) (*PR, error)
	// MarkReadyFn marks a draft ready; without it, there's no GitHub.
	MarkReadyFn func(org, repo string, number int) error

	// HeadsFn says where repos' branches are; without it, GitHub can't
	// say, and each repo's fetched instead.
	HeadsFn func(org string, repos []string, branch string) (map[string]string, error)
}

func (s *Stub) PRsForRepo(org, repo string) ([]PR, error) {
	if s.PRsForRepoFn != nil {
		return s.PRsForRepoFn(org, repo)
	}
	return nil, nil
}

func (s *Stub) MergedPRsForRepo(org, repo string) ([]PR, error) {
	if s.MergedPRsForRepoFn != nil {
		return s.MergedPRsForRepoFn(org, repo)
	}
	return nil, nil
}

func (s *Stub) PRFromNumber(org, repo string, number int) (*PR, error) {
	if s.PRFromNumberFn != nil {
		return s.PRFromNumberFn(org, repo, number)
	}
	return nil, nil
}

func (s *Stub) PRDetail(org, repo string, number int) (PRDetailResult, error) {
	if s.PRDetailFn != nil {
		return s.PRDetailFn(org, repo, number)
	}
	return PRDetailResult{}, nil
}

func (s *Stub) WorkflowRuns(org, repo, branch string, limit int) ([]WorkflowRun, error) {
	if s.WorkflowRunsFn != nil {
		return s.WorkflowRunsFn(org, repo, branch, limit)
	}
	return nil, nil
}

func (s *Stub) Viewer() (string, error) {
	if s.Me != "" {
		return s.Me, nil
	}
	return "you", nil
}

func (s *Stub) OpenPRs(org, repo string) ([]PR, error) {
	if s.OpenPRsFn != nil {
		return s.OpenPRsFn(org, repo)
	}
	return s.PRsForRepo(org, repo)
}

func (s *Stub) ReviewRequested(org string) ([]PRRef, error) {
	if s.ReviewRequestedFn != nil {
		return s.ReviewRequestedFn(org)
	}
	return nil, nil
}

func (s *Stub) StacksForRepo(org, repo string) ([]Stack, error) {
	if s.StacksForRepoFn != nil {
		return s.StacksForRepoFn(org, repo)
	}
	return nil, nil
}

func (s *Stub) SubmitReview(org, repo string, number int, r ReviewSubmission) (string, error) {
	if s.SubmitReviewFn != nil {
		return s.SubmitReviewFn(org, repo, number, r)
	}
	return "", nil
}

func (s *Stub) ReviewThreads(org, repo string, number int) ([]ReviewThread, error) {
	if s.ReviewThreadsFn != nil {
		return s.ReviewThreadsFn(org, repo, number)
	}
	return nil, nil
}

func (s *Stub) ReplyToThread(org, repo string, number int, threadID, body string) error {
	if s.ReplyToThreadFn != nil {
		return s.ReplyToThreadFn(org, repo, number, threadID, body)
	}
	return nil
}

func (s *Stub) ResolveThread(org, repo string, number int, threadID string, resolved bool) error {
	if s.ResolveThreadFn != nil {
		return s.ResolveThreadFn(org, repo, number, threadID, resolved)
	}
	return nil
}

func (s *Stub) Discussion(org, repo string, number int) ([]Said, error) {
	if s.DiscussionFn != nil {
		return s.DiscussionFn(org, repo, number)
	}
	return nil, nil
}

func (s *Stub) Comment(org, repo string, number int, body string) error {
	if s.CommentFn != nil {
		return s.CommentFn(org, repo, number, body)
	}
	return nil
}

func (s *Stub) ChangedPRs(org, repo string, since time.Time) ([]PRActivity, error) {
	if s.ChangedPRsFn != nil {
		return s.ChangedPRsFn(org, repo, since)
	}
	return nil, nil
}

func (s *Stub) Heads(org string, repos []string, branch string) (map[string]string, error) {
	if s.HeadsFn != nil {
		return s.HeadsFn(org, repos, branch)
	}
	return nil, errors.New("the stub has no heads")
}
