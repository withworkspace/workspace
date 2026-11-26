package mission

import (
	"os"
	"regexp"
	"strings"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/store"
)

// What a mission's log shares with the workspace's events (events) is
// written to both in the same transaction, from appendLog: the mission
// started, a capsule of it landed or burned, a pull request of it opened,
// merged or failing its checks, a decision made, the mission ended. The
// rest of its log stays its own.

// withdrawLine is a decision line that withdraws one ("d1 withdrawn: …").
var withdrawLine = regexp.MustCompile(`^d\d+ withdrawn: `)

// shared is the workspace event a line of a mission's log is, if it's one.
func shared(slug string, e Event) (events.Event, bool) {
	out := events.Event{
		At: e.Time, By: byOf(e), Cause: e.Run,
		Mission: slug, Repo: e.Repo, Capsule: e.Capsule, Text: e.Text,
	}
	switch e.Kind {
	case EventCreated:
		out.Kind = events.MissionStarted
	case EventLanded:
		out.Kind = events.CapsuleLanded
	case EventCapsuleBurned:
		out.Kind = events.CapsuleBurned
	case EventEnded:
		out.Kind = events.MissionEnded
	case EventDecision:
		if withdrawLine.MatchString(e.Text) {
			return out, false
		}
		out.Kind = events.DecisionMade
	case EventPR:
		switch e.What {
		case PREventOpened:
			out.Kind = events.PROpened
		case PREventMerged:
			out.Kind = events.PRMerged
		case PREventChecksFailed:
			out.Kind = events.PRChecksFailed
		default:
			return out, false
		}
		out.PR = prOf(e.Text)
	default:
		return out, false
	}
	return out, true
}

// byOf is who did what a line of the log says, as an event says it: its
// By, or whom it's for where that's who did it, or who this process is.
func byOf(e Event) string {
	who := e.By
	if who == "" {
		who = e.Crew
	}
	switch who {
	case "":
		if os.Getenv(EnvRole) == RoleOrchestrator {
			return events.ByOrchestrator
		}
		return events.ByYou
	case "you", "user":
		return events.ByYou
	case RoleOrchestrator:
		return events.ByOrchestrator
	case "ws":
		return events.ByWS
	}
	return events.ByCrew(who)
}

// prOf is the pull request a PR line of the log names first (demo#12).
func prOf(text string) string {
	for w := range strings.FieldsSeq(text) {
		w = strings.TrimSuffix(strings.TrimRight(w, ":,.)"), "'s")
		if repo, n, ok := strings.Cut(w, "#"); ok && repo != "" && n != "" && strings.Trim(n, "0123456789") == "" {
			return w
		}
	}
	return ""
}

// share writes the event a line of the log is, if it's one, in tx.
func share(tx *store.Tx, slug string, e Event) error {
	ev, ok := shared(slug, e)
	if !ok {
		return nil
	}
	_, err := events.Append(tx, ev)
	return err
}
