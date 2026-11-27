package automation

import (
	"fmt"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Proposing: propose(kind, {...}) puts something for you to accept in a
// conversation of the automation's: a mission to start, a decision to
// record, or a plugin's kind of proposal (Jira's changes to issues). It's
// the card a conversation's agent would make, made by the script; you
// accept or decline it as any other, and the conversation is there to
// talk it over. What it proposes in one run goes in one conversation.
//
// propose gives a handle, which wait, wait_any and wait_all take: what it
// comes to is whether you accepted it.

// MayPropose is proposing (propose): a card for you to accept.
const MayPropose = "propose"

// ProposalOpen is a proposal no one's settled: what a wait on it waits
// past.
const ProposalOpen = "open"

// Proposing is what an automation proposes: its kind, what it is (as the
// script gave it, JSON's types), and why, in its words.
type Proposing struct {
	Kind string `json:"kind"`
	// Input is what it is: a dict for a mission or a decision; for a
	// plugin's kind, whatever its guide says it's written as.
	Input any    `json:"input,omitempty"`
	Why   string `json:"why,omitempty"`
	// On is the event its run's on, said (e12: …).
	On string `json:"on,omitempty"`
}

// Says is what's proposed, in a line: "a mission: Fix the login".
func (p Proposing) Says() string {
	str := func(k string) string { s, _ := p.Fields()[k].(string); return s }
	switch p.Kind {
	case "mission":
		return "a mission: " + str("title")
	case "decision":
		return "a decision: " + str("what")
	}
	if t := str("title"); t != "" {
		return p.Kind + ": " + t
	}
	return p.Kind
}

// Fields are Input's, where it's a dict: none, where it isn't.
func (p Proposing) Fields() map[string]any {
	m, _ := p.Input.(map[string]any)
	return m
}

// checkProposing is why p can't be proposed, nil where it can: a mission
// has a title, a decision says what; a plugin's kind (plugin:what) is the
// plugin's to read.
func checkProposing(p Proposing) error {
	fields := p.Fields()
	if !strings.Contains(p.Kind, ":") && fields == nil {
		return fmt.Errorf("the proposal is a dict, as {\"title\": \"…\"}")
	}
	str := func(k string, need bool) error {
		v, ok := fields[k]
		if !ok {
			if need {
				return fmt.Errorf("a %s needs a %s", p.Kind, k)
			}
			return nil
		}
		if s, ok := v.(string); !ok || (need && strings.TrimSpace(s) == "") {
			return fmt.Errorf("a %s's %s must be text", p.Kind, k)
		}
		return nil
	}
	list := func(k string) error {
		v, ok := fields[k]
		if !ok {
			return nil
		}
		l, ok := v.([]any)
		if !ok {
			return fmt.Errorf("a %s's %s must be a list of refs", p.Kind, k)
		}
		for _, x := range l {
			if _, ok := x.(string); !ok {
				return fmt.Errorf("a %s's %s must be a list of refs", p.Kind, k)
			}
		}
		return nil
	}
	only := func(keys ...string) error {
		for k := range fields {
			found := false
			for _, want := range keys {
				found = found || k == want
			}
			if !found {
				return fmt.Errorf("a %s has no %s: use %s", p.Kind, k, strings.Join(keys, ", "))
			}
		}
		return nil
	}
	switch {
	case p.Kind == "mission":
		for _, err := range []error{only("title", "brief", "about"), str("title", true), str("brief", false), list("about")} {
			if err != nil {
				return err
			}
		}
	case p.Kind == "decision":
		for _, err := range []error{only("what", "why"), str("what", true), str("why", false)} {
			if err != nil {
				return err
			}
		}
	case strings.Contains(p.Kind, ":"):
		// The plugin's to read, when it drafts it.
	default:
		return fmt.Errorf("%q is not something an automation can propose: use \"mission\", \"decision\", or a plugin's kind (jira:changes)", p.Kind)
	}
	return nil
}

// proposalHandle is a proposal made, as a script holds it: what a wait
// takes.
type proposalHandle struct{ ref, kind string }

func (h proposalHandle) String() string        { return "proposal(" + h.ref + ")" }
func (h proposalHandle) Type() string          { return "proposal" }
func (h proposalHandle) Freeze()               {}
func (h proposalHandle) Truth() starlark.Bool  { return true }
func (h proposalHandle) Hash() (uint32, error) { return starlark.String(h.ref).Hash() }
func (h proposalHandle) AttrNames() []string   { return []string{"ref", "kind", "conversation"} }
func (h proposalHandle) Attr(name string) (starlark.Value, error) {
	switch name {
	case "ref":
		return starlark.String(h.ref), nil
	case "kind":
		return starlark.String(h.kind), nil
	case "conversation":
		c, _, _ := strings.Cut(h.ref, "#")
		return starlark.String(c), nil
	}
	return nil, nil
}

// proposalCame is what a proposal came to, as a wait gives it: its ref,
// how it was settled, and what accepting it made (a mission's slug).
func proposalCame(ref, state, result string) map[string]any {
	return map[string]any{"proposal": ref, "state": state, "accepted": state == "accepted", "result": result}
}

// propose is propose(kind, {...}, why = "…"): something proposed for you
// to accept, its handle at once.
func (r *run) propose(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if !r.a.Allows(MayPropose) {
		return nil, fmt.Errorf("propose: not allowed without may = [\"propose\"]")
	}
	var kind, why string
	var input starlark.Value
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "kind", &kind, "input?", &input, "why?", &why); err != nil {
		return nil, err
	}
	p := Proposing{Kind: strings.TrimSpace(kind), Why: strings.TrimSpace(why), On: r.ev.Ref() + ": " + r.ev.Says()}
	if r.ev.Kind == "schedule.due" {
		p.On = "schedule: " + r.ev.Says()
	}
	if input != nil && input != starlark.None {
		v, err := goOf(input)
		if err != nil {
			return nil, fmt.Errorf("propose: %w", err)
		}
		p.Input = v
	} else if !strings.Contains(p.Kind, ":") {
		p.Input = map[string]any{}
	}
	if err := checkProposing(p); err != nil {
		return nil, fmt.Errorf("propose: %w", err)
	}
	do := func() (string, error) {
		if r.e.Hooks.Propose == nil {
			return "", fmt.Errorf("propose: not available here")
		}
		return r.e.Hooks.Propose(r.a, p, r.conv, ChainOf(r.a.Name, r.ev, r.batch))
	}
	if r.dry != nil {
		do = func() (string, error) {
			r.dry.did = append(r.dry.did, "propose "+p.Says())
			return fmt.Sprintf("dry#%d", r.steps), nil
		}
	}
	ref, err := r.step("propose", do)
	if err != nil {
		return nil, err
	}
	if c, _, ok := strings.Cut(ref, "#"); ok && r.dry == nil {
		r.conv = c // what else it proposes this run goes there too
	}
	return proposalHandle{ref: ref, kind: p.Kind}, nil
}

// goOf is a script's value as JSON's types: None, bools, numbers,
// strings, lists and dicts (with string keys) of them, and a struct's
// fields (an event's, show's) as a dict.
func goOf(v starlark.Value) (any, error) {
	switch x := v.(type) {
	case starlark.NoneType:
		return nil, nil
	case starlark.Bool:
		return bool(x), nil
	case starlark.Int:
		n, ok := x.Int64()
		if !ok {
			return nil, fmt.Errorf("%s is too big a number", x)
		}
		return float64(n), nil
	case starlark.Float:
		return float64(x), nil
	case starlark.String:
		return string(x), nil
	case *starlark.List:
		out := make([]any, 0, x.Len())
		for i := range x.Len() {
			e, err := goOf(x.Index(i))
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
		return out, nil
	case starlark.Tuple:
		out := make([]any, 0, len(x))
		for _, e := range x {
			g, err := goOf(e)
			if err != nil {
				return nil, err
			}
			out = append(out, g)
		}
		return out, nil
	case *starlark.Dict:
		out := map[string]any{}
		for _, item := range x.Items() {
			k, ok := starlark.AsString(item[0])
			if !ok {
				return nil, fmt.Errorf("a dict's keys are strings, not %s", item[0].Type())
			}
			g, err := goOf(item[1])
			if err != nil {
				return nil, err
			}
			out[k] = g
		}
		return out, nil
	case *starlarkstruct.Struct:
		d := starlark.StringDict{}
		x.ToStringDict(d)
		out := map[string]any{}
		for k, e := range d {
			g, err := goOf(e)
			if err != nil {
				return nil, err
			}
			out[k] = g
		}
		return out, nil
	}
	return nil, fmt.Errorf("a %s cannot be written as JSON", v.Type())
}
