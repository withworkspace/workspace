package mission

import (
	"encoding/json"
	"os"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// EnvRun is the orchestrator's run that the process is part of: its
// agent, and every ws command the agent runs.
const EnvRun = "WS_RUN"

// Event kinds written to log.jsonl.
const (
	EventCreated        = "created"
	EventCapsuleAdded   = "capsule_added"
	EventCapsuleRemove  = "capsule_removed"
	EventCapsuleBurned  = "capsule_burned"
	EventLanded         = "landed" // a capsule's commits pushed straight to the default branch (ws land)
	EventNote           = "note"
	EventCrewStarted    = "crew_started"
	EventCrewEnded      = "crew_ended"
	EventCrewResumed    = "crew_resumed"
	EventCrewAsked      = "crew_asked"
	EventCrewDone       = "crew_done"
	EventArtifact       = "artifact"
	EventFlag           = "flag"
	EventFlagAnswered   = "flag_answered"
	EventConclude       = "conclude_suggested"
	EventEnded          = "ended"         // the mission ended: landed, answered, handed on or dropped
	EventCapsuleEnded   = "capsule_ended" // a capsule ended alone: handed on, or dropped
	EventResumeWith     = "resume_with"
	EventTold           = "told"              // you told running crew something (By you, Crew whom)
	EventOrchestrate    = "orchestrate"       // what an orchestrator's run said, or did
	EventOrchestrateRun = "orchestrate_start" // an orchestrator's run started (its Run)
	EventDispatch       = "dispatch"
	EventDispatchQueued = "dispatch_queued"
	EventState          = "state"
	EventStage          = "stage" // crew joined a stage, or it was accepted
	EventReview         = "review"
	EventReviewReply    = "review_reply" // a review comment answered, resolved or reopened
	EventWaiting        = "waiting"      // the mission waits on something outside it (ws mission wait)
	EventPR             = "pr"           // a capsule's PR changed on GitHub: checks, review, merged
	EventDecision       = "decision"     // a decision made ("d1: …") or withdrawn
	EventThread         = "thread"       // a thread on a document opened, answered, resolved or reopened
	EventChecks         = "checks"       // a Work step's checks, run and judged once its crew reported done
	EventLimit          = "limit"        // a step's time set, or run out
	EventBundle         = "bundle"       // a verification bundle handed over
)

// Event is one line of a mission's log.
type Event struct {
	Time    time.Time `json:"time"`
	Kind    string    `json:"kind"`
	Repo    string    `json:"repo,omitempty"`
	Capsule string    `json:"capsule,omitempty"`
	Crew    string    `json:"crew,omitempty"`
	Text    string    `json:"text,omitempty"`
	// By is who did it, where Crew is whom it's for: who said a message
	// to crew ("you", "orchestrator", "ws"). "" for events that don't say.
	By string `json:"by,omitempty"`
	// What is what kind of thing happened, for an event that has kinds
	// of its own: a PR's (PRChecksFailed…).
	What string `json:"what,omitempty"`
	// Run is the orchestrator's run that caused it, if one did: whatever
	// its agent does through ws is stamped with it ($WS_RUN), so what a
	// run did is read from what it recorded.
	Run string `json:"run,omitempty"`
}

// AppendLog adds an event to the mission's log. Time defaults to now, and
// Crew defaults to $WS_CREW so events from crew are attributed to them.
func (s Store) AppendLog(slug string, e Event) error {
	if !validSlug(slug) {
		return ErrNotFound
	}
	return s.write(func(tx *store.Tx) error { return s.appendLog(tx, slug, event(e)) })
}

// event is e with what it doesn't say filled in: now, and the crew and the
// orchestrator's run this process is, if it's either.
func event(e Event) Event {
	if e.Time.IsZero() {
		e.Time = now()
	}
	if e.Crew == "" {
		e.Crew = os.Getenv(EnvCrew)
	}
	if e.Run == "" {
		e.Run = os.Getenv(EnvRun)
	}
	return e
}

// ReadLog returns the mission's events, oldest first. If limit > 0 only the
// most recent limit events are returned. Malformed lines are skipped.
func (s Store) ReadLog(slug string, limit int) ([]Event, error) {
	q := `SELECT event FROM mission_log WHERE slug = ? ORDER BY n`
	args := []any{slug}
	if limit > 0 {
		q = `SELECT event FROM (SELECT n, event FROM mission_log WHERE slug = ? ORDER BY n DESC LIMIT ?) ORDER BY n`
		args = append(args, limit)
	}
	var events []Event
	err := s.read(func(tx *store.Tx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				return err
			}
			var e Event
			if json.Unmarshal([]byte(line), &e) == nil {
				events = append(events, e)
			}
		}
		return rows.Err()
	})
	return events, err
}
