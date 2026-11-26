package crew

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/withworkspace/workspace/internal/acp"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// Run is one running crew, however it runs: in a terminal the host holds,
// or as an agent over ACP. Its page reaches it here, and a command through
// its host (Host.Say): a message, what it's asking, an answer. A terminal
// run answers by sending keys to the menu on its screen, an ACP run by
// answering its agent's request; whoever asks doesn't know which.
type Run interface {
	Spec() Spec
	// Say gives it a message: pasted and submitted in its terminal, or
	// sent to its agent (who gets it when its turn ends, if one's under
	// way). It says what it did.
	Say(text string) (string, error)
	// Asking is what it's waiting on you for: the menu on its screen, or
	// its agent's open request.
	Asking() (Prompt, bool)
	// Answer answers it, and says what it did. c is what the mission
	// records of the crew: what its hooks said it was asking. An answer
	// to what isn't being asked is ErrNotOnScreen, and nothing's sent.
	Answer(c mission.Crew, a Answer) (string, error)
	// SwitchModel puts it on another model, its session going on: typed
	// into its terminal as you'd type it (Claude Code's /model), or its
	// agent's session switched. It says what it did; a harness that can't
	// is an error, and nothing's done.
	SwitchModel(model string) (string, error)
}

// Runs are the crew a host holds in this process (the daemon's server),
// each a Run, and the terminals of those that have one.
type Runs interface {
	Run(key string) (Run, bool)
	Screen(key string) (*Frame, bool)
	WatchScreen(key string) (changes, ended <-chan struct{}, cancel func(), ok bool)
	SendKey(key string, k uv.KeyPressEvent) error
	Paste(key, text string) error
}

// Prompt is what running crew are waiting on you for: what's asked
// (Title), what it's about (Context: the command, the file), and the
// answers offered.
type Prompt struct {
	Title            string
	Context, Options []string
}

// Answer answers running crew: a permission (allow, always, deny), one of
// its question's options, your own words, or an interrupt.
type Answer struct {
	Permission, Option, Text string
	Interrupt                bool
}

// ErrNotOnScreen is an answer refused because what crew are asking isn't
// what it answers: nothing was sent.
var ErrNotOnScreen = errors.New("its screen isn't showing that prompt")

// Frame is a crew's screen as it's drawn: its text (with its colours, as
// ANSI escapes), where it's scrolled to, the cursor, and its size.
type Frame struct {
	Screen        string `json:"screen"`
	Offset        int    `json:"offset"`
	ScrollbackLen int    `json:"sb"`
	AltScreen     bool   `json:"alt"`
	CursorX       int    `json:"cx"`
	CursorY       int    `json:"cy"`
	CursorVisible bool   `json:"cv"`
	Title         string `json:"title,omitempty"`
	Width         int    `json:"w"`
	Height        int    `json:"h"`
	// Changed is when the screen's text last changed, in Unix milliseconds.
	Changed int64 `json:"changed"`
	// Copied is what the crew copied to the clipboard since the frame
	// before, if anything.
	Copied string `json:"copied,omitempty"`
}

// settle is how long after typing the screen's given to show it took.
const settle = 300 * time.Millisecond

// TermRun is crew running in a terminal (T), in the workspace at Root.
type TermRun struct {
	S    Spec
	T    Keys
	Root string
}

// Keys is what a terminal run needs of its terminal: its screen as drawn,
// and keys and pastes into it. A Term is one.
type Keys interface {
	RenderScrolled(offset int) string
	SendKey(uv.KeyPressEvent)
	Paste(string)
}

func (r TermRun) Spec() Spec { return r.S }

// Say pastes text in and submits it, as you'd type it in its terminal (the
// end of its turn is then yours).
func (r TermRun) Say(text string) (string, error) {
	r.T.Paste(text)
	time.Sleep(settle)
	r.T.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	execlog.Note(r.Root, "crew "+r.S.Key(), "typed in (%d chars):\n%s", len(text), text)
	return "Sent to " + r.S.CrewID + ".", nil
}

// SwitchModel types its harness's commands for another model into its
// terminal, each key by key (a paste of one would be text to send, not a
// command) and submitted, one after another.
func (r TermRun) SwitchModel(model string) (string, error) {
	h, err := harness.Get(r.S.Harness)
	if err != nil {
		return "", err
	}
	sw, ok := h.(harness.ModelSwitcher)
	if !ok {
		return "", fmt.Errorf("%s's model can't be changed while it runs", r.S.Harness)
	}
	lines, err := sw.SwitchModel(model)
	if err != nil {
		return "", err
	}
	for i, line := range lines {
		if i > 0 {
			time.Sleep(settle) // the last one taken before the next is typed
		}
		for _, ch := range line {
			ev := uv.KeyPressEvent{Code: unicode.ToLower(ch), Text: string(ch)}
			if ch == ' ' {
				ev.Code = uv.KeySpace
			}
			if unicode.ToLower(ch) != ch {
				ev.ShiftedCode = ch
			}
			r.T.SendKey(ev)
		}
		time.Sleep(settle)
		r.T.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
		execlog.Note(r.Root, "crew "+r.S.Key(), "typed in: %s", line)
	}
	return "Switched " + r.S.CrewID + " to " + model + ".", nil
}

// Asking is the menu on its screen, read as answering it would read it.
func (r TermRun) Asking() (Prompt, bool) {
	p, ok := harness.ReadPrompt(r.T.RenderScrolled(0))
	if !ok {
		return Prompt{}, false
	}
	v := Prompt{Title: p.Title, Context: p.Context, Options: p.Options}
	if v.Context == nil {
		v.Context = []string{}
	}
	return v, true
}

// Answer reads the menu on its screen and moves to the answer, typing your
// words where the menu takes them. Nothing's typed if the screen isn't
// showing what its hooks said it's asking.
func (r TermRun) Answer(c mission.Crew, a Answer) (string, error) {
	id, key := r.S.CrewID, r.S.Key()
	send := func(k rune) { r.T.SendKey(uv.KeyPressEvent{Code: k}) }
	if a.Interrupt {
		send(uv.KeyEscape)
		execlog.Note(r.Root, "crew "+key, "interrupted")
		return "Interrupted " + id + ".", nil
	}
	p, found := harness.ReadPrompt(r.T.RenderScrolled(0))
	if !found {
		return "", fmt.Errorf("%w: there's no menu on it. Answer in the terminal", ErrNotOnScreen)
	}
	choice := -1
	var words string
	switch {
	case a.Permission != "":
		if c.Activity != mission.ActivityPermission || !p.IsPermission() {
			return "", fmt.Errorf("%w: it isn't asking for a permission (%q). Answer in the terminal", ErrNotOnScreen, p.Title)
		}
		choice = p.PermissionChoice(a.Permission)
		words = map[string]string{"allow": "Allowed it once", "always": "Allowed it from now on", "deny": "Denied it"}[a.Permission]
		if choice < 0 && a.Permission == "deny" {
			// No "No" to pick: Escape declines.
			send(uv.KeyEscape)
			return "Denied it.", nil
		}
	case a.Option != "":
		if !questionOnScreen(c, p) {
			return "", fmt.Errorf("%w: its question isn't the one on screen. Answer in the terminal", ErrNotOnScreen)
		}
		choice, words = p.Option(a.Option), "Answered “"+a.Option+"”"
	case strings.TrimSpace(a.Text) != "":
		if !questionOnScreen(c, p) {
			return "", fmt.Errorf("%w: its question isn't the one on screen. Answer in the terminal", ErrNotOnScreen)
		}
		choice, words = p.FreeText(), "Answered in your own words"
	}
	if choice < 0 {
		return "", fmt.Errorf("%w: the menu has no such choice (%s). Answer in the terminal", ErrNotOnScreen, strings.Join(p.Options, "; "))
	}
	move := uv.KeyDown
	n := p.Moves(choice)
	if n < 0 {
		move, n = uv.KeyUp, -n
	}
	for range n {
		send(move)
		time.Sleep(40 * time.Millisecond)
	}
	if strings.TrimSpace(a.Text) != "" && a.Option == "" && a.Permission == "" {
		// Your own words go in the option that takes them.
		time.Sleep(settle / 2)
		r.T.Paste(a.Text)
		time.Sleep(settle)
	}
	send(uv.KeyEnter)
	execlog.Note(r.Root, "crew "+key, "answered: %s (option %d of %q)", words, choice+1, p.Title)
	time.Sleep(settle)
	return words + ".", nil
}

// questionOnScreen is whether the menu on screen is the question crew's
// hook said it asked: one of its options is there.
func questionOnScreen(c mission.Crew, p harness.Prompt) bool {
	if c.Activity != mission.ActivityQuestion {
		return false
	}
	if len(c.Options) == 0 {
		return true // asked without options: the menu's its own
	}
	for _, o := range c.Options {
		if p.Option(o) >= 0 {
			return true
		}
	}
	return false
}

// AgentRun is crew running over ACP: an agent the host holds, with no
// terminal. A message is sent to it (queued while a turn runs), what it
// asks is answered with its own options, and interrupting cancels its
// turn. Nothing's read off a screen or typed in: there's none.
type AgentRun struct {
	S     Spec
	Agent *acp.Agent
	Root  string
}

func (r AgentRun) Spec() Spec { return r.S }

func (r AgentRun) Say(text string) (string, error) {
	busy := r.Agent.State().Working
	if err := r.Agent.Send(acp.Text(text)); err != nil {
		return "", err
	}
	execlog.Note(r.Root, "crew "+r.S.Key(), "sent (%d chars):\n%s", len(text), text)
	if busy {
		return "Sent: " + r.S.CrewID + " gets it when its turn ends.", nil
	}
	return "Sent to " + r.S.CrewID + ".", nil
}

// SwitchModel switches its agent's session to another model, as its
// harness names it over ACP.
func (r AgentRun) SwitchModel(model string) (string, error) {
	h, err := harness.Get(r.S.Harness)
	if err != nil {
		return "", err
	}
	name, settings, ok := harness.ACPModel(h, model)
	if !ok {
		return "", fmt.Errorf("%s's model can't be changed while it runs", r.S.Harness)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := r.Agent.SetModel(ctx, name, settings); err != nil {
		return "", err
	}
	execlog.Note(r.Root, "crew "+r.S.Key(), "model switched to %s", model)
	return "Switched " + r.S.CrewID + " to " + model + ".", nil
}

// Asking is what its agent is waiting on you for, as a prompt card shows
// it.
func (r AgentRun) Asking() (Prompt, bool) {
	open := r.Agent.Asks().Open()
	if len(open) == 0 {
		return Prompt{}, false
	}
	a := open[0]
	v := Prompt{Context: []string{}}
	switch {
	case a.Permission != nil:
		v.Title = "Do you want it to " + harness.AskedFor(*a.Permission) + "?"
		tc := a.Permission.ToolCall
		for _, c := range tc.ToolContent() {
			switch {
			case c.Type == "content" && c.Content != nil:
				v.Context = append(v.Context, c.Content.String())
			case c.Type == "diff":
				v.Context = append(v.Context, "changes "+c.Path)
			}
		}
		for _, o := range a.Permission.Options {
			v.Options = append(v.Options, o.Name)
		}
	default:
		q, options, ok := harness.AskQuestion(a)
		if !ok {
			return Prompt{}, false
		}
		v.Title, v.Options = q, options
	}
	return v, true
}

// Answer answers what its agent is waiting on you for: a permission, one
// of a question's answers or your words, or an interrupt.
func (r AgentRun) Answer(_ mission.Crew, a Answer) (string, error) {
	id, key := r.S.CrewID, r.S.Key()
	if a.Interrupt {
		if err := r.Agent.Interrupt(); err != nil {
			return "", err
		}
		execlog.Note(r.Root, "crew "+key, "interrupted")
		return "Interrupted " + id + ".", nil
	}
	var ask acp.Ask
	found := false
	for _, x := range r.Agent.Asks().Open() {
		if a.Permission != "" && x.Permission != nil || a.Permission == "" && x.Permission == nil {
			ask, found = x, true
			break
		}
	}
	if !found {
		what := "a question"
		if a.Permission != "" {
			what = "a permission"
		}
		return "", fmt.Errorf("%w: %s isn't waiting on %s now", ErrNotOnScreen, id, what)
	}
	var ans acp.Answer
	var err error
	var words string
	switch {
	case a.Permission != "":
		ans, err = harness.PermissionAnswer(*ask.Permission, a.Permission)
		words = map[string]string{"allow": "Allowed it once", "always": "Allowed it from now on", "deny": "Denied it"}[a.Permission]
	case a.Option != "":
		ans, err = harness.QuestionAnswer(ask, a.Option, "")
		words = "Answered “" + a.Option + "”"
	case strings.TrimSpace(a.Text) != "":
		ans, err = harness.QuestionAnswer(ask, "", a.Text)
		words = "Answered in your own words"
	default:
		return "", errors.New("no answer given")
	}
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotOnScreen, err)
	}
	ans.By = "you, in the web UI"
	if err := r.Agent.Asks().Answer(ask.ID, ans); err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotOnScreen, err)
	}
	execlog.Note(r.Root, "crew "+key, "answered: %s", words)
	return words + ".", nil
}
