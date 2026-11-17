package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// A crew's transcript, read from where its harness keeps the session, as
// it goes: what it was sent, what it said, and each tool call it made. The
// web UI shows it beside a running crew's terminal, and instead of one
// that's ended. Claude Code and pi keep sessions as JSON lines ws can read;
// Cursor keeps its chats in a database it can't (yet).

// TranscriptEntry is one thing in a session: a message sent to the agent
// (By "user"), one it wrote ("agent"), or a tool call it made ("call"),
// with the start of what came back.
type TranscriptEntry struct {
	By      string    `json:"by"`
	At      time.Time `json:"at,omitzero"` // when it was written, if the harness says
	Text    string    `json:"text,omitempty"`
	ID      string    `json:"id,omitempty"` // a call's id
	Tool    string    `json:"tool,omitempty"`
	Input   string    `json:"input,omitempty"`
	Result  string    `json:"result,omitempty"`
	Error   bool      `json:"error,omitempty"`
	Refused bool      `json:"refused,omitempty"`
	// Pending: a call nothing has come back for yet. While crew run, it's
	// the step in hand (or the one waiting for your permission).
	Pending bool `json:"pending,omitempty"`
	// What some calls are, as data, for the web UI to draw: an edit to a
	// file, the agent's plan (its todo list), a question it put to you.
	Edit      *TranscriptEdit      `json:"edit,omitempty"`
	Todos     []TranscriptTodo     `json:"todos,omitempty"`
	Questions []TranscriptQuestion `json:"questions,omitempty"`
}

// TranscriptEdit is a call that changes a file: Write (Old empty, the file
// whole) or Edit (the text it replaced, and with what).
type TranscriptEdit struct {
	Path string `json:"path"`
	Old  string `json:"old,omitempty"`
	New  string `json:"new"`
	All  bool   `json:"all,omitempty"` // every match replaced
}

// TranscriptTodo is an item of the agent's plan.
type TranscriptTodo struct {
	Content string `json:"content"`
	Status  string `json:"status"`           // pending, in_progress, completed
	Active  string `json:"active,omitempty"` // as it reads while in hand
}

// TranscriptQuestion is a question the agent asked with its question tool.
type TranscriptQuestion struct {
	Question string   `json:"question"`
	Header   string   `json:"header,omitempty"`
	Options  []string `json:"options,omitempty"`
	Multi    bool     `json:"multi,omitempty"`
}

// homeDir is where harnesses keep their sessions; tests replace it.
var homeDir = os.UserHomeDir

// transcriptTextMax keeps a message to its start.
const transcriptTextMax = 8 << 10

// SessionFile is where a harness keeps a session's transcript, if it keeps
// one ws can read: Claude Code's in ~/.claude/projects/<folder>/<id>.jsonl,
// pi's in ~/.pi/agent/sessions/<folder>/<time>_<id>.jsonl. Found by id,
// whichever folder it ran in.
func SessionFile(harnessName, id string) (string, bool) {
	home, err := homeDir()
	if err != nil || id == "" || strings.ContainsAny(id, `/\*?[`) {
		return "", false
	}
	var pattern string
	switch harnessName {
	case "claude":
		pattern = filepath.Join(home, ".claude", "projects", "*", id+".jsonl")
	case "pi":
		pattern = filepath.Join(home, ".pi", "agent", "sessions", "*", "*_"+id+".jsonl")
	default:
		return "", false
	}
	found, _ := filepath.Glob(pattern)
	if len(found) == 0 {
		return "", false
	}
	return found[0], true
}

// ReadTranscript reads a session's transcript, as its harness keeps it.
func ReadTranscript(harnessName string, data []byte) []TranscriptEntry {
	if CapsOf(harnessName).Transcript == TranscriptACP {
		return readACPTranscript(data) // ws's own record of the session
	}
	switch harnessName {
	case "claude":
		return readClaudeTranscript(data)
	case "pi":
		return readPiTranscript(data)
	}
	return nil
}

// transcript collects entries, pairing each call with its result.
type transcript struct {
	out []TranscriptEntry
	at  map[string]int // call id: index in out
}

func (t *transcript) say(by, text string, at time.Time) {
	if text = strings.TrimSpace(text); text != "" {
		t.out = append(t.out, TranscriptEntry{By: by, At: at, Text: ClipHead(text, transcriptTextMax, "…")})
	}
}

func (t *transcript) call(id, tool string, input json.RawMessage, at time.Time) {
	if t.at == nil {
		t.at = map[string]int{}
	}
	t.at[id] = len(t.out)
	e := TranscriptEntry{By: "call", At: at, ID: id, Tool: tool, Input: callInput(input), Pending: true}
	callDetail(&e, input)
	t.out = append(t.out, e)
}

// editMax is the most of an edit's text an entry keeps.
const editMax = 8 << 10

// callDetail keeps what a call is as data, for the calls the web UI draws
// as themselves: edits, the plan, questions.
func callDetail(e *TranscriptEntry, raw json.RawMessage) {
	var in struct {
		FilePath  string `json:"file_path"`
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
		Replace   bool   `json:"replace_all"`
		Content   string `json:"content"`
		Todos     []struct {
			Content    string `json:"content"`
			Status     string `json:"status"`
			ActiveForm string `json:"activeForm"`
		} `json:"todos"`
		Questions []struct {
			Question    string `json:"question"`
			Header      string `json:"header"`
			MultiSelect bool   `json:"multiSelect"`
			Options     []struct {
				Label string `json:"label"`
			} `json:"options"`
		} `json:"questions"`
	}
	if json.Unmarshal(raw, &in) != nil {
		return
	}
	path := in.FilePath
	if path == "" {
		path = in.Path
	}
	switch strings.ToLower(e.Tool) {
	case "edit":
		e.Edit = &TranscriptEdit{Path: path, Old: ClipHead(in.OldString, editMax, "…"), New: ClipHead(in.NewString, editMax, "…"), All: in.Replace}
	case "write":
		e.Edit = &TranscriptEdit{Path: path, New: ClipHead(in.Content, editMax, "…")}
	case "todowrite":
		for _, td := range in.Todos {
			e.Todos = append(e.Todos, TranscriptTodo{Content: td.Content, Status: td.Status, Active: td.ActiveForm})
		}
	case "askuserquestion":
		for _, q := range in.Questions {
			tq := TranscriptQuestion{Question: q.Question, Header: q.Header, Multi: q.MultiSelect}
			for _, o := range q.Options {
				tq.Options = append(tq.Options, o.Label)
			}
			e.Questions = append(e.Questions, tq)
		}
	}
}

func (t *transcript) result(id, text string, isErr bool) {
	i, ok := t.at[id]
	if !ok {
		return
	}
	e := &t.out[i]
	e.Result, e.Error, e.Pending = ClipHead(strings.TrimSpace(text), resultMax, "…"), isErr, false
	// Refused: by its permission rules, or by you at its prompt.
	e.Refused = isErr && (strings.HasPrefix(text, "Permission to use ") || strings.HasPrefix(text, piRefused) || strings.HasPrefix(text, "The user doesn't want to proceed"))
}

// readClaudeTranscript reads Claude Code's session file: user and
// assistant lines, each a message whose content is text or blocks (text,
// tool_use, tool_result); the rest of its lines are bookkeeping.
func readClaudeTranscript(data []byte) []TranscriptEntry {
	var t transcript
	for _, line := range strings.Split(string(data), "\n") {
		var e struct {
			Type        string    `json:"type"`
			IsMeta      bool      `json:"isMeta"`
			IsSidechain bool      `json:"isSidechain"`
			Timestamp   time.Time `json:"timestamp"`
			Message     struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &e) != nil || (e.Type != "user" && e.Type != "assistant") || e.IsMeta || e.IsSidechain {
			continue
		}
		by := map[string]string{"user": "user", "assistant": "agent"}[e.Type]
		var text string
		if json.Unmarshal(e.Message.Content, &text) == nil {
			t.say(by, text, e.Timestamp)
			continue
		}
		var blocks []struct {
			Type    string          `json:"type"`
			Text    string          `json:"text"`
			ID      string          `json:"id"`
			Name    string          `json:"name"`
			Input   json.RawMessage `json:"input"`
			UseID   string          `json:"tool_use_id"`
			Content json.RawMessage `json:"content"`
			IsError bool            `json:"is_error"`
		}
		json.Unmarshal(e.Message.Content, &blocks)
		for _, b := range blocks {
			switch b.Type {
			case "text":
				t.say(by, b.Text, e.Timestamp)
			case "tool_use":
				t.call(b.ID, b.Name, b.Input, e.Timestamp)
			case "tool_result":
				t.result(b.UseID, resultText(b.Content), b.IsError)
			}
		}
	}
	return t.out
}

// readPiTranscript reads pi's session file: message entries, each a user's,
// the assistant's (text and toolCall blocks) or a tool's result.
func readPiTranscript(data []byte) []TranscriptEntry {
	var t transcript
	for _, line := range strings.Split(string(data), "\n") {
		var e struct {
			Type      string    `json:"type"`
			Timestamp time.Time `json:"timestamp"`
			Message   struct {
				Role       string          `json:"role"`
				Content    json.RawMessage `json:"content"`
				ToolCallID string          `json:"toolCallId"`
				IsError    bool            `json:"isError"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &e) != nil || e.Type != "message" {
			continue
		}
		var blocks []struct {
			Type      string          `json:"type"`
			Text      string          `json:"text"`
			ID        string          `json:"id"`
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		var text string
		if json.Unmarshal(e.Message.Content, &text) == nil {
			blocks = append(blocks, struct {
				Type      string          `json:"type"`
				Text      string          `json:"text"`
				ID        string          `json:"id"`
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}{Type: "text", Text: text})
		} else {
			json.Unmarshal(e.Message.Content, &blocks)
		}
		switch e.Message.Role {
		case "user":
			for _, b := range blocks {
				if b.Type == "text" {
					t.say("user", b.Text, e.Timestamp)
				}
			}
		case "assistant":
			for _, b := range blocks {
				switch b.Type {
				case "text":
					t.say("agent", b.Text, e.Timestamp)
				case "toolCall":
					t.call(b.ID, b.Name, b.Arguments, e.Timestamp)
				}
			}
		case "toolResult":
			var parts []string
			for _, b := range blocks {
				parts = append(parts, b.Text)
			}
			t.result(e.Message.ToolCallID, strings.Join(parts, "\n"), e.Message.IsError)
		}
	}
	return t.out
}
