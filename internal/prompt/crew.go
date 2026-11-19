// Package prompt is what agents are told: pure functions from what ws knows
// to text, with no I/O. A prompt that asks for a form of reply sits beside
// the parser of that form. Each is pinned by a golden file in testdata (go
// test ./internal/prompt -update rewrites them), so a change to what an
// agent's told shows in a diff.
package prompt

import (
	"fmt"
	"strings"

	"github.com/withworkspace/workspace/internal/mission"
)

// CrewBrief is what crew are told about the mission they start on.
type CrewBrief struct {
	Slug  string
	Title string
	Brief string // brief.md
	// Role is "" for crew working on the mission, mission.RoleBriefing for
	// crew turning a request into a brief and capsules, or
	// mission.RoleAnswer for crew answering the orchestrator's question.
	Role    string
	Request string // what the user asked for (briefing)
	// Conversation is the conversation the mission came out of, whose
	// summary and turns the briefing starts from.
	Conversation string
	// Question and Options are the orchestrator's question, and the answers
	// it suggested (answer).
	Question string
	Options  []string
	// Guide is what's told of the workspace's own way of working.
	Guide string
	// Dispatch is what this crew was dispatched to do.
	Dispatch string
	// Stage is the stage of the brief's plan this crew is for, StageN its
	// number and StageOf how many there are; "" when the brief has none.
	// Steps: they're the steps of the mission's plan (plan.md) instead.
	Stage           string
	StageN, StageOf int
	Steps           bool
	// OpensPR: the step makes a pull request, which its crew open through
	// ws (ws mission open-pr). Crew at a plan's other steps don't open one.
	OpensPR bool
	// LeftOff is where earlier crew left off.
	LeftOff string
	// Planning is the process the planner works from (briefing).
	Planning Planning
	Org      string // the GitHub org the workspace's repos live in
}

// Crew is the first message given to new crew. It points the agent at
// the mission rather than pasting everything, so the brief stays the single
// source of truth, but includes the brief too so the agent can start at once.
func Crew(b CrewBrief) string {
	switch b.Role {
	case mission.RoleBriefing:
		return briefingPrompt(b)
	case mission.RoleAnswer:
		return answerPrompt(b)
	}
	var s strings.Builder
	fmt.Fprintf(&s, "You are crew on the mission %q (%s), managed by the ws workspace tool.\n\n", b.Title, b.Slug)
	route, ref := "stages", "a stage `s2`"
	if b.Steps {
		route, ref = "plan", "a step `s2`"
	}
	fmt.Fprintf(&s, "Before starting, run `ws show -f llm` to see the mission: its capsules (worktrees, possibly across repos), %s, crew, artifacts, reviews and log. ", route)
	fmt.Fprintf(&s, "`ws show -f llm <ref>` shows any one thing in it (%s, crew `c1`, a capsule `repo/name`, an artifact by its ref, a review `r1`) with what it links to: follow the links you need", ref)
	if b.StageN > 0 {
		noun := "stage"
		if b.Steps {
			noun = "step"
		}
		fmt.Fprintf(&s, ", starting with `ws show -f llm s%d`, your %s: who worked on it before you, and what they made for it", b.StageN, noun)
	}
	s.WriteString(". Before you change a file, `ws show -f llm <repo>/<path>` (with `:<lines>` for some of it) says what's been said of it in any mission: review comments still open on it, decisions about it, talk that named it. `ws find -f llm <words>` finds what's been said by what it says, in any mission (decisions and why, briefs, handoffs, threads, review comments) and in the workspace's documents, each with the command that shows it. ")
	s.WriteString("Work only in this mission's capsules; create new ones with `ws lift <repo> <name>` and they join the mission automatically. ")
	if b.Guide != "" {
		s.WriteString(b.Guide + " ")
	}
	s.WriteString("Record progress with `ws mission note \"...\"`, and decisions you make within your part (an approach taken, an option ruled out) with `ws mission decide \"<what>\" --why \"<why>\"`, so whoever comes next knows them. `ws show -f llm` lists the decisions already made: don't go against one without asking the user. Don't edit the brief.\n\n")
	if st := strings.TrimSpace(b.Stage); st != "" && b.Steps {
		fmt.Fprintf(&s, "The mission's plan (`ws show -f llm` has it) has %d steps, each done in turn by whoever's on it: crew, the user, CI or someone outside. You're on step %d:\n\n%s\n\n", b.StageOf, b.StageN, st)
	} else if st != "" {
		fmt.Fprintf(&s, "The brief's plan has %d stages, each done by separate crew in turn. You're on stage %d:\n\n%s\n\n", b.StageOf, b.StageN, st)
	}
	switch d := strings.TrimSpace(b.Dispatch); {
	case d != "":
		s.WriteString("You were dispatched for one part of this mission:\n\n")
		s.WriteString(d)
		s.WriteString("\n\nDo that part, then report done; ws's orchestrator decides what follows and dispatches it.\n\n")
	case b.Stage != "" && b.Steps:
		s.WriteString("Do that step only, then report done; ws's orchestrator decides what follows.\n\n")
	case b.Stage != "":
		s.WriteString("Do that stage only, then report done; ws's orchestrator decides what follows and dispatches it.\n\n")
	default:
		s.WriteString("If the brief describes the work in stages for separate crew, do only the first stage not yet done, then report done; ws's orchestrator dispatches the next.\n\n")
	}
	if l := strings.TrimSpace(b.LeftOff); l != "" {
		s.WriteString(l + "\n\n")
	}
	s.WriteString("What you write the user reads in ws's web UI (notes, questions, handoffs, replies to reviews and threads, documents). " + Naming)
	s.WriteString("If you notice something worth doing outside this mission, don't do it: run `ws mission new \"<what it is and why>\"` (plain text; it waits for the user to brief it) and mention it in one line.\n\n")
	s.WriteString("Tell ws where you are, so the user knows when you're waiting on them:\n")
	s.WriteString("- When you need a decision from the user, ask in your reply and also run `ws mission ask \"<the question>\"`, then wait for their answer: don't report done while a question is open.\n")
	s.WriteString("- When you make something the next crew or the user will need (a proposal, an ADR, notes, a doc), record it as you go: `ws mission artifact add <file> --kind <proposal|adr|note|doc> \"<label>\"` (a file in a capsule, in a .ground, or in the mission's private files folder; or a URL). `ws show -f llm` lists the mission's artifacts with where to read each, the private files folder, and the records repo for shared proposals and ADRs, if the workspace has one.\n")
	s.WriteString("- When you check your work by using it (a page in a browser, light and dark; what a command prints; a terminal's screen), keep what you saw (a screenshot, a short recording as .webm or .mp4, a command's output as text) and, once your work is committed, hand it over as a verification bundle: `ws mission bundle --set - <<'EOF'`, the bundle, then `EOF`. It says, for each of your step's checks, what it came to and what shows it, how the user can try it themselves, and what you didn't verify: `ws mission bundle --format` says how it's written, with your step's checks by number. Keep it short, since ws refuses one that's over length: the outcome first, each check's finding in a sentence or two, and what you measured in a text file the check shows. Write it in short, plain sentences, one fact in each, with no idioms. The mission's Verify tab shows it, images and videos as they are, so the user sees what you saw. If you change the work afterwards, hand over another: it takes the first one's place.\n")
	s.WriteString("- When your work needs something left running (a dev server to look at, a stack to test against), start it with `ws run`, never in the background: `ws run <recipe>` runs one of the repo's recipes (`ws run` alone lists them), and `ws run -- <command> --port {port}` runs anything else. ws gives it a free port, waits until it's up, and prints its link. It's ended once your step passes, so leave it running for whoever looks next. `ws run logs <name>` prints its output.\n")
	s.WriteString("- When the work calls for something to look at and use rather than read (a mock-up of a page, two takes on a design, a diagram to click through), make it an artifact: one HTML file with its CSS and JS inline, added with `ws artifact add <file> --title \"…\" --note \"<what it tries out>\"`, and its next version with `ws artifact add <file> --to a3 --note \"<what changed, and why>\"`. Where there's something to choose between (a density, a colour, two takes on a layout), don't draw controls for it in the page: declare it as tweaks, and ws draws them beside the page, with your presets as buttons (`ws artifact tweaks` says how). ws records it on the mission. Where the workspace keeps artifacts in a repo, it's written in your capsule of that repo (lift one if you have none): commit it with your work, and it lands with the capsule. Name it in your handoff (`a3`, `a3.v2`), and `ws show a3` says what each version is. Promoting, discarding and restoring one are the user's: say in your handoff which you'd keep, and which drop, and why.\n")
	s.WriteString("- If the user has reviewed this work, `ws show -f llm` lists the review, and `ws review show -f llm` prints the latest: each comment with the lines it's on, and what to do. Address it as it says, and answer each comment by its id once you've committed: `ws review addressed <id> \"what you did\"` (it records the commit), `ws review wontfix <id> \"why\"`, or `ws review reply <id> \"your answer\"` for a question. The user re-reviews what changed and resolves them.\n")
	s.WriteString("- If the user has commented on a document (a proposal, a plan), `ws show -f llm` lists the open threads: `ws show -f llm t<n>` shows one, with the text it's on and the markdown lines that text is in. Answer each with `ws thread reply t<n> \"…\"`: answer a question; for a change, change the document (and commit, if it's in a capsule) and say what you changed with `ws thread changed t<n> \"…\"`, which hands it back to the user, or say why not. The user resolves them.\n")
	switch {
	case b.Steps && b.OpensPR:
		s.WriteString("- When your part is finished, commit it, then open its pull request through ws, which pushes the branch and opens it, holding the process's rules about pull requests: `ws mission open-pr [<repo>/<capsule>] --title \"<title>\" --body - <<'EOF'`, its description, then `EOF`. If it's refused, it says why: when it's for want of the user's review, ask them to review it (`ws mission ask`) and wait. Don't push or open one any other way (no `gh pr create`). Then report done with a one-line summary and a handoff for whoever comes next, whether the next crew or the user. Learned matters most: where things live, what you tried that didn't work, surprises. Keep it short.\n\n")
	case b.Steps:
		s.WriteString("- When your part is finished, commit it (the user reviews commits, and the next crew builds on them: uncommitted work is left out). Don't push or open a pull request: the plan's step that makes one opens it, through ws. Then report done with a one-line summary and a handoff for whoever comes next, whether the next crew or the user. Learned matters most: where things live, what you tried that didn't work, surprises. Keep it short.\n\n")
	default:
		s.WriteString("- When your part is finished, commit it (the user reviews commits, and the next crew builds on them: uncommitted work is left out), and open PRs if that's how this repo works. Then report done with a one-line summary and a handoff for whoever comes next, whether the next crew or the user. Learned matters most: where things live, what you tried that didn't work, surprises. Keep it short. ws finds PRs from the mission's capsules itself.\n\n")
	}
	s.WriteString("  ws mission done \"<one-line summary>\" --handoff - <<'EOF'\n")
	for _, l := range strings.Split(mission.HandoffTemplate, "\n") {
		s.WriteString("  " + l + "\n")
	}
	s.WriteString("  EOF\n")
	if brief := strings.TrimSpace(b.Brief); brief != "" {
		s.WriteString("\nThe brief:\n\n")
		s.WriteString(brief)
		s.WriteString("\n")
	}
	return s.String()
}

// answerPrompt sets up crew the user asked to answer the orchestrator's
// question for them: find out, answer with ws mission answer, and stop
// there. It doesn't paste the brief: the question is the job, and the
// mission is one ws mission show away.
func answerPrompt(b CrewBrief) string {
	var s strings.Builder
	fmt.Fprintf(&s, "You are crew on the mission %q (%s), managed by the ws workspace tool. ", b.Title, b.Slug)
	s.WriteString("The mission's orchestrator (a headless agent that decides what happens next and dispatches crew) asked the user a question, and the user asked you to answer it for them.\n\n")
	fmt.Fprintf(&s, "The question:\n\n%s\n", strings.TrimSpace(b.Question))
	if len(b.Options) > 0 {
		s.WriteString("\nThe answers it suggested:\n")
		for i, o := range b.Options {
			fmt.Fprintf(&s, "%d. %s\n", i+1, o)
		}
	}
	if b.Guide != "" {
		s.WriteString("\n" + b.Guide + "\n")
	}
	s.WriteString("\nFind out what you need to answer it well. `ws show -f llm` gives the brief, the capsules (with their paths), the artifacts and the recent log, and `ws show -f llm <ref>` any one thing in it with what it links to; look at the capsules, read files, run commands or tests (things the orchestrator can't do itself). ")
	s.WriteString("If the answer is really the user's call (a preference, or a risk they should weigh), ask them in your reply and wait.\n\n")
	s.WriteString("Then answer:\n\n  ws mission answer \"<the answer>\"\n\n")
	s.WriteString("Make the answer what should happen next, as an instruction (usually one of the suggested answers), followed by what you found that supports it, in a sentence or two. It goes to the orchestrator, which acts on it. ")
	s.WriteString("Don't do the work the answer leads to (committing, opening PRs, implementing): the orchestrator dispatches that. Once you've answered, your part is done.\n\n")
	s.WriteString(Naming)
	return s.String()
}

// briefingPrompt sets up crew whose only job is to turn a request into a
// ready mission (brief and capsules) and then hand over.
func briefingPrompt(b CrewBrief) string {
	var s strings.Builder
	s.WriteString("You are briefing a new mission in a multi-repo workspace managed by the ws tool. ")
	s.WriteString("You are NOT doing the work: another agent will, starting from what you set up. ")
	s.WriteString("Your job is to turn the user's request into a ready mission, then stop.\n\n")
	fmt.Fprintf(&s, "The request (mission %s):\n\n%s\n\n", b.Slug, strings.TrimSpace(b.Request))
	if b.Conversation != "" {
		fmt.Fprintf(&s, "It came out of a conversation with the user, %s: `ws show -f llm %s` has its summary and turns. Start there: it knows what was explored and decided, and the mission's decisions (`ws show -m %s -f llm`) came from it.\n\n", b.Conversation, b.Conversation, b.Slug)
	}
	if b.Guide != "" {
		s.WriteString(b.Guide + " The brief you write should have the work done as the guide and its skills say.\n\n")
	}
	s.WriteString(Naming)
	s.WriteString("Steps:\n\n")
	s.WriteString("1. Understand the request. Run `ws status -f llm` to see the repos, their aliases and their worktrees. ")
	if b.Org != "" {
		fmt.Fprintf(&s, "Repos live on GitHub as %s/<repo>. ", b.Org)
	}
	s.WriteString("If the request names an issue or PR (\"fe issue 45\", \"api#12\", a URL), read it with `gh issue view` / `gh pr view` (use `--repo <org>/<repo>` and `--comments`). ")
	s.WriteString("Read code in a repo's `.ground` worktree (read-only) or with `ws rg`. Don't change any code.\n")
	s.WriteString("2. Ask the user about anything ambiguous: which repos, scope, what done looks like.\n")
	s.WriteString("3. Write the brief, replacing it whole:\n\n")
	s.WriteString("   ws mission brief --set - <<'EOF'\n   ## Goal\n   ...\n   ## Context\n   (links to issues/PRs, relevant files)\n   ## Constraints\n   ...\n   ## Acceptance\n   ...\n   EOF\n\n")
	s.WriteString("   The brief is the mission's purpose. How the work goes is the plan's (step 4), not the brief's: no `## Stages`.\n")
	s.WriteString("4. Write the plan, from the brief and the workspace's process (below): `ws mission plan --set - <<'EOF'`, the plan, then `EOF`. ws reads it strictly, and if it isn't right, it's refused with every problem and its line: put them right and run it again. Show it to the user with the brief, and ask where they want to check in (each a step of theirs).\n")
	s.WriteString("5. Name it: `ws mission title \"...\"`. A terse, plain-English name of at most 50 characters (it's how the mission reads in the user's list of missions), not the request or an issue number: \"Cache token lookups\", not \"fe issue 45\".\n")
	s.WriteString("6. Capsules aren't the brief's: the plan's steps name their repos, and ws lifts a capsule when each starts. Only to build on an existing branch or PR, dock it: `ws dock <repo> <branch|PR>`, which joins this mission.\n")
	s.WriteString("7. Show the user the brief and the plan (`ws mission show -f llm`) and check they're right.\n")
	s.WriteString("8. When they agree, run `ws mission ready` (add `--slug <short-slug>` if the current slug is poor). That confirms the brief and the plan, the go-ahead: the mission starts (ws reaches its plan's first step), and this session ends.\n\n")
	s.WriteString(processFor(b.Planning))
	s.WriteString(planWriting("`repo/name`, one you dock in step 6"))
	return s.String()
}
