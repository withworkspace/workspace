package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Cursor runs the Cursor CLI agent.
//
// Every assumption about the Cursor CLI lives in this file. Those checked
// against a real cursor-agent say VERIFIED; any left marked GUESS haven't
// been.
type Cursor struct {
	// lookPath and run are swapped in tests, and chats (where Cursor keeps
	// its chats; "" for ~/.cursor/chats).
	lookPath func(string) (string, error)
	run      func(ctx context.Context, bin string, args ...string) ([]byte, error)
	chats    string
}

// EnvCursorBin overrides the Cursor CLI binary (name or path).
const EnvCursorBin = "WS_CURSOR_AGENT"

// VERIFIED: the binary is installed as cursor-agent, with agent as a newer alias.
var cursorBinaries = []string{"cursor-agent", "agent"}

// VERIFIED: `create-chat` prints a new chat id (alone, on the last line) and
// exits, letting us know the conversation id before the agent starts.
var cursorCreateChatArgs = []string{"create-chat"}

const cursorCreateChatTimeout = 15 * time.Second

// VERIFIED: in a folder Cursor hasn't seen, an interactive cursor-agent opens
// on a "Workspace Trust Required" dialog, and the opening prompt typed in
// once the screen settles lands in it (an "a" in the text answers it; the
// rest is lost). Every newly lifted capsule is such a folder. --trust skips
// the dialog interactively too, alone or with --resume. ws made the capsule
// and the user chose to launch crew in it.
const cursorTrustFlag = "--trust"

// A chat id is one token: letters, digits, dashes, underscores.
var chatIDShape = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{5,}$`)

func init() {
	register(&Cursor{
		lookPath: exec.LookPath,
		run: func(ctx context.Context, bin string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, bin, args...).Output()
		},
	})
}

func (c *Cursor) Name() string { return "cursor" }

// Cursor keeps its chats in a database ws can't read: no transcript.
func (c *Cursor) held() (CrewVia, TranscriptVia) { return CrewTerminal, TranscriptNone }

// Shells: Cursor's stop hook says nothing of what the agent left running,
// so the process check alone answers. A command still running in one of
// its shells is a background shell, and Cursor wakes the agent when it
// ends, with a turn of its own. (Whether ws's hooks see that turn isn't
// known: if they don't, the backstop takes the turn a minute after the
// command ends, mission.WakeGrace.)
//
// READ (cursor-agent 2026.11.05, its bundled index.js): each command runs
// in a new `zsh -c` (bash with -O extglob -c), spawned detached from the
// main node process, which cursor-agent's launcher execs: one shell per
// command, exiting when it does. A background shell is the same, not
// waited on, and a foreground command that runs too long is moved to the
// background with its pid. In a terminal, a pending background completion
// starts a turn when nothing else is queued (backgroundTaskCompletionAction).
// Under a workspace sandbox policy the shell runs under cursorsandbox,
// which isn't a shell: the check doesn't see it. MCP servers are spawned
// without a shell, so aren't counted unless their config runs one.
func (c *Cursor) Shells(Pending) (Wake, bool) { return WakeYes, false }

func (c *Cursor) bin() (string, error) {
	if b := os.Getenv(EnvCursorBin); b != "" {
		return c.lookPath(b)
	}
	for _, name := range cursorBinaries {
		if p, err := c.lookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("cursor CLI not found (looked for %s; set %s to override)", strings.Join(cursorBinaries, ", "), EnvCursorBin)
}

func (c *Cursor) Available() error {
	_, err := c.bin()
	return err
}

func (c *Cursor) Start(b Brief) (Launch, error) {
	bin, err := c.bin()
	if err != nil {
		return Launch{}, err
	}
	conv, note := c.createChat(bin)
	args := []string{bin, cursorTrustFlag}
	if conv != "" {
		// Verified: --resume <id> opens the chat create-chat made,
		// interactively.
		args = append(args, "--resume", conv)
	}
	if b.Model != "" {
		args = append(args, "--model", b.Model)
	}
	// The opening prompt is typed in once the agent has drawn (see
	// Launch.Prompt), not passed as an argument: a long multi-line prompt
	// argument got new crew killed before they rendered.
	return Launch{Args: args, Dir: b.Dir, Conversation: conv, Prompt: b.Prompt, Notes: []string{note}}, nil
}

func (c *Cursor) Resume(dir, conversation string) (Launch, error) {
	if conversation == "" {
		return Launch{}, errors.New("no cursor chat id recorded for this crew, so it can't be resumed")
	}
	bin, err := c.bin()
	if err != nil {
		return Launch{}, err
	}
	// VERIFIED: --resume <id> reopens an earlier chat interactively.
	return Launch{Args: []string{bin, cursorTrustFlag, "--resume", conversation}, Dir: dir, Conversation: conversation}, nil
}

// createChat asks the CLI for a chat id up front. Any failure just means the
// crew can't be resumed later; the agent still starts. The note says what
// happened, for the exec log.
func (c *Cursor) createChat(bin string) (id, note string) {
	ctx, cancel := context.WithTimeout(context.Background(), cursorCreateChatTimeout)
	defer cancel()
	ran := bin + " " + strings.Join(cursorCreateChatArgs, " ")
	start := time.Now()
	out, err := c.run(ctx, bin, cursorCreateChatArgs...)
	took := time.Since(start).Round(100 * time.Millisecond)
	if err != nil {
		detail := err.Error()
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			detail += ": " + strings.TrimSpace(string(ee.Stderr))
		}
		return "", fmt.Sprintf("ran %s: failed after %s (%s); starting without a chat id, so this crew can't be resumed", ran, took, detail)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if chatIDShape.MatchString(last) {
		return last, fmt.Sprintf("ran %s: chat %s (%s)", ran, last, took)
	}
	return "", fmt.Sprintf("ran %s: no chat id in its output (last line %q); starting without one, so this crew can't be resumed", ran, last)
}

// VERIFIED (cursor-agent --help): -p/--print runs one prompt headless,
// --trust skips the workspace-trust prompt nobody is there to answer, and
// --model picks the model. VERIFIED: the default model name is one Cursor
// knows, but whether an account may use it depends on its plan, and
// --list-models lists models the plan doesn't include. A Free plan
// refuses it ("Model unavailable on Free", exit 1), so set [orchestrator]
// model = "auto" there. An unknown name fails the same way ("Cannot use
// this model").

// CursorOrchestratorModel is the model Cursor runs the orchestrator on
// unless [orchestrator] model says otherwise.
const CursorOrchestratorModel = "claude-haiku-5-5-medium"

func (c *Cursor) OrchestratorModel() string { return CursorOrchestratorModel }

// Orchestrate runs a headless orchestrator in Cursor's print mode,
// limited as a conversation is (see Converse): ask mode, so it edits
// nothing with its own tools, and without --force, so only the commands
// it's allowed (orchestratorCommands) run, from a folder of ws's whose
// .cursor/cli.json allows them.
//
// VERIFIED (cursor-agent 2026.11.05): without --force, its default (agent)
// mode still writes files with its edit tool, anywhere; only shell
// commands are refused. Hence ask mode.
func (c *Cursor) Orchestrate(r OrchestratorRun) (Launch, error) {
	bin, err := c.bin()
	if err != nil {
		return Launch{}, err
	}
	if r.Scratch == "" {
		return Launch{}, fmt.Errorf("cursor: no folder to hold the orchestrator's permissions in")
	}
	dir, err := writeCursorPermissions(r.Scratch, orchestratorCommands(r))
	if err != nil {
		return Launch{}, err
	}
	args := []string{bin, "-p", cursorTrustFlag, "--mode", "ask", "--output-format", "stream-json"}
	if r.Model != "" {
		args = append(args, "--model", r.Model)
	}
	return Launch{Args: append(args, "--", r.Prompt), Dir: dir, Stream: StreamCursor}, nil
}
