package harness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/acp"
)

// cursorTitle is a permission's title as Cursor's ACP agent writes one for
// a shell command: in backticks, each backtick in it escaped.
func cursorTitle(cmd string) string {
	return "`" + strings.ReplaceAll(cmd, "`", "\\`") + "`"
}

// The policy judges the command Cursor runs, not its escaped title: a
// substitution hidden by the escaping is refused.
func TestCommandPolicyCursorTitles(t *testing.T) {
	policy := CommandPolicy([]string{"ws mission"})
	for cmd, allowed := range map[string]bool{
		`ws mission note "done"`:         true,
		"ws mission note \"`rm -rf ~`\"": false,
		"ws mission note '`rm -rf ~`'":   true, // single quotes: literal
		"ws mission note `id`":           false,
		"ls":                             false,
	} {
		r := permission("execute", cursorTitle(cmd), nil)
		r.Options = []acp.PermissionOption{
			{OptionID: "allow-once", Name: "Allow once", Kind: acp.AllowOnce},
			{OptionID: "allow-always", Name: "Allow always", Kind: acp.AllowAlways},
			{OptionID: "reject-once", Name: "Reject", Kind: acp.RejectOnce},
		}
		ans, _ := policy(acp.Ask{Permission: r})
		if got := ans.Option == "allow-once"; got != allowed {
			t.Errorf("%s: answered %+v, want allowed %v", cmd, ans, allowed)
		}
	}
	// Cursor's question tool, asked as a permission when ws doesn't offer
	// it (kind other, its own skip option): skipped.
	q := &acp.PermissionRequest{Options: []acp.PermissionOption{
		{OptionID: "staging", Name: "Staging", Kind: acp.AllowOnce},
		{OptionID: "__ask_question_skip__", Name: "Skip", Kind: acp.RejectOnce},
	}}
	title := "Which environment?"
	q.ToolCall.Kind, q.ToolCall.Title = "other", &title
	if ans, _ := policy(acp.Ask{Permission: q}); ans.Option != "__ask_question_skip__" {
		t.Errorf("a question = %+v, want skipped", ans)
	}
	// In agent mode, from a folder of ws's own: a read outside it (asked
	// where a read boundary is set) is allowed, a write outside it refused.
	for title, allowed := range map[string]bool{"Read `/w/main.go`": true, "Edit `/w/main.go`": false, "Write /w/new.go": false} {
		kind := "read"
		if !strings.HasPrefix(title, "Read") {
			kind = "edit"
		}
		r := permission(kind, title, nil)
		r.Options = []acp.PermissionOption{
			{OptionID: "allow-once", Name: "Allow once", Kind: acp.AllowOnce},
			{OptionID: "reject-once", Name: "Reject", Kind: acp.RejectOnce},
		}
		if ans, _ := policy(acp.Ask{Permission: r}); (ans.Option == "allow-once") != allowed {
			t.Errorf("%s: answered %+v, want allowed %v", title, ans, allowed)
		}
	}
}

func TestCursorWorksIn(t *testing.T) {
	if got := cursorWorksIn("/w/.ws/converse/cursor-acp", "/w/c"); !strings.Contains(got, "the folder you work on is /w/c") || !strings.HasSuffix(got, "\n\n") {
		t.Errorf("cursorWorksIn = %q", got)
	}
	for _, dir := range []string{"", "/s"} {
		if got := cursorWorksIn("/s", dir); got != "" {
			t.Errorf("cursorWorksIn(/s, %q) = %q, want nothing", dir, got)
		}
	}
}

func TestCursorACPModel(t *testing.T) {
	for in, want := range map[string]string{
		CursorOrchestratorModel:     "claude-haiku-5-5",
		CursorConverseModel:         "claude-sonnet-5-5",
		"claude-opus-5-5-xhigh":     "claude-opus-5-5",
		"claude-opus-5-5-high-fast": "claude-opus-5-5",
		"gpt-5.3-codex-fast":        "gpt-5.3-codex",
		"gpt-5.5-extra-high":        "gpt-5.5",
		"auto":                      "auto",
		"gpt-5.4-mini":              "gpt-5.4-mini",
		"gemini-3.8-flash":          "gemini-3.8-flash",
		" claude-sonnet-5-5-low ":   "claude-sonnet-5-5",
		"":                          "",
	} {
		if got := cursorACPModel(in); got != want {
			t.Errorf("cursorACPModel(%q) = %q, want %q", in, got, want)
		}
	}
	// Each finds its value among Cursor's (as session/new gave them).
	opt := acp.ConfigOption{Values: []acp.ConfigValue{
		{Value: "default[]", Name: "Auto"},
		{Value: "claude-haiku-5-5[thinking=true,context=300k,reasoning_effort=high]", Name: "claude-haiku-5-5"},
		{Value: "claude-sonnet-5-5[context=300k,reasoning_effort=high]", Name: "claude-sonnet-5-5"},
	}}
	for in, want := range map[string]string{CursorOrchestratorModel: "claude-haiku-5-5[", CursorConverseModel: "claude-sonnet-5-5[", "auto": "default[]"} {
		if got := opt.Pick(cursorACPModel(in)); !strings.HasPrefix(got, want) {
			t.Errorf("%s picks %q", in, got)
		}
	}
}

func TestCursorACPLaunches(t *testing.T) {
	sessions := t.TempDir()
	os.MkdirAll(filepath.Join(sessions, "ran-1"), 0o755)
	os.WriteFile(filepath.Join(sessions, "ran-1", "store.db"), nil, 0o644)
	os.MkdirAll(filepath.Join(sessions, "refused-1"), 0o755) // meta.json alone: no turn ran
	c := &CursorACP{sessions: sessions, cursor: &Cursor{lookPath: func(name string) (string, error) {
		if name == "cursor-agent" {
			return "/bin/cursor-agent", nil
		}
		return "", errors.New("not found")
	}, run: func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		return nil, errors.New("not run")
	}}}
	scratch := t.TempDir()

	l, err := c.Converse(ConverseRun{Prompt: "hi", Dir: "/w", Session: "ran-1", Scratch: scratch, Model: CursorConverseModel})
	if err != nil {
		t.Fatal(err)
	}
	r := *l.ACP
	// Agent mode, so it can run ws's --propose commands (ask mode runs no
	// command but in a read-only sandbox), from a folder of ws's own, as
	// Cursor writes in its session's folder without asking: told where
	// it works.
	runDir := filepath.Join(scratch, "cursor-acp")
	if l.Stream != StreamACP || strings.Join(r.Agent, " ") != "/bin/cursor-agent acp" || r.Mode != "agent" || r.Model != "claude-sonnet-5-5" ||
		r.Load != "ran-1" || !strings.HasSuffix(r.Prompt, "\n\nhi") || !strings.Contains(r.Prompt, "the folder you work on is /w") ||
		r.Settings != nil || !commandAllowed("ws thread suggest t1", r.Allow) || r.Dir != runDir || l.Dir != runDir ||
		r.Auth != "cursor_login" || !strings.Contains(string(r.Answers["cursor/ask_question"]), `"skipped"`) {
		t.Errorf("conversation run = %+v", r)
	}
	if fi, err := os.Stat(runDir); err != nil || !fi.IsDir() {
		t.Errorf("run folder: %v", err)
	}
	talk, err := c.Converse(ConverseRun{Prompt: "brief it", Dir: "/w", Talk: true, Scratch: scratch})
	if err != nil || talk.ACP.Mode != "agent" || !commandAllowed("ws mission brief --propose m1", talk.ACP.Allow) {
		t.Errorf("talk run = %+v, %v", talk.ACP, err)
	}
	if _, err := c.Converse(ConverseRun{Prompt: "hi", Dir: "/w"}); err == nil {
		t.Error("a conversation with no scratch folder ran")
	}
	for _, id := range []string{"refused-1", "gone-1", "../x"} {
		if _, err := c.Converse(ConverseRun{Prompt: "hi", Dir: "/w", Session: id, Scratch: scratch}); !errors.Is(err, ErrSessionGone) {
			t.Errorf("session %s = %v, want ErrSessionGone", id, err)
		}
	}

	l, err = c.Orchestrate(OrchestratorRun{Prompt: "go", Dir: "/w", Scratch: scratch, Model: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	r = *l.ACP
	if r.Mode != "agent" || r.Model != "auto" || r.Dir != runDir || !strings.Contains(r.Prompt, "the folder you work on is /w") ||
		!commandAllowed("ws mission note x", r.Allow) || commandAllowed("ws burn a/b", r.Allow) {
		t.Errorf("orchestrator run = %+v", r)
	}
	l, err = c.Start(Brief{Slug: "m", Title: "M", Dir: "/w/c", Model: CursorOrchestratorModel})
	if err != nil || l.ACP == nil || l.ACP.Mode != "agent" || l.ACP.Model != "claude-haiku-5-5" || l.ACP.Auth != "cursor_login" ||
		strings.Join(l.ACP.Ask, ",") != "cursor/ask_question" || l.ACP.Answers["cursor/ask_question"] != nil {
		t.Errorf("crew = %+v, %v", l.ACP, err)
	}
	if l, err := c.Resume("/w/c", "ran-1"); err != nil || l.ACP.Load != "ran-1" {
		t.Errorf("resume = %+v, %v", l, err)
	}
	if _, err := c.Resume("/w/c", "refused-1"); !errors.Is(err, ErrNoResume) {
		t.Errorf("resume a session Cursor can't load = %v", err)
	}
	// With no cursor of its own, it uses the cursor harness's.
	if cur, err := (&CursorACP{}).cli(); err != nil || cur != registry["cursor"] {
		t.Errorf("cli = %v, %v", cur, err)
	}
}

// A turn Cursor couldn't finish ends as one it did, and only its last
// words say so: an error it names retriable. Anything else is a turn's
// end.
func TestCursorACPTurnDropped(t *testing.T) {
	c := &CursorACP{}
	for _, tt := range []struct {
		name string
		turn acp.Turn
		why  string
	}{
		{"the error alone, after a call", acp.Turn{Stop: "end_turn", Reply: "\n\nError: RetriableError: the stream was closed"}, "RetriableError: the stream was closed"},
		{"the error after what it was saying", acp.Turn{Stop: "end_turn", Reply: "Running the suite.\n\nError: RetriableError: [canceled] reset\n"}, "RetriableError: [canceled] reset"},
		{"a reply that ends on its own words", acp.Turn{Stop: "end_turn", Reply: "The suite passes."}, ""},
		{"the error quoted, then more", acp.Turn{Stop: "end_turn", Reply: "It printed:\nError: RetriableError: reset\nso I ran it again, and it passed."}, ""},
		{"another error", acp.Turn{Stop: "end_turn", Reply: "Error: the file isn't there"}, ""},
		{"a turn that was stopped", acp.Turn{Stop: "cancelled", Reply: "Error: RetriableError: reset"}, ""},
		{"nothing said", acp.Turn{Stop: "end_turn"}, ""},
	} {
		why, dropped := c.TurnDropped(tt.turn)
		if why != tt.why || dropped != (tt.why != "") {
			t.Errorf("%s: TurnDropped = %q, %v", tt.name, why, dropped)
		}
	}
	if _, dropped := TurnDropped("codex", acp.Turn{Stop: "end_turn", Reply: "Error: RetriableError: reset"}); dropped {
		t.Error("a harness that can't tell said a turn was dropped")
	}
	// The call it was cut off in isn't work left running.
	cut := acp.Turn{Stop: "end_turn", Reply: "Error: RetriableError: reset",
		Unfinished: []acp.ToolCall{{ID: "d1", Kind: "execute", Title: "`just test`", Status: acp.StatusInProgress}}}
	if p := AtTurnEnd("cursor-acp", cut, nil); p.Known || len(p.Work) != 0 {
		t.Errorf("AtTurnEnd of a dropped turn = %+v", p)
	}
}
