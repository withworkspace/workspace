package ops

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/ref"
)

// mergeGitHub is a GitHub with frontend's open pull requests (yours), its
// stacks and how it merges, which remembers what was merged, set to merge
// by itself and deleted.
type mergeGitHub struct {
	github.Stub
	prs      []*github.PR
	stacks   []github.Stack
	settings github.MergeSettings
	answer   github.Merging // what a merge answers (merged, by default)
	later    github.Merging // what asking how it's going answers

	merged  []string // #n head method
	auto    []string // #n on|off head method
	deleted []string
}

func readyPR(n int, branch, base string) *github.PR {
	return &github.PR{Number: n, Title: fmt.Sprintf("Change %d", n), State: "OPEN", Author: "you", URL: fmt.Sprintf("u/%d", n),
		HeadRefName: branch, BaseRefName: base, HeadRefOid: fmt.Sprintf("h%d", n),
		ReviewDecision: "APPROVED", StatusRollup: "success", Mergeable: "MERGEABLE", MergeState: "CLEAN"}
}

func mergeFixture(t *testing.T, prs ...*github.PR) (*Workspace, *mergeGitHub) {
	t.Helper()
	w, _ := fixture(t, "")
	gh := &mergeGitHub{prs: prs, settings: github.MergeSettings{Methods: []string{github.MethodSquash, github.MethodMerge}, AutoMerge: true},
		answer: github.Merging{ID: "m1", Status: github.MergeMerged}}
	find := func(n int) *github.PR {
		for _, p := range gh.prs {
			if p.Number == n {
				return p
			}
		}
		return nil
	}
	gh.PRFromNumberFn = func(org, repo string, n int) (*github.PR, error) {
		if p := find(n); p != nil {
			c := *p
			return &c, nil
		}
		return nil, fmt.Errorf("no PR #%d", n)
	}
	gh.OpenPRsFn = func(org, repo string) ([]github.PR, error) {
		var out []github.PR
		for _, p := range gh.prs {
			out = append(out, *p)
		}
		return out, nil
	}
	gh.StacksForRepoFn = func(org, repo string) ([]github.Stack, error) { return gh.stacks, nil }
	gh.MergeSettingsFn = func(org, repo string) (github.MergeSettings, error) { return gh.settings, nil }
	gh.MergePRFn = func(org, repo string, n int, ask github.MergeAsk) (github.Merging, error) {
		gh.merged = append(gh.merged, fmt.Sprintf("#%d %s %s", n, ask.Head, ask.Method))
		return gh.answer, nil
	}
	gh.MergeStatusFn = func(org, repo string, n int, id string) (github.Merging, error) { return gh.later, nil }
	gh.SetAutoMergeFn = func(org, repo string, n int, on bool, ask github.MergeAsk) error {
		gh.auto = append(gh.auto, strings.TrimSpace(fmt.Sprintf("#%d %v %s %s", n, on, ask.Head, ask.Method)))
		return nil
	}
	gh.DeleteBranchFn = func(org, repo, branch string) error { gh.deleted = append(gh.deleted, branch); return nil }
	w.opts.GitHub = gh
	w.opts.Reviews = &pulls.Service{Root: w.Root(), GitHub: gh,
		Remote: func(repo string) ref.Remote { return ref.Remote{Owner: "acme", Name: repo} },
		Repos:  func() []string { return []string{"frontend"} }}
	return w, gh
}

// A pull request ready to merge merges by the repo's method, at the
// commit ws showed, and its branch goes once it has.
func TestMergePR_ReadyMergesAsShown(t *testing.T) {
	w, gh := mergeFixture(t, readyPR(5, "fix", "main"))
	out, err := w.MergePR("frontend", 5, MergeAsk{}, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gh.merged, []string{"#5 h5 squash"}) || !slices.Equal(gh.deleted, []string{"fix"}) {
		t.Fatalf("merged %v, deleted %v", gh.merged, gh.deleted)
	}
	if out.String() != "Merged frontend#5, squashed." {
		t.Errorf("said %q", out)
	}
}

// What's merged is what you looked at: a branch pushed to since isn't.
func TestMergePR_RefusedWhenItsMoved(t *testing.T) {
	w, gh := mergeFixture(t, readyPR(5, "fix", "main"))
	_, err := w.MergePR("frontend", 5, MergeAsk{Head: "older"}, AsYou)
	refusedWith(t, err, "moved")
	if len(gh.merged) > 0 {
		t.Fatalf("merged %v", gh.merged)
	}
}

// Merging is yours: crew and the orchestrator are told so.
func TestMergePR_Yours(t *testing.T) {
	w, gh := mergeFixture(t, readyPR(5, "fix", "main"))
	_, err := w.MergePR("frontend", 5, MergeAsk{}, AsCrew("c1"))
	refusedWith(t, err, "merge-is-yours")
	_, err = w.MergePR("frontend", 5, MergeAsk{}, AsOrchestrator)
	refusedWith(t, err, "merge-is-yours")
	if len(gh.merged) > 0 {
		t.Fatalf("merged %v", gh.merged)
	}
}

// One its base's rules still hold isn't merged, though an admin could go
// round them: it can be left to merge by itself once they let it. One
// that needs work can't be.
func TestMergePR_HeldWaitsOrIsRefused(t *testing.T) {
	blocked := readyPR(5, "fix", "main")
	blocked.ReviewDecision, blocked.MergeState = "", "BLOCKED"
	failing := readyPR(6, "other", "main")
	failing.StatusRollup = "failure"
	w, gh := mergeFixture(t, blocked, failing)

	_, err := w.MergePR("frontend", 5, MergeAsk{}, AsYou)
	refusedWith(t, err, "not-ready")
	if err == nil || err.Error() != "frontend#5 can't merge yet: it is waiting for the review its base branch's rules ask for. Merge when ready has it merge by itself once it can" {
		t.Errorf("err = %v", err)
	}
	out, err := w.MergePR("frontend", 5, MergeAsk{When: MergeReady}, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "on" || !slices.Equal(gh.auto, []string{"#5 true h5 squash"}) {
		t.Fatalf("status %q, auto %v", out.Status, gh.auto)
	}

	_, err = w.MergePR("frontend", 6, MergeAsk{When: MergeReady}, AsYou)
	refusedWith(t, err, "not-waiting")
	_, err = w.MergePR("frontend", 6, MergeAsk{}, AsYou)
	refusedWith(t, err, "not-ready")

	gh.settings.AutoMerge = false
	w.mergeKept = keptMergeSettings{}
	_, err = w.MergePR("frontend", 5, MergeAsk{When: MergeReady}, AsYou)
	refusedWith(t, err, "no-auto-merge")
	if len(gh.merged) > 0 {
		t.Fatalf("merged %v", gh.merged)
	}
}

// Merge when ready, of one that's ready, merges it now; off stops one
// merging by itself.
func TestMergePR_WhenReadyAndOff(t *testing.T) {
	set := readyPR(6, "other", "main")
	set.StatusRollup, set.AutoMergeRequest = "pending", &github.AutoMerge{EnabledAt: "t"}
	w, gh := mergeFixture(t, readyPR(5, "fix", "main"), set)
	if _, err := w.MergePR("frontend", 5, MergeAsk{When: MergeReady}, AsYou); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gh.merged, []string{"#5 h5 squash"}) {
		t.Fatalf("merged %v", gh.merged)
	}
	out, err := w.MergePR("frontend", 6, MergeAsk{When: MergeOff}, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "off" || !slices.Equal(gh.auto, []string{"#6 false"}) {
		t.Fatalf("status %q, auto %v", out.Status, gh.auto)
	}
	_, err = w.MergePR("frontend", 5, MergeAsk{When: MergeOff}, AsYou)
	refusedWith(t, err, "not-set")
}

// The method is the repo's: squash where it allows it, else the one it
// does; ws.toml's, where the repo allows that.
func TestMergePR_Method(t *testing.T) {
	w, gh := mergeFixture(t, readyPR(5, "fix", "main"), readyPR(6, "other", "main"), readyPR(7, "third", "main"))
	gh.settings.Methods = []string{github.MethodMerge, github.MethodRebase}
	if _, err := w.MergePR("frontend", 5, MergeAsk{}, AsYou); err != nil {
		t.Fatal(err)
	}
	w.Config().Repos = map[string]config.RepoConfig{"frontend": {Merge: "rebase"}}
	if _, err := w.MergePR("frontend", 6, MergeAsk{}, AsYou); err != nil {
		t.Fatal(err)
	}
	w.Config().Repos = map[string]config.RepoConfig{"frontend": {Merge: "squash"}}
	out, err := w.MergePR("frontend", 7, MergeAsk{}, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gh.merged, []string{"#5 h5 merge", "#6 h6 rebase", "#7 h7 merge"}) {
		t.Fatalf("merged %v", gh.merged)
	}
	if !strings.Contains(out.String(), "doesn't allow squash merges") {
		t.Errorf("said %q", out)
	}
}

// A stack merges whole, asked of its top, once every layer's ready; a
// layer below is never merged alone.
func TestMergePR_StackWhole(t *testing.T) {
	w, gh := mergeFixture(t, readyPR(5, "fix", "main"), readyPR(6, "fix-2", "fix"))
	gh.stacks = []github.Stack{{Number: 3, Base: "main", Open: true, PRs: []github.StackPR{
		{Number: 5, State: "open", HeadRef: "fix", HeadSHA: "h5"}, {Number: 6, State: "open", HeadRef: "fix-2", HeadSHA: "h6"}}}}

	_, err := w.MergePR("frontend", 5, MergeAsk{}, AsYou)
	refusedWith(t, err, "stack-layer")
	out, err := w.MergePR("frontend", 6, MergeAsk{}, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gh.merged, []string{"#6 h6 squash"}) || out.Stack != 3 || !slices.Equal(out.Below, []int{5}) {
		t.Fatalf("merged %v, stack %d below %v", gh.merged, out.Stack, out.Below)
	}
	slices.Sort(gh.deleted)
	if !slices.Equal(gh.deleted, []string{"fix", "fix-2"}) {
		t.Errorf("deleted %v", gh.deleted)
	}
	if out.String() != "Merged stack 3 (frontend#5 and #6), squashed." {
		t.Errorf("said %q", out)
	}
}

func TestMergePR_StackNotReady(t *testing.T) {
	below := readyPR(5, "fix", "main")
	below.ReviewDecision = "REVIEW_REQUIRED"
	w, gh := mergeFixture(t, below, readyPR(6, "fix-2", "fix"))
	gh.stacks = []github.Stack{{Number: 3, Base: "main", Open: true, PRs: []github.StackPR{
		{Number: 5, State: "open", HeadRef: "fix"}, {Number: 6, State: "open", HeadRef: "fix-2"}}}}
	_, err := w.MergePR("frontend", 6, MergeAsk{}, AsYou)
	refusedWith(t, err, "stack-not-ready")
	_, err = w.MergePR("frontend", 6, MergeAsk{When: MergeReady}, AsYou)
	refusedWith(t, err, "stack-not-ready")
	if len(gh.merged) > 0 {
		t.Fatalf("merged %v", gh.merged)
	}
}

// A chain ws worked out (one pull request on another's branch) isn't a
// GitHub stack, and isn't merged in part.
func TestMergePR_ChainRefused(t *testing.T) {
	w, gh := mergeFixture(t, readyPR(5, "fix", "main"), readyPR(6, "on-fix", "fix"))
	_, err := w.MergePR("frontend", 5, MergeAsk{}, AsYou)
	refusedWith(t, err, "chain")
	if len(gh.merged) > 0 {
		t.Fatalf("merged %v", gh.merged)
	}
}

// A branch is kept where the repo deletes it itself, and where another
// open pull request is based on it (two on it, each a row of its own).
func TestMergePR_BranchesKept(t *testing.T) {
	w, gh := mergeFixture(t, readyPR(5, "fix", "main"), readyPR(6, "on-fix", "fix"), readyPR(7, "also-on-fix", "fix"))
	out, err := w.MergePR("frontend", 5, MergeAsk{}, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if len(gh.deleted) > 0 || !strings.Contains(out.String(), "is based on it") {
		t.Fatalf("deleted %v, said %q", gh.deleted, out)
	}

	w, gh = mergeFixture(t, readyPR(5, "fix", "main"))
	gh.settings.DeleteBranch = true
	if _, err := w.MergePR("frontend", 5, MergeAsk{}, AsYou); err != nil {
		t.Fatal(err)
	}
	if len(gh.deleted) > 0 {
		t.Fatalf("deleted %v", gh.deleted)
	}
}

// A merge GitHub does in the background is followed, from a command,
// till it's done: its branch goes then.
func TestMergePR_PendingFollowed(t *testing.T) {
	was := followMergeEvery
	followMergeEvery = time.Millisecond
	t.Cleanup(func() { followMergeEvery = was })
	w, gh := mergeFixture(t, readyPR(5, "fix", "main"))
	gh.answer = github.Merging{ID: "m1", Status: github.MergePending}
	gh.later = github.Merging{ID: "m1", Status: github.MergeMerged}
	out, err := w.MergePR("frontend", 5, MergeAsk{}, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != github.MergeMerged || !slices.Equal(gh.deleted, []string{"fix"}) {
		t.Fatalf("status %q, deleted %v", out.Status, gh.deleted)
	}
}
