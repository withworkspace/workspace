package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
)

func (in Input) now() time.Time {
	if in.Now.IsZero() {
		return time.Now()
	}
	return in.Now
}

// heldTurn tells the orchestrator of a quiet turn ws held while the work
// crew left running ran (mission.Crew.Hold), and hands on now: what ran,
// whether it ended, and how long after the turn.
func heldTurn(crewID string, p *mission.Pending, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Crew %s finished a turn %s ago without reporting done or asking the user anything, after a message ws sent it (its opening prompt, or your last nudge). It had left work running in the background, so ws held the turn until now, rather than hand it to you while the work ran:\n\n", crewID, minutes(now.Sub(p.At)))
	for _, w := range p.Work {
		fmt.Fprintf(&b, "- `%s`: ", mission.WorkWhat([]mission.Work{w}))
		if w.Ended != nil {
			fmt.Fprintf(&b, "ended %s after the turn did.", minutes(w.Ended.Sub(p.At)))
		} else {
			fmt.Fprintf(&b, "still running, %s after the turn ended.", minutes(now.Sub(p.At)))
		}
		switch w.Wakes {
		case mission.WakeYes:
			fmt.Fprintf(&b, " Its harness said it would wake %s when it ended", crewID)
			if w.Ended != nil {
				b.WriteString(", and it hasn't.\n")
			} else {
				b.WriteString(".\n")
			}
		case mission.WakeNo:
			fmt.Fprintf(&b, " Nothing wakes %s for it.\n", crewID)
		default:
			fmt.Fprintf(&b, " Nothing is known to wake %s for it.\n", crewID)
		}
	}
	if len(p.Running()) == 0 {
		fmt.Fprintf(&b, "\nThe work has ended and nobody has told %s: it's still waiting on it. Read its last message and output, then resume-with what to do: usually to read the work's result and carry on, or, if it was only waiting on that result, to check it and report done with a handoff. If it put a question to the user in its reply, flag the user with that question rather than answering it yourself.\n", crewID)
	} else {
		fmt.Fprintf(&b, "\nws stopped holding the turn because the work has run %s: a server or watcher meant to keep running, or something that hangs. Read its last message and output: if the work should have finished by now, resume-with to find out why and what to do; if it's meant to keep running, have %s carry on without waiting for it. If it put a question to the user in its reply, flag the user with that question rather than answering it yourself.\n", minutes(mission.HoldMax)+" or more", crewID)
	}
	return b.String()
}

// minutes is a while, in words, to the minute.
func minutes(d time.Duration) string {
	switch n := int(d.Round(time.Minute) / time.Minute); {
	case d < time.Minute:
		return "under a minute"
	case n == 1:
		return "a minute"
	default:
		return fmt.Sprintf("%d minutes", n)
	}
}
