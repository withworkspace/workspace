package automation

import (
	"fmt"
	"regexp"
	"strings"

	"go.starlark.net/syntax"
)

// An outline is an automation's script drawn as what it does, read from
// its syntax without running it: when it runs, then its run's steps in
// order, each a builtin it calls (a read, a notification, a conversation
// started, a wait), with its ifs and fors as branches. A wait whose
// result is tested for None splits in two, met and its time up. What
// isn't one of those is a line of code, as written. It's a picture of the
// script, not a promise: a value worked out as it runs is said as the
// code that works it out.

// Outline is an automation's script, drawn.
type Outline struct {
	// When is what it runs on: each event() of its when, with its pace.
	When []OutlineWhen `json:"when"`
	// Steps are its run's, in order; none with no run, when the event's
	// notified as it is (Default).
	Steps   []OutlineStep `json:"steps"`
	Default bool          `json:"default,omitempty"`
}

// OutlineWhen is one event() of a when: what it matches, and its pace.
type OutlineWhen struct {
	Match string `json:"match"`
	Pace  string `json:"pace,omitempty"`
}

// OutlineStep is one step of a run. Kind is notify, converse, wait, read
// (show, events, refs_in), log, if, for, stop (a return) or code. At is
// where it is in the script (line:col): for a builtin, where it's called,
// which is what its runs' steps are kept by, so they can be counted on
// it. Text is what it says (a notification's words, with what's filled in
// as it runs in braces; a wait's event; an if's condition), and Detail
// the rest (a wait's timeout; what a read's assigned to). Branches are
// what it splits into: an if's (if, elif, else), a for's body, a wait's
// met and time's up.
type OutlineStep struct {
	Kind     string          `json:"kind"`
	At       string          `json:"at"`
	Line     int             `json:"line"`
	Text     string          `json:"text,omitempty"`
	Detail   string          `json:"detail,omitempty"`
	Branches []OutlineBranch `json:"branches,omitempty"`
}

// OutlineBranch is one way a step goes: its label (else, met, for each
// x in e.batch) and its steps.
type OutlineBranch struct {
	Label string        `json:"label"`
	Steps []OutlineStep `json:"steps"`
}

// reads are the builtins that read, kept as steps.
var reads = map[string]bool{"show": true, "events": true, "refs_in": true, "repos": true, "kept": true, "things": true}

// OutlineOf is a's outline.
func OutlineOf(a *Automation) (Outline, error) {
	out := Outline{When: []OutlineWhen{}, Steps: []OutlineStep{}}
	for i, m := range a.When {
		w := OutlineWhen{Match: m.String()}
		if s := a.scheduleAt(i); s != nil {
			w.Match = "every " + s.Said
		}
		if i < len(a.Paces) {
			w.Pace = a.Paces[i].String()
		}
		out.When = append(out.When, w)
	}
	f, err := fileOptions.Parse(a.Name+".star", a.Source, 0)
	if err != nil {
		return out, scriptError(err)
	}
	o := &outliner{src: strings.Split(string(a.Source), "\n"), defs: map[string]*syntax.DefStmt{}, inside: map[string]bool{}}
	var run *syntax.DefStmt
	for _, st := range f.Stmts {
		if d, ok := st.(*syntax.DefStmt); ok {
			o.defs[d.Name.Name] = d
			if d.Name.Name == "run" {
				run = d
			}
		}
	}
	if run == nil {
		out.Default = true
		return out, nil
	}
	o.inside["run"] = true
	out.Steps = o.stmts(run.Body)
	return out, nil
}

type outliner struct {
	src  []string
	defs map[string]*syntax.DefStmt
	// inside are the defs being drawn: a helper's body is drawn where
	// it's called, once (not one it calls again).
	inside map[string]bool
	// bound are a helper's parameters, as it's called, while its body's
	// drawn: what its words say is what it was given.
	bound map[string]syntax.Expr
}

func (o *outliner) stmts(list []syntax.Stmt) []OutlineStep {
	var out []OutlineStep
	for i := 0; i < len(list); i++ {
		st := list[i]
		// x = wait(...), then if x == None / if x / if not x: one wait, met
		// and its time up.
		if w, name := waitAssigned(st); w != nil && i+1 < len(list) {
			if ifs, ok := list[i+1].(*syntax.IfStmt); ok {
				if met, ok := testsFound(ifs.Cond, name); ok {
					step := o.wait(w)
					yes, no := o.stmts(ifs.True), o.stmts(ifs.False)
					if !met {
						yes, no = no, yes
					}
					step.Branches = []OutlineBranch{{Label: "met", Steps: orNone(yes)}, {Label: "time's up", Steps: orNone(no)}}
					out = append(out, step)
					i++
					continue
				}
			}
		}
		out = append(out, o.stmt(st)...)
	}
	return mergeCode(out)
}

func orNone(s []OutlineStep) []OutlineStep {
	if s == nil {
		return []OutlineStep{}
	}
	return s
}

func (o *outliner) stmt(st syntax.Stmt) []OutlineStep {
	switch s := st.(type) {
	case *syntax.IfStmt:
		out := o.calls(s.Cond)
		step := OutlineStep{Kind: "if", Text: o.source(s.Cond)}
		step.place(s.If)
		var branches []OutlineBranch
		branches = append(branches, OutlineBranch{Label: "if " + o.source(s.Cond), Steps: orNone(o.stmts(s.True))})
		rest := s.False
		for len(rest) == 1 {
			elif, ok := rest[0].(*syntax.IfStmt)
			if !ok || elif.If != s.ElsePos { // else: if …, not an elif
				break
			}
			branches = append(branches, OutlineBranch{Label: "elif " + o.source(elif.Cond), Steps: orNone(o.stmts(elif.True))})
			rest, s = elif.False, elif
		}
		branches = append(branches, OutlineBranch{Label: "else", Steps: orNone(o.stmts(rest))})
		step.Branches = branches
		return append(out, step)
	case *syntax.ForStmt:
		out := o.calls(s.X)
		step := OutlineStep{Kind: "for", Text: o.source(s.Vars) + " in " + o.source(s.X)}
		step.place(s.For)
		step.Branches = []OutlineBranch{{Label: "for each " + o.source(s.Vars) + " in " + o.source(s.X), Steps: orNone(o.stmts(s.Body))}}
		return append(out, step)
	case *syntax.ReturnStmt:
		out := o.calls(s.Result)
		step := OutlineStep{Kind: "stop"}
		step.place(s.Return)
		return append(out, step)
	case *syntax.BranchStmt:
		step := OutlineStep{Kind: "code", Text: s.Token.String()}
		step.place(s.TokenPos)
		return []OutlineStep{step}
	case *syntax.DefStmt:
		return nil
	case *syntax.AssignStmt:
		out := o.calls(s.RHS)
		if len(out) == 1 && out[0].Kind == "read" {
			out[0].Detail = o.source(s.LHS)
			return out
		}
		if len(out) > 0 {
			return out
		}
	case *syntax.ExprStmt:
		if out := o.calls(s.X); len(out) > 0 {
			return out
		}
	}
	start, _ := st.Span()
	step := OutlineStep{Kind: "code", Text: o.source(st)}
	step.place(start)
	return []OutlineStep{step}
}

// calls are the steps the builtins called in x are, innermost first, and
// a helper's, drawn where it's called.
func (o *outliner) calls(x syntax.Expr) []OutlineStep {
	if x == nil {
		return nil
	}
	var out []OutlineStep
	var visit func(n syntax.Node) bool
	visit = func(n syntax.Node) bool {
		switch c := n.(type) {
		case *syntax.LambdaExpr:
			return false
		case *syntax.CallExpr:
			for _, arg := range c.Args {
				syntax.Walk(arg, visit)
			}
			id, ok := c.Fn.(*syntax.Ident)
			if !ok {
				syntax.Walk(c.Fn, visit)
				return false
			}
			if step, ok := o.builtin(id.Name, c); ok {
				out = append(out, step)
			} else if d := o.defs[id.Name]; d != nil && !o.inside[id.Name] {
				o.inside[id.Name] = true
				was := o.bound
				o.bound = bindings(d, c)
				out = append(out, o.stmts(d.Body)...)
				o.bound = was
				delete(o.inside, id.Name)
			}
			return false
		}
		return true
	}
	syntax.Walk(x, visit)
	return out
}

// builtin is the step a call to one of the builtins is.
func (o *outliner) builtin(name string, c *syntax.CallExpr) (OutlineStep, bool) {
	args, kw := splitArgs(c.Args)
	first := func() syntax.Expr {
		if len(args) > 0 {
			return args[0]
		}
		return nil
	}
	step := OutlineStep{}
	step.place(c.Lparen)
	switch {
	case name == "notify" || name == "converse":
		step.Kind, step.Text = name, o.words(first())
		if open := kw["open"]; open != nil {
			step.Detail = "opens " + o.source(open)
		}
		if about := kw["about"]; about != nil {
			step.Detail = "about " + o.source(about)
		}
	case name == "wait":
		return o.wait(c), true
	case name == "wait_all" || name == "wait_any":
		step.Kind = "wait"
		which := "all of "
		if name == "wait_any" {
			which = "the first of "
		}
		step.Text = which + o.source(first())
		if t := kw["timeout"]; t != nil {
			step.Detail = "up to " + o.words(t)
		}
	case name == "run_task":
		step.Kind = "run"
		step.Text = o.words(first())
		if repo := kw["repo"]; repo != nil {
			step.Text += " on " + o.words(repo)
		}
	case reads[name]:
		step.Kind, step.Text = "read", name+"("+o.argsSource(c)+")"
	case name == "output":
		step.Kind, step.Text = "output", "its page"
		if html := kw["html"]; html != nil {
			step.Detail = "as HTML"
		}
	case name == "keep":
		step.Kind, step.Text = "keep", o.words(first())
		if len(args) > 1 {
			step.Detail = o.source(args[1])
		} else if v := kw["value"]; v != nil {
			step.Detail = o.source(v)
		}
	case name == "log" || name == "print":
		step.Kind, step.Text = "log", o.words(first())
	default:
		return step, false
	}
	return step, true
}

// wait is a wait() as a step: what it waits for, and how long.
func (o *outliner) wait(c *syntax.CallExpr) OutlineStep {
	args, kw := splitArgs(c.Args)
	step := OutlineStep{Kind: "wait"}
	step.place(c.Lparen)
	if len(args) > 0 {
		step.Text = o.source(args[0])
		if ev, ok := args[0].(*syntax.CallExpr); ok {
			if id, ok := ev.Fn.(*syntax.Ident); ok && id.Name == "event" {
				step.Text = o.eventSaid(ev)
			}
		}
	}
	if t := kw["timeout"]; t != nil {
		step.Detail = "up to " + o.words(t)
	}
	return step
}

// eventSaid is event("pr.checks_passed", pr = e.pr) as a when says it:
// pr.checks_passed pr={e.pr}.
func (o *outliner) eventSaid(c *syntax.CallExpr) string {
	args, kw := splitArgs(c.Args)
	var parts []string
	if len(args) > 0 {
		parts = append(parts, o.words(args[0]))
	}
	for _, arg := range c.Args {
		if b, ok := arg.(*syntax.BinaryExpr); ok && b.Op == syntax.EQ {
			if id, ok := b.X.(*syntax.Ident); ok && kw[id.Name] != nil {
				parts = append(parts, id.Name+"="+o.words(b.Y))
			}
		}
	}
	return strings.Join(parts, " ")
}

// words is what an expression says, as words: a string as it's written,
// a % format with what fills it in braces ("{e.by.short} drew
// {e.title}"), and anything else as its code in braces.
func (o *outliner) words(x syntax.Expr) string {
	switch v := x.(type) {
	case nil:
		return ""
	case *syntax.Ident:
		if given := o.bound[v.Name]; given != nil {
			bound := o.bound
			o.bound = nil // what it was given is the caller's
			defer func() { o.bound = bound }()
			return o.words(given)
		}
	case *syntax.Literal:
		if s, ok := v.Value.(string); ok {
			return s
		}
	case *syntax.BinaryExpr:
		if v.Op == syntax.PERCENT {
			if lit, ok := v.X.(*syntax.Literal); ok {
				if format, ok := lit.Value.(string); ok {
					var fills []syntax.Expr
					if t, ok := v.Y.(*syntax.TupleExpr); ok {
						fills = t.List
					} else if p, ok := v.Y.(*syntax.ParenExpr); ok {
						if t, ok := p.X.(*syntax.TupleExpr); ok {
							fills = t.List
						} else {
							fills = []syntax.Expr{p.X}
						}
					} else {
						fills = []syntax.Expr{v.Y}
					}
					n := 0
					return verb.ReplaceAllStringFunc(format, func(m string) string {
						if m == "%%" {
							return "%"
						}
						if n >= len(fills) {
							return m
						}
						n++
						return "{" + o.source(fills[n-1]) + "}"
					})
				}
			}
		}
		if v.Op == syntax.PLUS {
			return o.words(v.X) + o.words(v.Y)
		}
	}
	return "{" + o.source(x) + "}"
}

// verb is a % format's verb.
var verb = regexp.MustCompile(`%(%|[-+# 0-9.]*[sdrfx])`)

// bindings are d's parameters as c gives them: by place, then by name.
func bindings(d *syntax.DefStmt, c *syntax.CallExpr) map[string]syntax.Expr {
	args, kw := splitArgs(c.Args)
	out := map[string]syntax.Expr{}
	for i, p := range d.Params {
		var name string
		switch q := p.(type) {
		case *syntax.Ident:
			name = q.Name
		case *syntax.BinaryExpr:
			if id, ok := q.X.(*syntax.Ident); ok {
				name = id.Name
			}
		}
		if name == "" {
			continue
		}
		if i < len(args) {
			out[name] = args[i]
		} else if v := kw[name]; v != nil {
			out[name] = v
		}
	}
	return out
}

// splitArgs is a call's arguments: those by place, and those by name.
func splitArgs(list []syntax.Expr) ([]syntax.Expr, map[string]syntax.Expr) {
	var args []syntax.Expr
	kw := map[string]syntax.Expr{}
	for _, a := range list {
		if b, ok := a.(*syntax.BinaryExpr); ok && b.Op == syntax.EQ {
			if id, ok := b.X.(*syntax.Ident); ok {
				kw[id.Name] = b.Y
				continue
			}
		}
		args = append(args, a)
	}
	return args, kw
}

// argsSource is a call's arguments, as written.
func (o *outliner) argsSource(c *syntax.CallExpr) string {
	var parts []string
	for _, a := range c.Args {
		parts = append(parts, o.source(a))
	}
	return strings.Join(parts, ", ")
}

// source is n as it's written, on one line.
func (o *outliner) source(n syntax.Node) string {
	if n == nil {
		return ""
	}
	start, end := n.Span()
	if start.Line < 1 || int(end.Line) > len(o.src) {
		return ""
	}
	var b strings.Builder
	for line := start.Line; line <= end.Line; line++ {
		runes := []rune(o.src[line-1])
		from, to := 0, len(runes)
		if line == start.Line {
			from = min(int(start.Col)-1, len(runes))
		}
		if line == end.Line {
			to = min(int(end.Col)-1, len(runes))
		}
		if from < to {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(strings.TrimSpace(string(runes[from:to])))
		}
	}
	return b.String()
}

func (s *OutlineStep) place(p syntax.Position) {
	s.Line, s.At = int(p.Line), PosKey(p)
}

// PosKey is where something is in a script, as its steps are kept by:
// line:col.
func PosKey(p syntax.Position) string {
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

// waitAssigned is x = wait(...): the wait, and x.
func waitAssigned(st syntax.Stmt) (*syntax.CallExpr, string) {
	a, ok := st.(*syntax.AssignStmt)
	if !ok || a.Op != syntax.EQ {
		return nil, ""
	}
	id, ok := a.LHS.(*syntax.Ident)
	if !ok {
		return nil, ""
	}
	c, ok := a.RHS.(*syntax.CallExpr)
	if !ok {
		return nil, ""
	}
	if fn, ok := c.Fn.(*syntax.Ident); !ok || fn.Name != "wait" {
		return nil, ""
	}
	return c, id.Name
}

// testsFound is whether cond tests name for what a wait came to: x, x !=
// None (true: it's met when cond is) or not x, x == None (false: it's
// met when it isn't).
func testsFound(cond syntax.Expr, name string) (met bool, ok bool) {
	is := func(x syntax.Expr) bool {
		id, ok := x.(*syntax.Ident)
		return ok && id.Name == name
	}
	isNone := func(x syntax.Expr) bool {
		id, ok := x.(*syntax.Ident)
		return ok && id.Name == "None"
	}
	switch c := cond.(type) {
	case *syntax.Ident:
		return true, is(c)
	case *syntax.UnaryExpr:
		return false, c.Op == syntax.NOT && is(c.X)
	case *syntax.BinaryExpr:
		if (is(c.X) && isNone(c.Y)) || (isNone(c.X) && is(c.Y)) {
			switch c.Op {
			case syntax.EQL:
				return false, true
			case syntax.NEQ:
				return true, true
			}
		}
	}
	return false, false
}

// mergeCode puts lines of code one after another into one step.
func mergeCode(list []OutlineStep) []OutlineStep {
	var out []OutlineStep
	for _, s := range list {
		if n := len(out); n > 0 && s.Kind == "code" && out[n-1].Kind == "code" {
			out[n-1].Text += "\n" + s.Text
			continue
		}
		out = append(out, s)
	}
	return out
}
