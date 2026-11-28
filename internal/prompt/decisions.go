package prompt

import (
	"fmt"
	"regexp"
	"strings"
)

// The decision look: a small model, headless, reading the decisions crew
// recorded beside what they're held to (their step's checks, and the
// user's own decisions), and saying of each whether it goes against one.
// It's a cheap look, made after crew have been quiet a moment: what it
// finds goes to the crew, who know their work, and a false alarm costs
// them a sentence. It's there so a decision against a check is put right
// while it's made, not a round of checks later.

// DecisionLook is what the decision look is told.
type DecisionLook struct {
	Mission, Title, Goal string
	// Step is the step the decisions were made at, as the plan says it
	// ("3: Work: fix it."), and Checks its checks, as written, numbered
	// from 1. Neither, for decisions made with no step.
	Step   string
	Checks []string
	// Yours are the user's decisions that stand, a line each ("d1: …").
	Yours []string
	// Decisions are crew's, to look at, a line each, each beginning with
	// its id ("d3 (c1): …").
	Decisions []string
}

// LookAtDecisions is the decision look's prompt.
func LookAtDecisions(l DecisionLook) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Crew (agents) working on the mission %q (%s), in a workspace managed by the ws tool, recorded the decisions below as they worked. Say, for each, whether it goes against what the work is held to: ", l.Title, l.Mission)
	switch {
	case len(l.Checks) > 0 && len(l.Yours) > 0:
		b.WriteString("their step's checks, or a decision the user made.\n\n")
	case len(l.Checks) > 0:
		b.WriteString("their step's checks.\n\n")
	default:
		b.WriteString("a decision the user made.\n\n")
	}
	if l.Goal != "" {
		fmt.Fprintf(&b, "The mission's goal: %s\n\n", sentenceEnd(oneLine(l.Goal)))
	}
	if l.Step != "" {
		fmt.Fprintf(&b, "They were made at step %s\n\n", sentenceEnd(l.Step))
	}
	if len(l.Checks) > 0 {
		b.WriteString("## The step's checks\n\nWhat has to be true for the step to be done, judged once crew report it done:\n\n")
		for i, c := range l.Checks {
			fmt.Fprintf(&b, "%d. %s\n", i+1, c)
		}
		b.WriteString("\n")
	}
	if len(l.Yours) > 0 {
		b.WriteString("## The user's decisions\n\nThese stand over anything crew decide:\n\n")
		for _, d := range l.Yours {
			b.WriteString("- " + d + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## Crew's decisions, to look at\n\n")
	for _, d := range l.Decisions {
		b.WriteString("- " + d + "\n")
	}
	b.WriteString("\nA decision goes against a check when, done as it says, the check can't be met (it says to read git where a check says nothing reads git; it drops what a check asks for). It goes against a user's decision when it does what theirs rules out, or undoes it. A decision that's only about how, where nothing above says otherwise, fits. Where you aren't sure, it fits: what you flag goes to the crew, who'd stop to look at it.\n\n")
	b.WriteString("Answer with a line for each decision, in order, and nothing else:\n\n")
	b.WriteString("```\nd3: fits\nd4: check 2: <what in the decision the check rules out, in a sentence>\nd5: d1: <what in it the user's d1 rules out, in a sentence>\n```\n")
	return b.String()
}

// DecisionFinding is what the decision look said of one decision: Against
// is what it goes against ("check 2", "d1"), or "" where it fits.
type DecisionFinding struct {
	ID, Against, Why string
}

// decisionLine is a line of the decision look's answer.
var decisionLine = regexp.MustCompile(`(?i)^\s*(?:[-*]\s*)?\**(d\d+)\**\s*[:\-—–]\s*(fits|check\s*#?\s*(\d+)|(d\d+))\s*[:\-—–]?\s*(.*)$`)

// ReadDecisionFindings reads the decision look's answer for the decisions
// ids: what it said of each, in the order given. One it said nothing of,
// or something it can't be read as, fits: a look that can't be read
// flags nothing.
func ReadDecisionFindings(said string, ids []string) []DecisionFinding {
	got := map[string]DecisionFinding{}
	for line := range strings.SplitSeq(said, "\n") {
		m := decisionLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id := strings.ToLower(m[1])
		f := DecisionFinding{ID: id}
		switch {
		case m[3] != "":
			f.Against, f.Why = "check "+m[3], strings.TrimSpace(m[5])
		case m[4] != "":
			f.Against, f.Why = strings.ToLower(m[4]), strings.TrimSpace(m[5])
		}
		got[id] = f
	}
	out := make([]DecisionFinding, len(ids))
	for i, id := range ids {
		out[i] = DecisionFinding{ID: id}
		if f, ok := got[id]; ok {
			out[i] = f
		}
	}
	return out
}
