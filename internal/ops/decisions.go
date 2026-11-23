package ops

import (
	"errors"
	"strings"

	"github.com/withworkspace/workspace/internal/mission"
)

// Decisions recorded on a mission as themselves: not the answer to a
// question, or what a thread or a conversation came to (those record one
// where the deciding happened), but one written down where the mission's
// decisions are read. A decision is never edited: one that no longer holds
// is replaced by another, or withdrawn, and both say so in the log.

// Deciding is a decision to record: what, why, what it bears on and where
// it was made (refs ws can show, which the front end checks), and the one
// it replaces.
type Deciding struct {
	What, Why  string
	About      []string
	From       string
	Supersedes string
}

// Decided is a decision as it was recorded.
type Decided struct {
	Slug     string
	Decision mission.Decision
}

func (d Decided) String() string {
	if d.Decision.Supersedes != "" {
		return "Recorded " + d.Decision.ID + ", in place of " + d.Decision.Supersedes + "."
	}
	return "Recorded " + d.Decision.ID + "."
}

// RecordDecision records what was decided on a mission, by whoever's
// asking: one that replaces another needs that one to stand.
func (w *Workspace) RecordDecision(slug string, d Deciding, by Actor) (Decided, error) {
	if strings.TrimSpace(d.What) == "" {
		return Decided{}, invalid("say what was decided")
	}
	if d.Supersedes != "" && !mission.IsDecisionRef(d.Supersedes) {
		return Decided{}, invalid("%q isn't a decision (they're named d1, d2, …)", d.Supersedes)
	}
	rec, err := w.Missions.Decide(slug, mission.Decision{What: d.What, Why: d.Why, By: by.By(), From: d.From, About: d.About, Supersedes: d.Supersedes})
	if err != nil {
		return Decided{}, notStanding(err)
	}
	w.youActed(slug, by)
	return Decided{Slug: slug, Decision: rec}, nil
}

// DecisionWithdrawn is a decision taken back.
type DecisionWithdrawn struct {
	Slug, ID, Why string
}

func (d DecisionWithdrawn) String() string { return "Withdrew " + d.ID + "." }

// WithdrawDecision takes a decision that stands back, saying why.
func (w *Workspace) WithdrawDecision(slug, id, why string, by Actor) (DecisionWithdrawn, error) {
	if !mission.IsDecisionRef(id) {
		return DecisionWithdrawn{}, invalid("%q isn't a decision (they're named d1, d2, …)", id)
	}
	if why = strings.TrimSpace(why); why == "" {
		return DecisionWithdrawn{}, invalid("say why it's withdrawn")
	}
	if err := w.Missions.Withdraw(slug, id, by.By(), why); err != nil {
		return DecisionWithdrawn{}, notStanding(err)
	}
	w.youActed(slug, by)
	return DecisionWithdrawn{Slug: slug, ID: id, Why: why}, nil
}

// notStanding is the store's error for a decision that can't be replaced
// or withdrawn: there's none by that name (not found), or it doesn't
// stand any more, which is the state's to refuse.
func notStanding(err error) error {
	if errors.Is(err, mission.ErrNotFound) {
		return wrap(err)
	}
	return &Refused{Code: "not-standing", Reason: err.Error(), Err: err}
}
