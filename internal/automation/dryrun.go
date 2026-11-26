package automation

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
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
	// Steps are how each step of its script would have gone, and Failures
	// where its runs would have failed (line:col, how many), for its
	// outline.
	Steps    []StepCount    `json:"steps,omitempty"`
	Failures map[string]int `json:"failures,omitempty"`
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
	// at is where each step was called from, and its builtin.
	at  map[int][2]string
	did []string
	// tasks are the tasks it asked for, by their (made up) handles.
	tasks map[int64]taskHandle
}

// DryRun is what a would have done over the events of the last back.
func (e *Engine) DryRun(a *Automation, back time.Duration) (DryRun, error) {
	out := DryRun{Since: e.now().Add(-back)}
	list, err := events.List(e.Root, events.Filter{Since: out.Since})
	if err != nil {
		return out, err
	}
	if len(a.Schedules()) > 0 {
		list = withSlots(list, a.slotsIn(out.Since, e.now(), maxSlots), a.Name)
	}
	out.Events = len(list)
	var did *dryRun
	dry := &Engine{Root: e.Root, Now: e.Now, Timeout: e.Timeout, Hooks: Hooks{
		Show: e.Hooks.Show, RefsIn: e.Hooks.RefsIn, Repos: e.Hooks.Repos,
		Notify: func(_ *Automation, text string, refs []string) error {
			did.did = append(did.did, "notify: "+text)
			return nil
		},
		Converse: func(_ *Automation, text string, _, _ []string) (string, error) {
			did.did = append(did.did, "converse: "+text)
			return "", nil
		},
	}}
	lines := []DryLine{}
	ranAt := map[string][]time.Time{} // each key's runs, for at_most
	counts := map[string]*StepCount{}
	var countOrder []string
	counted := func(at, builtin string) *StepCount {
		k := at + " " + builtin
		if counts[k] == nil {
			counts[k] = &StepCount{At: at, Builtin: builtin}
			countOrder = append(countOrder, k)
		}
		return counts[k]
	}
	runOne := func(ev events.Event, pace Pace, batch []events.Event) {
		line := DryLine{Event: ev.Ref(), At: ev.At, Says: ev.Says()}
		if ev.Kind == events.ScheduleDue {
			line.Event = "" // a time it'd have had, not an event kept
		}
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
		did = &dryRun{steps: map[int][2]string{}, at: map[int][2]string{}, tasks: map[int64]taskHandle{}}
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
			// What a task asked for comes to is unknown: a dry run runs
			// nothing.
			dryTask := func(id int64) (Task, bool) {
				h := did.tasks[id]
				return Task{ID: id, Repo: h.repo, Recipe: h.recipe, State: TaskEnded, Status: "unknown: a dry run runs nothing"}, true
			}
			notOurs := func(later events.Event) bool { return later.By == events.ByWorkflow(a.Name) }
			result, done := waitCame(w, list, e.now(), notOurs, dryTask)
			if !done {
				line.Waited = append(line.Waited, "would be waiting still, for "+waitSaid(w, did.tasks))
				out.Waiting++
				break
			}
			line.Waited = append(line.Waited, waitedSaid(w, result, ev, did.tasks))
			did.steps[n] = [2]string{"done", result}
		}
		line.Did = did.did
		for n, st := range did.steps {
			if at, ok := did.at[n]; ok {
				counted(at[0], at[1]).count(st[0], st[1], 1)
			}
		}
		if line.Error != "" {
			if at := failedAt(a.Name, line.Error); at != "" {
				if out.Failures == nil {
					out.Failures = map[string]int{}
				}
				out.Failures[at]++
			}
		}
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
		if why := LoopOf(a.Name, ev.Chain); why != "" {
			out.Held++
			lines = append(lines, DryLine{Event: ev.Ref(), At: ev.At, Says: ev.Says(), Held: why})
			continue
		}
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
	out.Steps = []StepCount{}
	for _, k := range countOrder {
		out.Steps = append(out.Steps, *counts[k])
	}
	return out, nil
}

// maxSlots is the most of a schedule's times a dry run runs it on.
const maxSlots = 200

// withSlots is list with a schedule's times in it, in order, in place of
// any schedule.due kept for name: each numbered as the event before it,
// so a wait() in a run on it looks at what came after.
func withSlots(list, slots []events.Event, name string) []events.Event {
	var out []events.Event
	for _, ev := range list {
		if ev.Kind != events.ScheduleDue || ev.Automation != name {
			out = append(out, ev)
		}
	}
	for i := range slots {
		for _, ev := range out {
			if ev.Kind != events.ScheduleDue && !ev.At.After(slots[i].At) {
				slots[i].N = ev.N
			}
		}
	}
	out = append(out, slots...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

// waitSaid is what a wait waits for, in words.
func waitSaid(w waitFor, tasks map[int64]taskHandle) string {
	if len(w.Items) == 0 {
		return w.Match
	}
	var parts []string
	for _, it := range w.Items {
		if it.Task != 0 {
			h := tasks[it.Task]
			parts = append(parts, h.recipe+" on "+h.repo)
		} else {
			parts = append(parts, it.Match)
		}
	}
	which := "all of "
	if w.Any {
		which = "any of "
	}
	return which + strings.Join(parts, ", ")
}

// waitedSaid is what a wait came to, in words, for a run on ev.
func waitedSaid(w waitFor, result string, ev events.Event, tasks map[int64]taskHandle) string {
	if len(w.Items) > 0 {
		return "waited for " + waitSaid(w, tasks) + " (what a task comes to is unknown in a dry run)"
	}
	if result == "null" {
		return "waited " + shortDuration(w.Due.Sub(ev.At)) + " for " + w.Match + ", and its time was up"
	}
	var later events.Event
	_ = json.Unmarshal([]byte(result), &later)
	return fmt.Sprintf("waited %s for %s (%s)", shortDuration(later.At.Sub(ev.At).Round(time.Minute)), later.Ref(), later.Says())
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
	// Outline is its script, drawn: its dry run's Steps counted on it.
	Outline *Outline `json:"outline,omitempty"`
	// SetsOff are your automations that what it does would set off (the
	// conversations it starts answering), and SetOffBy those whose doings
	// would set it off: where a chain can start, which MaxChain stops.
	SetsOff  []string `json:"setsOff,omitempty"`
	SetOffBy []string `json:"setOffBy,omitempty"`
}

// Linked is which of others what a does would set off, and which of
// them would set a off by what they do (others by the same name aside).
func Linked(a *Automation, others []*Automation) (setsOff, setOffBy []string) {
	for _, b := range others {
		if b.Name == a.Name {
			continue
		}
		if causesFor(a, b) {
			setsOff = append(setsOff, b.Name)
		}
		if causesFor(b, a) {
			setOffBy = append(setOffBy, b.Name)
		}
	}
	return setsOff, setOffBy
}

// causesFor is whether what a does could set b off: the events a
// conversation it starts has (its agent answering, proposing).
func causesFor(a, b *Automation) bool {
	if !a.Allows(MayConverse) {
		return false // a notification is no event
	}
	for _, kind := range []string{events.ConversationAnswered, events.ProposalMade} {
		ev := events.Event{Kind: kind, By: "ws", Conversation: "w0", Chain: []string{a.Name}}
		if b.Matches(ev) {
			return true
		}
	}
	return false
}
