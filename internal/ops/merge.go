package ops

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/notification"
	"github.com/withworkspace/workspace/internal/pulls"
)

// When a pull request's asked to merge.
const (
	MergeNow   = "now"   // merge it now: it's ready
	MergeReady = "ready" // have it merge by itself once it can
	MergeOff   = "off"   // stop it merging by itself
)

// MergeAsk is a pull request asked to merge: when, and at the commit you
// saw it at (Head; "" for the one ws last showed). A branch pushed to
// since isn't merged: what's merged is what you looked at.
type MergeAsk struct {
	When string
	Head string
}

// Merged is what came of asking a pull request to merge.
type Merged struct {
	Repo   string
	Number int
	When   string
	// Method is how it merges: squash, merge or rebase.
	Method string
	// Status is GitHub's: merged, pending (it's merging it), enqueued (in
	// its base's merge queue); for MergeReady, on; for MergeOff, off.
	Status string
	// Stack is the stack it's the top of, which merges whole with it, and
	// Below the layers under it that do.
	Stack int
	Below []int
	// Instead says the repo doesn't allow the method ws.toml asks for:
	// that method, which wasn't used.
	Instead string
	// Kept are branches left on GitHub that ws would have deleted, and why.
	Kept string

	id     string
	branch []string // the branches to delete once merged: its own, and the layers'
	delete bool
}

func (m Merged) Ref() string { return fmt.Sprintf("%s#%d", m.Repo, m.Number) }

// what is what merges: the pull request, or its stack and every layer,
// bottom first (stack 2: api#25 and #26).
func (m Merged) what() string {
	if m.Stack == 0 {
		return m.Ref()
	}
	var layers []string
	for i, n := range append(append([]int{}, m.Below...), m.Number) {
		if i == 0 {
			layers = append(layers, fmt.Sprintf("%s#%d", m.Repo, n))
		} else {
			layers = append(layers, fmt.Sprintf("#%d", n))
		}
	}
	last := layers[len(layers)-1]
	return fmt.Sprintf("stack %d (%s and %s)", m.Stack, strings.Join(layers[:len(layers)-1], ", "), last)
}

func (m Merged) String() string {
	how := map[string]string{github.MethodSquash: "squashed", github.MethodMerge: "with a merge commit", github.MethodRebase: "rebased"}[m.Method]
	var s string
	switch m.Status {
	case github.MergeMerged:
		s = fmt.Sprintf("Merged %s, %s.", m.what(), how)
	case github.MergePending:
		s = fmt.Sprintf("Merging %s, %s.", m.what(), how)
	case github.MergeEnqueued:
		s = fmt.Sprintf("%s added to the merge queue.", capitalFirst(m.what()))
	case "on":
		s = fmt.Sprintf("Auto-merge on for %s, %s.", m.Ref(), how)
	case "off":
		s = fmt.Sprintf("Auto-merge off for %s.", m.Ref())
	}
	if m.Instead != "" {
		s += fmt.Sprintf(" Repo does not allow %s merges (set in ws.toml), so its default was used.", m.Instead)
	}
	if m.Kept != "" {
		s += " " + m.Kept
	}
	return s
}

func capitalFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// MergePR merges a pull request of one of the workspace's repos, now or
// once it can, or stops it merging by itself; a stack merges whole, from
// its top, and never in part. It's yours alone, and never goes round the
// base branch's rules (an admin could): what isn't ready is refused, with
// why. The method is the repo's, as GitHub allows it ([repos.x] merge
// says which, where it allows more than one). A merge GitHub does in the
// background is followed till it's done (the daemon's, behind; a
// command's, before it returns), and once it's merged, its branch is
// deleted where the repo doesn't do that itself.
func (w *Workspace) MergePR(repo string, number int, ask MergeAsk, by Actor) (Merged, error) {
	if !by.IsYou() {
		return Merged{}, refused("merge-is-yours", "merging a pull request is the user's: say it's ready in your handoff, or ask them")
	}
	gh := w.GitHub()
	if gh == nil {
		return Merged{}, refused("no-github", "could not merge: no GitHub access")
	}
	if r, ok := w.WS().ResolveAlias(repo); ok {
		repo = r
	}
	if !slices.Contains(w.WS().RepoNames, repo) {
		return Merged{}, invalid("%s is not a repo in this workspace", repo)
	}
	rc := w.Config().Repos[repo]
	when := ask.When
	if when == "" {
		when = MergeNow
	}
	if when != MergeNow && when != MergeReady && when != MergeOff {
		return Merged{}, invalid("merge now, ready or off, not %q", ask.When)
	}
	remote := w.WS().RemoteOf(repo)
	out := Merged{Repo: repo, Number: number, When: when}
	ref := out.Ref()

	pr, err := gh.PRFromNumber(remote.Owner, remote.Name, number)
	if err != nil {
		return out, &Upstream{Err: err}
	}
	switch {
	case pr.State == "MERGED" || pr.MergedAt != "":
		return out, refused("merged", "%s already merged", ref)
	case pr.State != "OPEN":
		return out, refused("closed", "%s is closed", ref)
	}
	if when == MergeOff {
		if pr.AutoMergeRequest == nil {
			return out, refused("not-set", "%s does not have auto-merge on", ref)
		}
		if err := gh.SetAutoMerge(remote.Owner, remote.Name, number, false, github.MergeAsk{}); err != nil {
			return out, &Upstream{Err: err}
		}
		out.Status = "off"
		w.mergeAsked(repo)
		return out, nil
	}

	kept, st := w.shownPR(repo, number)
	head := ask.Head
	if head == "" && kept != nil {
		head = kept.Head
	}
	if head != "" && pr.HeadRefOid != "" && head != pr.HeadRefOid {
		return out, refused("moved", "%s has new commits: review again before merging", ref)
	}
	if head == "" {
		head = pr.HeadRefOid
	}
	if pr.IsDraft {
		return out, refused("draft", "%s is a draft: mark it ready for review first", ref)
	}
	stackHeld, err := stackMerges(&out, st, number)
	if err != nil {
		return out, err
	}
	held, wait := mission.MergeHeld(mission.PRFacts{Ref: ref, Review: pr.ReviewDecision, Checks: pr.StatusRollup,
		Conflict: pr.Mergeable == "CONFLICTING", MergeUnknown: pr.Mergeable == "UNKNOWN", Blocked: pr.MergeState == "BLOCKED"})

	settings, err := w.mergeSettings(gh, repo)
	if err != nil {
		return out, &Upstream{Err: err}
	}
	out.Method = settings.Method(rc.Merge)
	if rc.Merge != "" && out.Method != rc.Merge {
		out.Instead = rc.Merge
	}
	out.delete = !settings.DeleteBranch

	if when == MergeReady && held != "" {
		switch {
		case !wait:
			return out, refused("not-waiting", "%s cannot auto-merge: %s, which waiting will not fix", ref, aboutIt(held, ref))
		case out.Stack > 0:
			return out, refused("stack-auto", "auto-merge is not available for stacks: merge once ready")
		case !settings.AutoMerge:
			return out, refused("no-auto-merge", "%s has auto-merge off in its GitHub settings", repo)
		}
		if err := gh.SetAutoMerge(remote.Owner, remote.Name, number, true, github.MergeAsk{Head: head, Method: out.Method}); err != nil {
			return out, &Upstream{Err: err}
		}
		out.Status = "on"
		w.mergeAsked(repo)
		return out, nil
	}
	// Merge when ready, of one that's ready, is merging it now.
	if held != "" {
		if wait {
			return out, refused("not-ready", "%s cannot merge yet: %s. Use Merge when ready to merge once it can", ref, aboutIt(strings.TrimSuffix(held, " · nothing for you to do"), ref))
		}
		return out, refused("not-ready", "%s cannot merge: %s", ref, aboutIt(held, ref))
	}
	if stackHeld != "" {
		return out, refused("stack-not-ready", "stack %d cannot merge yet: %s", out.Stack, stackHeld)
	}
	out.branch = w.mergedBranches(pr, st, &out)
	m, err := gh.MergePR(remote.Owner, remote.Name, number, github.MergeAsk{Head: head, Method: out.Method})
	if err != nil {
		return out, &Upstream{Err: err}
	}
	if m.Status == github.MergeFailed {
		return out, refused("merge-failed", "GitHub did not merge %s: %s", ref, firstNonEmpty(m.Why, "no reason given"))
	}
	out.Status, out.id = m.Status, m.ID
	w.mergeAsked(repo)
	switch {
	case m.Status == github.MergeMerged:
		w.afterMerge(gh, &out)
	case m.Status == github.MergePending && w.opts.Life != nil:
		// The daemon's: followed behind, for as long as it lives.
		go w.followMerge(w.opts.Life, gh, out)
	case m.Status == github.MergePending:
		// A command's: it waits, as it would have for a merge done at once.
		ctx, cancel := context.WithTimeout(context.Background(), followMergeFor)
		defer cancel()
		if done := w.followMerge(ctx, gh, out); done.Status != "" {
			out = done
		}
	}
	return out, nil
}

// MergeMissionPR merges one of a mission's pull requests (repo#N), as
// MergePR does, at the commit its page last showed: the PR is found as the
// page finds it, from what the daemon keeps (missionPRView).
func (w *Workspace) MergeMissionPR(slug, target, when string, by Actor) (Merged, error) {
	m, err := w.load(slug)
	if err != nil {
		return Merged{}, err
	}
	repo, number, err := w.ParsePRRef(target)
	if err != nil {
		return Merged{}, invalid("%v", err)
	}
	head := ""
	found := false
	for _, c := range w.missionPRView(m).Capsules {
		if c.Repo == repo && c.PR != nil && c.PR.Number == number {
			head, found = c.PR.HeadRefOid, true
		}
	}
	if !found {
		return Merged{}, refused("not-attached", "%s is not one of the mission's pull requests", target)
	}
	out, err := w.MergePR(repo, number, MergeAsk{When: when, Head: head}, by)
	if err != nil {
		return out, err
	}
	w.youActed(slug, by)
	return out, nil
}

// stackMerges says what of a stack merges with its layer: none of it
// where it isn't in one; the whole stack, asked of its top; never part of
// one. held is the first layer below that isn't ready to merge, as the
// Reviews page says it, or "".
func stackMerges(out *Merged, st *pulls.Stack, number int) (held string, err error) {
	if st == nil || len(st.Layers) < 2 {
		return "", nil
	}
	if st.WorkedOut {
		return "", refused("chain", "%s builds on another pull request, not a GitHub stack: Workspace cannot merge part of it", out.Ref())
	}
	top := st.Layers[len(st.Layers)-1]
	if top.Number != number {
		return "", refused("stack-layer", "%s is in stack %d, which merges as a whole from its top: %s#%d", out.Ref(), st.Number, st.Repo, top.Number)
	}
	out.Stack = st.Number
	for _, l := range st.Layers[:len(st.Layers)-1] {
		if l.State != pulls.Ready && held == "" {
			held = fmt.Sprintf("#%d %s", l.Number, l.Words)
		}
		out.Below = append(out.Below, l.Number)
	}
	return held, nil
}

// shownPR is the pull request as ws last showed it (the Reviews page's
// list, or the page kept of it), and its stack: nothing's asked of
// GitHub but where neither has it.
func (w *Workspace) shownPR(repo string, number int) (*pulls.PR, *pulls.Stack) {
	svc := w.PRService()
	if svc == nil {
		return nil, nil
	}
	pr, st, err := svc.Known(repo, number)
	if err != nil {
		return nil, nil
	}
	return pr, st
}

// mergedBranches are the branches on GitHub that go once the merge has,
// where the repo doesn't delete them itself: its own and its stack's
// layers', but one in a fork, or one another open pull request is based
// on (GitHub would have it retargeted, but ws doesn't move someone's pull
// request under them).
func (w *Workspace) mergedBranches(pr *github.PR, st *pulls.Stack, out *Merged) []string {
	if !out.delete || pr.IsCrossRepository {
		return nil
	}
	merging := map[int]bool{out.Number: true}
	for _, n := range out.Below {
		merging[n] = true
	}
	branches := []string{pr.HeadRefName}
	if st != nil {
		for _, l := range st.Layers {
			if l.Number != out.Number && !l.Fork && l.Branch != "" {
				branches = append(branches, l.Branch)
			}
		}
	}
	var based []string
	if svc := w.PRService(); svc != nil {
		for _, o := range svc.Peek().All() {
			if o.Repo == out.Repo && !merging[o.Number] && slices.Contains(branches, o.Base) {
				based = append(based, o.Base)
				out.Kept = fmt.Sprintf("Branch %s kept: %s is based on it.", o.Base, o.Ref())
			}
		}
	}
	return slices.DeleteFunc(branches, func(b string) bool { return b == "" || slices.Contains(based, b) })
}

// followMergeFor is how long a merge GitHub does in the background is
// followed; followMergeEvery, how often it's asked how it's going (tests
// ask more often).
const followMergeFor = 3 * time.Minute

var followMergeEvery = 2 * time.Second

// followMerge asks GitHub how a merge it's doing in the background is
// going till it's done (or ctx is), then does what follows a merge, or
// says, in the feed, that it didn't go through. It returns the merge as
// it last stood.
func (w *Workspace) followMerge(ctx context.Context, gh github.Client, m Merged) Merged {
	remote := w.WS().RemoteOf(m.Repo)
	tick := time.NewTicker(followMergeEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return m
		case <-tick.C:
		}
		got, err := gh.MergeStatus(remote.Owner, remote.Name, m.Number, m.id)
		if err != nil {
			execlog.Note(w.Root(), "merge", "asking how merging %s goes: %v", m.Ref(), err)
			continue
		}
		switch got.Status {
		case github.MergePending:
			continue
		case github.MergeMerged:
			m.Status = got.Status
			w.mergeAsked(m.Repo)
			w.afterMerge(gh, &m)
		case github.MergeEnqueued:
			m.Status = got.Status
			w.mergeAsked(m.Repo)
		case github.MergeFailed:
			m.Status = got.Status
			w.mergeAsked(m.Repo)
			w.notify(notification.Notification{Source: notification.SourcePulls, Kind: notification.MergeFailed, Attention: true,
				Text: fmt.Sprintf("GitHub did not merge %s: %s", m.what(), firstNonEmpty(got.Why, "no reason given")), Refs: []string{m.Ref()}})
		}
		return m
	}
}

// afterMerge deletes the merged branches the repo doesn't delete itself.
// What isn't deleted is noted, never in the way: the merge is done.
func (w *Workspace) afterMerge(gh github.Client, m *Merged) {
	remote := w.WS().RemoteOf(m.Repo)
	for _, b := range m.branch {
		if err := gh.DeleteBranch(remote.Owner, remote.Name, b); err != nil {
			execlog.Note(w.Root(), "merge", "%s merged, but its branch %s wasn't deleted: %v", m.Ref(), b, err)
		}
	}
	m.branch = nil
}

// mergeAsked has what's said of the repo's pull requests asked of GitHub
// again: the missions' and the Reviews page's.
func (w *Workspace) mergeAsked(repo string) {
	w.opts.PRs.Forget(repo)
	if svc := w.PRService(); svc != nil {
		svc.Stale()
	}
}

// mergeSettingsFresh is how long what GitHub said of how a repo merges
// is trusted: it's a setting, changed rarely.
const mergeSettingsFresh = 10 * time.Minute

type keptMergeSettings struct {
	mu   sync.Mutex
	kept map[string]keptMergeSetting
}

type keptMergeSetting struct {
	at time.Time
	s  github.MergeSettings
}

// mergeSettings is how the repo's pull requests merge, as GitHub said
// lately.
func (w *Workspace) mergeSettings(gh github.Client, repo string) (github.MergeSettings, error) {
	k := &w.mergeKept
	k.mu.Lock()
	got, ok := k.kept[repo]
	k.mu.Unlock()
	if ok && time.Since(got.at) < mergeSettingsFresh {
		return got.s, nil
	}
	remote := w.WS().RemoteOf(repo)
	s, err := gh.RepoMergeSettings(remote.Owner, remote.Name)
	if err != nil {
		return s, err
	}
	k.mu.Lock()
	if k.kept == nil {
		k.kept = map[string]keptMergeSetting{}
	}
	k.kept[repo] = keptMergeSetting{at: time.Now(), s: s}
	k.mu.Unlock()
	return s, nil
}

// aboutIt is a headline's sentence about a pull request (ref), to follow
// a colon that's named it: "it" in its name's place, and no full stop.
func aboutIt(s, ref string) string {
	s = strings.TrimSuffix(strings.TrimSpace(s), ".")
	switch {
	case strings.HasPrefix(s, ref+"'s "):
		return "its " + strings.TrimPrefix(s, ref+"'s ")
	case strings.HasPrefix(s, ref+": "):
		return strings.TrimPrefix(s, ref+": ")
	case strings.HasPrefix(s, ref+" "):
		return "it " + strings.TrimPrefix(s, ref+" ")
	}
	if rest, ok := strings.CutPrefix(s, "Checks are failing on "+ref); ok {
		return "its checks are failing" + rest
	}
	return s
}
