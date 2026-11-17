package mission

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Work's expansion: what a Work step says has to be true for it to be done
// (its checks), whose crew it carries on from, and how long it has (its
// limit). Once its crew report done, ws runs each check that's a command,
// in the step's capsule, and has an agent who didn't do the work (the
// judge) judge the rest: met, unmet, or can't tell, with what shows it.
// Every round is kept on the step's record. All met, ws passes the step;
// otherwise it's the orchestrator's call, by the process, or yours. ws
// holds the step's clock from when crew start on it, and says once when it
// runs out.

// Check is one thing a Work step says has to be true for it to be done.
type Check struct {
	// Text is the check as written.
	Text string `json:"text"`
	// Command is the command in backticks of a check ws runs, and Expect
	// whether it's to pass or fail (CheckPasses, CheckFails); "" for a
	// check in words, which the judge judges.
	Command string `json:"command,omitempty"`
	Expect  string `json:"expect,omitempty"`
	// Yours is a check of a Verify step that the verifier doesn't settle:
	// it's the user's, with what the verifier left to help.
	Yours bool `json:"yours,omitempty"`
}

// What a command check expects of its command.
const (
	CheckPasses = "passes"
	CheckFails  = "fails"
)

// Run reports whether ws runs the check (it's a command), rather than the
// judge judging it.
func (c Check) Run() bool { return c.Command != "" }

// checkCommand is a check that's a command: `go test ./...` passes.
var checkCommand = regexp.MustCompile("^`([^`]+)`\\s+(passes|fails)\\b\\s*(.*)$")

// parseCheck reads a check: a command, or words for the judge. It says
// what's wrong, or "".
func parseCheck(v string) (Check, string) {
	v = strings.TrimSpace(v)
	m := checkCommand.FindStringSubmatch(v)
	if m == nil {
		return Check{Text: v}, ""
	}
	if rest := strings.Trim(m[3], " .;,"); rest != "" {
		return Check{}, fmt.Sprintf("check: %q: a check ws runs is a command in backticks, then passes or fails, and nothing more (it's run in the step's capsule once its crew report done): say %q as a check of its own", v, rest)
	}
	return Check{Text: v, Command: strings.TrimSpace(m[1]), Expect: m[2]}, ""
}

// What a check came to.
const (
	CheckMet      = "met"
	CheckUnmet    = "unmet"
	CheckCantTell = "can't tell"
	// CheckYours is what a verifier says of a check that's the user's to
	// settle (Check.Yours): it's left for them.
	CheckYours = "yours"
)

// CheckResult is what a check came to in a round, and what shows it.
type CheckResult struct {
	Check  string `toml:"check" json:"check"`
	Result string `toml:"result" json:"result"` // CheckMet, CheckUnmet or CheckCantTell
	// Evidence is what shows it: the end of the command's output and how
	// it exited, or what the judge says.
	Evidence string `toml:"evidence,omitempty" json:"evidence,omitempty"`
	// By is who said: ws (a command it ran), or the judge (its harness and
	// model).
	By string `toml:"by" json:"by"`
}

// CheckRound is a Work step's checks, run and judged once its crew
// reported done.
type CheckRound struct {
	N    int    `toml:"n" json:"n"`       // from 1
	Crew string `toml:"crew" json:"crew"` // whose done it checked
	// Done is which of their dones it checked (Crew.DoneKey).
	Done    string        `toml:"done,omitempty" json:"done,omitempty"`
	Capsule string        `toml:"capsule" json:"capsule"` // where: repo/name
	Commit  string        `toml:"commit,omitempty" json:"commit,omitempty"`
	Results []CheckResult `toml:"results" json:"results"`
	At      time.Time     `toml:"at" json:"at"`
}

// Met reports whether every check was met.
func (r CheckRound) Met() bool {
	for _, c := range r.Results {
		if c.Result != CheckMet {
			return false
		}
	}
	return len(r.Results) > 0
}

// With is the round's checks that came to result.
func (r CheckRound) With(result string) []CheckResult {
	var out []CheckResult
	for _, c := range r.Results {
		if c.Result == result {
			out = append(out, c)
		}
	}
	return out
}

// Summary is the round in a line: "2 of 3 met; unmet: the cause is named".
func (r CheckRound) Summary() string {
	met := len(r.With(CheckMet))
	s := fmt.Sprintf("%d of %d met", met, len(r.Results))
	for _, result := range []string{CheckUnmet, CheckCantTell, CheckYours} {
		var texts []string
		for _, c := range r.With(result) {
			texts = append(texts, clipCheck(c.Check))
		}
		if len(texts) > 0 {
			s += "; " + result + ": " + strings.Join(texts, "; ")
		}
	}
	return s
}

func clipCheck(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 80 {
		return string(r[:79]) + "…"
	}
	return s
}

// LatestChecks is the step's latest round of checks, if it's had one.
func (r *StageRecord) LatestChecks() (CheckRound, bool) {
	if r == nil || len(r.Checks) == 0 {
		return CheckRound{}, false
	}
	return r.Checks[len(r.Checks)-1], true
}

// ChecksMet reports whether the step's latest round of checks was all
// met: what lets the orchestrator pass a step with checks.
func (s StageState) ChecksMet() bool {
	r, ok := s.Record.LatestChecks()
	return ok && r.Met()
}

// RecordChecks keeps a round of step n's checks on its record, numbered,
// and logs it.
func (s Store) RecordChecks(slug string, n int, round CheckRound) (CheckRound, error) {
	brief, _ := s.ReadBrief(slug)
	var kept CheckRound
	if _, err := s.Update(slug, func(m *Mission) error {
		states, _ := m.StageProgress(brief)
		st, err := PickStage(states, n)
		if err != nil {
			return err
		}
		r := recordFor(m, m.Route(brief), st.Stage)
		round.N = len(r.Checks) + 1
		if round.At.IsZero() {
			round.At = now()
		}
		r.Checks = append(r.Checks, round)
		kept = round
		return nil
	}); err != nil {
		return CheckRound{}, err
	}
	return kept, s.AppendLog(slug, Event{Kind: EventChecks, Crew: "ws", Text: fmt.Sprintf("step %d's checks, round %d (%s's done): %s", n, kept.N, kept.Crew, kept.Summary())})
}

// SetVerifyHead records the commit a Verify step's capsule is at as its
// verifier starts: what the verifier's bundle has to be of.
func (s Store) SetVerifyHead(slug string, n int, head string) error {
	brief, _ := s.ReadBrief(slug)
	_, err := s.Update(slug, func(m *Mission) error {
		states, _ := m.StageProgress(brief)
		st, err := PickStage(states, n)
		if err != nil {
			return err
		}
		recordFor(m, m.Route(brief), st.Stage).VerifyHead = head
		return nil
	})
	return err
}

// The limit: how long a Work step's crew have.

// limitPart is a number and its unit, at the start of a limit: 1h, 45m.
var limitPart = regexp.MustCompile(`^(\d+)\s*([A-Za-z]+)\s*`)

// ParseLimit reads how long, from the start of v (45m, 1h, 1h30m, 2
// hours), and the words after it.
func ParseLimit(v string) (time.Duration, string, bool) {
	rest := strings.TrimSpace(v)
	var d time.Duration
	for {
		m := limitPart.FindStringSubmatch(rest)
		if m == nil {
			break
		}
		var unit time.Duration
		switch strings.ToLower(m[2]) {
		case "h", "hr", "hrs", "hour", "hours":
			unit = time.Hour
		case "m", "min", "mins", "minute", "minutes":
			unit = time.Minute
		}
		if unit == 0 {
			break
		}
		n, _ := strconv.Atoi(m[1])
		d += time.Duration(n) * unit
		rest = rest[len(m[0]):]
	}
	if d <= 0 {
		return 0, "", false
	}
	return d, strings.TrimSpace(strings.TrimLeft(rest, ",;.:- ")), true
}

// FormatLimit is a duration as a limit says it: 45m, 1h, 1h30m.
func FormatLimit(d time.Duration) string {
	d = d.Round(time.Minute)
	h, m := int(d.Hours()), int(d.Minutes())%60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

// SetLimit sets when step n's time runs out: from now, d. Shorter only
// (crew setting themselves less time) keeps an earlier deadline. It
// clears the step's being out of time, and says when it runs out.
func (s Store) SetLimit(slug string, n int, d time.Duration, by string, shorterOnly bool) (time.Time, error) {
	brief, _ := s.ReadBrief(slug)
	var deadline time.Time
	if _, err := s.Update(slug, func(m *Mission) error {
		states, _ := m.StageProgress(brief)
		st, err := PickStage(states, n)
		if err != nil {
			return err
		}
		if st.Accepted() {
			return fmt.Errorf("%s %d is passed already", st.Noun(), st.N)
		}
		r := recordFor(m, m.Route(brief), st.Stage)
		deadline = now().Add(d)
		if shorterOnly && r.Deadline != nil && r.Deadline.Before(deadline) {
			deadline = *r.Deadline
			return nil
		}
		r.Deadline, r.OutOfTime = &deadline, nil
		return nil
	}); err != nil {
		return time.Time{}, err
	}
	return deadline, s.AppendLog(slug, Event{Kind: EventLimit, Crew: by, Text: fmt.Sprintf("step %d's time runs out at %s", n, deadline.Local().Format("15:04"))})
}

// OutOfTime is a step whose limit has run out while its crew are on it.
type OutOfTime struct {
	Step     StageState
	Deadline time.Time
}

// RanOutOfTime is the step the mission is at, when its time has run out
// (at now) and ws hasn't said so yet: its crew started on it, and it isn't
// passed.
func (m *Mission) RanOutOfTime(brief string, at time.Time) (OutOfTime, bool) {
	states, _ := m.StageProgress(brief)
	cur, ok := CurrentStage(states)
	if !ok || cur.Record == nil || cur.Record.Deadline == nil || cur.Record.OutOfTime != nil || at.Before(*cur.Record.Deadline) {
		return OutOfTime{}, false
	}
	return OutOfTime{Step: cur, Deadline: *cur.Record.Deadline}, true
}

// MarkOutOfTime records that step n's time has run out (at), once: it
// reports false if it had been said already (or it's been given more time
// since).
func (s Store) MarkOutOfTime(slug string, n int, at time.Time) (bool, error) {
	brief, _ := s.ReadBrief(slug)
	marked := false
	if _, err := s.Update(slug, func(m *Mission) error {
		o, ok := m.RanOutOfTime(brief, at)
		if !ok || o.Step.N != n {
			return nil
		}
		r := recordFor(m, m.Route(brief), o.Step.Stage)
		r.OutOfTime, marked = &at, true
		return nil
	}); err != nil || !marked {
		return false, err
	}
	return true, s.AppendLog(slug, Event{Kind: EventLimit, Crew: "ws", Text: fmt.Sprintf("step %d is out of time", n)})
}

// UncheckedDone is crew who reported done at the step the mission is at, a
// Work step with checks, whose done its checks haven't been run for yet:
// what ws runs them for. The latest such crew, if any.
func (m *Mission) UncheckedDone(brief string) (StageState, string, bool) {
	states, _ := m.StageProgress(brief)
	cur, ok := CurrentStage(states)
	// A Work step with checks, or a Verify step, whose crew are its
	// verifiers: their done is their bundle, handed over.
	settled := ok && cur.Step != nil && (cur.Step.Type == StepWork && len(cur.Step.Checks) > 0 || cur.Step.Type == StepVerify)
	if !settled || cur.Record == nil {
		return StageState{}, "", false
	}
	checked := func(c Crew) bool {
		for _, r := range cur.Record.Checks {
			if r.Crew == c.ID && (r.Done == c.DoneKey() || r.Done == "" && !r.At.Before(*c.StatusAt)) {
				return true
			}
		}
		return false
	}
	for i := len(cur.Record.Crew) - 1; i >= 0; i-- {
		c, ok := m.CrewByID(cur.Record.Crew[i])
		if !ok || c.Role != "" || c.Status != CrewDone || c.StatusAt == nil || checked(c) {
			continue
		}
		return cur, c.ID, true
	}
	return StageState{}, "", false
}

// DoneKey says which of crew's dones it's at, for telling one apart from
// the next: when, and what it said.
func (c Crew) DoneKey() string {
	if c.Status != CrewDone || c.StatusAt == nil {
		return ""
	}
	return c.StatusAt.UTC().Format(time.RFC3339) + "|" + c.Note
}
