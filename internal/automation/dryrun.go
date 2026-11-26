package automation

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/withworkspace/workspace/internal/events"
)

// A dry run is what an automation would have done over the events kept
// lately, done with nothing reaching outside it: what it would notify,
// the conversations it would start, what its pace would hold back, and
// how long each wait() would have waited, and for what. It's what you
// read before you turn one on that a model wrote: what it does, not how
// it's written.

// DryRun is what a dry run came to.
type DryRun struct {
	// Since is how far back it looked, and Events how many events it
	// looked at.
	Since  time.Time `json:"since"`
	Events int       `json:"events"`
	// Matched is how many of them it's for; Ran how many runs it'd have
	// had (one a settled batch), Held how many at_most would have held,
	// Failed how many would have failed, Waiting how many would be waiting
	// still.
	Matched int `json:"matched"`
	Ran     int `json:"ran"`
	Held    int `json:"held,omitempty"`
	Failed  int `json:"failed,omitempty"`
	Waiting int `json:"waiting,omitempty"`
	// Settling is how many batches are settling still: their events
	// haven't been quiet as long as settle says yet.
	Settling int `json:"settling,omitempty"`
	// Lines are its runs, the latest last: at most DryLines of them.
	Lines []DryLine `json:"lines"`
}

// DryLine is one run a dry run had, or one held back.
type DryLine struct {
	Event string    `json:"event"` // e1203
	At    time.Time `json:"at"`
	Says  string    `json:"says"` // what the event says
	// Batch is how many events settled into it (settle), 0 for one.
	Batch int `json:"batch,omitempty"`
	// Did is what it would have done (notify: …, converse: …), Waited what
	// each wait() came to, Held why it would have been held, Error why it
	// would have failed.
	Did    []string `json:"did,omitempty"`
	Waited []string `json:"waited,omitempty"`
	Held   string   `json:"held,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// DryLines is the most runs a dry run says.
const DryLines = 20

// dryRun is a run's memory, in a dry run: its steps, and what it'd have
// done.
type dryRun struct {
	steps map[int][2]string
	did   []string
}

// DryRun is what a would have done over the events of the last back.
func (e *Engine) DryRun(a *Automation, back time.Duration) (DryRun, error) {
	out := DryRun{Since: e.now().Add(-back)}
	list, err := events.List(e.Root, events.Filter{Since: out.Since})
	if err != nil {
		return out, err
	}
	out.Events = len(list)
	var did *dryRun
	dry := &Engine{Root: e.Root, Now: e.Now, Timeout: e.Timeout, Hooks: Hooks{
		Show: e.Hooks.Show, RefsIn: e.Hooks.RefsIn,
		Notify: func(_ *Automation, text string, refs []string) error {
			did.did = append(did.did, "notify: "+text)
			return nil
		},
		Converse: func(_ *Automation, text string, _ []string) (string, error) {
			did.did = append(did.did, "converse: "+text)
			return "", nil
		},
	}}
	lines := []DryLine{}
	ranAt := map[string][]time.Time{} // each key's runs, for at_most
	runOne := func(ev events.Event, pace Pace, batch []events.Event) {
		line := DryLine{Event: ev.Ref(), At: ev.At, Says: ev.Says()}
		if len(batch) > 1 {
			line.Batch = len(batch)
		}
		key := keyOf(pace, ev)
		if pace.AtMost > 0 {
			n := 0
			for _, at := range ranAt[key] {
				if !at.Before(ev.At.Add(-pace.AtMostIn)) {
					n++
				}
			}
			if n >= pace.AtMost {
				line.Held = fmt.Sprintf("it'd have run %d of at most %d in %s", n, pace.AtMost, shortDuration(pace.AtMostIn))
				if key != "" {
					line.Held += " for " + key
				}
				out.Held++
				lines = append(lines, line)
				return
			}
		}
		ranAt[key] = append(ranAt[key], ev.At)
		did = &dryRun{steps: map[int][2]string{}}
		for {
			r := &run{e: dry, a: a, ev: ev, batch: batch, dry: did}
			err := r.do()
			if !errors.Is(err, errParked) {
				if err != nil {
					line.Error = err.Error()
					out.Failed++
				}
				break
			}
			// Parked at a wait: met by what came after it, timed out, or
			// waiting still.
			n, w, ok := did.waiting()
			if !ok {
				break
			}
			m, err := events.ParseMatch(w.Match)
			if err != nil {
				break
			}
			result := ""
			for _, later := range list {
				if later.N > w.After && !later.At.After(w.Due) && m.Matches(later) {
					b, _ := json.Marshal(later)
					result = string(b)
					line.Waited = append(line.Waited, fmt.Sprintf("waited %s for %s (%s)", shortDuration(later.At.Sub(ev.At).Round(time.Minute)), later.Ref(), later.Says()))
					break
				}
			}
			if result == "" {
				if e.now().Before(w.Due) {
					line.Waited = append(line.Waited, "would be waiting still, for "+w.Match)
					out.Waiting++
					break
				}
				result = "null"
				line.Waited = append(line.Waited, "waited "+shortDuration(w.Due.Sub(ev.At))+" for "+w.Match+", and its time was up")
			}
			did.steps[n] = [2]string{"done", result}
		}
		line.Did = did.did
		out.Ran++
		lines = append(lines, line)
	}

	type group struct {
		pace   Pace
		events []events.Event
	}
	settling := map[string]*group{}
	flush := func(key string) {
		g := settling[key]
		delete(settling, key)
		runOne(g.events[len(g.events)-1], g.pace, g.events)
	}
	for _, ev := range list {
		pace, ok := a.PaceOf(ev)
		if !ok || ev.By == events.ByWorkflow(a.Name) {
			continue
		}
		out.Matched++
		if pace.Settle == 0 {
			runOne(ev, pace, nil)
			continue
		}
		key := keyOf(pace, ev)
		if g := settling[key]; g != nil && ev.At.Sub(g.events[len(g.events)-1].At) >= pace.Settle {
			flush(key)
		}
		if settling[key] == nil {
			settling[key] = &group{pace: pace}
		}
		settling[key].events = append(settling[key].events, ev)
	}
	for key, g := range settling {
		last := g.events[len(g.events)-1]
		if e.now().Sub(last.At) >= g.pace.Settle {
			flush(key)
			continue
		}
		line := DryLine{Event: last.Ref(), At: last.At, Says: last.Says(), Waited: []string{fmt.Sprintf("settling still: it runs once these have been quiet %s", shortDuration(g.pace.Settle))}}
		if len(g.events) > 1 {
			line.Batch = len(g.events)
		}
		out.Settling++
		lines = append(lines, line)
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].At.Before(lines[j].At) })
	if len(lines) > DryLines {
		lines = lines[len(lines)-DryLines:]
	}
	out.Lines = lines
	return out, nil
}

// waiting is the step a dry run's parked at, if it is.
func (d *dryRun) waiting() (int, waitFor, bool) {
	for n, st := range d.steps {
		if st[0] == "waiting" {
			var w waitFor
			if json.Unmarshal([]byte(st[1]), &w) == nil {
				return n, w, true
			}
		}
	}
	return 0, waitFor{}, false
}

// Proposed is an automation as a conversation proposes it, for the card:
// what it's called, the user's words, when and what it may do, and what
// it would have done lately. Its script is the proposal's text.
type Proposed struct {
	Name   string   `json:"name"`
	About  string   `json:"about,omitempty"`
	When   string   `json:"when"`
	May    []string `json:"may"`
	DryRun DryRun   `json:"dryRun"`
	// Replaces is whether there's one by its name already, which accepting
	// it replaces.
	Replaces bool `json:"replaces,omitempty"`
}
