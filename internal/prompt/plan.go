package prompt

import (
	"fmt"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
)

// What the planner (briefing's agent) is told of the workspace's process
// and the plan it writes from it, beside the brief.

// Planning is what the planner plans by: the process as it's written
// (Text; "" when the workspace has none), and what ws holds whatever a
// plan says (Guarantees: what each does, in words).
type Planning struct {
	Text       string
	Guarantees []string
	// Tasks are each repo's own tasks (its ws.repo.toml recipes that end
	// by themselves), for checks: what a repo runs to test itself is
	// what its checks should run, not a command made up for the step.
	Tasks []RepoTasks
}

// RepoTasks are a repo's own tasks, as its ws.repo.toml names them.
type RepoTasks struct {
	Repo  string
	Tasks []Recipe
}

// processFor is the process, and the guarantees, as the planner reads
// them.
func processFor(p Planning) string {
	var b strings.Builder
	if strings.TrimSpace(p.Text) == "" {
		b.WriteString("The workspace has no process written down (PROCESS.md, beside ws.toml), so plan from the brief and what the work needs.\n\n")
	} else {
		b.WriteString("The workspace's process says how work goes here: the steps work takes, whose turn each is, what done means, and when it's the user's. It's the team's, and you plan by it:\n\n")
		b.WriteString("````markdown\n" + strings.TrimRight(p.Text, "\n") + "\n````\n\n")
	}
	if len(p.Tasks) > 0 {
		b.WriteString("The repos' own tasks, which ws runs by name (`ws run <task>`, in the step's capsule). They're how each repo tests and checks itself, so a check that's a command uses one where it can (`ws run test` passes) rather than a command made up for the step: a narrower one can pass what the repo's CI then fails.\n\n")
		for _, r := range p.Tasks {
			var tasks []string
			for _, t := range r.Tasks {
				s := "`" + t.Name + "`"
				if d := strings.TrimSpace(t.Description); d != "" {
					s += " (" + strings.TrimRight(d, ".") + ")"
				}
				tasks = append(tasks, s)
			}
			fmt.Fprintf(&b, "- %s: %s\n", r.Repo, strings.Join(tasks, ", "))
		}
		b.WriteString("\n")
	}
	if len(p.Guarantees) > 0 {
		b.WriteString("What ws holds for every mission, whatever a plan says (so a plan follows it too):\n\n")
		for _, g := range p.Guarantees {
			b.WriteString("- " + strings.TrimRight(g, ".") + ".\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// planWriting is how the planner writes a plan: from the kinds of step
// ws knows (mission.Catalogue), which PlanFormat lists.
func planWriting(capsuleWord string) string {
	var b strings.Builder
	b.WriteString("The plan is the mission's route from A (the request) to Z (the brief's purpose met), beside the brief: what Z is, where it may stop early, and its steps in order. Each step is one of the kinds of step ws knows (below), and ws does the rest of it: it starts crew on Work, and a verifier on Verify; asks the user at Review, Decide and Land, which nobody else can pass; opens the pull request at Open Pull Request, and passes the step when it merges; waits at Wait; and comes back to you at Plan. Say what each step is for, not how ws does it. Write it from the brief and the process:\n\n")
	b.WriteString("- Say which of the process's kinds of work this is, to yourself (it's your call: ws has none built in), and follow its route, fitted to this mission. Don't add steps the process and the brief don't call for.\n")
	b.WriteString("- Z comes from the brief's Acceptance: each item something that can be checked at the end.\n")
	b.WriteString("- Work is crew's, `- in:` a repo of the workspace (`api`: ws lifts a capsule there for the mission if it hasn't one) or one of the mission's capsules (" + capsuleWord + "). One step is one crew's session, so make each a piece of work that ends in something to look at. Put a Review wherever the process says people check, and wherever the user wants to look before the work goes on (ask them where); a Decide where only they can settle something. What can't be written until earlier steps have run (the fix, once the cause is found) goes after a Plan step.\n")
	b.WriteString("- Give each Work step its checks: what has to be true for it to be done, a line each, a command where one shows it (the repo's own task, `ws run test` passes, where it has one; a reproduction `fails`), words where it takes judgement. Each check is one the judge can settle once, from the diff and what ws ran: name the behaviours, cases or files it's about, never \"every change\" or \"all of\", which has no end and comes back unmet round after round. Where a step says to use something already in the code (a function, a type, a cache), read what it does first: the step's words and its checks have to be able to hold together (one that fetches can't meet \"makes no network call\"). ws runs and has them judged once crew report done, passes the step when all are met, and otherwise the orchestrator sends it back as the process says. A step that carries on what an earlier one found (fixing what it reproduced, in the same capsule) says `- carrying on from:` that step, and its crew start fresh there with the earlier crew's handoff; one with a time box, as the process asks, says `- limit:`.\n")
	b.WriteString("- Where the process wants work shown to be right by using it, or by someone who didn't do it (a change you can see, an API against the running stack), put a Verify step after the Work it verifies, `- in:` the same repo. Its checks are what's to be shown, in words or as commands; one the process keeps for the user's own judgement is a `- yours:`. The verifier isn't given the process, so copy into the step's words how the process says it's shown (a script to run, what to capture), and have the Work step make what the verifier needs (`- makes:` the script, and a decision saying where it is and how it's run). What the verifier hands over is a bundle the user approves at a Review step: put one after it wherever the process says they look.\n")
	b.WriteString("- Where the work leaves by pull request, an Open Pull Request step comes after the Work that makes the change: ws opens it, and it's done when it merges. Crew don't open one. Put a Review before it where the process wants the user's review first. A repo with no pull requests is landed by the user instead: a Land step.\n")
	b.WriteString("- Where a step carries out the process, end it with the process's words, in double quotes, so a change to the process shows which steps it touches.\n\n")
	b.WriteString(mission.PlanFormat + "\n")
	return b.String()
}

// Replanning is the plan as the planner comes back to it, once the
// mission's briefed: at a step of its own, when the orchestrator says the
// route doesn't fit, or when the user asks.
type Replanning struct {
	Plan string // plan.md as it is
	// Fixed is how many of its first steps are behind the mission (passed,
	// or the one it's at once begun): they stay as they are.
	Fixed int
	// At is where the mission is on it, in words ("Step 3 of 6, the
	// planner's: Plan the fix"); "" at its end.
	At string
}

// askedSince is what's been asked of the agent since its last word (and
// while it was saying it): by the user, and by ws itself (the mission
// reaching the planner's step, the orchestrator saying the route doesn't
// fit).
func askedSince(c *conversation.Conversation) []string {
	var said []string
	for _, t := range slices.Backward(c.Turns) {

		switch {
		case t.By == conversation.You:
			said = append([]string{t.Body}, said...)
		case t.By == conversation.WS && t.Asks:
			said = append([]string{"(ws) " + t.Body}, said...)
		case t.By == conversation.WS:
			continue // ws saying the agent couldn't answer isn't its word
		default:
			// What was asked while it answered, which that answer didn't
			// read, comes first.
			var missed []string
			for _, m := range c.Missed() {
				if m.By == conversation.WS {
					missed = append(missed, "(ws) "+m.Body)
				} else {
					missed = append(missed, m.Body)
				}
			}
			return append(missed, said...)
		}
	}
	return said
}

// PlannerFirst starts the planner coming back to a mission's plan, in a
// session of its own: in the mission's briefing conversation, carried on,
// or one opened for it.
func PlannerFirst(h Here, c *conversation.Conversation, r Replanning) string {
	var b strings.Builder
	m := h.Mission
	title, slug := "", ""
	if m != nil {
		title, slug = m.Title, m.Slug
	}
	fmt.Fprintf(&b, "You're the planner of the mission %q (%s), in conversation %s in the web UI of ws, which manages this multi-repo workspace (%s). The user reads your replies there, rendered as markdown. The plan is the mission's route from A to Z: what Z is, where it may stop early, and its steps, each with who's on it. The planner writes it at briefing and comes back to write more of it: at a step of its own (one whose steps after it couldn't be written until the steps before had run), when the orchestrator says the route doesn't fit what's happened, and when the user asks. That's now. You don't do the work, and you don't move the mission along (ws and the orchestrator do, by the plan): you plan.\n\n", title, slug, c.ID, h.Root)
	if len(c.Turns) > len(askedSince(c)) {
		fmt.Fprintf(&b, "This conversation has been going a while (it's where the mission was briefed, perhaps): `ws show -f llm %s` has its summary and turns so far.\n\n", c.ID)
	}
	b.WriteString("Start with `ws show -f llm` (WS_MISSION is set): the brief, the plan's steps with ws's record of each, the crew and their handoffs, what the steps made (artifacts), its decisions and the log. `ws show -f llm <ref>` shows any of it whole, with what it links to. Read what the steps before made (handoffs, documents, decisions) before you plan what comes after them.\n\n")
	b.WriteString(para(h.Agents))
	b.WriteString(talkReading(h))
	b.WriteString(replanState(r))
	b.WriteString(replanHow)
	b.WriteString(processFor(h.Planning))
	b.WriteString(planWriting("`repo/name`: one of the mission's capsules, or a new one in a repo of the workspace, which ws lifts for it (or joins to it, when it's lifted already)"))
	b.WriteString(repos(h))
	b.WriteString("What's asked of you:\n\n")
	for _, s := range askedSince(c) {
		b.WriteString(s + "\n\n")
	}
	b.WriteString("Do it, then say in your reply what you changed and why, or why the plan fits as it is, or what you need to know. Keep it short.\n\n")
	b.WriteString(talkEnding)
	return b.String()
}

// PlannerNext is a later turn of the planner, in its session: the plan as
// it is now, and what's been asked since.
func PlannerNext(h Here, c *conversation.Conversation, r Replanning) string {
	var b strings.Builder
	b.WriteString("You're the mission's planner, back at its plan. ")
	b.WriteString(replanState(r))
	b.WriteString("Revise it as before: `ws mission plan --propose --set -`, the whole plan, with the steps behind the mission as they are. What's asked of you:\n\n")
	for _, s := range askedSince(c) {
		b.WriteString(s + "\n\n")
	}
	b.WriteString("Reply as before. " + shellNext(h) + talkEnding)
	return b.String()
}

// replanState is the plan as it is, and how far the mission's got on it.
func replanState(r Replanning) string {
	var b strings.Builder
	b.WriteString("The plan as it is:\n\n````markdown\n" + strings.TrimRight(r.Plan, "\n") + "\n````\n\n")
	switch {
	case r.At == "":
		b.WriteString("Every step of it is passed.")
	default:
		fmt.Fprintf(&b, "The mission is at %s.", strings.TrimRight(strings.TrimSpace(r.At), "."))
	}
	switch {
	case r.Fixed == 1:
		b.WriteString(" Step 1 is behind it: it stays as it is, word for word, and a revision changes only what's after it.\n\n")
	case r.Fixed > 1:
		fmt.Fprintf(&b, " Steps 1 to %d are behind it: they stay as they are, word for word, and a revision changes only what's after them.\n\n", r.Fixed)
	default:
		b.WriteString(" Nothing's behind it yet.\n\n")
	}
	return b.String()
}

// replanHow is how the planner revises the plan, and what's whose.
const replanHow = "To revise it, write the whole plan as it should read: `ws mission plan --propose --set - <<'EOF'`, then the plan, then `EOF` on a line of its own. ws reads it strictly and refuses it with every problem and its line: put them right and run it again. Then, as ws says when you run it:\n\n" +
	"- A revision that removes or moves a checkpoint (a Review, Decide or Land step: the user's), or changes Z, is the user's to confirm: it becomes a proposal here, which they accept or not. Say in your reply why it's needed.\n" +
	"- Anything else applies at once, and the mission's log says what changed. Written at your own step, it passes that step, and ws moves on to the next (starting crew there, asking the user, or waiting).\n" +
	"- Change only what needs changing, and keep a checkpoint wherever the process or the user wants one. At your own step, if the steps after it are right as they are, write the plan as it is: that passes your step.\n" +
	"- A step's level the user set (a decision of theirs says so) stays as they set it.\n" +
	"- A step in a capsule the mission hasn't got yet (`repo/branch`, in a repo of the workspace), or a Work step in a repo it has none in, gets one lifted (or the one there joined) when the plan's written.\n" +
	"- When what the steps before found changes what the mission is for (it was to find a bug's cause; now it's to fix it), propose its new brief first, its Goal and Acceptance with the rest as it was: `ws mission brief --propose --set - <<'EOF'`, the brief, then `EOF`. Then propose the plan to get there: the two come to the user as one proposal, and confirming them is the go-ahead. A change to the brief is always theirs to confirm.\n" +
	"- If the rest of the work is better done as missions of their own (a redesign's build, once its proposal is accepted, is often several), propose each: `ws mission new --propose \"<title>\" --brief-file - <<'EOF'`, its brief (`## Goal`, `## Context`, `## Constraints`, `## Acceptance`), then `EOF`. A mission you propose here is raised from this one, and the user confirms it. Then plan what's left of this one, Z saying what's handed on.\n" +
	"- You may also propose a decision once something's settled: `ws mission decide --propose \"<the decision>\" --why \"<why>\"`.\n\n" +
	"Ask the user, here, when you need to know something to plan: what they want checked, which way to go. Run the commands yourself: don't paste them into your reply.\n\n"
