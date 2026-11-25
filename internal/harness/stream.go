package harness

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// What a headless run printed, read back: its final message, the session
// it ran in, each tool call it made (refused ones marked), and why it
// failed, if it did. Claude Code reports all of it as stream-json; other
// harnesses print their final message.

// ToolCall is one tool call a run made, and how it went.
type ToolCall struct {
	Tool    string `json:"tool"`
	Input   string `json:"input"`            // the command, or the tool's input
	Result  string `json:"result,omitempty"` // the start of what came back
	Error   bool   `json:"error,omitempty"`
	Refused bool   `json:"refused,omitempty"` // not on its allowed list
}

// Report is a run's output, read.
type Report struct {
	// Output is its final message (or, cut short, the last thing it said).
	Output  string
	Calls   []ToolCall
	Session string
	// Failed says why the run failed, by its own account: out of turns,
	// or an error. "" when it didn't.
	Failed string
	// Done: its final message is in (a run still going has none).
	Done bool
	// Draft is the message it's writing now, as far as it's got, where its
	// harness says as it goes (Claude Code's partial messages): "" once
	// that message is whole, and Output has it. Thinking: it's thinking
	// before it writes, which says nothing of what it thinks.
	Draft    string
	Thinking bool
	// Usage is what it spent, by model, where its harness says (Claude
	// Code's result line): nil where it doesn't, or it didn't finish.
	Usage []Usage
}

// SoFar is what it's said so far: the message it's writing, else the last
// it wrote. A harness that streams its reply as one message (ACP's) has it
// in Output as it goes.
func (r Report) SoFar() string {
	if r.Draft != "" {
		return r.Draft
	}
	return r.Output
}

// ReadStream reads what a run printed, as its harness reports it.
func ReadStream(stream, out string) Report {
	switch stream {
	case StreamClaude:
		return readClaudeStream(out)
	case StreamCursor:
		return readCursorStream(out)
	case StreamPi:
		return readPiStream(out)
	case StreamACP:
		return readACPStream(out)
	}
	return Report{Output: out, Done: true}
}

// A run file is what a run printed, as it goes, after a first line saying
// how to read it: the page of a conversation reads it while the run's
// going, whichever harness it is.
type runHeader struct {
	Stream string `json:"ws_stream"`
}

// RunHeader is a run file's first line.
func RunHeader(stream string) string {
	data, _ := json.Marshal(runHeader{Stream: stream})
	return string(data) + "\n"
}

// ReadRun reads a run file: what the run has done so far.
func ReadRun(data string) Report {
	first, rest, _ := strings.Cut(data, "\n")
	var h runHeader
	if json.Unmarshal([]byte(first), &h) != nil || h.Stream == "" {
		return ReadStream(StreamClaude, data) // from before run files said
	}
	return ReadStream(h.Stream, rest)
}

// readCursorStream reads cursor-agent's stream-json: the session (system
// init), each tool call (tool_call started, then completed with its
// result: success, rejected or error), each assistant message, and the
// result. Its result joins every message of the run, so the reply is the
// last message.
func readCursorStream(out string) Report {
	var rep Report
	at := map[string]int{}
	model := ""
	for _, line := range strings.Split(out, "\n") {
		var e struct {
			Type      string `json:"type"`
			Subtype   string `json:"subtype"`
			SessionID string `json:"session_id"`
			CallID    string `json:"call_id"`
			IsError   bool   `json:"is_error"`
			Result    string `json:"result"`
			Message   struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
			ToolCall map[string]json.RawMessage `json:"tool_call"`
			Model    string                     `json:"model"` // the init line's
			Usage    *struct {
				Input      int64 `json:"inputTokens"`
				Output     int64 `json:"outputTokens"`
				CacheRead  int64 `json:"cacheReadTokens"`
				CacheWrite int64 `json:"cacheWriteTokens"`
			} `json:"usage"` // the result's
		}
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		if e.SessionID != "" {
			rep.Session = e.SessionID
		}
		switch e.Type {
		case "system":
			if e.Subtype == "init" {
				model = e.Model
			}
		case "assistant":
			var parts []string
			for _, c := range e.Message.Content {
				if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
					parts = append(parts, c.Text)
				}
			}
			if len(parts) > 0 {
				rep.Output = strings.TrimSpace(strings.Join(parts, ""))
			}
		case "tool_call":
			for key, raw := range e.ToolCall {
				name, ok := strings.CutSuffix(key, "ToolCall")
				if !ok {
					continue
				}
				var tc struct {
					Args   json.RawMessage            `json:"args"`
					Result map[string]json.RawMessage `json:"result"`
				}
				json.Unmarshal(raw, &tc)
				i, seen := at[e.CallID]
				if !seen {
					at[e.CallID] = len(rep.Calls)
					i = len(rep.Calls)
					rep.Calls = append(rep.Calls, ToolCall{Tool: strings.ToUpper(name[:1]) + name[1:], Input: callInput(tc.Args)})
				}
				if e.Subtype != "completed" {
					continue
				}
				c := &rep.Calls[i]
				switch {
				case tc.Result["rejected"] != nil:
					c.Error, c.Refused = true, true
				case tc.Result["error"] != nil:
					var er struct {
						Message string `json:"errorMessage"`
					}
					json.Unmarshal(tc.Result["error"], &er)
					c.Error, c.Result = true, ClipHead(er.Message, resultMax, "…")
				case tc.Result["success"] != nil:
					var ok struct {
						Stdout  string `json:"stdout"`
						Content string `json:"content"`
					}
					json.Unmarshal(tc.Result["success"], &ok)
					c.Result = ClipHead(strings.TrimSpace(ok.Stdout+ok.Content), resultMax, "…")
				}
			}
		case "result":
			rep.Done = true
			if e.IsError {
				rep.Failed = "it failed: " + ClipHead(strings.TrimSpace(e.Result), 300, "…")
			}
			if rep.Output == "" {
				rep.Output = strings.TrimSpace(e.Result)
			}
			if u := e.Usage; u != nil {
				// The model the run was started on, which may be Auto:
				// Cursor doesn't say which it picked.
				rep.Usage = addUsage(rep.Usage, Usage{Model: model, Input: u.Input, Output: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite})
			}
		}
	}
	return rep
}

// piRefused is how ws's extension starts a refusal (see converse.go).
const piRefused = "Refused:"

// readPiStream reads pi's --mode json: the session header (its id), each
// tool's execution (start, then end with its result), each message as it
// ends, and agent_settled once pi won't go on by itself.
func readPiStream(out string) Report {
	var rep Report
	at := map[string]int{}
	for _, line := range strings.Split(out, "\n") {
		var e struct {
			Type       string          `json:"type"`
			ID         string          `json:"id"`
			ToolCallID string          `json:"toolCallId"`
			ToolName   string          `json:"toolName"`
			Args       json.RawMessage `json:"args"`
			IsError    bool            `json:"isError"`
			Result     struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
			Message struct {
				piMessage
				Content      json.RawMessage `json:"content"`
				StopReason   string          `json:"stopReason"`
				ErrorMessage string          `json:"errorMessage"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		switch e.Type {
		case "session":
			rep.Session = e.ID
		case "tool_execution_start":
			at[e.ToolCallID] = len(rep.Calls)
			rep.Calls = append(rep.Calls, ToolCall{Tool: e.ToolName, Input: callInput(e.Args)})
		case "tool_execution_end":
			i, ok := at[e.ToolCallID]
			if !ok {
				continue
			}
			var parts []string
			for _, c := range e.Result.Content {
				parts = append(parts, c.Text)
			}
			text := strings.TrimSpace(strings.Join(parts, "\n"))
			c := &rep.Calls[i]
			c.Error = e.IsError
			c.Refused = e.IsError && strings.HasPrefix(text, piRefused)
			c.Result = ClipHead(text, resultMax, "…")
		case "message_end":
			if e.Message.Role != "assistant" {
				continue
			}
			if u, ok := e.Message.usage(); ok {
				rep.Usage = addUsage(rep.Usage, u)
			}
			var blocks []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			json.Unmarshal(e.Message.Content, &blocks)
			var parts []string
			for _, b := range blocks {
				if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
					parts = append(parts, b.Text)
				}
			}
			if len(parts) > 0 {
				rep.Output = strings.TrimSpace(strings.Join(parts, "\n"))
			}
			if e.Message.StopReason == "error" || e.Message.StopReason == "aborted" {
				why := e.Message.ErrorMessage
				if why == "" {
					why = e.Message.StopReason
				}
				rep.Failed = "it failed: " + ClipHead(why, 300, "…")
			}
		case "agent_settled":
			rep.Done = true
		}
	}
	return rep
}

// resultMax keeps what a call returned to its start.
const resultMax = 400

// readClaudeStream reads Claude Code's stream-json: the session, each
// message as it ends (its text, and its tool_use blocks), each tool_result,
// and the result. With --include-partial-messages, a message's text also
// comes as it's written (stream_event text_delta), which is the Draft
// until the message is whole, and its thinking block's start and stop say
// when it's Thinking (its thinking_delta are empty).
func readClaudeStream(out string) Report {
	var rep Report
	at := map[string]int{} // tool_use_id: index in rep.Calls
	last := ""
	var draft strings.Builder
	thinking := false
	for _, line := range strings.Split(out, "\n") {
		var e struct {
			Type      string `json:"type"`
			Subtype   string `json:"subtype"`
			SessionID string `json:"session_id"`
			Result    string `json:"result"`
			IsError   bool   `json:"is_error"`
			Message   struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
			Denials []struct {
				ID string `json:"tool_use_id"`
			} `json:"permission_denials"`
			// A partial message's event, and the tool call it's within
			// (a subagent's), if it is.
			Event struct {
				Type  string `json:"type"`
				Block struct {
					Type string `json:"type"`
				} `json:"content_block"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			} `json:"event"`
			Parent string `json:"parent_tool_use_id"`
		}
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		if e.SessionID != "" {
			rep.Session = e.SessionID
		}
		if e.Type == "stream_event" {
			switch {
			case e.Parent != "":
			case e.Event.Type == "message_start":
				draft.Reset()
			case e.Event.Type == "content_block_start":
				thinking = e.Event.Block.Type == "thinking"
			case e.Event.Type == "content_block_stop":
				thinking = false
			case e.Event.Type == "content_block_delta" && e.Event.Delta.Type == "text_delta":
				draft.WriteString(e.Event.Delta.Text)
			}
			continue
		}
		if e.Type == "assistant" || e.Type == "result" {
			// What was being written is whole now, in this line.
			draft.Reset()
			thinking = false
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
		json.Unmarshal(e.Message.Content, &blocks) // a user message's content may be a plain string
		switch e.Type {
		case "assistant":
			for _, b := range blocks {
				switch b.Type {
				case "text":
					if t := strings.TrimSpace(b.Text); t != "" {
						last = t
					}
				case "tool_use":
					at[b.ID] = len(rep.Calls)
					rep.Calls = append(rep.Calls, ToolCall{Tool: b.Name, Input: callInput(b.Input)})
				}
			}
		case "user":
			for _, b := range blocks {
				i, ok := at[b.UseID]
				if b.Type != "tool_result" || !ok {
					continue
				}
				text := resultText(b.Content)
				c := &rep.Calls[i]
				c.Error = b.IsError
				c.Refused = b.IsError && strings.HasPrefix(text, "Permission to use ")
				c.Result = ClipHead(text, resultMax, "…")
			}
		case "result":
			rep.Done = true
			rep.Output = strings.TrimSpace(e.Result)
			rep.Usage = claudeResultUsage([]byte(line))
			for _, d := range e.Denials {
				if i, ok := at[d.ID]; ok {
					rep.Calls[i].Refused = true
				}
			}
			switch {
			case e.Subtype == "error_max_turns":
				rep.Failed = "it ran out of turns before answering"
			case e.IsError:
				rep.Failed = "it failed: " + ClipHead(strings.TrimSpace(e.Result), 300, "…")
			}
		}
	}
	if rep.Output == "" {
		rep.Output = last
	}
	if !rep.Done {
		rep.Draft, rep.Thinking = strings.TrimSpace(draft.String()), thinking
	}
	return rep
}

// callInput is a tool call's input in a line: a command as it is, else its
// fields.
func callInput(raw json.RawMessage) string {
	var in map[string]any
	if json.Unmarshal(raw, &in) != nil {
		return string(raw)
	}
	if cmd, ok := in["command"].(string); ok {
		return cmd
	}
	for _, k := range []string{"file_path", "pattern", "path"} {
		if v, ok := in[k].(string); ok {
			return v
		}
	}
	return ClipHead(string(raw), 200, "…")
}

// resultText is a tool result's text: a string, or text blocks.
func resultText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	json.Unmarshal(raw, &blocks)
	var parts []string
	for _, b := range blocks {
		parts = append(parts, b.Text)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// ClipHead keeps the start of s within limit bytes, at a line break where
// one is near, never splitting a character, and appends more if anything
// went.
func ClipHead(s string, limit int, more string) string {
	if len(s) <= limit {
		return s
	}
	cut := max(0, limit-len(more))
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	head := s[:cut]
	if i := strings.LastIndexByte(head, '\n'); i > len(head)/2 {
		head = head[:i+1]
	}
	return head + more
}
