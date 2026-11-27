package mission

import (
	"regexp"
	"strings"
)

// Sentence is a log entry in plain English, for a list of what's happened.
// Entries that only say an orchestrator run started report nothing.
func Sentence(e Event) (string, bool) {
	text := strings.TrimSpace(e.Text)
	if first, _, cut := strings.Cut(text, "\n"); cut {
		text = first + " …"
	}
	who := firstNonEmpty(e.Crew, "someone")
	capsule := e.Repo + "/" + e.Capsule
	switch e.Kind {
	case EventCreated:
		return "mission created", true
	case EventCapsuleAdded:
		return "capsule " + capsule + " added", true
	case EventCapsuleRemove:
		return "capsule " + capsule + " detached", true
	case EventCapsuleBurned:
		return "capsule " + capsule + " burned", true
	case EventLanded: // "2 commits on main (275b7dd)"
		return "you landed " + capsule + ": " + text, true
	case EventNote:
		if rest, ok := strings.CutPrefix(text, "Orchestrator: "); ok {
			return "orchestrator: " + rest, true
		}
		return "note: " + text, true
	case EventCrewStarted:
		return who + " started", true
	case EventCrewEnded:
		switch e.What {
		case EndedInterrupted:
			return who + " stopped when Workspace restarted", true
		case EndedHandedBack:
			return who + "'s session closed: work handed back", true
		case EndedStopped:
			return who + " was stopped", true
		}
		return who + " ended (" + text + ")", true
	case EventCrewResumed:
		return who + " resumed", true
	case EventCrewAsked:
		return who + " asks: " + text, true
	case EventCrewDone:
		return who + " done: " + text, true
	case EventArtifact:
		return who + " added " + text, true
	case EventFlag:
		return "asks you: " + text, true
	case EventFlagAnswered:
		return "you answered: " + text, true
	case EventReview:
		id, verdict, _ := strings.Cut(text, ": ")
		return "you reviewed " + capsule + ": " + verdict + " (" + id + ")", true
	case EventThread:
		return threadSentence(e.Crew, text), true
	case EventDecision: // "d1: Key per tenant." or "d1 withdrawn: …"
		who := firstNonEmpty(e.Crew, "you")
		if id, why, ok := strings.Cut(text, " withdrawn: "); ok {
			return who + " withdrew " + id + ": " + why, true
		}
		return who + " decided " + text, true
	case EventReviewReply: // "r1.c3 addressed in 275b7dd: …", by crew or you
		return firstNonEmpty(e.Crew, "you") + " on " + text, true
	case EventConclude:
		if endingFirst(text) {
			return "suggested ending, " + text, true
		}
		return "suggested concluding: " + text, true
	case EventEnded:
		return firstNonEmpty(e.Crew, "you") + " ended the mission, " + text, true
	case EventCapsuleEnded:
		return "capsule " + capsule + " " + text, true
	case EventTold:
		if text == "" {
			return "typed into " + who + "'s terminal", true
		}
		return "told " + who + ": " + text, true
	case EventResumeWith:
		if rest, ok := strings.CutPrefix(text, "typed in: "); ok {
			return "told " + who + ": " + rest, true
		}
		return "suggested resuming " + who + ": " + text, true
	case EventWaiting:
		return "waiting on " + text, true
	case EventPR, EventChecks:
		return text, true
	case EventBundle: // "b3 for step 5: 2 met, 1 can't tell, in place of b2"
		if e.What != "" { // your verdict of it: "b3 approved", "b3 sent back: …"
			return "you: bundle " + text, true
		}
		return who + " handed over bundle " + text, true
	case EventLimit:
		if e.Crew != "" && e.Crew != "ws" {
			return e.Crew + ": " + text, true
		}
		return text, true
	case EventDispatch:
		return who + " dispatched " + strings.TrimPrefix(text, "by "), true
	case EventDispatchQueued:
		return "next crew queued: " + text, true
	case EventOrchestrateRun:
		return "", false // a run started: what it says or does is what's worth saying
	case EventOrchestrate:
		return "orchestrator: " + text, true
	case EventState:
		return text, true
	case EventStage:
		if strings.HasPrefix(text, "on stage") {
			return who + " " + text, true
		}
		return text, true
	}
	return e.Kind + ": " + text, true
}

// endingFirst reports whether text starts with how a mission ends, as a
// conclusion's summary does ("answered (at Z: 2 of 2 met): …").
func endingFirst(text string) bool {
	for _, e := range Endings {
		if strings.HasPrefix(text, e.Words()) {
			return true
		}
	}
	return false
}

var (
	threadOpened  = regexp.MustCompile(`^(t[0-9]+) (asked|asked for a change|noted) on (.+?): (.*)$`)
	threadTurn    = regexp.MustCompile(`^(t[0-9]+) ([a-z0-9-]+)(?: \(([0-9a-f]+)\))?: (.*)$`)
	threadStatus  = regexp.MustCompile(`^(t[0-9]+) (resolved|reopened)$`)
	threadSuggest = regexp.MustCompile(`^(t[0-9]+) ([a-z0-9-]+) suggests: (.*)$`)
	threadSettled = regexp.MustCompile(`^(t[0-9]+) suggestion (accepted|declined)(?: \(([0-9a-f]+)\))?$`)
	docApproved   = regexp.MustCompile(`^approved (.+) \(version [0-9a-f]+\)$`)
	threadSent    = regexp.MustCompile(`^(t[0-9]+(?:, t[0-9]+)*) sent to ([a-z0-9-]+)$`)
	talkStarted   = regexp.MustCompile(`^(t[0-9]+) talk: (.*)$`)
	talkProposes  = regexp.MustCompile(`^(t[0-9]+) ([a-z0-9-]+) proposes (.*)$`)
)

// threadSentence words a thread event: "t3 asked on x.md: …" (opened, by
// crew or you), "t3 c1: …" (a turn), "t3 resolved" (by you).
func threadSentence(crew, text string) string {
	if g := threadOpened.FindStringSubmatch(text); g != nil {
		return firstNonEmpty(crew, "you") + " " + g[2] + " on " + g[3] + " (" + g[1] + "): " + g[4]
	}
	if g := threadStatus.FindStringSubmatch(text); g != nil {
		return "you " + g[2] + " " + g[1]
	}
	if g := threadSuggest.FindStringSubmatch(text); g != nil {
		return g[2] + " suggested an edit on " + g[1] + ": " + g[3]
	}
	if g := threadSettled.FindStringSubmatch(text); g != nil {
		if g[3] != "" {
			return "you " + g[2] + " the edit on " + g[1] + " (" + g[3] + ")"
		}
		return "you " + g[2] + " the edit on " + g[1]
	}
	if g := docApproved.FindStringSubmatch(text); g != nil {
		return "you approved " + g[1]
	}
	if g := talkStarted.FindStringSubmatch(text); g != nil {
		return "you started conversation " + g[1] + ": " + g[2]
	}
	if g := talkProposes.FindStringSubmatch(text); g != nil {
		return g[2] + " proposed " + g[3] + " in " + g[1]
	}
	if g := threadSent.FindStringSubmatch(text); g != nil {
		return "you sent " + g[1] + " to " + g[2]
	}
	if g := threadTurn.FindStringSubmatch(text); g != nil {
		if g[3] != "" {
			return g[2] + " on " + g[1] + " (" + g[3] + "): " + g[4]
		}
		return g[2] + " on " + g[1] + ": " + g[4]
	}
	return "thread " + text
}
