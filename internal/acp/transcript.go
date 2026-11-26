package acp

import (
	"encoding/json"
	"slices"
	"strings"
	"time"
)

// Transcript is a session's updates folded into what happened, in order:
// messages (their chunks joined), tool calls (each updated in place as
// its updates come), and notices. Beside them, what the session is like
// now: the agent's latest plan, its commands, its mode and settings, its
// title, and how full its context is.
//
// It isn't safe for use by more than one goroutine; Clone gives a copy.
type Transcript struct {
	Items    []Item             `json:"items"`
	Plan     []PlanEntry        `json:"plan,omitempty"`
	Commands []AvailableCommand `json:"commands,omitempty"`
	Mode     string             `json:"mode,omitempty"`
	Config   []ConfigOption     `json:"config,omitempty"`
	Title    string             `json:"title,omitempty"`
	// Used of Size: the context window's tokens, if the agent says.
	Used int64 `json:"used,omitempty"`
	Size int64 `json:"size,omitempty"`

	calls map[string]int // a tool call's item, by id
}

// The kinds of item.
const (
	ItemUser    = "user"    // a message sent to the agent
	ItemAgent   = "agent"   // a message the agent wrote
	ItemThought = "thought" // its thinking, where it shows it
	ItemTool    = "tool"    // a tool call
	ItemNotice  = "notice"  // something the agent wants seen (a warning, say)
)

// Item is one thing in a session.
type Item struct {
	Kind string    `json:"kind"`
	At   time.Time `json:"at,omitzero"` // when it began
	// Text is a message's (its blocks as text), or a notice's title.
	Text      string    `json:"text,omitempty"`
	MessageID string    `json:"messageId,omitempty"`
	Severity  string    `json:"severity,omitempty"` // a notice's: info, warning, error
	Tool      *ToolCall `json:"tool,omitempty"`
}

// ToolCall is a tool call as it stands, every update to it applied.
type ToolCall struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Name      string            `json:"name,omitempty"`
	Kind      string            `json:"kind,omitempty"`   // read, edit, delete, move, search, execute, think, fetch, switch_mode, other
	Status    string            `json:"status,omitempty"` // pending, in_progress, completed, failed
	Content   []ToolCallContent `json:"content,omitempty"`
	Locations []Location        `json:"locations,omitempty"`
	Input     json.RawMessage   `json:"input,omitempty"`
	Output    json.RawMessage   `json:"output,omitempty"`
	Updated   time.Time         `json:"updated,omitzero"`
}

// Tool call statuses.
const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// Done is whether the call has finished, one way or the other.
func (c ToolCall) Done() bool { return c.Status == StatusCompleted || c.Status == StatusFailed }

// Sent adds a message the client sent the agent (its prompt), which the
// agent doesn't echo while a session runs. (Loading a session replays the
// client's messages as user chunks: those come through Apply.)
func (t *Transcript) Sent(at time.Time, blocks ...ContentBlock) {
	t.Items = append(t.Items, Item{Kind: ItemUser, At: at, Text: blocksText(blocks)})
}

func blocksText(blocks []ContentBlock) string {
	var s strings.Builder
	for _, b := range blocks {
		s.WriteString(b.String())
	}
	return s.String()
}

// Apply folds in one update. Kinds it doesn't know (an agent's own) leave
// it as it was; a record keeps them.
func (t *Transcript) Apply(at time.Time, u SessionUpdate) {
	switch u.Update {
	case UpdateUserChunk:
		t.chunk(at, ItemUser, u)
	case UpdateAgentChunk:
		t.chunk(at, ItemAgent, u)
	case UpdateThoughtChunk:
		t.chunk(at, ItemThought, u)
	case UpdateToolCall, UpdateToolCallEdit:
		t.toolCall(at, u)
	case UpdatePlan:
		t.Plan = append([]PlanEntry(nil), u.Entries...)
	case UpdateCommands:
		t.Commands = append([]AvailableCommand(nil), u.AvailableCommands...)
	case UpdateMode:
		t.Mode = u.CurrentModeID
	case UpdateConfigOptions:
		t.Config = append([]ConfigOption(nil), u.ConfigOptions...)
	case UpdateSessionInfo:
		if u.Title != nil {
			t.Title = *u.Title
		}
	case UpdateUsage:
		t.Used, t.Size = u.Used, u.Size
	case UpdateNotice:
		title := ""
		if u.Title != nil {
			title = *u.Title
		}
		t.Items = append(t.Items, Item{Kind: ItemNotice, At: at, Text: title, Severity: u.Severity})
	}
}

// chunk adds a message's chunk: to the message it continues (the last
// item, of its kind, and its message if the agent names them), or as a
// new one.
func (t *Transcript) chunk(at time.Time, kind string, u SessionUpdate) {
	text := u.Block().String()
	if n := len(t.Items); n > 0 {
		last := &t.Items[n-1]
		if last.Kind == kind && last.MessageID == u.MessageID {
			last.Text += text
			return
		}
	}
	t.Items = append(t.Items, Item{Kind: kind, At: at, Text: text, MessageID: u.MessageID})
}

// toolCall adds a tool call, or applies an update to one: what the update
// has replaces what the call had. An update to a call it hasn't seen (or a
// call sent again) is taken as the call.
func (t *Transcript) toolCall(at time.Time, u SessionUpdate) {
	i, ok := t.index()[u.ToolCallID]
	if !ok {
		t.Items = append(t.Items, Item{Kind: ItemTool, At: at, Tool: &ToolCall{ID: u.ToolCallID}})
		i = len(t.Items) - 1
		t.calls[u.ToolCallID] = i
	}
	c := t.Items[i].Tool
	c.Updated = at
	if u.Title != nil {
		c.Title = *u.Title
	}
	if u.Name != "" {
		c.Name = u.Name
	}
	if u.Kind != "" {
		c.Kind = u.Kind
	}
	if u.Status != "" {
		c.Status = u.Status
	}
	if u.Content != nil {
		c.Content = u.ToolContent()
	}
	if u.Locations != nil {
		c.Locations = u.Locations
	}
	if u.RawInput != nil {
		c.Input = u.RawInput
	}
	if u.RawOutput != nil {
		c.Output = u.RawOutput
	}
}

// index is the tool calls' items by id, made from the items when it isn't
// there (a transcript read from JSON).
func (t *Transcript) index() map[string]int {
	if t.calls == nil {
		t.calls = map[string]int{}
		for i, it := range t.Items {
			if it.Tool != nil {
				t.calls[it.Tool.ID] = i
			}
		}
	}
	return t.calls
}

// Call is the tool call of that id, or nil.
func (t *Transcript) Call(id string) *ToolCall {
	if i, ok := t.index()[id]; ok {
		return t.Items[i].Tool
	}
	return nil
}

// Reply is the agent's last message since the last one sent to it: a
// turn's final message, once the turn has ended. "" if it's said nothing
// since.
func (t *Transcript) Reply() string {
	for _, v := range slices.Backward(t.Items) {
		switch v.Kind {
		case ItemAgent:
			return v.Text
		case ItemUser:
			return ""
		}
	}
	return ""
}

// Running are the tool calls not yet finished: while a turn runs, the step
// in hand (or one waiting for permission). A call left unfinished by a
// turn that's ended stays here too.
func (t *Transcript) Running() []ToolCall { return unfinished(t.Items) }

// unfinished are the tool calls among items not yet finished, as they
// stand.
func unfinished(items []Item) []ToolCall {
	var out []ToolCall
	for _, it := range items {
		if it.Tool != nil && !it.Tool.Done() {
			out = append(out, *it.Tool)
		}
	}
	return out
}

// Clone is a copy that shares nothing it could change.
func (t *Transcript) Clone() Transcript {
	c := *t
	c.Items = make([]Item, len(t.Items))
	copy(c.Items, t.Items)
	for i := range c.Items {
		if tc := c.Items[i].Tool; tc != nil {
			cp := *tc
			c.Items[i].Tool = &cp
		}
	}
	c.Plan = append([]PlanEntry(nil), t.Plan...)
	c.Commands = append([]AvailableCommand(nil), t.Commands...)
	c.Config = append([]ConfigOption(nil), t.Config...)
	c.calls = nil // made again when it's needed
	return c
}
