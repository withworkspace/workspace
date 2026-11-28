package ops

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/outbox"
)

// An idea is to brief until its brief and plan are confirmed; confirming
// them is the go-ahead, and the plan's first step is reached at once.
func TestConfirmBrief_StartsIt(t *testing.T) {
	plan := "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: find it.\n   - in: `frontend/one`\n2. Review: what's found.\n"
	w, _, host := reposFixture(t, plan)
	idea, err := w.Missions.CreateBriefing("the flaky test")
	if err != nil {
		t.Fatal(err)
	}
	if !idea.ToBrief() {
		t.Fatalf("an idea that isn't to brief: %+v", idea)
	}
	w.Missions.WritePlan(idea.Slug, plan)
	w.Attach(idea.Slug, "frontend", "one", true, AsYou)
	said, err := w.ConfirmBrief(idea.Slug, AsYou)
	if err != nil || !strings.Contains(said, "under way: started step 1: c1 in frontend/one") || len(host.specs) != 1 {
		t.Fatalf("confirmed: %q, %v", said, err)
	}
	m, _ := w.Missions.Load(idea.Slug)
	if m.ToBrief() || m.ConfirmedBy != "you" {
		t.Errorf("after: %+v", m)
	}
	_, err = w.ConfirmBrief(idea.Slug, AsYou)
	refusedWith(t, err, "confirmed")

	// Without a plan, it waits for its crew.
	bare, _ := w.Missions.CreateBriefing("tidy up")
	if said, _ := w.ConfirmBrief(bare.Slug, AsYou); said != "ready for crew" {
		t.Errorf("without a plan: %q", said)
	}
}

// Under way, a new goal and the plan to get there are proposed together,
// held to what's behind the mission, and confirming them moves it on:
// past the planner's step it was at.
func TestNewGoal_ConfirmedWithItsPlan(t *testing.T) {
	plan := "## Z\n- The cause is named.\n## Stops early\n- None.\n## Steps\n1. Work: find the cause.\n   - in: `frontend/one`\n2. Plan\n"
	w, _, _ := reposFixture(t, plan)
	w.Missions.WriteBrief("cache", "## Goal\nFind why the totals are off.\n\n## Acceptance\n- The cause, named.\n")
	w.Missions.StartCrew("cache", "shell", "")
	w.Missions.JoinStage("cache", "c1", 1)
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "found: rounding")
	if _, err := w.AcceptStage("cache", AsYou, "found"); err != nil {
		t.Fatal(err)
	}
	talks := conversation.For(w.Root(), "cache")
	c, _ := talks.OpenTalk(conversation.You, "the cause is found: plan the fix", time.Now())

	was, _ := w.Missions.ReadBrief("cache")
	brief := conversation.Proposal{Kind: conversation.ProposeBrief, By: conversation.Agent, Brief: "## Goal\nFix the rounding.\n\n## Acceptance\n- Totals are right, with a test.\n", Was: was}
	w.WithPlan(talks, c.ID, &brief)
	talks.Propose(c.ID, brief)
	newPlan := "## Z\n- Fixed, with a test.\n## Stops early\n- None.\n## Steps\n1. Work: find the cause.\n   - in: `frontend/one`\n2. Plan\n3. Work: fix it.\n   - in: `frontend/one`\n"
	p, err := w.PlanProposal(talks, c.ID, newPlan)
	if err != nil || p.Kind != conversation.ProposeBrief || p.Plan == "" || !strings.Contains(p.Brief, "Fix the rounding") {
		t.Fatalf("the plan didn't come with the new goal: %+v, %v", p, err)
	}
	c, _ = talks.Propose(c.ID, p)
	said, _, err := w.SettleProposal("cache", c.ID, len(c.Proposals), true, ProposalEdits{})
	if err != nil || !strings.Contains(said, "Wrote the new brief and plan (3 steps).") || !strings.Contains(said, "Passed step 2") {
		t.Fatalf("accepted: %q, %v", said, err)
	}
	m, _ := w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	if len(states) != 3 || !states[1].Accepted() || !states[2].Current {
		t.Errorf("after: %+v", states)
	}

	// One that changes a step behind the mission is refused.
	was, _ = w.Missions.ReadBrief("cache")
	planWas, _ := w.Missions.ReadPlan("cache")
	bad := conversation.Proposal{Kind: conversation.ProposeBrief, By: conversation.Agent, Brief: "## Goal\nSomething else.\n", Was: was,
		Plan: strings.Replace(newPlan, "find the cause", "find something else", 1), PlanWas: planWas}
	c, _ = talks.Propose(c.ID, bad)
	if _, _, err := w.SettleProposal("cache", c.ID, len(c.Proposals), true, ProposalEdits{}); err == nil || !strings.Contains(err.Error(), "only steps ahead can change") {
		t.Errorf("a new goal changing what's behind: %v", err)
	}
}

// A mission proposed with its brief is to brief until it's planned: the
// plan proposed in its conversation comes with that brief, as it is, and
// confirming them starts it. A plan accepted on its own does too, its
// brief being written; an idea's, with no brief yet, waits for the brief.
func TestBriefedMission_PlanStartsIt(t *testing.T) {
	plan := "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: **Find it.** Find the flake.\n   - in: `frontend`\n2. Review: **Check.** What's found.\n"
	w, _, _ := reposFixture(t, plan)
	brief := "## Goal\nThe flaky test passes every time.\n\n## Acceptance\n- It's fixed.\n"
	briefed := func(request string) (*mission.Mission, conversation.Store, *conversation.Conversation) {
		t.Helper()
		m, err := w.Missions.CreateBriefing(request)
		if err != nil {
			t.Fatal(err)
		}
		talks := conversation.For(w.Root(), m.Slug)
		c, err := talks.OpenBriefing(request, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return m, talks, c
	}

	// Proposed with the plan, the brief as it is isn't refused.
	m, talks, c := briefed("the flaky test")
	w.Missions.WriteBrief(m.Slug, brief)
	p, err := w.PlanProposal(talks, c.ID, plan)
	if err != nil || p.Kind != conversation.ProposeBrief || p.Brief != brief || p.Was != brief || p.Plan == "" || p.Title == "" {
		t.Fatalf("the plan didn't come with the brief as it is: %+v, %v", p, err)
	}
	p.By = conversation.Agent
	if c, err = talks.Propose(c.ID, p); err != nil {
		t.Fatalf("proposing the brief as it is with its plan: %v", err)
	}
	said, _, err := w.SettleProposal(m.Slug, c.ID, len(c.Proposals), true, ProposalEdits{})
	if err != nil || !strings.HasPrefix(said, "Wrote the plan (2 steps).") || !strings.Contains(said, "Under way") {
		t.Fatalf("confirmed: %q, %v", said, err)
	}
	if m, _ = w.Missions.Load(m.Slug); m.ToBrief() {
		t.Errorf("still to brief: %+v", m)
	}
	// Without a plan, the brief as it is is nothing to confirm.
	if _, err := talks.Propose(c.ID, conversation.Proposal{Kind: conversation.ProposeBrief, By: conversation.Agent, Brief: brief, Was: brief}); err == nil || !strings.Contains(err.Error(), "propose the plan") {
		t.Errorf("the brief as it is, alone: %v", err)
	}

	// A plan accepted on its own, proposed before it came with the brief.
	m, talks, c = briefed("the slow build")
	w.Missions.WriteBrief(m.Slug, brief)
	if m, _ = w.Missions.Load(m.Slug); !m.ToBrief() {
		t.Fatalf("not to brief: %+v", m)
	}
	if c, err = talks.Propose(c.ID, conversation.Proposal{Kind: conversation.ProposePlan, By: conversation.Agent, Plan: plan}); err != nil {
		t.Fatal(err)
	}
	said, _, err = w.SettleProposal(m.Slug, c.ID, len(c.Proposals), true, ProposalEdits{})
	if err != nil || !strings.Contains(said, "Under way") {
		t.Fatalf("a plan accepted on its own: %q, %v", said, err)
	}

	// An idea, its brief not written: the plan is only the plan.
	m, talks, c = briefed("tidy up")
	if p, err := w.PlanProposal(talks, c.ID, plan); err != nil || p.Kind != conversation.ProposePlan {
		t.Fatalf("an idea's plan: %+v, %v", p, err)
	}
	if c, err = talks.Propose(c.ID, conversation.Proposal{Kind: conversation.ProposePlan, By: conversation.Agent, Plan: plan}); err != nil {
		t.Fatal(err)
	}
	said, _, err = w.SettleProposal(m.Slug, c.ID, len(c.Proposals), true, ProposalEdits{})
	if err != nil || !strings.Contains(said, "Still being briefed") {
		t.Errorf("an idea's plan accepted: %q, %v", said, err)
	}
	if m, _ = w.Missions.Load(m.Slug); !m.ToBrief() {
		t.Errorf("an idea started: %+v", m)
	}
}

// A brief proposed with its plan is confirmed as you edited them: both,
// on the brief's own page. A brief proposed alone still has no plan to
// edit.
func TestBriefWithPlan_ConfirmedAsYouEditedThem(t *testing.T) {
	plan := "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: **Find it.** Find the flake.\n   - in: `frontend`\n2. Review: **Check.** What's found.\n"
	w, _, _ := reposFixture(t, plan)
	m, err := w.Missions.CreateBriefing("the flaky test")
	if err != nil {
		t.Fatal(err)
	}
	talks := conversation.For(w.Root(), m.Slug)
	c, err := talks.OpenBriefing("the flaky test", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	was, _ := w.Missions.ReadBrief(m.Slug)
	p := conversation.Proposal{Kind: conversation.ProposeBrief, By: conversation.Agent, Title: "Flaky test", Brief: "## Goal\nThe flaky test passes.\n", Was: was}
	w.WithPlan(talks, c.ID, &p)
	p.Plan = plan
	if c, err = talks.Propose(c.ID, p); err != nil {
		t.Fatal(err)
	}
	brief := "## Goal\nThe flaky test passes every time.\n"
	edited := strings.Replace(plan, "Find the flake.", "Find the flake, and why.", 1)
	said, _, err := w.SettleProposal(m.Slug, c.ID, len(c.Proposals), true, ProposalEdits{Brief: brief, Plan: edited})
	if err != nil || !strings.Contains(said, "Under way") {
		t.Fatalf("confirmed: %q, %v", said, err)
	}
	if got, _ := w.Missions.ReadBrief(m.Slug); got != brief {
		t.Errorf("brief = %q", got)
	}
	if got, _ := w.Missions.ReadPlan(m.Slug); !strings.Contains(got, "Find the flake, and why.") {
		t.Errorf("plan = %q", got)
	}
}

// In the daemon, a brief you accept is written and answered at once: its
// capsules are set up, and it's started, by the daemon's job, the mission
// showing it's setting up until then.
func TestBriefAccepted_SetUpBehind(t *testing.T) {
	plan := "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: **Find it.** Find the flake.\n   - in: `frontend`\n2. Review: **Check.** What's found.\n"
	w, _, _ := reposFixture(t, plan)
	w.opts.Outbox = &outbox.Queue{Root: w.Root()}
	var jobs []string
	w.opts.Jobs = JobsOf(func(j job.Job) { jobs = append(jobs, j.Kind+" "+j.Mission) }, job.SetUp)
	m, err := w.Missions.CreateBriefing("the flaky test")
	if err != nil {
		t.Fatal(err)
	}
	talks := conversation.For(w.Root(), m.Slug)
	c, err := talks.OpenBriefing("the flaky test", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	was, _ := w.Missions.ReadBrief(m.Slug)
	p := conversation.Proposal{Kind: conversation.ProposeBrief, By: conversation.Agent, Title: "Flaky test", Brief: "## Goal\nThe flaky test passes.\n", Was: was, Plan: plan}
	if c, err = talks.Propose(c.ID, p); err != nil {
		t.Fatal(err)
	}
	said, slug, err := w.SettleProposal(m.Slug, c.ID, len(c.Proposals), true, ProposalEdits{})
	if err != nil || !strings.HasSuffix(said, "Setting up its capsules, then starting it.") {
		t.Fatalf("accepted: %q, %v", said, err)
	}
	if !slices.Equal(jobs, []string{"set-up " + slug}) {
		t.Errorf("jobs: %v", jobs)
	}
	m, _ = w.Missions.Load(slug)
	if m.SettingUp == nil || !m.ToBrief() {
		t.Fatalf("before the job: setting up %+v, to brief %v", m.SettingUp, m.ToBrief())
	}
	if hs := mission.Resolve(mission.Snapshot{M: m, State: m.State}); !slices.ContainsFunc(hs, func(h mission.Headline) bool {
		return h.Label == "… setting up" && h.Court == mission.CourtAgent
	}) {
		t.Errorf("headlines: %+v", hs)
	}

	if said := w.SetUpBehind(slug); !strings.Contains(said, "Under way") {
		t.Errorf("the job said %q", said)
	}
	if m, _ = w.Missions.Load(slug); m.SettingUp != nil || m.ToBrief() {
		t.Errorf("after the job: setting up %+v, to brief %v", m.SettingUp, m.ToBrief())
	}
	if said := w.SetUpBehind(slug); said != "" {
		t.Errorf("again: %q", said)
	}
}
