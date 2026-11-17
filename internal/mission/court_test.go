package mission

import (
	"strings"
	"testing"
	"time"
)

func TestResolve_EveryMissionIsInOneCourt(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	stagedBrief := "## Stages\n1. Write the proposal in `records/adr`.\n2. Build it in `api/fix`.\n"

	cases := []struct {
		name   string
		state  State
		edit   func(s *Snapshot)
		court  Court
		kind   Kind
		group  Group
		verb   Verb
		text   string // in the headline's text
		target string
	}{
		{name: "ready, never started", state: Draft, court: CourtYou, kind: KindReady, group: GroupUpNext, verb: VerbLaunch},
		{name: "only briefed so far", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Role: RoleBriefing, Ended: ago(time.Hour)}}
		}, court: CourtYou, kind: KindReady, group: GroupUpNext, verb: VerbLaunch},
		{name: "waiting to be briefed", state: Briefing, edit: func(s *Snapshot) { s.M.Request = "fe issue 45" },
			court: CourtYou, kind: KindToBrief, group: GroupUpNext, verb: VerbBrief, text: "fe issue 45"},
		// Its brief and plan written, not confirmed: it's to confirm, not
		// to brief again.
		{name: "briefed and planned, to confirm", state: Briefing, edit: func(s *Snapshot) {
			s.M.Request, s.Brief = "fe issue 45", "## Goal\nCache tokens per tenant.\n"
			s.M.Plan, _ = ParsePlan("## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Work: **Cache them.** Cache tokens.\n", nil)
		}, court: CourtYou, kind: KindToBrief, group: GroupUpNext, verb: VerbConfirm, text: "Its brief and plan are written: confirm them to start it"},
		{name: "parked", state: Parked, court: CourtYou, kind: KindParked, group: GroupUpNext, verb: VerbUnpark},
		// The plan's at a step of yours, with nothing asking you of it.
		{name: "at a checkpoint", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan("## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Find it.\n   - who: crew\n2. Is it the cause?\n   - who: you\n", nil)
			s.M.Stages = []StageRecord{{Text: "Find it.", N: 1, Of: 2, Crew: []string{"c1"}, Accepted: ago(time.Minute)}}
			s.M.Crew = []Crew{{ID: "c1", Status: CrewDone, Ended: ago(time.Hour)}}
		}, court: CourtYou, kind: KindStepReview, group: GroupNeedsYou, verb: VerbReviewStep, text: "Step 2 is yours, to review: Is it the cause?"},
		{name: "at a decision", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan("## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Work: find it.\n2. Decide: is it the cause?\n", nil)
			s.M.Stages = []StageRecord{{Text: "find it.", N: 1, Of: 2, Crew: []string{"c1"}, Accepted: ago(time.Minute)}}
			s.M.Crew = []Crew{{ID: "c1", Status: CrewDone, Ended: ago(time.Hour)}}
		}, court: CourtYou, kind: KindAccept, group: GroupNeedsYou, verb: VerbAccept, text: "Step 2 is yours: is it the cause? Go on when you're satisfied"},
		// The plan's at the planner's step: the planner at work on it, a
		// revision of it that's yours to confirm, or neither (it asked).
		{name: "the planner at work", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithPlanner, nil)
			s.M.Stages = []StageRecord{{Text: "Find it.", N: 1, Of: 2, Accepted: ago(time.Minute)}}
			s.Threads.Briefing = &BriefingTalk{ID: "t1", Answering: true}
		}, court: CourtAgent, kind: KindBriefing, group: GroupInProgress, verb: VerbTalk, text: "The planner is at work on the plan, in t1", target: "t1"},
		{name: "the planner's revision is yours", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithPlanner, nil)
			s.M.Stages = []StageRecord{{Text: "Find it.", N: 1, Of: 2, Accepted: ago(time.Minute)}}
			s.Threads.Briefing = &BriefingTalk{ID: "t1", Waiting: true, PlanWhy: "It changes Z, which is the user's to confirm."}
		}, court: CourtYou, kind: KindBriefTalk, group: GroupNeedsYou, verb: VerbTalk, text: "The planner proposes a revision of the plan, in t1: It changes Z, which is the user's to confirm", target: "t1"},
		{name: "the planner asked something", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithPlanner, nil)
			s.M.Stages = []StageRecord{{Text: "Find it.", N: 1, Of: 2, Accepted: ago(time.Minute)}}
			s.Threads.Briefing = &BriefingTalk{ID: "t1", Waiting: true}
		}, court: CourtYou, kind: KindAccept, group: GroupNeedsYou, verb: VerbTalk, text: "Step 2 is the planner's: Plan the fix. It hasn't written the steps after it: answer it in t1", target: "t1"},
		// The plan's at a step that waits on someone outside ws.
		{name: "at an outside step, not happened", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithOutside, nil)
			s.M.Stages = []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Crew: []string{"c1"}, Accepted: ago(time.Minute)}}
			s.M.Crew = []Crew{{ID: "c1", Status: CrewDone, Ended: ago(time.Hour)}}
		}, court: CourtOutside, kind: KindWaitingOn, group: GroupWaiting, text: "Step 2 waits on outside: a teammate's merge: Merged."},
		{name: "at an outside step, every PR merged", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithOutside, nil)
			s.M.Stages = []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Crew: []string{"c1"}, Accepted: ago(time.Minute)}}
			s.M.Crew = []Crew{{ID: "c1", Status: CrewDone, Ended: ago(time.Hour)}}
			s.CapsulesLanded = true
		}, court: CourtYou, kind: KindAccept, group: GroupNeedsYou, verb: VerbAccept, text: "Every pull request has merged"},
		// Every step passed: at Z, if Z holds.
		{name: "every step passed, nothing suggested", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithOutside, nil)
			s.M.Stages = []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Accepted: ago(time.Hour)}, {Text: "Merged.", N: 2, Of: 2, Accepted: ago(time.Minute)}}
			s.M.Crew = []Crew{{ID: "c1", Status: CrewDone, Ended: ago(time.Hour)}}
		}, court: CourtYou, kind: KindWrapUp, group: GroupNeedsYou, verb: VerbEnd, text: "check each part of Z"},
		{name: "an ending suggested", state: Active, edit: func(s *Snapshot) {
			s.M.Conclude = &Conclusion{Ending: EndAnswered, Text: "Not a bug.", By: "orchestrator", Z: []ZCheck{{Z: "Product told.", Met: true, How: "in the handoff"}}}
		}, court: CourtYou, kind: KindWrapUp, group: GroupNeedsYou, verb: VerbEnd, text: "Ready to end, answered (at Z: 1 of 1 met): Not a bug."},
		{name: "ended, as you said", state: Answered, edit: func(s *Snapshot) {
			s.M.Ending = &Conclusion{Ending: EndAnswered, Text: "Not a bug.", By: "you"}
			s.M.Crew = []Crew{{ID: "c1", Capsule: "api/fix", Activity: ActivityWorking}} // history now
		}, court: CourtYou, kind: KindEnded, group: GroupPast, verb: VerbArchive, text: "Ended, answered: Not a bug."},
		{name: "landed", state: Landed, court: CourtYou, kind: KindLanded, group: GroupPast, verb: VerbArchive},
		{name: "archived", state: Archived, court: "", kind: KindArchived, group: GroupPast},

		// A repo with no pull requests: its capsule's commits land when you say.
		{name: "commits to land, nobody in the capsule", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Capsule: "records/adr", Ended: ago(time.Hour)}}
			s.Direct = []DirectFacts{{Capsule: "records/adr", Branch: "main", Ahead: 2}}
		}, court: CourtYou, kind: KindLand, group: GroupNeedsYou, verb: VerbLand, text: "records/adr has 2 commits to land on main", target: "records/adr"},
		{name: "commits to land, approved as they are", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Capsule: "records/adr", Ended: ago(time.Hour)}}
			s.Direct = []DirectFacts{{Capsule: "records/adr", Branch: "main", Ahead: 1, Approved: true}}
		}, court: CourtYou, kind: KindLand, group: GroupNeedsYou, verb: VerbLand, text: "records/adr is approved: land its 1 commit on main", target: "records/adr"},
		{name: "commits to land, crew still in the capsule", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Capsule: "records/adr", Activity: ActivityWorking, ActivityAt: ago(time.Minute)}}
			s.Direct = []DirectFacts{{Capsule: "records/adr", Branch: "main", Ahead: 2}}
		}, court: CourtAgent, kind: KindWorking, group: GroupInProgress, verb: VerbWatch, target: "c1"},
		{name: "everything landed, with no pull request", state: Landed, edit: func(s *Snapshot) {
			s.M.Capsules = []Capsule{{Repo: "records", Name: "adr", Landed: &Landing{At: now, Head: "abc1234"}}}
		}, court: CourtYou, kind: KindLanded, group: GroupPast, verb: VerbArchive, text: "Everything has landed"},

		{name: "crew working", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c2", Capsule: "api/fix", Activity: ActivityWorking, ActivityAt: ago(time.Minute), Dispatch: "Build the cache"}}
		}, court: CourtAgent, kind: KindWorking, group: GroupInProgress, verb: VerbWatch, text: "c2 is working: Build the cache", target: "c2"},
		{name: "several crew working", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Activity: ActivityWorking}, {ID: "c2", Activity: ActivityWorking}}
		}, court: CourtAgent, kind: KindWorking, group: GroupInProgress, verb: VerbWatch, text: "c1 and c2 are working"},
		{name: "briefing agent working", state: Briefing, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Role: RoleBriefing, Activity: ActivityWorking}}
		}, court: CourtAgent, kind: KindBriefing, group: GroupInProgress, verb: VerbWatch},
		{name: "says it's working, screen still: a prompt mid-turn", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Activity: ActivityWorking, ActivityAt: ago(time.Minute)}}
			s.Seen = Seen{Running: map[string]bool{"c1": true}, Still: map[string]time.Duration{"c1": time.Minute}}
		}, court: CourtYou, kind: KindReplied, group: GroupNeedsYou, verb: VerbReply, target: "c1"},
		{name: "no hooks, screen moving", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1"}}
			s.Seen = Seen{Running: map[string]bool{"c1": true}, Still: map[string]time.Duration{"c1": time.Second}}
		}, court: CourtAgent, kind: KindWorking, group: GroupInProgress, verb: VerbWatch},
		// ws starting crew: marked on the mission until they're recorded,
		// then on them until they're handed their prompt.
		{name: "ws starting crew, none recorded yet", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Hour), Status: CrewDone}}
			s.M.Launching = &Launching{What: "step 2", Since: now}
		}, court: CourtAgent, kind: KindLaunching, group: GroupInProgress, text: "Starting crew on step 2"},
		{name: "ws starting crew, for nothing in particular", state: Active, edit: func(s *Snapshot) {
			s.M.Launching = &Launching{Since: now}
		}, court: CourtAgent, kind: KindLaunching, group: GroupInProgress, text: "Starting crew"},
		{name: "a brief accepted, ws starting it before it's confirmed", state: Briefing, edit: func(s *Snapshot) {
			s.M.Launching = &Launching{What: "step 1", Since: now}
			s.Threads.Briefing = &BriefingTalk{ID: "t1", Waiting: true}
		}, court: CourtAgent, kind: KindLaunching, group: GroupInProgress, text: "Starting crew on step 1"},
		{name: "crew recorded, their session starting", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c2", Capsule: "api/fix", Starting: ago(time.Second), Dispatch: "Build the cache"}}
		}, court: CourtAgent, kind: KindLaunching, group: GroupInProgress, verb: VerbWatch, text: "Starting c2: Build the cache", target: "c2"},
		{name: "crew resumed, starting: what they last did is from before", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Starting: ago(time.Second), Activity: ActivityWaiting, ActivityAt: ago(time.Hour), LastReply: "Done?"}}
		}, court: CourtAgent, kind: KindLaunching, group: GroupInProgress, text: "Starting c1"},
		{name: "several crew starting", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Starting: ago(time.Second)}, {ID: "c2", Starting: ago(time.Second)}}
		}, court: CourtAgent, kind: KindLaunching, group: GroupInProgress, text: "Starting c1 and c2"},
		{name: "starting crew that ended: not starting", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Starting: ago(time.Minute), Ended: ago(time.Second), Exit: "failed to start"}}
		}, court: CourtYou, kind: KindStalled, group: GroupNeedsYou},
		{name: "orchestrator running", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Hour), Status: CrewDone, StatusAt: ago(time.Hour)}}
			s.M.Orchestrating = &OrchestratorRun{For: "c1 (done)", Since: now.Add(-time.Minute), Until: now.Add(time.Minute)}
		}, court: CourtAgent, kind: KindDeciding, group: GroupInProgress, verb: VerbLog, text: "looking at c1 (done)"},
		{name: "orchestrator run outlived its time: it died", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Hour)}}
			s.M.Orchestrating = &OrchestratorRun{Since: now.Add(-time.Hour), Until: now.Add(-time.Minute)}
		}, court: CourtYou, kind: KindStalled, group: GroupNeedsYou, verb: VerbLaunch},
		// Taking an event marks the orchestrator's run underway (ops.Notify):
		// there's no gap between them for the court to guess across.
		{name: "crew just reported done: the orchestrator's taken it", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Activity: ActivityWaiting, Status: CrewDone, Note: "Added it", StatusAt: ago(time.Second)}}
			s.M.Orchestrating = &OrchestratorRun{For: "c1 (done)", Since: now, Until: now.Add(time.Minute)}
		}, court: CourtAgent, kind: KindDeciding, group: GroupInProgress, text: "looking at c1 (done)"},
		{name: "crew just ended by itself: the orchestrator's taken it", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Second), Exit: "exit 0"}}
			s.M.Orchestrating = &OrchestratorRun{For: "c1 (ended)", Since: now, Until: now.Add(time.Minute)}
		}, court: CourtAgent, kind: KindDeciding, group: GroupInProgress, text: "c1 (ended)"},
		{name: "crew stopped by you: not handed over", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Second), Exit: "hangup", Stopped: true}}
		}, court: CourtYou, kind: KindStalled, group: GroupNeedsYou},

		{name: "crew went quiet after ws's message: the orchestrator's taken it", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Activity: ActivityWaiting, ActivityAt: ago(time.Second), LastReply: "Should I add a test?"}}
			s.M.Orchestrating = &OrchestratorRun{For: "c1 (quiet)", Since: now, Until: now.Add(time.Minute)}
		}, court: CourtAgent, kind: KindDeciding, group: GroupInProgress, text: "looking at c1 (quiet)"},
		{name: "crew went quiet, the orchestrator running", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Activity: ActivityWaiting, ActivityAt: ago(time.Minute)}}
			s.M.Orchestrating = &OrchestratorRun{For: "c1 (quiet)", Since: now.Add(-time.Minute), Until: now.Add(time.Minute)}
		}, court: CourtAgent, kind: KindDeciding, group: GroupInProgress},
		{name: "crew went quiet, the orchestrator looked and left it", state: Active, edit: func(s *Snapshot) {
			c := Crew{ID: "c1", Activity: ActivityWaiting, ActivityAt: ago(time.Second), LastReply: "Done?"}
			c.Orchestrated = c.eventKey()
			s.M.Crew = []Crew{c}
		}, court: CourtYou, kind: KindReplied, group: GroupNeedsYou, text: "c1 is waiting for you: Done?"},
		{name: "once the orchestrator has looked: crew done, nobody now", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Activity: ActivityWaiting, Status: CrewDone, StatusAt: ago(time.Second), Note: "Built it"}}
		}, court: CourtYou, kind: KindStalled, group: GroupNeedsYou},
		{name: "crew answering you", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Conversing: true, Activity: ActivityWaiting, ActivityAt: ago(time.Second), LastReply: "Yes, per-org."}}
		}, court: CourtYou, kind: KindReplied, group: GroupNeedsYou, verb: VerbReply, text: "c1 replied: Yes, per-org."},
		{name: "crew went quiet, orchestrator off", state: Active, edit: func(s *Snapshot) {
			s.OrchestratorOff = true
			s.M.Crew = []Crew{{ID: "c1", Activity: ActivityWaiting, ActivityAt: ago(time.Second)}}
		}, court: CourtYou, kind: KindReplied, group: GroupNeedsYou},
		{name: "a briefing agent went quiet: it's asking you", state: Briefing, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Role: RoleBriefing, Activity: ActivityWaiting, ActivityAt: ago(time.Second)}}
		}, court: CourtYou, kind: KindReplied, group: GroupNeedsYou},
		{name: "waiting on others", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Hour), Status: CrewDone}}
			s.M.Waiting = &Proposal{Text: "the platform team's deploy of the new token service", By: "orchestrator", At: now}
		}, court: CourtOutside, kind: KindWaitingOn, group: GroupWaiting, text: "Waiting on the platform team's deploy"},
		{name: "waiting on others, but crew are at work", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Activity: ActivityWorking}}
			s.M.Waiting = &Proposal{Text: "a deploy", At: now}
		}, court: CourtAgent, kind: KindWorking, group: GroupInProgress},
		{name: "crew asks", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c2", Status: CrewAsking, Note: "Per-org or global?", StatusAt: ago(time.Minute)}}
		}, court: CourtYou, kind: KindCrewAsks, group: GroupNeedsYou, verb: VerbReply, text: "c2 asks: Per-org or global?", target: "c2"},
		{name: "crew asks, another works: you first", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Activity: ActivityWorking}, {ID: "c2", Status: CrewAsking, Note: "Which?"}}
		}, court: CourtYou, kind: KindCrewAsks, group: GroupNeedsYou, target: "c2"},
		{name: "permission", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Activity: ActivityPermission, Notice: "Claude needs your permission to use Bash"}}
		}, court: CourtYou, kind: KindPermission, group: GroupNeedsYou, verb: VerbPermission, text: "use Bash", target: "c1"},
		{name: "crew's turn is over", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Activity: ActivityWaiting, ActivityAt: ago(time.Minute), LastReply: "## Done\nI added the cache."}}
		}, court: CourtYou, kind: KindReplied, group: GroupNeedsYou, verb: VerbReply, text: "c1 is waiting for you: Done"},
		{name: "flagged", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Hour)}}
			s.M.Tasks = []Task{{N: 1, Verb: TaskAnswer, Text: "CI is red", By: "orchestrator", Options: []string{"Fix it", "Leave it"}}}
		}, court: CourtYou, kind: KindFlag, group: GroupNeedsYou, verb: VerbAnswer, text: "The orchestrator asks: CI is red"},
		{name: "flagged, crew answering it for you", state: Active, edit: func(s *Snapshot) {
			s.M.Tasks = []Task{{N: 1, Verb: TaskAnswer, Text: "CI is red", By: "orchestrator"}}
			s.M.Crew = []Crew{{ID: "c3", Role: RoleAnswer, Activity: ActivityWorking}}
		}, court: CourtAgent, kind: KindWorking, group: GroupInProgress, text: "answer to the question"},
		{name: "back for re-review", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Hour)}}
			s.ReReview = []ReReview{{Capsule: "api/fix", Review: "r1", Answered: 2, Total: 3}}
		}, court: CourtYou, kind: KindReReview, group: GroupNeedsYou, verb: VerbReview, target: "api/fix", text: "2 of 3"},
		{name: "replies on a document's threads", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Hour)}}
			s.Threads.Replies = []DocReplies{{Doc: "records/tokens:proposal.md", Name: "proposal.md", Waiting: 2, Threads: []string{"t1", "t3"}}}
		}, court: CourtYou, kind: KindReplies, group: GroupNeedsYou, verb: VerbReviewDoc, target: "records/tokens:proposal.md", text: "2 replies on proposal.md, waiting for you (t1, t3)"},
		{name: "a conversation about it proposed crew and a decision", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Hour), Status: CrewDone}}
			s.Threads.Proposed = []TalkProposed{{ID: "t3", Kinds: []string{"dispatch", "decision", "decision"}}}
		}, court: CourtYou, kind: KindProposed, group: GroupNeedsYou, verb: VerbTalk, target: "t3", text: "t3 proposes crew and 2 decisions: confirm or decline them"},
		{name: "crew asks, and a conversation proposed something: crew first", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c2", Status: CrewAsking, Note: "Which?"}}
			s.Threads.Proposed = []TalkProposed{{ID: "t3", Kinds: []string{"mission"}}}
		}, court: CourtYou, kind: KindCrewAsks, group: GroupNeedsYou, target: "c2"},
		{name: "an agent answering a thread", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Hour)}}
			s.Threads.Answering = []DocReplies{{Doc: "records/tokens:proposal.md", Name: "proposal.md", Waiting: 1, Threads: []string{"t2"}}}
		}, court: CourtAgent, kind: KindAnswering, group: GroupInProgress, verb: VerbReviewDoc, target: "records/tokens:proposal.md", text: "an agent is answering a comment on proposal.md (t2)"},
		{name: "crew queued", state: Active, edit: func(s *Snapshot) {
			s.M.Dispatch = &Dispatch{Text: "implement it", Capsule: "api/fix", By: "orchestrator"}
		}, court: CourtYou, kind: KindQueued, group: GroupNeedsYou, verb: VerbLaunchNext, text: "queued in api/fix: implement it"},
		{name: "resume suggested", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Conversation: "x", Ended: ago(time.Hour), ResumeWith: "add a test"}}
		}, court: CourtYou, kind: KindResume, group: GroupNeedsYou, verb: VerbResume, target: "c1", text: "add a test"},
		{name: "carry on suggested, session still open", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Conversation: "x", Status: CrewDone, StatusAt: ago(time.Hour), ResumeWith: "add a test"}}
		}, court: CourtYou, kind: KindResume, group: GroupNeedsYou, verb: VerbReply, target: "c1"},
		{name: "looks finished", state: Review, edit: func(s *Snapshot) {
			s.M.Conclude = &Conclusion{Text: "all merged", By: "orchestrator"}
		}, court: CourtYou, kind: KindWrapUp, group: GroupNeedsYou, verb: VerbArchive},
		{name: "crew worked, nobody now", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Conversation: "x", Ended: ago(time.Hour)}}
		}, court: CourtYou, kind: KindStalled, group: GroupNeedsYou, verb: VerbLaunch, target: "c1"},
		{name: "the orchestrator decided nothing", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Hour), Status: CrewDone, StatusAt: ago(time.Hour)}}
			s.Events = []Event{
				{Kind: EventCrewDone, Crew: "c1"},
				{Kind: EventOrchestrateRun, Run: "r1", Text: "looking at c1 (done) with x"},
				{Kind: EventOrchestrate, Run: "r1", Text: "Waiting for CI.\nMore."},
			}
		}, court: CourtYou, kind: KindStalled, group: GroupNeedsYou, text: "the orchestrator said: Waiting for CI."},
		{name: "the orchestrator's cut report: its gist, not the cut", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Hour), Status: CrewDone, StatusAt: ago(time.Hour)}}
			s.Events = []Event{
				{Kind: EventCrewDone, Crew: "c1"},
				{Kind: EventOrchestrateRun, Run: "r1", Text: "looking at c1 (done) with x"},
				{Kind: EventOrchestrate, Run: "r1", Text: "…\n**My decision:** the note is accurate.\n\nMore."},
			}
		}, court: CourtYou, kind: KindStalled, group: GroupNeedsYou, text: "the orchestrator said: My decision: the note is accurate."},
		{name: "the orchestrator's note over its report", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Ended: ago(time.Hour), Status: CrewDone, StatusAt: ago(time.Hour)}}
			s.Events = []Event{
				{Kind: EventCrewDone, Crew: "c1"},
				{Kind: EventOrchestrateRun, Run: "r1", Text: "looking at c1 (done) with x"},
				{Kind: EventNote, Run: "r1", Text: "Orchestrator: c1's note is right; nothing more to do until the PR merges."},
				{Kind: EventOrchestrate, Run: "r1", Text: "I noted it.\nDone."},
			}
		}, court: CourtYou, kind: KindStalled, group: GroupNeedsYou, text: "the orchestrator said: c1's note is right; nothing more"},
		{name: "done crew, orchestrator off: accept", state: Active, edit: func(s *Snapshot) {
			s.OrchestratorOff = true
			s.Brief = stagedBrief
			s.M.Crew = []Crew{{ID: "c2", Status: CrewDone, Note: "Wrote it", StatusAt: ago(time.Second)}}
			s.M.Stages = []StageRecord{{Text: "Write the proposal in `records/adr`.", Crew: []string{"c2"}}}
		}, court: CourtYou, kind: KindAccept, group: GroupNeedsYou, verb: VerbAccept, text: "c2 reports stage 1 done: Wrote it"},

		{name: "PR waiting for review", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", URL: "u", Review: "REVIEW_REQUIRED", Checks: "success"}}
		}, court: CourtOutside, kind: KindAwaitingReview, group: GroupWaiting, verb: VerbOpen, target: "api#88", text: "nothing for you to do"},
		{name: "PR's checks running", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", Review: "APPROVED", Checks: "pending"}}
		}, court: CourtOutside, kind: KindChecksRunning, group: GroupWaiting, verb: VerbOpen},
		{name: "PR's checks failing", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", Review: "REVIEW_REQUIRED", Checks: "failure"}}
		}, court: CourtYou, kind: KindChecksFailing, group: GroupNeedsYou, verb: VerbOpen},
		{name: "PR's review asks for changes", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", Review: "CHANGES_REQUESTED"}}
		}, court: CourtYou, kind: KindChangesRequested, group: GroupNeedsYou, verb: VerbOpen},
		{name: "PR approved and green", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", URL: "u", Review: "APPROVED", Checks: "success"}}
		}, court: CourtYou, kind: KindMerge, group: GroupNeedsYou, verb: VerbMerge, target: "api#88", text: "approved and its checks pass"},
		{name: "PR needing no review", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88"}}
		}, court: CourtYou, kind: KindMerge, group: GroupNeedsYou, text: "needs no review, and has no checks"},
		{name: "two PRs: one to merge, one waiting: yours first", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{
				{Capsule: "web/fix", Ref: "web#3", Review: "REVIEW_REQUIRED"},
				{Capsule: "api/fix", Ref: "api#88", Review: "APPROVED", Checks: "success"},
			}
		}, court: CourtYou, kind: KindMerge, group: GroupNeedsYou, target: "api#88"},
		{name: "red checks with crew at it: the crew's", state: Review, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Capsule: "api/fix", Activity: ActivityWorking}}
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", Checks: "failure"}}
		}, court: CourtAgent, kind: KindWorking, group: GroupInProgress},
		{name: "merged PR alongside one waiting", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "a/b", Ref: "a#1", Merged: true}, {Capsule: "c/d", Ref: "c#2", Review: "REVIEW_REQUIRED"}}
		}, court: CourtOutside, kind: KindAwaitingReview, target: "c#2", group: GroupWaiting},
		{name: "in review, GitHub not asked", state: Review, court: CourtOutside, kind: KindAwaitingReview, group: GroupWaiting},
	}
	for _, c := range cases {
		s := Snapshot{M: &Mission{Slug: "s", Title: "T"}, State: c.state, Now: now}
		switch c.state {
		case Parked, Archived:
			s.M.State = c.state // stored, as DeriveState reads it
		case Briefing:
			s.M.Request = "an idea" // raised from words, not confirmed
		}
		if c.edit != nil {
			c.edit(&s)
		}
		hs := Resolve(s)
		if len(hs) == 0 {
			t.Errorf("%s: no headline", c.name)
			continue
		}
		h := hs[0]
		if h.Court != c.court || h.Kind != c.kind || h.Group() != c.group {
			t.Errorf("%s: %s/%s in %s; want %s/%s in %s (%q)", c.name, h.Court, h.Kind, GroupNames[h.Group()], c.court, c.kind, GroupNames[c.group], h.Text)
		}
		if c.verb != "" && h.Action.Verb != c.verb {
			t.Errorf("%s: action %q, want %q", c.name, h.Action.Verb, c.verb)
		}
		if c.target != "" && h.Action.Target != c.target {
			t.Errorf("%s: target %q, want %q", c.name, h.Action.Target, c.target)
		}
		if c.text != "" && !strings.Contains(h.Text, c.text) {
			t.Errorf("%s: text %q, want it to say %q", c.name, h.Text, c.text)
		}
		for _, h := range hs {
			if h.Text == "" || h.Label == "" {
				t.Errorf("%s: %s has no text or label: %+v", c.name, h.Kind, h)
			}
			if (h.Court == "") != (h.Kind == KindArchived) {
				t.Errorf("%s: only an archived mission has no court: %+v", c.name, h)
			}
			if h.Court == CourtYou && h.Action.Verb == VerbNone {
				t.Errorf("%s: %s is yours, with nothing to do", c.name, h.Kind)
			}
			if h.Action.Label == "" && h.Court != "" {
				t.Errorf("%s: %s's action has no words", c.name, h.Kind)
			}
		}
	}
}

// planWithOutside is a plan whose second step waits on someone outside ws.
const planWithOutside = "## Z\n- Merged.\n## Stops early\n- None.\n## Steps\n1. Fix it.\n   - who: crew\n2. Merged.\n   - who: outside: a teammate's merge\n"

func TestResolve_HeadlinesComeMostPressingFirst(t *testing.T) {
	s := Snapshot{M: &Mission{Slug: "s",
		Crew:  []Crew{{ID: "c1", Activity: ActivityWorking}, {ID: "c2", Activity: ActivityWaiting}, {ID: "c3", Status: CrewAsking, Note: "?"}},
		Tasks: []Task{{N: 1, Verb: TaskAnswer, Text: "go on?"}},
	}, State: Active}
	var kinds []Kind
	for _, h := range Resolve(s) {
		kinds = append(kinds, h.Kind)
	}
	want := []Kind{KindCrewAsks, KindFlag, KindReplied, KindWorking}
	if strings.Join(kindStrings(kinds), ",") != strings.Join(kindStrings(want), ",") {
		t.Errorf("order %v, want %v", kinds, want)
	}
}

func kindStrings(ks []Kind) []string {
	var out []string
	for _, k := range ks {
		out = append(out, string(k))
	}
	return out
}

func TestHeadlineKey_SameNewsSameKey(t *testing.T) {
	at := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	a := Headline{Kind: KindCrewAsks, By: "c2", Since: at}
	if a.Key() != (Headline{Kind: KindCrewAsks, By: "c2", Since: at, Text: "changed wording"}).Key() {
		t.Error("the key changed with the words")
	}
	if a.Key() == (Headline{Kind: KindCrewAsks, By: "c2", Since: at.Add(time.Second)}).Key() {
		t.Error("a new ask has the old one's key")
	}
}

func TestStore_WaitingClearsWhenAnythingHappens(t *testing.T) {
	s := owned(t)
	m, _ := s.Create("Cache tokens")
	for name, happen := range map[string]func(){
		"crew start":          func() { s.StartCrew(m.Slug, "claude", "") },
		"you act":             func() { s.UserActed(m.Slug) },
		"an orchestrator run": func() { s.StartOrchestrating(m.Slug, "the mission", time.Minute) },
	} {
		s.SetWaiting(m.Slug, "a deploy", "orchestrator")
		if got, _ := s.Load(m.Slug); got.Waiting == nil || got.Waiting.Text != "a deploy" {
			t.Fatalf("waiting = %+v", got.Waiting)
		}
		happen()
		if got, _ := s.Load(m.Slug); got.Waiting != nil {
			t.Errorf("%s didn't clear waiting", name)
		}
	}
}

func TestStore_TalkedToAndNudged(t *testing.T) {
	s := owned(t)
	m, _ := s.Create("Cache tokens")
	s.StartCrew(m.Slug, "claude", "conv")
	s.SetCrewStatus(m.Slug, "c1", CrewAsking, "which?")
	if cleared, err := s.TalkedTo(m.Slug, "c1", ""); err != nil || !cleared {
		t.Fatalf("talked to: %v, %v", cleared, err)
	}
	if got, _ := s.Load(m.Slug); !got.Crew[0].Conversing || got.Crew[0].Status != "" {
		t.Errorf("after you talked to it: %+v", got.Crew[0])
	}
	s.SetResumeWith(m.Slug, "c1", "add a test")
	s.Nudged(m.Slug, "c1", "add a test", "orchestrator")
	if got, _ := s.Load(m.Slug); got.Crew[0].Conversing || got.Crew[0].ResumeWith != "" {
		t.Errorf("after ws nudged it: %+v", got.Crew[0])
	}

	// What you tell crew is logged (what you typed in its terminal, once a
	// turn), and who said each message: you, or the orchestrator.
	s.TalkedTo(m.Slug, "c1", "")
	s.TalkedTo(m.Slug, "c1", "") // still its turn: not again
	s.TalkedTo(m.Slug, "c1", "push it, then the UI request")
	log, _ := s.ReadLog(m.Slug, 0)
	var said []string
	for _, e := range log {
		if e.Kind == EventTold || e.Kind == EventResumeWith {
			text, _ := Sentence(e)
			said = append(said, e.By+" | "+text)
		}
	}
	want := []string{
		"you | typed into c1's terminal",
		" | suggested resuming c1: add a test",
		"orchestrator | told c1: add a test",
		"you | typed into c1's terminal",
		"you | told c1: push it, then the UI request",
	}
	if strings.Join(said, "\n") != strings.Join(want, "\n") {
		t.Errorf("logged:\n%s\nwant:\n%s", strings.Join(said, "\n"), strings.Join(want, "\n"))
	}
}

func TestStore_Orchestrating(t *testing.T) {
	s := owned(t)
	m, err := s.Create("Cache tokens")
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.StartOrchestrating(m.Slug, "c1 (done)", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.Load(m.Slug)
	if got.Orchestrating == nil || got.Orchestrating.For != "c1 (done)" || !got.Orchestrating.Until.After(got.Orchestrating.Since) {
		t.Fatalf("orchestrating = %+v", got.Orchestrating)
	}
	// A run that started after another, and ends first, leaves the other's
	// mark alone.
	stale := first
	stale.Since = stale.Since.Add(-time.Hour)
	s.EndOrchestrating(m.Slug, stale)
	if got, _ := s.Load(m.Slug); got.Orchestrating == nil {
		t.Fatal("an older run's end cleared the newer run")
	}
	s.EndOrchestrating(m.Slug, first)
	if got, _ := s.Load(m.Slug); got.Orchestrating != nil {
		t.Errorf("still orchestrating after its run ended: %+v", got.Orchestrating)
	}
}

const planWithPlanner = "## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Find it.\n   - who: crew\n2. Plan the fix.\n   - who: the planner\n"

// The log says how crew ended: stopped by a restart, closed once it had
// handed its work back, stopped by you, or by itself, with its exit.
func TestEndCrew_SaysHow(t *testing.T) {
	s := owned(t)
	m, _ := s.Create("Cache tokens")
	for range 4 {
		s.StartCrew(m.Slug, "claude", "conv")
	}
	s.SetCrewStatus(m.Slug, "c2", CrewDone, "built it")
	s.EndCrewInterrupted(m.Slug, "c1", "stopped: exit 129")
	s.EndCrewStopped(m.Slug, "c2", "exit 129", true)
	s.EndCrewStopped(m.Slug, "c3", "exit 129", true)
	s.EndCrew(m.Slug, "c4", "exit 1")
	log, _ := s.ReadLog(m.Slug, 0)
	var said []string
	for _, e := range log {
		if e.Kind == EventCrewEnded {
			text, _ := Sentence(e)
			said = append(said, text)
		}
	}
	want := []string{
		"c1 was stopped by a daemon restart",
		"c2's session was closed: it had handed its work back",
		"c3 was stopped",
		"c4 ended (exit 1)",
	}
	if strings.Join(said, "\n") != strings.Join(want, "\n") {
		t.Errorf("said:\n%s\nwant:\n%s", strings.Join(said, "\n"), strings.Join(want, "\n"))
	}
}

// However many agents are on a mission at once, one headline says it's in
// the agents' court: the most pressing, crew on its route before the
// agents beside it. What's yours and what's outside all still list.
func TestResolve_OneAgentHeadlineAtATime(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	plan, err := ParsePlan(planWithPlanner, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := Snapshot{State: Active, Now: now, M: &Mission{Slug: "s", Title: "T", Plan: plan,
		Crew: []Crew{
			{ID: "c1", Capsule: "api/fix", Activity: ActivityWorking, ActivityAt: ago(time.Minute), Dispatch: "Build the cache"},
			{ID: "c2", Capsule: "web/fix", Starting: ago(time.Second), Dispatch: "Show it"},
			{ID: "c3", Status: CrewAsking, Note: "Per-org or global?", StatusAt: ago(time.Minute)},
		},
		Launching:     &Launching{What: "step 2", Since: now},
		Orchestrating: &OrchestratorRun{For: "c3 (asked)", Since: now.Add(-time.Minute), Until: now.Add(time.Minute)},
		Tasks:         []Task{{N: 1, Verb: TaskAnswer, Text: "CI is red", By: "orchestrator"}},
		Waiting:       &Proposal{Text: "the platform team's deploy", By: "orchestrator", At: now},
	}}
	s.Threads.Answering = []DocReplies{{Doc: "records/tokens:proposal.md", Name: "proposal.md", Waiting: 1, Threads: []string{"t2"}}}
	s.Threads.Briefing = &BriefingTalk{ID: "t1", Answering: true}
	s.PRs = []PRFacts{{Capsule: "docs/fix", Ref: "docs#3", Review: "REVIEW_REQUIRED"}}

	// kinds is the headlines' kinds by court, in the order they come.
	kinds := func() (agent, yours, outside []Kind) {
		for _, h := range Resolve(s) {
			switch h.Court {
			case CourtAgent:
				agent = append(agent, h.Kind)
			case CourtYou:
				yours = append(yours, h.Kind)
			case CourtOutside:
				outside = append(outside, h.Kind)
			}
		}
		return
	}
	// Each agent headline in turn, as the one before it stops applying.
	steps := []struct {
		want Kind
		then func() // what stops applying, for the next
	}{
		{KindWorking, func() { s.M.Crew[0].Ended = ago(time.Second) }},
		{KindLaunching, func() { s.M.Crew[1].Starting, s.M.Crew[1].Ended, s.M.Launching = nil, ago(time.Second), nil }},
		{KindAnswering, func() { s.Threads.Answering = nil }},
		{KindBriefing, func() { s.Threads.Briefing.Answering = false }},
		{KindDeciding, func() { s.M.Orchestrating = nil }},
	}
	for _, st := range steps {
		agent, yours, outside := kinds()
		if len(agent) != 1 || agent[0] != st.want {
			t.Errorf("agent-court headlines %v, want %s alone", agent, st.want)
		}
		// Yours and outside's are all there, yours first.
		if got := strings.Join(kindStrings(yours), ","); got != "crew-asks,flag" {
			t.Errorf("with %s: yours %s, want crew-asks,flag", st.want, got)
		}
		if got := strings.Join(kindStrings(outside), ","); got != "awaiting-review,waiting-on" {
			t.Errorf("with %s: outside %s, want awaiting-review,waiting-on", st.want, got)
		}
		if first := Resolve(s)[0]; first.Court != CourtYou {
			t.Errorf("with %s: the mission's headline is %s's %s, want yours", st.want, first.Court, first.Kind)
		}
		st.then()
	}
	// Nobody on it any more: none, and the rest as they were.
	if agent, yours, outside := kinds(); len(agent) != 0 || len(yours) != 2 || len(outside) != 2 {
		t.Errorf("nobody on it: agent %v, yours %v, outside %v", agent, yours, outside)
	}

	// Alone in its court, the one agent headline is the mission's.
	alone := Snapshot{State: Active, Now: now, M: &Mission{Slug: "s", Plan: plan,
		Crew: []Crew{{ID: "c1", Activity: ActivityWorking}, {ID: "c2", Starting: ago(time.Second)}},
	}, Threads: DocThreads{Briefing: &BriefingTalk{ID: "t1", Answering: true}}}
	if hs := Resolve(alone); len(hs) != 1 || hs[0].Kind != KindWorking {
		t.Errorf("crew working, crew starting and the planner answering: %d headlines, the first %s; want working alone", len(hs), hs[0].Kind)
	}
}
