package harness

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// What real runs printed (cursor-agent 2026.11.05, pi 1.5.0), asked to run
// cat note.txt, git log, git status and touch, with only the first two
// allowed: the session, each call (the refused one marked), and the reply.
func TestReadStream_Cursor(t *testing.T) {
	data, err := os.ReadFile("testdata/cursor-converse.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	rep := ReadStream(StreamCursor, string(data))
	if rep.Session != "50130cc6-8819-4668-ad6d-272a2bff4842" || !rep.Done || rep.Failed != "" {
		t.Errorf("report = %+v", rep)
	}
	var calls []string
	for _, c := range rep.Calls {
		calls = append(calls, c.Tool+" "+c.Input+map[bool]string{true: " (refused)"}[c.Refused])
	}
	// Started in this order, run at once: they end in another.
	if got := strings.Join(calls, "; "); got != "Shell cat note.txt; Shell git log -1 --oneline; Shell git status (refused); Shell git status (refused)" {
		t.Errorf("calls: %s", got)
	}
	if rep.Calls[0].Result != "hello from a file" {
		t.Errorf("a call's result: %q", rep.Calls[0].Result)
	}
	if !strings.HasPrefix(rep.Output, "(a) `cat note.txt`") || strings.Contains(rep.Output, "Retrying") {
		t.Errorf("the reply should be the last message alone: %q", rep.Output)
	}
}

func TestReadStream_Pi(t *testing.T) {
	data, err := os.ReadFile("testdata/pi-converse.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	rep := ReadStream(StreamPi, string(data))
	if rep.Session != "805847ab-5123-466a-bfd6-5c78f3c330b5" || !rep.Done || rep.Failed != "" {
		t.Errorf("report = %+v", rep)
	}
	var calls []string
	for _, c := range rep.Calls {
		calls = append(calls, c.Tool+" "+c.Input+map[bool]string{true: " (refused)"}[c.Refused])
	}
	if got := strings.Join(calls, "; "); got != "bash cat note.txt; bash git log -1 --oneline; bash git status (refused)" {
		t.Errorf("calls: %s", got)
	}
	if !strings.Contains(rep.Output, "`git status`: refused") {
		t.Errorf("reply: %q", rep.Output)
	}
}

// A run file says how to read it on its first line; one without is
// Claude Code's, as they all were before.
func TestReadRun(t *testing.T) {
	data, _ := os.ReadFile("testdata/pi-converse.jsonl")
	if rep := ReadRun(RunHeader(StreamPi) + string(data)); len(rep.Calls) != 3 {
		t.Errorf("headed: %+v", rep.Calls)
	}
	claude := `{"type":"assistant","message":{"content":[{"type":"text","text":"Looking."}]}}` + "\n"
	if rep := ReadRun(claude); rep.Output != "Looking." {
		t.Errorf("unheaded: %+v", rep)
	}
}

// pi's guard is TypeScript: run it under Node (when it can strip types)
// with a stand-in for pi, against what a thread and a talk may and may not
// run.
func TestPiGuard(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node")
	}
	for _, tc := range []struct {
		run   ConverseRun
		cases map[string]bool
	}{
		{ConverseRun{}, map[string]bool{
			"ws show -f llm t1":                                    true,
			"ws thread suggest t3 --from a --to b":                 true,
			"ws thread suggest t3 <<'EOF'\nold\n=======\nnew\nEOF": true,
			"ws thread suggest t3 <<EOF\n$(touch x)\nEOF":          false, // unquoted: it would run
			"ws thread suggest t3 <<'EOF'\nold\nEOF\ntouch x\nEOF": false, // ends early, then runs
			"ws thread suggest t3 <<'EOF'\nold\nEOF\n":             true,
			"ws mission list":                                      false,
			"touch x":                                              false,
			"ws show; touch x":                                     false,
			"cat a > b":                                            false,
		}},
		{ConverseRun{Talk: true, ReadPaths: []string{"/ws/repos/api/.ground"}}, map[string]bool{
			"ws mission list":                                                       true,
			"git -C /ws/repos/api/.ground log -3":                                   true,
			"git -C /ws/repos/api/.ground push":                                     false,
			"git -C /elsewhere log":                                                 false,
			"git -c core.pager=sh -C /ws/repos/api/.ground log":                     false,
			"ws thread suggest t3 --from a --to b":                                  false,
			"ws mission decide x":                                                   false,
			"ws mission decide --propose \"Key per tenant.\" --why \"Tokens are.\"": true,
			"ws mission new --propose \"Token cache\" --brief-file - <<'EOF'\n## Goal\nCache it.\nEOF": true,
			"ws mission new \"Token cache\"":                                                                       false,
			"ws mission replan --propose \"Add a step for the --shout flag.\"":                                     true,
			"ws mission replan \"Add a step for the --shout flag.\"":                                               false, // asked, not proposed
			"ws mission new --propose x <<EOF\n$(touch y)\nEOF":                                                    false,
			"ws doc add --propose doc - <<'EOF'\n# Todos\n\nWhat you've noticed.\nEOF":                             true,
			"workspace doc add --propose plan notes.md":                                                            true,
			"ws doc add --propose adr":                                                                             true, // shows the kind's guide first
			"ws doc add --propose adr --guide e4fd86180fac - <<'EOF'\n# Key it per tenant\nEOF":                    true,
			"ws doc amend --propose records:adr/0007-one-binary.md":                                                true, // shows it as it is first
			"ws doc amend --propose records:adr/0007-one-binary.md --on 3f2a9c1b04de - <<'EOF'\n# One binary\nEOF": true,
			"ws doc amend records:adr/0007-one-binary.md notes.md":                                                 false, // the user's, not the agent's
			"ws doc add doc - <<'EOF'\n# Todos\nEOF":                                                               false, // added, not proposed
			"ws doc add doc notes.md":                                                                              false,
			"ws doc add --propose doc - <<EOF\n# $(touch y)\nEOF":                                                  false, // unquoted: it would run
			"ws doc add --propose doc - <<'EOF'\n# Todos\nEOF\ngit push\nEOF":                                      false, // ends early, then runs
			"ws doc add --propose doc notes.md; touch y":                                                           false,
			"ws artifact add --propose":                                                                            true, // shows how one's written first
			"ws artifact add --propose --title \"Glow\" --note \"8% in dark\" - <<'EOF'\n<!doctype html>\n<script>if (a < b && c > d) x()</script>\nEOF": true,
			"ws artifact add --propose --to a3 --on v1 --note \"No glow in light\" - <<'EOF'\n<!doctype html>\nEOF":                                      true,
			"ws artifact show a3.v2":                             true,
			"ws artifact add glow.html --note x":                 false, // added, not proposed
			"ws artifact add --propose - <<EOF\n$(touch y)\nEOF": false, // unquoted: it would run
			"ws artifact discard a3":                             false,
		}},
	} {
		dir := t.TempDir()
		guard, err := writePiGuard(dir, converseCommands(tc.run))
		if err != nil {
			t.Fatal(err)
		}
		cases, _ := json.Marshal(tc.cases)
		script := filepath.Join(dir, "run.mts")
		os.WriteFile(script, []byte(`import guard from `+strconvQuote(guard)+`;
let handler;
guard({ on: (_, h) => (handler = h) });
const cases = `+string(cases)+`;
const out = {};
for (const [command] of Object.entries(cases)) out[command] = !(await handler({ toolName: "bash", input: { command } }))?.block;
out["(write)"] = !(await handler({ toolName: "write", input: {} }))?.block;
out["(read)"] = !(await handler({ toolName: "read", input: {} }))?.block;
console.log(JSON.stringify(out));
`), 0o644)
		got, err := exec.Command(node, "--experimental-strip-types", "--no-warnings", script).Output()
		if err != nil {
			t.Skipf("node can't run TypeScript here: %v", err)
		}
		var allowed map[string]bool
		if err := json.Unmarshal(got, &allowed); err != nil {
			t.Fatalf("%v: %s", err, got)
		}
		for command, want := range tc.cases {
			if allowed[command] != want {
				t.Errorf("talk=%v: %q allowed = %v", tc.run.Talk, command, allowed[command])
			}
		}
		if allowed["(write)"] || !allowed["(read)"] {
			t.Errorf("tools: %v", allowed)
		}
	}
}

func strconvQuote(s string) string { data, _ := json.Marshal(s); return string(data) }

// Claude Code's stream-json, as 2.1.319 printed it: a call that ran, one
// refused, and the final message with the refusal listed.
const claudeSample = `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Looking."},{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"ws mission show -f llm","description":"x"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"Mission: cache","is_error":false}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"git -C repos/demo/.ground log -1"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t2","content":"Permission to use Bash has been denied because Claude Code is running in don't ask mode.","is_error":true}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t3","name":"Read","input":{"file_path":"/w/brief.md"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t3","content":[{"type":"text","text":"## Goal"}]}]}}
{"type":"result","subtype":"success","result":"I'd flag it, once Bash is available.","is_error":false,"permission_denials":[{"tool_name":"Bash","tool_use_id":"t2"}]}
`

func TestReadStream_Claude(t *testing.T) {
	rep := ReadStream(StreamClaude, claudeSample)
	if rep.Output != "I'd flag it, once Bash is available." {
		t.Errorf("output = %q", rep.Output)
	}
	if len(rep.Calls) != 3 {
		t.Fatalf("calls = %+v", rep.Calls)
	}
	if c := rep.Calls[0]; c.Tool != "Bash" || c.Input != "ws mission show -f llm" || c.Error || c.Refused || c.Result != "Mission: cache" {
		t.Errorf("ran: %+v", c)
	}
	if c := rep.Calls[1]; !c.Refused || !c.Error || c.Input != "git -C repos/demo/.ground log -1" {
		t.Errorf("refused: %+v", c)
	}
	if c := rep.Calls[2]; c.Tool != "Read" || c.Input != "/w/brief.md" || c.Result != "## Goal" {
		t.Errorf("read: %+v", c)
	}
	// Cut short: no result line, so the last thing it said.
	cut := strings.Join(strings.Split(claudeSample, "\n")[:3], "\n")
	if rep := ReadStream(StreamClaude, cut); rep.Output != "Looking." || len(rep.Calls) != 1 {
		t.Errorf("cut short: %+v", rep)
	}
	if rep := ReadStream("", "plain final message"); rep.Output != "plain final message" || rep.Calls != nil {
		t.Errorf("text: %+v", rep)
	}
}

// What a real run printed with --include-partial-messages (claude 2.1.326,
// haiku, no tools; its init and result lines trimmed): the message's text
// as it's written, then the message whole, then the result. Read part way,
// the draft is the text so far; read whole, the reply is the message once,
// as it is without them.
func TestReadStream_ClaudePartial(t *testing.T) {
	data, err := os.ReadFile("testdata/claude-partial.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	const reply = "Rivers carry snowmelt from mountains to the sea.\nThey shape valleys slowly, one bend at a time.\nTheir currents feed forests, farms, and cities alike.\n\n## end\n- Done"
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	upTo := func(text string) Report {
		for i, l := range lines {
			if strings.Contains(l, text) {
				return ReadStream(StreamClaude, strings.Join(lines[:i+1], "\n"))
			}
		}
		t.Fatalf("no line with %q", text)
		return Report{}
	}
	if rep := upTo(`"message_start"`); rep.Draft != "" || rep.SoFar() != "" || rep.Done || rep.Session != "ebeb4e36-dbbe-412f-b27a-f87336b4c198" {
		t.Errorf("started: %+v", rep)
	}
	if rep := upTo(`"text":" sea.\nThey"`); rep.Draft != "Rivers carry snowmelt from mountains to the sea.\nThey" || rep.Output != "" || rep.SoFar() != rep.Draft {
		t.Errorf("part way: %+v", rep)
	}
	if rep := upTo(`"type":"assistant"`); rep.Draft != "" || rep.Output != reply || rep.Done {
		t.Errorf("the message whole: %+v", rep)
	}
	rep := ReadStream(StreamClaude, string(data))
	if rep.Output != reply || rep.Draft != "" || !rep.Done || rep.Failed != "" || rep.Calls != nil {
		t.Errorf("done: %+v", rep)
	}
	// Each message's draft is its own.
	two := lines[0] + "\n" + `{"type":"stream_event","event":{"type":"message_start","message":{"content":[]}}}` + "\n" +
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"First."}}}` + "\n" +
		`{"type":"assistant","message":{"content":[{"type":"text","text":"First."}]}}` + "\n" +
		`{"type":"stream_event","event":{"type":"message_start","message":{"content":[]}}}` + "\n" +
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Seco"}}}` + "\n" +
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"nd"}},"parent_tool_use_id":"t1"}`
	if rep := ReadStream(StreamClaude, two); rep.Output != "First." || rep.Draft != "Seco" || rep.SoFar() != "Seco" {
		t.Errorf("a second message: %+v", rep)
	}
}

// What the run of a review's guide had written when it was read part way
// (claude 2.1.326, haiku, no tools; its init line trimmed): a thinking
// block, whose deltas are empty, then the guide's text. Read up to each
// point, it's thinking, then writing, then has the message whole.
func TestReadRun_ClaudeGuideAsItGoes(t *testing.T) {
	data, err := os.ReadFile("testdata/claude-guide-run.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	upTo := func(text string, nth int) Report {
		for i, l := range lines {
			if strings.Contains(l, text) {
				if nth--; nth == 0 {
					return ReadRun(strings.Join(lines[:i+1], "\n"))
				}
			}
		}
		t.Fatalf("no line with %q", text)
		return Report{}
	}
	if rep := upTo(`"thinking_delta"`, 1); !rep.Thinking || rep.SoFar() != "" || rep.Done {
		t.Errorf("thinking: %+v", rep)
	}
	if rep := upTo(`"text_delta"`, 3); rep.Thinking || !strings.HasPrefix(rep.Draft, "The greeting moves from `main.go` into `internal/render") || strings.Contains(rep.Draft, "## differs") {
		t.Errorf("writing: %+v", rep)
	}
	rep := ReadRun(string(data))
	if rep.Thinking || rep.Draft != "" || rep.Done || !strings.Contains(rep.Output, "## read:") || rep.SoFar() != rep.Output {
		t.Errorf("the message whole, before the result: %+v", rep)
	}
	if strings.Count(rep.Output, "The greeting moves from") != 1 {
		t.Errorf("said twice: %q", rep.Output)
	}
}
