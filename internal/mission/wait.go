package mission

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Wait's expansion: what a Wait step waits on, and how ws knows it's
// happened. A Wait says who it's on. On you (something you do yourself),
// it's in your court: ws asks you, and you mark it done. On anyone else,
// it's outside. Either way nothing passes it by itself unless it has
// checks, commands ws runs every so often until they're met (the step is
// watched): without one, nobody is watching, and it's passed when someone
// says it's happened. A wait on someone else that nobody's watching comes
// to your court once it's waited a while (StaleWait), to ask whether it
// has.

// WaitDone is the answer that marks a Wait on you done: alone, or with
// what you did after it ("Done: tagged v0.2.0"), which is kept for the
// crew after.
const WaitDone = "Done"

// StaleWait is how long a wait on someone else that nobody's watching
// waits before it's asked of you: has it happened?
var StaleWait = 24 * time.Hour

// Watch is a Wait step being waited for: since when, and what ws's last
// look at its checks came to.
type Watch struct {
	// Since is when the mission reached the step.
	Since time.Time `toml:"since" json:"since"`
	// Looked is when ws last ran its checks, Looks how many times it has,
	// and Results what the last look came to, a result per check.
	Looked  *time.Time    `toml:"looked,omitempty" json:"looked,omitempty"`
	Looks   int           `toml:"looks,omitempty" json:"looks,omitempty"`
	Results []CheckResult `toml:"results,omitempty" json:"results,omitempty"`
}

// Met reports whether its last look found every check met.
func (w *Watch) Met() bool {
	if w == nil || len(w.Results) == 0 {
		return false
	}
	for _, r := range w.Results {
		if r.Result != CheckMet {
			return false
		}
	}
	return true
}

// Summary is what its last look came to, in a few words: "not yet" (and
// which check, of several), "couldn't tell", or "" before any look.
func (w *Watch) Summary() string {
	if w == nil || w.Looked == nil {
		return ""
	}
	if w.Met() {
		return "met"
	}
	var unmet, cant int
	for _, r := range w.Results {
		switch r.Result {
		case CheckUnmet:
			unmet++
		case CheckCantTell:
			cant++
		}
	}
	switch {
	case cant > 0 && unmet == 0:
		return "could not tell"
	case len(w.Results) > 1:
		return fmt.Sprintf("not yet (%d of %d met)", len(w.Results)-unmet-cant, len(w.Results))
	}
	return "not yet"
}

// How often ws looks at a watched step's checks: often at first, when
// what it waits for is likeliest to be about to happen, then less.
const (
	watchEvery     = time.Minute
	watchLater     = 5 * time.Minute
	watchLaterFrom = 15 * time.Minute
	watchLong      = 15 * time.Minute
	watchLongFrom  = 2 * time.Hour
)

// NextLook is when ws next runs a watched step's checks: at once when it
// hasn't, then every minute, every five once it's waited a quarter of an
// hour, and every fifteen once it's waited two.
func (w *Watch) NextLook() time.Time {
	if w == nil || w.Looked == nil {
		return time.Time{}
	}
	every := watchEvery
	switch waited := w.Looked.Sub(w.Since); {
	case waited >= watchLongFrom:
		every = watchLong
	case waited >= watchLaterFrom:
		every = watchLater
	}
	return w.Looked.Add(every)
}

// Watch is the step's watch: since when it's been waited for, and what
// the last look at its checks came to. Nil before the mission reaches it.
func (s StageState) Watch() *Watch {
	if s.Record == nil {
		return nil
	}
	return s.Record.Watch
}

// ReachWait records that the mission has reached step n, a Wait: what it
// waits for is waited for from now.
func (s Store) ReachWait(slug string, n int) error {
	brief, _ := s.ReadBrief(slug)
	_, err := s.Update(slug, func(m *Mission) error {
		states, _ := m.StageProgress(brief)
		st, err := PickStage(states, n)
		if err != nil {
			return err
		}
		r := recordFor(m, m.Route(brief), st.Stage)
		if r.Watch == nil {
			r.Watch = &Watch{Since: now()}
		}
		return nil
	})
	return err
}

// RecordLook keeps what a look at step n's checks came to. It's logged
// when it's the first, or came to something else than the look before:
// a step watched for a day isn't a thousand lines of the log.
func (s Store) RecordLook(slug string, n int, results []CheckResult) error {
	brief, _ := s.ReadBrief(slug)
	var said string
	if _, err := s.Update(slug, func(m *Mission) error {
		states, _ := m.StageProgress(brief)
		st, err := PickStage(states, n)
		if err != nil {
			return err
		}
		r := recordFor(m, m.Route(brief), st.Stage)
		at := now()
		if r.Watch == nil {
			r.Watch = &Watch{Since: at}
		}
		was := r.Watch.Summary()
		r.Watch.Looked, r.Watch.Results = &at, results
		r.Watch.Looks++
		if is := r.Watch.Summary(); is != was {
			said = fmt.Sprintf("step %d, watched: %s (%s)", n, is, lookList(results))
		}
		return nil
	}); err != nil {
		return err
	}
	if said == "" {
		return nil
	}
	return s.AppendLog(slug, Event{Kind: EventChecks, Crew: "ws", Text: said})
}

// lookList is a look's results, a clause each: "`gh release view v1` failed
// (exit 1): unmet".
func lookList(results []CheckResult) string {
	var out []string
	for _, r := range results {
		how, _, _ := strings.Cut(r.Evidence, "\n")
		out = append(out, strings.TrimSuffix(clipCheck(r.Check)+": "+strings.TrimSuffix(how, ":"), ": "))
	}
	return strings.Join(out, "; ")
}

// waitDone reads an answer that marks a Wait done: Done, alone or with
// what was done after it.
var waitDone = regexp.MustCompile(`(?is)^done\b[\s:;,.!\-–—]*(.*)$`)

// SaysDone reports whether an answer marks a Wait done, and what it says
// was done, if anything: "Done", "done: tagged v0.2.0".
func SaysDone(answer string) (note string, ok bool) {
	m := waitDone.FindStringSubmatch(strings.TrimSpace(answer))
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}
