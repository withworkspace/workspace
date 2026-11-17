package prompt

import (
	"fmt"
	"strings"
)

// The verifier: crew at a Verify step, who didn't do the work and shows
// whether it's as the step's checks say by using it. What they hand over
// is a verification bundle, and handing it over ends their part. They're
// told their step and where to start, not the process: the planner copied
// what applies into the step.

// Verifying is what a verifier is told.
type Verifying struct {
	Slug, Title string
	// Goal is the brief's goal.
	Goal string
	// StepN of StepOf is their step, Step what it says, and Checks its
	// checks, by number: Command is one ws runs itself, Yours one that's
	// the user's to settle.
	StepN, StepOf int
	Step          string
	Checks        []VerifyCheck
	// Limit is how long they have, as the step says it ("" for none).
	Limit string
	// Capsule (repo/name) is where the work is, at Commit.
	Capsule, Commit string
	// Worker is the crew who made the work, and Handoff where their
	// handoff is ("" when they left none).
	Worker, Handoff string
	// Before is the bundle of this step that went back before (b2), and
	// Unmet what it found unmet: this is the work verified again.
	Before string
	Unmet  []string
	// Files is the mission's private files folder, for what they capture.
	Files string
	// Guide is what's told of the workspace's own way of working.
	Guide string
}

// VerifyCheck is one of a Verify step's checks, as its verifier's told it.
type VerifyCheck struct {
	Text           string
	Command, Yours bool
}

// Verifier is the first message given to a verifier.
func Verifier(v Verifying) string {
	var s strings.Builder
	fmt.Fprintf(&s, "You are verifying work on the mission %q (%s), managed by the ws workspace tool. ", v.Title, v.Slug)
	s.WriteString("You didn't do the work, and you come to it fresh: your job is to show whether it's as its checks say, by using it, not by reading what its author says of it. ")
	s.WriteString("Run it, look at it, and keep what you see. You are NOT fixing anything: what isn't right goes back to the crew who made it.\n\n")
	if v.Goal != "" {
		fmt.Fprintf(&s, "The mission's goal: %s\n\n", sentenceEnd(oneLine(v.Goal)))
	}
	fmt.Fprintf(&s, "You're on step %d of the plan's %d, a Verify step:\n\n%s\n\n", v.StepN, v.StepOf, strings.TrimSpace(v.Step))
	s.WriteString("Its checks, by number:\n\n")
	for i, c := range v.Checks {
		who := "yours to settle: met, unmet or can't tell"
		switch {
		case c.Command:
			who = "a command: ws runs it itself once you hand over, so leave it out of your bundle"
		case c.Yours:
			who = "the user's to settle, not yours: say yours:, and leave what helps them (a screenshot, how to try it)"
		}
		fmt.Fprintf(&s, "%d. %s (%s)\n", i+1, c.Text, who)
	}
	fmt.Fprintf(&s, "\nThe work is in the capsule `%s`, at commit %s: you're in its folder. ", v.Capsule, shortCommit(v.Commit))
	if v.Worker != "" {
		fmt.Fprintf(&s, "Crew %s made it", v.Worker)
		if v.Handoff != "" {
			fmt.Fprintf(&s, ", and their handoff says where things are (`ws show -f llm %s`)", v.Handoff)
		}
		s.WriteString(". ")
	}
	fmt.Fprintf(&s, "Start with `ws show -f llm`: the mission, its decisions (where a script is, how something's run) and its handoffs; `ws show -f llm s%d` is your step. ", v.StepN)
	if v.Guide != "" {
		s.WriteString(v.Guide + " ")
	}
	s.WriteString("\n\n")
	if v.Before != "" {
		fmt.Fprintf(&s, "This is the work verified again: %s went back to its crew", v.Before)
		if len(v.Unmet) > 0 {
			s.WriteString(", with these unmet:\n\n")
			for _, u := range v.Unmet {
				s.WriteString("- " + u + "\n")
			}
			s.WriteString("\n")
		} else {
			s.WriteString(". ")
		}
		fmt.Fprintf(&s, "Verify all of it afresh, not only what was unmet: a fix can break what worked (`ws show -f llm %s` has what was shown before).\n\n", v.Before)
	}
	s.WriteString("How to go about it:\n\n")
	s.WriteString("- Use it as its user would: run it, drive it, look. Where the step or a decision names a script or a skill for showing it (a shots script, a demo), use that. A check you settle from the code alone, say so.\n")
	fmt.Fprintf(&s, "- Keep what you see as files: a screenshot, a short recording (.webm or .mp4), what a command printed as text. Put them in the mission's private files folder (%s), not in the capsule.\n", v.Files)
	s.WriteString("- Change nothing in the capsule: no edits to tracked files, no commits. ws refuses a bundle from a capsule that's moved or changed since you started. If you can't verify without a change, that's what you found: say so.\n")
	s.WriteString("- Don't guess. `can't tell` is an honest answer and goes to the user, with what would show it. `unmet` says what's missing or wrong, plainly enough for whoever puts it right.\n")
	s.WriteString("- If you need the user (something won't start, a credential), ask in your reply and run `ws mission ask \"<the question>\"`, then wait.\n")
	if v.Limit != "" {
		fmt.Fprintf(&s, "- You have %s: ws holds the clock from when you start.\n", strings.TrimSuffix(v.Limit, "."))
	}
	s.WriteString("\nThen hand over a verification bundle, which ends your part (there's no `ws mission done` for you: the bundle is it):\n\n")
	s.WriteString("  ws mission bundle --set - <<'EOF'\n  ## Summary\n  …\n  ## Try it\n  - try: `<setup> && cd <where> && <what starts it>`\n  - look for: …\n  ## Checks\n  1. met: …\n     - shows: `<file>` what it shows\n  ## Not verified\n  - …\n  EOF\n\n")
	s.WriteString("`ws mission bundle --format` says how it's written, with your step's checks by number. ws reads it strictly, and refuses one that isn't right with every problem and its line: put them right and hand it over again. ")
	s.WriteString("In Try it, a `try:` is one command, which the user copies once: chain setup, `cd` and what starts it with `&&` (`a && cd x && b`), with no second command and no words after it. What to do or see in the UI once it's running goes in `look for:` items, one each. ")
	s.WriteString("Every check that's met, or left for the user, and the step is done; one unmet, and the work goes back to its crew, to be verified again once they've put it right.\n\n")
	s.WriteString("What you write the user reads in ws's web UI, on the mission's Verify tab. " + Naming)
	return s.String()
}
