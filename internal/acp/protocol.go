package acp

import (
	"encoding/json"
	"strings"
)

// What ACP (v1) sends, as ws reads it. Each type keeps the fields ws uses;
// a session update is also kept whole (Raw), so a record loses nothing.

// ContentBlock is a piece of a message: text (all ws sends), an image, or a
// resource (a file, by its URI, perhaps with its text).
type ContentBlock struct {
	Type     string `json:"type"` // text, image, audio, resource_link, resource
	Text     string `json:"text,omitempty"`
	URI      string `json:"uri,omitempty"`
	Name     string `json:"name,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // an image's or audio's, base64
	// Resource is an embedded resource: its URI, and its text if it's text.
	Resource *struct {
		URI  string `json:"uri"`
		Text string `json:"text,omitempty"`
	} `json:"resource,omitempty"`
}

// Text is a text block.
func Text(s string) ContentBlock { return ContentBlock{Type: "text", Text: s} }

// String is the block as text: its text, or what it is.
func (b ContentBlock) String() string {
	switch b.Type {
	case "text":
		return b.Text
	case "resource_link":
		return "[" + firstOf(b.Name, b.URI) + "]"
	case "resource":
		if b.Resource != nil {
			if b.Resource.Text != "" {
				return b.Resource.Text
			}
			return "[" + b.Resource.URI + "]"
		}
	}
	return "[" + b.Type + "]"
}

func firstOf(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

// ToolCallContent is what a tool call produced or proposes: content (a
// block), a diff of a file, or a terminal it runs in (the client's, which
// ws doesn't offer).
type ToolCallContent struct {
	Type       string        `json:"type"` // content, diff, terminal
	Content    *ContentBlock `json:"content,omitempty"`
	Path       string        `json:"path,omitempty"`
	OldText    *string       `json:"oldText,omitempty"` // nil: a new file
	NewText    string        `json:"newText,omitempty"`
	TerminalID string        `json:"terminalId,omitempty"`
}

// Location is a file (and line) a tool call is about.
type Location struct {
	Path string `json:"path"`
	Line *int   `json:"line,omitempty"`
}

// PlanEntry is an item of the agent's plan.
type PlanEntry struct {
	Content  string `json:"content"`
	Priority string `json:"priority,omitempty"` // high, medium, low
	Status   string `json:"status"`             // pending, in_progress, completed
}

// AvailableCommand is a command the agent offers (a slash command), sent
// as a message starting with /name.
type AvailableCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Input       *struct {
		Hint string `json:"hint"`
	} `json:"input,omitempty"`
}

// Mode is one of a session's modes (Cursor's: agent, plan, ask).
type Mode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Modes are a session's modes and the one it's in.
type Modes struct {
	Current   string `json:"currentModeId"`
	Available []Mode `json:"availableModes"`
}

// ConfigOption is a setting of a session the client may change: its model
// or mode, say (Category). A select's values are in Values, groups
// flattened; a boolean's are true and false.
type ConfigOption struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Category string          `json:"category,omitempty"` // mode, model, model_config, thought_level, or the agent's own
	Type     string          `json:"type"`               // select, boolean
	Current  json.RawMessage `json:"currentValue,omitempty"`
	Values   []ConfigValue   `json:"options,omitempty"`
}

// ConfigValue is one value a select may take.
type ConfigValue struct {
	Value       string `json:"value"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (o *ConfigOption) UnmarshalJSON(data []byte) error {
	type plain ConfigOption
	var v struct {
		plain
		Options []struct {
			ConfigValue
			Group   string        `json:"group"`
			Options []ConfigValue `json:"options"`
		} `json:"options"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*o = ConfigOption(v.plain)
	for _, opt := range v.Options {
		if opt.Group != "" || opt.Options != nil {
			o.Values = append(o.Values, opt.Options...)
		} else {
			o.Values = append(o.Values, opt.ConfigValue)
		}
	}
	return nil
}

// CurrentValue is a select's current value ("" for a boolean).
func (o ConfigOption) CurrentValue() string {
	var s string
	json.Unmarshal(o.Current, &s) //nolint:errcheck // not a string: ""
	return s
}

// Pick is the value of o's that want names: its value or name, or the
// start of either, ignoring case; a value's bare form (before any [...]
// it carries) counts as its value. "auto" finds Cursor's "default[]",
// named Auto. "" for none.
func (o ConfigOption) Pick(want string) string {
	w := strings.ToLower(strings.TrimSpace(want))
	if w == "" {
		return ""
	}
	for _, exact := range []bool{true, false} {
		for _, v := range o.Values {
			id, name := strings.ToLower(v.Value), strings.ToLower(v.Name)
			bare := strings.SplitN(id, "[", 2)[0]
			if exact && (id == w || bare == w || name == w) {
				return v.Value
			}
			if !exact && (strings.HasPrefix(id, w) || strings.HasPrefix(name, w)) {
				return v.Value
			}
		}
	}
	return ""
}

// SessionUpdate is what the agent tells the client as a session goes: a
// chunk of a message (its, or yours replayed), a tool call or an update to
// one, its plan, its mode, its commands, its settings, a notice. Fields are
// those of whichever kind it is.
type SessionUpdate struct {
	Update string `json:"sessionUpdate"`
	// Message chunks: a ContentBlock, and the message it's part of (if the
	// agent says). Tool calls: []ToolCallContent.
	Content   json.RawMessage `json:"content,omitempty"`
	MessageID string          `json:"messageId,omitempty"`

	ToolCallID string          `json:"toolCallId,omitempty"`
	Title      *string         `json:"title,omitempty"` // a tool call's, a session's (session_info_update), a notice's
	Name       string          `json:"name,omitempty"`  // a tool call's tool
	Kind       string          `json:"kind,omitempty"`  // read, edit, delete, move, search, execute, think, fetch, switch_mode, other
	Status     string          `json:"status,omitempty"`
	Locations  []Location      `json:"locations,omitempty"`
	RawInput   json.RawMessage `json:"rawInput,omitempty"`
	RawOutput  json.RawMessage `json:"rawOutput,omitempty"`

	Entries []PlanEntry `json:"entries,omitempty"`

	CurrentModeID     string             `json:"currentModeId,omitempty"`
	AvailableCommands []AvailableCommand `json:"availableCommands,omitempty"`
	ConfigOptions     []ConfigOption     `json:"configOptions,omitempty"`

	// Usage (usage_update): the context window's tokens used, of its size.
	Used int64 `json:"used,omitempty"`
	Size int64 `json:"size,omitempty"`

	Severity string `json:"severity,omitempty"` // a notice's: info, warning, error

	// Raw is the update as it came.
	Raw json.RawMessage `json:"-"`
}

// The kinds of session update (ACP v1).
const (
	UpdateUserChunk     = "user_message_chunk"
	UpdateAgentChunk    = "agent_message_chunk"
	UpdateThoughtChunk  = "agent_thought_chunk"
	UpdateToolCall      = "tool_call"
	UpdateToolCallEdit  = "tool_call_update"
	UpdatePlan          = "plan"
	UpdateCommands      = "available_commands_update"
	UpdateMode          = "current_mode_update"
	UpdateConfigOptions = "config_option_update"
	UpdateSessionInfo   = "session_info_update"
	UpdateUsage         = "usage_update"
	UpdateNotice        = "notice"
)

// ParseUpdate reads a session update, keeping it whole as Raw.
func ParseUpdate(data json.RawMessage) (SessionUpdate, error) {
	var u SessionUpdate
	if err := json.Unmarshal(data, &u); err != nil {
		return SessionUpdate{}, err
	}
	u.Raw = append(json.RawMessage(nil), data...)
	return u, nil
}

// Block is a message chunk's block (zero for an update that isn't one).
func (u SessionUpdate) Block() ContentBlock {
	var b ContentBlock
	json.Unmarshal(u.Content, &b) //nolint:errcheck // not a block: zero
	return b
}

// ToolContent is a tool call's (or update's) content.
func (u SessionUpdate) ToolContent() []ToolCallContent {
	var cs []ToolCallContent
	json.Unmarshal(u.Content, &cs) //nolint:errcheck // none, or not a list: none
	return cs
}

// Permission option kinds.
const (
	AllowOnce    = "allow_once"
	AllowAlways  = "allow_always"
	RejectOnce   = "reject_once"
	RejectAlways = "reject_always"
)

// PermissionOption is an answer the agent offers to a permission request.
type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // allow_once, allow_always, reject_once, reject_always
}

// PermissionRequest is the agent asking leave to make a tool call: the
// call (as a tool call update: its title, kind, content and input) and the
// answers it offers.
type PermissionRequest struct {
	SessionID string             `json:"sessionId"`
	ToolCall  SessionUpdate      `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
}

// Option is the request's first option of one of kinds, in the order
// kinds are given ("" for none).
func (r PermissionRequest) Option(kinds ...string) string {
	for _, k := range kinds {
		for _, o := range r.Options {
			if o.Kind == k {
				return o.OptionID
			}
		}
	}
	return ""
}

// ElicitationRequest is the agent asking for a form to be filled in (a
// question with its options, say): Message, and the fields in
// RequestedSchema (a JSON Schema object). In url mode, it's a page to
// send someone to instead.
type ElicitationRequest struct {
	SessionID       string          `json:"sessionId,omitempty"`
	ToolCallID      string          `json:"toolCallId,omitempty"`
	Mode            string          `json:"mode"` // form, url
	Message         string          `json:"message"`
	RequestedSchema json.RawMessage `json:"requestedSchema,omitempty"`
	ElicitationID   string          `json:"elicitationId,omitempty"`
	URL             string          `json:"url,omitempty"`
}

// ElicitationResponse answers one: Action accept (with Content, the fields'
// values), decline or cancel.
type ElicitationResponse struct {
	Action  string         `json:"action"`
	Content map[string]any `json:"content,omitempty"`
}

// AgentInfo is what the agent said of itself when the connection started.
type AgentInfo struct {
	ProtocolVersion   int `json:"protocolVersion"`
	AgentCapabilities struct {
		LoadSession        bool `json:"loadSession"`
		PromptCapabilities struct {
			Image           bool `json:"image"`
			Audio           bool `json:"audio"`
			EmbeddedContext bool `json:"embeddedContext"`
		} `json:"promptCapabilities"`
		// Each is there (not nil) if the agent can.
		SessionCapabilities struct {
			List                  *struct{} `json:"list,omitempty"`
			Resume                *struct{} `json:"resume,omitempty"`
			Close                 *struct{} `json:"close,omitempty"`
			AdditionalDirectories *struct{} `json:"additionalDirectories,omitempty"`
			SystemPrompt          *struct{} `json:"systemPrompt,omitempty"`
		} `json:"sessionCapabilities"`
	} `json:"agentCapabilities"`
	AuthMethods []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	} `json:"authMethods"`
	Agent *struct {
		Name    string `json:"name"`
		Title   string `json:"title,omitempty"`
		Version string `json:"version"`
	} `json:"agentInfo,omitempty"`
}
