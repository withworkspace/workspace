package mission

import "fmt"

// A capsule's PR changes outside ws: checks go red, a reviewer asks for
// changes, it merges. Whoever watches GitHub (the deck, the daemon)
// records what it last saw of each capsule's PR, and a change since is
// logged once and, when someone has to act on it, handed to the
// orchestrator. Whoever sees it first wins: SeePR compares and records
// under the mission's lock.

// SeenPR is what was last seen of a capsule's PR.
type SeenPR struct {
	Number int  `toml:"number" json:"number"`
	Merged bool `toml:"merged,omitempty" json:"merged,omitempty"`
	// Closed: closed without merging.
	Closed bool   `toml:"closed,omitempty" json:"closed,omitempty"`
	Draft  bool   `toml:"draft,omitempty" json:"draft,omitempty"`
	Review string `toml:"review,omitempty" json:"review,omitempty"` // GitHub's review decision
	Checks string `toml:"checks,omitempty" json:"checks,omitempty"` // failure, pending, success
	// Conflict: it doesn't merge cleanly onto its base.
	Conflict bool `toml:"conflict,omitempty" json:"conflict,omitempty"`
	// Said is when the latest thing someone else said on it was, as far
	// as ws has read what's said (at an Open Pull Request step): what's
	// said after it is news.
	Said string `toml:"said,omitempty" json:"said,omitempty"`
}

// SeePR records what's now seen of a capsule's (repo/name) PR. It returns
// what was seen before (nil the first time) and whether anything changed.
func (s Store) SeePR(slug, capsule string, now SeenPR) (*SeenPR, bool, error) {
	var prev *SeenPR
	changed := false
	_, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Capsules {
			c := &m.Capsules[i]
			if c.Repo+"/"+c.Name != capsule {
				continue
			}
			prev = c.SeenPR
			if prev == nil || *prev != now {
				seen := now
				c.SeenPR, changed = &seen, true
			}
		}
		return nil
	})
	return prev, changed, err
}

// What happened to a PR, as an event's kind (Event.What).
const (
	PREventMerged         = "merged"
	PREventClosed         = "closed"
	PREventChecksFailed   = "checks-failed"
	PREventChecksPassed   = "checks-passed"
	PREventConflict       = "conflict"
	PREventChangesAsked   = "changes-requested"
	PREventApproved       = "approved"
	PREventMarkedReady    = "ready"
	PREventSaid           = "said"
	PREventOpened         = "opened"       // opened through ws
	PREventMarkedReadyBy  = "marked-ready" // ws marked it ready, at its step
	PREventReplied        = "replied"      // a reply was posted on it, from the mission
	PREventReplyDrafted   = "reply-drafted"
	PREventReplyDismissed = "reply-dismissed"
)

// PREvent is something that happened to a PR: its kind, in words for the
// log, and whether someone has to act on it.
type PREvent struct {
	What, Text string
	Act        bool
}

// PRChanges says what happened to a PR between two sightings, each in
// words for the log, and whether someone has to act on it: checks gone
// red, a conflict, a review asking for changes, a merge, closing it. A
// first sighting isn't a change; nor is anything about a PR that's merged
// or closed but that.
func PRChanges(ref string, prev *SeenPR, now SeenPR) []PREvent {
	if prev == nil || prev.Number != now.Number {
		return nil
	}
	switch {
	case now.Merged && !prev.Merged:
		return []PREvent{{PREventMerged, ref + " merged", true}}
	case now.Closed && !prev.Closed:
		return []PREvent{{PREventClosed, ref + " was closed without merging", true}}
	case now.Merged || now.Closed:
		return nil
	}
	var out []PREvent
	if now.Checks == "failure" && prev.Checks != "failure" {
		out = append(out, PREvent{PREventChecksFailed, ref + "'s checks are failing", true})
	}
	if now.Checks == "success" && prev.Checks != "success" {
		out = append(out, PREvent{PREventChecksPassed, ref + "'s checks pass", false})
	}
	if now.Conflict && !prev.Conflict {
		out = append(out, PREvent{PREventConflict, ref + " conflicts with its base", true})
	}
	if now.Review == "CHANGES_REQUESTED" && prev.Review != "CHANGES_REQUESTED" {
		out = append(out, PREvent{PREventChangesAsked, ref + ": a reviewer asked for changes", true})
	}
	if now.Review == "APPROVED" && prev.Review != "APPROVED" {
		out = append(out, PREvent{PREventApproved, ref + " was approved", false})
	}
	if prev.Draft && !now.Draft {
		out = append(out, PREvent{PREventMarkedReady, ref + " was marked ready for review", false})
	}
	return out
}

// PRChange is the first of PRChanges, as one change: its words, and
// whether someone has to act on any of them.
func PRChange(ref string, prev *SeenPR, now SeenPR) (text string, act bool) {
	events := PRChanges(ref, prev, now)
	if len(events) == 0 {
		return "", false
	}
	for _, e := range events {
		act = act || e.Act
	}
	return events[0].Text, act
}

// CrewAt says whether crew are at work in a capsule (repo/name): running,
// and not handed back. A change to its PR is theirs to deal with.
func (m *Mission) CrewAt(capsule string) bool {
	for _, c := range m.Crew {
		if c.Capsule == capsule && c.Role == "" && c.Ended == nil && !c.HandedBack(m) {
			return true
		}
	}
	return false
}

// String is how a sighting reads in the log.
func (p SeenPR) String() string {
	s := fmt.Sprintf("#%d", p.Number)
	if p.Merged {
		return s + " merged"
	}
	if p.Review != "" {
		s += " " + p.Review
	}
	if p.Checks != "" {
		s += ", checks " + p.Checks
	}
	return s
}
