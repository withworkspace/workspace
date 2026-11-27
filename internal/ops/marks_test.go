package ops

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
)

// probeCrew is crew whose harness looks at the mission as it's asked for
// their command: ws has decided to start them, and hasn't recorded them.
type probeCrew struct {
	name string
	at   func(when string)
	fail error
}

func (p probeCrew) Name() string     { return p.name }
func (p probeCrew) Available() error { return nil }
func (p probeCrew) Start(b harness.Brief) (harness.Launch, error) {
	p.at("before they're recorded")
	if p.fail != nil {
		return harness.Launch{}, p.fail
	}
	return harness.Launch{Args: []string{"/bin/sh"}, Dir: b.Dir, Prompt: b.Prompt}, nil
}
func (probeCrew) Resume(string, string) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}

// probeHost starts nothing, and looks at the mission as it's asked to:
// the crew are recorded, and haven't a process yet.
type probeHost struct{ at func(when string) }

func (h *probeHost) Launch(crew.Spec) (int, error) {
	h.at("recorded, their session starting")
	return deadPID, nil
}
func (h *probeHost) Stop(string) error                  { return crew.ErrNotRunning }
func (h *probeHost) Say(string, string) (string, error) { return "", crew.ErrNotRunning }

// seen is a mission's headlines at the moments a test looks at it.
type seen struct {
	t    *testing.T
	w    *Workspace
	slug string
	at   map[string]mission.Headline
}

func watching(t *testing.T, w *Workspace, slug string) *seen {
	return &seen{t: t, w: w, slug: slug, at: map[string]mission.Headline{}}
}

// now is the mission's headline, as its page and ws mission show have it.
func (s *seen) now() mission.Headline {
	s.t.Helper()
	d, err := s.w.Mission(s.slug)
	if err != nil {
		s.t.Fatal(err)
	}
	return d.Headlines[0]
}

func (s *seen) look(when string) { s.at[when] = s.now() }

// probe has the mission's crew run on a harness and a host that look at
// it as ws starts them.
func (s *seen) probe(name string, fail error) {
	harness.Register(probeCrew{name: name, at: s.look, fail: fail})
	s.w.Config().Crew.Harness = name
	s.w.opts.Host = &probeHost{at: s.look}
}

func (s *seen) want(when string, kind mission.Kind, text string) {
	s.t.Helper()
	h, ok := s.at[when]
	if !ok {
		s.t.Errorf("%s: never looked at", when)
		return
	}
	if h.Kind != kind || h.Court != mission.CourtAgent || !strings.Contains(h.Text, text) {
		s.t.Errorf("%s: %s/%s %q; want agent/%s saying %q", when, h.Court, h.Kind, h.Text, kind, text)
	}
}

// Starting crew is marked from the moment ws begins: while it's getting
// them ready, then on the crew once they're recorded, until ws has handed
// them their prompt (these crew's harness has no hooks). It's the gap that
// said "Nobody's on step 4" and offered to launch crew ws was already
// launching.
func TestLaunch_StartingFromWhenWsBegins(t *testing.T) {
	w, _ := replanFixture(t)
	// Crew worked on it before: without the mark, it reads as stalled.
	w.Missions.StartCrew("cache", "shell", "")
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "reproduced")
	w.Missions.EndCrew("cache", "c1", "exit 0")
	s := watching(t, w, "cache")
	s.probe("probe-launch", nil)

	// The planner's revision is yours to confirm: accepting it passes its
	// step, and starts the next one's crew.
	talks := conversation.For(w.Root(), "cache")
	id := talks.Planner().ID
	noReview := strings.Replace(fixPlanned, "5. Review the fix.\n   - who: you\n", "", 1)
	if rev, err := w.RevisePlan("cache", id, noReview); err != nil || rev.Proposed != 1 {
		t.Fatalf("revised: %+v, %v", rev, err)
	}
	said, _, err := w.SettleProposal("cache", id, 1, true, ProposalEdits{})
	if err != nil || !strings.Contains(said, "started step 4: c2 in frontend/two") {
		t.Fatalf("accepted: %q, %v", said, err)
	}
	s.want("before they're recorded", mission.KindLaunching, "Starting crew on step 4")
	s.want("recorded, their session starting", mission.KindLaunching, "Starting c2: Fix it, the test passing.")

	// Started, they hold the mark, and the mission's is gone.
	m := mustLoad(t, w, "cache")
	if c, _ := m.CrewByID("c2"); m.Launching != nil || c.Starting == nil {
		t.Errorf("after the launch: launching %+v, c2 starting %v", m.Launching, c.Starting)
	}
	if h := s.now(); h.Kind != mission.KindLaunching || h.Action.Verb != mission.VerbWatch || h.Action.Target != "c2" {
		t.Errorf("their session starting: %s %q, to %s %s", h.Kind, h.Text, h.Action.Verb, h.Action.Target)
	}
	// Handed their prompt (as the host does once it's typed it in), they're
	// at work: their harness has no hooks to report it with.
	w.Missions.CrewHandedPrompt("cache", "c2")
	if h := s.now(); h.Kind != mission.KindWorking || h.By != "c2" {
		t.Errorf("handed their prompt: %s %q", h.Kind, h.Text)
	}
}

// A launch that comes to nothing leaves no mark: the mission is as it
// was, yours to launch crew on.
func TestLaunch_FailedLeavesNoMark(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "one")
	s := watching(t, w, "cache")
	s.probe("probe-fails", errors.New("no such agent"))
	if _, err := w.Launch("cache", Dispatch{}, AsYou); err == nil {
		t.Fatal("launched on a harness that can't start")
	}
	s.want("before they're recorded", mission.KindLaunching, "Starting crew")
	m := mustLoad(t, w, "cache")
	if m.Launching != nil || len(m.Crew) != 0 {
		t.Errorf("after it failed: launching %+v, crew %+v", m.Launching, m.Crew)
	}
	if h := s.now(); h.Kind != mission.KindReady {
		t.Errorf("after it failed: %s %q", h.Kind, h.Text)
	}
	// Refused before ws began, it was never marked.
	s.at = map[string]mission.Headline{}
	_, err := w.Launch("cache", Dispatch{Resume: "c9"}, AsYou)
	if !errors.Is(err, ErrNotFound) || len(s.at) != 0 || mustLoad(t, w, "cache").Launching != nil {
		t.Errorf("a launch refused: %v, looked at %v", err, s.at)
	}
}

// Confirming a brief and its plan starts its first step's crew, and ws
// says so from before it's confirmed: through naming it and lifting its
// capsules, and then the launch. With crew queued for you instead, there's
// nothing starting to say.
func TestConfirmBrief_StartingBeforeItsConfirmed(t *testing.T) {
	plan := "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: **Find it.** Find the flake.\n   - in: `frontend/one`\n2. Review: **Check.** What's found.\n"
	w, _, _ := reposFixture(t, plan)
	idea, err := w.Missions.CreateBriefing("the flaky test")
	if err != nil {
		t.Fatal(err)
	}
	slug := idea.Slug
	w.Missions.WritePlan(slug, plan)
	w.Attach(slug, "frontend", "one", true, AsYou)
	talks := conversation.For(w.Root(), slug)
	c, _ := talks.OpenBriefing("the flaky test", time.Now())
	talks.Say(c.ID, conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "Shall I go on?"})
	s := watching(t, w, slug)
	if h := s.now(); h.Court != mission.CourtYou {
		t.Fatalf("before: %s %q", h.Kind, h.Text)
	}

	// Marked, it's ws's from before it's confirmed, whatever its briefing
	// conversation last said.
	end := w.launchingOnConfirm(slug)
	d, _ := w.Mission(slug)
	if h := d.Headlines[0]; d.State != mission.Briefing || h.Kind != mission.KindLaunching || h.Text != "Starting crew on step 1" {
		t.Errorf("accepted, not yet confirmed: %s, %s %q", d.State, h.Kind, h.Text)
	}
	end(slug)
	if m := mustLoad(t, w, slug); m.Launching != nil {
		t.Errorf("the mark's left: %+v", m.Launching)
	}

	// Confirmed: starting all the way to the crew.
	s.probe("probe-confirm", nil)
	if said, err := w.ConfirmBrief(slug, AsYou); err != nil || !strings.Contains(said, "started step 1: c1 in frontend/one") {
		t.Fatalf("confirmed: %q, %v", said, err)
	}
	s.want("before they're recorded", mission.KindLaunching, "Starting crew on step 1")
	s.want("recorded, their session starting", mission.KindLaunching, "Starting c1: Find it.")
	// Confirmed again, it's refused, and nothing's marked.
	_, err = w.ConfirmBrief(slug, AsYou)
	refusedWith(t, err, "confirmed")
	if m := mustLoad(t, w, slug); m.Launching != nil {
		t.Errorf("refused, and marked: %+v", m.Launching)
	}

	// Crew queued for you to launch aren't being started.
	queued, _ := w.Missions.CreateBriefing("tidy up")
	w.Missions.WritePlan(queued.Slug, plan)
	w.Attach(queued.Slug, "frontend", "one", true, AsYou)
	w.Config().Orchestrator.Dispatch = "ask"
	w.launchingOnConfirm(queued.Slug)
	if m := mustLoad(t, w, queued.Slug); m.Launching != nil {
		t.Errorf("crew to be queued, marked as starting: %+v", m.Launching)
	}
	if said, err := w.ConfirmBrief(queued.Slug, AsYou); err != nil || !strings.Contains(said, "queued") {
		t.Fatalf("confirmed, crew queued: %q, %v", said, err)
	}
	if m := mustLoad(t, w, queued.Slug); m.Launching != nil {
		t.Errorf("queued, and marked: %+v", m.Launching)
	}
}

// The briefing agent is at work from when its turn is asked for until its
// reply is recorded: a mark ws sets and clears on the mission, whatever
// the conversation's answering mark says (no process marked, or one
// that's gone), and however long it takes.
func TestBriefingTurn_AtWorkFromAskedToReplied(t *testing.T) {
	w, _ := fixture(t, "")
	w.cur.cfg.Conversation.Harness = fake.Name()
	defer func() { fake.during = nil }()
	idea, err := w.Missions.CreateBriefing("the flaky test")
	if err != nil {
		t.Fatal(err)
	}
	slug := idea.Slug
	s := watching(t, w, slug)
	talks := conversation.For(w.Root(), slug)
	c, _ := talks.OpenBriefing("the flaky test", time.Now())
	if h := s.now(); h.Kind != mission.KindToBrief {
		t.Fatalf("nothing asked of its agent yet: %s %q", h.Kind, h.Text)
	}

	// Asked for: at work as the job's handed over, before any process has
	// taken it.
	w.opts.Jobs = JobsOf(func(job.Job) { s.look("asked for, not yet taken") }, job.Answer, job.Thread, job.Draft)
	w.answer(job.Answer, doc.OfMission(slug), c.ID)
	s.want("asked for, not yet taken", mission.KindBriefing, "The briefing agent is working in "+c.ID)
	if got, _ := talks.Load(c.ID); got.Answering {
		t.Error("a process is marked answering it: the turn's mark isn't that")
	}

	// On its turn: at work, though the process that took it is gone.
	fake.reply = "What should it do when the cache is cold?"
	fake.during = func() {
		fake.during = nil
		talks.EndAnswering(c.ID)
		talks.StartAnswering(c.ID, 2147483646) // nothing has it
		s.look("on its turn")
	}
	if err := w.AnswerConversation(doc.OfMission(slug), c.ID); err != nil {
		t.Fatal(err)
	}
	s.want("on its turn", mission.KindBriefing, "The briefing agent is working in "+c.ID)

	// Its reply recorded, its turn's over: it waits for you.
	if m := mustLoad(t, w, slug); m.Planning != nil {
		t.Errorf("its turn is still marked: %+v", m.Planning)
	}
	if h := s.now(); h.Kind != mission.KindBriefTalk || h.Court != mission.CourtYou {
		t.Errorf("its reply recorded: %s %q", h.Kind, h.Text)
	}
	if got, _ := talks.Load(c.ID); got.Last().By != conversation.Agent || got.NeedsAnswer() {
		t.Errorf("its reply: %+v", got.Last())
	}

	// A conversation that isn't its planner's marks nothing.
	other, _ := talks.OpenTalk(conversation.You, "what's a cold cache?", time.Now())
	w.answer(job.Answer, doc.OfMission(slug), other.ID)
	if m := mustLoad(t, w, slug); m.Planning != nil {
		t.Errorf("marked for a conversation that isn't the planner's: %+v", m.Planning)
	}
}

// The planner, once the mission's briefed, is at work the same way: from
// being asked until its reply is recorded, and not after, though its
// conversation was asked something it never answered.
func TestPlannerTurn_AtWorkFromAskedToReplied(t *testing.T) {
	w, _ := replanFixture(t)
	w.cur.cfg.Conversation.Harness = fake.Name()
	defer func() { fake.during = nil }()
	s := watching(t, w, "cache")
	talks := conversation.For(w.Root(), "cache")
	id := talks.Planner().ID

	// Asked at its step with nobody to answer (no server here): its
	// conversation needs an answer, and that alone isn't a turn underway.
	if c, _ := talks.Load(id); !c.NeedsAnswer() {
		t.Fatal("the planner wasn't asked")
	}
	if h := s.now(); h.Court != mission.CourtYou || !strings.Contains(h.Text, "Step 3 is the planner's") {
		t.Errorf("asked, with nobody to answer: %s %q", h.Kind, h.Text)
	}

	// You say more, and its turn's asked for: at work from then.
	w.opts.Jobs = JobsOf(func(job.Job) { s.look("asked for, not yet taken") }, job.Answer, job.Thread, job.Draft)
	if _, err := w.SayInTalk(doc.OfMission("cache"), id, "Keep the fix small.", AsYou); err != nil {
		t.Fatal(err)
	}
	s.want("asked for, not yet taken", mission.KindBriefing, "The planner is working on the plan in "+id)

	fake.reply = "Which rounding do you want?"
	fake.during = func() {
		fake.during = nil
		s.look("on its turn")
	}
	if err := w.AnswerConversation(doc.OfMission("cache"), id); err != nil {
		t.Fatal(err)
	}
	s.want("on its turn", mission.KindBriefing, "The planner is working on the plan in "+id)
	if m := mustLoad(t, w, "cache"); m.Planning != nil {
		t.Errorf("its turn is still marked: %+v", m.Planning)
	}
	if h := s.now(); h.Court != mission.CourtYou || !strings.Contains(h.Text, "answer in "+id) {
		t.Errorf("its reply recorded: %s %q", h.Kind, h.Text)
	}
}
