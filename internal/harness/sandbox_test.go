package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// claudeSettings is the --settings a launch was given, read.
func claudeSettings(t *testing.T, l Launch) map[string]any {
	t.Helper()
	i := slices.Index(l.Args, "--settings")
	if i < 0 || i > slices.Index(l.Args, "--") {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(l.Args[i+1]), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// tree makes folders and files under a new folder, and says where it is.
func tree(t *testing.T, paths ...string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		full := filepath.Join(root, filepath.FromSlash(p))
		if strings.HasSuffix(p, "/") {
			os.MkdirAll(full, 0o755)
			continue
		}
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, nil, 0o644)
	}
	return root
}

// A conversation that asks for a sandbox, on Claude Code where it has one,
// runs its shell in it: any command unasked, the network open, writing
// where it's told and nowhere else under the workspace, with ws and gh
// left outside to the rules. Its file tools write in the folders it works
// in. Elsewhere, or with no tools, it runs as it did.
func TestClaude_ConverseSandbox(t *testing.T) {
	t.Setenv(EnvClaudeBin, "")
	root := tree(t, "ws.toml", ".ws/ws.db", "repos/api/.ground/main.go", "repos/api/try/main.go", "repos/api/cache-tokens/main.go",
		"repos/api/.bare/config", "repos/api/.bare/hooks/", "repos/api/.bare/objects/", "repos/api/.bare/refs/", "repos/api/.bare/worktrees/try/HEAD", "repos/api/.bare/worktrees/cache-tokens/HEAD",
		"repos/web/.ground/x")
	own := tree(t)
	try := filepath.Join(root, "repos/api/try")
	bare := filepath.Join(root, "repos/api/.bare")
	run := ConverseRun{Prompt: "How slow are the tests?", Dir: root, Talk: true, Sandbox: true, Lift: true,
		Scratch: t.TempDir(), Guard: []string{"/opt/ws", "conversation", "allowed"},
		Write:  []string{own, try},
		Beside: []string{filepath.Join(bare, "worktrees/try"), filepath.Join(bare, "objects"), filepath.Join(bare, "refs"), "/home/me/.cache"},
		Room:   []string{filepath.Join(root, "repos/api"), filepath.Join(bare, "worktrees"), filepath.Join(root, "repos/web")}}

	c := fakeClaude(map[string]string{"claude": "/bin/claude"})
	c.goos = "darwin"
	l, err := c.Converse(run)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(l.Args, " ")
	if !strings.Contains(args, "--max-turns "+claudeSandboxTurns) {
		t.Errorf("a turn that runs things goes no further than one that reads: %s", args)
	}
	tools := l.Args[slices.Index(l.Args, "--allowedTools")+1 : slices.Index(l.Args, "--settings")]
	for _, want := range []string{"Bash(ws lift *)", "Bash(ws show *)", "Bash(gh run view *)"} {
		if !slices.Contains(tools, want) {
			t.Errorf("no %s: %v", want, tools)
		}
	}
	for _, tool := range tools {
		if strings.HasPrefix(tool, "Edit") || strings.HasPrefix(tool, "Write") || tool == "Bash" || tool == "Bash(*)" || tool == "Bash(ws *)" || tool == "Bash(gh *)" {
			t.Errorf("it may %s", tool)
		}
	}
	// ws's guard is asked of the shell and of the tools that write files.
	pre := claudeSettings(t, l)["hooks"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)
	if pre["matcher"] != "Bash|Write|Edit|MultiEdit|NotebookEdit" {
		t.Errorf("what the guard's asked of: %v", pre["matcher"])
	}
	box, _ := claudeSettings(t, l)["sandbox"].(map[string]any)
	if box == nil || box["enabled"] != true || box["autoAllowBashIfSandboxed"] != true || box["allowUnsandboxedCommands"] != false {
		t.Fatalf("the sandbox: %v", box)
	}
	if got := strings.Join(strs(box["excludedCommands"]), "|"); got != "ws *|workspace *|gh *" {
		t.Errorf("what leaves it: %s", got)
	}
	fs := box["filesystem"].(map[string]any)
	if got := strs(fs["allowWrite"]); !slices.Equal(got, append(append([]string{}, run.Write...), run.Beside...)) {
		t.Errorf("it writes in %v", got)
	}
	closed := strs(fs["denyWrite"])
	for _, want := range []string{"ws.toml", ".ws", "repos/web/.ground", "repos/api/.ground", "repos/api/cache-tokens", "repos/api/.bare/config", "repos/api/.bare/hooks", "repos/api/.bare/worktrees/cache-tokens"} {
		if !slices.Contains(closed, filepath.Join(root, want)) {
			t.Errorf("%s is open to it: %v", want, closed)
		}
	}
	for _, never := range []string{"repos", "repos/api", "repos/web", "repos/api/try", "repos/api/.bare", "repos/api/.bare/objects", "repos/api/.bare/refs", "repos/api/.bare/worktrees", "repos/api/.bare/worktrees/try"} {
		if slices.Contains(closed, filepath.Join(root, never)) {
			t.Errorf("%s is closed to it: %v", never, closed)
		}
	}

	// With no room made in the repos, and nothing lifted, they're closed
	// whole.
	bare1 := run
	bare1.Write, bare1.Beside, bare1.Room = []string{own}, nil, nil
	l, _ = c.Converse(bare1)
	fs = claudeSettings(t, l)["sandbox"].(map[string]any)["filesystem"].(map[string]any)
	if closed := strs(fs["denyWrite"]); !slices.Contains(closed, filepath.Join(root, "repos")) {
		t.Errorf("with no capsule lifted: %v", closed)
	}

	// No sandbox where there's none to run (Linux without bubblewrap and
	// socat), for a run that didn't ask, or one with no tools: as before.
	linux := fakeClaude(map[string]string{"claude": "/bin/claude", "bwrap": "/usr/bin/bwrap"})
	linux.goos = "linux"
	if linux.Sandboxes() {
		t.Error("a sandbox on Linux without socat")
	}
	plain, alone := run, run
	plain.Sandbox, alone.Alone = false, true
	for name, l := range map[string]Launch{"linux": must(linux.Converse(run)), "unasked": must(c.Converse(plain)), "alone": must(c.Converse(alone))} {
		args := strings.Join(l.Args, " ")
		if s := claudeSettings(t, l); s["sandbox"] != nil || strings.Contains(args, "Write|Edit") || strings.Contains(args, "--max-turns "+claudeSandboxTurns) {
			t.Errorf("%s: sandboxed: %s", name, args)
		}
	}
	both := fakeClaude(map[string]string{"claude": "/bin/claude", "bwrap": "/usr/bin/bwrap", "socat": "/usr/bin/socat"})
	both.goos = "linux"
	if !both.Sandboxes() || !Sandboxes(both) || Sandboxes(&Pi{}) {
		t.Error("which harnesses sandbox")
	}

	// A workspace that can't be read isn't opened whole.
	gone := run
	gone.Dir = filepath.Join(root, "nowhere")
	if _, err := c.Converse(gone); err == nil {
		t.Error("sandboxed in a folder it couldn't list")
	}
}

func must(l Launch, err error) Launch {
	if err != nil {
		panic(err)
	}
	return l
}

func strs(v any) []string {
	var out []string
	list, _ := v.([]any)
	for _, x := range list {
		s, _ := x.(string)
		out = append(out, s)
	}
	return out
}

// What a script runs is read loosely, past pipes and redirections, but not
// into what's quoted or a heredoc's body.
func TestCommandStarts(t *testing.T) {
	for script, want := range map[string]string{
		"go test ./... 2>&1 | tail -20":                           "go tail",
		"cd repos/api/try && time go test ./... > /tmp/out.txt":   "cd time",
		"FOO=1 BAR=2 ws show w3":                                  "ws",
		"echo 'ws show' \"gh pr view\"":                           "echo",
		"cat > notes.md <<'EOF'\nws show w3\ngh pr list\nEOF\nls": "cat ls",
		"x=$(gh pr list); echo $x":                                "gh echo",
		"for f in a b; do wc -l $f; done":                         "for do done",
		"# ws show\nls":                                           "ls",
		"/opt/bin/ws status":                                      "/opt/bin/ws",
		"go test \\\n  ./... & wait":                              "go wait",
	} {
		if got := strings.Join(commandStarts(script), " "); got != want {
			t.Errorf("%q starts %q, want %q", script, got, want)
		}
	}
}

// A script runs ws or gh where one starts a command, or follows a word
// that runs what comes after it: every way Claude Code might read a
// command as one that leaves the sandbox. A word that only names one
// doesn't.
func TestRunsOutside(t *testing.T) {
	for script, want := range map[string]string{
		"ws burn api try":                          "ws",
		"  workspace  status":                      "workspace",
		"timeout 5 ws burn api try":                "ws",
		"time nice -n 5 gh pr merge 3":             "gh",
		"FOO=1 env BAR=2 ws land api x":            "ws",
		"ls | xargs gh pr view":                    "gh",
		"for c in a b; do ws burn api $c; done":    "ws",
		"if true; then gh pr close 4; fi":          "gh",
		"(ws burn api x)":                          "ws",
		"{ ws burn api x; }":                       "ws",
		"x=$(ws mission list)":                     "ws",
		"echo hi && \\ws burn api x":               "ws",
		"go test ./... > out.txt; 'ws' burn api x": "ws",
		"/usr/local/bin/gh pr merge 3":             "gh",
		"nohup stdbuf -oL ws daemon stop &":        "ws",
		"go test ./... | tee out.txt":              "",
		"grep -rw ws internal/":                    "",
		"ls repos/workspace":                       "",
		"echo \"ws burn api x\"":                   "",
		"cat ws.toml && git log --grep gh":         "",
		"cat > run.sh <<'EOF'\nws burn api x\nEOF": "",
		"sh -c 'ws burn api x'":                    "",
	} {
		if got := runsOutside(script); got != want {
			t.Errorf("%q runs %q outside, want %q", script, got, want)
		}
	}
}

// In a sandboxed run, ws allows what stays in the sandbox, which Claude
// Code's own rules would refuse where it can't read it as plain (a loop,
// a variable set first): anything that runs neither ws nor gh. In a run
// with no sandbox it says nothing of those.
func TestGuardAnswer_Sandboxed(t *testing.T) {
	answerOf := func(file, command string) string {
		event, _ := json.Marshal(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": command}})
		answer := GuardAnswer(event, file, nil)
		for _, d := range []string{"allow", "deny"} {
			if strings.Contains(answer, `"permissionDecision":"`+d+`"`) {
				return d
			}
		}
		return ""
	}
	guardFile := func(sandboxed bool) string {
		t.Helper()
		scratch := t.TempDir()
		hooks, err := claudeGuard([]string{"/opt/ws", "conversation", "allowed"}, filepath.Join(scratch, "claude"), converseCommands(ConverseRun{Talk: true, Lift: true}), sandboxed)
		if err != nil {
			t.Fatal(err)
		}
		cmd := hooks["PreToolUse"].([]map[string]any)[0]["hooks"].([]map[string]string)[0]["command"]
		return strings.TrimPrefix(cmd, "/opt/ws conversation allowed ")
	}
	boxed, plain := guardFile(true), guardFile(false)
	if boxed == plain {
		t.Fatal("one file for a sandboxed run and one that isn't")
	}
	for command, want := range map[string][2]string{
		"for i in 1 2 3; do /usr/bin/time -p go test -count=1 ./... 2>&1; done": {"allow", ""},
		"export GOCACHE=$(mktemp -d) && go test ./...":                          {"allow", ""},
		"git -C /r/try -c core.pager=cat log -3":                                {"allow", ""},
		"ws show -f llm w3":                                                     {"allow", "allow"},
		"ws lift api parallel-tests":                                            {"allow", "allow"},
		"gh run list --limit 20":                                                {"allow", "allow"},
		"ws burn api try":                                                       {"deny", "deny"},
		"timeout 5 ws burn api try":                                             {"deny", "deny"},
		"go test ./... && gh pr create --fill":                                  {"deny", "deny"},
		"for c in a b; do ws burn api $c; done":                                 {"deny", "deny"},
	} {
		if got := [2]string{answerOf(boxed, command), answerOf(plain, command)}; got != want {
			t.Errorf("%q: sandboxed %q, not %q; want %v", command, got[0], got[1], want)
		}
	}
}

// A ws or gh command a conversation's agent may not run is refused with
// what to do in its place; a command that runs neither is left to the
// agent's own rules, and its sandbox.
func TestWhyRefused(t *testing.T) {
	allow := converseCommands(ConverseRun{Talk: true, Lift: true})
	for cmd, want := range map[string]string{
		"ws mission new \"Cache tokens\"":        "--propose straight after `ws mission new`",
		"workspace doc add doc notes.md":         "--propose straight after `workspace doc add`",
		"ws burn api try":                        "isn't one of the ws commands a conversation's agent runs",
		"timeout 5 ws show w3":                   "runs on its own",
		"FOO=1 ws show w3":                       "runs on its own",
		"ws show w3 | head -20":                  "runs on its own",
		"ws status -f llm > /tmp/status.txt":     "runs on its own",
		"cd repos/api && gh pr list":             "runs on its own",
		"gh pr create --fill":                    "A pull request is opened by a mission",
		"gh pr merge 12":                         "gh pr view",
		"go test ./... | tail":                   "",
		"echo 'ws burn api try'":                 "",
		"cat > x.md <<'EOF'\nws burn api x\nEOF": "",
		"git -C /r push":                         "",
	} {
		got := whyRefused(cmd, allow)
		if (want == "") != (got == "") || !strings.Contains(got, want) {
			t.Errorf("%q: %q, want it to say %q", cmd, got, want)
		}
	}
}

// A ws command given its input by a heredoc is allowed as the same
// command reading that input from a file ws keeps: a sandbox would keep
// the heredoc's command inside it.
func TestGuardAnswer_HeredocAside(t *testing.T) {
	scratch := t.TempDir()
	c := fakeClaude(map[string]string{"claude": "/bin/claude"})
	l, err := c.Converse(ConverseRun{Prompt: "x", Dir: "/root", Talk: true, Scratch: scratch, Guard: []string{"/opt/ws", "conversation", "allowed"}})
	if err != nil {
		t.Fatal(err)
	}
	hooks := claudeSettings(t, l)["hooks"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)
	file := strings.TrimPrefix(hooks["command"].(string), "/opt/ws conversation allowed ")
	event, _ := json.Marshal(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{
		"command": "ws doc add --propose doc --guide 3f2a \"it's here\" - <<'EOF'\n# Slow tests\n\n{ \"x\": $y }\nEOF", "description": "Propose it", "timeout": 5000}})
	var answer struct {
		Out struct {
			Decision string         `json:"permissionDecision"`
			Input    map[string]any `json:"updatedInput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(GuardAnswer(event, file, nil)), &answer); err != nil || answer.Out.Decision != "allow" {
		t.Fatalf("the answer: %+v, %v", answer, err)
	}
	cmd, _ := answer.Out.Input["command"].(string)
	head, aside, ok := strings.Cut(cmd, " "+StdinFlag+" ")
	if !ok || head != `ws doc add --propose doc --guide 3f2a 'it'\''s here' -` || answer.Out.Input["description"] != "Propose it" || answer.Out.Input["timeout"] != float64(5000) {
		t.Fatalf("the command it runs: %q (%v)", cmd, answer.Out.Input)
	}
	if !SetAside(aside) || filepath.Dir(aside) != filepath.Join(scratch, "claude", asideDir) || SetAside("/home/me/notes.md") {
		t.Errorf("where its input's kept: %s", aside)
	}
	if body, _ := os.ReadFile(aside); string(body) != "# Slow tests\n\n{ \"x\": $y }\n" {
		t.Errorf("its input: %q", body)
	}
	// As what it is, the command's still one ws allows, and has no heredoc.
	if !commandAllowed(cmd, converseCommands(ConverseRun{Talk: true})) || strings.Contains(cmd, "<<") {
		t.Errorf("not allowed as rewritten: %s", cmd)
	}
	// One with no heredoc runs as it came; so does one that isn't ws's.
	for _, command := range []string{"ws show -f llm w3", "cat <<'EOF'\nx\nEOF"} {
		event, _ := json.Marshal(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": command}})
		if got := GuardAnswer(event, file, nil); strings.Contains(got, "updatedInput") {
			t.Errorf("%q rewritten: %s", command, got)
		}
	}
}

// Everything under the workspace is closed but what's opened, by name: a
// folder with something open in it has its own things named instead, and
// so has one there's room in, where what's made later is open.
func TestClosedUnder(t *testing.T) {
	root := tree(t, "a/x", "a/y/z", "b/", "c", "d/old/f")
	closed, err := closedUnder(root, []string{filepath.Join(root, "a/y"), "/elsewhere"}, []string{filepath.Join(root, "d")})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "a/x"), filepath.Join(root, "b"), filepath.Join(root, "c"), filepath.Join(root, "d/old")}
	if !slices.Equal(closed, want) {
		t.Errorf("closed %v, want %v", closed, want)
	}
	if closed, _ := closedUnder(root, nil, nil); !slices.Equal(closed, []string{filepath.Join(root, "a"), filepath.Join(root, "b"), filepath.Join(root, "c"), filepath.Join(root, "d")}) {
		t.Errorf("with nothing open: %v", closed)
	}
}

// A sandboxed run's tools for writing files write where its agent works
// (a folder of its own, the capsules its conversation has lifted, as they
// stand when it asks), and are refused anywhere else with where they may;
// a run with no sandbox is left to its rules, which allow neither.
func TestGuardAnswer_FileTools(t *testing.T) {
	root := tree(t, "own/", "repos/api/try/main.go", "repos/api/.ground/main.go")
	os.Symlink(filepath.Join(root, "repos/api/.ground"), filepath.Join(root, "repos/api/try/out"))
	dirs := []string{filepath.Join(root, "own"), filepath.Join(root, "repos/api/try")}
	file := func(sandboxed bool) string {
		hooks, err := claudeGuard([]string{"/opt/ws", "conversation", "allowed"}, filepath.Join(t.TempDir(), "claude"), converseCommands(ConverseRun{Talk: true}), sandboxed)
		if err != nil {
			t.Fatal(err)
		}
		cmd := hooks["PreToolUse"].([]map[string]any)[0]["hooks"].([]map[string]string)[0]["command"]
		return strings.TrimPrefix(cmd, "/opt/ws conversation allowed ")
	}
	boxed, plain := file(true), file(false)
	asked := 0
	folders := func() []string { asked++; return dirs }
	for _, c := range []struct{ tool, key, path, want string }{
		{"Write", "file_path", filepath.Join(root, "own/notes.md"), "allow"},
		{"Edit", "file_path", filepath.Join(root, "repos/api/try/main.go"), "allow"},
		{"Write", "file_path", filepath.Join(root, "repos/api/try/new/deep.go"), "allow"},
		{"NotebookEdit", "notebook_path", filepath.Join(root, "own/x.ipynb"), "allow"},
		{"Edit", "file_path", filepath.Join(root, "repos/api/.ground/main.go"), "deny"},
		{"Write", "file_path", filepath.Join(root, "repos/api/try/../.ground/x.go"), "deny"},
		{"Write", "file_path", filepath.Join(root, "repos/api/try/out/x.go"), "deny"}, // a link out of the capsule
		{"Write", "file_path", filepath.Join(root, "repos/api/try-not/x.go"), "deny"},
		{"Write", "file_path", "notes.md", "deny"},
	} {
		event, _ := json.Marshal(map[string]any{"tool_name": c.tool, "tool_input": map[string]any{c.key: c.path}})
		answer := GuardAnswer(event, boxed, folders)
		if !strings.Contains(answer, `"permissionDecision":"`+c.want+`"`) {
			t.Errorf("%s %s: %s, want %s", c.tool, c.path, answer, c.want)
		}
		if c.want == "deny" && !strings.Contains(answer, "lift a capsule of your own") {
			t.Errorf("%s %s refused with no way through: %s", c.tool, c.path, answer)
		}
		if got := GuardAnswer(event, plain, folders); got != "" {
			t.Errorf("%s %s, with no sandbox: %s", c.tool, c.path, got)
		}
	}
	if asked == 0 {
		t.Error("where it may write wasn't asked")
	}
}
