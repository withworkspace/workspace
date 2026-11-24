package prompt

import (
	"fmt"
	"strings"

	"github.com/withworkspace/workspace/internal/mission"
)

// What agents are told of a mission in plain words, one line a thing:
// its log, its stages, whose court it's in. The orchestrator reads them,
// and so does crew's ws mission show.

// EventLine is one event of a mission's log, its time in UTC.
func EventLine(e mission.Event) string {
	var detail []string
	if e.Repo != "" || e.Capsule != "" {
		detail = append(detail, e.Repo+"/"+e.Capsule)
	}
	if e.Text != "" {
		detail = append(detail, e.Text)
	}
	who := ""
	if e.Crew != "" {
		who = " (" + e.Crew + ")"
	}
	return fmt.Sprintf("%s %s%s %s", e.Time.UTC().Format("2006-01-02T15:04Z"), e.Kind, who, strings.Join(detail, " — "))
}

// CourtWords says whose court a mission is in, to crew and the
// orchestrator, for whom you're "the user".
func CourtWords(c mission.Court) string {
	switch c {
	case mission.CourtAgent:
		return "an agent's court (crew or the orchestrator is on it)"
	case mission.CourtYou:
		return "the user's court"
	case mission.CourtOutside:
		return "outside (waiting on someone else: reviewers, checks, a merge, what a Wait step waits for)"
	}
	return "nobody's (archived)"
}

// StageLines describes the brief's stages and their progress in words, one
// line each, for agents and plain output. A brief without stages has none,
// unless its work has been accepted.
func StageLines(m *mission.Mission, brief string) []string {
	states, orphans := m.StageProgress(brief)
	var out []string
	for _, st := range states {
		if st.Implicit && !st.Accepted() {
			return nil
		}
		mark := "to do"
		switch {
		case st.Skipped():
			mark = "skipped"
		case st.Accepted():
			mark = "accepted"
		case st.Current && st.Held():
			mark = "current, waiting for your call"
		case st.Current:
			mark = "current"
		}
		line := fmt.Sprintf("%d. [%s] %s", st.N, mark, oneLine(st.Text))
		if st.Implicit {
			line = "[accepted] the whole brief"
		}
		var facts []string
		if st.Step != nil {
			facts = append(facts, st.Step.Type+", "+st.Step.WhoSays())
			if st.Step.Type == mission.StepWait {
				// Who passes a Wait is by who it's on and whether ws can
				// see it happen: said on each, where it's read.
				facts[0] = waitFact(*st.Step)
			}
			if st.Step.Makes != "" {
				facts = append(facts, "makes: "+oneLine(st.Step.Makes))
			}
			if st.Step.From != "" {
				facts = append(facts, "options: "+oneLine(st.Step.From))
			}
			if st.Step.OnlyIf != "" {
				facts = append(facts, "only if "+oneLine(st.Step.OnlyIf))
			}
			if st.Step.InEach != "" {
				facts = append(facts, "in each "+oneLine(st.Step.InEach))
			}
			if st.Step.CarryOn > 0 {
				facts = append(facts, fmt.Sprintf("carrying on from step %d", st.Step.CarryOn))
			}
			if st.Step.LimitText != "" {
				facts = append(facts, "limit: "+oneLine(st.Step.LimitText))
			}
			if n := len(st.Step.Checks); n > 0 {
				var checks []string
				for _, c := range st.Step.Checks {
					checks = append(checks, oneLine(c.Text))
				}
				facts = append(facts, "checks: "+strings.Join(checks, "; "))
			}
		}
		if r := st.Record; r != nil && !st.Accepted() {
			if r.OutOfTime != nil {
				facts = append(facts, "out of time")
			} else if r.Deadline != nil {
				facts = append(facts, "time runs out at "+r.Deadline.Local().Format("15:04"))
			}
		}
		if ins := st.Instances(); len(ins) > 0 {
			var runs []string
			for _, in := range ins {
				r := in.Repo
				switch {
				case in.Done != nil:
					r += " done"
				case in.Capsule != "":
					r += " in " + in.Capsule
				}
				runs = append(runs, r)
			}
			facts = append(facts, "runs in "+strings.Join(runs, ", "))
		}
		if r := st.Record; r != nil && r.Attempts > 0 {
			facts = append(facts, fmt.Sprintf("attempt %d", r.Attempts+1))
		}
		if round, ok := st.Record.LatestChecks(); ok {
			facts = append(facts, fmt.Sprintf("checks round %d (%s's done): %s", round.N, round.Crew, round.Summary()))
		}
		if st.CheckIn {
			facts = append(facts, "check in with the user after it")
		}
		if st.Record != nil && len(st.Record.Crew) > 0 {
			facts = append(facts, "crew "+strings.Join(st.Record.Crew, ", "))
		}
		if st.Accepted() {
			a := "accepted by " + st.Record.AcceptedBy
			if st.Skipped() {
				a = "skipped by " + st.Record.AcceptedBy
			}
			if st.Record.Why != "" {
				a += ": " + st.Record.Why
			}
			facts = append(facts, a)
		}
		if len(facts) > 0 {
			line += " (" + strings.Join(facts, "; ") + ")"
		}
		out = append(out, line)
	}
	where := "the brief"
	if m.Plan != nil {
		where = "the plan"
	}
	for _, o := range orphans {
		line := "no longer in " + where + ": " + oneLine(o.Text)
		if o.Accepted != nil {
			line += " (accepted)"
		}
		out = append(out, line)
	}
	return out
}

// waitUnwatched is who passes a Wait that nobody's watching, as the
// orchestrator's told: the user, from the page, or the orchestrator once
// it's shown.
const waitUnwatched = "nobody's watching, so the user marks it done once it's happened, or you accept it once you're shown it has"

// waitFact is a Wait step's kind and who's on it, with who passes it: the
// user, ws watching its checks, or whoever can say it's happened.
func waitFact(s mission.Step) string {
	switch {
	case s.WaitOnYou() && s.Watched():
		return "Wait, on the user: theirs to do; ws watches its checks, and passes it"
	case s.WaitOnYou():
		return "Wait, on the user: theirs to do and mark done, and only theirs"
	case s.Watched():
		return "Wait, on " + firstNonEmpty(s.Whom, s.WhoSays()) + ": ws watches its checks, and passes it"
	case s.Whom != "":
		return "Wait, on " + s.Whom + ": " + waitUnwatched
	}
	return "Wait, " + s.WhoSays() + ": " + waitUnwatched
}
