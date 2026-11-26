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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/withworkspace/workspace/internal/events"
)

// Mays are what an automation may do, as may names them.
const (
	MayNotify   = "notify"
	MayConverse = "converse"
)

// Mays is every may there is.
var Mays = []string{MayNotify, MayConverse}

// Automation is one automation, as its file says it.
type Automation struct {
	Name string
	// About is what it's for, in your words: its first comment's lines.
	About string
	When  []events.Match
	May   []string
	// Source is its file, as read.
	Source []byte
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
	return slices.ContainsFunc(a.When, func(m events.Match) bool { return m.Matches(e) })
}

// WhenSaid is when, as written.
func (a *Automation) WhenSaid() string {
	var out []string
	for _, m := range a.When {
		out = append(out, m.String())
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
		return nil, fmt.Errorf("%q can't be an automation's name: lower-case letters, digits and hyphens", name)
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
		return nil, fmt.Errorf("it says no when: which events it's for, as when = event(\"pr.merged\", pr = \"api#212\")")
	}
	if a.When, err = matchesOf(when); err != nil {
		return nil, err
	}
	if may, ok := globals["may"]; ok {
		if a.May, err = maysOf(may); err != nil {
			return nil, err
		}
	}
	if run, ok := globals["run"]; ok {
		fn, ok := run.(*starlark.Function)
		if !ok {
			return nil, fmt.Errorf("run is a %s: it's a def run(e), called with each event", run.Type())
		}
		if fn.NumParams() != 1 {
			return nil, fmt.Errorf("run takes %d arguments: it's a def run(e), called with each event", fn.NumParams())
		}
		a.run = fn
	} else if len(a.May) > 0 && !slices.Equal(a.May, []string{MayNotify}) {
		return nil, fmt.Errorf("it may %s, and has no run to: without a def run(e), each event's notified as it is", strings.Join(a.May, ", "))
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

// matchValue is an event() as a script holds it.
type matchValue struct{ m events.Match }

func (v matchValue) String() string        { return "event(" + v.m.String() + ")" }
func (v matchValue) Type() string          { return "event" }
func (v matchValue) Freeze()               {}
func (v matchValue) Truth() starlark.Bool  { return true }
func (v matchValue) Hash() (uint32, error) { return starlark.String(v.m.String()).Hash() }

// eventBuiltin is event(kind, **fields): which events, as a Match.
func eventBuiltin(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("event takes its kind first (event(\"pr.merged\", pr = \"api#212\"))")
	}
	kind, ok := starlark.AsString(args[0])
	if !ok {
		return nil, fmt.Errorf("event's kind is a string, not %s", args[0].Type())
	}
	written := kind
	for _, kv := range kwargs {
		key, _ := starlark.AsString(kv[0])
		var value string
		switch v := kv[1].(type) {
		case starlark.String:
			value = string(v)
		case starlark.Int:
			value = v.String()
		default:
			return nil, fmt.Errorf("event's %s is a %s: say it as a string", key, kv[1].Type())
		}
		if strings.ContainsAny(value, " \t") {
			return nil, fmt.Errorf("event's %s (%q) has a space in it: a pattern can stand for one (%s)", key, value, strings.ReplaceAll(value, " ", "?"))
		}
		written += " " + key + "=" + value
	}
	m, err := events.ParseMatch(written)
	if err != nil {
		return nil, err
	}
	return matchValue{m}, nil
}

// matchesOf is when, read: an event(), or a list of them.
func matchesOf(v starlark.Value) ([]events.Match, error) {
	switch w := v.(type) {
	case matchValue:
		return []events.Match{w.m}, nil
	case *starlark.List, starlark.Tuple:
		var out []events.Match
		iter := starlark.Iterate(w)
		defer iter.Done()
		var x starlark.Value
		for iter.Next(&x) {
			m, ok := x.(matchValue)
			if !ok {
				return nil, fmt.Errorf("when has a %s in it: each is an event()", x.Type())
			}
			out = append(out, m.m)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("when is empty: say which events it's for")
		}
		return out, nil
	}
	return nil, fmt.Errorf("when is a %s: it's an event(), or a list of them", v.Type())
}

// maysOf is may, read: a list of what there is to do.
func maysOf(v starlark.Value) ([]string, error) {
	iterable, ok := v.(starlark.Iterable)
	if !ok {
		return nil, fmt.Errorf("may is a %s: it's a list, as may = [\"notify\"]", v.Type())
	}
	var out []string
	iter := iterable.Iterate()
	defer iter.Done()
	var x starlark.Value
	for iter.Next(&x) {
		s, ok := starlark.AsString(x)
		if !ok {
			return nil, fmt.Errorf("may has a %s in it: each is what it may do, as a string", x.Type())
		}
		if !slices.Contains(Mays, s) {
			return nil, fmt.Errorf("%q isn't something an automation may do yet: it may %s", s, strings.Join(Mays, " or "))
		}
		out = append(out, s)
	}
	return out, nil
}

// scriptError is what's wrong with a script, as its line says it.
func scriptError(err error) error {
	if ee, ok := err.(*starlark.EvalError); ok {
		return fmt.Errorf("%s", strings.TrimSpace(ee.Backtrace()))
	}
	return err
}

// builtinNames are every builtin a run's given (run.builtins).
var builtinNames = []string{"notify", "converse", "log", "show", "events", "refs_in"}

// checkBuiltins are what a file's top level is read with: event, and
// every other builtin's name, so run can use them, refusing to be called
// before there's an event to run on.
func checkBuiltins() starlark.StringDict {
	d := starlark.StringDict{"event": starlark.NewBuiltin("event", eventBuiltin)}
	for _, name := range builtinNames {
		d[name] = starlark.NewBuiltin(name, func(_ *starlark.Thread, b *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
			return nil, fmt.Errorf("%s is for run, called with each event: not for the file's top level", b.Name())
		})
	}
	return d
}
