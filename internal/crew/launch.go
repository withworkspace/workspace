package crew

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// Request is crew to start on a mission.
type Request struct {
	Store mission.Store
	Host  Host
	Root  string // workspace root, for the crew's environment

	Slug    string
	Harness string // harness name, as recorded
	Place   Place
	Role    string // "" or mission.RoleBriefing
	// Model is the model new crew were launched on, when one was asked
	// for (Dispatch.Model): recorded, to say what they run on, and for
	// resuming them over ACP on it.
	Model string

	// Resume, when set, continues that crew (same id) instead of recording
	// new crew; a resume-with message it was left becomes the prompt.
	Resume string
	// Dispatch, when set, is what the crew was dispatched to do, and By who
	// dispatched it.
	Dispatch, By string
	// Stage is the stage of the brief's plan work crew are for (from 1), or
	// 0 for the current one.
	Stage int

	Width, Height int

	// Prepare asks the harness for the command (Start or Resume).
	Prepare func() (harness.Launch, error)
}

// Started is crew that started.
type Started struct {
	Crew mission.Crew
	Pid  int
	Dir  string
}

// Start launches crew: asks the harness for the command, records the crew
// on the mission, starts it on the host and records its process. If it
// fails after recording the crew, the crew's end is recorded too.
func Start(r Request) (Started, error) {
	l, err := r.Prepare()
	if err != nil {
		execlog.Note(r.Root, "crew "+r.Slug, "%s couldn't get ready to launch: %v", r.Harness, err)
		return Started{}, err
	}
	// hooked: ws's hooks are in the session this starts, so the crew
	// report for themselves, their session's start first of all (an ACP
	// session takes no arguments: its host reports its turns).
	hooked := false
	if args, note := hookArgs(r.Root, r.Harness); len(args) > 0 && len(l.Args) > 0 {
		l.Args = append([]string{l.Args[0]}, append(args, l.Args[1:]...)...)
		l.Notes = append(l.Notes, note)
		hooked = l.ACP == nil
	} else if note != "" {
		l.Notes = append(l.Notes, note)
	}
	var c mission.Crew
	prompt := l.Prompt
	if r.Resume != "" {
		c, err = r.Store.ResumeCrew(r.Slug, r.Resume, r.By)
		if err == nil && c.ResumeWith != "" {
			prompt = c.ResumeWith
			if err := r.Store.SetResumeWith(r.Slug, c.ID, ""); err != nil {
				execlog.Note(r.Root, "crew "+r.Slug+"/"+c.ID, "couldn't clear what it was resumed with: %v", err)
			}
		}
	} else {
		c, err = r.Store.StartCrewOn(r.Slug, r.Harness, r.Model, l.Conversation, r.Role)
	}
	if err != nil {
		return Started{}, err
	}
	// Recorded before their session's up: with hooks, they're starting
	// until their first report, not until ws has handed over their prompt.
	r.Store.SetCrewHooks(r.Slug, c.ID, hooked) //nolint:errcheck // a mark
	who := "crew " + r.Slug + "/" + c.ID
	for _, n := range l.Notes {
		execlog.Note(r.Root, who, "%s", n)
	}
	if r.Dispatch != "" {
		if err := r.Store.SetCrewDispatch(r.Slug, c.ID, r.By, r.Dispatch); err != nil {
			execlog.Note(r.Root, who, "couldn't record who dispatched it: %v", err)
		}
	}
	if r.Resume == "" && r.Role == "" {
		if _, err := r.Store.JoinStage(r.Slug, c.ID, r.Stage); err != nil {
			execlog.Note(r.Root, who, "couldn't record its stage: %v", err)
		}
	}
	spec := Spec{
		Slug: r.Slug, CrewID: c.ID, Harness: r.Harness, Capsule: r.Place.Capsule, Role: r.Role,
		Args: l.Args, Dir: l.Dir, Env: Env(r.Root, r.Slug, c.ID),
		Width: r.Width, Height: r.Height, Prompt: prompt,
	}
	args := l.Args
	if l.ACP != nil {
		// The host sends the opening message (or, resumed, the one it
		// was left) as the session's first prompt.
		run := *l.ACP
		if prompt != "" {
			run.Prompt = prompt
		}
		spec.ACP, spec.Prompt, args = &run, "", run.Agent
	}
	pid, err := r.Host.Launch(spec)
	if errors.Is(err, ErrRunning) {
		// They're running already, from a launch alongside this one:
		// they're not ended, as recording that would say.
		execlog.Note(r.Root, who, "not started again: %v", err)
		return Started{}, err
	}
	if err != nil {
		execlog.Start(r.Root, who, args, l.Dir, spec.Env, 0)
		execlog.Note(r.Root, who, "failed to start: %v", err)
		if eerr := r.Store.EndCrew(r.Slug, c.ID, "failed to start: "+err.Error()); eerr != nil {
			execlog.Note(r.Root, who, "couldn't record it ended: %v", eerr)
		}
		return Started{}, err
	}
	execlog.Start(r.Root, who, args, l.Dir, spec.Env, pid)
	if err := r.Store.SetCrewProcess(r.Slug, c.ID, pid, r.Place.Capsule); err != nil {
		execlog.Note(r.Root, who, "couldn't record its process (%d): %v", pid, err)
	}
	if prompt == "" {
		// Nothing for ws to hand them once their session's up (a prompt
		// given as an argument, as Claude Code's is, they take up
		// themselves). ws's part is over: that starts crew whose session
		// has no hooks of ws's, and with them their first report does
		// (CrewHandedPrompt).
		r.Store.CrewHandedPrompt(r.Slug, c.ID) //nolint:errcheck // a mark
	}
	return Started{Crew: c, Pid: pid, Dir: l.Dir}, nil
}

// Typing the opening prompt in: wait for the agent's screen to draw and stop
// changing, then paste and submit, as a person would.
var (
	PromptSettle  = 1200 * time.Millisecond
	PromptMaxWait = 20 * time.Second
)

// TypePrompt waits for an agent's screen to draw and settle, then pastes the
// prompt and submits it, logging what it sent (root and key name the
// crew). It reports false if the agent exited first.
func TypePrompt(root, key string, t Term, prompt string) bool {
	if !typePrompt(t, prompt) {
		execlog.Note(root, "crew "+key, "exited before its prompt could be typed in (%s)", t.ExitSummary())
		return false
	}
	execlog.Note(root, "crew "+key, "typed in its prompt (%d chars):\n%s", len(prompt), prompt)
	// Handed their prompt, ws's part of starting them is over. That
	// starts crew whose session has no hooks of ws's, who are then taken
	// as working; with them, their first report does (CrewHandedPrompt).
	if slug, id, ok := strings.Cut(key, "/"); ok {
		mission.Store{Root: root}.CrewHandedPrompt(slug, id) //nolint:errcheck // a mark
	}
	return true
}

func typePrompt(t Term, prompt string) bool {
	deadline := time.Now().Add(PromptMaxWait)
	last, changed, drawn := "", time.Now(), false
	for time.Now().Before(deadline) {
		if t.Exited() {
			return false
		}
		if text := t.Text(); text != last {
			last, changed = text, time.Now()
			drawn = drawn || strings.TrimSpace(text) != ""
		}
		if drawn && time.Since(changed) >= PromptSettle {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if t.Exited() {
		return false
	}
	t.Paste(prompt)
	time.Sleep(300 * time.Millisecond)
	t.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	return true
}

// hookArgs installs ws's hooks in the crew's session when its harness has
// them, so the agent reports what it's doing (ws mission hook) instead of
// the deck guessing from its screen. The command is this ws (through the
// .ws/bin link crew get), told the workspace root.
func hookArgs(root, harnessName string) ([]string, string) {
	h, err := harness.Get(harnessName)
	if err != nil {
		return nil, ""
	}
	hk, ok := h.(harness.Hooks)
	if !ok {
		return nil, ""
	}
	shim, err := ShimDir(root)
	if err != nil {
		return nil, "no hooks: " + err.Error()
	}
	cmd := []string{filepath.Join(shim, "ws"), "mission", "hook", "--root", root}
	args, err := hk.HookArgs(cmd, filepath.Join(root, ".ws", "hooks", harnessName))
	if err != nil {
		return nil, "no hooks: " + err.Error()
	}
	return args, "installed hooks: " + strings.Join(cmd, " ") + " <event>"
}
