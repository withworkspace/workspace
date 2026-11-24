package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/store"
	"reflect"
)

// What the API serves of the workspace, its missions and the orchestrator:
// ops's views (ops.Overview, ops.MissionDetail, ops.Run) mapped to the
// wire types web/src/api.ts reads.

// Missions is what the workspace's pages are built from: its missions, a
// mission's page, and the orchestrator's.
type Missions interface {
	Overview() (*ops.Overview, error)
	// StartAbout makes a mission about something ws names, to brief.
	StartAbout(name string) (*mission.Mission, error)
	Mission(slug string) (*ops.MissionDetail, error)
	Orchestrator() ops.OrchestratorView
	OrchestratorRun(slug, id string) (*ops.Run, error)
	// Stamp names what the workspace's view is built from, cheaply: it
	// moves when that's changed. MissionStamp is a mission's page's, but
	// for its records, which say themselves when they change.
	Stamp() string
	MissionStamp(slug string) string
	// ConversationStamp is a conversation's (in a mission, or the
	// workspace's for none) but for its records: what its agent has
	// written of its steps as it answers.
	ConversationStamp(scope, id string) string
	// KeepMission reads where a mission's capsules stand, where that isn't
	// kept: before its page is told they've moved, so the page's read
	// waits on nothing.
	KeepMission(slug string)
	// Seen says when what was read behind a page's read, which didn't
	// wait on it, has found something the page wasn't given: the
	// workspace's view is looked at then, not at the next poll.
	Seen() <-chan struct{}
	// WatchGit watches where git writes (each capsule's git folder, each
	// repo's) until stop closes, and says on Seen when it has: a commit
	// in a capsule reaches its pages then, not at the next poll.
	WatchGit(stop <-chan struct{}, logf func(string, ...any))
	// BundleFile is a file of one of a mission's bundles: where it is, and
	// its content type.
	BundleFile(slug, id, name string) (path, media string, err error)
	// Artifacts is the workspace's artifacts: on this machine, and in the
	// repo it keeps them in, with crew's in its capsules; ArtifactsConfig
	// where they're kept, and how long a live one lasts.
	Artifacts() artifact.Store
	ArtifactsConfig() artifact.Config
}

// viewCache keeps views as they were last built, as the JSON they're
// served as, each with the stamp it was built at: the workspace's (GET
// /api/workspace) and each mission's (GET /api/missions/{slug}). Told one
// has changed, every page showing it asks for it at once (the workspace's
// hub has just built it, to see whether it had): built once, it's served to
// them all. It's built again when its stamp has moved (a write to the
// records, a file, a capsule) or a poll's passed since (for what a stamp
// can't see, as the hub's floor is for), after a write a page makes
// (forget), or for another App (a test's).
type viewCache struct {
	mu    sync.Mutex // held while one's built, so those asking at once wait for one build
	views map[string]*keptView
	built map[string]int // how many times each has been built: what a test counts
}

type keptView struct {
	stamp string
	app   uintptr
	at    time.Time
	data  []byte
}

// get is the view at key, as JSON: as kept, if it was built at its stamp
// as it is now within a poll, by this App; else built now (build), and
// kept. The stamp's taken once those asking before are done, so one
// asking while it's built is served what was built. A view that can't be
// built isn't kept, and its error is build's.
func (c *viewCache) get(s *Server, key string, stampOf func() string, build func() (any, error)) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stamp, app := stampOf(), appID(s.App)
	if v := c.views[key]; v != nil && v.stamp == stamp && app != 0 && v.app == app && time.Since(v.at) < s.Poll {
		return v.data, nil
	}
	if c.views == nil {
		c.views = map[string]*keptView{}
	}
	// Those nobody's asked for in a while go: a mission closed.
	for k, v := range c.views {
		if time.Since(v.at) > time.Minute {
			delete(c.views, k)
		}
	}
	if c.built == nil {
		c.built = map[string]int{}
	}
	c.built[key]++
	delete(c.views, key)
	v, err := build()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	c.views[key] = &keptView{stamp: stamp, app: app, at: time.Now(), data: data}
	return data, nil
}

// workspace is the workspace's view as GET /api/workspace serves it.
func (c *viewCache) workspace(s *Server) ([]byte, error) {
	return c.get(s, "workspace", func() string { return s.App.Stamp() + " " + s.keptStamp() }, func() (any, error) { return s.workspaceView() })
}

// mission is a mission's view as GET /api/missions/{slug} serves it: its
// stamp is what its page is built from but its records (MissionStamp),
// with the records' change number, which every write moves.
func (c *viewCache) mission(s *Server, slug string) ([]byte, error) {
	stamp := func() string {
		var seq int64
		if db, err := store.Dial(s.Root); err == nil {
			seq, _ = db.Seq(context.Background())
		}
		return fmt.Sprintf("%s %d %s", s.App.MissionStamp(slug), seq, s.keptStamp())
	}
	return c.get(s, missionTopic+slug, stamp, func() (any, error) { return s.missionDetail(slug) })
}

// forget drops what's kept: what it was built from has changed in a way
// its stamp doesn't say (a write a page made, a test's App changed in
// place).
func (c *viewCache) forget() {
	c.mu.Lock()
	c.views = nil
	c.mu.Unlock()
}

// appID is which App a is, by its pointer: 0 for one that isn't one, whose
// view isn't kept.
func appID(a App) uintptr {
	v := reflect.ValueOf(a)
	if v.Kind() != reflect.Pointer {
		return 0
	}
	return v.Pointer()
}

// workspaceView is what GET /api/workspace serves.
func (s *Server) workspaceView() (Workspace, error) {
	v, err := WorkspaceOf(s.App, s.Root, s.Reviews)
	// The daemon's own, which is what it gives its plugin host too: so a
	// plugin's app is told the ws its plugin was.
	v.Version = s.Version
	// A release later than it, as the daemon last found: the menu says so.
	v.Later = s.App.LaterWS()
	v.GroundsSeen, v.ReadsSeen = s.groundsSeen(), seenOf(s.reads.Seen())
	return v, err
}

// WorkspaceOf is the workspace at root as GET /api/workspace serves it:
// its missions with what git knows of their capsules, the pull requests
// asked for lately (GitHub is slow: it's asked in the background), and
// where each mission stands; and its conversations, pages, documents and
// the pull requests Reviews lists (reviews; nil for none).
func WorkspaceOf(a Missions, root string, reviews *pulls.Service) (Workspace, error) {
	v, err := a.Overview()
	if err != nil {
		return Workspace{}, err
	}
	// The workspace's conversations, read once for the list and for what
	// of them needs you.
	talks := conversation.ForWorkspace(root)
	list, _ := talks.Talks()
	out := Workspace{Name: v.Name, Root: root, Org: v.Org, Remotes: v.Elsewhere, DefaultBranch: v.DefaultBranch, Missions: []Mission{}, Conversations: talksOf(talks, list), Pages: Pages(root), Artifacts: ArtifactsOf(a.Artifacts(), a.ArtifactsConfig()), ArtifactsRepo: a.ArtifactsConfig().Repo, Kinds: docKinds(v.Kinds), Docs: DocsUpdated(root), Reviews: reviewsCount(reviews), Spaces: []SpaceRef{}}
	var asked []pulls.Asked
	if reviews != nil {
		asked = reviews.AskedOfYou()
	}
	out.Needs = needsOf(ops.Needs(v.Missions, list, asked))
	out.Focus = focusOf(v.Focus)
	out.Apps = []AppInfo{}
	for _, a := range v.Apps {
		out.Apps = append(out.Apps, AppInfo{Name: a.Name, Title: a.Title, Icon: a.Icon})
	}
	out.SpacesSeen = v.SpacesSeen
	out.CapsulesSeen = v.CapsulesSeen
	if v.Sidebar != nil || v.Menu != nil {
		out.Places = &Places{Sidebar: append([]string{}, v.Sidebar...), Menu: append([]string{}, v.Menu...)}
	}
	for _, s := range v.Spaces {
		out.Spaces = append(out.Spaces, SpaceRef{Repo: s.Repo, Folder: s.Folder, Label: s.Label, Color: s.Color})
	}
	for _, it := range v.Missions {
		m := Mission{Slug: it.M.Slug, Title: it.M.Title, State: it.State, Created: it.M.Created, Capsules: []Capsule{}, Crew: []Crew{},
			Updated: it.Updated, ReReview: it.ReReview, Headline: headline(it.Headline), At: it.At, About: it.M.About}
		if t := it.M.Asking(); t != nil {
			m.Flag = t.Text
		}
		for _, c := range it.Capsules {
			m.Capsules = append(m.Capsules, Capsule{Repo: c.Repo, Name: c.Name, Branch: c.Branch, Path: c.Path, Exists: c.Exists, Head: it.Heads[c.Repo+"/"+c.Name]})
		}
		for _, c := range it.M.Crew {
			m.Crew = append(m.Crew, crewOf(c))
		}
		out.Missions = append(out.Missions, m)
	}
	return out, nil
}

// needsOf is what needs you as the web UI is given it: each with where
// it's dealt with there.
func needsOf(needs []ops.Need) []Need {
	out := []Need{}
	for _, n := range needs {
		w := Need{Key: n.Key, Of: NeedOf(n.Of), Ref: n.Ref, In: n.In, Kind: n.Kind, Label: n.Label, Title: n.Title, Text: n.Text}
		if !n.Since.IsZero() {
			since := n.Since
			w.Since = &since
		}
		switch {
		case n.Of == ops.NeedPull:
			// A pull request's ref is repo#number: its review is at both.
			if at := strings.LastIndex(n.Ref, "#"); at > 0 {
				w.To = "/reviews/" + n.Ref[:at] + "/" + n.Ref[at+1:]
			}
		case n.Of == ops.NeedMission:
			w.To = "/missions/" + n.Ref
		case n.In != "":
			w.To = "/missions/" + n.In + "/conversations/" + n.Ref
		default:
			w.To = "/conversations/" + n.Ref
		}
		out = append(out, w)
	}
	return out
}

func focusOf(items []ops.FocusItem) []FocusPin {
	out := []FocusPin{}
	for _, it := range items {
		out = append(out, FocusPin{Name: it.Name, Kind: it.Kind, Title: it.Title, State: it.State})
	}
	return out
}

// reviewsCount is how many pull requests Reviews lists, as last asked.
func reviewsCount(reviews *pulls.Service) *ReviewsCount {
	if reviews == nil {
		return nil
	}
	l := reviews.Peek()
	if l == nil {
		return nil
	}
	return &ReviewsCount{Count: l.Count, At: l.At}
}

func docKinds(kinds []ops.Kind) []DocKind {
	out := make([]DocKind, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, DocKind{Name: k.Name, Title: k.Title, Where: k.Where, Guide: k.Guide, Passes: k.Passes})
	}
	return out
}

func crewOf(c mission.Crew) Crew {
	w := Crew{ID: c.ID, Harness: c.Harness, Model: c.Model, Role: c.Role, Capsule: c.Capsule, Started: c.Started,
		Ended: c.Ended != nil, Starting: c.Ended == nil && c.Starting != nil,
		Activity: CrewActivity(c.Activity), Status: c.Status, LastReply: c.LastReply, Notice: c.Notice, Options: c.Options,
		Structured: harness.CapsOf(c.Harness).Crew == harness.CrewACP}
	if work, ok := c.OwnWork(time.Now()); ok {
		w.WaitingOn = mission.WorkWhat(work)
	}
	w.Again = workingAgain(c)
	return w
}

// workingAgain is crew that reported done and has been at work since: its
// done stands for ws (it handed its work back), but what it's doing now is
// working on what it was sent back with.
func workingAgain(c mission.Crew) bool {
	return c.Ended == nil && c.Status == mission.CrewDone && c.Activity == mission.ActivityWorking &&
		c.StatusAt != nil && c.ActivityAt != nil && c.ActivityAt.After(*c.StatusAt)
}

// headline is a headline as the API serves it.
func headline(h mission.Headline) Headline {
	w := Headline{
		Court: h.Court, Kind: string(h.Kind), Label: h.Label, Text: h.Text, By: h.By,
		Group:  HeadlineGroup(strings.ToLower(strings.ReplaceAll(mission.GroupNames[h.Group()], " ", "-"))),
		Action: Action{Verb: string(h.Action.Verb), Label: h.Action.Label, Target: h.Action.Target, URL: h.Action.URL, Options: h.Action.Options},
	}
	for _, l := range h.Action.Look {
		w.Action.Look = append(w.Action.Look, Look{Bundle: l.Bundle, Capsule: l.Capsule, Doc: l.Doc, Name: l.Name, Read: l.Read, Approved: l.Approved})
	}
	if !h.Since.IsZero() {
		since := h.Since
		w.Since = &since
	}
	if c := h.Checking; c != nil {
		w.Checking = &Checking{Step: c.Step, N: c.N, Of: c.Of, Command: c.Command, Line: c.Line, Judged: c.Judged, Judge: c.Judge, Since: c.Since, Until: c.Until}
	}
	return w
}

// conclusionOf is how a mission ends, as the API serves it.
func conclusionOf(c mission.Conclusion) *Conclusion {
	out := &Conclusion{Ending: c.Ending, Text: c.Text, To: c.To, By: c.By}
	if !c.At.IsZero() {
		out.At = c.At.UTC().Format(time.RFC3339)
	}
	for _, z := range c.Z {
		out.Z = append(out.Z, ZCheck{Z: z.Z, Met: z.Met, How: z.How})
	}
	return out
}

// missionDetail is GET /api/missions/{slug}.
func (s *Server) missionDetail(slug string) (*MissionDetail, error) {
	return MissionOf(s.App, s.Root, slug)
}

// MissionOf is a mission as GET /api/missions/{slug} serves it: with what
// GitHub last said of its pull requests, never waiting on it (its event
// moves when GitHub, asked behind the read, says something different).
func MissionOf(a Missions, root, slug string) (*MissionDetail, error) {
	v, err := a.Mission(slug)
	if err != nil {
		return nil, err
	}
	m, now := v.M, time.Now()
	d := &MissionDetail{
		Slug: m.Slug, Title: m.Title, State: v.State, Created: m.Created, Request: m.Request, From: m.From, About: m.About,
		Brief: v.Brief, Plan: v.Plan, Conversation: m.Conversation, ConversationTitle: v.ConversationTitle, GitHub: v.GitHub,
		Stages: []Stage{}, Capsules: []CapsuleDetail{}, Crew: []CrewDetail{}, Artifacts: []Artifact{}, Events: []Event{},
		Runs: []OrchestratorRun{}, Decisions: []Decision{}, Bundles: []Bundle{},
	}
	for _, b := range v.Bundles {
		d.Bundles = append(d.Bundles, bundleOf(m.Slug, b, v.Status[b.Capsule].Head))
	}
	for _, r := range v.Runs {
		d.Runs = append(d.Runs, orchestratorRun(r))
	}
	for _, dc := range v.Decisions {
		out := Decision{ID: dc.ID, What: dc.What, Why: dc.Why, By: dc.By, From: dc.From, About: dc.About,
			At: dc.At, Step: dc.Step, State: DecisionState(dc.State), SupersededBy: dc.SupersededBy, WithdrawnWhy: dc.WithdrawnWhy}
		if dc.Asked != nil {
			out.Asked = &DecisionAsked{By: dc.Asked.By, Text: dc.Asked.Text}
		}
		d.Decisions = append(d.Decisions, out)
	}
	talks := conversation.For(root, slug)
	d.Talks = TalksIn(talks)
	if c := talks.Planner(); c != nil {
		d.Planner = c.ID
	}
	d.BriefUnwritten = mission.IsUnwrittenBrief(v.Brief, m.Title)
	if t := m.Asking(); t != nil {
		d.Flag = &Proposal{Text: t.Text, By: t.By, Options: t.Options}
	}
	for _, t := range m.Tasks {
		d.Tasks = append(d.Tasks, Task{N: t.N, Verb: TaskVerb(t.Verb), Text: t.Text, Step: t.Step, Options: t.Options, By: t.By, At: t.At})
	}
	if dp := m.Dispatch; dp != nil {
		d.Dispatch = &Proposal{Text: dp.Text, By: dp.By, Capsule: dp.Capsule}
	}
	if c := m.Conclude; c != nil && v.State != mission.Archived {
		d.Conclude = conclusionOf(*c)
	}
	if e := m.Ending; e != nil {
		d.Ending = conclusionOf(*e)
	}
	for _, e := range m.EndedCapsules {
		d.EndedCapsules = append(d.EndedCapsules, EndedCapsule{Repo: e.Repo, Name: e.Name, Ending: CapsuleEnding(e.Ending), To: e.To, Why: e.Why, By: e.By, At: e.At.UTC().Format(time.RFC3339)})
	}
	d.At, d.Z = v.At, v.Z
	for _, rr := range v.ReReviews {
		t := rr.Review.Tally()
		d.ReReview = append(d.ReReview, ReReview{Capsule: rr.Capsule, Review: rr.Review.ID, Answered: t.Answered(), Total: t.Total()})
	}
	for _, h := range v.Headlines {
		d.Headlines = append(d.Headlines, headline(h))
	}
	d.Cards = v.Cards
	if d.Cards == nil {
		d.Cards = []mission.Card{}
	}
	if m.PlanErr != nil {
		d.PlanProblem = m.PlanErr.Error()
	}
	for _, st := range v.Stages {
		sg := Stage{N: st.N, Title: st.Short(), Text: st.Plain(), State: "todo", CheckIn: st.CheckIn}
		if s := st.Step; s != nil {
			sg.Type, sg.Who, sg.WhoSays, sg.Makes, sg.OpensPR = StepType(s.Type), StepWho(s.Who), s.WhoSays(), s.Makes, s.MakesPR()
			sg.In = s.Capsule
		}
		if st.Record != nil {
			sg.Crew, sg.Why = st.Record.Crew, st.Record.Why
			if !st.Accepted() {
				sg.Deadline, sg.OutOfTime = st.Record.Deadline, st.Record.OutOfTime != nil
			}
		}
		if s := st.Step; s != nil {
			sg.OnlyIf, sg.InEach, sg.Held, sg.Skipped = s.OnlyIf, s.InEach, st.Held(), st.Skipped()
			if st.Record != nil {
				sg.Attempts = st.Record.Attempts
			}
			for _, in := range st.Instances() {
				sg.Instances = append(sg.Instances, StageRun{Repo: in.Repo, Capsule: in.Capsule, Done: in.Done != nil})
			}
			sg.CarryOn, sg.Limit = s.CarryOn, s.LimitText
			round, _ := st.Record.LatestChecks()
			sg.Round = round.N
			if s.Type == mission.StepWait {
				// Its checks aren't run in rounds: ws looks, every so often,
				// and its last look is what they came to.
				sg.On, sg.Passes = s.Whom, PassesSaid
				switch {
				case s.Watched():
					sg.Passes = PassesWatched
				case s.WaitOnYou():
					sg.Passes = PassesDone
				}
				if s.WaitOnYou() {
					sg.On = "you"
				}
				if w := st.Watch(); w != nil {
					sg.Watch = &StageWatch{Since: w.Since, Looked: w.Looked, Looks: w.Looks, Came: w.Summary()}
					round = mission.CheckRound{Results: w.Results}
				}
			}
			for i, c := range s.Checks {
				sc := StageCheck{Text: c.Text, Run: c.Run()}
				if i < len(round.Results) && round.Results[i].Check == c.Text {
					r := round.Results[i]
					sc.Result, sc.Evidence, sc.By, sc.Kept = CheckVerdict(r.Result), r.Evidence, r.By, r.Run
				}
				sg.Checks = append(sg.Checks, sc)
			}
		}
		switch {
		case st.Accepted():
			sg.State = "accepted"
		case st.Current:
			sg.State = "current"
		}
		d.Stages = append(d.Stages, sg)
	}
	for _, c := range v.Capsules {
		cd := CapsuleDetail{Capsule: Capsule{Repo: c.Repo, Name: c.Name, Branch: c.Branch, Path: c.Path, Exists: c.Exists}}
		if st, ok := v.Status[c.Repo+"/"+c.Name]; ok {
			cd.Dirty, cd.Ahead, cd.Behind, cd.Head, cd.Unread = st.Dirty, st.Ahead, st.Behind, st.Head, st.Unread
		}
		if c.PR != nil {
			cd.PR = &PR{Number: c.PR.Number, Title: c.PR.Title, URL: c.PR.URL, Merged: c.PRS == mission.PRMerged, Checks: c.PR.StatusRollup}
		}
		cd.Direct, cd.ToLand = c.Direct, c.Ahead
		if c.Landed != nil {
			cd.Landed = &c.Landed.At
		}
		for _, r := range v.CapsuleRuns[c.Repo+"/"+c.Name] {
			cd.Runs = append(cd.Runs, RunDetailOf(r, root, now))
		}
		cd.Recipes, cd.Tasks = v.Recipes[c.Repo+"/"+c.Name], v.Tasks[c.Repo+"/"+c.Name]
		d.Capsules = append(d.Capsules, cd)
	}
	for _, c := range m.Crew {
		cd := CrewDetail{Crew: crewOf(c), EndedAt: c.Ended, Exit: c.Exit, Note: c.Note, Dispatch: c.Dispatch}
		switch {
		case c.Ended == nil:
		case c.Interrupted:
			cd.How = mission.EndedInterrupted
		case c.Stopped && c.Status == mission.CrewDone:
			cd.How = mission.EndedHandedBack
		case c.Stopped:
			cd.How = mission.EndedStopped
		}
		if c.HandedOff != nil {
			cd.Handoff = mission.HandoffRef(c.ID)
		}
		d.Crew = append(d.Crew, cd)
	}
	threads := docThreads(root, slug)
	arts := artifact.Store{Root: root}
	for _, av := range v.Artifacts {
		a := Artifact{
			Ref: av.Ref, Kind: av.Kind, Label: av.Label, State: av.State, URL: av.URL, Path: av.Path,
			By: av.Crew, Added: av.Added, Readable: av.State != "missing" && av.Path != "" && isFile(av.Path),
			Threads: threads[av.Ref],
		}
		// An artifact (a3) is opened at /artifacts/a3, as the records last
		// had it: titled as it is now, with its versions.
		if av.Parsed.Loc == ops.LocArtifact {
			if e, ok := arts.Known(av.Parsed.ID); ok {
				a.Label, a.Versions = e.Title, len(e.Versions)
			}
		}
		d.Artifacts = append(d.Artifacts, a)
	}
	for _, e := range v.Events {
		if text, ok := mission.Sentence(e); ok {
			ev := Event{Time: e.Time, Kind: e.Kind, Text: text, By: e.Crew}
			if e.Kind == mission.EventResumeWith || e.Kind == mission.EventTold {
				ev.By, ev.To = e.By, e.Crew
			}
			d.Events = append(d.Events, ev)
		}
	}
	d.CrewPicks = crewPicks(v.CrewPicks)
	return d, nil
}

// crewPicks are the picks a mission's page offers new crew: the first is
// the default.
func crewPicks(picks []harness.Pick) CrewPicks {
	out := CrewPicks{Allowed: []Pick{}}
	for i, p := range picks {
		if i == 0 {
			out.Default = Pick{Harness: p.Harness, Model: p.Model}
		} else {
			out.Allowed = append(out.Allowed, Pick{Harness: p.Harness, Model: p.Model})
		}
	}
	return out
}

// docThreads is where the open threads on a mission's documents stand, by
// the documents' refs.
func docThreads(root, slug string) map[string]*ArtifactThreads {
	out := map[string]*ArtifactThreads{}
	threads, _ := conversation.For(root, slug).List()
	for _, c := range threads {
		if c.Anchor.Doc == "" || c.Status != conversation.Open {
			continue
		}
		at := out[c.Anchor.Doc]
		if at == nil {
			at = &ArtifactThreads{}
			out[c.Anchor.Doc] = at
		}
		at.Open++
		if c.WaitingOnYou() {
			at.Waiting++
		}
	}
	return out
}

// orchestratorRun is one of the orchestrator's runs in brief.
func orchestratorRun(r ops.Run) OrchestratorRun {
	w := OrchestratorRun{Mission: r.Mission, MissionTitle: r.MissionTitle, ID: r.ID, Started: r.Started, Ended: r.Ended,
		For: r.For, Trigger: r.Trigger, Harness: r.Harness, Model: r.Model, Retry: r.Retry, Acted: r.Acted, Error: r.Error,
		Said: r.Said, Calls: len(r.Calls)}
	for _, c := range r.Calls {
		if c.Refused {
			w.Refused++
		}
	}
	return w
}

// isFile is whether p is a file there is.
func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// OrchestratorOf is GET /api/orchestrator: how the orchestrator's set up,
// and its latest runs across missions.
func OrchestratorOf(a Missions) OrchestratorInfo {
	v := a.Orchestrator()
	info := OrchestratorInfo{Enabled: v.Enabled, Dispatch: Dispatch(v.Dispatch), MaxDispatches: v.MaxDispatches, Harness: v.Harness, Model: v.Model, Problem: v.Problem, Runs: []OrchestratorRun{}}
	for _, r := range v.Runs {
		info.Runs = append(info.Runs, orchestratorRun(r))
	}
	return info
}

// OrchestratorRunOf is one of the orchestrator's runs, in full: ErrNotFound
// for none.
func OrchestratorRunOf(a Missions, slug, id string) (*OrchestratorRunDetail, error) {
	run, err := a.OrchestratorRun(slug, id)
	if err != nil {
		return nil, err
	}
	d := &OrchestratorRunDetail{OrchestratorRun: orchestratorRun(*run), Prompt: run.Prompt, Output: run.Output, Log: []OrchestratorCall{}}
	for _, c := range run.Calls {
		d.Log = append(d.Log, OrchestratorCall{Tool: c.Tool, Input: c.Input, Result: c.Result, Error: c.Error, Refused: c.Refused})
	}
	return d, nil
}

// bundleOf is a bundle as the Verify tab shows it: its files by where
// they're served from, what ws ran of its step's checks, what the judge
// said of each of its own, and whether its capsule (at head) has moved on.
func bundleOf(slug string, b ops.BundleView, head string) Bundle {
	out := Bundle{ID: b.ID, Step: b.StepN, StepName: b.StepName, Crew: b.Crew, By: b.By, Own: b.Own,
		Capsule: b.Capsule, Commit: b.Commit, At: b.At, Replaces: b.Replaces, ReplacedBy: b.ReplacedBy,
		Summary: b.Summary, NotVerified: b.NotVerified, Noticed: b.Noticed, Checks: []BundleCheck{},
		Verdict: BundleVerdict(b.Verdict), VerdictAt: b.VerdictAt, Looked: b.Looked, Asked: b.Asked, AskedAt: b.AskedAt}
	for _, n := range b.Notes {
		out.Notes = append(out.Notes, BundleNote{Check: n.Check, Text: n.Text})
	}
	if b.Commit != "" && head != "" && !sameCommit(b.Commit, head) {
		out.Stale, out.Head = true, head
	}
	tries := func(ts []mission.BundleTry) []BundleTry {
		var out []BundleTry
		for _, t := range ts {
			out = append(out, BundleTry{Text: t.Text, Command: t.Command, Look: t.Look})
		}
		return out
	}
	out.Try = tries(b.Try)
	base := "/api/missions/" + url.PathEscape(slug) + "/bundles/" + url.PathEscape(b.ID) + "/file/"
	var evidence func(e mission.Evidence) BundleEvidence
	evidence = func(e mission.Evidence) BundleEvidence {
		ev := BundleEvidence{Name: e.File, URL: base + url.PathEscape(e.File), Kind: "text", Size: e.Size, Caption: e.Caption, MadeBy: e.MadeBy}
		switch {
		case strings.HasPrefix(e.Media, "image/"):
			ev.Kind = "image"
		case strings.HasPrefix(e.Media, "video/"):
			ev.Kind = "video"
		}
		if e.Before != nil {
			before := evidence(*e.Before)
			ev.Before = &before
		}
		return ev
	}
	judged := map[string]mission.CheckResult{}
	if b.Ran != nil {
		for _, r := range b.Ran.Results {
			// A command is ws's to run; anything else was judged (and where
			// the judge couldn't be asked, it's ws that says so).
			n, command := 0, r.By == "ws"
			for i, c := range b.StepChecks {
				if c.Text == r.Check {
					n, command = i+1, c.Run()
				}
			}
			if !command && !b.Own {
				continue // its verifier's own word, which the bundle has already
			}
			if command {
				out.Ran = append(out.Ran, BundleRan{N: n, Check: r.Check, Result: CheckVerdict(r.Result), Evidence: r.Evidence, By: r.By, Kept: r.Run})
				continue
			}
			judged[r.Check] = r
		}
	}
	for _, c := range b.Checks {
		bc := BundleCheck{N: c.N, Check: c.Check, Verdict: CheckVerdict(c.Verdict), Said: c.Said, Points: c.Points, Lines: c.Lines, Try: tries(c.Try), Asks: c.Asks}
		for _, e := range c.Shows {
			bc.Shows = append(bc.Shows, evidence(e))
		}
		if r, ok := judged[c.Check]; ok && c.Check != "" {
			bc.Judged = &BundleRan{N: c.N, Check: r.Check, Result: CheckVerdict(r.Result), Evidence: r.Evidence, By: r.By}
		}
		out.Checks = append(out.Checks, bc)
	}
	return out
}

// sameCommit reports whether two commits are the same, either given
// whole or cut short.
func sameCommit(a, b string) bool {
	return a != "" && b != "" && (strings.HasPrefix(a, b) || strings.HasPrefix(b, a))
}
