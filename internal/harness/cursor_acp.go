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
// READ: ask mode ("Q&A mode - no edits or command execution", Cursor
// says of it) runs a shell command only in a read-only sandbox, or not at
// all, so it can't run ws's --propose commands: a conversation in it can't
// brief or plan, nor the orchestrator dispatch. Plan mode is read-only
// too. So every role runs in agent mode. There, without --force, a shell
// command that Cursor's own allowlists (~/.cursor/cli-config.json, a
// project's .cursor/cli.json) don't allow is asked of the client, with
// the command as its title, in backticks, and ws's policy decides it, with
// no cli.json of ws's. A no is "reject-once" and the turn goes on. A write
// in the session's folder (its workspace) isn't asked for at all, unless
// you've told Cursor to ask; one outside it is, with its diff ("Out of
// workspace"), and ws refuses it. A read outside it is asked for only
// where a read boundary is set, and ws allows it. So the headless roles
// run from a folder of ws's own (cursorRunDir), not the one they work on,
// and are told where that is: Cursor's shell takes the folder to run a
// command in. Its questions (cursor/ask_question) are answered skipped,
// with why, and a plan for approval (cursor/create_plan, plan mode's)
// rejected: nobody's there. Unanswered, Cursor would ask each
// single-choice question as a permission instead, and drop the rest.
type CursorACP struct {
	// cursor finds the CLI (nil: the cursor harness's own); tests set one.
	cursor *Cursor
	// sessions is where Cursor keeps ACP sessions ("" for
	// ~/.cursor/acp-sessions); tests change it.
	sessions string
}

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

// cursorRetriable is how the last line of a turn's reply begins when an
// error Cursor names retriable ended it.
const cursorRetriable = "Error: RetriableError:"

// TurnDropped: a turn that ended on an error Cursor calls retriable.
//
// SEEN (cursor-agent, November 2026): when the agent's connection to
// Cursor's servers drops mid-turn, the prompt answers {stopReason:
// end_turn} as for a turn it finished. Its last message ends with a line
// "Error: RetriableError: …" saying what closed the stream, and the tool
// call it was in is left in_progress. Cursor doesn't retry it. The
// session stays up, and a message sent to it goes on from where it was.
func (c *CursorACP) TurnDropped(t acp.Turn) (string, bool) {
	if t.Err != nil || t.Stop != "end_turn" {
		return "", false
	}
	lines := strings.Split(strings.TrimSpace(t.Reply), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if !strings.HasPrefix(last, cursorRetriable) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(last, "Error:")), true
}

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

// cursorACPMode is the mode every role runs in: Cursor's own agent mode,
// all its tools, asking leave for what its allowlists don't allow. Crew
// work in their capsule; the headless roles in a folder of ws's own
// (cursorRunDir), which is all they may write in.
const cursorACPMode = "agent"

// Start is new crew: a Cursor session in the capsule, in agent mode, its
// opening prompt the brief. Its questions (cursor/ask_question) wait for
// you, as a permission does; a plan for approval is rejected (crew don't
// run in plan mode).
func (c *CursorACP) Start(b Brief) (Launch, error) {
	agent, err := c.agent()
	if err != nil {
		return Launch{}, err
	}
	run := ACPRun{Agent: agent, Dir: b.Dir, Mode: cursorACPMode, Model: cursorACPModel(b.Model), Allow: crewCommands,
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
	run := ACPRun{Agent: agent, Dir: dir, Load: conversation, Mode: cursorACPMode, Allow: crewCommands, Auth: cursorLogin,
		Ask: []string{"cursor/ask_question"}, Answers: map[string]json.RawMessage{"cursor/create_plan": cursorHeadlessAnswers["cursor/create_plan"]}}
	return Launch{Dir: dir, Conversation: conversation, ACP: &run}, nil
}

// Its models are Cursor's, as the cursor harness's are.
// Its defaults are the cursor harness's models, named as ACP has them:
// the effort the CLI's names say isn't one ACP runs at (cursorACPModel).
func (c *CursorACP) ConverseModel() string     { return cursorACPModel(CursorConverseModel) }
func (c *CursorACP) OrchestratorModel() string { return cursorACPModel(CursorOrchestratorModel) }

// Converse runs one turn of a conversation in agent mode, from a folder
// of ws's own, its commands those a conversation may run, going on with
// its session.
func (c *CursorACP) Converse(r ConverseRun) (Launch, error) {
	agent, err := c.agent()
	if err != nil {
		return Launch{}, err
	}
	if r.Session != "" && !c.sessionKept(r.Session) {
		return Launch{}, fmt.Errorf("%w: Cursor has no ACP session %s", ErrSessionGone, r.Session)
	}
	dir, err := cursorRunDir(r.Scratch)
	if err != nil {
		return Launch{}, err
	}
	works := cursorWorksIn(dir, r.Dir)
	runs := r.Sandbox && !r.Alone
	if runs {
		works = cursorRunsIn(dir, r.Dir)
	}
	run := ACPRun{Agent: agent, Dir: dir, Load: r.Session, Mode: cursorACPMode, Model: cursorACPModel(r.Model), Prompt: works + r.Prompt,
		Allow: converseCommands(r), Auth: cursorLogin, Answers: cursorHeadlessAnswers}
	if runs {
		run.Runs, run.Folders = true, r.Folders
	}
	if r.Alone {
		run.Allow = nil
	}
	return acpLaunch(run, r.Scratch)
}

// Trusted: a conversation's agent on Cursor runs things, with no sandbox
// of Cursor's own, ws answering what it asks (ConversePolicy). The user's
// call (2026-11-26): "I trust cursor", as crew run there.
func (c *CursorACP) Trusted() bool { return true }

// cursorRunsIn is cursorWorksIn for a conversation that runs things: it
// writes where its instructions say, not nowhere.
func cursorRunsIn(ran, dir string) string {
	if dir == "" || dir == ran {
		return ""
	}
	return "(ws runs you from " + ran + ", a folder of its own, but the folder you work on is " + dir +
		": where your instructions say \"here\" or \"you're in\", they mean " + dir +
		". Give it in full when you read a file, and as the folder to run a shell command in. Write files only where your instructions say you may.)\n\n"
}

// Orchestrate runs the orchestrator in agent mode, from a folder of ws's
// own, its commands those it may run.
func (c *CursorACP) Orchestrate(r OrchestratorRun) (Launch, error) {
	agent, err := c.agent()
	if err != nil {
		return Launch{}, err
	}
	dir, err := cursorRunDir(r.Scratch)
	if err != nil {
		return Launch{}, err
	}
	run := ACPRun{Agent: agent, Dir: dir, Mode: cursorACPMode, Model: cursorACPModel(r.Model), Prompt: cursorWorksIn(dir, r.Dir) + r.Prompt,
		Allow: orchestratorCommands(r), Auth: cursorLogin, Answers: cursorHeadlessAnswers}
	return acpLaunch(run, r.Scratch)
}

// cursorRunDir is the folder a headless role's Cursor session runs from:
// one of ws's own in scratch. Cursor writes in its session's folder
// without asking, so that's this, never the workspace or a capsule; a
// write anywhere else is asked of ws, which refuses it. One folder for
// every run: a conversation's session is loaded from where it began.
func cursorRunDir(scratch string) (string, error) {
	if scratch == "" {
		return "", errors.New("cursor-acp: no folder to keep its run in")
	}
	dir := filepath.Join(scratch, "cursor-acp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// cursorWorksIn tells a headless role run from ran that the folder it
// works on is dir: its prompt says "you're in" that one.
func cursorWorksIn(ran, dir string) string {
	if dir == "" || dir == ran {
		return ""
	}
	return "(ws runs you from " + ran + ", a folder of its own, but the folder you work on is " + dir +
		": where your instructions say \"here\" or \"you're in\", they mean " + dir +
		". Give it in full when you read a file, and as the folder to run a shell command in. Don't write files: you may only read, and run the commands your instructions allow.)\n\n"
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

// cursorSuffix is what the CLI's model names end in that ACP's don't: an
// effort, thinking (before or after the effort), fast.
var cursorSuffix = regexp.MustCompile(`-(?:none|minimal|low|medium|high|xhigh|max|extra-high|thinking|fast)$`)

// cursorACPModel is a model as the cursor harness names it, for ACP: the
// CLI's names carry an effort ("claude-haiku-5-5-medium"), but ACP offers
// each model once, at its own effort, so the effort is dropped and the
// model found by its name ("claude-haiku-5-5"). So are "-fast" and
// "-thinking": ACP's model is the one name either way
// ("claude-opus-5-5-high-fast" is "claude-opus-5-5",
// "claude-opus-5-thinking-high" "claude-opus-5"). Others ("auto", a name
// ACP has) are as given.
//
// VERIFIED (cursor-agent 2026.11.22, a paid plan, the model each turn ran
// on read from Cursor's usage dashboard): ACP runs a model at the one
// effort it lists for it, whatever's asked. `--model claude-haiku-5-5-low
// acp`, -max, -thinking-low, a bracket override, and the CLI's own config
// set to low before launching, each ran "Haiku 5.5 thinking high". A value
// it doesn't list is refused ("Invalid model value"). --model at launch
// does pick the model (while session/new reports the last one chosen).
func cursorACPModel(m string) string {
	m = strings.TrimSpace(m)
	for {
		bare := cursorSuffix.ReplaceAllString(m, "")
		if bare == m {
			return m
		}
		m = bare
	}
}

// sessionsDir is where Cursor keeps ACP sessions ("" if it can't tell).
func (c *CursorACP) sessionsDir() string {
	if c.sessions != "" {
		return c.sessions
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cursor", "acp-sessions")
}

// sessionKept is whether Cursor can load an ACP session: it has its
// store.db. Where it can't tell (no sessions folder at all), it says yes,
// and loading says if not.
func (c *CursorACP) sessionKept(id string) bool {
	dir := c.sessionsDir()
	if dir == "" {
		return true
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
