package harness

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/acp"
)

// fakeOpenCode finds opencode at /bin/opencode (and a path as itself), and
// keeps its sessions in db.
func fakeOpenCode(db string) *OpenCode {
	return &OpenCode{db: db, lookPath: func(name string) (string, error) {
		if name == "opencode" {
			return "/bin/opencode", nil
		}
		return "", errors.New("not found")
	}}
}

// openCodeDB is a database as OpenCode keeps it (WAL, a session table),
// with the sessions given, held open as OpenCode would hold it.
func openCodeDB(t *testing.T, sessions ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode.db")
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE session (id TEXT PRIMARY KEY, directory TEXT, title TEXT)`); err != nil {
		t.Fatal(err)
	}
	for _, id := range sessions {
		if _, err := db.Exec(`INSERT INTO session (id, directory, title) VALUES (?, '/w', 'x')`, id); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestOpenCodeBinary(t *testing.T) {
	t.Setenv(EnvOpenCodeBin, "")
	o := fakeOpenCode("")
	if a, err := o.agent(); err != nil || strings.Join(a, " ") != "/bin/opencode acp" {
		t.Errorf("agent = %q, %v", a, err)
	}
	t.Setenv(EnvOpenCodeBin, "oc-nightly")
	if err := o.Available(); err == nil || !strings.Contains(err.Error(), EnvOpenCodeBin) || !strings.Contains(err.Error(), "oc-nightly") {
		t.Errorf("a binary that isn't there = %v", err)
	}
}

func TestOpenCodeModel(t *testing.T) {
	if m, s := openCodeModel(" anthropic/claude-sonnet-5-5 "); m != "anthropic/claude-sonnet-5-5" || s != nil {
		t.Errorf("a model alone = %q, %+v", m, s)
	}
	// A model whose id has a slash of its own, at one of its variants.
	if m, s := openCodeModel("openrouter/anthropic/claude-opus-5-5[high]"); m != "openrouter/anthropic/claude-opus-5-5" || len(s) != 1 ||
		s[0] != (acp.Setting{Category: "thought_level", Want: "high"}) {
		t.Errorf("a model at an effort = %q, %+v", m, s)
	}
}

func TestOpenCodeCrew(t *testing.T) {
	t.Setenv(EnvOpenCodeBin, "")
	o := fakeOpenCode(openCodeDB(t, "ses_1"))
	l, err := o.Start(Brief{Slug: "m", Title: "M", Dir: "/w/c", Model: "anthropic/claude-haiku-5-5[high]", Prompt: "You are crew on the mission \"M\"."})
	if err != nil || l.ACP == nil || len(l.Args) != 0 || l.Dir != "/w/c" {
		t.Fatalf("crew = %+v, %v", l, err)
	}
	// In OpenCode's build agent, as your config has it: no config of ws's.
	if r := *l.ACP; r.Mode != "build" || r.Model != "anthropic/claude-haiku-5-5" || len(r.Settings) != 1 || r.Settings[0].Want != "high" ||
		r.Prompt != "You are crew on the mission \"M\"." || r.Env != nil || r.Auth != "" ||
		!commandsAllowed("git add a.go && git commit -m x", r.Allow) || commandAllowed("rm -rf ~", r.Allow) {
		t.Errorf("crew run = %+v", r)
	}
	l, err = o.Resume("/w/c", "ses_1")
	if err != nil || l.ACP == nil || l.ACP.Load != "ses_1" || l.ACP.Prompt != "" || l.ACP.Mode != "build" || l.Conversation != "ses_1" {
		t.Errorf("resume = %+v, %v", l, err)
	}
	// Resumed, it's put on the role's model.
	if m := WithModel(o, l, "openai/gpt-6.1-sol[low]").ACP; m.Model != "openai/gpt-6.1-sol" || len(m.Settings) != 1 || l.ACP.Model != "" {
		t.Errorf("with the model = %+v", m)
	}
	for _, id := range []string{"", "ses_gone"} {
		if _, err := o.Resume("/w/c", id); !errors.Is(err, ErrNoResume) {
			t.Errorf("resumed %q = %v", id, err)
		}
	}
	if c := CapsOf("opencode"); c.Crew != CrewACP || c.Transcript != TranscriptACP || !c.Models || !c.Converse || !c.Headless || c.Hooks {
		t.Errorf("caps = %+v", c)
	}
}

// openCodeRunConfig is the config a headless run gives OpenCode, read.
func openCodeRunConfig(t *testing.T, r ACPRun) (raw string, cfg struct {
	Agent        map[string]map[string]json.RawMessage `json:"agent"`
	Experimental map[string]bool                       `json:"experimental"`
}) {
	t.Helper()
	for _, kv := range r.Env {
		if v, ok := strings.CutPrefix(kv, EnvOpenCodeConfig+"="); ok {
			raw = v
		}
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("%s isn't JSON: %v\n%s", EnvOpenCodeConfig, err, raw)
	}
	return raw, cfg
}

func TestOpenCodeHeadless(t *testing.T) {
	t.Setenv(EnvOpenCodeBin, "")
	o := fakeOpenCode(openCodeDB(t, "ses_1"))

	l, err := o.Converse(ConverseRun{Prompt: "hi", Dir: "/w", Session: "ses_1", Talk: true, ReadPaths: []string{"/w/repos/a/.ground"}, Model: "anthropic/claude-sonnet-5-5"})
	if err != nil {
		t.Fatal(err)
	}
	// Nothing to start: the run, held for its turn in process.
	if len(l.Args) != 0 || l.ACP == nil || l.Stream != StreamACP || l.Dir != "/w" || l.Conversation != "ses_1" {
		t.Fatalf("launch = %+v", l)
	}
	r := *l.ACP
	if strings.Join(r.Agent, " ") != "/bin/opencode acp" || r.Mode != openCodeHeadless || r.Load != "ses_1" || r.Prompt != "hi" ||
		r.Model != "anthropic/claude-sonnet-5-5" || r.Settings != nil || r.Auth != "" {
		t.Errorf("conversation run = %+v", r)
	}
	if !commandAllowed("ws show -f llm m1", r.Allow) || !commandAllowed("git -C /w/repos/a/.ground log -3", r.Allow) || commandAllowed("ws mission done x", r.Allow) {
		t.Errorf("a talk allows %q", r.Allow)
	}
	raw, cfg := openCodeRunConfig(t, r)
	if !cfg.Experimental["continue_loop_on_deny"] {
		t.Errorf("a refusal would end the turn: %s", raw)
	}
	agent := cfg.Agent[openCodeHeadless]
	if string(agent["mode"]) != `"primary"` {
		t.Errorf("its agent = %s", raw)
	}
	// The last rule that matches wins: everything denied, then what it
	// may do; a .env file isn't read; every command is asked of ws.
	perm := string(agent["permission"])
	for _, want := range []string{`"bash":"ask"`, `"grep":"allow"`, `"*.env":"deny"`, `"external_directory":"allow"`} {
		if !strings.Contains(perm, want) {
			t.Errorf("permission %s: want %s", perm, want)
		}
	}
	if !strings.HasPrefix(perm, `{"*":"deny",`) || strings.Index(perm, `"*.env":"deny"`) > strings.Index(perm, `"*.env.example":"allow"`) {
		t.Errorf("permission in the wrong order: %s", perm)
	}

	if r.Runs || r.Folders != nil || strings.Contains(perm, `"edit"`) {
		t.Errorf("a talk that only reads runs things: %+v, %s", r, perm)
	}
	// One that runs things: its edits asked of ws too, after everything's
	// denied, fetches allowed, answered by ConversePolicy, its edits
	// judged by its folders.
	l, err = o.Converse(ConverseRun{Prompt: "bump it", Dir: "/w", Talk: true, Sandbox: true, Lift: true, Folders: func() []string { return []string{"/w/repos/a/bump"} }})
	if err != nil {
		t.Fatal(err)
	}
	_, cfg = openCodeRunConfig(t, *l.ACP)
	runs := string(cfg.Agent[openCodeHeadless]["permission"])
	if !l.ACP.Runs || l.ACP.Folders == nil || l.ACP.Folders()[0] != "/w/repos/a/bump" || !strings.HasPrefix(runs, `{"*":"deny",`) ||
		!strings.Contains(runs, `"edit":"ask"`) || !strings.Contains(runs, `"webfetch":"allow"`) || !strings.Contains(runs, `"bash":"ask"`) || !commandAllowed("ws dock a 12", l.ACP.Allow) {
		t.Errorf("a run that runs things = %+v, %s", *l.ACP, runs)
	}
	if !RunsThings(o) || Sandboxes(o) {
		t.Errorf("OpenCode: runs things %v, sandboxes %v", RunsThings(o), Sandboxes(o))
	}

	// A session OpenCode hasn't: gone, so the conversation starts afresh.
	if _, err := o.Converse(ConverseRun{Prompt: "hi", Dir: "/w", Session: "ses_gone"}); !errors.Is(err, ErrSessionGone) {
		t.Errorf("a session OpenCode hasn't = %v", err)
	}

	// Alone: no tools at all, nothing let out.
	l, err = o.Converse(ConverseRun{Prompt: "read this", Dir: "/w", Alone: true})
	if err != nil {
		t.Fatal(err)
	}
	_, cfg = openCodeRunConfig(t, *l.ACP)
	if l.ACP.Allow != nil || string(cfg.Agent[openCodeHeadless]["permission"]) != `{"*":"deny"}` || l.ACP.Model != "" {
		t.Errorf("a run alone = %+v", *l.ACP)
	}

	l, err = o.Orchestrate(OrchestratorRun{Prompt: "go", Dir: "/w", Model: "opencode/big-pickle", Capsules: []string{"/w/repos/a/feat"}})
	if err != nil {
		t.Fatal(err)
	}
	r = *l.ACP
	if r.Mode != openCodeHeadless || r.Model != "opencode/big-pickle" || r.Load != "" ||
		!commandAllowed(`ws mission dispatch c2 "go on"`, r.Allow) || !commandAllowed("git -C /w/repos/a/feat diff", r.Allow) || commandAllowed("ws burn a/feat", r.Allow) {
		t.Errorf("orchestrator run = %+v", r)
	}
	if o.ConverseModel() != "" || o.OrchestratorModel() != "" {
		t.Error("OpenCode's own default is the default")
	}
}

func TestOpenCodeSessionKept(t *testing.T) {
	db := openCodeDB(t, "ses_1")
	o := fakeOpenCode(db)
	if !o.sessionKept("ses_1") || o.sessionKept("ses_2") || o.sessionKept("ses_1' OR '1'='1") {
		t.Error("read from its database")
	}
	// Where it can't tell, yes: resuming says if not.
	if !fakeOpenCode(filepath.Join(t.TempDir(), "none.db")).sessionKept("ses_1") {
		t.Error("no database")
	}
	other := filepath.Join(t.TempDir(), "other.db")
	if odb, err := sql.Open("sqlite", other); err == nil {
		odb.Exec(`CREATE TABLE thing (id TEXT)`)
		odb.Close()
	}
	if !fakeOpenCode(other).sessionKept("ses_1") {
		t.Error("a database that isn't OpenCode's")
	}

	// Where OpenCode keeps it.
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("OPENCODE_DB", "")
	o = &OpenCode{}
	if p := o.dbPath(); p != filepath.Join(data, "opencode", "opencode.db") {
		t.Errorf("db = %s", p)
	}
	t.Setenv("OPENCODE_DB", "work.db")
	if p := o.dbPath(); p != filepath.Join(data, "opencode", "work.db") {
		t.Errorf("a db named = %s", p)
	}
	t.Setenv("OPENCODE_DB", db)
	if p := o.dbPath(); p != db || !o.sessionKept("ses_1") || o.sessionKept("ses_2") {
		t.Errorf("a db by its path = %s", p)
	}
	// In memory, no session outlives its run.
	t.Setenv("OPENCODE_DB", ":memory:")
	if o.sessionKept("ses_1") {
		t.Error("a session in memory")
	}
}

func TestOpenCodeModels(t *testing.T) {
	stubModelsCLI(t, map[string]string{"/bin/opencode models": "anthropic/claude-haiku-5-5\nanthropic/claude-sonnet-5-5\n\nopenrouter/anthropic/claude-opus-5-5\n"})
	ms, err := fakeOpenCode("").Models(context.Background())
	if err != nil || ids(ms) != "anthropic/claude-haiku-5-5,anthropic/claude-sonnet-5-5,openrouter/anthropic/claude-opus-5-5" ||
		ms[0].Name != "claude-haiku-5-5" || ms[2].Name != "anthropic/claude-opus-5-5" {
		t.Errorf("models = %+v, %v", ms, err)
	}
}

// What OpenCode asks, as the mission's notice says it.
func TestOpenCodeAskedFor(t *testing.T) {
	edit := permission("edit", "/w/c/main.go", map[string]any{"filepath": "/w/c/main.go", "diff": "@@ -1 +1 @@"})
	if got := AskedFor(*edit); got != "edit /w/c/main.go" {
		t.Errorf("an edit = %q", got)
	}
	if got := AskedFor(*permission("execute", "go test ./...", map[string]any{"command": "go test ./..."})); got != "run `go test ./...`" {
		t.Errorf("a command = %q", got)
	}
	outside := permission("other", "Access external directory /etc", map[string]any{"filepath": "/etc/hosts", "parentDir": "/etc"})
	if got := AskedFor(*outside); got != "Access external directory /etc" {
		t.Errorf("outside its folder = %q", got)
	}
}

// fakeOpenCodeAgent answers RunACP as `opencode acp` does (READ in its
// code): its settings as config options, permissions offered once, always
// and reject, and a command's exit code in its output's metadata.
type fakeOpenCodeAgent struct {
	conn *acp.Conn
	mu   sync.Mutex
	got  map[string][]string
}

func (f *fakeOpenCodeAgent) handle(ctx context.Context, method string, params json.RawMessage) (any, error) {
	f.mu.Lock()
	f.got[method] = append(f.got[method], string(params))
	f.mu.Unlock()
	update := func(u map[string]any) {
		f.conn.Notify("session/update", map[string]any{"sessionId": "ses_1", "update": u})
	}
	opt := func(id, cat, cur string, vals ...[2]string) map[string]any {
		var vs []any
		for _, v := range vals {
			vs = append(vs, map[string]any{"value": v[0], "name": v[1]})
		}
		return map[string]any{"id": id, "name": id, "category": cat, "type": "select", "currentValue": cur, "options": vs}
	}
	config := []any{
		opt("model", "model", "opencode/big-pickle", [2]string{"opencode/big-pickle", "OpenCode Zen/Big Pickle"}, [2]string{"anthropic/claude-sonnet-5-5", "Anthropic/Claude Sonnet 5.5"}),
		opt("effort", "thought_level", "default", [2]string{"default", "default"}, [2]string{"low", "low"}, [2]string{"high", "high"}),
		opt("mode", "mode", "build", [2]string{"build", "build"}, [2]string{"plan", "plan"}, [2]string{openCodeHeadless, openCodeHeadless}),
	}
	switch method {
	case "initialize":
		return map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{"loadSession": true,
			"sessionCapabilities": map[string]any{"close": map[string]any{}, "fork": map[string]any{}, "list": map[string]any{}, "resume": map[string]any{}}},
			"authMethods": []any{map[string]any{"id": "opencode-login", "name": "Login with opencode"}}}, nil
	case "session/new", "session/resume":
		return map[string]any{"sessionId": "ses_1", "configOptions": config}, nil
	case "session/set_config_option":
		return map[string]any{"configOptions": config}, nil
	case "session/prompt":
		for i, cmd := range []string{`ws mission note "looked"`, `go test ./...`, `ws status`} {
			id := []string{"call_1", "call_2", "call_3"}[i]
			update(map[string]any{"sessionUpdate": "tool_call", "toolCallId": id, "title": "bash", "kind": "execute", "status": "pending",
				"rawInput": map[string]any{"command": cmd, "description": "Runs it", "cwd": "/w"}})
			req := map[string]any{"sessionId": "ses_1", "toolCall": map[string]any{"toolCallId": id, "status": "pending", "kind": "execute", "title": cmd,
				"rawInput": map[string]any{"command": cmd}, "locations": []any{}},
				"options": []any{
					map[string]any{"optionId": "once", "kind": "allow_once", "name": "Allow once"},
					map[string]any{"optionId": "always", "kind": "allow_always", "name": "Always allow"},
					map[string]any{"optionId": "reject", "kind": "reject_once", "name": "Reject"},
				}}
			var out struct {
				Outcome struct {
					OptionID string `json:"optionId"`
				} `json:"outcome"`
			}
			f.conn.Call(ctx, "session/request_permission", req, &out)
			if out.Outcome.OptionID != "once" {
				// With continue_loop_on_deny, the call fails and the turn
				// goes on.
				msg := "The user rejected permission to use this specific tool call."
				update(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": id, "status": "failed",
					"content": []any{map[string]any{"type": "content", "content": acp.Text(msg)}}, "rawOutput": map[string]any{"error": msg}})
				continue
			}
			exit := 0
			if cmd == "ws status" {
				exit = 1 // a command that fails still ends completed
			}
			update(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": id, "status": "completed",
				"content":   []any{map[string]any{"type": "content", "content": acp.Text("output of " + cmd)}},
				"rawOutput": map[string]any{"output": "output of " + cmd, "metadata": map[string]any{"output": "output of " + cmd, "exit": exit, "truncated": false}}})
		}
		update(map[string]any{"sessionUpdate": "agent_message_chunk", "messageId": "msg_1", "content": acp.Text("Noted; ")})
		update(map[string]any{"sessionUpdate": "agent_message_chunk", "messageId": "msg_1", "content": acp.Text("tests weren't mine to run.")})
		update(map[string]any{"sessionUpdate": "usage_update", "used": 1200, "size": 200000})
		return map[string]any{"stopReason": "end_turn", "_meta": map[string]any{}}, nil
	}
	return nil, &acp.RPCError{Code: acp.CodeMethodNotFound, Message: method}
}

// A headless run on OpenCode: its settings chosen, its session resumed, an
// allowed command let out and another refused, the turn going on.
func TestOpenCodeRunACP(t *testing.T) {
	t.Setenv(EnvOpenCodeBin, "")
	f := &fakeOpenCodeAgent{got: map[string][]string{}}
	f.conn = dialFakeAgent(t, f.handle)
	l, err := fakeOpenCode(openCodeDB(t, "ses_1")).Orchestrate(OrchestratorRun{Prompt: "look", Dir: t.TempDir(), Model: "anthropic/claude-sonnet-5-5[high]"})
	if err != nil {
		t.Fatal(err)
	}
	run := *l.ACP
	run.Load = "ses_1"
	var out, errOut bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := RunACP(ctx, run, &out, &errOut); err != nil {
		t.Fatalf("RunACP = %v (%s)", err, errOut.String())
	}

	f.mu.Lock()
	sets := strings.Join(f.got["session/set_config_option"], "\n")
	resumed, signedIn := len(f.got["session/resume"]), len(f.got["authenticate"])
	f.mu.Unlock()
	for _, want := range []string{`"configId":"mode","sessionId":"ses_1","value":"ws-headless"`, `"configId":"model","sessionId":"ses_1","value":"anthropic/claude-sonnet-5-5"`,
		`"configId":"effort","sessionId":"ses_1","value":"high"`} {
		if !strings.Contains(sets, want) {
			t.Errorf("settings chosen:\n%s\nwant %s", sets, want)
		}
	}
	if resumed != 1 || signedIn != 0 {
		t.Errorf("resumed %d times, signed in %d", resumed, signedIn)
	}

	rep := ReadStream(StreamACP, out.String())
	if rep.Session != "ses_1" || !rep.Done || rep.Failed != "" || rep.Output != "Noted; tests weren't mine to run." {
		t.Errorf("report = %+v\n%s", rep, out.String())
	}
	if len(rep.Calls) != 3 {
		t.Fatalf("calls = %+v", rep.Calls)
	}
	if c := rep.Calls[0]; c.Tool != "Shell" || c.Input != `ws mission note "looked"` || c.Refused || c.Error || c.Result != "output of "+c.Input {
		t.Errorf("a command let out = %+v", c)
	}
	if c := rep.Calls[1]; c.Input != "go test ./..." || !c.Refused || !c.Error {
		t.Errorf("a command refused = %+v", c)
	}
	if c := rep.Calls[2]; c.Input != "ws status" || c.Refused || !c.Error {
		t.Errorf("a command that failed = %+v", c)
	}
}
