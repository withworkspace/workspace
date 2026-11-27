package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Merge methods, as GitHub names them in its REST API.
const (
	MethodSquash = "squash"
	MethodMerge  = "merge"
	MethodRebase = "rebase"
)

// MergeSettings is how a repo on GitHub lets its pull requests merge:
// the methods it allows, whether it lets one merge by itself once it's
// ready (auto-merge), and whether it deletes a branch once its pull
// request has merged.
type MergeSettings struct {
	Methods      []string `json:"methods"`
	AutoMerge    bool     `json:"autoMerge,omitempty"`
	DeleteBranch bool     `json:"deleteBranch,omitempty"`
}

// Method is how a pull request merges in the repo: want, where the repo
// allows it; else squash, where it does; else the one it allows (a merge
// commit before a rebase, where both are). "" when it allows none, which
// GitHub doesn't.
func (s MergeSettings) Method(want string) string {
	if want != "" && slices.Contains(s.Methods, want) {
		return want
	}
	for _, m := range []string{MethodSquash, MethodMerge, MethodRebase} {
		if slices.Contains(s.Methods, m) {
			return m
		}
	}
	return ""
}

// RepoMergeSettings asks GitHub how a repo's pull requests merge.
//
// VERIFIED (docs.github.com/en/rest/repos/repos#get-a-repository): the
// repository's allow_squash_merge, allow_merge_commit,
// allow_rebase_merge, allow_auto_merge and delete_branch_on_merge, given
// to whoever can push to it.
func RepoMergeSettings(org, repo string) (MergeSettings, error) {
	stdOut, stdErr, err := ghExec("api", "repos/"+repoSlug(org, repo))
	if err != nil {
		return MergeSettings{}, fmt.Errorf("could not read merge settings for %s: %s", repoSlug(org, repo), ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	return parseMergeSettings(stdOut.Bytes())
}

func parseMergeSettings(data []byte) (MergeSettings, error) {
	var r struct {
		Squash       bool `json:"allow_squash_merge"`
		Merge        bool `json:"allow_merge_commit"`
		Rebase       bool `json:"allow_rebase_merge"`
		AutoMerge    bool `json:"allow_auto_merge"`
		DeleteBranch bool `json:"delete_branch_on_merge"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return MergeSettings{}, fmt.Errorf("could not read merge settings: %w", err)
	}
	s := MergeSettings{AutoMerge: r.AutoMerge, DeleteBranch: r.DeleteBranch}
	for _, m := range []struct {
		on   bool
		name string
	}{{r.Squash, MethodSquash}, {r.Merge, MethodMerge}, {r.Rebase, MethodRebase}} {
		if m.on {
			s.Methods = append(s.Methods, m.name)
		}
	}
	return s, nil
}

// MergeAsk is a pull request asked to merge: at the commit you saw
// (Head: GitHub refuses it if the branch has moved since), by Method.
type MergeAsk struct {
	Head   string
	Method string
}

// Merging is how a merge asked for stands: Status is pending (GitHub is
// merging it in the background), merged, enqueued (it's in the base
// branch's merge queue, which merges it when it's its turn) or failed,
// with what GitHub said of it in Why. ID is what its result is asked by
// again (MergeStatus), while it's pending.
type Merging struct {
	ID     string `json:"id,omitempty"`
	Status string `json:"status"`
	Why    string `json:"why,omitempty"`
}

// Merge statuses.
const (
	MergePending  = "pending"
	MergeMerged   = "merged"
	MergeEnqueued = "enqueued"
	MergeFailed   = "failed"
)

// mergeAsyncVersion is the REST API version the asynchronous merge was
// read at, the stacks' own.
const mergeAsyncVersion = StacksAPIVersion

// MergePR asks GitHub to merge a pull request, in the background.
//
// VERIFIED (docs.github.com/en/rest/pulls/pulls, "Merge a pull request
// asynchronously", 2026-11-27): PUT
// /repos/{owner}/{repo}/pulls/{number}/merge-async, with sha (the head it
// must still be at), merge_method, and merge_action default (the base
// branch's merge queue where it has one, else merged straight away). It
// answers 202 with {id, status, details}, or 200 where it had merged or
// was queued already. It's "the required API for merging stacked pull
// requests": asked of a stack's layer, it merges every open layer below
// it too, so a stack merges whole when asked of its top. The older PUT
// .../merge doesn't do stacks or merge queues. bypass_rules isn't sent:
// ws never goes round a branch's rules.
func MergePR(org, repo string, number int, ask MergeAsk) (Merging, error) {
	args := []string{"api", "--method", "PUT",
		fmt.Sprintf("repos/%s/pulls/%d/merge-async", repoSlug(org, repo), number),
		"-H", "X-GitHub-Api-Version: " + mergeAsyncVersion,
		"-f", "merge_action=default"}
	if ask.Head != "" {
		args = append(args, "-f", "sha="+ask.Head)
	}
	if ask.Method != "" {
		args = append(args, "-f", "merge_method="+ask.Method)
	}
	stdOut, stdErr, err := ghExec(args...)
	if err != nil {
		return Merging{}, fmt.Errorf("could not merge %s#%d: %s", repo, number, ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	return parseMerging(stdOut.Bytes())
}

// MergeStatus asks again how a merge asked for (MergePR's ID) stands.
//
// VERIFIED (the same page): GET
// /repos/{owner}/{repo}/pulls/{number}/merge-async/{id}, kept 24 hours.
func MergeStatus(org, repo string, number int, id string) (Merging, error) {
	stdOut, stdErr, err := ghExec("api",
		fmt.Sprintf("repos/%s/pulls/%d/merge-async/%s", repoSlug(org, repo), number, url.PathEscape(id)),
		"-H", "X-GitHub-Api-Version: "+mergeAsyncVersion)
	if err != nil {
		return Merging{}, fmt.Errorf("could not check the merge of %s#%d: %s", repo, number, ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	m, err := parseMerging(stdOut.Bytes())
	if m.ID == "" {
		m.ID = id
	}
	return m, err
}

// parseMerging reads the asynchronous merge's answer. Its details are an
// object the docs don't spell out: a message in it is read as why, and
// otherwise it's said as it came.
func parseMerging(data []byte) (Merging, error) {
	var r struct {
		ID      json.RawMessage `json:"id"`
		UUID    string          `json:"uuid"`
		Status  string          `json:"status"`
		Details json.RawMessage `json:"details"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return Merging{}, fmt.Errorf("could not read the merge result: %w", err)
	}
	m := Merging{ID: r.UUID, Status: strings.ToLower(r.Status)}
	if m.ID == "" && len(r.ID) > 0 {
		var s string
		if json.Unmarshal(r.ID, &s) == nil {
			m.ID = s
		} else {
			m.ID = strings.Trim(string(r.ID), `"`)
		}
	}
	if len(r.Details) > 0 && string(r.Details) != "null" && string(r.Details) != "{}" {
		var d struct {
			Message string `json:"message"`
			Reason  string `json:"reason"`
			Error   string `json:"error"`
		}
		_ = json.Unmarshal(r.Details, &d)
		m.Why = strings.TrimSpace(firstNonEmpty(d.Message, d.Reason, d.Error))
		if m.Why == "" && m.Status == MergeFailed {
			m.Why = string(r.Details)
		}
	}
	if m.Status == "" {
		return m, fmt.Errorf("GitHub returned no merge status")
	}
	return m, nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// AutoMerge is a pull request set to merge by itself once it's ready.
type AutoMerge struct {
	EnabledAt   string `json:"enabledAt,omitempty"`
	MergeMethod string `json:"mergeMethod,omitempty"`
}

// SetAutoMerge has a pull request merge by itself once its checks and
// reviews let it (on), at the commit you saw and by method, or stops it
// doing so (off).
//
// VERIFIED (docs.github.com/en/graphql/reference/mutations):
// enablePullRequestAutoMerge takes pullRequestId, mergeMethod (MERGE,
// SQUASH, REBASE) and expectedHeadOid; disablePullRequestAutoMerge,
// pullRequestId. The repo has to allow auto-merge, and GitHub refuses it
// on a pull request that's ready already ("clean status").
func SetAutoMerge(org, repo string, number int, on bool, ask MergeAsk) error {
	id, err := prNodeID(org, repo, number)
	if err != nil {
		return err
	}
	args := []string{"api", "graphql", "-f", "pr=" + id}
	if on {
		args = append(args, "-f", `query=mutation($pr: ID!, $method: PullRequestMergeMethod, $head: GitObjectID) {
  enablePullRequestAutoMerge(input: {pullRequestId: $pr, mergeMethod: $method, expectedHeadOid: $head}) { clientMutationId }
}`, "-f", "method="+strings.ToUpper(firstNonEmpty(ask.Method, MethodSquash)))
		if ask.Head != "" {
			args = append(args, "-f", "head="+ask.Head)
		}
	} else {
		args = append(args, "-f", `query=mutation($pr: ID!) {
  disablePullRequestAutoMerge(input: {pullRequestId: $pr}) { clientMutationId }
}`)
	}
	stdOut, stdErr, err := ghExec(args...)
	if err == nil {
		err = graphQLError(stdOut.Bytes())
	}
	if err != nil {
		how := "could not turn on auto-merge for %s#%d: %s"
		if !on {
			how = "could not turn off auto-merge for %s#%d: %s"
		}
		return fmt.Errorf(how, repo, number, ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	return nil
}

// prNodeID is a pull request's GraphQL id.
func prNodeID(org, repo string, number int) (string, error) {
	stdOut, stdErr, err := ghExec("api", "graphql",
		"-f", `query=query($owner: String!, $repo: String!, $number: Int!) { repository(owner: $owner, name: $repo) { pullRequest(number: $number) { id } } }`,
		"-f", "owner="+org, "-f", "repo="+repo, "-F", "number="+strconv.Itoa(number))
	if err != nil {
		return "", fmt.Errorf("could not find %s#%d on GitHub: %s", repo, number, ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	var r struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ID string `json:"id"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdOut.Bytes(), &r); err != nil || r.Data.Repository.PullRequest.ID == "" {
		return "", fmt.Errorf("%s#%d not found on GitHub", repo, number)
	}
	return r.Data.Repository.PullRequest.ID, nil
}

// graphQLError is the first error a GraphQL answer carries, which gh
// can exit 0 with.
func graphQLError(data []byte) error {
	var r struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(data, &r) == nil && len(r.Errors) > 0 {
		return fmt.Errorf("%s", r.Errors[0].Message)
	}
	return nil
}

// DeleteBranch deletes a branch on GitHub, as its Delete branch button
// does once a pull request has merged.
//
// VERIFIED (docs.github.com/en/rest/git/refs#delete-a-reference): DELETE
// /repos/{owner}/{repo}/git/refs/heads/{branch}, 204; 422 where it's gone
// already. Open pull requests based on it are retargeted onto its pull
// request's base by GitHub.
func DeleteBranch(org, repo, branch string) error {
	stdOut, stdErr, err := ghExec("api", "--method", "DELETE",
		fmt.Sprintf("repos/%s/git/refs/heads/%s", repoSlug(org, repo), escapeRef(branch)))
	if err != nil {
		return fmt.Errorf("could not delete %s's branch %s: %s", repo, branch, ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	return nil
}

// escapeRef escapes each part of a branch's name for a path, keeping its
// slashes.
func escapeRef(branch string) string {
	parts := strings.Split(branch, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func (LiveClient) RepoMergeSettings(org, repo string) (MergeSettings, error) {
	return RepoMergeSettings(org, repo)
}
func (LiveClient) MergePR(org, repo string, number int, ask MergeAsk) (Merging, error) {
	return MergePR(org, repo, number, ask)
}
func (LiveClient) MergeStatus(org, repo string, number int, id string) (Merging, error) {
	return MergeStatus(org, repo, number, id)
}
func (LiveClient) SetAutoMerge(org, repo string, number int, on bool, ask MergeAsk) error {
	return SetAutoMerge(org, repo, number, on, ask)
}
func (LiveClient) DeleteBranch(org, repo, branch string) error {
	return DeleteBranch(org, repo, branch)
}

// fixtureMergeSettings is a fixture repo's, or the default: any method,
// auto-merge allowed, branches kept.
func fixtureMergeSettings(r *FixtureRepo) MergeSettings {
	if r != nil && r.Merge != nil {
		return *r.Merge
	}
	return MergeSettings{Methods: []string{MethodSquash, MethodMerge, MethodRebase}, AutoMerge: true}
}

func (c FixtureClient) RepoMergeSettings(org, repo string) (MergeSettings, error) {
	_, r, err := c.repo(repo)
	if err != nil {
		return MergeSettings{}, err
	}
	return fixtureMergeSettings(r), nil
}

// MergePR merges the fixture's pull request at once, as GitHub would
// where there's no merge queue: refused when its head has moved, it's a
// draft or it conflicts. A stack's layer merges every open layer below
// it with it.
func (c FixtureClient) MergePR(org, repo string, number int, ask MergeAsk) (Merging, error) {
	at := c.now().UTC().Format(time.RFC3339)
	err := c.change(func(f *Fixture) error {
		r := f.Repos[repo]
		if r == nil {
			return fmt.Errorf("no open PR #%d in %s/%s", number, org, repo)
		}
		i := slices.IndexFunc(r.PRs, func(p PR) bool { return p.Number == number })
		if i < 0 {
			return fmt.Errorf("no open PR #%d in %s/%s", number, org, repo)
		}
		pr := r.PRs[i]
		switch {
		case ask.Head != "" && pr.HeadRefOid != "" && pr.HeadRefOid != ask.Head:
			return fmt.Errorf("head branch was modified. Review and try the merge again")
		case pr.IsDraft:
			return fmt.Errorf("pull request is still a draft")
		case pr.Mergeable == "CONFLICTING":
			return fmt.Errorf("pull request is not mergeable")
		case ask.Method != "" && !slices.Contains(fixtureMergeSettings(r).Methods, ask.Method):
			return fmt.Errorf("%s merges are not allowed on this repository", ask.Method)
		}
		merging := []int{number}
		for si := range r.Stacks {
			st := &r.Stacks[si]
			at := slices.IndexFunc(st.PRs, func(p StackPR) bool { return p.Number == number })
			if at < 0 {
				continue
			}
			for li := 0; li <= at; li++ {
				if l := &st.PRs[li]; !l.Merged && l.State != "closed" {
					l.Merged, l.State = true, "closed"
					if l.Number != number {
						merging = append(merging, l.Number)
					}
				}
			}
			st.Open = slices.ContainsFunc(st.PRs, func(p StackPR) bool { return !p.Merged && p.State != "closed" })
		}
		for _, n := range merging {
			j := slices.IndexFunc(r.PRs, func(p PR) bool { return p.Number == n })
			if j < 0 {
				continue
			}
			done := r.PRs[j]
			done.State, done.MergedAt, done.UpdatedAt, done.AutoMergeRequest = "MERGED", at, at, nil
			r.Merged = append(r.Merged, done)
			r.PRs = slices.Delete(r.PRs, j, j+1)
		}
		return nil
	})
	if err != nil {
		return Merging{}, fmt.Errorf("could not merge %s#%d: %w", repo, number, err)
	}
	return Merging{ID: fmt.Sprintf("fixture-%s-%d", repo, number), Status: MergeMerged}, nil
}

func (c FixtureClient) MergeStatus(org, repo string, number int, id string) (Merging, error) {
	pr, err := c.PRFromNumber(org, repo, number)
	if err != nil {
		return Merging{}, err
	}
	if pr.MergedAt != "" {
		return Merging{ID: id, Status: MergeMerged}, nil
	}
	return Merging{ID: id, Status: MergePending}, nil
}

// SetAutoMerge sets or clears the fixture's pull request's auto-merge,
// refused where the repo doesn't allow it.
func (c FixtureClient) SetAutoMerge(org, repo string, number int, on bool, ask MergeAsk) error {
	at := c.now().UTC().Format(time.RFC3339)
	return c.change(func(f *Fixture) error {
		r := f.Repos[repo]
		if r == nil {
			return fmt.Errorf("no open PR #%d in %s/%s", number, org, repo)
		}
		if on && !fixtureMergeSettings(r).AutoMerge {
			return fmt.Errorf("auto merge is not allowed for this repository")
		}
		for i := range r.PRs {
			if r.PRs[i].Number != number {
				continue
			}
			if on && ask.Head != "" && r.PRs[i].HeadRefOid != "" && r.PRs[i].HeadRefOid != ask.Head {
				return fmt.Errorf("head branch was modified")
			}
			r.PRs[i].AutoMergeRequest = nil
			if on {
				r.PRs[i].AutoMergeRequest = &AutoMerge{EnabledAt: at, MergeMethod: strings.ToUpper(firstNonEmpty(ask.Method, MethodSquash))}
			}
			r.PRs[i].UpdatedAt = at
			return nil
		}
		return fmt.Errorf("no open PR #%d in %s/%s", number, org, repo)
	})
}

func (c FixtureClient) DeleteBranch(org, repo, branch string) error {
	return c.change(func(f *Fixture) error {
		r := f.Repos[repo]
		if r == nil {
			return fmt.Errorf("no repo %s/%s", org, repo)
		}
		r.Deleted = append(r.Deleted, branch)
		return nil
	})
}
