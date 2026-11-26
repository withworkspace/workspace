// Package harness knows how to launch and resume each kind of agent CLI
// (Cursor, Claude Code, pi, a plain shell) in the roles agents play on a
// mission: crew doing the work, briefing a new mission, and the headless
// orchestrator. Roles says which harness and model each role uses.
//
// A harness only builds commands. Running them in a terminal pane, setting
// WS_MISSION/WS_CREW, and recording crew are package crew's job.
package harness

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// EnvDefault picks the harness crew run, over [crew] harness in config.
const EnvDefault = "WS_DECK_HARNESS"

// Brief is what a harness needs to know about the mission it starts on.
type Brief struct {
	Slug  string
	Title string
	Brief string // brief.md contents
	Dir   string // working directory (a mission capsule, or the workspace root when briefing)
	// Model is the model to run, or "" for the agent's own default.
	Model string

	// Prompt is the opening prompt, written for the crew by whoever starts
	// it (prompt.Crew): a harness gives it to the agent, as the agent's CLI
	// takes one, and doesn't write it.
	Prompt string
	// Role is "" for crew working on the mission, or "briefing" for crew
	// turning a request into a brief and capsules.
	Role    string
	Request string // what the user asked for (briefing)
	// Conversation is the conversation the mission came out of (w3), whose
	// summary and turns the briefing starts from.
	Conversation string
	// Question and Options are the orchestrator's flagged question, and the
	// answers it suggested, for crew asked to answer it (Role "answer").
	Question string
	Options  []string
	// Guide is what the crew is told of the workspace's own way of working:
	// where its guide for agents is, and how to list its skills. "" where
	// it has neither.
	Guide string
	// Dispatch is what this crew was dispatched to do: its part of the
	// mission.
	Dispatch string
	// Stage is the stage of the brief's plan this crew is for (its text),
	// StageN its number and StageOf how many there are. Stage is "" when
	// the brief lists no stages. Steps: they're the steps of the mission's
	// plan, not the brief's stages.
	Stage           string
	StageN, StageOf int
	Steps           bool
	// OpensPR: the step makes a pull request, which its crew open
	// through ws.
	OpensPR bool
	// LeftOff is where earlier crew left off: their latest handoff in
	// full, and where the others are (mission.LeftOff).
	LeftOff string
	Org     string // GitHub org the workspace's repos live in
	// Elsewhere are the repos that aren't Org's by their name here: repo
	// to owner/name on GitHub.
	Elsewhere map[string]string
}

// Launch is a ready-to-run agent command.
type Launch struct {
	Args []string // argv, Args[0] is the program
	Dir  string
	// Conversation is the agent's own session id when known before the
	// agent starts, so the crew record can be resumed later.
	Conversation string
	// Prompt, when set, is pasted into the agent once its interface has
	// drawn and settled, then submitted with Enter, as a person would.
	// Typing it in avoids relying on how a CLI treats a long multi-line
	// prompt argument.
	Prompt string
	// Stdin, when set, is a headless run's input: its prompt, for a CLI
	// that reads one there, kept off its command line.
	Stdin string
	// Notes say what the harness ran getting ready (and how it went), for
	// the exec log: a chat id fetched, or why there isn't one.
	Notes []string
	// Stream says how a headless run reports: "" for its final message as
	// plain text, StreamClaude for Claude Code's stream-json (every tool
	// call and its result, then the final message).
	Stream string
	// ACP, for crew, is an agent run over ACP rather than a command for a
	// terminal (Args is empty): the daemon holds it, and the web UI is
	// how you see it and talk to it. Its Prompt is the opening message.
	ACP *ACPRun
}

// Streams: how a headless run reports, when it does more than print its
// final message.
const (
	StreamClaude = "claude-stream-json" // Claude Code's --output-format stream-json
	StreamCursor = "cursor-stream-json" // cursor-agent's --output-format stream-json
	StreamPi     = "pi-json"            // pi's --mode json
)

// Harness launches one kind of agent.
type Harness interface {
	Name() string
	// Available reports why the harness can't run here, or nil.
	Available() error
	// Start builds the command for new crew on a mission.
	Start(b Brief) (Launch, error)
	// Resume builds the command to continue an earlier conversation. It
	// returns ErrNoResume if the harness can't.
	Resume(dir, conversation string) (Launch, error)
}

// ErrNoResume is returned by harnesses that can't resume a conversation.
var ErrNoResume = errors.New("this harness can't resume conversations")

var registry = map[string]Harness{}

func register(h Harness) { registry[h.Name()] = h }

// Register adds a harness (or replaces one of the same name). Built-in
// harnesses register themselves; tests use this for fakes.
func Register(h Harness) { register(h) }

// Get returns a harness by name.
func Get(name string) (Harness, error) {
	if h, ok := registry[name]; ok {
		return h, nil
	}
	return nil, fmt.Errorf("unknown harness %q (have %s)", name, strings.Join(Names(), ", "))
}

// Names lists the registered harnesses.
func Names() []string {
	var names []string
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Caps is what a harness can run, and how: what the Settings page offers
// it for, how its crew are held, and where their transcript is read from.
// Ask it of a harness here rather than by asserting an interface.
type Caps struct {
	Crew     CrewVia
	Headless bool // the orchestrator: one-off turns (an Orchestrator)
	Converse bool // turns that resume a session (a Converser)
	Hooks    bool // lifecycle hooks ws can install (Hooks)
	Models   bool // lists its own models (a ModelLister)
	// SwitchModel: its crew can be put on another model while they run,
	// their session going on (a ModelSwitcher, or crew over ACP).
	SwitchModel bool
	Transcript  TranscriptVia
	Spends      Spends // where it says what its agents spend
}

// CrewVia is how a harness's crew run.
type CrewVia int

const (
	CrewNone     CrewVia = iota // it runs no crew
	CrewTerminal                // in a terminal the daemon holds
	CrewACP                     // over ACP, held by the daemon, with no terminal
)

// TranscriptVia is where a crew's transcript is read from.
type TranscriptVia int

const (
	TranscriptNone    TranscriptVia = iota // nowhere ws can read
	TranscriptSession                      // the harness's own session file (SessionFile)
	TranscriptACP                          // ws's own record of the ACP session
)

// held is a harness that says how its crew run and where their transcript
// is: what can't be told from the interfaces it has. One that doesn't say
// (a test's fake) runs crew in a terminal, with no transcript.
type held interface {
	held() (CrewVia, TranscriptVia)
}

// CapsOf is what the named harness can do: none of it, for a name that
// isn't registered.
func CapsOf(name string) Caps {
	h, ok := registry[name]
	if !ok {
		return Caps{}
	}
	c := Caps{Crew: CrewTerminal}
	if d, ok := h.(held); ok {
		c.Crew, c.Transcript = d.held()
	}
	_, c.Headless = h.(Orchestrator)
	_, c.Converse = h.(Converser)
	_, c.Hooks = h.(Hooks)
	_, c.Models = h.(ModelLister)
	_, typed := h.(ModelSwitcher)
	_, acp := h.(acpModeler)
	c.SwitchModel = typed || (acp && c.Crew == CrewACP)
	if s, ok := h.(spender); ok {
		c.Spends = s.spends()
	}
	return c
}

// NamesWhere lists the registered harnesses whose Caps pass ok.
func NamesWhere(ok func(Caps) bool) []string {
	var names []string
	for _, n := range Names() {
		if ok(CapsOf(n)) {
			names = append(names, n)
		}
	}
	return names
}

// CrewNames lists the registered harnesses that can run crew.
func CrewNames() []string {
	return NamesWhere(func(c Caps) bool { return c.Crew != CrewNone })
}

// Converser is a harness that can hold a conversation headless: each turn
// is a run that resumes the conversation's session with the new message,
// reads what it needs, and replies with its final message. It may only
// read, unless it runs things (RunsThings): in a sandbox of its own
// (Sandboxer), or trusted to, ws answering what it asks (Trusted).
type Converser interface {
	Harness
	// Converse builds the headless command for one turn.
	Converse(r ConverseRun) (Launch, error)
	// ConverseModel is the model it talks on when none is configured.
	ConverseModel() string
}

// ConverseRun is one turn of a conversation.
type ConverseRun struct {
	Prompt  string // the turn's message
	Dir     string // where it runs: always the same for a conversation (sessions are kept per directory)
	Model   string
	Session string // the session to resume; "" starts one
	// Talk is a conversation about the workspace or a mission, rather than
	// a thread on a document: it may read more widely (ws status, the
	// missions, GitHub), and git in ReadPaths (capsules and .grounds).
	Talk      bool
	ReadPaths []string
	// Scratch is a folder the harness may keep what limits a run in (its
	// permissions, an extension): ws's own, never the user's.
	Scratch string
	// Alone is a run with no tools at all: what it's to read is in its
	// prompt, and it can read nothing else (a cold read of a page). Only
	// Claude Code holds to it; elsewhere the agent is only told.
	Alone bool
	// Once is a run that's never gone back to: its session isn't kept
	// (Claude Code's; the others keep theirs).
	Once bool
	// Guard is ws's own command, which Claude Code runs before each shell
	// command (a PreToolUse hook) with a file of what's allowed: it allows
	// a command on the list as ws reads it (ws conversation allowed), where
	// Claude Code's own rules can't (claudeGuard), and says why not, and
	// what to do, of a ws or gh command that isn't on it. nil leaves it to
	// them.
	Guard []string
	// Sandbox is a run whose shell runs what it likes (RunsThings). In the
	// harness's own sandbox (a Sandboxer that can here) it reads anything,
	// reaches the network, and writes only in Write and Beside, the
	// system's temp folder, and nowhere else under Dir; only the commands
	// it's allowed (converseCommands: ws, gh) leave the sandbox. A harness
	// trusted to run things (Trusted) has no sandbox: ws answers what it
	// asks, allowing any command but a ws or gh one that isn't allowed, and
	// writes by its tools only in Folders. A harness that's neither runs it
	// as it would without: reading, and the commands allowed.
	Sandbox bool
	// Folders are where its tools may write files, as they stand when it's
	// asked (a capsule lifted a moment ago is there): a trusted harness's
	// writes are judged by them (ConversePolicy).
	Folders func() []string
	// Write are the folders it works in: one of its own (first), and the
	// capsules its conversation lifted. Beside are what its shell writes besides:
	// git's own folders for the repos' capsules, and where tools keep
	// what they've built or fetched. Room are folders that what's made
	// after the run starts may be written in, though what's in them now
	// is closed: the repos' folders, where a capsule it lifts as it goes
	// is put. Each is a path in full, as it resolves.
	Write, Beside, Room []string
	// Lift is a run that may lift capsules for its conversation (ws lift).
	Lift bool
	// Plugins are the plugins' commands a conversation's agent may run itself, as
	// the words after ws ("sketch edit"): those a plugin marked for
	// conversations.
	Plugins []string
}

// Sandboxer is a harness that can run a conversation's shell in a sandbox
// of its own (ConverseRun.Sandbox).
type Sandboxer interface {
	// Sandboxes is whether it can on this machine.
	Sandboxes() bool
}

// Sandboxes is whether h runs a conversation's shell in a sandbox here.
func Sandboxes(h Harness) bool {
	s, ok := h.(Sandboxer)
	return ok && s.Sandboxes()
}

// Truster is a harness ws lets run a conversation's shell with no sandbox
// of its own: it asks ws before each command and each write outside its
// session's folder, and ws answers (ConversePolicy). The user's call, of
// Cursor: they trust it as they do crew, who run there the same way.
type Truster interface {
	// Trusted is whether its conversations run things unsandboxed.
	Trusted() bool
}

// Escalator is a Sandboxer whose sandbox keeps the network closed and
// lets a command out of it, whole, when the agent asks (Codex's escalated
// permissions), ws answering (ConversePolicy): what needs the network
// (installing a package, git push) runs that way.
type Escalator interface {
	Sandboxer
	// Escalates is whether its conversations ask to leave the sandbox.
	Escalates() bool
}

// Escalates is whether h's sandbox is one its agent asks to leave.
func Escalates(h Harness) bool {
	e, ok := h.(Escalator)
	return ok && e.Sandboxes() && e.Escalates()
}

// RunsThings is whether h's conversations that talk run things (any
// command, writes in their own capsules), in a sandbox of its own or
// trusted to: those of a harness that's neither only read.
func RunsThings(h Harness) bool {
	if Sandboxes(h) {
		return true
	}
	t, ok := h.(Truster)
	return ok && t.Trusted()
}

// Orchestrator is a harness that can run a one-shot, headless agent: a
// prompt in, its final message out, using tools (shell) along the way.
type Orchestrator interface {
	Harness
	// Orchestrate builds the headless command.
	Orchestrate(r OrchestratorRun) (Launch, error)
	// OrchestratorModel is the model it runs the orchestrator on when
	// none is configured: small and quick, it only decides what's next.
	OrchestratorModel() string
}

// OrchestratorRun is one headless orchestrator run.
type OrchestratorRun struct {
	// Prompt is the whole prompt. Pointer, where there is one, is a line
	// pointing at the file it's kept in, for a harness that's given its
	// prompt on a command line (OnCommandLine).
	Prompt, Pointer string
	Dir             string // where it runs (the workspace root)
	Model           string // "" for the agent's own default
	// Capsules are the paths of the mission's capsules, and Grounds the
	// workspace's repos' .grounds (their default branches), which it may
	// inspect read-only (git -C <path> log, …).
	Capsules []string
	Grounds  []string
	// Scratch is a folder the harness may keep what limits a run in (its
	// permissions, an extension): ws's own, never the user's.
	Scratch string
}

// OnCommandLine is the prompt as a harness that's given it on a command
// line takes it (pi): the line pointing at its file where there's one, as
// a long argument could pass the argument size limit. Cursor's CLI and an
// ACP agent are sent the prompt itself (Prompt), on stdin: read from a
// file, it costs the agent a turn first, every run.
func (r OrchestratorRun) OnCommandLine() string {
	if r.Pointer != "" {
		return r.Pointer
	}
	return r.Prompt
}
