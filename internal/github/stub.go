package github

import (
	"errors"
	"time"

	"github.com/withworkspace/workspace/internal/ref"
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
	// AskedByNameFn answers AskedByName, and DropReviewRequestFn takes a
	// name off a PR's reviewers; without it, there's no GitHub to.
	AskedByNameFn       func(org string) ([]PRRef, error)
	DropReviewRequestFn func(org, repo string, number int, login string) error
	StacksForRepoFn     func(org, repo string) ([]Stack, error)
	SubmitReviewFn      func(org, repo string, number int, r ReviewSubmission) (string, error)
	ReviewThreadsFn     func(org, repo string, number int) ([]ReviewThread, error)
	ReplyToThreadFn     func(org, repo string, number int, threadID, body string) error
	ResolveThreadFn     func(org, repo string, number int, threadID string, resolved bool) error
	DiscussionFn        func(org, repo string, number int) ([]Said, error)
	CommentFn           func(org, repo string, number int, body string) error

	// The Team page's.
	ChangedPRsFn func(org, repo string, since time.Time) ([]PRActivity, error)

	// CreatePRFn opens a pull request; without it, there's no GitHub to
	// open one on.
	CreatePRFn func(org, repo string, n NewPR) (*PR, error)
	// MarkReadyFn marks a draft ready; without it, there's no GitHub.
	MarkReadyFn func(org, repo string, number int) error
	// The merge's (merge.go): without MergePRFn, SetAutoMergeFn or
	// DeleteBranchFn there's no GitHub to; without MergeSettingsFn, the
	// repo allows squash alone; without MergeStatusFn, a merge is still
	// pending.
	MergeSettingsFn func(org, repo string) (MergeSettings, error)
	MergePRFn       func(org, repo string, number int, ask MergeAsk) (Merging, error)
	MergeStatusFn   func(org, repo string, number int, id string) (Merging, error)
	SetAutoMergeFn  func(org, repo string, number int, on bool, ask MergeAsk) error
	DeleteBranchFn  func(org, repo, branch string) error

	// HeadsFn says where repos' branches are; without it, GitHub can't
	// say, and each repo's fetched instead.
	HeadsFn func(repos []ref.Remote, branch string) (map[ref.Remote]string, error)
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

func (s *Stub) Heads(repos []ref.Remote, branch string) (map[ref.Remote]string, error) {
	if s.HeadsFn != nil {
		return s.HeadsFn(repos, branch)
	}
	return nil, errors.New("the stub has no heads")
}

func (s *Stub) RepoMergeSettings(org, repo string) (MergeSettings, error) {
	if s.MergeSettingsFn != nil {
		return s.MergeSettingsFn(org, repo)
	}
	return MergeSettings{Methods: []string{MethodSquash}}, nil
}

func (s *Stub) MergePR(org, repo string, number int, ask MergeAsk) (Merging, error) {
	if s.MergePRFn != nil {
		return s.MergePRFn(org, repo, number, ask)
	}
	return Merging{}, errors.New("no GitHub here to merge a pull request on")
}

func (s *Stub) MergeStatus(org, repo string, number int, id string) (Merging, error) {
	if s.MergeStatusFn != nil {
		return s.MergeStatusFn(org, repo, number, id)
	}
	return Merging{ID: id, Status: MergePending}, nil
}

func (s *Stub) SetAutoMerge(org, repo string, number int, on bool, ask MergeAsk) error {
	if s.SetAutoMergeFn != nil {
		return s.SetAutoMergeFn(org, repo, number, on, ask)
	}
	return errors.New("no GitHub here to set a pull request merging by itself")
}

func (s *Stub) DeleteBranch(org, repo, branch string) error {
	if s.DeleteBranchFn != nil {
		return s.DeleteBranchFn(org, repo, branch)
	}
	return errors.New("no GitHub here to delete a branch on")
}
