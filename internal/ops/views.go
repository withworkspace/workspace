package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Views: a mission as every front end shows it, from its record, its
// capsules as git has them, their pull requests and where its threads
// stand. What's built here is data; cli prints it and web maps it to what
// its API serves.

// CapsuleView is a mission capsule with live facts attached.
type CapsuleView struct {
	mission.Capsule
	Path   string
	Exists bool
	Branch string
	Head   string // the commit it's at, for one that exists
	PR     *github.PR
	PRS    mission.PRState
	// Direct: its repo takes commits straight to the default branch, with
	// no pull request. Then Ahead is how many it has to land, Dirty whether
	// it has changes not committed, and Approved whether your latest review
	// of it approves it as it is.
	Direct   bool
	Ahead    int
	Dirty    bool
	Approved bool
}

type MissionView struct {
	M        *mission.Mission
	State    mission.State
	Capsules []CapsuleView
	// CapsulesLanded: every capsule with work has merged its pull request
	// or landed (mission.CapsulesLanded), as far as GitHub was asked.
	CapsulesLanded bool
	// DefaultBranch is the workspace's: where a capsule with no pull
	// request lands.
	DefaultBranch string
}

// PRIndex holds open and merged PRs per repo, keyed by head branch.
type PRIndex map[string]repoPRs

type repoPRs struct{ open, merged map[string]*github.PR }

// FetchPRs fetches open and merged PRs for the given repos, once each and
// in parallel. A nil gh returns an empty index.
func FetchPRs(ws *workspace.Workspace, gh github.Client, repos []string) PRIndex {
	prs := make(PRIndex, len(repos))
	if gh == nil {
		return prs
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, repo := range repos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := repoPRs{open: indexPRs(gh.PRsForRepo(ws.Org, repo))}
			r.merged = indexPRs(gh.MergedPRsForRepo(ws.Org, repo))
			mu.Lock()
			prs[repo] = r
			mu.Unlock()
		}()
	}
	wg.Wait()
	return prs
}

// MissionRepos lists the distinct repos the missions' capsules live in.
func MissionRepos(missions []*mission.Mission) []string {
	seen := map[string]bool{}
	var repos []string
	for _, m := range missions {
		for _, c := range m.Capsules {
			if !seen[c.Repo] {
				seen[c.Repo] = true
				repos = append(repos, c.Repo)
			}
		}
	}
	return repos
}

// BuildMissionViews gathers git and (when gh is non-nil) GitHub facts for
// every capsule of the given missions, fetching each repo's PRs once.
func BuildMissionViews(ws *workspace.Workspace, gh github.Client, missions []*mission.Mission) []MissionView {
	return BuildMissionViewsFrom(ws, FetchPRs(ws, gh, MissionRepos(missions)), missions)
}

// BuildMissionViewsFrom is BuildMissionViews with PRs supplied by the
// caller, so a page can be told of git's changes often while GitHub is
// asked rarely (PRCache). Git is asked of each capsule there and then.
func BuildMissionViewsFrom(ws *workspace.Workspace, prs PRIndex, missions []*mission.Mission) []MissionView {
	return buildMissionViews(askedGit{}, ws, prs, missions)
}

// buildMissionViews is BuildMissionViewsFrom, with what git says of each
// capsule from g: as kept, for a page's view (capsuleGit).
func buildMissionViews(g capsuleGit, ws *workspace.Workspace, prs PRIndex, missions []*mission.Mission) []MissionView {
	views := make([]MissionView, len(missions))
	for i, m := range missions {
		v := MissionView{M: m, DefaultBranch: ws.DefaultBranch}
		facts := map[mission.Capsule]mission.CapsuleFacts{}
		for _, c := range m.Capsules {
			cv := CapsuleView{Capsule: c, Path: filepath.Join(ws.RepoDir(c.Repo), c.Name)}
			if info, err := os.Stat(cv.Path); err == nil && info.IsDir() {
				cv.Exists = true
				cv.Head, cv.Branch = g.headBranch(cv.Path)
			}
			if cv.Branch == "" {
				cv.Branch = c.Branch
			}
			r := prs[c.Repo]
			if pr := LookupPR(r.open, cv.Branch, c.Name); pr != nil {
				cv.PR, cv.PRS = pr, mission.PROpen
			} else if pr := LookupPR(r.merged, cv.Branch, c.Name); pr != nil {
				cv.PR, cv.PRS = pr, mission.PRMerged
			}
			if cv.Direct = ws.Direct(c.Repo); cv.Direct && cv.Exists && c.Landed == nil {
				cv.Ahead = g.commitsSince(cv.Path, "origin/"+ws.DefaultBranch)
				if cv.Ahead > 0 {
					cv.Dirty = g.dirty(cv.Path)
					r, _ := review.For(ws.Root, m.Slug).Latest(c.Repo + "/" + c.Name)
					cv.Approved = r != nil && r.Verdict == review.Approve && cv.Head != "" && r.Head == cv.Head
				}
			}
			facts[capsuleKey(c)] = mission.CapsuleFacts{Exists: cv.Exists, PR: cv.PRS, Landed: c.Landed != nil}
			v.Capsules = append(v.Capsules, cv)
		}
		v.State = mission.DeriveState(m, func(c mission.Capsule) mission.CapsuleFacts { return facts[capsuleKey(c)] })
		v.CapsulesLanded = mission.CapsulesLanded(m, func(c mission.Capsule) mission.CapsuleFacts { return facts[capsuleKey(c)] })
		views[i] = v
	}
	return views
}

func indexPRs(list []github.PR, err error) map[string]*github.PR {
	if err != nil {
		return nil
	}
	m := make(map[string]*github.PR, len(list))
	for i := range list {
		m[list[i].HeadRefName] = &list[i]
	}
	return m
}

// capsuleKey is a capsule as a map key: its repo and name, without what's
// recorded about it.
func capsuleKey(c mission.Capsule) mission.Capsule {
	return mission.Capsule{Repo: c.Repo, Name: c.Name}
}

// prCacheFor is how long a repo's PRs, once fetched, answer for the web UI:
// an open page refetches on every change, and GitHub is slow.
const prCacheFor = time.Minute

type PRCache struct {
	mu       sync.Mutex
	at       map[string]time.Time
	prs      PRIndex
	fetching map[string]bool
	// fetched counts the answers GitHub has given: a view that shows them
	// has changed when it moves (Stamp).
	fetched int64
	// news is told when a page's fetch (Peek) comes back with something
	// changed: the watcher acts on it then (News).
	news chan struct{}
}

// News is told when GitHub's answer to a page's fetch isn't what the cache
// held: a pull request merged, say. The daemon's watcher looks then, so
// what a page shows is acted on at once and not at the watcher's next
// look. One telling stands for any number not yet taken.
func (c *PRCache) News() <-chan struct{} {
	if c == nil {
		return nil
	}
	return c.news
}

// Fetched counts the answers the cache has had from GitHub.
func (c *PRCache) Fetched() int64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fetched
}

func NewPRCache() *PRCache {
	return &PRCache{at: map[string]time.Time{}, prs: PRIndex{}, fetching: map[string]bool{}, news: make(chan struct{}, 1)}
}

// peek is the repos' PRs as last fetched, without waiting on GitHub: repos
// not fetched lately are fetched in the background, for the next ask.
func (c *PRCache) Peek(ws *workspace.Workspace, gh github.Client, repos []string) PRIndex {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := PRIndex{}
	var stale []string
	for _, r := range repos {
		if pr, ok := c.prs[r]; ok {
			out[r] = pr
		}
		if gh != nil && time.Since(c.at[r]) > prCacheFor && !c.fetching[r] {
			c.fetching[r] = true
			stale = append(stale, r)
		}
	}
	if len(stale) > 0 {
		go func() {
			fresh := FetchPRs(ws, gh, stale)
			c.mu.Lock()
			defer c.mu.Unlock()
			changed := false
			for _, r := range stale {
				changed = changed || !reflect.DeepEqual(c.prs[r], fresh[r])
				c.prs[r], c.at[r] = fresh[r], time.Now()
				delete(c.fetching, r)
			}
			c.fetched++
			if changed {
				select {
				case c.news <- struct{}{}:
				default: // the watcher's already been told
				}
			}
		}()
	}
	return out
}

// Forget has a repo's PRs asked of GitHub again when they're next wanted:
// ws has just opened one there.
func (c *PRCache) Forget(repo string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.at, repo)
}

// get is the repos' PRs, fetching those not fetched lately.
func (c *PRCache) Get(ws *workspace.Workspace, gh github.Client, repos []string) PRIndex {
	c.mu.Lock()
	var stale []string
	for _, r := range repos {
		if time.Since(c.at[r]) > prCacheFor {
			stale = append(stale, r)
		}
	}
	c.mu.Unlock()
	fresh := FetchPRs(ws, gh, stale)
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range stale {
		c.prs[r], c.at[r] = fresh[r], time.Now()
	}
	if len(stale) > 0 {
		c.fetched++
	}
	out := PRIndex{}
	for _, r := range repos {
		out[r] = c.prs[r]
	}
	return out
}

// MissionSnapshot gathers what mission.Resolve needs from a mission's view
// (capsules and PRs), brief, log, reviews and threads.
func MissionSnapshot(v MissionView, brief string, events []mission.Event, rr []review.ReReview, threads mission.DocThreads, orchestratorOff bool) mission.Snapshot {
	s := mission.Snapshot{M: v.M, State: v.State, Brief: brief, Events: events, OrchestratorOff: orchestratorOff, Now: time.Now(), Threads: threads, CapsulesLanded: v.CapsulesLanded}
	for _, c := range v.Capsules {
		if c.Direct && c.Ahead > 0 {
			s.Direct = append(s.Direct, mission.DirectFacts{Capsule: c.Repo + "/" + c.Name, Branch: v.DefaultBranch, Ahead: c.Ahead, Dirty: c.Dirty, Approved: c.Approved})
		}
		if c.PR == nil {
			continue
		}
		s.PRs = append(s.PRs, mission.PRFacts{
			Capsule: c.Repo + "/" + c.Name, Ref: fmt.Sprintf("%s#%d", c.Repo, c.PR.Number), URL: c.PR.URL,
			Merged: c.PRS == mission.PRMerged, Review: c.PR.ReviewDecision, Checks: c.PR.StatusRollup,
		})
	}
	for _, r := range rr {
		t := r.Review.Tally()
		s.ReReview = append(s.ReReview, mission.ReReview{Capsule: r.Capsule, Review: r.Review.ID, Answered: t.Answered(), Total: t.Total()})
	}
	return s
}

// DocThreadCourt is where the threads on the mission's documents stand:
// those waiting on you (someone answered), those an agent is answering,
// and for the current stage's documents, the changes asked for that hold
// up accepting it, and those with every thread resolved, to approve.
func DocThreadCourt(ws *workspace.Workspace, v MissionView, brief string) mission.DocThreads {
	store := conversation.For(ws.Root, v.M.Slug)
	threads, _ := store.List()
	var out mission.DocThreads
	add := func(list *[]mission.DocReplies, c *conversation.Conversation) {
		for i := range *list {
			if (*list)[i].Doc == c.Anchor.Doc {
				(*list)[i].Waiting++
				(*list)[i].Threads = append((*list)[i].Threads, c.ID)
				return
			}
		}
		*list = append(*list, mission.DocReplies{Doc: c.Anchor.Doc, Name: c.Anchor.Path, Waiting: 1, Threads: []string{c.ID}})
	}
	for _, c := range threads {
		if c.Goal == conversation.GoalBrief && c.Status == conversation.Open {
			// Its agent's at work while its turn is marked underway on the
			// mission: set as ws asks for it, cleared once its reply is
			// recorded (Mission.Planning). Not whether a process is alive,
			// nor for how long.
			b := &mission.BriefingTalk{ID: c.ID, Answering: planningIn(v.M, c.ID), Waiting: c.WaitingOnYou()}
			for _, p := range c.Proposals {
				b.Proposed = b.Proposed || (p.Kind == conversation.ProposeBrief && p.State == conversation.ProposalOpen)
				if p.Kind == conversation.ProposePlan && p.State == conversation.ProposalOpen {
					b.PlanWhy = firstNonEmpty(p.Why, "a change to the plan, yours to confirm")
				}
			}
			out.Briefing = b
		}
		if c.Kind == conversation.Talk && c.Status == conversation.Open {
			// What it proposed for you to confirm, but what the briefing's
			// and the planner's own headlines say (the brief while it's
			// briefed, a revision of the plan).
			var kinds []string
			for _, p := range c.Proposals {
				if p.State != conversation.ProposalOpen {
					continue
				}
				if c.Goal == conversation.GoalBrief && (p.Kind == conversation.ProposePlan || v.M.ToBrief() && p.Kind == conversation.ProposeBrief) {
					continue
				}
				kinds = append(kinds, p.Kind)
			}
			if len(kinds) > 0 {
				out.Proposed = append(out.Proposed, mission.TalkProposed{ID: c.ID, Kinds: kinds})
			}
		}
		switch {
		case c.Anchor.Doc == "":
		case c.Answering && c.Status == conversation.Open: // not drafting a decision from a resolved one
			add(&out.Answering, c)
		case c.WaitingOnYou():
			add(&out.Replies, c)
		}
	}
	if len(threads) == 0 {
		return out
	}
	_, docs := v.M.CurrentStageArtifacts(brief)
	var approvals map[string]conversation.Approval
	for _, a := range docs {
		var on []*conversation.Conversation
		open := false
		for _, c := range threads {
			if c.Anchor.Doc == a.Ref {
				on = append(on, c)
				open = open || c.Status == conversation.Open
			}
		}
		out.Blocking = append(out.Blocking, conversation.OpenChanges(on, a.Ref)...)
		if len(on) == 0 || open {
			continue
		}
		if approvals == nil {
			approvals, _ = store.Approvals()
		}
		av := ResolveArtifact(ws, mission.Store{Root: ws.Root}, v, a)
		if av.Path == "" || av.State == "missing" {
			continue
		}
		text, err := os.ReadFile(av.Path)
		if err != nil {
			continue
		}
		if ap, ok := approvals[a.Ref]; ok && ap.Version == doc.Version(text) {
			continue
		}
		var ids []string
		for _, c := range on {
			ids = append(ids, c.ID)
		}
		out.Approve = append(out.Approve, mission.DocReplies{Doc: a.Ref, Name: on[0].Anchor.Path, Threads: ids})
	}
	return out
}

// ReReviews are the mission's capsules back with the user to re-review:
// their latest review asked for changes, they've moved on, and nobody's
// working in them.
func ReReviews(ws *workspace.Workspace, m *mission.Mission) []review.ReReview {
	head := func(capsule string) string {
		repo, name, _ := strings.Cut(capsule, "/")
		dir := filepath.Join(ws.RepoDir(repo), name)
		if _, err := os.Stat(dir); err != nil {
			return ""
		}
		return workspace.GitRevParse(dir, "HEAD")
	}
	rr, _ := review.For(ws.Root, m.Slug).AwaitingReReview(m, head)
	return rr
}
