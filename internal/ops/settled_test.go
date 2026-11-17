package ops

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/job"
)

const settledPlan = "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: find it.\n   - in: `frontend/one`\n2. Review: what's found.\n"

// settledFixture is the cache mission under way, with a brief and a plan,
// and a talk on its Talk tab (t1) its agent has replied in; it says which
// conversations the agent was woken to answer.
func settledFixture(t *testing.T) (*Workspace, conversation.Store, *[]string) {
	t.Helper()
	w, _, _ := reposFixture(t, settledPlan)
	w.Missions.WriteBrief("cache", "## Goal\nFind why the totals are off.\n")
	var woken []string
	w.opts.Answer = func(kind, slug, id string) { woken = append(woken, kind+" "+slug+"/"+id) }
	talks := conversation.For(w.Root(), "cache")
	talks.OpenTalk(conversation.You, "the totals are off", time.Now())
	talks.Say("t1", conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "Here's what I'd do."})
	return w, talks, &woken
}

// proposal is one of each kind, as the cache mission's agent would
// propose it: a brief and a plan that change the mission's.
func proposal(t *testing.T, w *Workspace, kind string) conversation.Proposal {
	t.Helper()
	brief, _ := w.Missions.ReadBrief("cache")
	plan, _ := w.Missions.ReadPlan("cache")
	p := conversation.Proposal{Kind: kind, By: conversation.Agent, At: time.Now()}
	switch kind {
	case conversation.ProposeMission:
		p.Title = "Round the totals"
	case conversation.ProposeDecision:
		p.What, p.Why = "Round half to even.", "It's what the ledger does."
	case conversation.ProposeDocument:
		p.DocKind, p.Title, p.Text = "doc", "Rounding", "# Rounding\n\nHalf to even."
	case conversation.ProposeBrief:
		p.Brief, p.Was = brief+"\n## Acceptance\n- The cause, named.\n", brief
	case conversation.ProposePlan:
		p.Plan, p.Was = strings.Replace(plan, "find it.", "find the cause.", 1), plan
	}
	return p
}

// Settling any card writes a turn of yours that you didn't type, marked
// with the proposal it's for, saying what became of it; the agent's woken
// for it on every decline and on accepting a brief or a plan, and not on
// accepting a mission, a decision or a document, whose turn it reads on
// its next. A quiet turn doesn't change whose turn it is.
func TestSettledCards_TurnAndWake(t *testing.T) {
	for _, tc := range []struct {
		kind   string
		accept bool
		wakes  bool
		says   string
	}{
		{conversation.ProposeMission, true, false, "Accepted proposal 1, the mission idea “Round the totals”: it's mission round-the-totals now."},
		{conversation.ProposeMission, false, true, "Declined proposal 1, the mission idea “Round the totals”."},
		{conversation.ProposeDecision, true, false, "Accepted proposal 1, the decision “Round half to even”: recorded as d1 on mission cache."},
		{conversation.ProposeDecision, false, true, "Declined proposal 1, the decision “Round half to even”."},
		{conversation.ProposeDocument, true, false, "Accepted proposal 1, the document “Rounding”: added as records:docs/rounding.md."},
		{conversation.ProposeDocument, false, true, "Declined proposal 1, the document “Rounding”."},
		{conversation.ProposeBrief, true, true, "Accepted proposal 1, the edit to the brief: written to mission cache."},
		{conversation.ProposeBrief, false, true, "Declined proposal 1, the edit to the brief."},
		{conversation.ProposePlan, true, true, "Accepted proposal 1, the edit to the plan: written to mission cache."},
		{conversation.ProposePlan, false, true, "Declined proposal 1, the edit to the plan."},
	} {
		action := map[bool]string{true: "accepting", false: "declining"}[tc.accept]
		t.Run(action+" a "+tc.kind, func(t *testing.T) {
			w, talks, woken := settledFixture(t)
			c, err := talks.Propose("t1", proposal(t, w, tc.kind))
			if err != nil {
				t.Fatal(err)
			}
			if tc.kind == conversation.ProposeDocument && tc.accept {
				// Adding a document lands it in its repo, which
				// TestDocAdd_ProposedInATalkAndAccepted does for real: here,
				// it's settled as accepting it settles it, and the agent's
				// told as settling it tells it.
				talks.SettleProposal("t1", 1, conversation.ProposalAccepted, "records:docs/rounding.md", conversation.You, time.Now())
				w.tellAgent(talks, "cache", "t1", 1, ProposalEdits{})
			} else if _, _, err := w.SettleProposal("cache", c.ID, 1, tc.accept, ProposalEdits{}); err != nil {
				t.Fatal(err)
			}
			c, _ = talks.Load("t1")
			if len(c.Turns) != 3 {
				t.Fatalf("turns: %+v", c.Turns)
			}
			turn := c.Turns[2]
			if turn.By != conversation.You || turn.Origin != "proposal:1" || turn.OriginProposal() != 1 || turn.Quiet == tc.wakes {
				t.Errorf("the turn: %+v", turn)
			}
			if turn.Body != tc.says {
				t.Errorf("it says %q, want %q", turn.Body, tc.says)
			}
			want := []string{}
			if tc.wakes {
				want = []string{job.Answer + " cache/t1"}
			}
			if !slices.Equal(append([]string{}, *woken...), want) {
				t.Errorf("woken: %q, want %q", *woken, want)
			}
			// Woken, it's the agent's turn; quiet, it's still yours.
			if c.NeedsAnswer() != tc.wakes || c.WaitingOnYou() == tc.wakes {
				t.Errorf("needs an answer %v, waiting on you %v", c.NeedsAnswer(), c.WaitingOnYou())
			}
		})
	}
}

// What you edited before accepting is in the turn: a line as you had it
// and as proposed, text of more lines as a diff from the proposal. What
// you sent unchanged isn't an edit.
func TestSettledCards_Edits(t *testing.T) {
	w, talks, _ := settledFixture(t)
	talks.Propose("t1", proposal(t, w, conversation.ProposeDecision))
	if _, _, err := w.SettleProposal("cache", "t1", 1, true, ProposalEdits{What: "Round half up.", Why: "It's what the ledger does."}); err != nil {
		t.Fatal(err)
	}
	c, _ := talks.Load("t1")
	want := "Accepted proposal 1, the decision “Round half up”: recorded as d1 on mission cache.\n\nEdited before accepting.\n\nThe decision: “Round half up.” (proposed: “Round half to even.”)."
	if got := c.Last().Body; got != want {
		t.Errorf("a decision edited:\n%s\nwant:\n%s", got, want)
	}

	talks.Propose("t1", proposal(t, w, conversation.ProposePlan))
	edited, _ := w.Missions.ReadPlan("cache")
	edited = strings.Replace(edited, "what's found.", "the cause, before it's fixed.", 1)
	if _, _, err := w.SettleProposal("cache", "t1", 2, true, ProposalEdits{Plan: edited}); err != nil {
		t.Fatal(err)
	}
	c, _ = talks.Load("t1")
	body := c.Last().Body
	for _, want := range []string{
		"Accepted proposal 2, the edit to the plan: written to mission cache.\n\nEdited before accepting.\n\nThe plan, against what was proposed:\n\n```diff\n--- proposed\n+++ accepted\n",
		"-1. Work: find the cause.\n",
		"+1. Work: find it.\n",
		"-2. Review: what's found.\n+2. Review: the cause, before it's fixed.\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("a plan edited lacks %q:\n%s", want, body)
		}
	}
	if !strings.HasSuffix(body, "\n```") {
		t.Errorf("the diff isn't closed:\n%s", body)
	}

	// A mission whose title you changed, and whose brief you sent as it
	// was.
	talks.Propose("t1", conversation.Proposal{Kind: conversation.ProposeMission, By: conversation.Agent, Title: "Round the totals", Brief: "## Goal\nRound them.\n"})
	if _, _, err := w.SettleProposal("cache", "t1", 3, true, ProposalEdits{Title: "Round the totals per ledger", Brief: "## Goal\nRound them.\n\n"}); err != nil {
		t.Fatal(err)
	}
	c, _ = talks.Load("t1")
	want = "Accepted proposal 3, the mission “Round the totals per ledger”: it's mission round-the-totals-per-ledger now.\n\nEdited before accepting.\n\nThe title: “Round the totals per ledger” (proposed: “Round the totals”)."
	if got := c.Last().Body; got != want {
		t.Errorf("a mission retitled:\n%s\nwant:\n%s", got, want)
	}
}

// An artifact accepted is named as it was added: a new one by its title,
// a version by what it's of; an edited note is in the turn.
func TestSettledCards_Artifact(t *testing.T) {
	added := conversation.Proposal{N: 4, Kind: conversation.ProposeArtifact, Title: "Horizon glow", Note: "A first.", State: conversation.ProposalAccepted, Result: "a1.v1"}
	if got := settledText(added, "", "w1", ProposalEdits{}); got != "Accepted proposal 4, the artifact “Horizon glow”: added as a1.v1." {
		t.Errorf("a new one: %q", got)
	}
	version := conversation.Proposal{N: 5, Kind: conversation.ProposeArtifact, To: "a1", On: 1, Note: "Softer.", State: conversation.ProposalAccepted, Result: "a1.v2"}
	if got := settledText(version, "", "w1", ProposalEdits{Note: "Softer, and slower."}); got != "Accepted proposal 5, a version of a1, on v1: added as a1.v2.\n\nEdited before accepting.\n\nIts note: “Softer, and slower.” (proposed: “Softer.”)." {
		t.Errorf("a version, its note edited: %q", got)
	}
	version.State, version.Result = conversation.ProposalDeclined, ""
	if got := settledText(version, "", "w1", ProposalEdits{}); got != "Declined proposal 5, a version of a1, on v1." {
		t.Errorf("declined: %q", got)
	}
}

// A brief whose text has a code block in it is diffed in a longer fence.
func TestSettledCards_FencedDiff(t *testing.T) {
	p := conversation.Proposal{N: 1, Kind: conversation.ProposeBrief, Brief: "## Goal\nx\n", State: conversation.ProposalAccepted}
	text := settledText(p, "cache", "t1", ProposalEdits{Brief: "## Goal\n```go\nx()\n```\n"})
	if !strings.Contains(text, "````diff\n") || !strings.HasSuffix(text, "\n````") {
		t.Errorf("fenced:\n%s", text)
	}
}

// A conversation you'd put away: a card settled quietly leaves it so, and
// one that wakes the agent takes it up again. Only settling writes a turn,
// once: a proposal settled already is refused, and says nothing more.
func TestSettledCards_PutAway(t *testing.T) {
	w, talks, woken := settledFixture(t)
	talks.Propose("t1", proposal(t, w, conversation.ProposeDecision))
	talks.Propose("t1", proposal(t, w, conversation.ProposeMission))
	talks.SetStatus("t1", conversation.You, conversation.Resolved, time.Now())

	w.SettleProposal("cache", "t1", 1, true, ProposalEdits{})
	c, _ := talks.Load("t1")
	if c.Status != conversation.Resolved || len(c.Turns) != 3 || !c.Last().Quiet || len(*woken) != 0 {
		t.Errorf("accepted quietly: %s, %d turns, woken %q", c.Status, len(c.Turns), *woken)
	}
	w.SettleProposal("cache", "t1", 2, false, ProposalEdits{})
	c, _ = talks.Load("t1")
	if c.Status != conversation.Open || len(c.Turns) != 4 || c.Last().Quiet || !slices.Equal(*woken, []string{job.Answer + " cache/t1"}) {
		t.Errorf("declined: %s, %d turns, woken %q", c.Status, len(c.Turns), *woken)
	}
	if _, _, err := w.SettleProposal("cache", "t1", 2, true, ProposalEdits{}); err == nil {
		t.Error("settled twice")
	}
	if c, _ = talks.Load("t1"); len(c.Turns) != 4 {
		t.Errorf("settling it again wrote %d turns", len(c.Turns)-4)
	}
}

// Briefing renames the mission, and its conversation moves with it: the
// turn's written there, naming the mission as it's called now, and the
// planner's woken.
func TestSettledCards_BriefingRenames(t *testing.T) {
	w, _, _ := reposFixture(t, settledPlan)
	var woken []string
	w.opts.Answer = func(kind, slug, id string) { woken = append(woken, kind+" "+slug+"/"+id) }
	m, _ := w.Missions.CreateBriefing("fe issue 45")
	talks := conversation.For(w.Root(), m.Slug)
	talks.OpenBriefing("fe issue 45", time.Now())
	talks.Say("t1", conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "Proposed."})
	was, _ := w.Missions.ReadBrief(m.Slug)
	planWas, _ := w.Missions.ReadPlan(m.Slug)
	plan := strings.Replace(settledPlan, "`frontend/one`", "`frontend`", 1)
	talks.Propose("t1", conversation.Proposal{Kind: conversation.ProposeBrief, By: conversation.Agent, Title: "Fix the totals", Slug: "fix-totals",
		Brief: "## Goal\nThe totals add up.\n", Was: was, Plan: plan, PlanWas: planWas})
	said, after, err := w.SettleProposal(m.Slug, "t1", 1, true, ProposalEdits{})
	if err != nil || after != "fix-totals" {
		t.Fatalf("accepted: %q, %q, %v", said, after, err)
	}
	c, err := conversation.For(w.Root(), "fix-totals").Load("t1")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Last(); got.Origin != "proposal:1" || got.Quiet || got.Body != "Accepted proposal 1, the brief and its plan: written to mission fix-totals." {
		t.Errorf("the turn: %+v", got)
	}
	if !slices.Contains(woken, job.Answer+" fix-totals/t1") {
		t.Errorf("woken: %q", woken)
	}
}

// A card settled while the agent's answering is written before its
// reply, which was written without it: the agent's told it next, and one
// that wakes it has it answer again at once, while a quiet one waits for
// what's said next.
func TestSettledCards_WhileAnswering(t *testing.T) {
	w, talks, _ := settledFixture(t)
	w.cur.cfg.Conversation.Harness = fake.Name()
	defer func() { fake.during = nil }()
	talks.Propose("t1", proposal(t, w, conversation.ProposeDecision))
	talks.Propose("t1", proposal(t, w, conversation.ProposeMission))
	talks.Say("t1", conversation.Turn{By: conversation.You, At: time.Now(), Body: "and the rounding?"})

	// Declined as it answers: it answers again, told of it.
	fake.reply, fake.prompts = "Half to even.\n\nTitle: Rounding", nil
	fake.during = func() {
		fake.during = nil
		w.SettleProposal("cache", "t1", 1, false, ProposalEdits{})
	}
	if err := w.AnswerConversation(doc.OfMission("cache"), "t1"); err != nil {
		t.Fatal(err)
	}
	c, _ := talks.Load("t1")
	if len(fake.prompts) != 2 || strings.Contains(fake.prompts[0], "Declined proposal 1") || !strings.Contains(fake.prompts[1], "Declined proposal 1, the decision “Round half to even”.") {
		t.Fatalf("told: %q", fake.prompts)
	}
	if n := len(c.Turns); n != 6 || c.Turns[3].Origin != "proposal:1" || c.Turns[4].Seen != 3 || c.Turns[5].Seen != 5 || c.NeedsAnswer() {
		t.Errorf("turns: %+v", c.Turns)
	}

	// Accepted quietly as it answers: it isn't asked again, and it's told
	// with what's said next.
	talks.Say("t1", conversation.Turn{By: conversation.You, At: time.Now(), Body: "ok"})
	fake.prompts = nil
	fake.during = func() {
		fake.during = nil
		w.SettleProposal("cache", "t1", 2, true, ProposalEdits{})
	}
	if err := w.AnswerConversation(doc.OfMission("cache"), "t1"); err != nil {
		t.Fatal(err)
	}
	if c, _ = talks.Load("t1"); len(fake.prompts) != 1 || c.NeedsAnswer() || !c.WaitingOnYou() || len(c.Missed()) != 1 {
		t.Fatalf("a quiet one: told %q, missed %+v", fake.prompts, c.Missed())
	}
	talks.Say("t1", conversation.Turn{By: conversation.You, At: time.Now(), Body: "what next?"})
	fake.prompts = nil
	if err := w.AnswerConversation(doc.OfMission("cache"), "t1"); err != nil {
		t.Fatal(err)
	}
	if len(fake.prompts) != 1 || !strings.Contains(fake.prompts[0], "Accepted proposal 2, the mission idea “Round the totals”: it's mission round-the-totals now.\n\nwhat next?") {
		t.Errorf("told next: %q", fake.prompts)
	}
}
