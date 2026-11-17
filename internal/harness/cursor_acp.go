package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/withworkspace/workspace/internal/acp"
)

// CursorACP runs the Cursor CLI over ACP (`cursor-agent acp`): the headless
// roles (conversations, the orchestrator), as an alternative to cursor's
// print mode, and crew, held by the daemon and seen in the web UI, as an
// alternative to cursor's terminal.
//
// Every assumption about Cursor's ACP lives here and in the design. Those
// marked VERIFIED were seen running cursor-agent 2026.11.05; READ ones were
// read in its code (a Free plan past its trial can't run a turn).
//
// VERIFIED: a session offers the modes agent, plan and ask, and a model
// setting whose values are one per model with its parameters
// ("claude-haiku-5-5[thinking=true,context=300k,reasoning_effort=high]",
// named "claude-haiku-5-5"; Auto is "default[]"). Sessions are kept in
// ~/.cursor/acp-sessions/<id>/, apart from the TUI's chats; one with no
// store.db (no turn ran) can't be loaded ("Session not found"). It offers
// load, not resume. An exhausted plan answers a prompt with "Upgrade your
// plan to continue" as an ordinary reply.
//
// DOCUMENTED (cursor.com/docs/cli/acp): sign in with authenticate,
// cursor_login, after initialize (it uses `agent login`'s); permissions are
// answered allow-once, allow-always or reject-once; cursor/ask_question
// and cursor/create_plan block until the client answers, and
// cursor/update_todos, cursor/task and cursor/generate_image need no
// answer (the CLI sends them as requests all the same, READ).
//
// READ: ACP's ask mode is the CLI's read-only one (no edit tools), as
// print mode's --mode ask is. Without --force, a call that Cursor's own
// allowlists (~/.cursor/cli-config.json, a project's .cursor/cli.json)
// don't allow is asked of the client: a shell command with the command as
// its title, in backticks; a write with its diff. So ws's policy decides
// every command but what you've allowed Cursor yourself, with no
// cli.json of ws's. A no is "reject-once" and the turn goes on. Its
// questions (cursor/ask_question) are answered skipped, with why, and a
// plan for approval (cursor/create_plan, plan mode's) rejected: nobody's
// there. Unanswered, Cursor would ask each single-choice question as a
// permission instead, and drop the rest.
type CursorACP struct {
	// cursor finds the CLI (nil: the cursor harness's own); tests set one.
	cursor *Cursor
	// sessions is where Cursor keeps ACP sessions ("" for
	// ~/.cursor/acp-sessions); tests change it.
	sessions string
}

// cursorACPMode is the mode ws runs Cursor's headless roles in.
const cursorACPMode = "ask"

// cursorLogin is Cursor's ACP auth method: what `agent login` signed in.
const cursorLogin = "cursor_login"

// cursorHeadlessAnswers answer Cursor's blocking requests in a run nobody
// watches.
var cursorHeadlessAnswers = map[string]json.RawMessage{
	"cursor/ask_question": json.RawMessage(`{"outcome":{"outcome":"skipped","reason":"Questions can't be shown in this run: ask what you need to in your reply (or as your instructions say), or decide, and say what you assumed."}}`),
	"cursor/create_plan":  json.RawMessage(`{"outcome":{"outcome":"rejected","reason":"Plans aren't reviewed in this run: go on without one."}}`),
}

func init() {
	register(&CursorACP{})
}

// cli is the cursor harness, which finds the CLI: looked up when needed,
// not when registered, as init order between files isn't to be relied on.
func (c *CursorACP) cli() (*Cursor, error) {
	if c.cursor != nil {
		return c.cursor, nil
	}
	if cur, ok := registry["cursor"].(*Cursor); ok {
		return cur, nil
	}
	return nil, errors.New("the cursor harness isn't registered")
}

func (c *CursorACP) Name() string { return "cursor-acp" }

// PendingAtTurnEnd is what Cursor left running when a turn ended, read as
// Codex's is (acpLeftRunning): a command whose call is still in_progress.
//
// READ: a background shell started with an output pattern to watch for is
// "watched", and the prompt doesn't return until it has ended (each
// completion runs as a follow-up inside it), so it never reaches a turn's
// end. Any other background shell can outlive the turn, and nothing wakes
// the agent when it ends: an ACP agent can't start a turn. VERIFIED: such
// a turn's response is {stopReason: end_turn} alone (on a turn its plan
// refused). Not checked: whether Cursor leaves that shell's tool call
// in_progress. If it doesn't, ws sees nothing running here.
func (c *CursorACP) PendingAtTurnEnd(t acp.Turn) Pending { return acpLeftRunning(t) }

// Shells: a command still running in one of Cursor's shells at a turn's
// end is a background shell, which nothing wakes the agent for over ACP.
// One whose call is still in_progress the shell gives its pid; if Cursor
// doesn't leave the call open, the shell is the work.
func (c *CursorACP) Shells(Pending) (Wake, bool) { return WakeNo, false }

// Its crew run over ACP, held by the daemon, seen in the web UI.
func (c *CursorACP) held() (CrewVia, TranscriptVia) { return CrewACP, TranscriptACP }

func (c *CursorACP) Available() error {
	_, err := c.agent()
	return err
}

// cursorCrewMode is the mode crew run in: Cursor's own agent mode, all
// its tools, asking leave for what its allowlists don't allow.
const cursorCrewMode = "agent"

// Start is new crew: a Cursor session in the capsule, in agent mode, its
// opening prompt the brief. Its questions (cursor/ask_question) wait for
// you, as a permission does; a plan for approval is rejected (crew don't
// run in plan mode).
func (c *CursorACP) Start(b Brief) (Launch, error) {
	agent, err := c.agent()
	if err != nil {
		return Launch{}, err
	}
	run := ACPRun{Agent: agent, Dir: b.Dir, Mode: cursorCrewMode, Model: cursorACPModel(b.Model), Allow: crewCommands,
		Prompt: b.Prompt, Auth: cursorLogin, Ask: []string{"cursor/ask_question"},
		Answers: map[string]json.RawMessage{"cursor/create_plan": cursorHeadlessAnswers["cursor/create_plan"]}}
	return Launch{Dir: b.Dir, ACP: &run}, nil
}

// Resume goes on with crew's session, if Cursor still has it (it keeps
// one only once a turn has run).
func (c *CursorACP) Resume(dir, conversation string) (Launch, error) {
	if conversation == "" || !c.sessionKept(conversation) {
		return Launch{}, fmt.Errorf("%w: Cursor has no ACP session %q to resume", ErrNoResume, conversation)
	}
	agent, err := c.agent()
	if err != nil {
		return Launch{}, err
	}
	run := ACPRun{Agent: agent, Dir: dir, Load: conversation, Mode: cursorCrewMode, Allow: crewCommands, Auth: cursorLogin,
		Ask: []string{"cursor/ask_question"}, Answers: map[string]json.RawMessage{"cursor/create_plan": cursorHeadlessAnswers["cursor/create_plan"]}}
	return Launch{Dir: dir, Conversation: conversation, ACP: &run}, nil
}

// Its models are Cursor's, as the cursor harness's are.
func (c *CursorACP) ConverseModel() string     { return CursorConverseModel }
func (c *CursorACP) OrchestratorModel() string { return CursorOrchestratorModel }

// Converse runs one turn of a conversation in ask mode, its commands
// those a conversation may run, going on with its session.
func (c *CursorACP) Converse(r ConverseRun) (Launch, error) {
	agent, err := c.agent()
	if err != nil {
		return Launch{}, err
	}
	if r.Scratch == "" {
		return Launch{}, errors.New("cursor-acp: no folder to keep its run in")
	}
	if r.Session != "" && !c.sessionKept(r.Session) {
		return Launch{}, fmt.Errorf("%w: Cursor has no ACP session %s", ErrSessionGone, r.Session)
	}
	run := ACPRun{Agent: agent, Dir: r.Dir, Load: r.Session, Mode: cursorACPMode, Model: cursorACPModel(r.Model), Prompt: r.Prompt, Allow: converseCommands(r),
		Auth: cursorLogin, Answers: cursorHeadlessAnswers}
	if r.Alone {
		run.Allow = nil
	}
	return acpLaunch(run, r.Scratch)
}

// Orchestrate runs the orchestrator in ask mode, its commands those it
// may run.
func (c *CursorACP) Orchestrate(r OrchestratorRun) (Launch, error) {
	agent, err := c.agent()
	if err != nil {
		return Launch{}, err
	}
	if r.Scratch == "" {
		return Launch{}, errors.New("cursor-acp: no folder to keep its run in")
	}
	run := ACPRun{Agent: agent, Dir: r.Dir, Mode: cursorACPMode, Model: cursorACPModel(r.Model), Prompt: r.Prompt, Allow: orchestratorCommands(r),
		Auth: cursorLogin, Answers: cursorHeadlessAnswers}
	return acpLaunch(run, r.Scratch)
}

func (c *CursorACP) agent() ([]string, error) {
	cur, err := c.cli()
	if err != nil {
		return nil, err
	}
	bin, err := cur.bin()
	if err != nil {
		return nil, err
	}
	return []string{bin, "acp"}, nil
}

// cursorEffort is an effort the CLI's model names end in.
var cursorEffort = regexp.MustCompile(`-(low|medium|high|xhigh|max)$`)

// cursorACPModel is a model as the cursor harness names it, for ACP: the
// CLI's names carry an effort ("claude-haiku-5-5-medium"), but ACP offers
// each model once, at its own effort, so the effort is dropped and the
// model found by its name ("claude-haiku-5-5"). Others ("auto", a name
// ACP has) are as given.
func cursorACPModel(m string) string {
	return cursorEffort.ReplaceAllString(strings.TrimSpace(m), "")
}

// sessionKept is whether Cursor can load an ACP session: it has its
// store.db. Where it can't tell (no sessions folder at all), it says yes,
// and loading says if not.
func (c *CursorACP) sessionKept(id string) bool {
	dir := c.sessions
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return true
		}
		dir = filepath.Join(home, ".cursor", "acp-sessions")
	}
	if _, err := os.Stat(dir); err != nil {
		return true
	}
	if id == "" || strings.ContainsAny(id, `/\`) || strings.HasPrefix(id, ".") {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, id, "store.db"))
	return err == nil
}
