package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrSessionGone is the answer for resuming a conversation whose agent
// session isn't there any more: start again, from what ws keeps of it.
var ErrSessionGone = errors.New("its session is gone")

// A conversation's agent changes nothing of the workspace's: each harness
// that can hold one runs it headless, resuming its session per turn,
// limited to reading files and the shell commands below. Each says it its
// own way: Claude Code's --allowedTools, Cursor's permissions in a
// .cursor/cli.json, pi's tool_call hook in an extension ws writes.
//
// Where the harness has a sandbox for its shell (a Sandboxer: Claude
// Code), a run that asks for it (ConverseRun.Sandbox) runs anything there,
// and the list is only what leaves the sandbox: ws, which keeps the
// workspace's records, and gh, which needs your login. Where it's trusted
// to run things with none (a Truster: Cursor over ACP), the list is the
// ws and gh it may run, and anything else runs (ConversePolicy).

// converseCommands are the shell commands a conversation's agent may run,
// each the start of a command run alone (nothing chained): reading, ws
// show and its kin (an artifact's versions printed among them, and
// checked out into a folder it may write in, to work on), and for a
// thread, suggesting an edit (which the user accepts or not); for a talk,
// the missions, GitHub's views, read-only git in each of ReadPaths,
// proposing (which the user confirms, or not) a decision, a mission, an
// edit to the brief or the plan, crew, a replan by the mission's planner,
// a review comment, a document of one of the workspace's kinds (or an
// edit to one), or an artifact (or a version of one), and, where it may
// (Lift), lifting a capsule for its conversation, or docking one on a
// branch that's on the remote (a pull request's). Each proposal is the
// --propose form of the verb that would do it, with --propose first: the
// verb alone isn't allowed, and refuses a conversation's agent besides.
// Every conversation also runs the plugins' commands their plugins marked
// for conversations (Plugins), which change things as they run: the
// plugin's call, made for what the user watches change and can undo.
func converseCommands(r ConverseRun) []string {
	var out []string
	for _, ws := range []string{"ws", "workspace"} {
		out = append(out, ws+" show", ws+" review show", ws+" thread list", ws+" status", ws+" rg", ws+" documents", ws+" find", ws+" artifact show", ws+" artifact list", ws+" artifact checkout", ws+" artifact url")
		if r.Talk {
			out = append(out, ws+" mission list", ws+" conversation list", ws+" mission decide --propose", ws+" mission new --propose",
				ws+" mission brief --propose", ws+" mission plan --propose", ws+" mission plan --format", ws+" mission dispatch --propose", ws+" mission replan --propose", ws+" pr comment --propose",
				ws+" doc add --propose", ws+" doc amend --propose", ws+" artifact add --propose", ws+" conversation withdraw", ws+" propose",
				ws+" automation add --propose", ws+" automation kinds", ws+" automation list", ws+" automation runs", ws+" events")
			if r.Lift {
				out = append(out, ws+" lift", ws+" dock")
			}
		} else {
			out = append(out, ws+" thread suggest")
		}
		for _, c := range r.Plugins {
			out = append(out, ws+" "+c)
		}
	}
	out = append(out, "pwd", "ls", "cat", "head", "tail", "wc")
	if r.Talk {
		out = append(out, talkGH...)
		out = append(out, gitCommands(r.ReadPaths)...)
	}
	return out
}

// runsFrom tells a conversation's agent that runs things, run from ran
// (a folder of ws's own, or of its own) that the folder it works on is
// dir, and that it writes only where it's told: as cursorWorksIn, for one
// that writes.
func runsFrom(ran, dir string) string {
	if dir == "" || dir == ran {
		return ""
	}
	return "(ws runs you from " + ran + ", but the folder you work on is " + dir +
		": where your instructions say \"here\" or \"you're in\", or name a path that isn't in full (repos/<repo>/…), they mean " + dir +
		". Give paths in full when you read or write a file, and as the folder to run a shell command in. Write files only where your instructions say you may.)\n\n"
}

// talkGH is the gh a conversation's agent may run: reading GitHub, as you.
// (Not gh api, which writes as readily as it reads.)
var talkGH = []string{"gh pr view", "gh pr list", "gh pr checks", "gh pr diff", "gh issue view", "gh issue list",
	"gh run list", "gh run view", "gh workflow list", "gh workflow view", "gh release list", "gh release view", "gh repo view", "gh search"}

// gitCommands are git -C <path> <sub> for each path and read-only
// subcommand (TalkGit).
//
// VERIFIED: a * before the git subcommand (Bash(git -C * log *)) would also
// match options put in its place, and git -c / --exec-path run arbitrary
// commands. With the path spelled out, what's allowed only goes on after
// the subcommand, where -c can't go.
func gitCommands(paths []string) []string {
	var out []string
	for _, p := range paths {
		if p == "" || strings.ContainsAny(p, " \t\n\"'()*") {
			continue // can't be written into a rule exactly
		}
		for _, sub := range TalkGit {
			out = append(out, "git -C "+p+" "+sub)
		}
	}
	return out
}

// CursorConverseModel is what Cursor runs conversations on when none is
// configured: a capable model, as Claude Code's is.
const CursorConverseModel = "claude-sonnet-5-5-medium"

func (c *Cursor) ConverseModel() string { return CursorConverseModel }

// Converse runs one turn of a conversation headless, read-only.
//
// VERIFIED (cursor-agent 2026.11.05): -p --output-format stream-json
// prints a line per event (system init with session_id, tool_call
// started/completed, each assistant message, then the result), and
// --resume <chatId> continues that chat headless, from any folder.
// --mode ask is read-only (no edit or write tools), but runs no ws
// commands where Cursor's sandbox is on (see Orchestrate), so ws doesn't
// use it: the default (agent) mode can edit. Without --force, a
// shell command runs only if the permissions allow it, else it comes back
// "rejected"; a .cursor/cli.json in the folder it runs in adds to them,
// and Shell(git log) allows git log … but not git status. It reads files
// outside that folder. So ws runs it in a folder of its own (Scratch),
// whose cli.json allows converseCommands: one folder per list, as a
// thread and a talk may run at once, allowed different things.
// The prompt goes on stdin, which -p reads when no prompt is given
// (VERIFIED: with none there either, "No prompt provided for print mode").
// VERIFIED (cursor-agent 2026.11.22, a Mac with an endpoint-security agent): with a
// long command line it's killed as it starts, "signal: killed" after
// ~100ms with no output and nothing in the system log: 14 of 15 starts
// with ~19 KB of filler as its prompt argument, 15 of 15 with a real ws
// prompt, none of 15 with that prompt on stdin.
// VERIFIED: --resume of a chat that isn't there starts an empty one with
// that id, saying nothing; chats are kept in ~/.cursor/chats/<hash>/<id>,
// so ws looks for it first.
func (c *Cursor) Converse(r ConverseRun) (Launch, error) {
	bin, err := c.bin()
	if err != nil {
		return Launch{}, err
	}
	if r.Scratch == "" {
		return Launch{}, fmt.Errorf("cursor: no folder to hold a conversation's permissions in")
	}
	if r.Session != "" && !c.chatKept(r.Session) {
		return Launch{}, fmt.Errorf("%w: Cursor has no chat %s", ErrSessionGone, r.Session)
	}
	dir, err := writeCursorPermissions(r.Scratch, converseCommands(r))
	if err != nil {
		return Launch{}, err
	}
	args := []string{bin, "-p", cursorTrustFlag, "--output-format", "stream-json"}
	if r.Model != "" {
		args = append(args, "--model", r.Model)
	}
	if r.Session != "" {
		args = append(args, "--resume", r.Session)
	}
	return Launch{Args: args, Stdin: r.Prompt, Dir: dir, Stream: StreamCursor, Conversation: r.Session}, nil
}

// writeCursorPermissions writes a .cursor/cli.json allowing the shell
// commands given (each a command's start), in a folder of ws's own for
// that list, and says the folder: Cursor run there may run them alone.
func writeCursorPermissions(scratch string, cmds []string) (string, error) {
	var allow []string
	for _, cmd := range cmds {
		allow = append(allow, "Shell("+cmd+")")
	}
	perms, _ := json.MarshalIndent(map[string]any{"permissions": map[string]any{"allow": allow, "deny": []string{}}}, "", "  ")
	sum := sha256.Sum256(perms)
	dir := filepath.Join(scratch, "cursor", hex.EncodeToString(sum[:])[:12])
	if err := os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755); err != nil {
		return "", err
	}
	return dir, writeAtomically(filepath.Join(dir, ".cursor", "cli.json"), perms)
}

// orchestratorCommands are the shell commands the orchestrator may run,
// as Claude Code's are (claudeOrchestratorTools): ws's mission verbs,
// lift, status, show and find, GitHub's PR views, read-only git (where it is,
// and by the full path of each capsule and .ground), and reading.
func orchestratorCommands(r OrchestratorRun) []string {
	var out []string
	for _, ws := range []string{"ws", "workspace"} {
		out = append(out, ws+" mission", ws+" lift", ws+" status", ws+" show", ws+" review show", ws+" find")
	}
	out = append(out, "git diff", "git log", "git show", "git status", "gh pr view", "gh pr checks", "gh pr list",
		"cd", "pwd", "ls", "cat", "head", "tail", "wc", "echo")
	for _, p := range append(append([]string(nil), r.Capsules...), r.Grounds...) {
		if p == "" || strings.ContainsAny(p, " \t\n\"'()*") {
			continue
		}
		for _, sub := range []string{"diff", "log", "show", "status"} {
			out = append(out, "git -C "+p+" "+sub)
		}
	}
	return out
}

// chatKept is whether Cursor still has a chat. Where it can't tell (its
// chats aren't where they were), it says yes: resuming is no worse than
// before.
func (c *Cursor) chatKept(id string) bool {
	dir := c.chats
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return true
		}
		dir = filepath.Join(home, ".cursor", "chats")
	}
	if _, err := os.Stat(dir); err != nil {
		return true
	}
	found, _ := filepath.Glob(filepath.Join(dir, "*", id))
	return len(found) > 0
}

// ConverseModel is "": pi runs on whichever providers you've logged in to,
// so ws can't name a model sure to exist. Conversations run on your pi
// default unless [conversation] model names one.
func (p *Pi) ConverseModel() string { return "" }

// piConverseTools are the built-in tools a conversation gets: reading, and
// the shell, which ws's extension limits to converseCommands.
const piConverseTools = "read,bash,grep,find,ls"

// piRunsTools are those of a conversation that runs things: writing and
// editing files too, where ws's guard allows (piAskGuardSource).
const piRunsTools = "read,bash,grep,find,ls,write,edit"

// Trusted: a conversation's agent on pi runs things, with no sandbox (pi
// has none), ws's guard deciding each command and write as it does
// Claude Code's (GuardAnswer, by `ws conversation allowed`): any command
// but a ws or gh one that isn't allowed, and writes in its folders.
func (p *Pi) Trusted() bool { return true }

// writePiAskGuard writes the extension that has pi ask ws's guard (guard:
// the command, then its list's file) before each shell command, write and
// edit, named by what it runs.
func writePiAskGuard(dir string, guard []string) (string, error) {
	cmd, _ := json.Marshal(guard)
	src := strings.Replace(piAskGuardSource, "$GUARD", string(cmd), 1)
	sum := sha256.Sum256([]byte(src))
	path := filepath.Join(dir, "ws-converse-ask-"+hex.EncodeToString(sum[:])[:12]+".ts")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return path, writeAtomically(path, []byte(src))
}

// piAskGuardSource asks ws's guard as Claude Code's hook does: the event
// as Claude Code would send it (Bash's command; Write's and Edit's
// file_path, in full), and its answer's permissionDecision. Anything but
// allow is blocked, with ws's reason, which the agent's told. Reading is
// allowed; any other tool, refused.
const piAskGuardSource = `// Written by ws: a conversation's agent runs things, ws's guard deciding
// each command and each file written.
import { execFileSync } from "node:child_process";
import { homedir } from "node:os";
import { isAbsolute, resolve } from "node:path";

const guard: string[] = $GUARD;
const reading = ["read", "grep", "find", "ls"];

function ask(tool: string, input: Record<string, string>): { allowed: boolean; reason: string } {
  try {
    const out = execFileSync(guard[0], guard.slice(1), {
      input: JSON.stringify({ tool_name: tool, tool_input: input }),
      encoding: "utf8",
      timeout: 30000,
    });
    const said = out.trim() ? JSON.parse(out).hookSpecificOutput ?? {} : {};
    return { allowed: said.permissionDecision === "allow", reason: said.permissionDecisionReason ?? "" };
  } catch (e) {
    return { allowed: false, reason: "ws couldn't say whether it may: " + String(e) };
  }
}

function full(path: string): string {
  if (path === "~" || path.startsWith("~/")) path = homedir() + path.slice(1);
  return isAbsolute(path) ? path : resolve(process.cwd(), path);
}

export default function (pi: any) {
  pi.on("tool_call", async (event: any) => {
    const name = event.toolName;
    if (reading.includes(name)) return;
    let said;
    if (name === "bash") {
      said = ask("Bash", { command: String(event.input?.command ?? "") });
    } else if (name === "write" || name === "edit") {
      said = ask(name === "write" ? "Write" : "Edit", { file_path: full(String(event.input?.path ?? "")) });
    } else {
      return { block: true, reason: "Refused: that tool isn't one a conversation uses here." };
    }
    if (!said.allowed) {
      return { block: true, reason: said.reason || "Refused: ws doesn't allow that here." };
    }
  });
}
`

// Converse runs one turn of a conversation headless, read-only.
//
// VERIFIED (pi 1.5.0): -p --mode json prints the session header (its id),
// then events: tool_execution_start/end, message_end per message.
// --session-id <id> starts that session; --session <id> continues it,
// headless, and fails if it's gone (--session-id would quietly start an
// empty one). --tools limits it to the tools named, and an
// extension's tool_call handler can block a call ({block, reason}), which
// comes back to the model as the tool's error. --no-extensions and
// --no-mcp leave out tools ws didn't name (an -e extension still loads).
// It reads stdin while it's open: ws runs it with none.
func (p *Pi) Converse(r ConverseRun) (Launch, error) {
	bin, err := p.bin()
	if err != nil {
		return Launch{}, err
	}
	if r.Scratch == "" {
		return Launch{}, fmt.Errorf("pi: no folder to keep ws's extension in")
	}
	tools := piConverseTools
	var guard string
	if r.Sandbox && !r.Alone && len(r.Guard) > 0 {
		// One that runs things: ws's guard decides each command and write
		// (piAskGuard), as it does Claude Code's.
		list, err := writeGuardList(filepath.Join(r.Scratch, "pi"), converseCommands(r), true)
		if err != nil {
			return Launch{}, err
		}
		if guard, err = writePiAskGuard(filepath.Join(r.Scratch, "pi"), append(append([]string{}, r.Guard...), list)); err != nil {
			return Launch{}, err
		}
		tools = piRunsTools
	} else if guard, err = writePiGuard(filepath.Join(r.Scratch, "pi"), converseCommands(r)); err != nil {
		return Launch{}, err
	}
	session, flag := r.Session, "--session"
	if session == "" {
		if session, err = p.newID(); err != nil {
			return Launch{}, err
		}
		flag = "--session-id"
	}
	args := []string{bin, "-p", "--mode", "json", flag, session, "--no-extensions", "--no-mcp", "-e", guard, "--tools", tools}
	if r.Model != "" {
		args = append(args, "--model", r.Model)
	}
	return Launch{Args: append(args, "--", r.Prompt), Dir: r.Dir, Stream: StreamPi, Conversation: session}, nil
}

// writePiGuard writes the extension that limits pi's tools to reading,
// and its shell to the commands allowed, each run alone; what one reads on
// stdin (a suggestion, a brief) may come as a quoted heredoc. Named by what it allows, so runs
// allowed different things don't overwrite each other's.
func writePiGuard(dir string, allow []string) (string, error) {
	list, _ := json.Marshal(allow)
	src := strings.Replace(piGuardSource, "$ALLOW", string(list), 1)
	sum := sha256.Sum256([]byte(src))
	path := filepath.Join(dir, "ws-converse-guard-"+hex.EncodeToString(sum[:])[:12]+".ts")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return path, writeAtomically(path, []byte(src))
}

const piGuardSource = `// Written by ws: a conversation's agent reads, and runs only the shell
// commands ws allows, one at a time.
const allow: string[] = $ALLOW;
const reading = ["read", "grep", "find", "ls"];

function one(cmd: string): boolean {
  if (/[;&|$<>` + "`" + `\\\n]/.test(cmd)) return false;
  return allow.some((a) => cmd === a || cmd.startsWith(a + " "));
}

// An allowed command reading stdin from a heredoc (a suggestion, a
// brief): cmd <<'EOF' … EOF, the delimiter quoted (so nothing in it runs)
// and nothing after it.
function heredoc(cmd: string): boolean {
  const lines = cmd.split("\n");
  const m = /^(.*) <<'([A-Za-z_]+)'$/.exec(lines[0]);
  if (!m || !one(m[1])) return false;
  const end = lines.findIndex((l, i) => i > 0 && l === m[2]);
  return end === lines.length - 1;
}

function allowed(cmd: string): boolean {
  cmd = cmd.trim();
  return cmd.includes("\n") ? heredoc(cmd) : one(cmd);
}

export default function (pi: any) {
  pi.on("tool_call", async (event: any) => {
    if (event.toolName === "bash") {
      const cmd = String(event.input?.command ?? "");
      if (!allowed(cmd)) {
        return { block: true, reason: "Refused: that command isn't allowed here. Run one command at a time, nothing chained, and only those you were told you may." };
      }
      return;
    }
    if (!reading.includes(event.toolName)) {
      return { block: true, reason: "Refused: you may only read here." };
    }
  });
}
`
