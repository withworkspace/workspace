package store

import (
	"math"
	"time"
)

// Standing is where a thing stands, in words every kind shares: each
// kind says its own states in its own words (a decision's current or
// superseded, a conversation's open or resolved, a Jira issue's Done), and
// says which of these each is, so what reads the graph can weigh what
// it's found by whether it's still so, without knowing every kind.
type Standing string

const (
	// Lasting is kept true by being kept: a page, a space's document.
	// What it says now is what it says, however long ago it was first
	// written, and it doesn't age.
	Lasting Standing = "lasting"
	// InForce is in force, or still to do: a decision that stands, a
	// mission in play, an open issue. It doesn't age while it's open. On
	// the wire, as a plugin says it of its things, it's open
	// (protocol.StandingOpen), and so it's kept.
	InForce Standing = "open"
	// Passing is of its moment: a conversation's turns, a review's
	// comments, a pull request's words. True when it was said; it ages
	// from when it was last said.
	Passing Standing = "passing"
	// Settled is done: a mission ended, a conversation resolved, an issue
	// closed. It ages from when it settled.
	Settled Standing = "settled"
	// Gone is no longer so: a decision superseded or withdrawn, a mission
	// dropped, an artifact discarded. It ages from when it went, and is
	// found below what's still so even then.
	Gone Standing = "gone"
)

// How what's found weighs by its standing and age. Age halves the part of
// its score that can go every halfLife, down to a floor: a strong match
// that's old is still found, below a like one that's new. Gone is found
// lower still.
const (
	halfLife  = 21 * 24 * time.Hour
	ageFloor  = 0.5
	goneWorth = 0.6
)

// Stands is a thing's standing as of now: what it is, and since when for
// one that's settled or gone (zero when that's not known).
type Stands struct {
	Standing Standing
	Since    time.Time
}

// Under is how a thing stands when it's in something else that stands as
// well (what's said in a mission, as the mission): gone if either is; else
// settled, since the later of the two, if either is; else its own.
func (s Stands) Under(in Stands) Stands {
	switch {
	case s.Standing == Gone:
		return s
	case in.Standing == Gone:
		return in
	case in.Standing == Settled && s.Standing == Settled:
		if in.Since.After(s.Since) {
			return in
		}
		return s
	case in.Standing == Settled && s.Standing != Lasting:
		return in
	}
	return s
}

// Worth is what a thing's score is multiplied by, standing as it does, as
// of now, said last at said: what's passing ages from then, and what's
// settled or gone from when it was, or from then where that's later (a
// word said after its mission ended).
func (s Stands) Worth(said, now time.Time) float64 {
	from := said
	switch s.Standing {
	case "", Lasting, InForce: // a thing whose kind says nothing ages as it always has: not at all
		return 1
	case Settled, Gone:
		if s.Since.After(from) {
			from = s.Since
		}
	}
	w := aged(now.Sub(from))
	if s.Standing == Gone {
		w *= goneWorth
	}
	return w
}

// aged is what age leaves of a score: all of it, halving towards the floor.
func aged(age time.Duration) float64 {
	if age <= 0 {
		return 1
	}
	return ageFloor + (1-ageFloor)*math.Exp2(-float64(age)/float64(halfLife))
}

// standingColumns are the store part's fifth step: a thing's standing, and
// since when (empty where it isn't known). Each store's own step then says
// how what it wrote before stands; a thing none says (empty) doesn't age.
const standingColumns = `
	ALTER TABLE things ADD COLUMN standing TEXT NOT NULL DEFAULT '';
	ALTER TABLE things ADD COLUMN since TEXT NOT NULL DEFAULT '';
`
