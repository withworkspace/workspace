package mission

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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
		}, court: CourtYou, kind: KindToBrief, group: GroupUpNext, verb: VerbConfirm, text: "Brief and plan written: confirm to start"},
		{name: "parked", state: Parked, court: CourtYou, kind: KindParked, group: GroupUpNext, verb: VerbUnpark},
		// The plan's at a step of yours, with nothing asking you of it.
		{name: "at a checkpoint", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan("## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Find it.\n   - who: crew\n2. Is it the cause?\n   - who: you\n", nil)
			s.M.Stages = []StageRecord{{Text: "Find it.", N: 1, Of: 2, Crew: []string{"c1"}, Accepted: ago(time.Minute)}}
			s.M.Crew = []Crew{{ID: "c1", Status: CrewDone, Ended: ago(time.Hour)}}
		}, court: CourtYou, kind: KindStepReview, group: GroupNeedsYou, verb: VerbReviewStep, text: "Step 2 is yours to review: Is it the cause?"},
		{name: "at a decision", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan("## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Work: find it.\n2. Decide: is it the cause?\n", nil)
			s.M.Stages = []StageRecord{{Text: "find it.", N: 1, Of: 2, Crew: []string{"c1"}, Accepted: ago(time.Minute)}}
			s.M.Crew = []Crew{{ID: "c1", Status: CrewDone, Ended: ago(time.Hour)}}
		}, court: CourtYou, kind: KindAccept, group: GroupNeedsYou, verb: VerbAccept, text: "Step 2 is yours: is it the cause? Go on when satisfied"},
		// The plan's at the planner's step: the planner at work on it, a
		// revision of it that's yours to confirm, or neither (it asked).
		{name: "the planner at work", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithPlanner, nil)
			s.M.Stages = []StageRecord{{Text: "Find it.", N: 1, Of: 2, Accepted: ago(time.Minute)}}
			s.Threads.Briefing = &BriefingTalk{ID: "t1", Answering: true}
		}, court: CourtAgent, kind: KindBriefing, group: GroupInProgress, verb: VerbTalk, text: "The planner is working on the plan in t1", target: "t1"},
		{name: "the planner's revision is yours", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithPlanner, nil)
			s.M.Stages = []StageRecord{{Text: "Find it.", N: 1, Of: 2, Accepted: ago(time.Minute)}}
			s.Threads.Briefing = &BriefingTalk{ID: "t1", Waiting: true, PlanWhy: "It changes Z, which is the user's to confirm."}
		}, court: CourtYou, kind: KindBriefTalk, group: GroupNeedsYou, verb: VerbTalk, text: "The planner proposes a plan revision in t1: It changes Z, which is the user's to confirm", target: "t1"},
		{name: "the planner asked something", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithPlanner, nil)
			s.M.Stages = []StageRecord{{Text: "Find it.", N: 1, Of: 2, Accepted: ago(time.Minute)}}
			s.Threads.Briefing = &BriefingTalk{ID: "t1", Waiting: true}
		}, court: CourtYou, kind: KindAccept, group: GroupNeedsYou, verb: VerbTalk, text: "Step 2 is the planner's: Plan the fix. Next steps not written: answer in t1", target: "t1"},
		// The plan's at a step that waits on someone outside ws.
		{name: "at an outside step, not happened", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithOutside, nil)
			s.M.Stages = []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Crew: []string{"c1"}, Accepted: ago(time.Minute)}}
			s.M.Crew = []Crew{{ID: "c1", Status: CrewDone, Ended: ago(time.Hour)}}
		}, court: CourtOutside, kind: KindWaitingOn, group: GroupWaiting, verb: VerbWait, text: "Step 2 waits on a teammate's merge: Merged. Not watched: mark done once it happens"},
		// The orchestrator said the mission waits, at a step that waits
		// already: the step's is the headline, with marking it done.
		{name: "at an outside step, and the orchestrator said it waits", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithOutside, nil)
			s.M.Stages = []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Crew: []string{"c1"}, Accepted: ago(time.Minute)}}
			s.M.Crew = []Crew{{ID: "c1", Status: CrewDone, Ended: ago(time.Hour)}}
			s.M.Waiting = &Proposal{Text: "a teammate's merge; the user says when it has happened", By: "orchestrator", At: now}
		}, court: CourtOutside, kind: KindWaitingOn, group: GroupWaiting, verb: VerbWait, text: "Step 2 waits on a teammate's merge: Merged. Not watched: mark done once it happens"},
		{name: "at an outside step, every PR merged", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithOutside, nil)
			s.M.Stages = []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Crew: []string{"c1"}, Accepted: ago(time.Minute)}}
			s.M.Crew = []Crew{{ID: "c1", Status: CrewDone, Ended: ago(time.Hour)}}
			s.CapsulesLanded = true
		}, court: CourtYou, kind: KindAccept, group: GroupNeedsYou, verb: VerbWait, text: "All pull requests merged"},
		// A Wait says how it ends: who it's on, and whether ws watches for it.
		{name: "a wait nobody's watching, a day on", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithWait("   - on: the platform team\n"), nil)
			s.M.Stages = []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Accepted: ago(50 * time.Hour)}, {Text: "**Deploy.** The token service is deployed.", N: 2, Of: 2, Watch: &Watch{Since: *ago(50 * time.Hour)}}}
		}, court: CourtYou, kind: KindAccept, group: GroupNeedsYou, verb: VerbWait, text: "Step 2 has waited 2 days on the platform team: Deploy. The token service is deployed. Not watched: has it happened?"},
		{name: "a wait ws watches", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithWait("   - on: npm\n   - check: `npm view pkg@1.0.0 version` passes\n"), nil)
			s.M.Stages = []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Accepted: ago(50 * time.Hour)}, {Text: "**Deploy.** The token service is deployed.", N: 2, Of: 2,
				Watch: &Watch{Since: *ago(50 * time.Hour), Looked: ago(2 * time.Minute), Looks: 9, Results: []CheckResult{{Check: "`npm view pkg@1.0.0 version` passes", Result: CheckUnmet}}}}}
			s.CapsulesLanded = true // watched, it isn't asked of you however long it waits
		}, court: CourtOutside, kind: KindWaitingOn, group: GroupWaiting, verb: VerbWait, text: "Step 2 waits on npm: Deploy. The token service is deployed. Watching (`npm view pkg@1.0.0 version`); last checked 2m ago: not yet"},
		{name: "a wait on you, asked", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithWait("   - on: you\n"), nil)
			s.M.Stages = []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Accepted: ago(time.Minute)}}
			s.M.Tasks = []Task{{N: 1, Verb: TaskDo, Step: 2, Text: "Step 2 is yours, to do: Deploy.", Options: []string{WaitDone}, By: "ws", At: now}}
		}, court: CourtYou, kind: KindToDo, group: GroupNeedsYou, verb: VerbAnswer, target: "1", text: "Step 2 is yours, to do: Deploy."},
		{name: "a wait on you, nothing asking", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithWait("   - on: you\n"), nil)
			s.M.Stages = []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Accepted: ago(time.Minute)}}
		}, court: CourtYou, kind: KindToDo, group: GroupNeedsYou, verb: VerbAccept, text: "Step 2 is yours to do: Deploy. The token service is deployed. Mark done when finished"},
		// Every step passed: at Z, if Z holds.
		{name: "every step passed, nothing suggested", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithOutside, nil)
			s.M.Stages = []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Accepted: ago(time.Hour)}, {Text: "Merged.", N: 2, Of: 2, Accepted: ago(time.Minute)}}
			s.M.Crew = []Crew{{ID: "c1", Status: CrewDone, Ended: ago(time.Hour)}}
		}, court: CourtYou, kind: KindWrapUp, group: GroupNeedsYou, verb: VerbEnd, text: "All steps done"},
		{name: "every step passed and its work merged, nothing suggested", state: Active, edit: func(s *Snapshot) {
			s.M.Plan, _ = ParsePlan(planWithOutside, nil)
			s.M.Stages = []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Accepted: ago(time.Hour)}, {Text: "Merged.", N: 2, Of: 2, Accepted: ago(time.Minute)}}
			s.M.Crew = []Crew{{ID: "c1", Status: CrewDone, Ended: ago(time.Hour)}}
			s.CapsulesLanded = true
		}, court: CourtYou, kind: KindWrapUp, group: GroupNeedsYou, verb: VerbEnd, text: "All steps done and work merged"},
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
		}, court: CourtYou, kind: KindLand, group: GroupNeedsYou, verb: VerbLand, text: "records/adr is approved: land 1 commit on main", target: "records/adr"},
		{name: "commits to land, crew still in the capsule", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Capsule: "records/adr", Activity: ActivityWorking, ActivityAt: ago(time.Minute)}}
			s.Direct = []DirectFacts{{Capsule: "records/adr", Branch: "main", Ahead: 2}}
		}, court: CourtAgent, kind: KindWorking, group: GroupInProgress, verb: VerbWatch, target: "c1"},
		{name: "everything landed, with no pull request", state: Landed, edit: func(s *Snapshot) {
			s.M.Capsules = []Capsule{{Repo: "records", Name: "adr", Landed: &Landing{At: now, Head: "abc1234"}}}
		}, court: CourtYou, kind: KindLanded, group: GroupPast, verb: VerbArchive, text: "Everything landed"},

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
		{name: "crew asked, then a restart ended them: still asking, a reply resumes them", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{{ID: "c1", Conversation: "x", Capsule: "api/fix", Status: CrewAsking, Note: "Re-run it or fix the race?",
				StatusAt: ago(5 * time.Hour), Ended: ago(time.Minute), Stopped: true, Interrupted: true}}
		}, court: CourtYou, kind: KindCrewAsks, group: GroupNeedsYou, verb: VerbReply, text: "Re-run it or fix the race?", target: "c1"},
		{name: "crew asked and ended, later crew took the capsule: theirs now", state: Active, edit: func(s *Snapshot) {
			s.M.Crew = []Crew{
				{ID: "c1", Conversation: "x", Capsule: "api/fix", Status: CrewAsking, Note: "Which?", Started: *ago(3 * time.Hour), StatusAt: ago(2 * time.Hour), Ended: ago(2 * time.Hour)},
				{ID: "c2", Capsule: "api/fix", Started: *ago(time.Hour), Activity: ActivityWorking, ActivityAt: ago(time.Minute)},
			}
		}, court: CourtAgent, kind: KindWorking, group: GroupInProgress},
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
		}, court: CourtAgent, kind: KindWorking, group: GroupInProgress, text: "is finding the answer"},
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
		}, court: CourtAgent, kind: KindAnswering, group: GroupInProgress, verb: VerbReviewDoc, target: "records/tokens:proposal.md", text: "An agent is answering a comment on proposal.md (t2)"},
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
		{name: "PR's checks running, set to merge by itself", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", URL: "u", Review: "APPROVED", Checks: "pending", AutoMerge: true}}
		}, court: CourtOutside, kind: KindChecksRunning, group: GroupWaiting, verb: VerbOpen, target: "api#88", text: "auto-merge on"},
		{name: "PR's checks failing", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", Review: "REVIEW_REQUIRED", Checks: "failure"}}
		}, court: CourtYou, kind: KindChecksFailing, group: GroupNeedsYou, verb: VerbOpen},
		{name: "PR's review asks for changes", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", Review: "CHANGES_REQUESTED"}}
		}, court: CourtYou, kind: KindChangesRequested, group: GroupNeedsYou, verb: VerbOpen},
		{name: "PR approved and green, but it conflicts", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", URL: "u", Review: "APPROVED", Checks: "success", Conflict: true}}
		}, court: CourtYou, kind: KindConflict, group: GroupNeedsYou, verb: VerbOpen, target: "api#88", text: "api#88 has merge conflicts"},
		{name: "PR approved and green, GitHub working out whether it merges", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", URL: "u", Review: "APPROVED", Checks: "success", MergeUnknown: true}}
		}, court: CourtOutside, kind: KindWaitingOn, group: GroupWaiting, verb: VerbOpen, target: "api#88", text: "nothing for you to do"},
		{name: "PR waiting for review, GitHub working out whether it merges", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", Review: "REVIEW_REQUIRED", Checks: "success", MergeUnknown: true}}
		}, court: CourtOutside, kind: KindAwaitingReview, group: GroupWaiting, verb: VerbOpen},
		{name: "PR approved and green", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", URL: "u", Review: "APPROVED", Checks: "success"}}
		}, court: CourtYou, kind: KindMerge, group: GroupNeedsYou, verb: VerbMerge, target: "api#88", text: "approved, checks pass"},
		{name: "PR green, no review decision, but its rules block it (an admin could bypass)", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", URL: "u", Checks: "success", Blocked: true}}
		}, court: CourtOutside, kind: KindAwaitingReview, group: GroupWaiting, verb: VerbOpen, target: "api#88", text: "waiting for a required review"},
		{name: "PR approved and green, but its rules still block it", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", URL: "u", Review: "APPROVED", Checks: "success", Blocked: true}}
		}, court: CourtOutside, kind: KindWaitingOn, group: GroupWaiting, verb: VerbOpen, target: "api#88", text: "approved but held by branch rules"},
		{name: "PR needing no review", state: Review, edit: func(s *Snapshot) {
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88"}}
		}, court: CourtYou, kind: KindMerge, group: GroupNeedsYou, text: "no review or checks needed"},
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
		// A page hears of a merge before ws has passed the Open Pull
		// Request step it's at: it's ws's, not a mission nobody's on.
		{name: "merged at its Open Pull Request step, ws yet to pass it", state: Active, edit: func(s *Snapshot) {
			atOpenPR(s, &SeenPR{Number: 88, Checks: "success"}, ago(time.Minute))
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", Merged: true}}
		}, court: CourtAgent, kind: KindMovingOn, group: GroupInProgress, verb: VerbNone, text: "api#88 merged: passing step 2"},
		{name: "merged, as ws has seen, and the step still stands", state: Active, edit: func(s *Snapshot) {
			atOpenPR(s, &SeenPR{Number: 88, Merged: true}, ago(time.Minute))
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", Merged: true}}
		}, court: CourtYou, kind: KindStalled, group: GroupNeedsYou, verb: VerbLaunch},
		{name: "merged away from an Open Pull Request step", state: Active, edit: func(s *Snapshot) {
			atOpenPR(s, &SeenPR{Number: 88}, nil)
			s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", Merged: true}}
		}, court: CourtYou, kind: KindStalled, group: GroupNeedsYou, verb: VerbLaunch, text: "Nobody's on step 1, crew's"},
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
		cardsKeepTheRule(t, c.name, s, hs)
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

// (the loop above hands each case's cards to card_test.go, which takes
// them for the web UI's stories when asked: TestCourtCardsForStories.)

// atOpenPR is a mission whose crew fixed it in api/fix, with ws having
// last seen its pull request as seen: at step 2, Open Pull Request, once
// step 1 was accepted (when), or still at step 1.
func atOpenPR(s *Snapshot, seen *SeenPR, accepted *time.Time) {
	s.M.Plan, _ = ParsePlan("## Z\n- Merged.\n## Stops early\n- None.\n## Steps\n1. Work: fix it.\n   - in: `api/fix`\n2. Open Pull Request\n   - in: `api/fix`\n", nil)
	s.M.Capsules = []Capsule{{Repo: "api", Name: "fix", SeenPR: seen}}
	s.M.Crew = []Crew{{ID: "c1", Capsule: "api/fix", Status: CrewDone, Ended: accepted}}
	s.M.Stages = []StageRecord{{Text: "fix it.", N: 1, Of: 2, Crew: []string{"c1"}, Accepted: accepted}}
	if accepted == nil {
		ended := s.Now.Add(-time.Hour)
		s.M.Crew[0].Status, s.M.Crew[0].Ended = "", &ended
	}
}

// planWithOutside is a plan whose second step waits on someone outside ws.
const planWithOutside = "## Z\n- Merged.\n## Stops early\n- None.\n## Steps\n1. Fix it.\n   - who: crew\n2. Merged.\n   - who: outside: a teammate's merge\n"

// A step that waits says so once: what the orchestrator said of the same
// wait (ws mission wait) isn't a second headline, with nothing to do, in
// front of the one that can be marked done.
func TestResolve_AWaitingStepSaysItOnce(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	done := now.Add(-time.Minute)
	plan, _ := ParsePlan(planWithOutside, nil)
	s := Snapshot{Now: now, State: Active, M: &Mission{Slug: "s", Plan: plan,
		Stages:  []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Accepted: &done}},
		Waiting: &Proposal{Text: "a teammate's merge", By: "orchestrator", At: now},
	}}
	hs := Resolve(s)
	if len(hs) != 1 || hs[0].Action.Verb != VerbWait {
		t.Fatalf("headlines %+v; want the step's alone, to mark done", hs)
	}
	// Anywhere else, what the orchestrator said is where it stands.
	s.M.Plan, s.M.Stages = nil, nil
	if hs := Resolve(s); len(hs) != 1 || !strings.Contains(hs[0].Text, "Waiting on a teammate's merge") {
		t.Errorf("headlines %+v; want what the orchestrator said it waits on", hs)
	}
}

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

// Crew that asked how to go on with a red pull request, and whose session
// a restart ended, are asked first: the red checks are what they asked
// about, and a link to them alone says nothing of it.
func TestResolve_AnEndedQuestionComesBeforeItsRedPR(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	s := Snapshot{State: Review, Now: now, M: &Mission{Slug: "s", Crew: []Crew{
		{ID: "c1", Conversation: "x", Capsule: "api/fix", Status: CrewAsking, Note: "The flake isn't ours: re-run, or fix it here?",
			Started: *ago(6 * time.Hour), StatusAt: ago(5 * time.Hour), Ended: ago(time.Minute), Stopped: true},
		// Gone before c1 was last at work there: not taken over.
		{ID: "c2", Capsule: "api/fix", Started: *ago(5*time.Hour + 20*time.Minute), Ended: ago(5*time.Hour + 10*time.Minute)},
	}}}
	s.PRs = []PRFacts{{Capsule: "api/fix", Ref: "api#88", URL: "u", Checks: "failure"}}
	hs := Resolve(s)
	if hs[0].Kind != KindCrewAsks || hs[0].By != "c1" || hs[0].Action.Verb != VerbReply {
		t.Fatalf("first headline %s by %s (%s), want c1's question to reply to", hs[0].Kind, hs[0].By, hs[0].Action.Verb)
	}
	if !slices.ContainsFunc(hs, func(h Headline) bool { return h.Kind == KindChecksFailing }) {
		t.Errorf("the red checks went: %v", kindStrings(kindsOf(hs)))
	}
}

func kindsOf(hs []Headline) []Kind {
	var out []Kind
	for _, h := range hs {
		out = append(out, h.Kind)
	}
	return out
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
		"c1 stopped when Workspace restarted",
		"c2's session closed: work handed back",
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
	// At a step of yours, not crew's: at crew's, a PR's headline is the
	// plan's to come to (TestResolve_PRsAtACrewStep).
	plan, err := ParsePlan("## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Is it the cause?\n   - who: you\n2. Plan the fix.\n   - who: the planner\n", nil)
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

// planWithWait is a plan of a Work step and a Wait, with the Wait's
// parameters as given.
func planWithWait(params string) string {
	return "## Z\n- Deployed.\n## Stops early\n- None.\n## Steps\n1. Work: Fix it.\n2. Wait: **Deploy.** The token service is deployed.\n" + params
}

// TestCourtCardsForStories (after the table's test, in this file, so it has
// its cards) writes the court's cases as cards where the web
// UI's stories read them, when asked (scripts/story-fixtures.mjs asks):
//
//	WS_STORY_FIXTURES=1 go test ./internal/mission -run 'TestResolve_EveryMissionIsInOneCourt|TestCourtCardsForStories'
func TestCourtCardsForStories(t *testing.T) {
	if os.Getenv("WS_STORY_FIXTURES") == "" {
		t.Skip("WS_STORY_FIXTURES isn't set")
	}
	if len(courtCards) == 0 {
		t.Fatal("no cards: run it with TestResolve_EveryMissionIsInOneCourt, which makes them")
	}
	data, err := json.MarshalIndent(courtCards, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("..", "..", "web", "src", "mission", "fixtures", "court.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The plan at a step of crew's, nobody on it: a pull request in one of its
// capsules (one that conflicts, while the step's to merge in main), or
// commits to land, aren't the headline; that nobody's on the step is. Past
// it, at a step that waits on the PR, the PR is.
func TestResolve_PRsAtACrewStep(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	plan, err := ParsePlan("## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Work: merge in main and make the changes.\n2. Wait: **CI.** Its checks pass.\n   - on: CI\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	conflicts := PRFacts{Capsule: "api/fix", Ref: "api#88", URL: "u", Review: "APPROVED", Checks: "success", Conflict: true}
	s := Snapshot{State: Active, Now: now, M: &Mission{Slug: "s", Plan: plan,
		Crew: []Crew{{ID: "c1", Capsule: "api/fix", Ended: ago(time.Hour)}},
	}}
	s.PRs = []PRFacts{conflicts}
	s.Direct = []DirectFacts{{Capsule: "records/fix", Ahead: 2}}
	hs := Resolve(s)
	if len(hs) != 1 || hs[0].Kind != KindStalled || !strings.Contains(hs[0].Text, "Nobody's on step 1") {
		t.Fatalf("at crew's step, nobody on it: %d headlines, the first %s %q; want stalled alone, on step 1", len(hs), hs[0].Kind, hs[0].Text)
	}

	// Step 1 passed: at CI's step, the conflict is yours.
	s.M.Stages = []StageRecord{{Text: "merge in main and make the changes.", N: 1, Of: 2, Crew: []string{"c1"}, Accepted: ago(time.Minute)}}
	if hs := Resolve(s); hs[0].Kind != KindConflict {
		t.Errorf("at CI's step: the first headline %s, want conflict", hs[0].Kind)
	}

	// No plan: the PR's headline as ever.
	s.M.Plan, s.M.Stages = nil, nil
	if hs := Resolve(s); hs[0].Kind != KindConflict {
		t.Errorf("no plan: the first headline %s, want conflict", hs[0].Kind)
	}
}

// A pull request is yours to merge when its headline says so; what only
// time passes (checks, a review, the base's rules) can be left to merge
// by itself; what needs work can't.
func TestMergeHeld(t *testing.T) {
	for _, c := range []struct {
		name       string
		pr         PRFacts
		held, wait bool
	}{
		{"approved and green", PRFacts{Ref: "api#1", Review: "APPROVED", Checks: "success"}, false, false},
		{"needs no review", PRFacts{Ref: "api#1"}, false, false},
		{"checks running", PRFacts{Ref: "api#1", Review: "APPROVED", Checks: "pending"}, true, true},
		{"waiting for review", PRFacts{Ref: "api#1", Review: "REVIEW_REQUIRED", Checks: "success"}, true, true},
		{"held by its rules (an admin could bypass)", PRFacts{Ref: "api#1", Checks: "success", Blocked: true}, true, true},
		{"GitHub working it out", PRFacts{Ref: "api#1", Review: "APPROVED", MergeUnknown: true}, true, true},
		{"conflicts", PRFacts{Ref: "api#1", Review: "APPROVED", Conflict: true}, true, false},
		{"checks failing", PRFacts{Ref: "api#1", Checks: "failure"}, true, false},
		{"changes asked for", PRFacts{Ref: "api#1", Review: "CHANGES_REQUESTED"}, true, false},
	} {
		held, wait := MergeHeld(c.pr)
		if (held != "") != c.held || wait != c.wait {
			t.Errorf("%s: held %q, wait %v; want held %v, wait %v", c.name, held, wait, c.held, c.wait)
		}
	}
}

// The card of a pull request ready to merge merges it, here; one only
// waiting offers to merge by itself once it can, or to stop.
func TestMergeCards(t *testing.T) {
	ready := prHeadline(PRFacts{Ref: "api#1", URL: "u", Review: "APPROVED", Checks: "success"})
	if ready.Action.Verb != VerbMerge || ready.Action.AutoMerge != "" {
		t.Fatalf("ready: %+v", ready.Action)
	}
	waiting := prHeadline(PRFacts{Ref: "api#1", URL: "u", Review: "APPROVED", Checks: "pending"})
	if waiting.Action.AutoMerge != "can" {
		t.Fatalf("waiting: %+v", waiting.Action)
	}
	on := prHeadline(PRFacts{Ref: "api#1", URL: "u", Review: "APPROVED", Checks: "pending", AutoMerge: true})
	if on.Action.AutoMerge != "on" {
		t.Fatalf("set to merge: %+v", on.Action)
	}
	failing := prHeadline(PRFacts{Ref: "api#1", URL: "u", Checks: "failure"})
	if failing.Action.AutoMerge != "" {
		t.Fatalf("failing: %+v", failing.Action)
	}
}
