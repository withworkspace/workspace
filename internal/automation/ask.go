package automation

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go.starlark.net/starlark"

	"github.com/withworkspace/workspace/internal/store"
)

// Asking an agent: ask(question, answer = {...}) has one agent turn
// answer a question, in the shape the script asked for, and gives the
// script what it said to go on with. It's how a script hands judgment to
// a model and branches on it ("is this a flake?"), where converse hands
// the whole thing over to a conversation and hears nothing back.
//
// The turn is ws's to run, not the script's: an ask is queued as a task
// is, the run parks on it as at a wait(), and the daemon runs the turn
// beside the automations' passes (a turn takes a minute; a pass, none).
// The answer wakes the run, which is run again from the top, its ask
// given what was said. Each automation's asks spend from a budget of
// tokens a day (Engine.AskBudget), kept in the spend ledger.

// MayAsk is asking an agent (ask): its turns spend tokens, up to the
// automation's budget.
const MayAsk = "ask"

// MaxAsks is how many asks are answered at once.
const MaxAsks = 2

// DefaultAskBudget is how many tokens a day an automation's asks may
// spend, unless the engine says otherwise ([automations] ask_budget).
const DefaultAskBudget = 250_000

// askWaitsFor is how long a run waits for its ask's answer: past it, the
// ask's None.
const askWaitsFor = time.Hour

// overBudget starts why an ask wasn't asked when its automation's spent
// its tokens for the day: the run fails with it, and you're told.
const overBudget = "over its budget"

// ErrAskAgain is an ask that wasn't answered because ws stopped first: it's
// asked again.
var ErrAskAgain = errors.New("ws stopped before it was answered")

func init() {
	store.Define("automation-asks", store.SQL(`
		-- What automations asked an agent (ask): queued, answered one or two
		-- at a time, and what was said.
		CREATE TABLE automation_asks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,                 -- the automation
			event INTEGER NOT NULL,             -- the event its run's on
			prompt TEXT NOT NULL,               -- what the agent's asked, whole
			shape TEXT NOT NULL DEFAULT '',     -- the answer's fields, as JSON ('' for words)
			state TEXT NOT NULL,                -- queued, running, ended
			answer TEXT NOT NULL DEFAULT '',    -- what it came to, as JSON (null for none)
			why TEXT NOT NULL DEFAULT '',       -- why there's no answer
			queued TEXT NOT NULL,
			started TEXT NOT NULL DEFAULT '',
			ended TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX automation_asks_state ON automation_asks (state, id);
	`))
}

// AskInfo is an ask, as kept.
type AskInfo struct {
	ID     int64
	Name   string
	Event  int64
	Prompt string
	Shape  Shape
	State  string
	// Answer is what it came to, as JSON: "null" for none, with Why.
	Answer string
	Why    string
}

// outcome is what an ask came to, as its step keeps it.
type outcome struct {
	Answer any    `json:"answer"`
	Why    string `json:"why,omitempty"`
}

// outcomeOf is what an ended ask came to, as JSON.
func (a AskInfo) outcomeOf() string {
	var v any
	_ = json.Unmarshal([]byte(a.Answer), &v)
	b, _ := json.Marshal(outcome{Answer: v, Why: a.Why})
	return string(b)
}

// Shape is the answer an ask wants, by field: "bool", "int", "number",
// "text", or a list of the words it's one of. Nil is words.
type Shape map[string]any

// shapeOf is answer = {...} as a Shape: each value a type's name, or a
// list of strings it's one of.
func shapeOf(v starlark.Value) (Shape, error) {
	if v == nil || v == starlark.None {
		return nil, nil
	}
	d, ok := v.(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("answer is a dict of each field's type, as {\"flaky\": \"bool\", \"why\": \"text\"}")
	}
	out := Shape{}
	for _, item := range d.Items() {
		k, ok := starlark.AsString(item[0])
		if !ok || k == "" {
			return nil, fmt.Errorf("answer: a field's name is a string")
		}
		switch t := item[1].(type) {
		case starlark.String:
			switch string(t) {
			case "bool", "int", "number", "text":
				out[k] = string(t)
			default:
				return nil, fmt.Errorf("answer: %s is %q: a field is \"bool\", \"int\", \"number\", \"text\", or a list of choices", k, string(t))
			}
		case *starlark.List:
			words := stringsOf(t)
			if len(words) < 2 || len(words) != t.Len() {
				return nil, fmt.Errorf("answer: %s needs a list of two or more choices", k)
			}
			out[k] = words
		default:
			return nil, fmt.Errorf("answer: %s's type is a name, as \"bool\", or a list of words", k)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("answer has no fields: leave out for a plain-text answer")
	}
	return out, nil
}

// said is a field's type, in the words the agent's told it in.
func (s Shape) said(k string) string {
	switch t := s[k].(type) {
	case string:
		return map[string]string{"bool": "true or false", "int": "a whole number", "number": "a number", "text": "a string"}[t]
	case []string:
		quoted := make([]string, len(t))
		for i, w := range t {
			quoted[i] = fmt.Sprintf("%q", w)
		}
		return "one of " + strings.Join(quoted, ", ")
	}
	return "anything"
}

// fields are s's fields, in order.
func (s Shape) fields() []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// askPrompt is what the agent's asked: the question, what it's about as
// ws shows each, and how to answer.
func askPrompt(name, question string, about map[string]any, shape Shape) string {
	var b strings.Builder
	fmt.Fprintf(&b, "An automation of this workspace's, %s, asks you a question. Answer it from what's below: you have no tools, and nothing else to go on.\n\n", name)
	b.WriteString(strings.TrimSpace(question))
	b.WriteString("\n")
	if len(about) > 0 {
		b.WriteString("\nWhat it's about, as ws shows each:\n")
		for _, ref := range sortedKeys(about) {
			j, _ := json.MarshalIndent(about[ref], "", "  ")
			fmt.Fprintf(&b, "\n%s:\n%s\n", ref, j)
		}
	}
	if shape == nil {
		b.WriteString("\nAnswer in a few plain sentences, and nothing else.\n")
		return b.String()
	}
	b.WriteString("\nAnswer with one JSON object and nothing else: no words before or after it, no code fence. Its fields:\n")
	for _, k := range shape.fields() {
		fmt.Fprintf(&b, "- %s: %s\n", k, shape.said(k))
	}
	return b.String()
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// readAnswer is what an agent said, read as shape asks: the words, for
// none; else the JSON object it wrote, each field of the type asked.
func readAnswer(said string, shape Shape) (any, error) {
	said = strings.TrimSpace(said)
	if said == "" {
		return nil, errors.New("the agent said nothing")
	}
	if shape == nil {
		return said, nil
	}
	start, end := strings.Index(said, "{"), strings.LastIndex(said, "}")
	if start < 0 || end < start {
		return nil, fmt.Errorf("the agent did not answer with a JSON object: %s", clipped(said, 200))
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(said[start:end+1]), &got); err != nil {
		return nil, fmt.Errorf("answer is not JSON: %v", err)
	}
	out := map[string]any{}
	for _, k := range shape.fields() {
		v, ok := got[k]
		if !ok || v == nil {
			return nil, fmt.Errorf("answer has no %s", k)
		}
		bad := fmt.Errorf("answer's %s is not %s: %v", k, shape.said(k), v)
		switch t := shape[k].(type) {
		case string:
			switch t {
			case "bool":
				b, ok := v.(bool)
				if !ok {
					return nil, bad
				}
				out[k] = b
			case "int":
				f, ok := v.(float64)
				if !ok || f != math.Trunc(f) {
					return nil, bad
				}
				out[k] = f
			case "number":
				f, ok := v.(float64)
				if !ok {
					return nil, bad
				}
				out[k] = f
			case "text":
				s, ok := v.(string)
				if !ok {
					return nil, bad
				}
				out[k] = s
			}
		case []string:
			s, ok := v.(string)
			if !ok {
				return nil, bad
			}
			found := ""
			for _, w := range t {
				if strings.EqualFold(w, strings.TrimSpace(s)) {
					found = w
				}
			}
			if found == "" {
				return nil, bad
			}
			out[k] = found
		}
	}
	return out, nil
}

func clipped(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ask is ask(question, answer = {...}, about = [...]): one agent turn
// asked, the run parked till it's answered; then what it said, in the
// shape asked (words, without one), or None where there's no answer.
func (r *run) ask(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if !r.a.Allows(MayAsk) {
		return nil, fmt.Errorf("ask: not allowed without may = [\"ask\"]")
	}
	var question string
	var answer starlark.Value
	var about *starlark.List
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "question", &question, "answer?", &answer, "about?", &about); err != nil {
		return nil, err
	}
	if strings.TrimSpace(question) == "" {
		return nil, fmt.Errorf("ask: question is empty")
	}
	shape, err := shapeOf(answer)
	if err != nil {
		return nil, fmt.Errorf("ask: %w", err)
	}
	r.steps++
	n := r.steps
	state, result, err := r.stepState(n)
	switch {
	case err != nil:
		return nil, err
	case state == "done":
		return r.answered(result)
	case state == "waiting":
		return nil, errParked
	case state == "started":
		return nil, errUnsure
	}
	if r.dry != nil {
		// A dry run asks no one: what it'd ask is said, and it's None.
		r.dry.did = append(r.dry.did, "ask: "+clipped(question, 120))
		kept, _ := json.Marshal(outcome{Why: "not asked in a dry run"})
		if err := r.keepStep(n, "ask", "done", string(kept)); err != nil {
			return nil, err
		}
		return starlark.None, nil
	}
	shown := map[string]any{}
	if r.e.Hooks.Show != nil {
		for _, ref := range stringsOf(about) {
			if got, err := r.e.Hooks.Show(ref); err == nil && got != nil {
				shown[ref] = got
			}
		}
	}
	prompt := askPrompt(r.a.Name, question, shown, shape)
	now := r.e.now()
	err = write(r.e.Root, func(tx *store.Tx) error {
		shapeJSON := ""
		if shape != nil {
			b, _ := json.Marshal(shape)
			shapeJSON = string(b)
		}
		res, err := tx.Exec(`INSERT INTO automation_asks (name, event, prompt, shape, state, queued) VALUES (?, ?, ?, ?, ?, ?)`,
			r.a.Name, r.ev.N, prompt, shapeJSON, TaskQueued, stampOf(now))
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		kept, _ := json.Marshal(waitFor{Items: []waitItem{{Ask: id}}, Any: true, After: r.ev.N, Due: now.Add(askWaitsFor)})
		// Asked and parked at once: one without the other would ask twice,
		// or wait on nothing.
		_, err = tx.Exec(`INSERT INTO automation_steps (name, event, step, builtin, state, result, at) VALUES (?, ?, ?, 'ask', 'waiting', ?, ?)
			ON CONFLICT (name, event, step) DO UPDATE SET state = excluded.state, result = excluded.result, at = excluded.at`,
			r.a.Name, r.ev.N, n, string(kept), r.at)
		return err
	})
	if err != nil {
		return nil, err
	}
	return nil, errParked
}

// answered is what an ask came to (as its step keeps it), as the script's
// given it: the answer, or None, the why in the run's log; over budget,
// the run fails.
func (r *run) answered(result string) (starlark.Value, error) {
	var o outcome
	if strings.TrimSpace(result) == "null" {
		o.Why = fmt.Sprintf("no answer within %s", shortDuration(askWaitsFor))
	} else if err := json.Unmarshal([]byte(result), &o); err != nil {
		return nil, fmt.Errorf("ask: could not read the result: %w", err)
	}
	if strings.HasPrefix(o.Why, overBudget) {
		return nil, fmt.Errorf("ask: %s", o.Why)
	}
	if o.Answer == nil {
		if r.dry == nil {
			r.log = append(r.log, "ask: no answer: "+o.Why)
		}
		return starlark.None, nil
	}
	return valueOf(o.Answer), nil
}

// Asks are the asks in state ("" for any), oldest first.
func Asks(root, state string) ([]AskInfo, error) {
	var out []AskInfo
	err := read(root, func(tx *store.Tx) error {
		q, args := `SELECT id, name, event, prompt, shape, state, answer, why FROM automation_asks`, []any{}
		if state != "" {
			q, args = q+` WHERE state = ?`, append(args, state)
		}
		rows, err := tx.Query(q+` ORDER BY id`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a AskInfo
			var shape string
			if err := rows.Scan(&a.ID, &a.Name, &a.Event, &a.Prompt, &shape, &a.State, &a.Answer, &a.Why); err != nil {
				return err
			}
			if shape != "" {
				_ = json.Unmarshal([]byte(shape), &a.Shape)
				a.Shape = normalShape(a.Shape)
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}

// normalShape is a shape read back from JSON: its lists of words as
// []string again.
func normalShape(s Shape) Shape {
	for k, v := range s {
		if l, ok := v.([]any); ok {
			words := make([]string, 0, len(l))
			for _, w := range l {
				if str, ok := w.(string); ok {
					words = append(words, str)
				}
			}
			s[k] = words
		}
	}
	return s
}

// askByID is one ask.
func askByID(root string, id int64) (AskInfo, bool) {
	var a AskInfo
	found := false
	_ = read(root, func(tx *store.Tx) error {
		if tx.QueryRow(`SELECT id, name, state, answer, why FROM automation_asks WHERE id = ?`, id).Scan(&a.ID, &a.Name, &a.State, &a.Answer, &a.Why) == nil {
			found = true
		}
		return nil
	})
	return a, found
}

func setAsk(root string, id int64, state string, at time.Time) error {
	col := "started"
	if state == TaskQueued {
		col = "queued"
	}
	return write(root, func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE automation_asks SET state = ?, `+col+` = ? WHERE id = ?`, state, stampOf(at), id)
		return err
	})
}

func endAsk(root string, id int64, answer any, why string, at time.Time) error {
	b, _ := json.Marshal(answer)
	return write(root, func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE automation_asks SET state = ?, answer = ?, why = ?, ended = ? WHERE id = ?`, TaskEnded, string(b), why, stampOf(at), id)
		return err
	})
}

// dayStart is the start of t's day, UTC: what a day's budget counts from.
func dayStart(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// asksPass moves the asks along: one left running by a ws that stopped is
// asked again, and what's queued is asked as there's room and budget, each
// turn run beside the pass, its answer kept as it comes. It writes only
// what's moved.
func (e *Engine) asksPass() []string {
	// Read with the answers held off: one ending between the read and
	// the look at what's being answered would be taken to be left over.
	e.amu.Lock()
	defer e.amu.Unlock()
	running, err := Asks(e.Root, TaskRunning)
	if err != nil {
		return nil
	}
	queued, _ := Asks(e.Root, TaskQueued)
	if len(running) == 0 && len(queued) == 0 {
		return nil
	}
	if e.asking == nil {
		e.asking = map[int64]bool{}
	}
	now := e.now()
	busy := 0
	for _, a := range running {
		if e.asking[a.ID] {
			busy++
			continue
		}
		// Running, but not by this ws: one before it stopped.
		_ = setAsk(e.Root, a.ID, TaskQueued, now)
		queued = append(queued, a)
	}
	sort.Slice(queued, func(i, j int) bool { return queued[i].ID < queued[j].ID })
	budget := e.AskBudget
	if budget == 0 {
		budget = DefaultAskBudget
	}
	var said []string
	for _, a := range queued {
		if busy >= MaxAsks {
			break
		}
		if e.Hooks.Ask == nil {
			_ = endAsk(e.Root, a.ID, nil, "no agent available to ask", now)
			continue
		}
		if budget > 0 && e.Hooks.Spent != nil {
			if spent, err := e.Hooks.Spent(a.Name, dayStart(now)); err == nil && spent >= budget {
				why := fmt.Sprintf("%s: spent %s of %s tokens today ([automations] ask_budget)", overBudget, tokens(spent), tokens(budget))
				_ = endAsk(e.Root, a.ID, nil, why, now)
				said = append(said, fmt.Sprintf("%s's ask: %s", a.Name, why))
				continue
			}
		}
		_ = setAsk(e.Root, a.ID, TaskRunning, now)
		e.asking[a.ID] = true
		busy++
		said = append(said, fmt.Sprintf("%s asked an agent", a.Name))
		e.inFlight.Add(1)
		go e.answer(a)
	}
	return said
}

// answer has a's turn run, and keeps what it came to: its answer, read
// as a's shape, or why there's none. A ws that stops first leaves it to
// be asked again.
func (e *Engine) answer(a AskInfo) {
	defer e.inFlight.Done()
	said, err := e.Hooks.Ask(&Automation{Name: a.Name}, a.Prompt)
	e.amu.Lock()
	defer e.amu.Unlock()
	delete(e.asking, a.ID)
	now := e.now()
	switch {
	case errors.Is(err, ErrAskAgain):
		_ = setAsk(e.Root, a.ID, TaskQueued, now)
	case err != nil:
		_ = endAsk(e.Root, a.ID, nil, "the agent could not answer: "+err.Error(), now)
	default:
		v, err := readAnswer(said, a.Shape)
		if err != nil {
			_ = endAsk(e.Root, a.ID, nil, err.Error(), now)
			return
		}
		_ = endAsk(e.Root, a.ID, v, "", now)
	}
}

// Quiet waits till no ask's being answered: for a daemon stopping, and
// tests.
func (e *Engine) Quiet() { e.inFlight.Wait() }

// tokens is n tokens, short: 1.2k, 250k, 1.5M.
func tokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1e6), ".0") + "M"
	case n >= 1000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1e3), ".0") + "k"
	}
	return fmt.Sprint(n)
}
