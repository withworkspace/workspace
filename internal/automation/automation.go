// Package automation is what you've asked ws to do when something
// happens: a rule, written as a small Starlark file, run on the
// workspace's events (internal/events). Notify me if Brendan pushes a new
// drawing; when CI fails on my pull request, have a look and tell me why.
//
// An automation is .ws/automations/<name>.star, on this machine:
//
//	# Notify me if Brendan pushes a new drawing.
//	when = event("thing.added", kind = "sketch", by = "brendan")
//	may = ["notify"]
//
//	def run(e):
//	    notify("%s drew %s" % (e.by.short, e.title), open = e.thing)
//
// when is which events it's for (event(), as events.ParseMatch reads
// them; a list for any of several), matched by ws without running the
// script. may is what it may do. run is called once for each event that
// matches, with it; without run, the event's notified as it is.
//
// A script can do nothing by itself: Starlark has no files, network or
// clock. What it does is through its builtins, and what reaches outside
// it (notify, converse) only where may says. Each of those is a step,
// kept before it's done and after (store.go), so a run that's run again
// (the daemon stopped mid-run) doesn't do it twice. Each automation keeps
// its place in the events: one turned on runs from the next event, and
// one the daemon was down for catches up.
package automation

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/withworkspace/workspace/internal/events"
)

// Mays are what an automation may do, as may names them.
const (
	MayNotify   = "notify"
	MayConverse = "converse"
	// MayRun is starting a repo's task (run_task): this machine's time.
	MayRun = "run"
)

// Mays is every may there is.
var Mays = []string{MayNotify, MayConverse, MayRun, MayOutput, MayAsk, MayPropose}

// Automation is one automation, as its file says it.
type Automation struct {
	Name string
	// About is what it's for, in your words: its first comment's lines.
	About string
	When  []events.Match
	// Paces are how often each of When runs it (settle, at_most, per).
	Paces []Pace
	May   []string
	// Source is its file, as read.
	Source []byte
	// schedules are When's every()s, by place (nil for an event()).
	schedules []*Schedule
	// actions are When's action()s, by place (nil for the rest).
	actions []*Action
	// run is its run, nil for none (the event's notified as it is).
	run starlark.Callable
}

// Dir is where root's automations are.
func Dir(root string) string { return filepath.Join(root, ".ws", "automations") }

// Path is name's file in root.
func Path(root, name string) string { return filepath.Join(Dir(root), name+".star") }

// nameRule is what an automation may be called: its file's name.
var nameRule = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// ValidName is whether name may be an automation's.
func ValidName(name string) bool { return nameRule.MatchString(name) }

// Allows is whether a may do what.
func (a *Automation) Allows(what string) bool {
	return slices.Contains(a.May, what) || (what == MayNotify && a.run == nil)
}

// Matches is whether e is one a is for.
func (a *Automation) Matches(e events.Event) bool {
	_, ok := a.PaceOf(e)
	return ok
}

// PaceOf is how often the first of When that e matches runs a: false for
// none.
func (a *Automation) PaceOf(e events.Event) (Pace, bool) {
	for i, m := range a.When {
		if m.Matches(e) {
			if i < len(a.Paces) {
				return a.Paces[i], true
			}
			return Pace{}, true
		}
	}
	return Pace{}, false
}

// WhenSaid is when, as written.
func (a *Automation) WhenSaid() string {
	var out []string
	for i, m := range a.When {
		said := m.String()
		if s := a.scheduleAt(i); s != nil {
			said = "every " + s.Said
		}
		if i < len(a.actions) && a.actions[i] != nil {
			x := a.actions[i]
			said = "you run " + strconv.Quote(x.Title)
			if x.On != "" {
				said += " on a " + x.On
				for _, k := range sortedWhere(x.Where) {
					said += " " + k + "=" + strings.Join(x.Where[k], ",")
				}
			}
		}
		if i < len(a.Paces) {
			if p := a.Paces[i].String(); p != "" {
				said += " (" + p + ")"
			}
		}
		out = append(out, said)
	}
	return strings.Join(out, " · ")
}

// fileOptions are the Starlark a script is read as: what the language
// is, and no more (no while, no recursion, no top-level control flow).
var fileOptions = &syntax.FileOptions{}

// Load reads name's file in root.
func Load(root, name string) (*Automation, error) {
	src, err := os.ReadFile(Path(root, name))
	if err != nil {
		return nil, err
	}
	return Check(name, src)
}

// Check reads an automation from src, as name: its when, may and run, by
// running only the file's top level. It says what's wrong with one it
// can't use.
func Check(name string, src []byte) (*Automation, error) {
	if !ValidName(name) {
		return nil, fmt.Errorf("%q is not a valid automation name: use lower-case letters, digits and hyphens", name)
	}
	thread := &starlark.Thread{Name: "automation " + name}
	thread.SetMaxExecutionSteps(100_000)
	globals, err := starlark.ExecFileOptions(fileOptions, thread, name+".star", src, checkBuiltins())
	if err != nil {
		return nil, scriptError(err)
	}
	a := &Automation{Name: name, Source: src, About: aboutOf(src)}
	when, ok := globals["when"]
	if !ok {
		return nil, fmt.Errorf("no when: say which events, as when = event(\"pr.merged\", pr = \"api#212\")")
	}
	ms, err := matchesOf(when)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		if m.schedule != nil {
			m.m.Fields = []events.MatchField{{Key: "automation", Value: name}}
		}
		if m.action != nil {
			for _, x := range a.Actions() {
				if x.Title == m.action.Title {
					return nil, fmt.Errorf("two actions called %q: rename one", x.Title)
				}
			}
			m.m.Fields = []events.MatchField{{Key: "automation", Value: name}, {Key: "action", Value: m.action.Title}}
		}
		a.When, a.Paces, a.schedules = append(a.When, m.m), append(a.Paces, m.pace), append(a.schedules, m.schedule)
		a.actions = append(a.actions, m.action)
	}
	if may, ok := globals["may"]; ok {
		if a.May, err = maysOf(may); err != nil {
			return nil, err
		}
	}
	if run, ok := globals["run"]; ok {
		fn, ok := run.(*starlark.Function)
		if !ok {
			return nil, fmt.Errorf("run is a %s: write def run(e), called with each event", run.Type())
		}
		if fn.NumParams() != 1 {
			return nil, fmt.Errorf("run takes %d arguments: write def run(e), called with each event", fn.NumParams())
		}
		a.run = fn
	} else if len(a.May) > 0 && !slices.Equal(a.May, []string{MayNotify}) {
		return nil, fmt.Errorf("may %s, but has no run: without def run(e), each event is only notified", strings.Join(a.May, ", "))
	}
	return a, nil
}

// List is root's automations, by name, with what's wrong with any that
// can't be read.
func List(root string) ([]*Automation, map[string]error) {
	entries, _ := os.ReadDir(Dir(root))
	var out []*Automation
	bad := map[string]error{}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".star")
		if !ok || e.IsDir() {
			continue
		}
		a, err := Load(root, name)
		if err != nil {
			bad[name] = err
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, bad
}

// aboutOf is a script's first comment: its lines, joined.
func aboutOf(src []byte) string {
	var out []string
	for _, line := range bytes.Split(src, []byte("\n")) {
		t := strings.TrimSpace(string(line))
		if !strings.HasPrefix(t, "#") {
			if len(out) > 0 || t != "" {
				break
			}
			continue
		}
		out = append(out, strings.TrimSpace(strings.TrimPrefix(t, "#")))
	}
	return strings.Join(out, " ")
}

// Pace is how often an event() runs an automation: settled (once its
// events have been quiet for Settle, with all of them), at most AtMost
// times each AtMostIn, each kept apart by Per (one of the event's
// fields, as a Match names them: pr, thing, repo…).
type Pace struct {
	Settle   time.Duration
	AtMost   int
	AtMostIn time.Duration
	Per      string
}

// String is the Pace as event() says it: settle=10m at_most=1/h per=pr.
func (p Pace) String() string {
	var out []string
	if p.Settle > 0 {
		out = append(out, "settle="+shortDuration(p.Settle))
	}
	if p.AtMost > 0 {
		out = append(out, fmt.Sprintf("at_most=%d/%s", p.AtMost, shortDuration(p.AtMostIn)))
	}
	if p.Per != "" {
		out = append(out, "per="+p.Per)
	}
	return strings.Join(out, " ")
}

// shortDuration is d as a script writes it: 10m, 1h, 2h30m.
func shortDuration(d time.Duration) string {
	s := d.String()
	s = strings.TrimSuffix(strings.TrimSuffix(s, "0s"), "0m")
	if strings.HasSuffix(s, "h0") {
		s = strings.TrimSuffix(s, "0")
	}
	return s
}

// matchValue is an event() as a script holds it.
type matchValue struct {
	m    events.Match
	pace Pace
	// schedule is an every()'s: its own schedule.due, by the clock.
	schedule *Schedule
	// action is an action()'s: its own action.run, by you.
	action *Action
}

func (v matchValue) String() string {
	if v.schedule != nil {
		return "every(" + strconv.Quote(v.schedule.Said) + ")"
	}
	if v.action != nil {
		return "action(" + strconv.Quote(v.action.Title) + ")"
	}
	return "event(" + v.m.String() + ")"
}
func (v matchValue) Type() string          { return "event" }
func (v matchValue) Freeze()               {}
func (v matchValue) Truth() starlark.Bool  { return true }
func (v matchValue) Hash() (uint32, error) { return starlark.String(v.m.String()).Hash() }

// eventBuiltin is event(kind, **fields): which events, as a Match.
func eventBuiltin(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("event takes a kind first (event(\"pr.merged\", pr = \"api#212\"))")
	}
	kind, ok := starlark.AsString(args[0])
	if !ok {
		return nil, fmt.Errorf("event's kind is a string, not %s", args[0].Type())
	}
	written := kind
	var pace Pace
	for _, kv := range kwargs {
		key, _ := starlark.AsString(kv[0])
		if key == "settle" || key == "at_most" || key == "per" {
			s, ok := starlark.AsString(kv[1])
			if !ok {
				return nil, fmt.Errorf("event's %s is a string, as settle = \"10m\", at_most = \"1/h\", per = \"pr\"", key)
			}
			if err := pace.set(key, s); err != nil {
				return nil, err
			}
			continue
		}
		var value string
		switch v := kv[1].(type) {
		case starlark.String:
			value = string(v)
		case starlark.Int:
			value = v.String()
		default:
			return nil, fmt.Errorf("event's %s is a %s: use a string", key, kv[1].Type())
		}
		if strings.ContainsAny(value, " \t") {
			return nil, fmt.Errorf("event's %s (%q) has a space: use a pattern instead (%s)", key, value, strings.ReplaceAll(value, " ", "?"))
		}
		written += " " + key + "=" + value
	}
	m, err := events.ParseMatch(written)
	if err != nil {
		return nil, err
	}
	if pace.Per != "" {
		if _, err := events.ParseMatch(kind + " " + pace.Per + "=*"); err != nil {
			return nil, fmt.Errorf("per: %w", err)
		}
	}
	return matchValue{m: m, pace: pace}, nil
}

// set reads one of a Pace's: settle (a length of time), at_most (n/time,
// as 1/h, 3/1d) or per (a field).
func (p *Pace) set(key, v string) error {
	switch key {
	case "settle":
		d, err := lengthOf(v)
		if err != nil {
			return fmt.Errorf("settle: %w", err)
		}
		p.Settle = d
	case "at_most":
		n, in, ok := strings.Cut(v, "/")
		count, err := strconv.Atoi(n)
		if !ok || err != nil || count < 1 {
			return fmt.Errorf("at_most: %q is how many in how long, as 1/h or 3/1d", v)
		}
		if in == "h" || in == "d" || in == "m" {
			in = "1" + in
		}
		d, err := lengthOf(in)
		if err != nil {
			return fmt.Errorf("at_most: %w", err)
		}
		p.AtMost, p.AtMostIn = count, d
	case "per":
		p.Per = strings.ToLower(strings.TrimSpace(v))
	}
	return nil
}

// matchesOf is when, read: an event(), or a list of them.
func matchesOf(v starlark.Value) ([]matchValue, error) {
	switch w := v.(type) {
	case matchValue:
		return []matchValue{w}, nil
	case *starlark.List, starlark.Tuple:
		var out []matchValue
		iter := starlark.Iterate(w)
		defer iter.Done()
		var x starlark.Value
		for iter.Next(&x) {
			m, ok := x.(matchValue)
			if !ok {
				return nil, fmt.Errorf("when has a %s in it: each is an event(), every() or action()", x.Type())
			}
			out = append(out, m)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("when is empty: say which events")
		}
		return out, nil
	}
	return nil, fmt.Errorf("when is a %s: use an event(), or a list of them", v.Type())
}

// maysOf is may, read: a list of what there is to do.
func maysOf(v starlark.Value) ([]string, error) {
	iterable, ok := v.(starlark.Iterable)
	if !ok {
		return nil, fmt.Errorf("may is a %s: use a list, as may = [\"notify\"]", v.Type())
	}
	var out []string
	iter := iterable.Iterate()
	defer iter.Done()
	var x starlark.Value
	for iter.Next(&x) {
		s, ok := starlark.AsString(x)
		if !ok {
			return nil, fmt.Errorf("may has a %s in it: each item is a string", x.Type())
		}
		if !slices.Contains(Mays, s) && !IsPluginKind(s) {
			return nil, fmt.Errorf("%q is not something an automation may do: use %s, or a plugin's kind of proposal (jira:changes)", s, strings.Join(Mays, " or "))
		}
		out = append(out, s)
	}
	return out, nil
}

// scriptError is what's wrong with a script, as its line says it.
func scriptError(err error) error {
	if ee, ok := err.(*starlark.EvalError); ok {
		// The script's own line: the innermost frame that's in it, not in
		// a builtin it called.
		for i := len(ee.CallStack) - 1; i >= 0; i-- {
			if pos := ee.CallStack[i].Pos; pos.Filename() != "<builtin>" {
				return fmt.Errorf("%s: %s", pos, ee.Msg)
			}
		}
		return errors.New(ee.Msg)
	}
	return err
}

// builtinNames are every builtin a run's given (run.builtins).
var builtinNames = []string{"notify", "converse", "log", "show", "events", "refs_in", "wait", "wait_all", "wait_any", "run_task", "repos", "keep", "kept", "output", "chart", "things", "ask", "propose", "act"}

// checkBuiltins are what a file's top level is read with: event, and
// every other builtin's name, so run can use them, refusing to be called
// before there's an event to run on.
func checkBuiltins() starlark.StringDict {
	d := starlark.StringDict{"event": starlark.NewBuiltin("event", eventBuiltin), "every": starlark.NewBuiltin("every", everyBuiltin), "action": starlark.NewBuiltin("action", actionBuiltin)}
	for _, name := range builtinNames {
		d[name] = starlark.NewBuiltin(name, func(_ *starlark.Thread, b *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
			return nil, fmt.Errorf("%s works only inside run, not at the file's top level", b.Name())
		})
	}
	return d
}
