package harness

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func fakeCursor(found map[string]string, chatOut string, chatErr error) *Cursor {
	return &Cursor{
		lookPath: func(name string) (string, error) {
			if p, ok := found[name]; ok {
				return p, nil
			}
			return "", errors.New("not found")
		},
		run: func(ctx context.Context, bin string, args ...string) ([]byte, error) {
			return []byte(chatOut), chatErr
		},
	}
}

func TestCursor_BinaryLookup(t *testing.T) {
	t.Setenv(EnvCursorBin, "")
	if err := fakeCursor(nil, "", nil).Available(); err == nil || !strings.Contains(err.Error(), EnvCursorBin) {
		t.Errorf("err = %v", err)
	}
	c := fakeCursor(map[string]string{"agent": "/bin/agent"}, "", nil)
	if b, _ := c.bin(); b != "/bin/agent" {
		t.Errorf("fallback bin = %q", b)
	}
	c = fakeCursor(map[string]string{"cursor-agent": "/bin/ca", "agent": "/bin/agent"}, "", nil)
	if b, _ := c.bin(); b != "/bin/ca" {
		t.Errorf("preferred bin = %q", b)
	}
	t.Setenv(EnvCursorBin, "/opt/my-agent")
	c = fakeCursor(map[string]string{"/opt/my-agent": "/opt/my-agent"}, "", nil)
	if b, _ := c.bin(); b != "/opt/my-agent" {
		t.Errorf("override bin = %q", b)
	}
}

func TestCursor_StartWithChatID(t *testing.T) {
	t.Setenv(EnvCursorBin, "")
	c := fakeCursor(map[string]string{"cursor-agent": "/bin/ca"}, "Created chat\n3f2a9c1e-7b4d-4e2a-9f00-1234abcd\n", nil)
	l, err := c.Start(Brief{Slug: "auth", Title: "Auth", Brief: "Fix it.", Dir: "/w/fe/auth", Prompt: "Read ws show -f llm. Fix it."})
	if err != nil {
		t.Fatal(err)
	}
	if l.Conversation != "3f2a9c1e-7b4d-4e2a-9f00-1234abcd" {
		t.Errorf("conversation = %q", l.Conversation)
	}
	// --trust: a new capsule's trust dialog would swallow the opening prompt.
	if strings.Join(l.Args, " ") != "/bin/ca --trust --resume "+l.Conversation {
		t.Errorf("args = %q (trusted, and the prompt must not be an argument)", l.Args)
	}
	if !strings.Contains(l.Prompt, "ws show -f llm") || !strings.Contains(l.Prompt, "Fix it.") {
		t.Errorf("prompt = %q", l.Prompt)
	}
	if l.Dir != "/w/fe/auth" {
		t.Errorf("dir = %q", l.Dir)
	}
	if len(l.Notes) != 1 || !strings.HasPrefix(l.Notes[0], "ran /bin/ca create-chat: chat 3f2a9c1e-7b4d-4e2a-9f00-1234abcd (") {
		t.Errorf("notes = %q", l.Notes)
	}
}

func TestCursor_StartWithoutChatID(t *testing.T) {
	t.Setenv(EnvCursorBin, "")
	for name, c := range map[string]*Cursor{
		"create-chat fails":     fakeCursor(map[string]string{"cursor-agent": "/bin/ca"}, "", errors.New("unknown command")),
		"create-chat is chatty": fakeCursor(map[string]string{"cursor-agent": "/bin/ca"}, "Usage: cursor-agent [options]", nil),
	} {
		l, err := c.Start(Brief{Slug: "x", Title: "X", Prompt: "Do X."})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if l.Conversation != "" || strings.Join(l.Args, " ") != "/bin/ca --trust" || l.Prompt == "" {
			t.Errorf("%s: launch = %+v", name, l)
		}
		// Why it can't be resumed goes in the exec log.
		if len(l.Notes) != 1 || !strings.Contains(l.Notes[0], "can't be resumed") {
			t.Errorf("%s: notes = %q", name, l.Notes)
		}
	}
}

func TestCursor_Resume(t *testing.T) {
	t.Setenv(EnvCursorBin, "")
	c := fakeCursor(map[string]string{"cursor-agent": "/bin/ca"}, "", nil)
	l, err := c.Resume("/w", "abc123")
	if err != nil || strings.Join(l.Args, " ") != "/bin/ca --trust --resume abc123" {
		t.Errorf("resume = %+v, %v", l, err)
	}
	if _, err := c.Resume("/w", ""); err == nil {
		t.Error("resume without a chat id should fail")
	}
}

func TestShell(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	h, err := Get("shell")
	if err != nil {
		t.Fatal(err)
	}
	l, _ := h.Start(Brief{Dir: "/w"})
	if strings.Join(l.Args, " ") != "/bin/sh -i" || l.Dir != "/w" {
		t.Errorf("launch = %+v", l)
	}
	if _, err := h.Resume("/w", "x"); !errors.Is(err, ErrNoResume) {
		t.Errorf("resume err = %v", err)
	}
}

func TestRegistry(t *testing.T) {
	if got := strings.Join(Names(), ","); got != "claude,codex,cursor,cursor-acp,opencode,pi,shell" {
		t.Errorf("names = %s", got)
	}
	// Codex, cursor-acp and OpenCode run crew too, over ACP.
	if got := strings.Join(CrewNames(), ","); got != "claude,codex,cursor,cursor-acp,opencode,pi,shell" {
		t.Errorf("crew names = %s", got)
	}
	if _, err := Get("nope"); err == nil {
		t.Error("unknown harness should error")
	}
}

func TestRoles_Crew(t *testing.T) {
	t.Setenv(EnvDefault, "")
	if p := (Roles{}).ForCrew(); p != (Pick{Harness: "cursor"}) {
		t.Errorf("default crew = %+v", p)
	}
	r := Roles{Crew: Pick{Harness: "claude", Model: "opus"}}
	if p := r.ForCrew(); p != (Pick{Harness: "claude", Model: "opus"}) {
		t.Errorf("configured crew = %+v", p)
	}
	// $WS_DECK_HARNESS wins, and the configured model stays with the
	// configured harness.
	t.Setenv(EnvDefault, "shell")
	if p := r.ForCrew(); p != (Pick{Harness: "shell"}) {
		t.Errorf("env crew = %+v", p)
	}
	t.Setenv(EnvDefault, "claude")
	if p := r.ForCrew(); p != (Pick{Harness: "claude", Model: "opus"}) {
		t.Errorf("env naming the configured harness keeps its model: %+v", p)
	}
}

// Crew may be launched on the default or on what [crew] allowed lists,
// each as it would run, and on nothing else.
func TestRoles_CrewPicks(t *testing.T) {
	t.Setenv(EnvDefault, "")
	pick := func(s string) Pick { h, m, _ := strings.Cut(s, ":"); return Pick{Harness: h, Model: m} }
	picks := func(r Roles) string {
		var out []string
		for _, p := range r.CrewPicks() {
			out = append(out, p.String())
		}
		return strings.Join(out, " ")
	}
	r := Roles{Crew: pick("claude:opus")}
	if got := picks(r); got != "claude:opus" {
		t.Errorf("nothing allowed: %s", got)
	}
	if !r.AllowsCrew(pick("claude:opus")) || !r.AllowsCrew(pick("claude")) || r.AllowsCrew(pick("claude:sonnet")) || r.AllowsCrew(pick("cursor")) {
		t.Error("with nothing allowed, only the default is")
	}

	// The default first; a harness alone takes the default's model on the
	// default's harness (so claude is claude:opus, said once), and its
	// agent's own on another; duplicates go.
	r.CrewAllowed = []Pick{pick("claude:sonnet"), pick("claude"), pick("cursor"), pick("cursor:gpt-5.5-high"), pick("claude:sonnet")}
	if got := picks(r); got != "claude:opus claude:sonnet cursor cursor:gpt-5.5-high" {
		t.Errorf("allowed: %s", got)
	}
	for _, p := range []string{"claude:opus", "claude", "claude:sonnet", "cursor", "cursor:gpt-5.5-high"} {
		if !r.AllowsCrew(pick(p)) {
			t.Errorf("%s isn't allowed", p)
		}
	}
	for _, p := range []string{"claude:haiku", "cursor:auto", "pi", "pi:sonnet", ":opus"} {
		if r.AllowsCrew(pick(p)) {
			t.Errorf("%s is allowed", p)
		}
	}

	// $WS_DECK_HARNESS makes another default, which is allowed in turn.
	t.Setenv(EnvDefault, "shell")
	if got := picks(r); got != "shell claude:sonnet claude cursor cursor:gpt-5.5-high" || !r.AllowsCrew(pick("shell")) {
		t.Errorf("env default: %s", got)
	}
}

func TestRoles_Briefing(t *testing.T) {
	t.Setenv(EnvDefault, "")
	crew := Pick{Harness: "cursor", Model: "claude-opus-5-5-high"}
	if p := (Roles{Crew: crew}).ForBriefing(); p != crew {
		t.Errorf("briefing follows crew: %+v", p)
	}
	if p := (Roles{Crew: crew, Briefing: Pick{Model: "gpt-5.5-high"}}).ForBriefing(); p != (Pick{Harness: "cursor", Model: "gpt-5.5-high"}) {
		t.Errorf("briefing model on crew's harness: %+v", p)
	}
	if p := (Roles{Crew: crew, Briefing: Pick{Harness: "claude"}}).ForBriefing(); p != (Pick{Harness: "claude"}) {
		t.Errorf("briefing harness of its own: %+v", p)
	}
}

func TestRoles_Orchestrator(t *testing.T) {
	t.Setenv(EnvDefault, "")
	pick, o, err := (Roles{}).ForOrchestrator()
	if err != nil || o.Name() != "cursor" || pick != (Pick{Harness: "cursor", Model: CursorOrchestratorModel}) {
		t.Errorf("default = %+v %v", pick, err)
	}
	// Follows crew's harness, with that harness's own model.
	pick, _, _ = (Roles{Crew: Pick{Harness: "claude", Model: "opus"}}).ForOrchestrator()
	if pick != (Pick{Harness: "claude", Model: ClaudeOrchestratorModel}) {
		t.Errorf("claude crew = %+v", pick)
	}
	// Configured outright, or just its model.
	pick, _, _ = (Roles{Crew: Pick{Harness: "claude"}, Orchestrator: Pick{Harness: "cursor", Model: "auto"}}).ForOrchestrator()
	if pick != (Pick{Harness: "cursor", Model: "auto"}) {
		t.Errorf("configured = %+v", pick)
	}
	pick, _, _ = (Roles{Orchestrator: Pick{Model: "auto"}}).ForOrchestrator()
	if pick != (Pick{Harness: "cursor", Model: "auto"}) {
		t.Errorf("model only = %+v", pick)
	}
	// The shell can't run headless: an explicit choice says so.
	if _, _, err := (Roles{Orchestrator: Pick{Harness: "shell"}}).ForOrchestrator(); err == nil || !strings.Contains(err.Error(), "can't run the headless orchestrator") {
		t.Errorf("shell orchestrator err = %v", err)
	}
}

func TestPick_ModelBelongsToItsHarness(t *testing.T) {
	p := Pick{Harness: "claude", Model: "opus"}
	if p.ModelFor("claude") != "opus" || p.ModelFor("cursor") != "" {
		t.Errorf("ModelFor: %q %q", p.ModelFor("claude"), p.ModelFor("cursor"))
	}
}

func fakeClaude(found map[string]string) *Claude {
	return &Claude{
		lookPath: func(name string) (string, error) {
			if p, ok := found[name]; ok {
				return p, nil
			}
			return "", errors.New("not found")
		},
		newID: func() (string, error) { return "1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed", nil },
	}
}

func TestClaude_Start(t *testing.T) {
	t.Setenv(EnvClaudeBin, "")
	c := fakeClaude(map[string]string{"claude": "/bin/claude"})
	l, err := c.Start(Brief{Slug: "auth", Title: "Auth", Brief: "Fix it.", Dir: "/w/fe/auth", Model: "opus", Prompt: "Read ws show -f llm. Fix it."})
	if err != nil {
		t.Fatal(err)
	}
	const id = "1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed"
	want := []string{"/bin/claude", "--session-id", id, "--name", "ws auth", "--model", "opus", "--"}
	if len(l.Args) != len(want)+1 || strings.Join(l.Args[:len(want)], " ") != strings.Join(want, " ") {
		t.Fatalf("args = %q", l.Args)
	}
	// The prompt is the argument, not typed in: typed, its Enter would
	// answer a trust dialog's default "No, exit".
	if prompt := l.Args[len(want)]; !strings.Contains(prompt, "ws show -f llm") || !strings.Contains(prompt, "Fix it.") || l.Prompt != "" {
		t.Errorf("prompt arg = %q, typed = %q", prompt, l.Prompt)
	}
	if l.Conversation != id || l.Dir != "/w/fe/auth" {
		t.Errorf("launch = %+v", l)
	}
	if len(l.Notes) != 2 || !strings.Contains(l.Notes[1], "opening prompt is its last argument") {
		t.Errorf("notes = %q", l.Notes)
	}

	l, _ = c.Start(Brief{Slug: "new-thing", Role: "briefing", Request: "fe issue 45", Prompt: "You are briefing a new mission: fe issue 45."})
	if strings.Join(l.Args[3:5], " ") != "--name ws new-thing · briefing" || strings.Contains(strings.Join(l.Args, " "), "--model") {
		t.Errorf("briefing args = %q", l.Args)
	}
	if !strings.Contains(l.Args[len(l.Args)-1], "You are briefing a new mission") {
		t.Errorf("briefing prompt = %q", l.Args[len(l.Args)-1])
	}
}

func TestClaude_ResumeAndAvailability(t *testing.T) {
	t.Setenv(EnvClaudeBin, "")
	c := fakeClaude(map[string]string{"claude": "/bin/claude"})
	l, err := c.Resume("/w", "abc")
	if err != nil || strings.Join(l.Args, " ") != "/bin/claude --resume abc" || l.Dir != "/w" || l.Conversation != "abc" {
		t.Errorf("resume = %+v, %v", l, err)
	}
	if _, err := c.Resume("/w", ""); err == nil {
		t.Error("resume without a session id should fail")
	}
	if err := fakeClaude(nil).Available(); err == nil || !strings.Contains(err.Error(), EnvClaudeBin) {
		t.Errorf("missing claude: %v", err)
	}
	t.Setenv(EnvClaudeBin, "/opt/claude-dev")
	if b, err := fakeClaude(map[string]string{"/opt/claude-dev": "/opt/claude-dev"}).bin(); err != nil || b != "/opt/claude-dev" {
		t.Errorf("override bin = %q, %v", b, err)
	}
}

// The orchestrator runs headless with only ws mission/lift and read-only
// tools allowed; dontAsk refuses everything else without a prompt.
func TestClaude_Orchestrate(t *testing.T) {
	t.Setenv(EnvClaudeBin, "")
	l, err := fakeClaude(map[string]string{"claude": "/bin/claude"}).Orchestrate(OrchestratorRun{
		Prompt: "Read the prompt file.", Dir: "/root", Model: "haiku",
		Capsules: []string{"/root/repos/api/fix", "/root/repos/odd path/x"},
		Grounds:  []string{"/root/repos/demo/.ground"},
	})
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(l.Args, " ")
	for _, want := range []string{
		"/bin/claude -p --output-format stream-json --verbose --permission-mode dontAsk --no-session-persistence --model haiku --allowedTools ",
		"Bash(ws mission *)", "Bash(ws lift *)", "Bash(ws show *)", "Bash(ws review show *)", "Bash(git diff *)", "Bash(gh pr checks *)",
		"Bash(git -C /root/repos/api/fix log *)", "Bash(git -C /root/repos/api/fix status *)",
		"Bash(git -C /root/repos/demo/.ground log *)", "Bash(git -C /root/repos/demo/.ground show *)",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("args missing %q: %s", want, args)
		}
	}
	// No * before a git subcommand (git -C * log * would also match
	// git -C x -c core.fsmonitor=<cmd> log), and no path that can't be
	// written into a rule exactly.
	for _, never := range []string{"--dangerously-skip-permissions", "bypassPermissions", "Bash(ws *)", "Edit", "Write", "git -C * ", "odd path"} {
		if strings.Contains(args, never) {
			t.Errorf("args shouldn't contain %q: %s", never, args)
		}
	}
	// -- ends --allowedTools' list, so the prompt isn't taken for a rule.
	if n := len(l.Args); l.Args[n-2] != "--" || l.Args[n-1] != "Read the prompt file." || l.Dir != "/root" {
		t.Errorf("tail = %q, dir %q", l.Args[n-2:], l.Dir)
	}
}

func TestCursor_ModelAndOrchestratorDefault(t *testing.T) {
	t.Setenv(EnvCursorBin, "")
	c := fakeCursor(map[string]string{"cursor-agent": "/bin/ca"}, "abc123def\n", nil)
	l, _ := c.Start(Brief{Slug: "x", Model: "gpt-5.5-high"})
	if strings.Join(l.Args, " ") != "/bin/ca --trust --resume abc123def --model gpt-5.5-high" {
		t.Errorf("args = %q", l.Args)
	}
	if c.OrchestratorModel() != CursorOrchestratorModel {
		t.Errorf("orchestrator model = %q", c.OrchestratorModel())
	}
}

// Cursor's orchestrator is limited as its conversations are: ask mode, no
// --force, and a cli.json of ws's allowing the orchestrator's commands.
func TestCursor_Orchestrate(t *testing.T) {
	c := fakeCursor(map[string]string{"cursor-agent": "/bin/ca"}, "", nil)
	scratch := t.TempDir()
	l, err := c.Orchestrate(OrchestratorRun{Prompt: "Read /w/p.md.", Dir: "/w", Model: "auto", Scratch: scratch, Grounds: []string{"/w/repos/api/.ground"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(l.Args, " "); got != "/bin/ca -p --trust --mode ask --output-format stream-json --model auto -- Read /w/p.md." || l.Stream != StreamCursor || filepath.Dir(l.Dir) != filepath.Join(scratch, "cursor") {
		t.Errorf("launch = %q in %s", got, l.Dir)
	}
	data, _ := os.ReadFile(filepath.Join(l.Dir, ".cursor", "cli.json"))
	for _, want := range []string{"Shell(ws mission)", "Shell(gh pr view)", "Shell(git -C /w/repos/api/.ground log)"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("permissions lack %q:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "--force") || strings.Contains(string(data), "Shell(rm") {
		t.Errorf("permissions:\n%s", data)
	}
}

// What the orchestrator may run is the same whichever harness runs it.
func TestOrchestratorCommands_MatchClaude(t *testing.T) {
	for _, cmd := range orchestratorCommands(OrchestratorRun{}) {
		if !slices.Contains(claudeOrchestratorTools, "Bash("+cmd+" *)") && !slices.Contains(claudeOrchestratorTools, "Bash("+cmd+")") {
			t.Errorf("Claude Code's orchestrator may not run %q", cmd)
		}
	}
}

func TestNewSessionID(t *testing.T) {
	a, err := newSessionID()
	b, _ := newSessionID()
	shape := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if err != nil || !shape.MatchString(a) || a == b {
		t.Errorf("ids %q %q, %v", a, b, err)
	}
}

// ws's hooks go into the one session: Claude Code through an inline
// --settings, Cursor through a plugin folder ws writes.
func TestHookArgs(t *testing.T) {
	cmd := []string{"/w/.ws/bin/ws", "mission", "hook", "--root", "/w s"}
	args, err := (&Claude{}).HookArgs(cmd, t.TempDir())
	if err != nil || len(args) != 2 || args[0] != "--settings" {
		t.Fatalf("claude args = %q, %v", args, err)
	}
	var settings struct {
		Hooks map[string][]struct {
			Matcher string
			Hooks   []struct{ Type, Command string }
		}
	}
	if err := json.Unmarshal([]byte(args[1]), &settings); err != nil {
		t.Fatal(err)
	}
	for event, ours := range map[string]string{"SessionStart": HookStart, "UserPromptSubmit": HookPrompt, "Stop": HookStop, "Notification": HookNotify, "PreToolUse": HookQuestion, "PostToolUse": HookAnswered} {
		h := settings.Hooks[event]
		if len(h) != 1 || h[0].Hooks[0].Type != "command" || h[0].Hooks[0].Command != "/w/.ws/bin/ws mission hook --root '/w s' "+ours {
			t.Errorf("claude %s hook = %+v", event, h)
		}
	}
	for _, event := range []string{"PreToolUse", "PostToolUse"} {
		if m := settings.Hooks[event][0].Matcher; m != "AskUserQuestion" {
			t.Errorf("claude %s matches %q, not just the question tool", event, m)
		}
	}

	dir := t.TempDir()
	args, err = (&Cursor{}).HookArgs(cmd, dir)
	if err != nil || strings.Join(args, " ") != "--plugin-dir "+dir {
		t.Fatalf("cursor args = %q, %v", args, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cursor-plugin", "plugin.json")); err != nil {
		t.Errorf("no plugin manifest: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "hooks", "hooks.json"))
	var hooks struct {
		Version int
		Hooks   map[string][]struct{ Command string }
	}
	json.Unmarshal(data, &hooks)
	if _, ok := hooks.Hooks["afterAgentResponse"]; ok {
		t.Error("cursor's afterAgentResponse fires for every message, out of order: the reply comes from the transcript at stop")
	}
	for event, ours := range map[string]string{"sessionStart": HookStart, "beforeSubmitPrompt": HookPrompt, "stop": HookStop} {
		if h := hooks.Hooks[event]; hooks.Version != 1 || len(h) != 1 || !strings.HasSuffix(h[0].Command, "'/w s' "+ours) {
			t.Errorf("cursor %s hook = %+v", event, h)
		}
	}
}

// Cursor runs ws's plugin hooks only for events your own hooks file has a
// hook (by path) for: the fix adds do-nothing ones, keeping what's there.
// Cursor's plugin hooks its question tool's events, matched to it by a
// pattern that covers the spellings Cursor's code uses.
func TestCursorHookArgs_QuestionTool(t *testing.T) {
	dir := t.TempDir()
	if _, err := (&Cursor{}).HookArgs([]string{"/w/ws", "mission", "hook"}, dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "hooks", "hooks.json"))
	var doc struct {
		Hooks map[string][]struct{ Command, Matcher string }
	}
	json.Unmarshal(data, &doc)
	for event, ours := range map[string]string{"preToolUse": HookQuestion, "postToolUse": HookAnswered, "postToolUseFailure": HookAnswered} {
		h := doc.Hooks[event]
		if len(h) != 1 || !strings.HasSuffix(h[0].Command, " "+ours) || h[0].Matcher != cursorQuestionMatcher {
			t.Errorf("%s = %+v", event, h)
		}
	}
	re := regexp.MustCompile(cursorQuestionMatcher)
	for _, name := range []string{"AskQuestion", "askQuestion", "ask_question"} {
		if !re.MatchString(name) {
			t.Errorf("the matcher misses %s", name)
		}
	}
	for _, name := range []string{"Shell", "Read", "Write", "Grep"} {
		if re.MatchString(name) {
			t.Errorf("the matcher catches %s", name)
		}
	}
}

func TestEnableCursorHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	missing, err := CursorHooksMissing()
	if err != nil || strings.Join(missing, ",") != "beforeSubmitPrompt,stop,postToolUse,postToolUseFailure,preToolUse" {
		t.Fatalf("no file: missing %v, %v", missing, err)
	}
	path := filepath.Join(home, ".cursor", "hooks.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(`{"version":1,"hooks":{"stop":[{"command":"/opt/mine/notify.sh"}],"beforeSubmitPrompt":[{"command":"true"}],"afterFileEdit":[{"command":"/opt/fmt.sh"}]},"extra":"kept"}`), 0o644)
	if missing, _ := CursorHooksMissing(); strings.Join(missing, ",") != "beforeSubmitPrompt,postToolUse,postToolUseFailure,preToolUse" {
		t.Errorf("your stop hook counts, a bare 'true' doesn't: missing %v", missing)
	}
	if err := EnableCursorHooks(); err != nil {
		t.Fatal(err)
	}
	if missing, _ := CursorHooksMissing(); len(missing) != 0 {
		t.Errorf("still missing %v", missing)
	}
	data, _ := os.ReadFile(path)
	for _, want := range []string{`"/opt/mine/notify.sh"`, `"/opt/fmt.sh"`, `"extra": "kept"`, `"command": "true"`, `"command": "/usr/bin/true"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("hooks file lost or lacks %s:\n%s", want, data)
		}
	}
	// One for beforeSubmitPrompt, one per question-tool event (matched to
	// the question tool), none for stop: your own hook counts there.
	if strings.Count(string(data), "/usr/bin/true") != 4 || strings.Count(string(data), `"matcher": "`+cursorQuestionMatcher+`"`) != 3 {
		t.Errorf("no-ops added:\n%s", data)
	}
}

func fakePi(found map[string]string) *Pi {
	return &Pi{
		lookPath: func(name string) (string, error) {
			if p, ok := found[name]; ok {
				return p, nil
			}
			return "", errors.New("not found")
		},
		newID: func() (string, error) { return "1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed", nil },
	}
}

// pi crew start on a session id ws chose, with the opening prompt as the
// argument, like Claude Code's.
func TestPi_Start(t *testing.T) {
	t.Setenv(EnvPiBin, "")
	p := fakePi(map[string]string{"pi": "/bin/pi"})
	l, err := p.Start(Brief{Slug: "auth", Title: "Auth", Brief: "Fix it.", Dir: "/w/fe/auth", Model: "anthropic/claude-sonnet-5-5", Prompt: "Read ws show -f llm. Fix it."})
	if err != nil {
		t.Fatal(err)
	}
	const id = "1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed"
	want := []string{"/bin/pi", "--session-id", id, "--name", "ws auth", "--model", "anthropic/claude-sonnet-5-5", "--"}
	if len(l.Args) != len(want)+1 || strings.Join(l.Args[:len(want)], " ") != strings.Join(want, " ") {
		t.Fatalf("args = %q", l.Args)
	}
	if prompt := l.Args[len(want)]; !strings.Contains(prompt, "ws show -f llm") || !strings.Contains(prompt, "Fix it.") || l.Prompt != "" {
		t.Errorf("prompt arg = %q, typed = %q", prompt, l.Prompt)
	}
	if l.Conversation != id || l.Dir != "/w/fe/auth" || len(l.Notes) != 2 {
		t.Errorf("launch = %+v", l)
	}
	// No --approve: trusting a project loads the extensions it ships, so
	// that's the user's call.
	if strings.Contains(strings.Join(l.Args, " "), "--approve") {
		t.Errorf("args = %q", l.Args)
	}

	l, _ = p.Start(Brief{Slug: "new-thing", Role: "briefing", Request: "fe issue 45"})
	if strings.Join(l.Args[3:5], " ") != "--name ws new-thing · briefing" || strings.Contains(strings.Join(l.Args, " "), "--model") {
		t.Errorf("briefing args = %q", l.Args)
	}
}

func TestPi_ResumeAndAvailability(t *testing.T) {
	t.Setenv(EnvPiBin, "")
	p := fakePi(map[string]string{"pi": "/bin/pi"})
	// --session, not --session-id: that would quietly start a blank session
	// if pi had lost this one.
	l, err := p.Resume("/w", "abc")
	if err != nil || strings.Join(l.Args, " ") != "/bin/pi --session abc" || l.Dir != "/w" || l.Conversation != "abc" {
		t.Errorf("resume = %+v, %v", l, err)
	}
	if _, err := p.Resume("/w", ""); err == nil {
		t.Error("resume without a session id should fail")
	}
	if err := fakePi(nil).Available(); err == nil || !strings.Contains(err.Error(), EnvPiBin) {
		t.Errorf("missing pi: %v", err)
	}
	t.Setenv(EnvPiBin, "/opt/pi-dev")
	if b, err := fakePi(map[string]string{"/opt/pi-dev": "/opt/pi-dev"}).bin(); err != nil || b != "/opt/pi-dev" {
		t.Errorf("override bin = %q, %v", b, err)
	}
}

// The orchestrator runs in print mode without a saved session, reading and
// running commands but not editing; with no model configured, on pi's own
// default.
func TestPi_Orchestrate(t *testing.T) {
	t.Setenv(EnvPiBin, "")
	p := fakePi(map[string]string{"pi": "/bin/pi"})
	scratch := t.TempDir()
	l, err := p.Orchestrate(OrchestratorRun{Prompt: "Read the prompt file.", Dir: "/root", Model: "haiku", Scratch: scratch, Capsules: []string{"/ws/repos/api/cache"}})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(l.Args, " ")
	if !strings.HasPrefix(got, "/bin/pi -p --mode json --no-session --no-extensions --no-mcp -e "+scratch+"/pi/ws-converse-guard-") || !strings.HasSuffix(got, " --tools read,bash,grep,find,ls --model haiku -- Read the prompt file.") || l.Dir != "/root" || l.Stream != StreamPi {
		t.Errorf("args = %q, dir %q", got, l.Dir)
	}
	src, _ := os.ReadFile(l.Args[slices.Index(l.Args, "-e")+1])
	if !strings.Contains(string(src), `"ws mission"`) || !strings.Contains(string(src), `"git -C /ws/repos/api/cache log"`) {
		t.Errorf("guard:\n%s", src)
	}
	if _, err := p.Orchestrate(OrchestratorRun{Prompt: "x", Dir: "/root"}); err == nil {
		t.Error("no folder for its extension")
	}
	if p.OrchestratorModel() != "" {
		t.Errorf("orchestrator model = %q", p.OrchestratorModel())
	}
	pick, o, err := (Roles{Crew: Pick{Harness: "pi", Model: "sonnet"}}).ForOrchestrator()
	if err != nil || o.Name() != "pi" || pick != (Pick{Harness: "pi"}) {
		t.Errorf("pi crew's orchestrator = %+v, %v", pick, err)
	}
	l, _ = p.Orchestrate(OrchestratorRun{Prompt: "x", Dir: "/root", Scratch: scratch})
	if strings.Contains(strings.Join(l.Args, " "), "--model") {
		t.Errorf("no model configured: args = %q", l.Args)
	}
}

// ws's hooks go into a pi session as an extension ws writes, which runs the
// hook command with each of ws's events.
func TestPi_HookArgs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hooks", "pi")
	cmd := []string{"/w/.ws/bin/ws", "mission", "hook", "--root", "/w s"}
	args, err := (&Pi{}).HookArgs(cmd, dir)
	path := filepath.Join(dir, "ws-crew-hooks.ts")
	if err != nil || strings.Join(args, " ") != "--extension "+path {
		t.Fatalf("args = %q, %v", args, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	// The command is a JSON array, run without a shell.
	if !strings.Contains(src, `const command: string[] = ["/w/.ws/bin/ws","mission","hook","--root","/w s"];`) {
		t.Errorf("command missing:\n%s", src)
	}
	for event, ours := range map[string]string{
		"session_start": HookStart, "before_agent_start": HookPrompt,
		"agent_settled": HookStop, "ui_prompt_start": HookNotify,
	} {
		re := regexp.MustCompile(`pi\.on\("` + event + `",[^}]*?send\("` + ours + `"`)
		if !re.MatchString(src) {
			t.Errorf("no %s hook sending %q", event, ours)
		}
	}
	if strings.Contains(src, "$") {
		t.Errorf("a placeholder wasn't filled in:\n%s", src)
	}
	// agent_end isn't the end of a turn: pi may still retry or go on.
	if strings.Contains(src, `"agent_end"`) {
		t.Error("stop should come from agent_settled, not agent_end")
	}
	if _, err := (&Pi{}).HookArgs(nil, dir); err == nil {
		t.Error("no command should fail")
	}
}

// A conversation's turn runs headless, resuming its session when it has
// one, allowed only to read and ws show.
func TestClaude_Converse(t *testing.T) {
	t.Setenv(EnvClaudeBin, "")
	c := fakeClaude(map[string]string{"claude": "/bin/claude"})
	l, err := c.Converse(ConverseRun{Prompt: "Answer t3.", Dir: "/root", Model: "sonnet"})
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(l.Args, " ")
	for _, want := range []string{"/bin/claude -p --output-format stream-json --verbose --permission-mode dontAsk --max-turns 30 --model sonnet --allowedTools Read Grep Glob", "Bash(ws show *)", "-- Answer t3."} {
		if !strings.Contains(args, want) {
			t.Errorf("args missing %q: %s", want, args)
		}
	}
	for _, never := range []string{"--resume", "--no-session-persistence", "--include-partial-messages", "Edit", "Write", "Bash(ws mission *)", "Bash(ws *)"} {
		if strings.Contains(args, never) {
			t.Errorf("args shouldn't contain %q: %s", never, args)
		}
	}
	if l.Stream != StreamClaude || l.Dir != "/root" {
		t.Errorf("launch = %+v", l)
	}
	// With no tools (a review's guide), it says what it's writing as it
	// writes it: there's nothing else to show while it runs.
	l, _ = c.Converse(ConverseRun{Prompt: "Guide it.", Dir: "/root", Alone: true})
	if args := strings.Join(l.Args, " "); !strings.Contains(args, "--include-partial-messages --tools  -- Guide it.") || strings.Contains(args, "--allowedTools") {
		t.Errorf("alone: %s", args)
	}
	l, _ = c.Converse(ConverseRun{Prompt: "And?", Dir: "/root", Session: "abc-123"})
	if args := strings.Join(l.Args, " "); !strings.Contains(args, "--resume abc-123") || strings.Contains(args, "--model") || l.Conversation != "abc-123" {
		t.Errorf("resumed: %s", args)
	}
	// A talk proposes a document: ws doc add with --propose, and only so.
	l, _ = c.Converse(ConverseRun{Prompt: "Make it a doc.", Dir: "/root", Talk: true})
	tools := l.Args[slices.Index(l.Args, "--allowedTools")+1 : slices.Index(l.Args, "--")]
	for _, want := range []string{"Bash(ws doc add --propose *)", "Bash(ws mission new --propose *)", "Bash(ws artifact add --propose *)", "Bash(ws artifact show *)"} {
		if !slices.Contains(tools, want) {
			t.Errorf("a talk can't %s: %v", want, tools)
		}
	}
	for _, tool := range tools {
		if tool == "Bash(ws doc *)" || tool == "Bash(ws doc add *)" || tool == "Bash(ws artifact *)" || tool == "Bash(ws artifact add *)" || tool == "Bash(ws *)" || tool == "Edit" || tool == "Write" {
			t.Errorf("a talk may %s: %v", tool, tools)
		}
	}
}

// Claude Code's rules can't admit an artifact's HTML on stdin (a quoted
// string in braces, in the heredoc's body, is refused under an allowed
// prefix), so a talk's run asks ws first: a PreToolUse hook on Bash runs
// ws's guard with a file of what's allowed, and it says allow for a
// command ws reads as allowed, deny (with what to do) for one that runs ws
// and isn't, and nothing for anything else.
func TestClaude_ConverseGuard(t *testing.T) {
	t.Setenv(EnvClaudeBin, "")
	c := fakeClaude(map[string]string{"claude": "/bin/claude"})
	scratch := t.TempDir()
	l, err := c.Converse(ConverseRun{Prompt: "Show me.", Dir: "/root", Talk: true, Scratch: scratch, Guard: []string{"/opt/ws bin/workspace", "conversation", "allowed"}})
	if err != nil {
		t.Fatal(err)
	}
	i := slices.Index(l.Args, "--settings")
	if i < 0 || i > slices.Index(l.Args, "--") {
		t.Fatalf("no --settings before the prompt: %v", l.Args)
	}
	var settings struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct{ Type, Command string }
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(l.Args[i+1]), &settings); err != nil {
		t.Fatal(err)
	}
	pre := settings.Hooks["PreToolUse"]
	if len(settings.Hooks) != 1 || len(pre) != 1 || pre[0].Matcher != "Bash" || len(pre[0].Hooks) != 1 || pre[0].Hooks[0].Type != "command" {
		t.Fatalf("hooks: %+v", settings.Hooks)
	}
	cmd := pre[0].Hooks[0].Command
	file := strings.TrimPrefix(cmd, "'/opt/ws bin/workspace' conversation allowed ")
	if file == cmd || filepath.Dir(file) != filepath.Join(scratch, "claude") {
		t.Fatalf("the hook's command: %s", cmd)
	}
	var list guardList
	data, _ := os.ReadFile(file)
	if json.Unmarshal(data, &list) != nil || !slices.Equal(list.Allow, converseCommands(ConverseRun{Talk: true})) || list.Sandboxed {
		t.Errorf("the allow list: %s", data)
	}
	// None without a guard, or with no tools at all.
	for _, r := range []ConverseRun{{Prompt: "x", Dir: "/root", Talk: true, Scratch: scratch}, {Prompt: "x", Dir: "/root", Alone: true, Scratch: scratch, Guard: []string{"ws"}}} {
		if l, _ := c.Converse(r); slices.Contains(l.Args, "--settings") {
			t.Errorf("a guard for %+v: %v", r, l.Args)
		}
	}

	event := func(tool, command string) []byte {
		b, _ := json.Marshal(map[string]any{"tool_name": tool, "tool_input": map[string]string{"command": command}})
		return b
	}
	glow := "ws artifact add --propose --title \"Glow\" --note \"8% in dark\" - <<'EOF'\n<!doctype html>\n<style>.p::before { content: \"\"; }</style>\n<script>const o = { \"k\": 1 };</script>\nEOF"
	for command, want := range map[string]string{
		glow:                                 "allow",
		"ws artifact add --propose":          "allow",
		"ws show -f llm a3":                  "allow",
		"ws artifact add glow.html --note x": "deny",
		"ws artifact add --propose x.html && touch y":          "deny",
		"ws artifact add --propose - <<EOF\n$(touch y)\nEOF":   "deny",
		"ws artifact add --propose - <<'EOF'\nx\nEOF\ntouch y": "deny",
		"touch y": "",
	} {
		answer := GuardAnswer(event("Bash", command), file, nil)
		got := ""
		for _, d := range []string{"allow", "deny"} {
			if strings.Contains(answer, `"permissionDecision":"`+d+`"`) {
				got = d
			}
		}
		if got != want {
			t.Errorf("%q: %s, want %q", command, answer, want)
		}
	}
	if GuardAnswer(event("Write", "x"), file, nil) != "" || GuardAnswer([]byte("{"), file, nil) != "" || GuardAnswer(event("Bash", "ws show"), filepath.Join(scratch, "none.json"), nil) != "" {
		t.Error("allowed what isn't a shell command on the list")
	}
}

// Conversations are held by [conversation], else the crew's harness, on
// its conversation model: Claude Code, Cursor and pi all can.
func TestRoles_ForConversation(t *testing.T) {
	t.Setenv(EnvDefault, "")
	p, _, err := Roles{Crew: Pick{Harness: "cursor"}}.ForConversation()
	if err != nil || p.Harness != "cursor" || p.Model != CursorConverseModel {
		t.Errorf("cursor crew: %+v, %v", p, err)
	}
	p, _, _ = Roles{Crew: Pick{Harness: "claude"}, Conversation: Pick{Model: "opus"}}.ForConversation()
	if p.Harness != "claude" || p.Model != "opus" {
		t.Errorf("a model: %+v", p)
	}
	if p, _, err := (Roles{Conversation: Pick{Harness: "pi"}}).ForConversation(); err != nil || p.Harness != "pi" || p.Model != "" {
		t.Errorf("pi: %+v, %v", p, err)
	}
}

// The judge is the planner's agent and model unless [judge] says, and a
// [judge] model alone goes on the planner's harness.
func TestRoles_ForJudge(t *testing.T) {
	t.Setenv(EnvDefault, "")
	p, _, err := Roles{Crew: Pick{Harness: "cursor"}, Briefing: Pick{Harness: "claude", Model: "opus"}}.ForJudge()
	if err != nil || p.Harness != "claude" || p.Model != "opus" {
		t.Errorf("the planner's: %+v, %v", p, err)
	}
	p, _, _ = Roles{Crew: Pick{Harness: "claude"}, Judge: Pick{Model: "sonnet"}}.ForJudge()
	if p.Harness != "claude" || p.Model != "sonnet" {
		t.Errorf("a model alone: %+v", p)
	}
	p, _, _ = Roles{Briefing: Pick{Harness: "claude", Model: "opus"}, Judge: Pick{Harness: "cursor"}}.ForJudge()
	if p.Harness != "cursor" || p.Model != CursorConverseModel {
		t.Errorf("its own harness: %+v", p)
	}
	if _, _, err := (Roles{Judge: Pick{Harness: "nope"}}).ForJudge(); err == nil {
		t.Error("judged on no harness")
	}
}

// Every harness that holds conversations allows the same: reading, ws show
// and its kin, a thread's suggestion, and for a talk, the missions, GitHub
// and read-only git by each path in full; never ws mission's other verbs,
// editing, or git with options before its subcommand.
func TestConverseCommandsTakeThePluginsMarked(t *testing.T) {
	talk := converseCommands(ConverseRun{Talk: true, Plugins: []string{"sketch edit"}})
	if !commandAllowed("ws sketch edit sketch-1 '[]'", talk) || !commandAllowed("workspace sketch edit sketch-1 <<'EOF'\n[]\nEOF", talk) {
		t.Errorf("a talk can't run a plugin's command marked for it: %v", talk)
	}
	if commandAllowed("ws sketch delete sketch-1", talk) || commandAllowed("ws sketch", talk) {
		t.Error("a talk may run a plugin's command that isn't marked")
	}
	if !commandAllowed("ws sketch edit sketch-1 '[]'", converseCommands(ConverseRun{Plugins: []string{"sketch edit"}})) {
		t.Error("a thread can't run a plugin's command marked for conversations")
	}
}

func TestConverseCommands(t *testing.T) {
	thread := strings.Join(converseCommands(ConverseRun{}), "|")
	talk := strings.Join(converseCommands(ConverseRun{Talk: true, ReadPaths: []string{"/ws/repos/api/.ground", "/has space"}}), "|")
	for _, want := range []string{"ws show", "workspace show", "ws thread suggest", "cat", "ws artifact show", "ws artifact list"} {
		if !strings.Contains("|"+thread+"|", "|"+want+"|") {
			t.Errorf("a thread can't %q: %s", want, thread)
		}
	}
	for _, want := range []string{"ws mission list", "gh pr view", "git -C /ws/repos/api/.ground log", "git -C /ws/repos/api/.ground blame"} {
		if !strings.Contains("|"+talk+"|", "|"+want+"|") {
			t.Errorf("a talk can't %q: %s", want, talk)
		}
	}
	for _, never := range []string{"ws mission decide", "ws mission new", "ws thread suggest", "has space", "git log", "ws mission dispatch", "ws mission replan", "ws doc add", "ws doc", "workspace doc add", "ws artifact", "ws artifact add", "ws artifacts", "ws artifact discard"} {
		if strings.Contains("|"+talk+"|", "|"+never+"|") || strings.Contains(talk, "has space") {
			t.Errorf("a talk may %q: %s", never, talk)
		}
	}
	for _, want := range []string{"ws mission decide --propose", "ws mission new --propose", "ws mission replan --propose", "ws doc add --propose", "workspace doc add --propose", "ws doc amend --propose", "workspace doc amend --propose", "ws artifact add --propose", "workspace artifact add --propose"} {
		if !strings.Contains("|"+talk+"|", "|"+want+"|") {
			t.Errorf("a talk can't %q: %s", want, talk)
		}
	}
	// A document or an artifact is only proposed, and only in a talk: a
	// thread suggests an edit to the document it's on.
	if strings.Contains(thread, "doc add") || strings.Contains(thread, "artifact add") {
		t.Errorf("a thread may add a document or an artifact: %s", thread)
	}
	// No reading prefix the list allows takes in the verb itself.
	for _, cmd := range converseCommands(ConverseRun{Talk: true}) {
		for _, verb := range []string{"doc add", "artifact add", "artifact discard"} {
			for _, ws := range []string{"ws", "workspace"} {
				if cmd != ws+" "+verb+" --propose" && strings.HasPrefix(ws+" "+verb+" x", cmd+" ") {
					t.Errorf("%q allows %s %s without --propose", cmd, ws, verb)
				}
			}
		}
	}
}

// Cursor holds a conversation in ask mode (read-only) without --force, in
// a folder of ws's whose .cursor/cli.json allows exactly the commands.
func TestCursor_Converse(t *testing.T) {
	c := fakeCursor(map[string]string{"cursor-agent": "/bin/cursor-agent"}, "", nil)
	scratch := t.TempDir()
	l, err := c.Converse(ConverseRun{Prompt: "Answer t3.", Dir: "/root", Model: "auto", Scratch: scratch})
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(l.Args, " ")
	if args != "/bin/cursor-agent -p --trust --mode ask --output-format stream-json --model auto -- Answer t3." {
		t.Errorf("args: %s", args)
	}
	if filepath.Dir(l.Dir) != filepath.Join(scratch, "cursor") || l.Stream != StreamCursor {
		t.Errorf("launch = %+v", l)
	}
	data, _ := os.ReadFile(filepath.Join(l.Dir, ".cursor", "cli.json"))
	var cfg struct {
		Permissions struct{ Allow []string } `json:"permissions"`
	}
	json.Unmarshal(data, &cfg)
	if allow := strings.Join(cfg.Permissions.Allow, " "); !strings.Contains(allow, "Shell(ws show)") || !strings.Contains(allow, "Shell(ws thread suggest)") || strings.Contains(allow, "Shell(ws mission") {
		t.Errorf("allowed: %s", allow)
	}
	// A thread and a talk are allowed different things: each its own folder.
	talk, _ := c.Converse(ConverseRun{Prompt: "x", Dir: "/root", Scratch: scratch, Talk: true})
	if talk.Dir == l.Dir {
		t.Error("a thread's and a talk's permissions share a folder")
	}
	// A talk proposes a document: Shell(ws doc add --propose), and no
	// shorter rule that would let it add one. A thread doesn't.
	if allow := strings.Join(cfg.Permissions.Allow, " "); strings.Contains(allow, "doc add") {
		t.Errorf("a thread may propose a document: %s", allow)
	}
	data, _ = os.ReadFile(filepath.Join(talk.Dir, ".cursor", "cli.json"))
	cfg.Permissions.Allow = nil
	json.Unmarshal(data, &cfg)
	if !slices.Contains(cfg.Permissions.Allow, "Shell(ws doc add --propose)") || slices.Contains(cfg.Permissions.Allow, "Shell(ws doc add)") || slices.Contains(cfg.Permissions.Allow, "Shell(ws doc)") ||
		!slices.Contains(cfg.Permissions.Allow, "Shell(ws artifact add --propose)") || slices.Contains(cfg.Permissions.Allow, "Shell(ws artifact add)") || slices.Contains(cfg.Permissions.Allow, "Shell(ws artifact)") {
		t.Errorf("a talk's allowed: %v", cfg.Permissions.Allow)
	}
	c.chats = t.TempDir()
	os.MkdirAll(filepath.Join(c.chats, "abc123", "chat-1"), 0o755)
	l, _ = c.Converse(ConverseRun{Prompt: "And?", Dir: "/root", Session: "chat-1", Scratch: scratch})
	if args := strings.Join(l.Args, " "); !strings.Contains(args, "--resume chat-1") || strings.Contains(args, "--force") || l.Conversation != "chat-1" {
		t.Errorf("resumed: %s", args)
	}
	if _, err := c.Converse(ConverseRun{Prompt: "And?", Dir: "/root", Session: "chat-gone", Scratch: scratch}); !errors.Is(err, ErrSessionGone) {
		t.Errorf("resuming a chat Cursor doesn't have: %v", err)
	}
	if _, err := c.Converse(ConverseRun{Prompt: "x", Dir: "/root"}); err == nil {
		t.Error("no folder for its permissions")
	}
}

// pi holds a conversation on a session ws names, with only its reading
// tools and the shell, which ws's extension limits.
func TestPi_Converse(t *testing.T) {
	p := fakePi(map[string]string{"pi": "/bin/pi"})
	scratch := t.TempDir()
	l, err := p.Converse(ConverseRun{Prompt: "Answer t3.", Dir: "/root", Scratch: scratch})
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(l.Args, " ")
	for _, want := range []string{"/bin/pi -p --mode json --session-id 1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed --no-extensions --no-mcp -e " + scratch + "/pi/ws-converse-guard-", "--tools read,bash,grep,find,ls -- Answer t3."} {
		if !strings.Contains(args, want) {
			t.Errorf("args missing %q: %s", want, args)
		}
	}
	if l.Dir != "/root" || l.Stream != StreamPi || l.Conversation != "1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed" {
		t.Errorf("launch = %+v", l)
	}
	guard := l.Args[slices.Index(l.Args, "-e")+1]
	src, _ := os.ReadFile(guard)
	if !strings.Contains(string(src), `"ws thread suggest"`) || !strings.Contains(string(src), `pi.on("tool_call"`) {
		t.Errorf("guard:\n%s", src)
	}
	l, _ = p.Converse(ConverseRun{Prompt: "And?", Dir: "/root", Session: "sess-9", Scratch: scratch, Model: "openrouter/x"})
	if args := strings.Join(l.Args, " "); !strings.Contains(args, "--session sess-9") || strings.Contains(args, "--session-id") || !strings.Contains(args, "--model openrouter/x") || l.Conversation != "sess-9" {
		t.Errorf("resumed: %s", args)
	}
}
