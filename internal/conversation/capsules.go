package conversation

import (
	"fmt"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// A conversation's agent, where its shell is sandboxed, tries things in
// capsules it lifts: the conversation's, not a mission's (a mission's are
// its plan's). The conversation keeps which it lifted, and they're put
// away with it: burned, their branches kept, so they come back if it's
// taken up again.

// Capsule is a capsule a conversation's agent lifted.
type Capsule struct {
	Repo   string    `json:"repo"`
	Name   string    `json:"name"`
	Branch string    `json:"branch"`
	At     time.Time `json:"at"`
	// Away is when it was put away, with its conversation: burned, its
	// branch kept, with what wasn't committed committed to it (Kept).
	Away *time.Time `json:"away,omitempty"`
	Kept bool       `json:"kept,omitempty"`
}

// Ref is how a capsule's named: repo/name.
func (c Capsule) Ref() string { return c.Repo + "/" + c.Name }

// awayLine is a capsule put away.
type awayLine struct {
	Repo string    `json:"repo"`
	Name string    `json:"name"`
	Kept bool      `json:"kept,omitempty"`
	At   time.Time `json:"at"`
}

// CanLift is why a conversation's agent can't lift capsules for it, or ""
// if it can: a conversation you'd come back to, and put away. Not an ask
// nobody's kept (it's swept, with nobody to put its capsules away), nor
// one that fills a page or passes over one, nor a thread on a document.
func (c *Conversation) CanLift() string {
	switch {
	case c.Ephemeral:
		return "this is an ask nobody's kept, which is swept once it's sat a while: say in your reply what you'd try in a capsule, and that keeping it as a conversation lets you"
	case c.What != WhatTalk && c.What != WhatPR && c.What != WhatDoc && c.What != WhatThing && c.What != WhatBriefing:
		return "this conversation is about a document or a page as it stands, and holds no capsules: say in your reply what you'd try, and the user can ask it in a conversation of its own"
	}
	return ""
}

// Capsule is the capsule of that repo and name the conversation lifted,
// or nil.
func (c *Conversation) Capsule(repo, name string) *Capsule {
	for i := range c.Capsules {
		if c.Capsules[i].Repo == repo && c.Capsules[i].Name == name {
			return &c.Capsules[i]
		}
	}
	return nil
}

// Lifted are the capsules it lifted that haven't been put away.
func (c *Conversation) Lifted() []Capsule {
	var out []Capsule
	for _, cp := range c.Capsules {
		if cp.Away == nil {
			out = append(out, cp)
		}
	}
	return out
}

// PutAway are the capsules it lifted that were put away with it.
func (c *Conversation) PutAway() []Capsule {
	var out []Capsule
	for _, cp := range c.Capsules {
		if cp.Away != nil {
			out = append(out, cp)
		}
	}
	return out
}

// Lift records a capsule the conversation's agent lifted, or brought back
// (one it had put away: it's lifted again).
func (s Store) Lift(id string, cp Capsule) (*Conversation, error) {
	if cp.Repo == "" || cp.Name == "" {
		return nil, fmt.Errorf("a capsule is a repo's, and has a name")
	}
	cp.Away, cp.Kept = nil, false
	return s.update(id, func(tx *store.Tx, c *Conversation) error {
		if why := c.CanLift(); why != "" {
			return fmt.Errorf("%s holds no capsules: %s", id, why)
		}
		return s.add(tx, c, line{Lifted: &cp})
	})
}

// PutCapsuleAway records that a capsule the conversation lifted was put
// away: kept says what wasn't committed in it was committed to its branch
// first.
func (s Store) PutCapsuleAway(id, repo, name string, kept bool, at time.Time) (*Conversation, error) {
	return s.update(id, func(tx *store.Tx, c *Conversation) error {
		if cp := c.Capsule(repo, name); cp == nil || cp.Away != nil {
			return nil
		}
		return s.add(tx, c, line{Away: &awayLine{Repo: repo, Name: name, Kept: kept, At: at}})
	})
}

// LiftedBy is the conversation of this store whose agent lifted a
// capsule, and hasn't put it away, or nil.
func (s Store) LiftedBy(repo, name string) *Conversation {
	all, err := s.Talks()
	if err != nil {
		return nil
	}
	for _, c := range all {
		if cp := c.Capsule(repo, name); cp != nil && cp.Away == nil {
			return c
		}
	}
	return nil
}
