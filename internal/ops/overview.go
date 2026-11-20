package ops

import (
	"errors"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/orchestrator"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/workspace"
)

// What the web UI's pages show of the workspace, its missions and the
// orchestrator, worked out here (git, GitHub, the court) for web to map to
// what its API serves.

// logTail is how much of a mission's log a list of missions weighs.
const logTail = 8

// Overview is the workspace as its lists show it: its missions, each with
// where it stands, and the kinds of document it has.
type Overview struct {
	Name     string // the workspace's display name, or its folder's
	Missions []MissionItem
	Kinds    []Kind
	// Org and DefaultBranch are where its repos are on GitHub: what a
	// ref's address says.
	Org, DefaultBranch string
	// Spaces are its spaces of documents, as config says them: where a
	// document's address is in Documents rather than Files.
	Spaces []SpaceView
	// Sidebar and Menu are where the web UI's places go, as config says
	// (config.UIConfig): nil for as they come.
	Sidebar, Menu []string
	// Focus is what you've pinned that stands, in your order (focus.go).
	Focus []FocusItem
	// CapsulesSeen moves when a look behind a read finds a capsule other
	// than the read said (keptGit.dirtyMoved): a page showing capsules'
	// statuses that weren't in this view reads them again.
	CapsulesSeen string
}

// MissionItem is a mission in a list: its view, when anything of it last
// changed, the capsules back for re-review, its headline, and each
// existing capsule's HEAD (by repo/name).
type MissionItem struct {
	MissionView
	Updated  time.Time
	ReReview []string
	Headline mission.Headline
	Heads    map[string]string
	// At is where it is on its route, in words (Snapshot.Position).
	At string
}

// Kind is a kind of document the workspace has: where one is filed, its
// guide, and the workspace's passes that are its own.
type Kind struct {
	Name, Title, Where, Guide string
	// Subject is the first line of the commit that changes a document of
	// the kind, for a document of it ("" in a list of kinds).
	Subject string
	Passes  []string
}

// Kinds are the workspace's kinds of document.
func (w *Workspace) Kinds() []Kind {
	ws := w.WS()
	out := make([]Kind, 0, len(ws.Kinds))
	for _, k := range ws.Kinds {
		out = append(out, Kind{Name: k.Name, Title: k.Title, Where: k.Where(), Guide: ws.KindGuide(k), Passes: k.Passes})
	}
	return out
}

// Overview is the workspace's missions as its lists show them, with the
// pull requests GitHub was last asked for (Options.PRs; it's asked again
// in the background when that was a while ago). Missions in play are
// worked out in full; archived ones, after them, are what their rows say.
func (w *Workspace) Overview() (*Overview, error) {
	ws := w.WS()
	missions, err := w.Missions.InPlay()
	if err != nil {
		return nil, err
	}
	out := &Overview{Name: ws.Name, Missions: []MissionItem{}, Kinds: w.Kinds(), Org: ws.Org, DefaultBranch: firstNonEmpty(ws.DefaultBranch, "main"), Spaces: []SpaceView{}}
	out.Sidebar, out.Menu = w.Config().UI.Sidebar, w.Config().UI.Menu
	if moved := w.git.dirtyMoved(); moved != "" {
		h := fnv.New64a()
		h.Write([]byte(moved))
		out.CapsulesSeen = strconv.FormatUint(h.Sum64(), 16)
	}
	if out.Focus, err = w.Focus(); err != nil {
		return nil, err
	}
	colors := w.repoColors()
	for _, s := range ws.Spaces() {
		out.Spaces = append(out.Spaces, SpaceView{Repo: s.Repo, Folder: s.Folder, Label: ws.DisplayNameFor(s.Repo), Color: colors[s.Repo]})
	}
	if out.Name == "" {
		out.Name = filepath.Base(w.Root())
	}
	var idx PRIndex
	if gh := w.GitHub(); w.opts.PRs != nil && gh != nil {
		idx = w.opts.PRs.Peek(ws, gh, MissionRepos(missions))
	}
	off := !w.Config().OrchestratorEnabled()
	for _, v := range buildMissionViews(&w.git, ws, idx, missions) {
		it := MissionItem{MissionView: v, Updated: MissionUpdated(w.Missions, v.M.Slug), Heads: map[string]string{}}
		rrs := ReReviews(ws, v.M)
		for _, rr := range rrs {
			it.ReReview = append(it.ReReview, rr.Capsule)
		}
		brief, _ := w.Missions.ReadBrief(v.M.Slug)
		events, _ := w.Missions.ReadLog(v.M.Slug, logTail)
		snap := MissionSnapshot(v, brief, events, rrs, DocThreadCourt(ws, v, brief), off)
		it.Headline, it.At = mission.Resolve(snap)[0], snap.Position()
		for _, c := range v.Capsules {
			if c.Exists {
				it.Heads[c.Repo+"/"+c.Name] = c.Head
			}
		}
		out.Missions = append(out.Missions, it)
	}
	past, err := w.Missions.Past()
	if err != nil {
		return nil, err
	}
	for _, p := range past {
		out.Missions = append(out.Missions, pastItem(ws, p))
	}
	return out, nil
}

// pastItem is an archived mission in the lists: its row, and whether its
// capsules are still there; no git, nothing of its log or its brief.
func pastItem(ws *workspace.Workspace, p mission.Past) MissionItem {
	m := &mission.Mission{Slug: p.Slug, Title: p.Title, Created: p.Created, State: mission.Archived, Capsules: p.Capsules}
	v := MissionView{M: m, State: mission.Archived, DefaultBranch: ws.DefaultBranch}
	for _, c := range p.Capsules {
		cv := CapsuleView{Capsule: c, Path: filepath.Join(ws.RepoDir(c.Repo), c.Name), Branch: c.Branch}
		if info, err := os.Stat(cv.Path); err == nil && info.IsDir() {
			cv.Exists = true
		}
		v.Capsules = append(v.Capsules, cv)
	}
	return MissionItem{MissionView: v, Updated: p.Changed, Heads: map[string]string{}, Headline: mission.Resolve(mission.Snapshot{M: m, State: mission.Archived})[0]}
}

// MissionUpdated is when a mission's record, brief, log, reviews or
// conversations last changed: the web UI's pages hear of a change by it.
func MissionUpdated(store mission.Store, slug string) time.Time {
	latest := store.Changed(slug)
	for _, c := range []time.Time{conversation.For(store.Root, slug).Changed(), review.For(store.Root, slug).Changed()} {
		if c.After(latest) {
			latest = c
		}
	}
	for _, name := range []string{"brief.md"} {
		if info, err := os.Stat(filepath.Join(store.Dir(), slug, name)); err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	// A conversation's run writes what it's doing as it goes, beside where
	// its lines used to be.
	entries, _ := os.ReadDir(filepath.Join(store.Dir(), slug, "conversations"))
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest
}

// MissionDetail is a mission as its page shows it.
type MissionDetail struct {
	MissionView
	Brief  string
	Plan   string          // plan.md, "" for none
	Events []mission.Event // its whole log
	// Decisions are those standing, then the rest, latest first.
	Decisions         []mission.Decision
	ConversationTitle string // of the conversation it came from
	ReReviews         []review.ReReview
	Headlines         []mission.Headline
	// Cards is the headlines as the page draws them (mission.Cards).
	Cards  []mission.Card
	Stages []mission.StageState // none for a brief with no stages
	// At is where it is on its route, in words (Snapshot.Position), and
	// Z what has to be true for it to end there: its plan's, or its
	// brief's Acceptance.
	At string
	Z  []string
	// Status is each existing capsule's, by repo/name.
	Status    map[string]CapsuleStatus
	Artifacts []ArtifactView
	// Bundles are its verification bundles, oldest first.
	Bundles []BundleView
	Runs    []Run // the orchestrator's latest on it
	// CapsuleRuns are the runs in each capsule (ws run), and Recipes the
	// names of the recipes each can run, by repo/name.
	CapsuleRuns map[string][]runs.Run
	Recipes     map[string][]string
	// CrewPicks are what new crew may be launched on from its page: the
	// default first, then what [crew] allowed lists.
	CrewPicks []harness.Pick
}

// CapsuleStatus is where a capsule stands with git.
type CapsuleStatus struct {
	Dirty         bool
	Ahead, Behind int
	Head          string
}

// missionRuns is how many of the orchestrator's runs a mission's page
// lists, and runsShown how many its own page lists across missions.
const missionRuns, runsShown = 8, 40

// Mission is a mission as its page shows it: with what GitHub says of its
// pull requests (asked, if it hasn't been lately), when withGitHub.
func (w *Workspace) Mission(slug string, withGitHub bool) (*MissionDetail, error) {
	m, err := w.load(slug)
	if err != nil {
		return nil, err
	}
	ws := w.WS()
	var idx PRIndex
	if gh := w.GitHub(); withGitHub && w.opts.PRs != nil && gh != nil {
		idx = w.opts.PRs.Get(ws, gh, MissionRepos([]*mission.Mission{m}))
	}
	v := buildMissionViews(&w.git, ws, idx, []*mission.Mission{m})[0]
	d := &MissionDetail{MissionView: v, Status: map[string]CapsuleStatus{}, Runs: w.runsOf(m, missionRuns)}
	d.CapsuleRuns, d.Recipes = w.capsuleRuns(v)
	d.Brief, _ = w.Missions.ReadBrief(slug)
	d.Plan, _ = w.Missions.ReadPlan(slug)
	d.Events, _ = w.Missions.ReadLog(slug, 0)
	if all, err := w.Missions.Decisions(slug); err == nil {
		d.Decisions = mission.CurrentDecisions(all)
		for i := len(all) - 1; i >= 0; i-- {
			if all[i].State != mission.DecisionCurrent {
				d.Decisions = append(d.Decisions, all[i])
			}
		}
	}
	if ref := m.Conversation; ref != "" {
		store, id := conversation.ForWorkspace(w.Root()), ref
		if from, t, ok := strings.Cut(ref, "/"); ok {
			store, id = conversation.For(w.Root(), from), t
		}
		if c, err := store.Load(id); err == nil {
			d.ConversationTitle = c.Title
		}
	}
	d.ReReviews = ReReviews(ws, m)
	tail := d.Events
	if len(tail) > logTail {
		tail = tail[len(tail)-logTail:]
	}
	snap := MissionSnapshot(v, d.Brief, tail, d.ReReviews, DocThreadCourt(ws, v, d.Brief), !w.Config().OrchestratorEnabled())
	d.Headlines, d.At, d.Z = mission.Resolve(snap), snap.Position(), m.Z(d.Brief)
	d.Cards = mission.CardsOf(snap, d.Headlines)
	if states, _ := m.StageProgress(d.Brief); !(len(states) == 1 && states[0].Implicit) {
		d.Stages = states
	}
	// Each capsule's, asked of git together where it isn't kept: the
	// first read after the daemon starts, or after they've moved.
	statuses := make([]CapsuleStatus, len(v.Capsules))
	var asking sync.WaitGroup
	for i, c := range v.Capsules {
		if !c.Exists {
			continue
		}
		asking.Go(func() {
			st := CapsuleStatus{Dirty: w.git.dirty(c.Path), Head: c.Head}
			if w.git.hasUpstream(c.Path) {
				st.Ahead, st.Behind = w.git.aheadBehind(c.Path)
			} else {
				// Never pushed, it has no upstream to be ahead of: what it
				// has that the default branch hasn't is what it's ahead by
				// (what a pull request would open with).
				st.Ahead = w.git.commitsSince(c.Path, "origin/"+ws.DefaultBranch)
			}
			statuses[i] = st
		})
	}
	asking.Wait()
	for i, c := range v.Capsules {
		if c.Exists {
			d.Status[c.Repo+"/"+c.Name] = statuses[i]
		}
	}
	d.Artifacts = ResolveArtifacts(ws, w.Missions, v)
	d.Bundles = w.Bundles(m)
	d.CrewPicks = w.CrewPicks()
	return d, nil
}

// Run is one of the orchestrator's runs, with the mission it was on, and
// a line of what it said.
type Run struct {
	orchestrator.RunRecord
	Mission, MissionTitle string
	Said                  string
}

func runOf(m *mission.Mission, r orchestrator.RunRecord) Run {
	return Run{RunRecord: r, Mission: m.Slug, MissionTitle: m.Title, Said: orchestrator.Gist(r.Output)}
}

// runsOf is a mission's latest runs, at most n.
func (w *Workspace) runsOf(m *mission.Mission, n int) []Run {
	out := []Run{}
	for i, r := range orchestrator.Runs(w.Missions, m.Slug) {
		if i == n {
			break
		}
		out = append(out, runOf(m, r))
	}
	return out
}

// OrchestratorView is how the orchestrator's set up, as ws works it out
// from the config (Problem, why it can't run), and its latest runs across
// missions.
type OrchestratorView struct {
	Enabled        bool
	Dispatch       string // auto or ask
	MaxDispatches  int
	Harness, Model string
	Problem        string
	Runs           []Run
}

// Orchestrator is how the orchestrator's set up, and what it's done lately.
func (w *Workspace) Orchestrator() OrchestratorView {
	cfg := w.Config()
	v := OrchestratorView{Enabled: cfg.OrchestratorEnabled(), Dispatch: "auto", MaxDispatches: cfg.OrchestratorMaxDispatches(), Runs: []Run{}}
	if !cfg.OrchestratorAutoDispatch() {
		v.Dispatch = "ask"
	}
	pick, h, err := Roles(cfg).ForOrchestrator()
	v.Harness, v.Model = pick.Harness, pick.Model
	switch {
	case err != nil:
		v.Problem = err.Error()
	case h != nil:
		if err := h.Available(); err != nil {
			v.Problem = err.Error()
		}
	}
	missions, _ := w.Missions.List()
	for _, m := range missions {
		v.Runs = append(v.Runs, w.runsOf(m, -1)...)
	}
	sort.SliceStable(v.Runs, func(i, j int) bool { return v.Runs[i].Started.After(v.Runs[j].Started) })
	if len(v.Runs) > runsShown {
		v.Runs = v.Runs[:runsShown]
	}
	return v
}

// OrchestratorRun is one of the orchestrator's runs on a mission, in full.
func (w *Workspace) OrchestratorRun(slug, id string) (*Run, error) {
	m, err := w.load(slug)
	if err != nil {
		return nil, err
	}
	r, err := orchestrator.LoadRun(w.Missions, slug, id)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	run := runOf(m, *r)
	return &run, nil
}
