package harness

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// Claude runs Claude Code (the claude CLI).
//
// Every assumption about the Claude Code CLI lives in this file. Those
// checked against a real claude (2.1.318) say VERIFIED.
type Claude struct {
	// lookPath and newID are swapped in tests, as is goos (the system it's
	// on: "" for this one).
	lookPath func(string) (string, error)
	newID    func() (string, error)
	goos     string
}

// EnvClaudeBin overrides the Claude Code binary (name or path).
const EnvClaudeBin = "WS_CLAUDE_BIN"

func init() { register(&Claude{lookPath: exec.LookPath, newID: newSessionID}) }

func (c *Claude) Name() string { return "claude" }

func (c *Claude) held() (CrewVia, TranscriptVia) { return CrewTerminal, TranscriptSession }

func (c *Claude) bin() (string, error) {
	name := "claude"
	if b := os.Getenv(EnvClaudeBin); b != "" {
		name = b
	}
	p, err := c.lookPath(name)
	if err != nil {
		return "", fmt.Errorf("claude CLI not found (looked for %s; set %s to override)", name, EnvClaudeBin)
	}
	return p, nil
}

func (c *Claude) Available() error {
	_, err := c.bin()
	return err
}

// Start builds new crew (or a briefing agent).
//
// VERIFIED: --session-id <uuid> names the conversation before it starts, so
// ws knows what to resume without asking the CLI for an id.
//
// VERIFIED: the opening prompt goes in as the argument, not typed in. In a
// folder Claude Code hasn't been trusted in, it opens on a trust dialog whose
// default answer is "No, exit": a typed-in prompt's Enter would choose that.
// A prompt argument waits for the user's answer and runs after it. The
// session stays interactive afterwards.
//
// VERIFIED: in a git repo, trust is per repository. Trusting one capsule
// covered that repo's later capsules (worktrees of the same repo), but
// trusting the workspace root (the config repo) didn't cover the repos
// under it, so the first crew in each repo asks once. Until someone answers,
// no hook runs: the host watches for the dialog (crew.WatchTrust), and marks
// the crew as asking you, or answers it where ws.local.toml says to.
//
// VERIFIED: --name sets the session's display name, which is also the
// terminal title the deck shows.
func (c *Claude) Start(b Brief) (Launch, error) {
	bin, err := c.bin()
	if err != nil {
		return Launch{}, err
	}
	id, err := c.newID()
	if err != nil {
		return Launch{}, fmt.Errorf("making a session id: %w", err)
	}
	args := []string{bin, "--session-id", id, "--name", sessionName(b)}
	if b.Model != "" {
		args = append(args, "--model", b.Model)
	}
	prompt := b.Prompt
	args = append(args, "--", prompt)
	return Launch{Args: args, Dir: b.Dir, Conversation: id, Notes: []string{
		fmt.Sprintf("chose session id %s (--session-id)", id),
		fmt.Sprintf("opening prompt is its last argument (%d chars):\n%s", len(prompt), prompt),
	}}, nil
}

// sessionName is how the session reads in claude's /resume picker and the
// terminal title.
func sessionName(b Brief) string {
	if b.Role == "briefing" {
		return "ws " + b.Slug + " · briefing"
	}
	return "ws " + b.Slug
}

// VERIFIED: --resume <session-id> reopens the conversation interactively,
// history and all.
func (c *Claude) Resume(dir, conversation string) (Launch, error) {
	if conversation == "" {
		return Launch{}, fmt.Errorf("no claude session id recorded for this crew, so it can't be resumed")
	}
	bin, err := c.bin()
	if err != nil {
		return Launch{}, err
	}
	return Launch{Args: []string{bin, "--resume", conversation}, Dir: dir, Conversation: conversation}, nil
}

// ClaudeOrchestratorModel is the model Claude Code runs the orchestrator on
// unless [orchestrator] model says otherwise (an alias for the latest).
const ClaudeOrchestratorModel = "haiku"

func (c *Claude) OrchestratorModel() string { return ClaudeOrchestratorModel }

// claudeOrchestratorTools is all the orchestrator may use. It acts through
// ws mission (and ws lift for a capsule the next crew needs) and looks around
// read-only; anything else is refused without a prompt (dontAsk), and the
// refusal comes back to it as an error it can correct.
//
// VERIFIED: a chained command (a && b; c | d) runs if every part is allowed,
// except cd <dir> && git …, which Claude Code always refuses. One refused
// command reads to a model as "Bash is denied" (that's how the refusal is
// worded): it stops using the shell and never runs its ws commands. So the
// harmless helpers models chain by habit are allowed, and the prompt asks
// for one command per call and git -C <capsule path> (see capsuleGitRules).
var claudeOrchestratorTools = []string{
	"Read", "Grep", "Glob",
	"Bash(ws mission *)", "Bash(workspace mission *)",
	"Bash(ws lift *)", "Bash(workspace lift *)",
	"Bash(ws status *)", "Bash(workspace status *)",
	"Bash(ws show)", "Bash(ws show *)", "Bash(workspace show)", "Bash(workspace show *)",
	"Bash(ws review show *)", "Bash(workspace review show *)",
	"Bash(ws find *)", "Bash(workspace find *)",
	"Bash(git diff *)", "Bash(git log *)", "Bash(git show *)", "Bash(git status *)",
	"Bash(gh pr view *)", "Bash(gh pr checks *)", "Bash(gh pr list *)",
	"Bash(cd *)", "Bash(pwd)", "Bash(ls *)", "Bash(cat *)", "Bash(head *)", "Bash(tail *)", "Bash(wc *)", "Bash(echo *)",
}

// capsuleGitRules allows read-only git in each capsule (and .ground) by its
// exact path.
//
// VERIFIED: a * before the git subcommand (Bash(git -C * log *)) would also
// match options put in its place, and git -c / --exec-path run arbitrary
// commands; Claude Code warns about such rules. With the path spelled out,
// * only covers what follows the subcommand, where -c can't go.
func capsuleGitRules(paths []string) []string {
	var rules []string
	for _, p := range paths {
		if p == "" || strings.ContainsAny(p, " \t\n\"'()*") {
			continue // can't be written into a rule exactly; cd and plain git still work
		}
		for _, sub := range []string{"diff", "log", "show", "status"} {
			rules = append(rules, "Bash(git -C "+p+" "+sub+" *)")
		}
	}
	return rules
}

// Orchestrate runs a headless orchestrator in print mode.
//
// VERIFIED (claude 2.1.319): --output-format stream-json (which in print
// mode needs --verbose) prints a JSON line per event: assistant messages
// with their tool_use blocks, user messages with each tool_result
// (is_error, and for a refused command the text "Permission to use Bash has
// been denied because Claude Code is running in don't ask mode"), and last
// a result line with the final message and permission_denials. ws keeps
// what it ran from them. Some read-only commands (date) run unlisted.
//
// VERIFIED: -p runs one prompt and skips the trust dialog. --permission-mode dontAsk with
// --allowedTools runs what's allowed and refuses the rest without asking;
// --allowedTools takes one rule per argument, so -- ends the list before the
// prompt. --no-session-persistence keeps these runs out of the user's
// /resume list (their output is in orchestrator.log).
//
// It's given the whole prompt as its argument, as the judge's and the PR
// writer's are, not the line pointing at its file: reading that cost every
// run a turn first. ws's prompts stay under Linux's limit on one argument
// (128 KiB): orchestrator.promptMax is 112 KiB.
func (c *Claude) Orchestrate(r OrchestratorRun) (Launch, error) {
	bin, err := c.bin()
	if err != nil {
		return Launch{}, err
	}
	args := []string{bin, "-p", "--output-format", "stream-json", "--verbose", "--permission-mode", "dontAsk", "--no-session-persistence"}
	if r.Model != "" {
		args = append(args, "--model", r.Model)
	}
	args = append(args, "--allowedTools")
	args = append(args, claudeOrchestratorTools...)
	args = append(args, capsuleGitRules(append(append([]string(nil), r.Capsules...), r.Grounds...))...)
	return Launch{Args: append(args, "--", r.Prompt), Dir: r.Dir, Stream: StreamClaude}, nil
}

// newSessionID makes a random (version 4) UUID, the form --session-id takes.
func newSessionID() (string, error) {
	var u [16]byte
	if _, err := rand.Read(u[:]); err != nil {
		return "", err
	}
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	return strings.ToLower(fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])), nil
}

// ClaudeConverseModel is what conversations run on when none is
// configured: they're thinking with the user, not just deciding.
const ClaudeConverseModel = "sonnet"

// TalkGit is the git a conversation's agent may run in a capsule or
// .ground (git -C <path> <sub> …): reading only. (Not grep: -O runs a
// pager it names.)
var TalkGit = []string{"diff", "log", "show", "status", "ls-files", "ls-tree", "blame"}

// claudeConverseTurns bounds how far one turn can wander; a sandboxed one
// (claudeSandboxTurns), which runs things as well as reading, goes further.
const (
	claudeConverseTurns = "30"
	claudeSandboxTurns  = "150"
)

// Sandboxes is whether Claude Code's sandbox runs here.
//
// READ (code.claude.com/docs/en/sandboxing): on macOS it's Seatbelt, which
// is always there; on Linux and WSL2 it needs bubblewrap and socat; it
// runs nowhere else. Where it can't start, Claude Code runs commands
// unsandboxed, and then none is allowed for being sandboxed: so ws only
// asks for it where it's there.
func (c *Claude) Sandboxes() bool {
	goos := c.goos
	if goos == "" {
		goos = runtime.GOOS
	}
	switch goos {
	case "darwin":
		return true
	case "linux":
		_, bwrap := c.lookPath("bwrap")
		_, socat := c.lookPath("socat")
		return bwrap == nil && socat == nil
	}
	return false
}

// Converse runs one turn of a conversation headless.
//
// VERIFIED (claude 2.1.319): -p --resume <id> continues a session
// headless, keeping its id, with --allowedTools and stream-json; the
// first run's init line carries the new session's id. Sessions are kept
// per directory, so every turn of a conversation runs from the same one.
func (c *Claude) Converse(r ConverseRun) (Launch, error) {
	bin, err := c.bin()
	if err != nil {
		return Launch{}, err
	}
	sandbox := r.Sandbox && !r.Alone && c.Sandboxes()
	turns := claudeConverseTurns
	if sandbox {
		turns = claudeSandboxTurns
	}
	args := []string{bin, "-p", "--output-format", "stream-json", "--verbose", "--permission-mode", "dontAsk", "--max-turns", turns}
	if r.Model != "" {
		args = append(args, "--model", r.Model)
	}
	if r.Session != "" {
		args = append(args, "--resume", r.Session)
	}
	if r.Once {
		// VERIFIED (claude --help): with --print, sessions aren't saved
		// to disk and can't be resumed.
		args = append(args, "--no-session-persistence")
	}
	if r.Alone {
		// VERIFIED (claude --help): --tools "" disables every built-in tool.
		// A run with no tools has nothing to show as it goes but what it's
		// writing (a review's guide, a cold pass), so it says that as it
		// writes it. VERIFIED (claude 2.1.326): --include-partial-messages
		// adds stream_event lines (message_start, then content_block_delta
		// with each text_delta) before each message's own assistant line,
		// which, and the result, are as they were (readClaudeStream).
		return Launch{Args: append(args, "--include-partial-messages", "--tools", "", "--", r.Prompt), Dir: r.Dir, Stream: StreamClaude, Conversation: r.Session}, nil
	}
	allow := converseCommands(r)
	args = append(args, "--allowedTools", "Read", "Grep", "Glob")
	for _, cmd := range allow {
		args = append(args, "Bash("+cmd+")", "Bash("+cmd+" *)")
	}
	settings := map[string]any{}
	if len(r.Guard) > 0 && r.Scratch != "" {
		hooks, err := claudeGuard(r.Guard, filepath.Join(r.Scratch, "claude"), allow, sandbox)
		if err != nil {
			return Launch{}, err
		}
		settings["hooks"] = hooks
	}
	if sandbox {
		box, err := claudeSandbox(r)
		if err != nil {
			return Launch{}, err
		}
		settings["sandbox"] = box
	}
	if len(settings) > 0 {
		data, err := json.Marshal(settings)
		if err != nil {
			return Launch{}, err
		}
		args = append(args, "--settings", string(data))
	}
	return Launch{Args: append(args, "--", r.Prompt), Dir: r.Dir, Stream: StreamClaude, Conversation: r.Session}, nil
}

// claudeSandbox is the --settings that runs a conversation's shell in
// Claude Code's sandbox: anything runs there unasked, reading anything
// and reaching the network; it writes in r.Write and r.Beside, the
// system's temp folder, and nowhere else under r.Dir (closedUnder); and
// the commands that can't run inside it (outside) leave it, where the
// rules above and ws's guard decide them as they always did.
//
// It's loose on purpose, for now: every host, every socket. A conversation
// competes with a harness run by hand, which asks nothing, so what's
// closed is what would change the workspace under a mission or under you,
// and no more. Tighten it from what goes wrong.
//
// VERIFIED (claude 2.1.327, -p --permission-mode dontAsk, on macOS):
//   - With enabled and autoAllowBashIfSandboxed, a plain command runs with
//     no rule allowing it, piped and chained as it likes; what Claude Code
//     doesn't read as plain (a loop, a variable set before a command), ws's
//     guard allows (GuardAnswer).
//   - It writes in the folder it runs in and the system's temp folder
//     ($TMPDIR is /tmp/claude-<uid> there), and in allowWrite's; anywhere
//     else is "Operation not permitted". A path in denyWrite stays closed
//     though allowWrite names it, or a folder inside it: so what's open
//     under r.Dir is left off denyWrite, not opened over it.
//   - The network is closed unless allowedDomains names the host; "*" is
//     every host. A commit signed over a socket (1Password's agent) needs
//     allowAllUnixSockets, and a test that listens on localhost needs
//     allowLocalBinding.
//   - excludedCommands takes a Bash rule's pattern ("ws *", not "ws"): a
//     command matching one runs outside the sandbox, by the rules (so
//     refused, in dontAsk, unless one allows it). Only alone: piped,
//     chained or redirected (a heredoc is a redirection), the whole stays
//     inside, where ws can't write its records and gh can't read the
//     keychain. So ws's guard moves a heredoc's body to a file
//     (heredocAside).
//   - With allowUnsandboxedCommands false the agent can't ask to run a
//     command outside (the Bash tool's dangerouslyDisableSandbox).
func claudeSandbox(r ConverseRun) (map[string]any, error) {
	open := append(append([]string{}, r.Write...), r.Beside...)
	closed, err := closedUnder(r.Dir, open, r.Room)
	if err != nil {
		return nil, err
	}
	var excluded []string
	for _, name := range outside {
		excluded = append(excluded, name+" *")
	}
	return map[string]any{
		"enabled":                  true,
		"autoAllowBashIfSandboxed": true,
		"allowUnsandboxedCommands": false,
		"excludedCommands":         excluded,
		"network":                  map[string]any{"allowedDomains": []string{"*"}, "allowAllUnixSockets": true, "allowLocalBinding": true},
		"filesystem":               map[string]any{"allowWrite": open, "denyWrite": closed},
	}, nil
}

// claudeGuard is the hooks, for --settings, that have Claude Code ask ws
// before each shell command (a PreToolUse hook on Bash): guard, then a
// file holding allow, written in dir and named by what it allows.
//
// Its own rules (Bash(ws artifact add --propose *)) can't admit a
// proposal's HTML. VERIFIED (claude 2.1.325, --permission-mode dontAsk):
// a command under an allowed prefix whose quoted heredoc's body has a
// double-quoted string inside braces ({ "x" }, as CSS's content: "" and
// a script's object literals do) is denied, every time, though each of
// its lines is allowed alone. A PreToolUse hook that answers
// permissionDecision "allow" lets it run; one that answers nothing leaves
// it to the rules, which still refuse what they don't allow. So the hook
// allows only what ws's own reading of the command allows (commandAllowed,
// as the ACP policy reads it: one command, a quoted heredoc read as it
// is, nothing chained).
//
// VERIFIED (claude 2.1.327): an answer of "deny" refuses the command, and
// its permissionDecisionReason is what the agent's told ("PreToolUse:Bash
// hook error: <reason>"); an answer of "allow" with updatedInput runs the
// command it gives in place of the agent's, placed in or out of the
// sandbox by what it then is.
func claudeGuard(guard []string, dir string, allow []string, sandboxed bool) (map[string]any, error) {
	list, _ := json.Marshal(guardList{Allow: allow, Sandboxed: sandboxed})
	sum := sha256.Sum256(list)
	file := filepath.Join(dir, "allow-"+hex.EncodeToString(sum[:])[:12]+".json")
	if _, err := os.Stat(file); err != nil {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		if err := writeAtomically(file, list); err != nil {
			return nil, err
		}
	}
	var cmd []string
	for _, c := range append(append([]string{}, guard...), file) {
		cmd = append(cmd, shellQuote(c))
	}
	// A sandboxed run's tools for writing files are ws's to decide too:
	// the sandbox is the shell's alone.
	matcher := "Bash"
	if sandboxed {
		matcher = "Bash|" + strings.Join(claudeFileTools, "|")
	}
	return map[string]any{
		"PreToolUse": []map[string]any{{"matcher": matcher, "hooks": []map[string]string{{"type": "command", "command": strings.Join(cmd, " ")}}}},
	}, nil
}

// claudeFileTools are Claude Code's tools that write files.
var claudeFileTools = []string{"Write", "Edit", "MultiEdit", "NotebookEdit"}

// guardList is what ws's guard goes by, as its file holds it: the
// commands allowed, and whether the run's shell is sandboxed.
type guardList struct {
	Allow     []string `json:"allow"`
	Sandboxed bool     `json:"sandboxed,omitempty"`
}

// GuardAnswer is what ws's PreToolUse hook says to Claude Code of the tool
// call event it's given (its JSON), with what it goes by in allowFile:
// allow, for a shell command the list allows (a ws command given its
// input by a heredoc as one reading it from a file beside allowFile:
// heredocAside); deny, saying why and what to do, for one that runs ws or
// gh and isn't allowed (whyRefused); for anything else, allow where the
// run's shell is sandboxed, and nothing ("") where it isn't, which leaves
// it to Claude Code's own rules.
//
// VERIFIED (claude 2.1.327, dontAsk, sandboxed): being sandboxed allows
// only a command Claude Code reads as plain. A loop, a variable set
// before a command or exported, a command run by its path
// (/usr/bin/time -p …): each is refused, by the rules, though it would
// have run in the sandbox. Allowed by this hook, it runs, and in the
// sandbox still: what it writes outside is "Operation not permitted". So
// a command is only allowed here for staying in the sandbox if it runs
// neither ws nor gh (runsOutside), the commands Claude Code lets out.
//
// A sandboxed run's tools for writing files (Write, Edit) aren't the
// sandbox's to stop, and no rule allows them: ws allows one writing in
// folders() (a folder of the agent's own, and the capsules its
// conversation has lifted, as they stand when it's asked: one lifted a
// moment ago is there), and refuses any other, saying where it may write.
// VERIFIED (claude 2.1.327, dontAsk): the hook's allow lets Write and Edit
// run, and its deny's reason is what the agent's told.
func GuardAnswer(event []byte, allowFile string, folders func() []string) string {
	var e struct {
		Tool  string         `json:"tool_name"`
		Input map[string]any `json:"tool_input"`
	}
	if json.Unmarshal(event, &e) != nil {
		return ""
	}
	data, err := os.ReadFile(allowFile)
	if err != nil {
		return ""
	}
	var list guardList
	if json.Unmarshal(data, &list) != nil || len(list.Allow) == 0 {
		return ""
	}
	answer := map[string]any{"hookEventName": "PreToolUse"}
	say := func(decision, why string) string {
		answer["permissionDecision"], answer["permissionDecisionReason"] = decision, why
		out, _ := json.Marshal(map[string]any{"hookSpecificOutput": answer})
		return string(out)
	}
	if slices.Contains(claudeFileTools, e.Tool) {
		if !list.Sandboxed || folders == nil {
			return ""
		}
		path, _ := e.Input["file_path"].(string)
		if path == "" {
			path, _ = e.Input["notebook_path"].(string)
		}
		dirs := folders()
		if path != "" && within(path, dirs) {
			return say("allow", "in a folder the conversation's agent works in")
		}
		return say("deny", whyNotWritten(path, dirs))
	}
	command, _ := e.Input["command"].(string)
	if e.Tool != "Bash" || command == "" {
		return ""
	}
	switch {
	case commandAllowed(command, list.Allow):
		if aside := heredocAside(command, filepath.Join(filepath.Dir(allowFile), asideDir)); aside != "" {
			e.Input["command"] = aside
			answer["updatedInput"] = e.Input
		}
		return say("allow", "on ws's list of what a conversation may run")
	case runsOutside(command) != "":
		return say("deny", whyRefused(command, list.Allow))
	case list.Sandboxed:
		return say("allow", "it runs in the conversation's sandbox")
	}
	return ""
}

// ConverseModel is the model conversations run on by default.
func (c *Claude) ConverseModel() string { return ClaudeConverseModel }
