package harness

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/acp"
)

// Codex runs OpenAI's Codex over ACP, through its adapter codex-acp
// (github.com/agentclientprotocol/codex-acp, npm
// @agentclientprotocol/codex-acp). It's ws's first ACP harness: the
// headless roles (conversations, the orchestrator, the Team page), and
// crew, held by the daemon and seen in the web UI.
//
// VERIFIED (codex-acp 2.6.0, Codex 0.166): sessions offer four modes:
// read-only (a read-only sandbox, no network, and anything more asked of
// the client), workspace-write (the session's folder writable), agent
// ("Auto review": a reviewer model approves most things itself) and
// agent-full-access (no sandbox, nothing asked). Headless runs use
// read-only, so ws's policy (CommandPolicy) decides every command that
// leaves the sandbox, and the sandbox limits the rest: an approved command
// runs outside it. Permission requests carry the command and its folder as
// rawInput, and offer allow_once, an always-allow and cancel (reject_once).
// A command's output comes in _meta.terminal_output_delta, as ws offers no
// terminal. Sessions are Codex's own threads (~/.codex/sessions), so
// `codex resume <id>` opens one in Codex's TUI, and session/resume picks
// one up without replaying it.
type Codex struct {
	// lookPath is swapped in tests, and sessions (where Codex keeps its
	// sessions; "" for $CODEX_HOME/sessions, else ~/.codex/sessions).
	lookPath func(string) (string, error)
	sessions string
}

// EnvCodexACP overrides the codex-acp command (a name or path, or a
// command line: "npx -y @agentclientprotocol/codex-acp").
const EnvCodexACP = "WS_CODEX_ACP"

// codexACPBinary is the adapter's command once installed (npm install -g
// @agentclientprotocol/codex-acp).
const codexACPBinary = "codex-acp"

// The modes ws runs Codex in (codex-acp's ids).
const codexReadOnly = "read-only"

// Codex's models by role, as its ACP settings name them: a model, then
// its reasoning effort in brackets (the adapter's own spelling, in its
// list of models). The orchestrator's is small and quick, as Haiku is on
// Claude Code; conversations' is the workhorse.
const (
	CodexOrchestratorModel = "gpt-6-luna[low]"
	CodexConverseModel     = "gpt-6.1-sol[medium]"
)

func init() {
	register(&Codex{lookPath: exec.LookPath})
}

func (c *Codex) Name() string { return "codex" }

// agent is codex-acp's command.
func (c *Codex) agent() ([]string, error) {
	if env := strings.Fields(os.Getenv(EnvCodexACP)); len(env) > 0 {
		p, err := c.lookPath(env[0])
		if err != nil {
			return nil, fmt.Errorf("%s=%q: %w", EnvCodexACP, os.Getenv(EnvCodexACP), err)
		}
		return append([]string{p}, env[1:]...), nil
	}
	p, err := c.lookPath(codexACPBinary)
	if err != nil {
		return nil, fmt.Errorf("could not find codex-acp: install it (npm install -g @agentclientprotocol/codex-acp), or set %s", EnvCodexACP)
	}
	return []string{p}, nil
}

func (c *Codex) Available() error {
	_, err := c.agent()
	return err
}

// Codex crew run over ACP, held by the daemon, seen in the web UI.
func (c *Codex) held() (CrewVia, TranscriptVia) { return CrewACP, TranscriptACP }

// codexWorkspaceWrite is the mode crew run in: the capsule writable (and
// its git folder, for commits), no network, and anything more asked.
const codexWorkspaceWrite = "workspace-write"

// codexCrewNote is added to crew's opening prompt: what their sandbox
// lets them do, and that what needs more is asked for, not given up on.
const codexCrewNote = "\n\nYour shell runs in a sandbox: you can change files in this capsule and commit, but not write outside it, and there's no network. ws commands (ws mission …, ws lift, ws show), git push and gh need to run outside it: request escalated permissions for those when you run them (don't wait for them to fail), and ws approves them. Anything else that needs more (installing a package, the network) also needs escalated permissions: ask, and the user decides."

// Start is new crew: a Codex session in the capsule, in workspace-write
// mode, its opening prompt the brief.
func (c *Codex) Start(b Brief) (Launch, error) {
	agent, err := c.agent()
	if err != nil {
		return Launch{}, err
	}
	run := ACPRun{Agent: agent, Dir: b.Dir, Dirs: gitDirsOf(b.Dir), Mode: codexWorkspaceWrite,
		Allow: crewCommands, Prompt: b.Prompt + codexCrewNote}
	run.Model, run.Settings = codexModel(b.Model)
	return Launch{Dir: b.Dir, ACP: &run}, nil
}

// Resume goes on with crew's session: Codex picks it up without replaying
// it (ws has its record).
func (c *Codex) Resume(dir, conversation string) (Launch, error) {
	if conversation == "" {
		return Launch{}, errors.New("cannot resume: no Codex session recorded for this crew")
	}
	agent, err := c.agent()
	if err != nil {
		return Launch{}, err
	}
	run := ACPRun{Agent: agent, Dir: dir, Dirs: gitDirsOf(dir), Load: conversation, Mode: codexWorkspaceWrite, Allow: crewCommands}
	return Launch{Dir: dir, Conversation: conversation, ACP: &run}, nil
}

func (c *Codex) ConverseModel() string     { return CodexConverseModel }
func (c *Codex) OrchestratorModel() string { return CodexOrchestratorModel }

// Converse runs one turn of a conversation: read-only, its commands those
// a conversation may run (converseCommands), going on with its session;
// or, for one that runs things (r.Sandbox), in workspace-write.
func (c *Codex) Converse(r ConverseRun) (Launch, error) {
	agent, err := c.agent()
	if err != nil {
		return Launch{}, err
	}
	if r.Scratch == "" {
		return Launch{}, errors.New("codex: no folder to keep its run in")
	}
	if r.Session != "" && !c.sessionKept(r.Session) {
		return Launch{}, fmt.Errorf("%w: Codex has no session %s", ErrSessionGone, r.Session)
	}
	run := ACPRun{Agent: agent, Dir: r.Dir, Load: r.Session, Mode: codexReadOnly, Prompt: r.Prompt + codexSandboxNote, Allow: converseCommands(r), Images: r.Images}
	if r.Alone {
		run.Allow, run.Prompt = nil, r.Prompt // nothing leaves the sandbox
	} else if r.Sandbox && len(r.Write) > 0 {
		// One that runs things: workspace-write, from its own folder (the
		// session's, which the sandbox lets it write), the capsules it's
		// lifted and git's folders for them its others. Not from the
		// workspace, which would open it all. What needs more it asks
		// for, and ConversePolicy answers; its prompt says how
		// (prompt.Sandbox.Asks).
		run.Mode, run.Dir = codexWorkspaceWrite, r.Write[0]
		run.Dirs = append(append([]string{}, r.Write[1:]...), r.Beside...)
		run.Runs, run.Folders = true, r.Folders
		run.Prompt = runsFrom(r.Write[0], r.Dir) + r.Prompt
	}
	run.Model, run.Settings = codexModel(r.Model)
	return acpLaunch(run, r.Scratch)
}

// Codex runs a conversation's shell in its own sandbox, in workspace-write
// (Converse), which keeps the network closed, and lets out whatever the
// agent asks to run outside it, as ws answers (ConversePolicy): so it's
// an Escalator. VERIFIED (codex-acp 2.6.0, Codex 0.170, workspace-write
// with a folder given as additionalDirectories): apply_patch and the
// shell write in the session's folder and that one without asking;
// apply_patch elsewhere is asked of the client, its path in locations,
// and refused by ConversePolicy (the agent's told why); a shell write
// elsewhere fails "operation not permitted", and curl "Could not resolve
// host"; asked with escalated permissions, each comes to the client as a
// command (rawInput: command and cwd), and allowed, runs outside the
// sandbox. There's no asking for the network alone: the mode's own
// policy says networkAccess false, over Codex's config. The system's temp
// folders are writable too (/tmp, $TMPDIR).
func (c *Codex) Sandboxes() bool { return true }
func (c *Codex) Escalates() bool { return true }

// Orchestrate runs the orchestrator: read-only, its commands those it may
// run (orchestratorCommands), which the sandbox would otherwise stop (ws
// mission writes the mission; gh needs the network).
func (c *Codex) Orchestrate(r OrchestratorRun) (Launch, error) {
	agent, err := c.agent()
	if err != nil {
		return Launch{}, err
	}
	if r.Scratch == "" {
		return Launch{}, errors.New("codex: no folder to keep its run in")
	}
	run := ACPRun{Agent: agent, Dir: r.Dir, Mode: codexReadOnly, Prompt: r.Prompt + codexSandboxNote, Allow: orchestratorCommands(r)}
	run.Model, run.Settings = codexModel(r.Model)
	return acpLaunch(run, r.Scratch)
}

// codexSandboxNote is added to a headless run's prompt. Run read-only, a
// command that writes (ws mission's, ws lift) or reaches the network (gh)
// fails inside Codex's sandbox, and a small model, seeing it fail, takes
// the workspace for read-only and gives up rather than asking to run it
// outside (VERIFIED, gpt-6-luna low: it never asked). Asking is what
// brings the command to ws's policy.
const codexSandboxNote = "\n\nYour shell runs in a read-only sandbox without network access. ws commands (they reach the workspace's daemon, which the sandbox keeps them from) and gh need to run outside it: request escalated permissions for each of those when you run it (don't wait for it to fail first), and ws approves the ones you're allowed. Run each command on its own, not chained with others."

// PendingAtTurnEnd is what Codex left running when a turn ended: each
// command still running, a background terminal, which Codex never wakes
// the agent for (acpLeftRunning).
//
// VERIFIED (codex-acp 2.6.0, Codex 0.166, told to start `sleep 45; echo
// finished > bg-done.txt` in the background and end its turn): Codex's
// command tool hands a command that's still running back as a "background
// terminal", and the turn ends: stopReason end_turn, with the command's
// tool_call (kind execute) still in_progress. When the command ended, 45 s
// later, codex-acp sent one tool_call_update, status completed with
// _meta.terminal_exit, outside any turn, and started no turn. READ (its
// bundled code): at a turn's end it forgets the calls it had open without
// failing them, so a background terminal's call is finished only by that
// update; it reports background terminals as async_task_spawned and
// async_task_state_update only to a client that declares JetBrains'
// asyncTasks capability, which ws doesn't; closing the session stops them.
func (c *Codex) PendingAtTurnEnd(t acp.Turn) Pending { return acpLeftRunning(t) }

// Shells: Codex runs commands from its app-server, which codex-acp runs,
// so none is in a shell that is the direct child of the agent ws started
// (codex-acp, under its launcher's /bin/sh): its background terminals are
// its calls (PendingAtTurnEnd). The process check finds work the agent
// detached, which nothing wakes it for. Were one found, nothing would wake
// it for that either.
//
// READ: pnpm's codex-acp (2.6.0) is a /bin/sh script that runs node
// without exec. VERIFIED (Codex 0.166, on macOS): a background terminal is
// a child of the Codex app-server, in a process group of its own.
func (c *Codex) Shells(Pending) (Wake, bool) { return WakeNo, false }

// codexModel is a model as ws's config names it for Codex, "gpt-6.1-sol"
// or with its effort, "gpt-6.1-sol[medium]", as the model and the setting
// for its effort.
func codexModel(m string) (string, []acp.Setting) {
	name, effort, ok := strings.Cut(strings.TrimSpace(m), "[")
	if !ok {
		return name, nil
	}
	effort = strings.TrimSuffix(effort, "]")
	if effort == "" {
		return name, nil
	}
	return name, []acp.Setting{{Category: "thought_level", Want: effort}}
}

// sessionKept is whether Codex still has a session: a rollout file of its
// id, under sessions/<year>/<month>/<day>/. Where it can't tell (no
// sessions folder at all), it says yes, and resuming says if not.
func (c *Codex) sessionKept(id string) bool {
	dir := c.sessions
	if dir == "" {
		home := os.Getenv("CODEX_HOME")
		if home == "" {
			h, err := os.UserHomeDir()
			if err != nil {
				return true
			}
			home = filepath.Join(h, ".codex")
		}
		dir = filepath.Join(home, "sessions")
	}
	if _, err := os.Stat(dir); err != nil {
		return true
	}
	if strings.ContainsAny(id, `/\*?[`) {
		return false
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*", "*", "*", "rollout-*-"+id+".jsonl"))
	return len(matches) > 0
}
