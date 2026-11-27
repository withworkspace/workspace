package automation

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"go.starlark.net/starlark"

	"github.com/withworkspace/workspace/internal/events"
)

// Actions: an automation you run yourself. when = action("Is it ready to
// build?", on = "sketch") offers it where you act on things: in Find,
// under Do (one on nothing in particular) or Here (on the thing whose page
// you're on), and in the menu of a row that goes to one. on is the kind
// of thing it's for (pr, mission, conversation, artifact, repo, or a
// plugin's, jira:issue), and its other fields which of them, as they
// stand (status = ["In Review", "Blocked"]). Run, it's an action.run of
// its own, by you, on that thing (e.on), with what input asked for
// (e.input), and the run goes as any other.

// Action is one of an automation's actions, as its when says it.
type Action struct {
	Title string `json:"title"`
	// On is the kind of thing it's for, "" for none (it's run on nothing
	// in particular).
	On string `json:"on,omitempty"`
	// Where is which of them, by their fields: each a value or a pattern,
	// any of a list.
	Where map[string][]string `json:"where,omitempty"`
	// Input is what it asks before it runs, in order.
	Input []InputField `json:"input,omitempty"`
}

// InputField is one thing an action asks before it runs: its name, and
// its type (text, bool, int, number), or the words it's one of.
type InputField struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Choices []string `json:"choices,omitempty"`
}

// Actions are a's: those of its when that are action()s.
func (a *Automation) Actions() []Action {
	var out []Action
	for _, x := range a.actions {
		if x != nil {
			out = append(out, *x)
		}
	}
	return out
}

// ActionNamed is a's action by its title.
func (a *Automation) ActionNamed(title string) (Action, bool) {
	for _, x := range a.Actions() {
		if x.Title == title {
			return x, true
		}
	}
	return Action{}, false
}

// kindAliases are the names a kind of thing goes by, as ws's refs name
// them.
var kindAliases = map[string]string{"talk": "conversation", "pull": "pr", "page": "doc"}

// normalKind is a kind of thing by the one name it's matched by.
func normalKind(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	if to, ok := kindAliases[k]; ok {
		return to
	}
	return k
}

// Applies says whether x is offered on a thing of kind with fields (as
// they stand): "" with nothing for none, which only one on nothing is.
func (x Action) Applies(kind string, fields map[string]string) bool {
	if x.On == "" || kind == "" {
		return x.On == "" && kind == ""
	}
	if normalKind(x.On) != normalKind(kind) {
		return false
	}
	for k, patterns := range x.Where {
		v, ok := fieldOf(fields, k)
		if !ok {
			return false
		}
		hit := false
		for _, p := range patterns {
			hit = hit || like(p, v)
		}
		if !hit {
			return false
		}
	}
	return true
}

// WhyNot is why x isn't for a thing of kind with fields, in words: "" when
// it is.
func (x Action) WhyNot(kind string, fields map[string]string) string {
	switch {
	case x.Applies(kind, fields):
		return ""
	case x.On == "":
		return "runs on nothing in particular"
	case kind == "":
		return "runs on a " + x.On
	case normalKind(x.On) != normalKind(kind):
		return fmt.Sprintf("runs on a %s, not a %s", x.On, kind)
	}
	var said []string
	for _, k := range sortedWhere(x.Where) {
		v, _ := fieldOf(fields, k)
		said = append(said, fmt.Sprintf("%s is %q, not %s", k, v, strings.Join(x.Where[k], " or ")))
	}
	return "only for those whose " + strings.Join(said, ", ")
}

func sortedWhere(w map[string][]string) []string {
	out := make([]string, 0, len(w))
	for k := range w {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// fieldOf is a thing's field, by its name in any case.
func fieldOf(fields map[string]string, k string) (string, bool) {
	if v, ok := fields[k]; ok {
		return v, true
	}
	for name, v := range fields {
		if strings.EqualFold(name, k) {
			return v, true
		}
	}
	return "", false
}

// ReadInput is what you gave x as you ran it, each field as its type
// says: a field it asks for that's missing, or not of its type, is
// refused, and one it doesn't ask for.
func (x Action) ReadInput(in map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for _, f := range x.Input {
		v, ok := in[f.Name]
		if !ok || v == nil {
			if f.Type == "bool" {
				out[f.Name] = false
				continue
			}
			return nil, fmt.Errorf("%s needs %s", x.Title, f.Name)
		}
		got, err := f.read(v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		out[f.Name] = got
	}
	for k := range in {
		if !x.asks(k) {
			return nil, fmt.Errorf("%s does not take %s", x.Title, k)
		}
	}
	return out, nil
}

func (x Action) asks(name string) bool {
	for _, f := range x.Input {
		if f.Name == name {
			return true
		}
	}
	return false
}

// read is v as f's type: a form's words read as a number or a yes too.
func (f InputField) read(v any) (any, error) {
	s, isString := v.(string)
	switch f.Type {
	case "text":
		if !isString {
			return nil, fmt.Errorf("must be text, not %v", v)
		}
		return s, nil
	case "bool":
		if b, ok := v.(bool); ok {
			return b, nil
		}
		if b, err := strconv.ParseBool(s); isString && err == nil {
			return b, nil
		}
		return nil, fmt.Errorf("must be yes or no, not %v", v)
	case "int", "number":
		n, ok := v.(float64)
		if isString {
			p, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
			n, ok = p, err == nil
		}
		if !ok || (f.Type == "int" && n != math.Trunc(n)) {
			return nil, fmt.Errorf("must be %s, not %v", map[string]string{"int": "a whole number", "number": "a number"}[f.Type], v)
		}
		return n, nil
	case "choice":
		for _, c := range f.Choices {
			if isString && strings.EqualFold(c, strings.TrimSpace(s)) {
				return c, nil
			}
		}
		return nil, fmt.Errorf("must be one of %s, not %v", strings.Join(f.Choices, ", "), v)
	}
	return nil, fmt.Errorf("unknown type %q", f.Type)
}

// actionBuiltin is action(title, on = "kind", input = {...}, **where): an
// automation you run yourself, on a thing of that kind (none: on nothing
// in particular). It's a match for its own action.run, the automation's
// name filled in once it's known (Check).
func actionBuiltin(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	x := &Action{}
	if len(args) != 1 {
		return nil, fmt.Errorf(`action needs a title, as action("Is it ready to build?", on = "sketch")`)
	}
	title, ok := starlark.AsString(args[0])
	if !ok || strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf(`action's title must be text, as action("Is it ready to build?")`)
	}
	x.Title = strings.TrimSpace(title)
	for _, kv := range kwargs {
		k, _ := starlark.AsString(kv[0])
		switch k {
		case "on":
			on, ok := starlark.AsString(kv[1])
			if !ok {
				return nil, fmt.Errorf("action: on is a kind of thing, as \"pr\" or \"jira:issue\"")
			}
			x.On = normalKind(on)
			if x.On == "" {
				return nil, fmt.Errorf("action: on is a kind of thing, as \"pr\" or \"jira:issue\": leave out to run on nothing")
			}
		case "input":
			fields, err := inputOf(kv[1])
			if err != nil {
				return nil, fmt.Errorf("action: %w", err)
			}
			x.Input = fields
		default:
			var values []string
			switch v := kv[1].(type) {
			case starlark.String:
				values = []string{string(v)}
			case *starlark.List:
				values = stringsOf(v)
				if len(values) == 0 || len(values) != v.Len() {
					return nil, fmt.Errorf("action: %s is a value, or a list of values", k)
				}
			default:
				return nil, fmt.Errorf("action: %s is a value, or a list of values, not a %s", k, kv[1].Type())
			}
			if x.Where == nil {
				x.Where = map[string][]string{}
			}
			x.Where[k] = values
		}
	}
	if x.On == "" && len(x.Where) > 0 {
		return nil, fmt.Errorf("action: filtering on %s needs a kind, as on = \"pr\"", strings.Join(sortedWhere(x.Where), ", "))
	}
	return matchValue{m: events.Match{Kind: events.ActionRun}, action: x}, nil
}

// inputOf is input = {...}: each field's type ("text", "bool", "int",
// "number"), or a list of the words it's one of, in the order written.
func inputOf(v starlark.Value) ([]InputField, error) {
	d, ok := v.(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("input is a dict of fields, as {\"note\": \"text\"}")
	}
	var out []InputField
	for _, item := range d.Items() {
		k, ok := starlark.AsString(item[0])
		if !ok || k == "" {
			return nil, fmt.Errorf("input: a field's name is a string")
		}
		switch t := item[1].(type) {
		case starlark.String:
			switch string(t) {
			case "text", "bool", "int", "number":
				out = append(out, InputField{Name: k, Type: string(t)})
			default:
				return nil, fmt.Errorf("input: %s is %q: a field is \"text\", \"bool\", \"int\", \"number\", or a list of choices", k, string(t))
			}
		case *starlark.List:
			words := stringsOf(t)
			if len(words) < 2 || len(words) != t.Len() {
				return nil, fmt.Errorf("input: %s needs a list of two or more choices", k)
			}
			out = append(out, InputField{Name: k, Type: "choice", Choices: words})
		default:
			return nil, fmt.Errorf("input: %s's type is a name, as \"text\", or a list of words", k)
		}
	}
	return out, nil
}

// RunAction has an automation's action run, by you, on what on names ("" for
// nothing) with input as read: an action.run of its own, which the daemon
// runs it on.
func RunAction(root, name, title, on string, input map[string]any) (events.Event, error) {
	ev := events.Event{Kind: events.ActionRun, By: events.ByYou, Automation: name, Title: title, Input: input}
	if on != "" {
		ev.About = []string{on}
	}
	return events.Emit(root, ev)
}
