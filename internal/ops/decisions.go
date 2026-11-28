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
	// HeldTo is what the work is held to, for crew to hold the decision
	// they've made against as they make it: none for anyone else's.
	HeldTo HeldTo
}

// HeldTo is what crew's work is held to: their step's checks, the checks
// of the Work steps after it (a decision made now can rule one out), and
// the user's decisions that stand, each as written.
type HeldTo struct {
	Step   int
	Checks []string
	Later  []LaterChecks
	Yours  []string // "d1: what"
}

// LaterChecks are a later Work step's checks.
type LaterChecks struct {
	Step   int
	Checks []string
}

// Any is whether there's anything the work is held to.
func (h HeldTo) Any() bool { return len(h.Checks) > 0 || len(h.Later) > 0 || len(h.Yours) > 0 }

func (d Decided) String() string {
	if d.Decision.Supersedes != "" {
		return "Recorded " + d.Decision.ID + ", replacing " + d.Decision.Supersedes + "."
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
		return Decided{}, invalid("%q is not a decision (d1, d2, …)", d.Supersedes)
	}
	rec, err := w.Missions.Decide(slug, mission.Decision{What: d.What, Why: d.Why, By: by.By(), From: d.From, About: d.About, Supersedes: d.Supersedes})
	if err != nil {
		return Decided{}, notStanding(err)
	}
	w.youActed(slug, by)
	out := Decided{Slug: slug, Decision: rec}
	if by.Kind == Crew {
		out.HeldTo = w.heldTo(slug, rec)
	}
	return out, nil
}

// heldTo is what crew's decision d is to be held against: what's to
// stay true of the work at d's step and after it, and what the user has
// decided. A decision against a check otherwise comes to light when the
// check's judged, a round of work later; one made at an early step can
// rule out a later step's check.
func (w *Workspace) heldTo(slug string, d mission.Decision) HeldTo {
	h := HeldTo{Step: d.Step}
	if all, err := w.Missions.Decisions(slug); err == nil {
		for _, o := range all {
			if o.State == mission.DecisionCurrent && o.By == AsYou.By() {
				h.Yours = append(h.Yours, o.ID+": "+oneLine(o.What))
			}
		}
	}
	m, err := w.Missions.Load(slug)
	if err != nil || d.Step == 0 {
		return h
	}
	brief, _ := w.Missions.ReadBrief(slug)
	states, _ := m.StageProgress(brief)
	for _, st := range states {
		if st.Step == nil || st.N < d.Step || len(st.Step.Checks) == 0 {
			continue
		}
		var checks []string
		for _, c := range st.Step.Checks {
			checks = append(checks, c.Text)
		}
		switch {
		case st.N == d.Step:
			h.Checks = checks
		case st.Step.Type == mission.StepWork:
			h.Later = append(h.Later, LaterChecks{Step: st.N, Checks: checks})
		}
	}
	return h
}

// DecisionWithdrawn is a decision taken back.
type DecisionWithdrawn struct {
	Slug, ID, Why string
}

func (d DecisionWithdrawn) String() string { return "Withdrew " + d.ID + "." }

// WithdrawDecision takes a decision that stands back, saying why.
func (w *Workspace) WithdrawDecision(slug, id, why string, by Actor) (DecisionWithdrawn, error) {
	if !mission.IsDecisionRef(id) {
		return DecisionWithdrawn{}, invalid("%q is not a decision (d1, d2, …)", id)
	}
	if why = strings.TrimSpace(why); why == "" {
		return DecisionWithdrawn{}, invalid("give a reason")
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
