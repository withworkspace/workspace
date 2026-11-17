package mission

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Endings. A mission ends at Z, or earlier with a reason, in one of four
// ways: landed (at Z, its code merged or landed), answered (a finding and
// no code), handed on (its work raised as other missions) or dropped. Z is
// checked when it ends, each part met or not, and how. The orchestrator
// (or crew) suggests the ending (Mission.Conclude); it's yours to confirm
// (Mission.Ending). Archiving stays, for putting a mission out of sight. A
// capsule can end alone too: handed on to another mission, which takes it,
// or dropped (EndedCapsule).

// Ending is how a mission, or a capsule alone, ended.
type Ending string

const (
	EndLanded   Ending = "landed"    // at Z, its code merged or landed
	EndAnswered Ending = "answered"  // a finding, and no code: at Z, or stopping early with an answer
	EndHandedOn Ending = "handed-on" // its work raised as other missions, which take it on
	EndDropped  Ending = "dropped"   // given up
)

// Endings are the ways a mission ends, in the order they're offered.
var Endings = []Ending{EndLanded, EndAnswered, EndHandedOn, EndDropped}

// Words is the ending as a sentence says it: "handed on".
func (e Ending) Words() string { return strings.ReplaceAll(string(e), "-", " ") }

// ParseEnding reads an ending as it's written: landed, answered, handed
// on (handed-on) or dropped.
func ParseEnding(s string) (Ending, error) {
	t := strings.ToLower(strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '-' || r == '_' }), "-"))
	switch t {
	case "landed", "answered", "handed-on", "dropped":
		return Ending(t), nil
	case "hand-on", "handed":
		return EndHandedOn, nil
	case "drop":
		return EndDropped, nil
	case "":
		return "", fmt.Errorf("say how it ends: %s", endingList())
	}
	return "", fmt.Errorf("%q isn't how a mission ends: it's %s", s, endingList())
}

func endingList() string {
	words := make([]string, len(Endings))
	for i, e := range Endings {
		words[i] = string(e)
	}
	return strings.Join(words[:len(words)-1], ", ") + " or " + words[len(words)-1]
}

// ZCheck is one part of Z, checked when the mission ends: met or not, and
// how (what shows it), or why not.
type ZCheck struct {
	Z   string `toml:"z" json:"z"`
	Met bool   `toml:"met" json:"met"`
	How string `toml:"how,omitempty" json:"how,omitempty"`
}

// Conclusion is how a mission ends: suggested, for you to confirm
// (Mission.Conclude), or confirmed (Mission.Ending).
type Conclusion struct {
	// Ending is "" for a conclusion suggested before missions had endings:
	// it only says the mission looks finished.
	Ending Ending `toml:"ending,omitempty" json:"ending,omitempty"`
	Text   string `toml:"text" json:"text"` // what it came to
	// Z is each part of Z, checked; none for a mission without a Z.
	Z []ZCheck `toml:"z,omitempty" json:"z,omitempty"`
	// To is the missions it was handed on to.
	To []string  `toml:"to,omitempty" json:"to,omitempty"`
	By string    `toml:"by,omitempty" json:"by,omitempty"`
	At time.Time `toml:"at" json:"at"`
}

// Met is how many parts of Z were met, of how many.
func (c Conclusion) Met() (met, of int) {
	for _, z := range c.Z {
		if z.Met {
			met++
		}
	}
	return met, len(c.Z)
}

// AtZ reports whether it ends at Z: every part met. A mission without a Z
// is at Z when it's landed or answered.
func (c Conclusion) AtZ() bool {
	met, of := c.Met()
	if of == 0 {
		return c.Ending == EndLanded || c.Ending == EndAnswered
	}
	return met == of
}

// ZWords is Z as checked, in a few words: "Z met (2 of 2)", "early: Z 1 of
// 3 met"; "" for a mission without a Z.
func (c Conclusion) ZWords() string {
	met, of := c.Met()
	switch {
	case of == 0:
		return ""
	case met == of:
		return fmt.Sprintf("at Z: %d of %d met", met, of)
	}
	return fmt.Sprintf("early: %d of %d of Z met", met, of)
}

// Summary is the conclusion in a line: its ending, Z as checked, and
// what it came to.
func (c Conclusion) Summary() string {
	head := c.Ending.Words()
	if z := c.ZWords(); z != "" {
		head += " (" + z + ")"
	}
	if len(c.To) > 0 {
		head += " to " + strings.Join(c.To, ", ")
	}
	return head + ": " + oneLineOf(c.Text)
}

func oneLineOf(s string) string { return strings.Join(strings.Fields(s), " ") }

// Z is what has to be true for the mission's purpose to be met, the parts
// checked when it ends: its plan's Z, or without a plan, its brief's
// Acceptance (each item of the list under it, or its text as one part).
// None when neither says.
func (m *Mission) Z(brief string) []string {
	if m.Plan != nil {
		return m.Plan.Z
	}
	return BriefAcceptance(brief)
}

var acceptanceHeading = regexp.MustCompile(`(?i)^(#{1,6})\s+acceptance(?:\s+criteria)?\s*:?\s*#*\s*$`)

// BriefAcceptance is what a brief's ## Acceptance says has to be true: the
// items of its list, each with its indented lines, or the section's text
// as one part when it's not a list. It ends at the next heading.
func BriefAcceptance(brief string) []string {
	var items []string
	var prose []string
	in := false
	for _, line := range strings.Split(strings.ReplaceAll(brief, "\r\n", "\n"), "\n") {
		if acceptanceHeading.MatchString(line) && !in {
			in = true
			continue
		}
		if !in {
			continue
		}
		if anyHeading.MatchString(line) {
			break
		}
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if g := stageItem.FindStringSubmatch(line); g != nil && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			items = append(items, strings.TrimSpace(g[1]))
			continue
		}
		if len(items) > 0 {
			items[len(items)-1] += " " + t
			continue
		}
		prose = append(prose, t)
	}
	if len(items) == 0 && len(prose) > 0 {
		return []string{strings.Join(prose, " ")}
	}
	return items
}

var zCheckLine = regexp.MustCompile(`^\s*(?:Z\s*)?(\d+)\s*[:.)-]\s*(.*)$`)

// CheckZ reads Z checked part by part: each of met and unmet says
// "<n>: <how>", n a part of z from 1. Every part must be checked once,
// with how it was met or why not, unless some may go unchecked (they're
// then not met, with nothing said). It refuses, with every problem, what
// isn't so.
func CheckZ(z []string, met, unmet []string, someMay bool) ([]ZCheck, error) {
	if len(z) == 0 {
		if len(met)+len(unmet) > 0 {
			return nil, fmt.Errorf("the mission has no Z to check (no plan, and its brief has no ## Acceptance): leave out --met and --unmet")
		}
		return nil, nil
	}
	out := make([]ZCheck, len(z))
	seen := make([]bool, len(z))
	var problems []string
	read := func(said string, isMet bool) {
		g := zCheckLine.FindStringSubmatch(said)
		if g == nil {
			problems = append(problems, fmt.Sprintf("%q doesn't say which part of Z: write it \"<n>: <how>\", n from 1 to %d", clipPlan(said), len(z)))
			return
		}
		n, _ := strconv.Atoi(g[1])
		how := strings.TrimSpace(g[2])
		switch {
		case n < 1 || n > len(z):
			problems = append(problems, fmt.Sprintf("Z has no part %d: it has %d", n, len(z)))
			return
		case seen[n-1]:
			problems = append(problems, fmt.Sprintf("part %d of Z is checked twice: once, met or not", n))
			return
		case how == "" && isMet:
			problems = append(problems, fmt.Sprintf("part %d of Z is met, but not how: say what shows it (\"%d: go test passes in api/x\")", n, n))
		case how == "":
			problems = append(problems, fmt.Sprintf("part %d of Z isn't met, but not why: say why not", n))
		}
		seen[n-1] = true
		out[n-1] = ZCheck{Z: z[n-1], Met: isMet, How: how}
	}
	for _, s := range met {
		read(s, true)
	}
	for _, s := range unmet {
		read(s, false)
	}
	for i, ok := range seen {
		if ok {
			continue
		}
		if !someMay {
			problems = append(problems, fmt.Sprintf("part %d of Z (%s) isn't checked: --met \"%d: <how>\" or --unmet \"%d: <why not>\"", i+1, clipPlan(z[i]), i+1, i+1))
			continue
		}
		out[i] = ZCheck{Z: z[i]}
	}
	if len(problems) > 0 {
		return nil, &MalformedZ{Problems: problems}
	}
	return out, nil
}

// MalformedZ is Z checked wrongly: what's wrong.
type MalformedZ struct{ Problems []string }

func (m *MalformedZ) Error() string {
	if len(m.Problems) == 1 {
		return "Z isn't checked right: " + m.Problems[0]
	}
	return "Z isn't checked right:\n- " + strings.Join(m.Problems, "\n- ")
}

// EndedCapsule is a capsule that ended alone, before its mission: handed
// on to another mission, which took it, or dropped (its worktree is kept,
// in no mission). It's no longer one of the mission's capsules, and
// doesn't hold the mission open.
type EndedCapsule struct {
	Repo   string    `toml:"repo" json:"repo"`
	Name   string    `toml:"name" json:"name"`
	Branch string    `toml:"branch,omitempty" json:"branch,omitempty"`
	Ending Ending    `toml:"ending" json:"ending"` // EndHandedOn or EndDropped
	To     string    `toml:"to,omitempty" json:"to,omitempty"`
	Why    string    `toml:"why,omitempty" json:"why,omitempty"`
	By     string    `toml:"by,omitempty" json:"by,omitempty"`
	At     time.Time `toml:"at" json:"at"`
}

// Ref is the capsule as repo/name.
func (e EndedCapsule) Ref() string { return e.Repo + "/" + e.Name }

// Words is how it ended, in a line: "api/x handed on to fix-x: …".
func (e EndedCapsule) Words() string {
	s := e.Ref() + " " + e.Ending.Words()
	if e.To != "" {
		s += " to " + e.To
	}
	if e.Why != "" {
		s += ": " + oneLineOf(e.Why)
	}
	return s
}

// HandedOnTo is the missions the mission's capsules were handed on to.
func (m *Mission) HandedOnTo() []string {
	var out []string
	for _, e := range m.EndedCapsules {
		if e.Ending == EndHandedOn && e.To != "" && !slices.Contains(out, e.To) {
			out = append(out, e.To)
		}
	}
	return out
}

// ErrNoCapsule is a capsule that isn't the mission's.
var ErrNoCapsule = fmt.Errorf("not one of the mission's capsules")

// EndCapsule records a capsule ending alone: it leaves the mission's
// capsules for its ended ones.
func (s Store) EndCapsule(slug string, e EndedCapsule) error {
	if _, err := s.Update(slug, func(m *Mission) error {
		i := slices.IndexFunc(m.Capsules, func(c Capsule) bool { return c.Repo == e.Repo && c.Name == e.Name })
		if i < 0 {
			return fmt.Errorf("%s: %w", e.Ref(), ErrNoCapsule)
		}
		if e.Branch == "" {
			e.Branch = m.Capsules[i].Branch
		}
		if e.At.IsZero() {
			e.At = now()
		}
		m.Capsules = slices.Delete(m.Capsules, i, i+1)
		m.EndedCapsules = append(m.EndedCapsules, e)
		return nil
	}); err != nil {
		return err
	}
	text := e.Ending.Words()
	if e.To != "" {
		text += " to " + e.To
	}
	if e.Why != "" {
		text += ": " + oneLineOf(e.Why)
	}
	return s.AppendLog(slug, Event{Kind: EventCapsuleEnded, Repo: e.Repo, Capsule: e.Name, Crew: e.By, Text: text})
}

// SetConclude records an ending suggested, for you to confirm; nil clears
// it.
func (s Store) SetConclude(slug string, c *Conclusion) error {
	if _, err := s.Update(slug, func(m *Mission) error {
		m.Conclude = c
		return nil
	}); err != nil || c == nil {
		return err
	}
	text := oneLineOf(c.Text)
	if c.Ending != "" {
		text = c.Summary()
	}
	return s.AppendLog(slug, Event{Kind: EventConclude, Crew: c.By, Text: text})
}

// End ends the mission as c says: it's at its ending, out of what's in
// play once archived. What it asked of you (a question, crew queued, an
// ending suggested, waiting) goes with it, and parked, it isn't.
func (s Store) End(slug string, c Conclusion) error {
	if c.At.IsZero() {
		c.At = now()
	}
	if _, err := s.Update(slug, func(m *Mission) error {
		m.Ending = &c
		m.Conclude, m.Tasks, m.Dispatch, m.Waiting = nil, nil, nil, nil
		if m.State == Parked {
			m.State = ""
		}
		return nil
	}); err != nil {
		return err
	}
	return s.AppendLog(slug, Event{Kind: EventEnded, Crew: c.By, Text: c.Summary()})
}

// Unend takes back a mission's ending: it's in play again, where it was.
func (s Store) Unend(slug, by string) error {
	var was *Conclusion
	if _, err := s.Update(slug, func(m *Mission) error {
		was, m.Ending = m.Ending, nil
		return nil
	}); err != nil || was == nil {
		return err
	}
	return s.AppendLog(slug, Event{Kind: EventState, Crew: by, Text: "reopened: no longer " + was.Ending.Words()})
}
