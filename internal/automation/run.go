package automation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/store"
)

// Hooks are what an automation's builtins do outside it, given by
// whoever runs it (the daemon): ws's own operations.
type Hooks struct {
	// Notify keeps a notification of the automation's, about refs (the
	// first what it opens).
	Notify func(a *Automation, text string, refs []string) error
	// Failed tells you the automation couldn't run, and why (Notify, with
	// what's said, where it's unset).
	Failed func(a *Automation, why string) error
	// Converse starts a conversation of the workspace's with a first
	// message, about refs, and says its id.
	Converse func(a *Automation, text string, about []string) (string, error)
	// Show is a ref as ws show has it, as JSON's types (nil where it
	// can't be said).
	Show func(ref string) (map[string]any, error)
	// RefsIn are the refs a text names, each its name and its kind.
	RefsIn func(text string) [][2]string
}

// Engine runs a workspace's automations on its events.
type Engine struct {
	Root  string
	Hooks Hooks
	// Timeout is the longest a run may take (10 s unset).
	Timeout time.Duration
	// Now is the clock (time.Now unset).
	Now func() time.Time

	mu    sync.Mutex // one pass at a time
	cache map[string]cached
}

type cached struct {
	mod time.Time
	a   *Automation
	err error
}

func (e *Engine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// Add keeps src as name's automation, on, matched from the next event: it
// says what's wrong with one it can't use. One already called name is
// put in its place, and what's kept of it starts again.
func (e *Engine) Add(name string, src []byte) (*Automation, error) {
	a, err := Check(name, src)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(Dir(e.Root), 0o755); err != nil {
		return nil, err
	}
	err = write(e.Root, func(tx *store.Tx) error {
		latest, err := latestEvent(tx)
		if err != nil {
			return err
		}
		if err := forget(tx, name); err != nil {
			return err
		}
		if err := keep(tx, name, latest, e.now()); err != nil {
			return err
		}
		return os.WriteFile(Path(e.Root, name), src, 0o644)
	})
	return a, err
}

// Remove takes name away: its file, and what's kept of it.
func (e *Engine) Remove(name string) error {
	if _, err := os.Stat(Path(e.Root, name)); err != nil {
		return ErrNotFound
	}
	return write(e.Root, func(tx *store.Tx) error {
		if err := forget(tx, name); err != nil {
			return err
		}
		return os.Remove(Path(e.Root, name))
	})
}

// load is name's automation, read again only when its file's changed.
func (e *Engine) load(name string) (*Automation, error) {
	info, err := os.Stat(Path(e.Root, name))
	if err != nil {
		return nil, err
	}
	if c, ok := e.cache[name]; ok && c.mod.Equal(info.ModTime()) {
		return c.a, c.err
	}
	a, err := Load(e.Root, name)
	if e.cache == nil {
		e.cache = map[string]cached{}
	}
	e.cache[name] = cached{mod: info.ModTime(), a: a, err: err}
	return a, err
}

// passMost is the most events one pass matches an automation against.
const passMost = 500

// RunDue runs each automation that's on on the events after its place,
// those it's for, and moves its place past them: as each one's pace says
// (settling, held back past at_most), with what it parked at a wait()
// picked up where it's met or its time's up. It says what it did, a line
// a run.
func (e *Engine) RunDue() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	states, err := States(e.Root)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(states))
	for name := range states {
		names = append(names, name)
	}
	sort.Strings(names)
	var said []string
	for _, name := range names {
		st := states[name]
		if !st.On {
			continue
		}
		a, err := e.load(name)
		if err != nil {
			e.failed(name, &st, "its file can't be read: "+err.Error())
			continue
		}
		var own map[string]bool
		_ = read(e.Root, func(tx *store.Tx) error {
			own, err = conversationsOf(tx, name)
			return err
		})
		ours := func(ev events.Event) bool { return ev.By == events.ByWorkflow(name) || own[ev.Conversation] }
		said = append(said, e.resume(a, &st, ours)...)
		list, err := events.List(e.Root, events.Filter{After: st.After})
		if err == nil && len(list) > 0 {
			if len(list) > passMost {
				list = list[:passMost]
			}
			for _, ev := range list {
				pace, ok := a.PaceOf(ev)
				if !ok || ours(ev) {
					continue
				}
				if pace.Settle > 0 {
					_ = settle(e.Root, name, keyOf(pace, ev), ev.N, e.now().Add(pace.Settle))
					continue
				}
				if line := e.runOn(a, &st, ev, pace, nil, false); line != "" {
					said = append(said, line)
				}
			}
			// Past every event it's been matched against: those it ran on
			// moved it as they ended.
			last := list[len(list)-1].N
			_ = write(e.Root, func(tx *store.Tx) error { return passed(tx, name, last) })
		}
		// What's settled, run once on its last event, with all of them.
		done, _ := settled(e.Root, name, e.now())
		for _, s := range done {
			batch := EventsNumbered(e.Root, s.events)
			if len(batch) == 0 {
				continue
			}
			ev := batch[len(batch)-1]
			pace, _ := a.PaceOf(ev)
			if line := e.runOn(a, &st, ev, pace, batch, false); line != "" {
				said = append(said, line)
			}
		}
	}
	return said
}

// resume picks up a's runs parked at a wait(): one whose event's come
// (after it was parked), or whose time's up, is run again, its wait given
// the event, or None.
func (e *Engine) resume(a *Automation, st *State, ours func(events.Event) bool) []string {
	waits, err := waitingSteps(e.Root, a.Name)
	if err != nil || len(waits) == 0 {
		return nil
	}
	var said []string
	for _, w := range waits {
		m, err := events.ParseMatch(w.w.Match)
		if err != nil {
			continue
		}
		result := ""
		list, _ := events.List(e.Root, events.Filter{After: w.w.After, Kinds: []string{m.Kind}})
		for _, ev := range list {
			if m.Matches(ev) && !ours(ev) {
				b, _ := json.Marshal(ev)
				result = string(b)
				break
			}
		}
		if result == "" && e.now().Before(w.w.Due) {
			continue // still waiting
		}
		if result == "" {
			result = "null" // its time's up
		}
		if err := keepStep(e.Root, a.Name, w.event, w.step, "wait", "done", result); err != nil {
			continue
		}
		evs := EventsNumbered(e.Root, []int64{w.event})
		if len(evs) == 0 {
			continue
		}
		pace, _ := a.PaceOf(evs[0])
		if line := e.runOn(a, st, evs[0], pace, nil, true); line != "" {
			said = append(said, line)
		}
	}
	return said
}

// keyOf is what pace keeps ev apart by: its per field's value, "" for
// none.
func keyOf(p Pace, ev events.Event) string {
	if p.Per == "" {
		return ""
	}
	return events.FieldOf(ev, p.Per)
}

// EventsNumbered are the events numbered ns, in that order, as kept.
func EventsNumbered(root string, ns []int64) []events.Event {
	if len(ns) == 0 {
		return nil
	}
	lowest := ns[0]
	for _, n := range ns {
		lowest = min(lowest, n)
	}
	list, err := events.List(root, events.Filter{After: lowest - 1})
	if err != nil {
		return nil
	}
	by := map[int64]events.Event{}
	for _, ev := range list {
		by[ev.N] = ev
	}
	var out []events.Event
	for _, n := range ns {
		if ev, ok := by[n]; ok {
			out = append(out, ev)
		}
	}
	return out
}

// runOn runs a on ev (with batch, what settled with it), once: a run
// that's been had already isn't run again, but one stopped part way
// through is (its steps done are given what they came to), and one
// parked at a wait() is when it's picked up (again). One at_most holds
// back is kept as held. It says what it did.
func (e *Engine) runOn(a *Automation, st *State, ev events.Event, pace Pace, batch []events.Event, again bool) string {
	var was string
	_ = read(e.Root, func(tx *store.Tx) error {
		var err error
		was, err = runState(tx, a.Name, ev.N)
		return err
	})
	switch {
	case was == Ran, was == Failed, was == Unsure, was == Held, was == Waiting && !again:
		_ = endRun(e.Root, a.Name, ev.N, was, "", "") // its place, past it
		return ""
	}
	key := keyOf(pace, ev)
	if pace.AtMost > 0 && was == "" {
		if n, err := runsSince(e.Root, a.Name, key, e.now().Add(-pace.AtMostIn)); err == nil && n >= pace.AtMost {
			why := fmt.Sprintf("held back: it's run %d of at most %d in %s", n, pace.AtMost, shortDuration(pace.AtMostIn))
			if key != "" {
				why += " for " + key
			}
			_ = startRun(e.Root, a.Name, ev.N, key, e.now())
			_ = endRun(e.Root, a.Name, ev.N, Held, why, "")
			return fmt.Sprintf("%s on %s: %s", a.Name, ev.Ref(), why)
		}
	}
	if err := startRun(e.Root, a.Name, ev.N, key, e.now()); err != nil {
		return ""
	}
	r := &run{e: e, a: a, ev: ev, batch: batch}
	err := r.do()
	state, why := Ran, ""
	switch {
	case errors.Is(err, errParked):
		state = Waiting
	case errors.Is(err, errUnsure):
		state, why = Unsure, err.Error()
	case err != nil:
		state, why = Failed, err.Error()
	}
	_ = endRun(e.Root, a.Name, ev.N, state, why, strings.Join(r.log, "\n"))
	switch {
	case (state == Ran || state == Waiting) && st.Failed != "":
		_ = setFailed(e.Root, a.Name, "")
		st.Failed = ""
	case state == Failed || state == Unsure:
		e.failed(a.Name, st, why)
	}
	line := fmt.Sprintf("%s on %s: %s", a.Name, ev.Ref(), state)
	if why != "" {
		line += ": " + why
	}
	return line
}

// failed tells you an automation failed, once for what it failed with.
func (e *Engine) failed(name string, st *State, why string) {
	if st.Failed == why {
		return
	}
	_ = setFailed(e.Root, name, why)
	st.Failed = why
	a := &Automation{Name: name}
	switch {
	case e.Hooks.Failed != nil:
		_ = e.Hooks.Failed(a, why)
	case e.Hooks.Notify != nil:
		_ = e.Hooks.Notify(a, fmt.Sprintf("Automation %s couldn't run: %s", name, why), nil)
	}
}

// errUnsure is a run stopped at a step that was started and never said
// it was done.
var errUnsure = errors.New("a step was started and never said it was done: it isn't done again, in case it was")

// errParked is a run parked at a wait(): it's run again when what it waits
// for comes, or its time's up.
var errParked = errors.New("parked at a wait")

// run is one automation run on one event.
type run struct {
	e  *Engine
	a  *Automation
	ev events.Event
	// dry, for a dry run, keeps its steps in memory and does nothing
	// outside it (DryRun): nil for a run that does.
	dry *dryRun
	// batch is what settled with ev (settle), ev last: nil for none.
	batch []events.Event
	steps int
	log   []string
}

// do runs r: its run(e), or, without one, the event notified as it is.
func (r *run) do() error {
	if r.a.run == nil {
		_, err := r.step("notify", func() (string, error) {
			return r.ev.Says(), r.e.Hooks.Notify(r.a, r.ev.Says(), r.ev.Refs())
		})
		return err
	}
	thread := &starlark.Thread{Name: "automation " + r.a.Name, Print: func(_ *starlark.Thread, msg string) { r.log = append(r.log, msg) }}
	thread.SetMaxExecutionSteps(1_000_000)
	timeout := r.e.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	timer := time.AfterFunc(timeout, func() { thread.Cancel("it took longer than " + timeout.String()) })
	defer timer.Stop()
	// Read again for each run, so a run's globals are its own.
	globals, err := starlark.ExecFileOptions(fileOptions, thread, r.a.Name+".star", r.a.Source, r.builtins())
	if err != nil {
		return scriptError(err)
	}
	fn, ok := globals["run"].(starlark.Callable)
	if !ok {
		return fmt.Errorf("it has no run any more")
	}
	if _, err := starlark.Call(thread, fn, starlark.Tuple{batchValue(r.ev, r.batch)}, nil); err != nil {
		for _, sentinel := range []error{errParked, errUnsure} {
			if errors.Is(err, sentinel) {
				return sentinel
			}
			if ee, ok := err.(*starlark.EvalError); ok && errors.Is(ee.Unwrap(), sentinel) {
				return sentinel
			}
		}
		return scriptError(err)
	}
	return nil
}

// read is a read that's kept as a step, so a run that's run again (a
// wait() met, the daemon stopped) is given what it read the first time
// and goes the same way: its result kept as JSON.
func (r *run) read(builtin string, get func() (any, error)) (any, error) {
	result, err := r.step(builtin, func() (string, error) {
		v, err := get()
		if err != nil {
			return "", err
		}
		b, err := json.Marshal(v)
		return string(b), err
	})
	if err != nil {
		return nil, err
	}
	var v any
	return v, json.Unmarshal([]byte(result), &v)
}

// step does what reaches outside the run, once: a step done already (the
// run's being run again) is given what it came to, and one started and
// never done stops the run.
func (r *run) step(builtin string, do func() (string, error)) (string, error) {
	r.steps++
	n := r.steps
	state, result, err := r.stepState(n)
	switch {
	case err != nil:
		return "", err
	case state == "done":
		return result, nil
	case state == "started":
		return "", errUnsure
	}
	if err := r.keepStep(n, builtin, "started", ""); err != nil {
		return "", err
	}
	result, err = do()
	if err != nil {
		_ = r.keepStep(n, builtin, "", "") // not done: it may be again
		return "", err
	}
	return result, r.keepStep(n, builtin, "done", result)
}

// builtins are what a run's script is given.
func (r *run) builtins() starlark.StringDict {
	return starlark.StringDict{
		"event":    starlark.NewBuiltin("event", eventBuiltin),
		"notify":   starlark.NewBuiltin("notify", r.notify),
		"converse": starlark.NewBuiltin("converse", r.converse),
		"log":      starlark.NewBuiltin("log", r.logLine),
		"show":     starlark.NewBuiltin("show", r.show),
		"events":   starlark.NewBuiltin("events", r.events),
		"refs_in":  starlark.NewBuiltin("refs_in", r.refsIn),
		"wait":     starlark.NewBuiltin("wait", r.wait),
	}
}

// notify is notify(text, open = ref, refs = [...]).
func (r *run) notify(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if !r.a.Allows(MayNotify) {
		return nil, fmt.Errorf("notify: it may not (may = [\"notify\"] says it may)")
	}
	var text, open string
	var refs *starlark.List
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "text", &text, "open?", &open, "refs?", &refs); err != nil {
		return nil, err
	}
	all := []string{}
	if open != "" {
		all = append(all, open)
	}
	all = append(all, stringsOf(refs)...)
	// What it said is kept, for its runs on the Automations page.
	_, err := r.step("notify", func() (string, error) { return text, r.e.Hooks.Notify(r.a, text, all) })
	return starlark.None, err
}

// converse is converse(text, about = [...]): a conversation started, its
// id said.
func (r *run) converse(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if !r.a.Allows(MayConverse) {
		return nil, fmt.Errorf("converse: it may not (may = [\"converse\"] says it may)")
	}
	var text string
	var about *starlark.List
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "text", &text, "about?", &about); err != nil {
		return nil, err
	}
	if r.e.Hooks.Converse == nil {
		return nil, fmt.Errorf("converse: there's no conversation to start here")
	}
	id, err := r.step("converse", func() (string, error) { return r.e.Hooks.Converse(r.a, text, stringsOf(about)) })
	return starlark.String(id), err
}

// logLine is log(text): a line in the run's record.
func (r *run) logLine(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var text string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "text", &text); err != nil {
		return nil, err
	}
	r.log = append(r.log, text)
	return starlark.None, nil
}

// show is show(ref): what ws show says of it, or None.
func (r *run) show(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var ref string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "ref", &ref); err != nil {
		return nil, err
	}
	got, err := r.read("show", func() (any, error) {
		if r.e.Hooks.Show == nil {
			return nil, nil
		}
		got, err := r.e.Hooks.Show(ref)
		if err != nil {
			return nil, nil // what can't be said is None
		}
		return got, nil
	})
	if err != nil {
		return nil, err
	}
	return valueOf(got), nil
}

// events is events(kind, since = "7d", **fields): those kept, the oldest
// first, as when matches them.
func (r *run) events(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	since := "7d"
	var rest []starlark.Tuple
	for _, kv := range kwargs {
		if k, _ := starlark.AsString(kv[0]); k == "since" {
			s, ok := starlark.AsString(kv[1])
			if !ok {
				return nil, fmt.Errorf("events: since is a length of time, as \"7d\" or \"2h\"")
			}
			since = s
			continue
		}
		rest = append(rest, kv)
	}
	m, err := eventBuiltin(nil, b, args, rest)
	if err != nil {
		return nil, err
	}
	back, err := lengthOf(since)
	if err != nil {
		return nil, fmt.Errorf("events: since: %v", err)
	}
	match := m.(matchValue).m
	got, err := r.read("events", func() (any, error) {
		list, err := events.List(r.e.Root, events.Filter{Since: r.e.now().Add(-back), Kinds: []string{match.Kind}})
		if err != nil {
			return nil, err
		}
		out := []events.Event{}
		for _, ev := range list {
			if match.Matches(ev) {
				out = append(out, ev)
			}
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	var out []starlark.Value
	for _, ev := range eventsOf(got) {
		out = append(out, eventValue(ev))
	}
	return starlark.NewList(out), nil
}

// eventsOf is events as a read kept them (JSON's types), as events.
func eventsOf(v any) []events.Event {
	b, _ := json.Marshal(v)
	var out []events.Event
	_ = json.Unmarshal(b, &out)
	return out
}

// wait is wait(event(...), timeout = "1d"): the run parks here till an
// event it's for is kept, and is run again from the top once one is (or
// its time's up): this is then the event, or None.
func (r *run) wait(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var which starlark.Value
	timeout := "1d"
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "event", &which, "timeout?", &timeout); err != nil {
		return nil, err
	}
	m, ok := which.(matchValue)
	if !ok {
		return nil, fmt.Errorf("wait takes an event(), as wait(event(\"run.ended\", run = \"deploy\"), timeout = \"2h\")")
	}
	d, err := lengthOf(timeout)
	if err != nil {
		return nil, fmt.Errorf("wait: timeout: %v", err)
	}
	if d > maxWait {
		return nil, fmt.Errorf("wait: timeout: at most %s", shortDuration(maxWait))
	}
	r.steps++
	n := r.steps
	state, result, err := r.stepState(n)
	switch {
	case err != nil:
		return nil, err
	case state == "done":
		if result == "null" || result == "" {
			return starlark.None, nil
		}
		var ev events.Event
		if err := json.Unmarshal([]byte(result), &ev); err != nil {
			return nil, err
		}
		return eventValue(ev), nil
	case state == "waiting":
		return nil, errParked
	}
	// It waits for what's kept after it's reached: after the latest event
	// now, or in a dry run, after the event it's run on, from when that was.
	var after int64
	due := r.e.now().Add(d)
	if r.dry != nil {
		after, due = r.ev.N, r.ev.At.Add(d)
	} else if after, err = latest(r.e.Root); err != nil {
		return nil, err
	}
	kept, _ := json.Marshal(waitFor{Match: m.m.String(), After: after, Due: due})
	if err := r.keepStep(n, "wait", "waiting", string(kept)); err != nil {
		return nil, err
	}
	return nil, errParked
}

// maxWait is the longest a run may wait.
const maxWait = 7 * 24 * time.Hour

// latest is the number of the latest event kept.
func latest(root string) (int64, error) {
	var n int64
	err := read(root, func(tx *store.Tx) error {
		var err error
		n, err = latestEvent(tx)
		return err
	})
	return n, err
}

// refsIn is refs_in(text): the refs a text names, each with .ref and
// .kind.
func (r *run) refsIn(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var text string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "text", &text); err != nil {
		return nil, err
	}
	got, err := r.read("refs_in", func() (any, error) {
		out := [][2]string{}
		if r.e.Hooks.RefsIn != nil {
			out = append(out, r.e.Hooks.RefsIn(text)...)
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	var out []starlark.Value
	pairs, _ := got.([]any)
	for _, p := range pairs {
		f, _ := p.([]any)
		if len(f) == 2 {
			ref, _ := f[0].(string)
			kind, _ := f[1].(string)
			out = append(out, starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{"ref": starlark.String(ref), "kind": starlark.String(kind)}))
		}
	}
	return starlark.NewList(out), nil
}

// lengthOf is a length of time back: 7d, 2h, 90m.
func lengthOf(s string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(s, "d"); ok {
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err == nil && n > 0 {
			return time.Duration(n) * 24 * time.Hour, nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%q isn't a length of time: say it as 7d, 2h or 90m", s)
	}
	return d, nil
}

func stringsOf(l *starlark.List) []string {
	if l == nil {
		return nil
	}
	var out []string
	for i := range l.Len() {
		if s, ok := starlark.AsString(l.Index(i)); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// batchValue is the event a run's run on, with what settled with it as
// e.batch (it alone, for one that didn't settle).
func batchValue(e events.Event, batch []events.Event) starlark.Value {
	if len(batch) == 0 {
		batch = []events.Event{e}
	}
	vs := make([]starlark.Value, len(batch))
	for i, b := range batch {
		vs[i] = eventValue(b)
	}
	return eventValueWith(e, starlark.StringDict{"batch": starlark.NewList(vs)})
}

// eventValue is an event as a script has it: e.kind, e.by.short, e.title…
func eventValue(e events.Event) starlark.Value { return eventValueWith(e, nil) }

func eventValueWith(e events.Event, more starlark.StringDict) starlark.Value {
	var data map[string]any
	b, _ := json.Marshal(e)
	_ = json.Unmarshal(b, &data)
	d := starlark.StringDict{}
	for k, v := range data {
		d[snake(k)] = valueOf(v)
	}
	for _, k := range []string{"repo", "capsule", "mission", "pr", "doc", "thing", "thing_kind", "run", "status", "title", "text", "branch", "conversation", "origin", "proposal_kind"} {
		if _, ok := d[k]; !ok {
			d[k] = starlark.String("")
		}
	}
	d["ref"] = starlark.String(e.Ref())
	d["by"] = byValue(e.By)
	d["about"] = listOf(append([]string(nil), e.About...))
	d["refs"] = listOf(e.Refs())
	d["at"] = starlark.String(e.At.Format(time.RFC3339))
	if _, ok := d["paths"]; !ok {
		d["paths"] = starlark.NewList(nil)
	}
	if _, ok := d["changes"]; !ok {
		d["changes"] = starlark.NewList(nil)
	}
	if _, ok := d["commits"]; !ok {
		d["commits"] = starlark.NewList(nil)
	}
	for k, v := range more {
		d[k] = v
	}
	return starlarkstruct.FromStringDict(starlarkstruct.Default, d)
}

// byValue is who did it: .who as an event says it, .key a person's key,
// .short what they're called in a sentence.
func byValue(by string) starlark.Value {
	key, person := events.PersonOf(by)
	short := by
	switch {
	case by == events.ByYou:
		short = "you"
	case person:
		first, _, _ := strings.Cut(key, "-")
		short = strings.ToUpper(first[:1]) + first[1:]
	}
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"who": starlark.String(by), "key": starlark.String(key), "short": starlark.String(short),
	})
}

func listOf(ss []string) *starlark.List {
	vs := make([]starlark.Value, len(ss))
	for i, s := range ss {
		vs[i] = starlark.String(s)
	}
	return starlark.NewList(vs)
}

// valueOf is JSON's value as a script has it: an object a struct, its
// keys as snake_case fields.
func valueOf(v any) starlark.Value {
	switch x := v.(type) {
	case nil:
		return starlark.None
	case bool:
		return starlark.Bool(x)
	case float64:
		if x == float64(int64(x)) {
			return starlark.MakeInt64(int64(x))
		}
		return starlark.Float(x)
	case string:
		return starlark.String(x)
	case []any:
		vs := make([]starlark.Value, len(x))
		for i, e := range x {
			vs[i] = valueOf(e)
		}
		return starlark.NewList(vs)
	case map[string]any:
		d := starlark.StringDict{}
		for k, e := range x {
			d[snake(k)] = valueOf(e)
		}
		return starlarkstruct.FromStringDict(starlarkstruct.Default, d)
	}
	return starlark.String(fmt.Sprint(v))
}

// snake is a JSON key as a field's name: thingKind is thing_kind.
func snake(k string) string {
	var b strings.Builder
	for i, c := range k {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			c += 'a' - 'A'
		}
		if c == '-' {
			c = '_'
		}
		b.WriteRune(c)
	}
	return b.String()
}

// stepState is a step of r as kept: in the records, or a dry run's memory.
func (r *run) stepState(n int) (state, result string, err error) {
	if r.dry != nil {
		st := r.dry.steps[n]
		return st[0], st[1], nil
	}
	return stepState(r.e.Root, r.a.Name, r.ev.N, n)
}

// keepStep keeps a step of r as it stands.
func (r *run) keepStep(n int, builtin, state, result string) error {
	if r.dry != nil {
		if state == "" {
			delete(r.dry.steps, n)
		} else {
			r.dry.steps[n] = [2]string{state, result}
		}
		return nil
	}
	return keepStep(r.e.Root, r.a.Name, r.ev.N, n, builtin, state, result)
}
