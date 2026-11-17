package mission

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// What ws is running while it checks a step crew reported done: the
// check, its command, when it started and the last line it printed, or the
// judge's turn. It's kept on the mission as each check begins, so the
// headline can say it from a snapshot, and cleared when the round is
// recorded (RecordChecks).

// Checking is what ws is running while it checks step Step: a check that's
// a command (N, from 1, of the step's Of), or the judge (N 0), reading the
// checks at Judged.
type Checking struct {
	Step int    `json:"step"`
	Crew string `json:"crew"` // whose done it checks
	Of   int    `json:"of"`   // how many checks the step has
	// N is the check running, from 1; 0 while the judge is reading.
	N       int    `json:"n,omitempty"`
	Command string `json:"command,omitempty"`
	// Line is the last line the command has printed so far.
	Line string `json:"line,omitempty"`
	// Judged are the checks the judge judges (from 1): after the
	// commands, or now, with N 0.
	Judged []int `json:"judged,omitempty"`
	// Judge is who's reading them, once the judge has started.
	Judge string `json:"judge,omitempty"`
	// Since is when the check, or the judge, started; Until when it's
	// stopped if it hasn't finished (its timeout).
	Since time.Time `json:"since"`
	Until time.Time `json:"until"`
}

// Judging reports whether it's the judge's turn.
func (c Checking) Judging() bool { return c.N == 0 }

// Timeout is how long it may run before it's stopped.
func (c Checking) Timeout() time.Duration { return c.Until.Sub(c.Since) }

// liveFor reports whether c is what's running for crew's done at step n,
// at now: set for it, and not past when it would have been stopped (a ws
// that died mid-check leaves it set).
func (c *Checking) liveFor(n int, crew string, now time.Time) bool {
	return c != nil && c.Step == n && c.Crew == crew && now.Before(c.Until.Add(time.Minute))
}

// SetChecking records what ws is running now as it checks a step: a check
// that's begun, or the judge.
func (s Store) SetChecking(slug string, c Checking) error {
	_, err := s.Update(slug, func(m *Mission) error {
		m.Checking = &c
		return nil
	})
	return err
}

// CheckingLine records the last line printed by the check that started
// at since, if it's still the one running.
func (s Store) CheckingLine(slug string, since time.Time, line string) error {
	_, err := s.Update(slug, func(m *Mission) error {
		if c := m.Checking; c != nil && c.Since.Equal(since) {
			c.Line = line
		}
		return nil
	})
	return err
}

// ClearChecking clears what's running for step n, if anything is: for a
// round that ended without being recorded.
func (s Store) ClearChecking(slug string, n int) error {
	_, err := s.Update(slug, func(m *Mission) error {
		if m.Checking != nil && m.Checking.Step == n {
			m.Checking = nil
		}
		return nil
	})
	return err
}

// checkingHeadline is ws checking step st, crew id's done: what's running
// now, when it's known, or that it's checking.
func checkingHeadline(st StageState, id string, c *Checking, now time.Time) Headline {
	h := Headline{Court: CourtAgent, Kind: KindChecking, Label: "… checking", By: "ws",
		Text:   fmt.Sprintf("ws is checking step %d: what %s reported done, against what has to be true for it to be done", st.N, id),
		Action: Action{Label: "nothing to do"}}
	if !c.liveFor(st.N, id, now) {
		return h
	}
	h.Since, h.Checking = c.Since, c
	so := fmt.Sprintf("%s so far (stopped at %s)", Elapsed(now.Sub(c.Since)), FormatLimit(c.Timeout()))
	if c.Judging() {
		h.Label = "… judging"
		who := firstNonEmpty(c.Judge, "the judge")
		h.Text = fmt.Sprintf("%s%s is reading %s of step %d: %s", strings.ToUpper(who[:1]), who[1:], CheckNumbers(c.Judged), st.N, so)
		return h
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Check %d of %d for step %d: `%s`, %s", c.N, c.Of, st.N, c.Command, so)
	if c.Line != "" {
		fmt.Fprintf(&b, ". Last line: “%s”", c.Line)
	}
	if len(c.Judged) > 0 {
		fmt.Fprintf(&b, ". The judge reads %s after", CheckNumbers(c.Judged))
	}
	h.Text = b.String()
	return h
}

// Elapsed is how long something has been going, to the second: 45s,
// 1m20s, 1h2m3s.
func Elapsed(d time.Duration) string {
	d = max(d, 0).Round(time.Second)
	if d >= time.Minute && d%time.Minute == 0 {
		return strings.TrimSuffix(d.String(), "0s")
	}
	return d.String()
}

// CheckNumbers names checks by their numbers: "check 2", "checks 2 and
// 3", "checks 2, 4 and 5".
func CheckNumbers(ns []int) string {
	switch len(ns) {
	case 0:
		return "no checks"
	case 1:
		return "check " + strconv.Itoa(ns[0])
	}
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = strconv.Itoa(n)
	}
	return "checks " + strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}
