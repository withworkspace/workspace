// Package crew runs agents (crew) on missions: where they work, how they
// start, and how their ends are recorded. Everything that launches crew
// does it through this, on a Host: the workspace's daemon, or (for tests)
// this process.
package crew

import (
	"errors"
	"io"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// Term is a crew's terminal: a pane in this process (PaneTerm), or one
// reached over the server's socket.
type Term interface {
	RenderScrolled(offset int) string
	ScrollbackLen() int
	AltScreen() bool
	Text() string
	Cursor() (x, y int, visible bool)
	Title() string
	// LastChange is when the screen's text last changed.
	LastChange() time.Time

	// TakeCopied returns what the crew copied to the clipboard since the
	// last call, if it copied anything.
	TakeCopied() (string, bool)

	SendKey(uv.KeyPressEvent)
	SendMouse(uv.MouseEvent)
	Paste(string)
	WriteRaw([]byte)
	SetMirror(io.Writer)
	Resize(width, height int)

	Updates() <-chan struct{}
	// Done is closed once the crew has exited and its end is recorded.
	Done() <-chan struct{}
	Exited() bool
	ExitSummary() string
	Pid() int
	// Close stops the crew: SIGHUP, then SIGKILL after grace.
	Close(grace time.Duration)
}

// Spec is everything needed to start a crew process.
type Spec struct {
	Slug    string   `json:"slug"`
	CrewID  string   `json:"crew"`
	Harness string   `json:"harness"`
	Capsule string   `json:"capsule"` // repo/name it works in, for display
	Role    string   `json:"role,omitempty"`
	Args    []string `json:"args"`
	Dir     string   `json:"dir"`
	Env     []string `json:"env"`
	Width   int      `json:"w"`
	Height  int      `json:"h"`
	// Prompt is the opening prompt, for the host to type in once the
	// crew's screen has drawn and settled (TypePrompt): nobody who launched
	// it waits to. "" for none (the harness passed it as an argument).
	Prompt string `json:"prompt,omitempty"`
	// ACP, for crew whose harness runs them over ACP, is the agent's run
	// (Args is then empty): the host holds an agent, not a terminal.
	ACP *harness.ACPRun `json:"acp,omitempty"`
}

// Key is the crew's identity: slug/crew.
func (s Spec) Key() string { return s.Slug + "/" + s.CrewID }

// Host runs crew, and records their ends itself (through Finish), so
// whoever launched them never has to.
type Host interface {
	// Launch starts crew and says its process: a terminal running
	// Spec.Args, its opening prompt (Spec.Prompt) typed in by the host; or,
	// with Spec.ACP, an agent spoken to over ACP, its opening message
	// (Spec.ACP's Prompt) sent by the host.
	Launch(Spec) (pid int, err error)
	// Stop stops crew running by key (slug/crew), recorded as stopped: its
	// session can be resumed.
	Stop(key string) error
	// Say gives running crew a message (Run.Say), and says what it did:
	// ErrNotRunning for crew it isn't running.
	Say(key, text string) (string, error)
}

// ErrNotRunning is Stop or Say for crew the host isn't running.
var ErrNotRunning = errors.New("not running here")

// ErrRunning is Launch for crew the host is running (or starting) already:
// they're not to be recorded as ended for it.
var ErrRunning = errors.New("already running")

// EndHook runs after a crew's end is recorded. stopped is true when it was
// stopped (by the user, or the host shutting down) rather than ending by
// itself.
type EndHook func(spec Spec, exit string, stopped bool)

// How is how a crew's run ended, as its host saw it.
type How int

const (
	// Exited: it ended by itself.
	Exited How = iota
	// Stopped: someone asked it to stop (you, from its page, or ws ending
	// a session that had handed its work back).
	Stopped
	// Interrupted: its host shut down under it (the daemon restarted, was
	// upgraded or closed) without anyone asking it to stop. The next
	// daemon resumes it, if it was still at its work.
	Interrupted
)

// HowEnded is how crew ended, from what its host knows: whether it was
// asked to stop, and whether the host was closing.
func HowEnded(stopRequested, closing bool) How {
	switch {
	case stopRequested:
		return Stopped
	case closing:
		return Interrupted
	}
	return Exited
}

// Finish records the end of a crew run: its output, then the end, then the
// hook. Every host ends crew through here.
func Finish(store mission.Store, spec Spec, exit, transcript string, how How, hook EndHook) error {
	stopped := how != Exited
	result := exit
	if stopped {
		result += " (stopped)"
	}
	execlog.End(store.Root, "crew "+spec.Key(), 0, 0, result)
	errT := store.SaveTranscript(spec.Slug, spec.CrewID, exit, transcript)
	var errE error
	if how == Interrupted {
		errE = store.EndCrewInterrupted(spec.Slug, spec.CrewID, exit)
	} else {
		errE = store.EndCrewStopped(spec.Slug, spec.CrewID, exit, stopped)
	}
	if hook != nil {
		hook(spec, exit, stopped)
	}
	if errE != nil {
		return errE
	}
	return errT
}
