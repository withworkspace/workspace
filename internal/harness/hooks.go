package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Hooks is a harness whose agent can report its activity to ws through its
// own hooks, installed for one session when ws launches it. ws learns that
// crew is working (you sent it a message), waiting (it finished its turn),
// or stopped for permission, rather than guessing from its screen.
type Hooks interface {
	// HookArgs are arguments, placed right after the program, that make the
	// session run cmd followed by one of the Hook* events below. dir is a
	// folder the harness may write its hook setup into.
	HookArgs(cmd []string, dir string) ([]string, error)
}

// The events ws's hook command understands, whichever agent sends them.
const (
	HookStart  = "start"  // a session started (its id in the payload)
	HookPrompt = "prompt" // it was sent a message: it's working
	HookReply  = "reply"  // its turn's final message
	HookStop   = "stop"   // it finished its turn: it's waiting, for you or on work it left running
	HookNotify = "notify" // it raised a notification (waiting, permission)
	// It put a question to you with its question tool (its choices in the
	// payload), and you answered it.
	HookQuestion = "question"
	HookAnswered = "answered"
)

// hookCommand is cmd and the event, quoted for the shell that runs hooks.
func hookCommand(cmd []string, event string) string {
	parts := make([]string, 0, len(cmd)+1)
	for _, c := range append(append([]string{}, cmd...), event) {
		parts = append(parts, shellQuote(c))
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	if s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !(r == '/' || r == '.' || r == '-' || r == '_' || r == ':' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z')
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// HookArgs installs ws's hooks in this Claude Code session alone, through
// --settings (an inline JSON string): they add to your own hooks.
//
// VERIFIED (claude 2.1.318): SessionStart (session_id), UserPromptSubmit
// (also for the opening prompt given as the argument), Stop
// (last_assistant_message is the turn's final message), and Notification
// (notification_type idle_prompt after about a minute's wait, or a
// permission request) all fire, with the session's environment (WS_MISSION,
// WS_CREW) and the event as JSON on stdin.
//
// Its question tool, AskUserQuestion, waits for you the way a permission
// prompt does, and its Notification says only that Claude needs your
// permission. So ws also hooks the tool itself (PreToolUse and PostToolUse,
// matched by name): before it runs, tool_input.questions has the question
// and its options' labels; after, you've answered. (From the tool's input
// schema, not yet seen firing from a crew session.)
func (c *Claude) HookArgs(cmd []string, dir string) ([]string, error) {
	entry := func(event string) []map[string]any {
		return []map[string]any{{"hooks": []map[string]string{{"type": "command", "command": hookCommand(cmd, event)}}}}
	}
	tool := func(name, event string) []map[string]any {
		e := entry(event)
		e[0]["matcher"] = name
		return e
	}
	settings, err := json.Marshal(map[string]any{"hooks": map[string]any{
		"SessionStart":     entry(HookStart),
		"UserPromptSubmit": entry(HookPrompt),
		"Stop":             entry(HookStop),
		"Notification":     entry(HookNotify),
		"PreToolUse":       tool("AskUserQuestion", HookQuestion),
		"PostToolUse":      tool("AskUserQuestion", HookAnswered),
	}})
	if err != nil {
		return nil, err
	}
	return []string{"--settings", string(settings)}, nil
}

// PendingAtStop reads what the agent left running from Stop's
// background_tasks: each task still running is work Claude Code will
// wake the agent for. It doesn't say the task's pid.
//
// VERIFIED (claude 2.1.322, headless, hooks logging their input): Stop has
// background_tasks, the tasks the agent started with run_in_background,
// each {id, type: "shell", status: "running", description, command}, and
// session_crons. A task that has finished is gone from the list, not
// listed as finished. When a task finishes, Claude Code wakes the agent
// itself: UserPromptSubmit fires with a prompt that starts
// <task-notification> (its task id, status and summary inside). Work the
// agent detached itself (nohup … & in an ordinary Bash call) isn't listed:
// Claude Code doesn't know it's running. Two tasks that finish together
// wake the agent one after the other, and the first wake-up's Stop already
// lists neither. The same in crew sessions ws launched (haiku, in a
// throwaway workspace): the task was on the crew, the wake-up's prompt hook
// fired, and a done the crew reported before it was kept.
//
// With no background_tasks (an older Claude Code), or one it can't read,
// the answer is unknown.
func (c *Claude) PendingAtStop(p map[string]any) Pending {
	tasks, ok := p["background_tasks"].([]any)
	if !ok {
		return Pending{}
	}
	out := Pending{Known: true}
	for _, t := range tasks {
		task, ok := t.(map[string]any)
		if !ok {
			return Pending{}
		}
		status, ok := task["status"].(string)
		if !ok {
			return Pending{}
		}
		if status != "running" {
			continue
		}
		what := ""
		for _, k := range []string{"command", "description", "id"} {
			if s, _ := task[k].(string); strings.TrimSpace(s) != "" {
				what = strings.TrimSpace(s)
				break
			}
		}
		if what == "" {
			what = "a background task"
		}
		out.Work = append(out.Work, Work{What: what, From: FromHarness, Wakes: WakeYes})
	}
	return out
}

// WakeUp is a prompt that is Claude Code waking the agent because a
// background task finished (see PendingAtStop), not a message.
func (c *Claude) WakeUp(p map[string]any) bool {
	prompt, _ := p["prompt"].(string)
	return strings.HasPrefix(strings.TrimSpace(prompt), "<task-notification>")
}

// Shells: a command still running in a shell Claude Code ran is one of
// the tasks Stop's background_tasks lists, and the shell gives it its pid,
// so ws sees it end (and Claude Code then has a minute to wake the agent,
// mission.WakeGrace). Any other shell of Claude Code's at a Stop isn't
// the agent's work: another Stop hook running beside ws's, say. Without
// background_tasks, each such shell is work that ws can't tell whether
// Claude Code wakes the agent for.
//
// VERIFIED (claude 2.1.322): each Bash command runs in a shell that is
// claude's direct child, `/bin/zsh -c source <snapshot> … && eval '<the
// command>' < /dev/null && pwd -P >| <file>`, its quotes written '"'"'. A
// run_in_background task's shell lives as long as the task, with the
// command its child. Work the agent detached from its shell (nohup … &)
// is re-parented to launchd when the shell exits.
func (c *Claude) Shells(own Pending) (Wake, bool) { return WakeUnknown, own.Known }

// HookArgs installs ws's hooks in this Cursor session alone, as a plugin
// written into dir and loaded with --plugin-dir: they add to your own hooks.
//
// VERIFIED (agent 2026.11.05): a plugin is a folder with
// .cursor-plugin/plugin.json and hooks/hooks.json; sessionStart
// (conversation_id), beforeSubmitPrompt and stop (status, transcript_path)
// fire from it with the session's environment and the event as JSON on
// stdin. The turn's final message is the last assistant entry in the
// transcript (JSON lines), read at stop: afterAgentResponse fires for every
// message of a turn, and hooks finish out of order, so it can't say which
// was last. Cursor has no notification for a permission prompt.
//
// Its question tool, AskQuestion, waits for you as Claude Code's does, and
// ws hooks it the same way: preToolUse and postToolUse (and
// postToolUseFailure, for a question skipped), matched on tool_name with a
// regular expression. From the CLI's code (agent 2026.11.05): tool_input
// is a title and questions, each with a prompt and options with a label
// (AskQuestionArgs). The tool is offered on some models only, not the free
// ones (auto on a trial), so it hasn't been seen firing: tool_name comes
// from Cursor's server and is unseen, hence a matcher for the spellings the
// code uses (AskQuestion, askQuestion, ask_question).
//
// VERIFIED: Cursor runs a plugin's hooks for an event only when one of your
// own hooks files has a hook for that event too, and one whose command is a
// path (a bare "true" doesn't count); sessionStart is the exception. So
// without entries for beforeSubmitPrompt and stop in ~/.cursor/hooks.json,
// only sessionStart reaches ws. EnableCursorHooks adds entries that do
// nothing (ws doctor --fix).
func (c *Cursor) HookArgs(cmd []string, dir string) ([]string, error) {
	hooks := map[string][]map[string]string{}
	for event, ours := range map[string]string{
		"sessionStart":       HookStart,
		"beforeSubmitPrompt": HookPrompt,
		"stop":               HookStop,
	} {
		hooks[event] = []map[string]string{{"command": hookCommand(cmd, ours)}}
	}
	for event, ours := range cursorToolEvents {
		hooks[event] = []map[string]string{{"command": hookCommand(cmd, ours), "matcher": cursorQuestionMatcher}}
	}
	files := map[string]any{
		filepath.Join(".cursor-plugin", "plugin.json"): map[string]string{
			"name": "ws-crew-hooks", "version": "1.0.0",
			"description": "Tells ws (the workspace tool) what this crew is doing.",
		},
		filepath.Join("hooks", "hooks.json"): map[string]any{"version": 1, "hooks": hooks},
	}
	for name, v := range files {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, err
		}
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("writing cursor hooks: %w", err)
		}
		// Atomically: crew launching at once write the same files, and a
		// session starting must never read half of one.
		if err := writeAtomically(path, append(data, '\n')); err != nil {
			return nil, fmt.Errorf("writing cursor hooks: %w", err)
		}
	}
	return []string{"--plugin-dir", dir}, nil
}

// writeAtomically writes path through a temporary file of its own, renamed
// into place.
func writeAtomically(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	cerr := f.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(f.Name(), 0o644)
	}
	if werr == nil {
		werr = os.Rename(f.Name(), path)
	}
	if werr != nil {
		os.Remove(f.Name())
	}
	return werr
}

// cursorTurnEvents are the events Cursor runs ws's plugin hooks for only if
// ~/.cursor/hooks.json has a hook for them too.
var cursorTurnEvents = []string{"beforeSubmitPrompt", "stop"}

// cursorToolEvents are the tool events ws hooks for Cursor's question tool,
// with what each means to ws; they need a hook in ~/.cursor/hooks.json too,
// matched to the question tool alone so it doesn't run for every tool.
var cursorToolEvents = map[string]string{
	"preToolUse":         HookQuestion,
	"postToolUse":        HookAnswered,
	"postToolUseFailure": HookAnswered, // skipped, or refused
}

// cursorQuestionMatcher matches the question tool's name, however Cursor
// spells it (a regular expression, as Cursor's matchers are).
const cursorQuestionMatcher = "[Aa]sk_?[Qq]uestion"

// cursorNoopCommand is a hook that does nothing, by path.
const cursorNoopCommand = "/usr/bin/true"

// CursorUserHooksPath is where Cursor reads your own hooks.
func CursorUserHooksPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cursor", "hooks.json"), nil
}

// CursorHooksMissing lists the turn events ~/.cursor/hooks.json has no
// hook for (by path), so Cursor won't run ws's plugin hooks for them.
func CursorHooksMissing() ([]string, error) {
	path, err := CursorUserHooksPath()
	if err != nil {
		return nil, err
	}
	doc, err := readCursorHooks(path)
	if err != nil {
		return nil, err
	}
	hooks, _ := doc["hooks"].(map[string]any)
	var missing []string
	for _, event := range cursorTurnEvents {
		if !hasPathHook(hooks[event], "") {
			missing = append(missing, event)
		}
	}
	for _, event := range cursorToolEventNames() {
		if !hasPathHook(hooks[event], cursorQuestionMatcher) {
			missing = append(missing, event)
		}
	}
	return missing, nil
}

// cursorToolEventNames are cursorToolEvents' names, in order.
func cursorToolEventNames() []string {
	names := make([]string, 0, len(cursorToolEvents))
	for e := range cursorToolEvents {
		names = append(names, e)
	}
	sort.Strings(names)
	return names
}

// hasPathHook reports whether entries have a hook whose command is a path,
// for any tool or (with matcher) one matched as ws's is.
func hasPathHook(entries any, matcher string) bool {
	list, _ := entries.([]any)
	for _, e := range list {
		m, _ := e.(map[string]any)
		cmd, _ := m["command"].(string)
		got, _ := m["matcher"].(string)
		if c := strings.TrimSpace(cmd); (strings.HasPrefix(c, "/") || strings.HasPrefix(c, "~/")) && (got == "" || got == "*" || got == matcher) {
			return true
		}
	}
	return false
}

// readCursorHooks reads a hooks file as generic JSON (keeping whatever it
// holds), or an empty one if there's none.
func readCursorHooks(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{"version": 1, "hooks": map[string]any{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s isn't valid JSON: %w", path, err)
	}
	if _, ok := doc["hooks"].(map[string]any); !ok {
		doc["hooks"] = map[string]any{}
	}
	return doc, nil
}

// EnableCursorHooks adds a do-nothing hook to ~/.cursor/hooks.json for each
// turn event it has none for, and for the question tool's events (matched
// to it alone), so Cursor runs ws's per-crew plugin hooks.
// Everything already in the file stays.
func EnableCursorHooks() error {
	path, err := CursorUserHooksPath()
	if err != nil {
		return err
	}
	doc, err := readCursorHooks(path)
	if err != nil {
		return err
	}
	hooks := doc["hooks"].(map[string]any)
	for _, event := range cursorTurnEvents {
		if !hasPathHook(hooks[event], "") {
			list, _ := hooks[event].([]any)
			hooks[event] = append(list, map[string]any{"command": cursorNoopCommand})
		}
	}
	for _, event := range cursorToolEventNames() {
		if !hasPathHook(hooks[event], cursorQuestionMatcher) {
			list, _ := hooks[event].([]any)
			hooks[event] = append(list, map[string]any{"command": cursorNoopCommand, "matcher": cursorQuestionMatcher})
		}
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeAtomically(path, append(data, '\n'))
}
