// Package orchestrator decides what happens next on a mission. When crew
// report done or end by themselves, a headless agent reads the brief and the
// history and acts through ws commands: dispatch new crew, resume crew,
// flag the user, raise a follow-up or suggest concluding.
package orchestrator

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/prompt"
)

// The prompt goes to the agent as one argument, and Linux caps a single
// argument at 128 KiB (MAX_ARG_STRLEN), so each section has a budget and the
// whole stays well under it. What's cut is a command away.
const (
	promptCapsulesMax   = 16 << 10
	promptLogMax        = 16 << 10
	promptHandoffMax    = 12 << 10
	promptTranscriptMax = 32 << 10
	// promptTailMax is the transcript's share when crew left a handoff:
	// enough for what a handoff doesn't say (errors, test output).
	promptTailMax = 8 << 10
	// promptProcessMax is the process's share: it's read whole, as a
	// team writes it, but a process longer than this is cut.
	promptProcessMax = 12 << 10
	promptMax        = 112 << 10
	// promptLeanLog is how many of the log's latest lines the prompt has:
	// what just happened.
	promptLeanLog = 15
)

// Input is what the orchestrator is told about a mission.
type Input struct {
	Mission *mission.Mission
	// CrewID and Trigger say what prompted this run: crew that reported
	// "done" or "ended"; Trigger "manual" (or "") for a run by hand.
	CrewID, Trigger string
	// Brief is the mission's brief. Without a plan, its Acceptance is Z,
	// which the prompt lists part by part for the ending to check.
	Brief string
	// Guide is what's said of the workspace's own way of working: where
	// its guide for agents is, and how to list its skills. "" with neither.
	Guide    string
	Capsules []Capsule
	Log      []string // recent log lines, oldest first
	Output   string   // the end of CrewID's output
	// Handoff is the handoff CrewID left when it reported done, if it did.
	Handoff string
	// Stages describes the brief's stages and their progress, one line
	// each (none when the brief lists none), and CrewStage the stage
	// CrewID was on ("2: Implement it"), if it joined one. For a mission
	// with a plan (Mission.Plan), they're its steps, and the orchestrator
	// is given the plan in place of the brief's stages.
	Stages    []string
	CrewStage string
	// Review is the review the user just submitted (Trigger "review"), as
	// crew read it, ReviewRef where it's kept, and ReviewCrew the crew that
	// worked in its capsule, latest last.
	Review     string
	ReviewRef  string
	ReviewCrew []string
	// Standing is where the mission would stand if this run did nothing:
	// whose court, and why, as the deck would show it.
	Standing string
	// Decisions are what's been decided on the mission and why, the ones
	// that stand, a line each, newest first.
	Decisions []string
	// LastReply is what CrewID last said, when its turn ended quietly
	// (Trigger "quiet").
	LastReply string
	// Now is when the orchestrator's told it: how long a quiet turn ws
	// held was held (zero is now).
	Now time.Time
	// Reviews says where each capsule's latest review stands, a line each.
	Reviews []string
	// Process is the workspace's process, as it's written: how work goes
	// here, whose turn each step is, and when it's the user's. The
	// orchestrator reads it to know what to do about what's happened.
	Process string
	// Checks are what a Work step's latest round of checks came to, a
	// line each (Trigger "checks"), Round its number and MaxRounds how
	// many come round before the step's the user's.
	Checks           []string
	Round, MaxRounds int
	// OutOfTime is the step whose time ran out ("2: Work: find it"), for
	// Trigger "limit".
	OutOfTime string
	// PREvents are what happened to the mission's pull requests since its
	// last run, a line each (Trigger "pr"), and PRStep the Open Pull
	// Request step the mission's at ("3 of 4"), if it is; Round is how
	// many times its pull request has come back for fixing.
	PREvents []string
	PRStep   string
	// HeldStep is the step that waits for its call ("2 of 4: Decide:
	// which fix?"), and HeldWhy why (Trigger "step").
	HeldStep, HeldWhy string
	// Guarantees are what ws holds for every mission where it acts,
	// whatever an agent reads (ws.toml's [missions]): what each does, in
	// words.
	Guarantees []string
}

// Capsule is one of the mission's capsules, with what's known about it.
type Capsule struct {
	Name     string // repo/name
	Path     string
	Exists   bool
	Branch   string
	PR       int // 0 if none
	PRMerged bool
	Checks   string
	URL      string
	DiffStat string
	// Direct: its repo has no pull requests, and its commits land straight
	// on the default branch when the user says. Ahead is how many it has to
	// land; Landed says they have.
	Direct bool
	Ahead  int
	Landed bool
}

// Prompt is everything the orchestrator needs, gathered up front so it can
// usually decide without digging.
func Prompt(in Input) string {
	m := in.Mission
	var b strings.Builder
	fmt.Fprintf(&b, "You are the orchestrator for the mission %q (%s) in a multi-repo workspace managed by the ws tool. Crew (coding agents) do the work; you decide what happens between them.\n", m.Title, m.Slug)
	if in.Guide != "" {
		b.WriteString(in.Guide + " Crew are told the same. When what you dispatch is work a skill covers, name the skill in the dispatch.\n")
	}
	switch in.Trigger {
	case "done":
		fmt.Fprintf(&b, "Crew %s has just reported that its part is done.\n", in.CrewID)
	case "ended":
		fmt.Fprintf(&b, "Crew %s has just ended without reporting done.\n", in.CrewID)
	case "quiet":
		if c, _ := m.CrewByID(in.CrewID); c.Pending != nil {
			b.WriteString(heldTurn(in.CrewID, c.Pending, in.now()))
		} else {
			fmt.Fprintf(&b, "Crew %s has just finished a turn without reporting done or asking the user anything, after a message ws sent it (its opening prompt, or your last nudge). Its session is still open. Work out why it stopped from its last message and output: if it's finished, have it report done with a handoff (resume-with); if it's stuck or stopped early, resume-with what to do; if it put a question to the user in its reply, flag the user with that question rather than answering it yourself.\n", in.CrewID)
		}
		if r := strings.TrimSpace(in.LastReply); r != "" {
			fmt.Fprintf(&b, "\nIts last message:\n\n%s\n\n", clipHead(r, 4<<10, "\n…"))
		}
	}
	planned := m.Plan != nil
	noun, route := "stage", "brief"
	if planned {
		noun, route = "step", "plan"
	}
	if in.CrewStage != "" && (in.Trigger == "done" || in.Trigger == "ended" || in.Trigger == "quiet" || in.Trigger == "checks") {
		fmt.Fprintf(&b, "It was working on %s %s.\n", noun, in.CrewStage)
	}
	switch in.Trigger {
	case "review":
		fmt.Fprintf(&b, "The user has just reviewed a capsule's work and submitted the review below (%s). Act on it now: it's their judgement of what the crew did.\n\n", in.ReviewRef)
		b.WriteString("- Changes requested, or comments to act on: have them addressed. Prefer resuming the crew that did the work, with what to address (ws mission resume-with <crew> \"Address the review in " + in.ReviewRef + ": …\"), when its context matters; otherwise dispatch fresh crew into that capsule with --read " + in.ReviewRef + ".\n")
		fmt.Fprintf(&b, "- Approved: if the current %s's work is complete, accept it (with the review as the reason) and move on as the %s says; comments on an approval are optional polish.\n", noun, route)
		b.WriteString("- Never edit or answer the review yourself, and don't resolve its comments: crew address them, and the user resolves them.\n")
		if len(in.ReviewCrew) > 0 {
			fmt.Fprintf(&b, "Crew who worked in that capsule (latest last): %s.\n", strings.Join(in.ReviewCrew, ", "))
		} else {
			fmt.Fprintf(&b, "No crew has worked in that capsule yet (its commits were made outside the mission), so there's nobody to resume: dispatch fresh crew into it with --read %s.\n", in.ReviewRef)
		}
		fmt.Fprintf(&b, "\n%s\n", strings.TrimSpace(in.Review))
	case "plan":
		b.WriteString("The planner has revised the plan (below; the log's latest says what changed): look at where the mission stands on it now, and move it on.\n")
	case "restarted":
		b.WriteString("Your last run on this mission was cut short (ws's server stopped while it ran), so what prompted it wasn't acted on. Look at where the mission stands and do what that run would have: the log's latest events say what happened.\n")
	case "pr":
		b.WriteString("One of the mission's pull requests has changed on GitHub while no crew were at work in its capsule (what happened is below). What's done about it is the process's to say: read it for how pull requests are looked after here, and do that.\n")
		if in.PRStep != "" {
			fmt.Fprintf(&b, "The mission is at step %s, an Open Pull Request, ws's own: ws passes the step when it merges, and you don't accept it. Red checks or a conflict (round %d; after round %d it's the user's, whatever the process says): resume the crew who worked in its capsule with what failed (gh pr checks and gh pr view say), or flag the user, as the process says. A review or a comment: resume that crew to fix what's asked, or to draft a reply (they run ws mission reply <repo>#<n> --thread <id> or --to <login>; a reply to a person waits for the user's approval before it's posted), or flag the user, as the process says. A draft whose checks pass: mark it ready (ws mission pr-ready), or flag the user to review it first, as the process says. Closed without merging: flag the user.\n", in.PRStep, in.Round, in.MaxRounds)
		} else if planned {
			b.WriteString("Checks failing, a conflict, or a reviewer asking for changes: have it fixed as the process says (resume the crew that worked in that capsule with what to fix, else dispatch fresh crew there; gh pr checks and gh pr view say what failed and what was asked), or flag the user. Merged: move on along the plan: if the current step is CI's or outside's and the merge is what it waits for, accept it; or once the mission is at Z, suggest how it ends, with Z checked.\n")
		} else {
			b.WriteString("Checks failing, a conflict, or a reviewer asking for changes: have it fixed as the process says (resume the crew that worked in that capsule with what to fix, else dispatch fresh crew there; gh pr checks and gh pr view say what failed and what was asked), or flag the user. Merged: move on as the brief says: accept the stage if its work is complete, then the next stage, or suggest how it ends, with Z checked, once everything the brief asks for has merged.\n")
		}
		if len(in.PREvents) > 0 {
			b.WriteString("\nWhat happened, since your last run:\n\n")
			for _, e := range in.PREvents {
				b.WriteString("- " + e + "\n")
			}
			b.WriteString("\n")
		}
	case "checks":
		fmt.Fprintf(&b, "Crew %s reported its step done, and ws ran and had judged the step's checks: not all are met (round %d; after round %d it's the user's, whatever the process says). Decide what follows by the process: send the step back to %s with what's unmet and what showed it (ws mission resume-with), or, where the process says it's the user's, flag them with what's unmet and the choices. Don't accept the step: ws passes it once its checks are all met, running them again when crew report done.\n", in.CrewID, in.Round, in.MaxRounds, in.CrewID)
		if len(in.Checks) > 0 {
			b.WriteString("\nWhat the checks came to:\n\n")
			for _, c := range in.Checks {
				b.WriteString("- " + c + "\n")
			}
			b.WriteString("\n")
		}
	case "step":
		fmt.Fprintf(&b, "The mission has reached step %s, which waits for your call: %s. Decide from what the steps before made, the plan's words and the process: start it (ws mission start, with --in <repo> for each repo it runs in), or skip it, saying why (ws mission skip \"…\"), where its only if doesn't hold. Where you can't tell, flag the user.\n", in.HeldStep, in.HeldWhy)
	case "limit":
		fmt.Fprintf(&b, "Step %s is out of time: its limit ran out while crew were still at it. Decide what follows by the process: more time (ws mission limit), have them wrap up and report done with what they have (ws mission resume-with <crew> \"Your time is up: …\"), or, where the process says it's the user's, flag them with those choices.\n", in.OutOfTime)
	case "answer":
		if a := m.Answer; a != nil {
			who := "They answered"
			if a.By != "" && a.By != "you" {
				who = "Crew " + a.By + ", which the user asked to answer it for them, answered"
			}
			if a.Step > 0 && a.Verb == mission.TaskReview {
				fmt.Fprintf(&b, "Step %d is the user's, to review, and they've said what's to change in what the steps before it made (it stays theirs, to review again).\n\nws asked them: %s\nThey said: %s\n\nAct on it now, as on a review asking for changes: send it back to the crew who made it (resume them with ws mission resume-with <crew> \"…\", or dispatch fresh crew into that capsule), saying what's to change.\n", a.Step, a.Question, a.Text)
				break
			}
			fmt.Fprintf(&b, "You flagged the user, and they've answered.\n\nYou asked: %s\n%s: %s\n\nAct on the answer now: it's the decision you were waiting for.\n", a.Question, who, a.Text)
		}
	}
	if planned {
		if slices.ContainsFunc(m.Plan.Steps, func(s mission.Step) bool { return s.Type == mission.StepVerify }) {
			// Said only of a plan that has one: the rest is said for every plan.
			b.WriteString("\nThe plan has a Verify step, which isn't yours to accept. ws starts a verifier at it, crew who didn't do the work, and passes the step on the bundle they hand over once nothing in it is unmet. Unmet, ws sends the work back to the crew who made it, and has it verified again when they report done; past its rounds, it's the user's. The bundle is the user's to approve, at a Review.\n")
		}
		b.WriteString(planIntro)
		b.WriteString(planCommands)
	} else {
		b.WriteString(briefIntro)
		b.WriteString(briefCommands)
	}
	b.WriteString(crewAndReviews)
	if !planned {
		b.WriteString(briefDrafts)
	}
	b.WriteString(investigating)
	// What it notes, flags and dispatches is read in the web UI.
	b.WriteString(prompt.Naming)
	b.WriteString("Start with `ws show -f llm`: the mission's brief and its stages' progress, its capsules and the repos' default branches, its artifacts, its crew (what each was dispatched with, their handoffs), open threads on its documents, its decisions and the recent log. `ws show -f llm <ref>` shows any of those in full, with what it links to. What's below is what that doesn't say: what just happened, and what's live in the capsules.\n\n")
	if m.TitleIsRequest() {
		b.WriteString("The mission's title is still the request it was raised with: give it a name with ws mission title first.\n\n")
	}

	if planned {
		b.WriteString(planSection(m.Plan, in.Stages))
	} else {
		if len(in.Stages) > 0 {
			b.WriteString("## Stage progress\n\nThe brief's stages, with ws's record of each (the current one is the first not accepted):\n\n")
			for _, l := range in.Stages {
				b.WriteString("- " + l + "\n")
			}
			b.WriteString("\n")
		}
		if z := mission.BriefAcceptance(in.Brief); len(z) > 0 {
			b.WriteString("## Z\n\nThe brief's Acceptance, part by part: what has to be true for the mission to end at Z. When it ends, each is checked, met or not, by its number (ws mission conclude --met/--unmet).\n\n")
			for i, part := range z {
				fmt.Fprintf(&b, "%d. %s\n", i+1, part)
			}
			b.WriteString("\n")
		}
	}
	b.WriteString(processSection(in.Process, in.Guarantees))
	if len(m.EndedCapsules) > 0 {
		b.WriteString("## Capsules that ended alone\n\nNo longer the mission's, and they don't hold it open:\n\n")
		for _, e := range m.EndedCapsules {
			b.WriteString("- " + e.Words() + "\n")
		}
		b.WriteString("\n")
	}
	if c := m.Conclude; c != nil && c.Ending != "" {
		fmt.Fprintf(&b, "## Ending suggested\n\n%s, waiting for the user to end it (or not).\n\n", c.Summary())
	}

	if len(in.Decisions) > 0 {
		b.WriteString("## Decisions\n\nWhat's been decided on this mission and why, newest first (ws show d<n> shows one, with what it's about). Don't dispatch work that goes against one: if one seems wrong now, flag the user.\n\n")
		for _, l := range in.Decisions {
			b.WriteString("- " + l + "\n")
		}
		b.WriteString("\n")
	}

	if in.Standing != "" {
		fmt.Fprintf(&b, "## Where it stands\n\nIf this run does nothing: %s\n\n", in.Standing)
	}
	if len(m.Tasks) > 0 {
		b.WriteString("## Waiting on the user\n\nWhat the mission waits on the user for, oldest first: each is theirs until they answer it. Don't ask them what's asked already: asking again (ws mission flag) takes the place of your own open question.\n\n")
		for _, t := range m.Tasks {
			what := t.Verb
			if t.Step > 0 {
				what += fmt.Sprintf(", step %d", t.Step)
			}
			by := t.By
			if by == "" {
				by = "ws"
			}
			fmt.Fprintf(&b, "- task %d (%s, from %s): %s\n", t.N, what, by, strings.Join(strings.Fields(t.Text), " "))
		}
		b.WriteString("\n")
	}

	if len(in.Reviews) > 0 {
		b.WriteString("## Reviews\n\nThe user's latest review of each capsule (ws review show <id> prints one):\n\n")
		for _, l := range in.Reviews {
			b.WriteString("- " + l + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Capsules\n\n")
	if len(in.Capsules) == 0 {
		b.WriteString("(none)\n")
	}
	var caps strings.Builder
	for _, c := range in.Capsules {
		fmt.Fprintf(&caps, "- %s", c.Name)
		if c.Exists {
			fmt.Fprintf(&caps, " at %s (branch %s)", c.Path, c.Branch)
		} else {
			caps.WriteString(" (burned)")
		}
		if c.PR != 0 {
			state := "open"
			if c.PRMerged {
				state = "merged"
			}
			fmt.Fprintf(&caps, " — PR #%d %s", c.PR, state)
			if c.Checks != "" {
				fmt.Fprintf(&caps, ", checks %s", c.Checks)
			}
			if c.URL != "" {
				fmt.Fprintf(&caps, " %s", c.URL)
			}
		}
		switch {
		case c.Landed:
			caps.WriteString(" — landed on the default branch (its repo has no pull requests): done, as a merged PR is")
		case c.Direct && c.Exists:
			fmt.Fprintf(&caps, " — no pull request: its repo takes commits straight to the default branch, and the user lands them (%d to land). Never have crew push or open a PR there: once the work's committed and done, it's the user's to review and land", c.Ahead)
		}
		caps.WriteByte('\n')
		if c.DiffStat != "" {
			for _, line := range strings.Split(c.DiffStat, "\n") {
				caps.WriteString("    " + line + "\n")
			}
		}
	}
	b.WriteString(clipHead(caps.String(), promptCapsulesMax, "… (more: ws mission show -f llm, and git diff --stat in each capsule)\n"))

	if len(in.Log) > 0 {
		log := in.Log
		if len(log) > promptLeanLog {
			log = log[len(log)-promptLeanLog:] // what just happened: the rest is in ws show
		}
		b.WriteString("\n## Recent log\n\n")
		b.WriteString(clipTail(strings.Join(log, "\n")+"\n", promptLogMax, "… (earlier: ws mission show -f llm)\n"))
	}

	tail := promptTranscriptMax
	if h := strings.TrimSpace(in.Handoff); in.CrewID != "" && h != "" {
		fmt.Fprintf(&b, "\n## %s's handoff\n\nWhat %s says it did, learned and left (kept as %s, which new crew are given):\n\n%s\n", in.CrewID, in.CrewID, mission.HandoffRef(in.CrewID), clipHead(h, promptHandoffMax, "\n… (the rest: read the file)"))
		tail = promptTailMax
	}
	if in.CrewID != "" && in.Output != "" {
		fmt.Fprintf(&b, "\n## The end of %s's output\n\n```\n%s\n```\n", in.CrewID, clipTail(in.Output, tail, "…\n"))
	}
	return clipHead(b.String(), promptMax, "\n… (cut to fit)")
}

// briefIntro is what the orchestrator's told of moving a mission along
// its brief's stages, for a mission without a plan.
const briefIntro = `
Read the brief and the history (below, and in ws show -f llm) and decide what should happen next: is what just finished good, what does the brief still need, and who should do it? The brief may list stages for separate crew (say, a proposal and then its implementation); Stage progress below says which are accepted and which is current. When the current stage's work is complete and good enough to build on, accept it: ws then starts the next stage itself (in the capsule it names, with its text), or asks the user first where the stage says to check in. Don't dispatch the next stage yourself, unless ws says it couldn't start it. Act only through these commands (WS_MISSION is set for you):
`

// planIntro is what it's told of moving a mission along its plan, in
// place of the brief's stages.
const planIntro = `
Move the mission along its plan (below): its route from A to Z, which the planner wrote and the user confirmed. You never change it. Its current step is the first not accepted: when what just happened has done it well enough to build on, accept it, and ws reaches the next by its kind. Work: it starts crew. Review, Decide and Land: it asks the user, and only they pass them. Plan: the planner writes the steps after it, and ws passes it. Wait: its step says who passes it. A Work step with checks is ws's to pass, once they're all met: ws checks when its crew report done, and brings you what's unmet. A step with a limit has a clock, and ws tells you if it runs out. A step with only if, or in each, waits at its start for your call. Open Pull Request: ws opens the pull request itself, and passes the step when it merges; while it's open, what happens to it comes to you, to act on as the process says. In a plan written before steps had kinds, a crew step that makes a pull request has its crew open it, through ws. ws holds the route, refusing what isn't yours, saying why. If the plan no longer fits what's happened (a step can't be done as it's written, a step found something the plan didn't foresee, or Z is wrong), don't work around it: ask the planner (ws mission replan), saying what doesn't fit. It revises what's ahead, or says why not; once at a step, after which it's the user's, so flag them. When every step is accepted, the mission is at Z if what Z says is true: check each part against what the steps made, and suggest how it ends; where the plan says it may stop early and that's what's happened, suggest that ending, with the parts of Z it didn't reach unmet. End each run with the mission moved on, or a note of why it waits: where it stands (below) is what the user sees if you only note. Act only through these commands (WS_MISSION is set for you):
`

// briefCommands are the orchestrator's commands, and what it leaves the
// mission as, for a mission without a plan: its brief's stages, which it
// moves along by these rules.
const briefCommands = `
- ws mission note "Orchestrator: <what you found and decided, two or three sentences>"   (always, once; the next orchestrator run reads it)
- ws mission decide "<what was decided>" --why "<why>" [--about <ref>]   (when you settle something about the mission's course that whoever comes next should know and not undo: an approach chosen, an option ruled out. Not the user's choices of scope or direction: flag those. Decisions below that stand are to be respected)
- ws mission dispatch "<what to do>" [--capsule <repo>/<name>] [--read <artifact>]...   (start new, fresh crew on the next piece of work: it joins the current stage (or step), or the one named with --stage <n>. Beyond the brief it knows only what you dispatch it with and where earlier crew left off, which ws gives it (the latest handoff from its capsule, or the mission's). Say what to do, where earlier crew's output is (files, commits, PRs) and what to watch for. Name each artifact it should read first with --read and its reference, as the mission's artifacts list it (ws show -f llm), handoffs included (mission:handoffs/<crew>.md), rather than retelling it; ws adds where to read it)
- ws mission accept "<why it's complete and good>"   (the current stage, or the plan's current step, is done: ws starts the next, or checks in with the user, and says which. Refused while its crew still work or ask the user something, and at a step that's the user's or the planner's. A brief without stages is one stage)
- ws mission resume-with <crew> "<what's left>"   (that crew should carry on: unfinished, tests missing, CI failing. Better than dispatching when its context matters. Crew still running get it typed straight in; this counts toward your dispatch limit)
- ws mission flag "<question for the user>" --option "<answer>" --option "<answer>"   (one question at a time: asking again takes the place of your open one. A decision or problem only the user can resolve, or the brief says to check with them before the next stage. Ask one clear question and give the two or three answers you'd act on, each a short instruction like "Commit both and open PRs": the user answers with one key, or writes their own, and you're run again with their answer. Give what they need to decide in the question itself: paths, what you checked)
- ws mission new "<follow-up, what and why>"   (separate work worth doing, outside this mission's scope)
- ws mission conclude "<what it came to>" --as <landed|answered|handed-on|dropped> --met "<n>: <how>" --unmet "<n>: <why not>"   (suggest how the mission ends, for the user to end it: once it's at Z, or where it may stop early and has. Check every part of Z, by its number below: --met with what shows it, or --unmet with why not; ws refuses one that doesn't. landed: at Z, its code merged or landed. answered: a finding and no code, at Z or stopping early. handed-on: its work raised as other missions, each --to <mission>. dropped: given up. The user ends it, or not)
- ws mission hand-on <repo>/<capsule> "<what the mission it goes to is for>"   (a capsule whose work belongs in a mission of its own, as a plan may say of a repo that fails: one is raised for it, to be briefed, and takes the capsule, which no longer holds this mission open)
- ws mission wait "<what it waits on>"   (nothing should happen until something outside the mission happens that ws can't see: "the platform team's deploy of the token service". Not for what ws already shows, like a PR waiting for review or crew at work, and not for something the user must do: flag them)
- ws mission title "<terse name>"   (at most 50 characters, plain English; only when the title below is still the raw request, or no longer says what the mission is)

Leave the mission in someone's court: every run ends with crew carrying on (dispatch, resume-with), the user asked (flag), a stage or step accepted and the next one started (or the user asked, at a check-in or a checkpoint), a suggested conclusion, or ws mission wait. Where it stands, below, is what the user sees if you do nothing else: when that's already theirs for something ws shows them (crew asking, a re-review, a PR to merge), a note is enough. A run that only notes otherwise leaves the user a mission nobody's working on, with your note.
`

// planCommands are its commands for a mission with a plan: shorter, since
// ws holds the route (who's on each step, which step is current, which
// steps only the user passes) and refuses what isn't the orchestrator's.
const planCommands = `
- ws mission note "Orchestrator: <what you found and decided, two or three sentences>"   (always, once; the next orchestrator run reads it)
- ws mission accept "<why it's done and good>"   (the current step: ws reaches the next by its kind, and says what it did. Where the step isn't yours to pass, ws refuses it and says why)
- ws mission resume-with <crew> "<what's left>"   (that crew carries on: unfinished, tests missing, CI failing, its step's checks unmet, its step's pull request to open. Better than dispatching when its context matters)
- ws mission limit <45m|1h>   (more time for the step, from now)
- ws mission pr-ready [<repo>]   (an Open Pull Request step's draft, its checks green, marked ready, as the process says)
- ws mission start [--in <repo>]...   (the step waiting for your call: one that applies only if, or runs in each of the repos its words name)
- ws mission skip "<why>"   (that step, its only if not holding)
- ws mission repeat [--in <repo>] [--on <harness:model>] "<why>"   (the Work step again, fresh crew, when resuming won't do; max_rounds caps it. --on: one [crew] allowed lists)
- ws mission dispatch "<what to do>" [--capsule <repo>/<name>] [--on <harness:model>] [--read <artifact>]...   (fresh crew on the current Work step, nobody on it: ws couldn't start it. Say what to do and where earlier work is; name what to read first with --read (handoffs are mission:handoffs/<crew>.md))
- ws mission replan "<what doesn't fit, and what happened>"   (the plan no longer fits what's happened: the planner revises what's ahead, and ws runs you again once it has. Once at a step)
- ws mission flag "<question for the user>" --option "<answer>" --option "<answer>"   (one question at a time: asking again takes the place of your open one. What only the user can settle, and the plan still not fitting once the planner's had it: one clear question with what they need to decide in it, and the two or three answers you'd act on, each a short instruction)
- ws mission wait "<what it waits on>"   (the step waits on something ws can't see happen: a deploy, a teammate elsewhere)
- ws mission conclude "<what it came to>" --as <landed|answered|handed-on|dropped> --met "<n>: <how>" --unmet "<n>: <why not>" [--to <mission>]   (at Z, or stopped early where the plan says it may: suggest how it ends, every part of Z checked by its number below. landed: its code merged or landed; answered: a finding and no code; handed-on: its work raised as other missions; dropped: given up. The user ends it)
- ws mission hand-on <repo>/<capsule> "<what the mission it goes to is for>"   (a capsule whose work belongs in a mission of its own, as a plan may say of a repo that fails)
- ws mission decide "<what was decided>" --why "<why>" [--about <ref>]   (something settled about the mission's course that whoever comes next should know and not undo; not the user's choices of scope: flag those)
- ws mission new "<follow-up, what and why>"   (work worth doing outside this mission)
- ws mission title "<terse name>"   (at most 50 characters; only when the title below is still the raw request)

`

// crewAndReviews is what it's told of crew waiting on the user, and of
// reviews asking for changes.
const crewAndReviews = `If crew asked the user something they haven't answered yet (crew_asked in the log, after that crew last started), the user answers the crew directly: don't flag the same question, just note that you're waiting for their answer.
When a capsule's latest review asked for changes, crew address it (resumed, or dispatched with --read and the review), answering each comment with ws review addressed/wontfix/reply. Once they've committed and reported done, it's the user's turn: they re-review, and ws mission accept is refused until they approve, so don't try to accept, and don't flag them (ws shows them it's ready): note that it's waiting for their re-review.
`

// briefDrafts is what it's told of building on documents not yet landed,
// for a mission without a plan: a plan's checkpoints say where to wait.
const briefDrafts = `An artifact still a draft or in review (its PR open) can be built on: the next crew reads it from its capsule. Only wait for it to land if the brief says to check in first; then flag instead.
`

// investigating is what it may look at, and how it runs commands.
const investigating = `If the next crew needs a capsule the mission doesn't have yet, create it first with ws lift <repo> <branch> (it joins the mission), then dispatch with --capsule.
You may investigate read-only: ws show -f llm [ref] (the mission, or any stage s2, crew c1, capsule, artifact, review r1 or orchestrator run in it, with what it links to: crew's handoffs, what they made, a document's history), ws review show <id>, git -C <path> diff/log/show/status for the capsule paths and default-branch (.ground) paths below, spelled exactly as listed (repos/<repo>/ itself isn't a repository), gh pr view/checks, and reading files. Don't change code, commit or push yourself. Prefer one clear next step; don't invent work, and if nothing is needed, just note why. If a command fails, read its message and correct it (a refused dispatch means flag the user instead). Finish with one short paragraph saying what you did.

Run one command per shell call rather than chaining them with && or ;. You may only be allowed the commands above: one outside them is refused, whatever words the refusal uses ("Permission to use Bash has been denied", "rejected", "Refused"). That refusal covers that one command alone, not the shell: carry on with the commands above. Always end by running your ws mission commands; never stop at saying what you would run.

`

// processSection is the workspace's process, as the orchestrator reads
// it, and what ws holds itself. "" with neither.
func processSection(text string, guarantees []string) string {
	var b strings.Builder
	if strings.TrimSpace(text) != "" {
		b.WriteString("## The process\n\nHow work goes in this workspace, in the team's words: the steps work takes, whose turn each is, and when it's the user's. Before you accept, dispatch or suggest how it ends, check that what you're doing fits it, and when what's happened goes against it, flag the user rather than going on.\n\n")
		b.WriteString(clipHead("````markdown\n"+strings.TrimRight(text, "\n")+"\n````\n", promptProcessMax, "… (the rest: ws show process)\n````\n"))
		b.WriteString("\n")
	}
	if len(guarantees) > 0 {
		b.WriteString("## What ws holds\n\nWhatever anyone reads, ws refuses otherwise, where it acts:\n\n")
		for _, g := range guarantees {
			b.WriteString("- " + strings.TrimRight(g, ".") + ".\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// planSection is the plan as the orchestrator's given it: Z, where it
// may stop early, and its steps with ws's record of each.
func planSection(p *mission.Plan, steps []string) string {
	var b strings.Builder
	b.WriteString("## The plan\n\nZ, where the mission ends: each has to be true. When it ends, each is checked, met or not, by its number (ws mission conclude --met/--unmet).\n\n")
	for i, z := range p.Z {
		fmt.Fprintf(&b, "%d. %s\n", i+1, z)
	}
	if len(p.Early) > 0 {
		b.WriteString("\nWhere it may stop early, and how that ends:\n\n")
		for _, e := range p.Early {
			b.WriteString("- " + e + "\n")
		}
	}
	b.WriteString("\nIts steps, each with its kind, who's on it and ws's record of it (the current one is the first not accepted; ws show s<n> shows one):\n\n")
	for _, l := range steps {
		b.WriteString("- " + l + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// clipHead keeps the start of s within limit bytes, at a line break where one
// is near, never splitting a character, and appends more if anything went.
func clipHead(s string, limit int, more string) string {
	if len(s) <= limit {
		return s
	}
	cut := max(0, limit-len(more))
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	head := s[:cut]
	if i := strings.LastIndexByte(head, '\n'); i > len(head)/2 {
		head = head[:i+1]
	}
	return head + more
}

// clipTail keeps the end of s within limit bytes, from a line break where one
// is near, never splitting a character, and prepends more if anything went.
func clipTail(s string, limit int, more string) string {
	if len(s) <= limit {
		return s
	}
	cut := len(s) - limit + len(more)
	for cut < len(s) && !utf8.RuneStart(s[cut]) {
		cut++
	}
	tail := s[cut:]
	if i := strings.IndexByte(tail, '\n'); i >= 0 && i < len(tail)/2 {
		tail = tail[i+1:]
	}
	return more + tail
}
