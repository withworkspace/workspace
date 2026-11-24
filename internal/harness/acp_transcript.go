package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/acp"
)

// readACPTranscript reads ws's record of an ACP crew's session (as the deck
// server writes it) as the crew page's transcript: what was sent, what the
// agent said, each tool call as it now stands (an edit as its diff, a
// command with its output, refused ones marked, the one in hand pending),
// the agent's latest plan where it last said it, the questions it asked,
// and notices (a turn interrupted) as a harness's are.
func readACPTranscript(data []byte) []TranscriptEntry {
	events, _ := acp.ReadRecord(bytes.NewReader(data))
	var tr acp.Transcript
	asked := map[string]acp.Ask{}
	refused := map[string]bool{}
	output := map[string]*strings.Builder{}
	// extra are entries that aren't the transcript's items (a plan, a
	// question, a notice), by the number of items there were when each
	// came: they go in after those.
	type extra struct {
		at    int
		entry TranscriptEntry
	}
	var extras []extra
	plan := -1 // which of extras is the latest plan
	questions := map[string]int{}
	for _, e := range events {
		switch {
		case e.Sent != nil:
			tr.Sent(e.At, e.Sent...)
		case e.Update != nil:
			u, err := acp.ParseUpdate(e.Update)
			if err != nil {
				continue
			}
			tr.Apply(e.At, u)
			if id, out := terminalOutput(e.Update); id != "" {
				if output[id] == nil {
					output[id] = &strings.Builder{}
				}
				output[id].WriteString(out)
			}
			if u.Update == acp.UpdatePlan {
				todos := make([]TranscriptTodo, len(u.Entries))
				for i, p := range u.Entries {
					todos[i] = TranscriptTodo{Content: p.Content, Status: TodoStatus(p.Status)}
				}
				if plan >= 0 {
					extras[plan].entry.Todos = nil // only the latest stands
				}
				plan = len(extras)
				extras = append(extras, extra{len(tr.Items), TranscriptEntry{By: "call", At: e.At, Tool: "Plan", Input: "the plan", Todos: todos}})
			}
		case e.Ask != nil:
			asked[e.Ask.ID] = *e.Ask
			if q, ok := askQuestion(*e.Ask); ok {
				questions[e.Ask.ID] = len(extras)
				extras = append(extras, extra{len(tr.Items), TranscriptEntry{By: "call", At: e.At, ID: e.Ask.ID, Tool: "Question",
					Input: q.Question, Pending: true, Questions: []TranscriptQuestion{q}}})
			}
		case e.Answer != nil:
			ask, ok := asked[e.Answer.Ask]
			if ok && ask.Permission != nil && !permissionAllowed(*ask.Permission, *e.Answer) {
				refused[ask.Permission.ToolCall.ToolCallID] = true
			}
			if i, ok := questions[e.Answer.Ask]; ok {
				q := &extras[i].entry
				q.Pending = false
				q.Result = answeredWith(*e.Answer)
				q.Refused = e.Answer.Cancelled
			}
		case e.Stop == "cancelled":
			extras = append(extras, extra{len(tr.Items), TranscriptEntry{By: "user", At: e.At, Text: "[Request interrupted]"}})
		case e.Error != "":
			extras = append(extras, extra{len(tr.Items), TranscriptEntry{By: "user", At: e.At, Text: "[" + ClipHead(e.Error, 300, "…") + "]"}})
		}
	}

	var out []TranscriptEntry
	next := 0
	flush := func(upTo int) {
		for next < len(extras) && extras[next].at <= upTo {
			if x := extras[next].entry; x.Tool != "Plan" || len(x.Todos) > 0 {
				out = append(out, x)
			}
			next++
		}
	}
	for i, it := range tr.Items {
		flush(i)
		switch it.Kind {
		case acp.ItemUser:
			if t := strings.TrimSpace(it.Text); t != "" {
				out = append(out, TranscriptEntry{By: "user", At: it.At, Text: ClipHead(t, transcriptTextMax, "…")})
			}
		case acp.ItemAgent:
			if t := strings.TrimSpace(it.Text); t != "" {
				out = append(out, TranscriptEntry{By: "agent", At: it.At, Text: ClipHead(t, transcriptTextMax, "…")})
			}
		case acp.ItemNotice:
			out = append(out, TranscriptEntry{By: "user", At: it.At, Text: "[" + it.Text + "]"})
		case acp.ItemTool:
			c := *it.Tool
			tc := acpCall(c, output[c.ID], refused[c.ID])
			e := TranscriptEntry{By: "call", At: it.At, ID: c.ID, Tool: tc.Tool, Input: tc.Input, Result: tc.Result,
				Error: tc.Error, Refused: tc.Refused, Pending: !c.Done() && !tc.Refused}
			for _, ct := range c.Content {
				if ct.Type == "diff" {
					old := ""
					if ct.OldText != nil {
						old = *ct.OldText
					}
					e.Edit = &TranscriptEdit{Path: ct.Path, Old: ClipHead(old, editMax, "…"), New: ClipHead(ct.NewText, editMax, "…")}
					break
				}
			}
			out = append(out, e)
		}
	}
	flush(len(tr.Items))
	return out
}

// askQuestion is an ask that's a question for a person: a form (Codex's
// questions come as forms, a field each), or Cursor's question tool.
func askQuestion(a acp.Ask) (TranscriptQuestion, bool) {
	switch {
	case a.Form != nil:
		q := TranscriptQuestion{Question: strings.TrimSpace(a.Form.Message)}
		if f, ok := formField(a.Form.RequestedSchema); ok {
			if q.Question == "" {
				q.Question = f.title
			}
			q.Options = f.options
		}
		return q, q.Question != ""
	case a.Method == "cursor/ask_question":
		var p cursorQuestions
		if json.Unmarshal(a.Params, &p) != nil || len(p.Questions) == 0 {
			return TranscriptQuestion{}, false
		}
		first := p.Questions[0]
		q := TranscriptQuestion{Question: strings.TrimSpace(first.Prompt), Header: p.Title, Multi: first.AllowMultiple}
		for _, o := range first.Options {
			q.Options = append(q.Options, o.Label)
		}
		return q, q.Question != ""
	}
	return TranscriptQuestion{}, false
}

// cursorQuestions is cursor/ask_question's request.
type cursorQuestions struct {
	ToolCallID string `json:"toolCallId"`
	Title      string `json:"title"`
	Questions  []struct {
		ID      string `json:"id"`
		Prompt  string `json:"prompt"`
		Options []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"options"`
		AllowMultiple bool `json:"allowMultiple"`
	} `json:"questions"`
}

// field is a form's first field: its name, what it asks, the answers it
// offers (options, as they read, and values, as they're sent), and whether
// it takes several (a list).
type field struct {
	name, title string
	options     []string
	values      []string
	multi       bool
}

// formField reads a form's first field from its JSON Schema: a property
// with a title (or description), and its values as an enum, or as oneOf
// consts (each with a title).
func formField(schema json.RawMessage) (field, bool) {
	var s struct {
		Properties map[string]struct {
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Enum        []string `json:"enum"`
			OneOf       []struct {
				Const string `json:"const"`
				Title string `json:"title"`
			} `json:"oneOf"`
			Items *struct {
				Enum  []string `json:"enum"`
				AnyOf []struct {
					Const string `json:"const"`
					Title string `json:"title"`
				} `json:"anyOf"`
			} `json:"items"`
		} `json:"properties"`
		Order []string `json:"x-order"`
	}
	if json.Unmarshal(schema, &s) != nil || len(s.Properties) == 0 {
		return field{}, false
	}
	name := ""
	for _, n := range s.Order {
		if _, ok := s.Properties[n]; ok {
			name = n
			break
		}
	}
	if name == "" {
		for n := range s.Properties {
			if name == "" || n < name {
				name = n
			}
		}
	}
	p := s.Properties[name]
	f := field{name: name, title: firstNonEmptyString(p.Title, p.Description, name), multi: p.Items != nil}
	add := func(label, value string) {
		f.options = append(f.options, label)
		f.values = append(f.values, value)
	}
	for _, e := range p.Enum {
		add(e, e)
	}
	for _, o := range p.OneOf {
		add(firstNonEmptyString(o.Title, o.Const), o.Const)
	}
	if p.Items != nil {
		for _, e := range p.Items.Enum {
			add(e, e)
		}
		for _, o := range p.Items.AnyOf {
			add(firstNonEmptyString(o.Title, o.Const), o.Const)
		}
	}
	return f, true
}

// PermissionAnswer is a permission's answer for what a person chose on its
// card: allow (once), always, or deny; the request's own option for each.
// A deny where it offers no reject is a cancel.
func PermissionAnswer(r acp.PermissionRequest, choice string) (acp.Answer, error) {
	var id string
	switch choice {
	case "allow":
		id = r.Option(acp.AllowOnce)
	case "always":
		id = r.Option(acp.AllowAlways)
	case "deny":
		if id = r.Option(acp.RejectOnce, acp.RejectAlways); id == "" {
			return acp.Answer{Cancelled: true}, nil
		}
	}
	if id == "" {
		return acp.Answer{}, fmt.Errorf("it offers no %q", choice)
	}
	return acp.Answer{Option: id}, nil
}

// QuestionAnswer answers a question with one of its options (by how it
// reads) or a person's own words: a form filled in, or Cursor's question
// answered (its own words, which it has no field for, go as the reason it
// was skipped).
func QuestionAnswer(a acp.Ask, option, text string) (acp.Answer, error) {
	switch {
	case a.Form != nil:
		f, ok := formField(a.Form.RequestedSchema)
		if !ok {
			return acp.Answer{}, errors.New("its form has no field to answer")
		}
		v := strings.TrimSpace(text)
		if option != "" {
			i := slices.Index(f.options, option)
			if i < 0 {
				return acp.Answer{}, fmt.Errorf("%q isn't one of its answers", option)
			}
			v = f.values[i]
		}
		var value any = v
		if f.multi {
			value = []any{v}
		}
		return acp.Answer{Form: &acp.ElicitationResponse{Action: "accept", Content: map[string]any{f.name: value}}}, nil
	case a.Method == "cursor/ask_question":
		var p cursorQuestions
		if json.Unmarshal(a.Params, &p) != nil || len(p.Questions) == 0 {
			return acp.Answer{}, errors.New("its question can't be read")
		}
		q := p.Questions[0]
		if option == "" {
			out, _ := json.Marshal(map[string]any{"outcome": map[string]any{"outcome": "skipped",
				"reason": "The user answered in their own words: " + strings.TrimSpace(text)}})
			return acp.Answer{Result: out}, nil
		}
		for _, o := range q.Options {
			if o.Label == option {
				out, _ := json.Marshal(map[string]any{"outcome": map[string]any{"outcome": "answered",
					"answers": []any{map[string]any{"questionId": q.ID, "selectedOptionIds": []string{o.ID}}}}})
				return acp.Answer{Result: out}, nil
			}
		}
		return acp.Answer{}, fmt.Errorf("%q isn't one of its answers", option)
	}
	return acp.Answer{}, errors.New("it isn't a question")
}

// answeredWith is what a question was answered, as the transcript says it.
func answeredWith(a acp.Answer) string {
	switch {
	case a.Cancelled:
		return "Not answered"
	case a.Form != nil && a.Form.Action != "accept":
		return "Declined"
	case a.Form != nil:
		var vals []string
		for _, v := range a.Form.Content {
			if s, ok := v.(string); ok {
				vals = append(vals, s)
			}
		}
		return "Answered: " + strings.Join(vals, ", ")
	case len(a.Result) > 0:
		return "Answered"
	}
	return ""
}

// ACPTranscriptText is an ACP crew's record as plain text, for its saved
// transcript (what the orchestrator reads of crew that have ended, as a
// terminal's last screens are for others): what was sent and said, each
// call with the start of what came back.
func ACPTranscriptText(record []byte) string {
	var b strings.Builder
	for _, e := range readACPTranscript(record) {
		switch e.By {
		case "user":
			b.WriteString("> " + strings.ReplaceAll(e.Text, "\n", "\n> ") + "\n\n")
		case "agent":
			b.WriteString(e.Text + "\n\n")
		case "call":
			fmt.Fprintf(&b, "[%s] %s", e.Tool, e.Input)
			if e.Refused {
				b.WriteString(" (refused)")
			}
			b.WriteString("\n")
			if e.Result != "" {
				b.WriteString(e.Result + "\n")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}
