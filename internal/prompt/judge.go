package prompt

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// The judge: an agent with fresh context, headless and read-only, who
// didn't do a Work step's work, judging what the step's checks say in
// words once its crew report done (the checks ws runs, commands, it runs
// itself). Each check is met, unmet, or can't tell, with what shows it:
// can't tell is the user's, never a guess.

// Judging is what the judge is told.
type Judging struct {
	Mission, Title string // the mission's slug and title
	// Step is the step, as the plan says it ("3: Work: fix it."), Makes
	// what it says it makes, and Goal the brief's goal.
	Step, Makes, Goal string
	// Checks are the checks it judges, as written, numbered from 1 in the
	// order given.
	Checks []string
	// Crew is who did the work, Capsule where (repo/name), and Commit the
	// commit it's judged at. Dir is the capsule's folder, where the judge
	// runs.
	Crew, Capsule, Commit, Dir string
	// Handoff is what the crew handed off when they reported done.
	Handoff string
	// Diff is what the capsule's commits change, from where it left its
	// default branch: a stat, then the change, cut to fit.
	Diff string
	// Ran is what the checks ws ran came to, a line each, for what they
	// show.
	Ran []string
	// Artifacts are what the mission has made for the step, a line each
	// with its ref, for ws show.
	Artifacts []string
}

// Judge is the judge's prompt.
func Judge(j Judging) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are judging work done on the mission %q (%s), in a workspace managed by the ws tool. Crew (an agent) did a step of the mission's plan and reported it done; you didn't do it, and you come to it fresh. Say, for each check below, whether what it says is true of the work as it is now.\n\n", j.Title, j.Mission)
	fmt.Fprintf(&b, "The step is %s\n", sentenceEnd(j.Step))
	if j.Makes != "" {
		fmt.Fprintf(&b, "It makes: %s\n", sentenceEnd(j.Makes))
	}
	if j.Goal != "" {
		fmt.Fprintf(&b, "The mission's goal: %s\n", sentenceEnd(oneLine(j.Goal)))
	}
	fmt.Fprintf(&b, "\nCrew %s did it in the capsule `%s`, at commit %s: you're in its folder, and may read what's there (Read, Grep, Glob), but change nothing. `ws show -f llm <ref>` shows any of the mission's documents and handoffs.\n\n", j.Crew, j.Capsule, shortCommit(j.Commit))
	b.WriteString("## The checks\n\n")
	for i, c := range j.Checks {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c)
	}
	b.WriteString("\nFor each, one of:\n\n")
	b.WriteString("- **met**: it's true, and you can say what shows it: a file and its lines, a passage of a document, a test.\n")
	b.WriteString("- **unmet**: it isn't, or not wholly: say what's missing or wrong, so whoever puts it right knows what to do.\n")
	b.WriteString("- **can't tell**: what's here can't show it either way (it needs running something you can't, or someone who knows what you don't). Say what would show it. Don't guess: can't tell goes to the user, and a guess goes nowhere.\n\n")
	b.WriteString("Judge the work, not the handoff: what crew say they did is where to look, not what shows it. A check is about what's true now, not what crew meant to do.\n\n")
	b.WriteString("Answer with a line for each check, in order, and nothing else:\n\n")
	b.WriteString("```\n1. met: <what shows it>\n2. unmet: <what's missing or wrong>\n3. can't tell: <what would show it>\n```\n\n")
	b.WriteString(Naming)
	if len(j.Ran) > 0 {
		b.WriteString("## What ws ran\n\nThe step's checks that are commands, which ws ran in the capsule:\n\n")
		for _, r := range j.Ran {
			b.WriteString("- " + r + "\n")
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(j.Handoff) != "" {
		fmt.Fprintf(&b, "## %s's handoff\n\n%s\n\n", j.Crew, strings.TrimSpace(j.Handoff))
	}
	if len(j.Artifacts) > 0 {
		b.WriteString("## What the mission has made\n\n")
		for _, a := range j.Artifacts {
			b.WriteString("- " + a + "\n")
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(j.Diff) != "" {
		fmt.Fprintf(&b, "## What the capsule's commits change\n\n```diff\n%s\n```\n", strings.TrimRight(j.Diff, "\n"))
	} else {
		b.WriteString("## What the capsule's commits change\n\nNothing yet: it has no commits of its own.\n")
	}
	return b.String()
}

// Verdict is what the judge said of one check.
type Verdict struct {
	Result string // met, unmet, can't tell
	Why    string
}

// judgeLine is a line of the judge's answer: "2. unmet: …".
var judgeLine = regexp.MustCompile(`(?i)^\s*(?:[-*]\s*)?(\d+)[.):]?\s*\**\s*(met|unmet|not met|can['’]?t tell|cannot tell|can not tell)\s*\**\s*[:\-—–]?\s*(.*)$`)

// ReadVerdicts reads the judge's answer for n checks: what it said of
// each, in order. A check it said nothing of is can't tell.
func ReadVerdicts(said string, n int) []Verdict {
	out := make([]Verdict, n)
	at := -1
	for _, line := range strings.Split(said, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			continue
		}
		if m := judgeLine.FindStringSubmatch(line); m != nil {
			i, _ := strconv.Atoi(m[1])
			at = -1 // a check it hasn't, or said twice: nothing of it counts
			if i >= 1 && i <= n && out[i-1].Result == "" {
				at = i - 1
				out[at] = Verdict{Result: verdictOf(m[2]), Why: strings.TrimSpace(m[3])}
			}
			continue
		}
		if at >= 0 && strings.TrimSpace(line) != "" {
			out[at].Why = strings.TrimSpace(out[at].Why + " " + strings.TrimSpace(line))
		}
	}
	for i := range out {
		if out[i].Result == "" {
			out[i] = Verdict{Result: "can't tell", Why: "the judge said nothing of it"}
		}
	}
	return out
}

func verdictOf(s string) string {
	s = strings.ToLower(s)
	switch {
	case s == "met":
		return "met"
	case s == "unmet" || s == "not met":
		return "unmet"
	}
	return "can't tell"
}

func sentenceEnd(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s[len(s)-1:], ".?!") {
		return s
	}
	return s + "."
}

func shortCommit(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	if sha == "" {
		return "(none yet)"
	}
	return sha
}
