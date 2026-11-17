package mission

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// Decisions are what was decided on a mission and why: the reasons a new
// agent most needs and that are least often written down. Each is made
// where the deciding happens (by crew, the orchestrator, or you), is about
// the things it bears on, and is never edited: a later decision supersedes
// it, or it's withdrawn. They're kept in the records, a line per decision
// made or withdrawn (records.go).

// Decision states.
const (
	DecisionCurrent    = "current"
	DecisionSuperseded = "superseded"
	DecisionWithdrawn  = "withdrawn"
)

// Decision is one decision, as it stands.
type Decision struct {
	ID    string    `json:"id"` // d1, d2, … per mission
	What  string    `json:"what"`
	Why   string    `json:"why,omitempty"`
	By    string    `json:"by"`             // "you", a crew id, or "orchestrator"
	From  string    `json:"from,omitempty"` // where it was decided: a ref
	About []string  `json:"about,omitempty"`
	At    time.Time `json:"at"`
	// Supersedes is the decision this one replaces.
	Supersedes string `json:"supersedes,omitempty"`

	// Worked out from later lines, never stored on the decision itself.
	State        string     `json:"state"`
	SupersededBy string     `json:"supersededBy,omitempty"`
	Withdrawn    *time.Time `json:"withdrawn,omitempty"`
	WithdrawnWhy string     `json:"withdrawnWhy,omitempty"`
}

// decisionLine is a line of decisions.jsonl: a decision made, or one
// withdrawn.
type decisionLine struct {
	Made     *Decision  `json:"made,omitempty"`
	Withdraw *withdrawn `json:"withdraw,omitempty"`
}

type withdrawn struct {
	ID  string    `json:"id"`
	Why string    `json:"why"`
	By  string    `json:"by"`
	At  time.Time `json:"at"`
}

var decisionShape = regexp.MustCompile(`^d([1-9][0-9]*)$`)

// IsDecisionRef reports whether ref names a decision (d1).
func IsDecisionRef(ref string) bool { return decisionShape.MatchString(ref) }

// Decisions is every decision on the mission, oldest first, with how each
// stands now.
func (s Store) Decisions(slug string) ([]Decision, error) {
	var out []Decision
	err := s.read(func(tx *store.Tx) error {
		var err error
		out, err = s.decisionsIn(tx, slug)
		return err
	})
	return out, err
}

// CurrentDecisions is the decisions that stand, newest first.
func CurrentDecisions(all []Decision) []Decision {
	var out []Decision
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].State == DecisionCurrent {
			out = append(out, all[i])
		}
	}
	return out
}

// FindDecision is one of the mission's decisions by its id.
func FindDecision(all []Decision, id string) (Decision, bool) {
	for _, d := range all {
		if d.ID == id {
			return d, true
		}
	}
	return Decision{}, false
}

// Decide records a decision, with the next id, and logs it. A decision
// that supersedes another needs that one to stand.
func (s Store) Decide(slug string, d Decision) (Decision, error) {
	d.What, d.Why = strings.TrimSpace(d.What), strings.TrimSpace(d.Why)
	if d.What == "" {
		return Decision{}, errors.New("say what was decided")
	}
	if d.By == "" {
		return Decision{}, errors.New("say who decided")
	}
	err := s.write(func(tx *store.Tx) error {
		if _, err := s.loadIn(tx, slug); err != nil {
			return err
		}
		all, err := s.decisionsIn(tx, slug)
		if err != nil {
			return err
		}
		if d.Supersedes != "" {
			old, ok := FindDecision(all, d.Supersedes)
			switch {
			case !ok:
				return fmt.Errorf("%w: no decision %s to supersede", ErrNotFound, d.Supersedes)
			case old.State != DecisionCurrent:
				return fmt.Errorf("%s is %s already: supersede what stands now", old.ID, old.State)
			}
		}
		next := 1
		for _, o := range all {
			if n, _ := strconv.Atoi(strings.TrimPrefix(o.ID, "d")); n >= next {
				next = n + 1
			}
		}
		d.ID, d.At, d.State, d.SupersededBy, d.Withdrawn, d.WithdrawnWhy = "d"+strconv.Itoa(next), now(), "", "", nil, ""
		if err := s.addDecision(tx, slug, decisionLine{Made: &d}); err != nil {
			return err
		}
		text := d.ID + ": " + d.What
		if d.Supersedes != "" {
			text += " (instead of " + d.Supersedes + ")"
		}
		return s.appendLog(tx, slug, event(Event{Kind: EventDecision, Crew: crewOf(d.By), Text: text}))
	})
	if err != nil {
		return Decision{}, err
	}
	d.State = DecisionCurrent
	return d, nil
}

// Withdraw takes a decision back, saying why.
func (s Store) Withdraw(slug, id, by, why string) error {
	why = strings.TrimSpace(why)
	if why == "" {
		return errors.New("say why it's withdrawn")
	}
	return s.write(func(tx *store.Tx) error {
		if _, err := s.loadIn(tx, slug); err != nil {
			return err
		}
		all, err := s.decisionsIn(tx, slug)
		if err != nil {
			return err
		}
		d, ok := FindDecision(all, id)
		switch {
		case !ok:
			return fmt.Errorf("%w: mission %s has no decision %s", ErrNotFound, slug, id)
		case d.State != DecisionCurrent:
			return fmt.Errorf("%s is %s already", id, d.State)
		}
		if err := s.addDecision(tx, slug, decisionLine{Withdraw: &withdrawn{ID: id, Why: why, By: by, At: now()}}); err != nil {
			return err
		}
		return s.appendLog(tx, slug, event(Event{Kind: EventDecision, Crew: crewOf(by), Text: id + " withdrawn: " + why}))
	})
}

// crewOf is who decided, as a log event names them: crew by id, the
// orchestrator by role, the user not at all.
func crewOf(by string) string {
	if by == "you" {
		return ""
	}
	return by
}

// Line is a decision in one line, as agents read it:
// "d1 (you, from t2): Key per tenant. Why: tokens are issued per tenant."
func (d Decision) Line() string {
	who := d.By
	if d.From != "" {
		who += ", from " + d.From
	}
	line := fmt.Sprintf("%s (%s): %s", d.ID, who, d.What)
	if d.Why != "" {
		line += " Why: " + d.Why
	}
	switch d.State {
	case DecisionSuperseded:
		line += " [superseded by " + d.SupersededBy + "]"
	case DecisionWithdrawn:
		line += " [withdrawn: " + d.WithdrawnWhy + "]"
	}
	return line
}

// IsAbout reports whether the decision bears on ref: it's about it, or
// was decided there.
func (d Decision) IsAbout(ref string) bool {
	if d.From == ref {
		return true
	}
	for _, a := range d.About {
		if a == ref {
			return true
		}
	}
	return false
}
