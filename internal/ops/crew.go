package ops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/images"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Roles is which harness and model each role uses, as the config says.
func Roles(cfg *config.Config) harness.Roles {
	if cfg == nil {
		return harness.Roles{}
	}
	r := harness.Roles{
		Crew:         harness.Pick{Harness: cfg.Crew.Harness, Model: cfg.Crew.Model},
		Briefing:     harness.Pick{Harness: cfg.Briefing.Harness, Model: cfg.Briefing.Model},
		Orchestrator: harness.Pick{Harness: cfg.Orchestrator.Harness, Model: cfg.Orchestrator.Model},
		Conversation: harness.Pick{Harness: cfg.Conversation.Harness, Model: cfg.Conversation.Model},
		Team:         harness.Pick{Harness: cfg.Team.Harness, Model: cfg.Team.Model},
		Judge:        harness.Pick{Harness: cfg.Judge.Harness, Model: cfg.Judge.Model},
		PRWriter:     harness.Pick{Harness: cfg.PRWriter.Harness, Model: cfg.PRWriter.Model},
		Automation:   harness.Pick{Harness: cfg.Automations.Harness, Model: cfg.Automations.Model},
	}
	for _, a := range cfg.Crew.Allowed {
		if h, m, err := config.ParsePick(a); err == nil { // Load refuses the rest
			r.CrewAllowed = append(r.CrewAllowed, harness.Pick{Harness: h, Model: m})
		}
	}
	return r
}

// CrewPicks are what crew may be launched on from a mission's page: the
// default first, then those [crew] allowed lists (harness.Roles.CrewPicks).
func (w *Workspace) CrewPicks() []harness.Pick { return Roles(w.Config()).CrewPicks() }

// LaunchPicked starts new work crew on a mission, as Launch does, on the
// harness and model picked on its page: refused unless it's the default
// or one [crew] allowed lists. An empty pick is the default.
func (w *Workspace) LaunchPicked(slug string, p harness.Pick, by Actor) (Launched, error) {
	if p == (harness.Pick{}) {
		return w.Launch(slug, Dispatch{}, by)
	}
	if p.Harness == "" {
		return Launched{}, invalid("pick a harness for the model %s", p.Model)
	}
	roles := Roles(w.Config())
	if !roles.AllowsCrew(p) {
		var picks []string
		for _, a := range roles.CrewPicks() {
			picks = append(picks, a.String())
		}
		return Launched{}, refused("not-allowed", "crew can't be launched on %s: it's not in [crew] allowed (crew may run on %s)", p, strings.Join(picks, ", "))
	}
	return w.Launch(slug, Dispatch{Harness: p.Harness, Model: p.Model}, by)
}

// RootPlace is how the working place of crew with no capsule reads
// (briefing crew, crew answering a question): they work from the
// workspace's root.
const RootPlace = "workspace"

// Dispatch is crew to start on a mission.
type Dispatch struct {
	// Role is "" for work crew, mission.RoleBriefing, or mission.RoleAnswer
	// (crew answering the mission's question for you).
	Role string
	// Resume is ended crew to continue (its id), instead of new crew.
	Resume string
	// Harness is the harness to run; "" for the role's.
	Harness string
	// Capsule is where work crew work (repo/name); "" for one nobody's
	// working in. Shared lets them work beside crew still working there.
	Capsule string
	Shared  bool
	// Text is what work crew are dispatched to do ("" for nothing in
	// particular), and Stage the stage of the plan it's for (0: the current
	// one). DispatchedBy is who it's recorded as dispatched by: "" for
	// whoever launches it.
	Text         string
	Stage        int
	DispatchedBy string
	// Model is what crew run on, when a step of the plan says (on haiku)
	// or it's picked on the mission's page; "" for the one the role gives
	// the harness, if any, else the agent's own. It's recorded on the
	// crew, and resuming them asks for it again, which harnesses over ACP
	// take (CLI ones resume their session without naming a model).
	Model string
}

// Launched is crew started.
type Launched struct {
	Slug, ID string
	Capsule  string // where it works: repo/name, or RootPlace
	Resumed  bool
	// HandedBack are crew that had handed their work back in that capsule,
	// and whose sessions were ended for the new crew to work there; Missed
	// those it couldn't reach to end.
	HandedBack, Missed []string
}

func (l Launched) String() string {
	if l.Resumed {
		return fmt.Sprintf("Resuming %s in %s.", l.ID, l.Capsule)
	}
	return fmt.Sprintf("Started %s in %s.", l.ID, l.Capsule)
}

// Launch starts crew on a mission, in the workspace's daemon, which sends
// it its opening prompt: nobody waits to type it in. Crew the orchestrator
// or ws start count against the crew it may start without you (refused
// with dispatch-limit once it's started as many as it may); anything you
// start counts as you stepping in. Crew don't start other crew.
func (w *Workspace) Launch(slug string, d Dispatch, by Actor) (Launched, error) {
	if by.Kind == Crew {
		return Launched{}, refused("not-yours", "crew don't dispatch other crew: run ws mission done when your part is finished, and the orchestrator decides what follows")
	}
	if w.opts.Host == nil {
		return Launched{}, errors.New("crew run in the workspace's daemon, which isn't reachable from here")
	}
	m, err := w.load(slug)
	if err != nil {
		return Launched{}, err
	}
	roles := Roles(w.Config())
	var resumed *mission.Crew
	carryOn := false
	place := crew.Place{Dir: w.Root(), Capsule: RootPlace}
	switch {
	case d.Resume != "":
		c, ok := m.CrewByID(d.Resume)
		switch {
		case !ok:
			return Launched{}, fmt.Errorf("%w: mission %s has no crew %s", ErrNotFound, m.Slug, d.Resume)
		case c.Ended == nil:
			return Launched{}, refused("crew-running", "%s is still running: talk to it on its page", c.ID)
		case c.Conversation == "":
			return Launched{}, refused("no-conversation", "%s recorded no conversation, so it can't be resumed", c.ID)
		}
		if c.Role == "" { // briefing and answer crew work from the root
			dir := filepath.Join(w.WS().RepoDir(strings.SplitN(c.Capsule, "/", 2)[0]), capsuleName(c.Capsule))
			if info, err := os.Stat(dir); c.Capsule == "" || err != nil || !info.IsDir() {
				return Launched{}, refused("capsule-gone", "%s's capsule %s is gone, so there's nowhere to resume it", c.ID, firstNonEmpty(c.Capsule, "(none)"))
			}
			place = crew.Place{Dir: dir, Capsule: c.Capsule}
		}
		d.Role, d.Harness, d.Model, resumed = c.Role, c.Harness, c.Model, &c
		// Crew that ended mid-turn, resumed with nothing to say, are told
		// to carry on (as a restart's are): else they'd sit waiting.
		carryOn = c.Role == "" && c.ResumeWith == "" && midTurn(c) && !c.HandedBack(m)
	case d.Role == mission.RoleBriefing:
		if !m.ToBrief() {
			return Launched{}, refused("briefed", "it's briefed already")
		}
		for _, c := range m.Crew {
			if c.Role == mission.RoleBriefing && c.Ended == nil {
				return Launched{}, refused("briefing", "%s is briefing it already", c.ID)
			}
		}
		d.Harness = firstNonEmpty(d.Harness, roles.ForBriefing().Harness)
	case d.Role == mission.RoleAnswer:
		if _, ok := m.Question(); !ok {
			return Launched{}, refused("nothing-asked", "nothing to answer: the mission isn't asking a question (a step of the user's is theirs)")
		}
		d.Harness = firstNonEmpty(d.Harness, roles.ForCrew().Harness)
	default:
		if m.ToBrief() {
			return Launched{}, refused("briefing", "mission %s is still being briefed: start a briefing agent instead", m.Slug)
		}
		if err := w.launchAtHeld(m, d, by); err != nil {
			return Launched{}, err
		}
		if place, err = w.place(m, d.Capsule, d.Shared); err != nil {
			return Launched{}, err
		}
		if d.Harness != "" || d.Model != "" {
			// A guarantee: crew run only on what [crew] allowed lists.
			if err := w.allowsCrew(harness.Pick{Harness: firstNonEmpty(d.Harness, roles.ForCrew().Harness), Model: d.Model}); err != nil {
				return Launched{}, err
			}
		}
		d.Harness = firstNonEmpty(d.Harness, roles.ForCrew().Harness)
	}
	// ws is starting crew from here: marked until they're recorded, who
	// then hold the mark until their first report (or, with no hooks of
	// ws's in their session, until they've their prompt: crew.Start), or
	// until this returns without any.
	what := ""
	if d.Stage > 0 {
		what = fmt.Sprintf("stage %d", d.Stage)
		if m.Plan != nil {
			what = fmt.Sprintf("step %d", d.Stage)
		}
	}
	defer w.launching(m.Slug, what)()
	h, err := harness.Get(d.Harness)
	if err == nil {
		err = h.Available()
	}
	if err != nil {
		return Launched{}, &Invalid{Reason: err.Error()}
	}
	prepared, err := w.prepare(m, h, d, place, resumed)
	if err != nil {
		return Launched{}, err
	}
	// What you paste them is a picture they read where ws keeps it.
	prepare := func() (harness.Launch, error) {
		l, err := prepared()
		return harness.ReadsPictures(h, l, images.Dir(w.Root())), err
	}
	counted := false
	// Resuming crew (ws does, after a restart) isn't starting more.
	if resumed == nil && (by.Kind == Orchestrator || by.Kind == WS) {
		if err := w.Missions.CountDispatch(m.Slug, w.Config().OrchestratorMaxDispatches()); err != nil {
			return Launched{}, wrap(err)
		}
		counted = true
	}
	out := Launched{Slug: m.Slug, Capsule: place.Capsule, Resumed: resumed != nil}
	if resumed == nil && d.Role == "" {
		out.HandedBack, out.Missed = w.endHandedBack(m, place.Capsule)
	}
	resume := ""
	if resumed != nil {
		resume = resumed.ID
		if carryOn {
			if err := w.Missions.SetResumeWithBy(m.Slug, resume, ResumedCarryOn, AsWS.By()); err != nil {
				return Launched{}, wrap(err)
			}
		}
	}
	started, err := crew.Start(crew.Request{
		Store: w.Missions, Host: w.opts.Host, Root: w.Root(),
		Slug: m.Slug, Harness: h.Name(), Model: d.Model, Place: place, Role: d.Role, Resume: resume,
		Dispatch: d.Text, By: firstNonEmpty(d.DispatchedBy, by.By()), Stage: d.Stage,
		Width: crewWidth, Height: crewHeight,
		Prepare: prepare,
	})
	if err != nil {
		if counted {
			logFailed(w.Missions.ReleaseDispatch(m.Slug)) // nothing started: the allowance isn't used up
		}
		if carryOn {
			w.Missions.SetResumeWith(m.Slug, resume, "") //nolint:errcheck // a message for a resume that didn't happen
		}
		if errors.Is(err, mission.ErrCrewRunning) || errors.Is(err, crew.ErrRunning) {
			// Another launch of theirs got there first.
			return Launched{}, refused("crew-running", "%s is still running: talk to it on its page", firstNonEmpty(resume, "the crew"))
		}
		return Launched{}, err
	}
	w.youActed(m.Slug, by)
	out.ID = started.Crew.ID
	return out, nil
}

// LaunchQueued starts the crew queued for you to launch: as it was queued,
// by whoever queued it, and launched by you.
func (w *Workspace) LaunchQueued(slug string, by Actor) (Launched, error) {
	m, err := w.load(slug)
	if err != nil {
		return Launched{}, err
	}
	q := m.Dispatch
	if q == nil {
		return Launched{}, refused("nothing-queued", "no crew is queued")
	}
	l, err := w.Launch(m.Slug, Dispatch{Harness: q.Harness, Capsule: q.Capsule, Text: q.Text, Stage: q.Stage, Model: q.Model, DispatchedBy: firstNonEmpty(q.By, by.By())}, by)
	if err != nil {
		return l, err
	}
	logFailed(w.Missions.QueueDispatch(m.Slug, nil))
	// A step that runs in several repos queues its runs' crew one at a
	// time: the next is queued now.
	w.queueNextRun(m.Slug)
	return l, nil
}

// A crew's screen size until a page shows it and resizes it.
const crewWidth, crewHeight = 120, 40

func capsuleName(c string) string {
	_, name, _ := strings.Cut(c, "/")
	return name
}

// Place is the capsule new work crew would work in: the one asked for, or
// one nobody is working in, refused as Launch would refuse it. Crew queued
// for you are placed so (shared: a queued dispatch may go beside crew still
// working, as it's launched later).
func (w *Workspace) Place(slug, want string, shared bool) (string, error) {
	m, err := w.load(slug)
	if err != nil {
		return "", err
	}
	p, err := w.place(m, want, shared)
	return p.Capsule, err
}

// place is where new work crew work: the capsule asked for, or one nobody
// is working in. Crew that have handed their work back don't count.
func (w *Workspace) place(m *mission.Mission, want string, shared bool) (crew.Place, error) {
	var caps []crew.Capsule
	for _, c := range m.Capsules {
		dir := filepath.Join(w.WS().RepoDir(c.Repo), c.Name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			caps = append(caps, crew.Capsule{Name: c.Repo + "/" + c.Name, Path: dir})
		}
	}
	p, err := crew.PreferFree(caps, crew.BusyIn(m), crew.HandedBackIn(m), want)
	var unknown *crew.UnknownCapsuleError
	switch {
	case errors.As(err, &unknown):
		return p, refused("no-capsule", "mission %s has %s; create it with ws lift <repo> <branch>, which joins it to the mission", m.Slug, unknown)
	case errors.Is(err, crew.ErrNoCapsule):
		return p, refused("no-capsule", "mission %s has no capsule to work in: create one with ws lift <repo> <branch> (it joins the mission), then dispatch with --capsule <repo>/<branch>", m.Slug)
	case err != nil:
		return p, err
	case p.Busy != "" && !shared && want != "":
		return p, refused("capsule-busy", "%s is still working in %s; wait for it to finish, or create another capsule with ws lift <repo> <branch> (it joins the mission) and dispatch there", p.Busy, p.Capsule)
	case p.Busy != "" && !shared:
		var names []string
		for _, c := range caps {
			names = append(names, c.Name)
		}
		return p, refused("capsule-busy", "crew are working in every capsule of %s (%s); wait for them, or create another capsule with ws lift <repo> <branch> and dispatch with --capsule", m.Slug, strings.Join(names, ", "))
	}
	return p, nil
}

// prepare is how the harness's command is built for the crew: resumed on
// its conversation, or new, told what it's for.
func (w *Workspace) prepare(m *mission.Mission, h harness.Harness, d Dispatch, place crew.Place, resumed *mission.Crew) (func() (harness.Launch, error), error) {
	roles := Roles(w.Config())
	guide := AgentsNoteFor(w.WS(), RoleCrew)
	brief, _ := w.Missions.ReadBrief(m.Slug)
	switch {
	case resumed != nil:
		model := roles.ForCrew().ModelFor(h.Name())
		if resumed.Role == mission.RoleBriefing {
			model = roles.ForBriefing().ModelFor(h.Name())
		}
		model = firstNonEmpty(d.Model, model)
		return func() (harness.Launch, error) {
			l, err := h.Resume(place.Dir, resumed.Conversation)
			return harness.WithModel(h, l, model), err
		}, nil
	case d.Role == mission.RoleBriefing:
		b := harness.Brief{Slug: m.Slug, Title: m.Title, Brief: brief, Dir: place.Dir, Role: mission.RoleBriefing,
			Request: m.Request, Conversation: m.Conversation, Org: w.WS().Org, Elsewhere: elsewhere(w.WS()), Model: firstNonEmpty(d.Model, roles.ForBriefing().ModelFor(h.Name())), Guide: guide}
		cb := crewBrief(b)
		cb.Planning = w.planning()
		b.Prompt = prompt.Crew(cb)
		return func() (harness.Launch, error) { return h.Start(b) }, nil
	case d.Role == mission.RoleAnswer:
		// Told the question and the answers suggested, not the whole
		// brief: the question is the job. They answer with ws mission
		// answer, which runs the orchestrator as your answer would.
		q, _ := m.Question()
		b := harness.Brief{Slug: m.Slug, Title: m.Title, Dir: place.Dir, Role: mission.RoleAnswer,
			Question: q.Text, Options: q.Options, Org: w.WS().Org, Elsewhere: elsewhere(w.WS()), Model: firstNonEmpty(d.Model, roles.ForCrew().ModelFor(h.Name())), Guide: guide}
		b.Prompt = prompt.Crew(crewBrief(b))
		return func() (harness.Launch, error) { return h.Start(b) }, nil
	}
	leftOff := LeftOff(w.WS(), w.Missions, m, place.Capsule)
	b := harness.Brief{Slug: m.Slug, Title: m.Title, Brief: brief, Dir: place.Dir, Dispatch: d.Text,
		LeftOff: leftOff, Model: firstNonEmpty(d.Model, roles.ForCrew().ModelFor(h.Name())), Guide: guide}
	states, _ := m.StageProgress(brief)
	st, err := mission.PickStage(states, d.Stage)
	if err != nil {
		return nil, &Invalid{Reason: err.Error()}
	}
	if !st.Implicit {
		b.Stage, b.StageN, b.StageOf, b.Steps = st.Whole(), st.N, len(states), st.Step != nil
		b.OpensPR = st.Step != nil && st.Step.MakesPR()
	}
	b.Prompt = prompt.Crew(crewBrief(b))
	if st.Step != nil && st.Step.Type == mission.StepVerify {
		// Crew at a Verify step are its verifier: told their step and
		// where to start, to show the work, not to do it.
		b.Prompt = prompt.Verifier(w.verifying(m, states, st, place.Capsule, place.Dir, brief, guide))
	}
	return func() (harness.Launch, error) { return h.Start(b) }, nil
}

// crewBrief is what a crew's prompt is written from.
func crewBrief(b harness.Brief) prompt.CrewBrief {
	return prompt.CrewBrief{Slug: b.Slug, Title: b.Title, Brief: b.Brief, Role: b.Role, Request: b.Request, Conversation: b.Conversation,
		Question: b.Question, Options: b.Options, Guide: b.Guide, Dispatch: b.Dispatch, Stage: b.Stage, StageN: b.StageN, StageOf: b.StageOf, Steps: b.Steps,
		OpensPR: b.OpensPR, LeftOff: b.LeftOff, Org: b.Org, Elsewhere: b.Elsewhere}
}

// elsewhere is the repos that aren't the org's by their names here, each
// as owner/name on GitHub: what crew are told besides the org.
func elsewhere(ws *workspace.Workspace) map[string]string {
	var out map[string]string
	for _, repo := range ws.RepoNames {
		if r := ws.RemoteOf(repo); r.Owner != "" && !r.Is(ws.Org, repo) {
			if out == nil {
				out = map[string]string{}
			}
			out[repo] = r.String()
		}
	}
	return out
}

// endHandedBack ends the sessions of crew that handed their work back in
// a capsule new crew are starting in, so two agents never work in one: the
// orchestrator can't be asked, so it isn't. Each is logged, whether it
// was reached or not; they can be resumed.
func (w *Workspace) endHandedBack(m *mission.Mission, capsule string) (ended, missed []string) {
	ids := crew.HandedBackIn(m)[capsule]
	for _, id := range ids {
		if err := w.opts.Host.Stop(m.Slug + "/" + id); err != nil {
			missed = append(missed, id)
		} else {
			ended = append(ended, id)
		}
	}
	if len(ended) > 0 {
		text := fmt.Sprintf("ended %s's session in %s: it had handed back its work, and new crew start there (its page resumes it)", strings.Join(ended, ", "), capsule)
		w.Missions.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventNote, Text: text})
		execlog.Note(w.Root(), "crew "+m.Slug, "%s", text)
	}
	if len(missed) > 0 {
		text := fmt.Sprintf("couldn't reach %s in %s to end its session, though it had handed back its work: new crew start there all the same", strings.Join(missed, ", "), capsule)
		w.Missions.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventNote, Text: text})
		execlog.Note(w.Root(), "crew "+m.Slug, "%s", text)
	}
	return ended, missed
}
