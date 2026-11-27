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

// stubModelsCLI answers each CLI's model listing as it prints it.
func stubModelsCLI(t *testing.T, out map[string]string) *[]string {
	t.Helper()
	var ran []string
	old := runModelsCLI
	runModelsCLI = func(ctx context.Context, argv []string) ([]byte, error) {
		cmd := strings.Join(argv, " ")
		ran = append(ran, cmd)
		for suffix, o := range out {
			if strings.HasSuffix(cmd, suffix) {
				return []byte(o), nil
			}
		}
		return nil, errors.New("not stubbed: " + cmd)
	}
	t.Cleanup(func() { runModelsCLI = old })
	return &ran
}

func ids(ms []Model) string {
	var out []string
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return strings.Join(out, ",")
}

func TestModels(t *testing.T) {
	stubModelsCLI(t, map[string]string{
		"--list-models": "Available models\n\nauto - Auto (current, default)\nclaude-haiku-5-5-medium - Haiku 5.5 Medium\nclaude-haiku-5-5-high - Haiku 5.5 High\ngpt-5.4-mini - GPT 5.4 Mini\n",
		"cli debug models": `{"models":[
			{"slug":"gpt-6.1-sol","display_name":"GPT-6.1-Sol","description":"Workhorse.","visibility":"list","default_reasoning_level":"low",
			 "supported_reasoning_levels":[{"effort":"low","description":"Fast"},{"effort":"high","description":"Deep"}]},
			{"slug":"codex-auto-review","display_name":"Auto Review","visibility":"hide","supported_reasoning_levels":[]}]}`,
	})
	found := func(name string) (string, error) { return "/bin/" + name, nil }
	cur := &Cursor{lookPath: found}

	ms, err := cur.Models(context.Background())
	if err != nil || ids(ms) != "auto,claude-haiku-5-5-medium,claude-haiku-5-5-high,gpt-5.4-mini" || !ms[0].Default || ms[1].Name != "Haiku 5.5 Medium" {
		t.Errorf("cursor: %+v, %v", ms, err)
	}
	ms, err = (&Codex{lookPath: found}).Models(context.Background())
	if err != nil || ids(ms) != "gpt-6.1-sol,gpt-6.1-sol[low],gpt-6.1-sol[high]" || !strings.Contains(ms[0].About, "Default effort: low") {
		t.Errorf("codex: %+v, %v", ms, err)
	}
	// Each alias, then each at each effort, one model (its label) in a picker.
	ms, _ = (&Claude{}).Models(context.Background())
	if got := ids(ms); !strings.HasPrefix(got, "opus,opus[low],opus[medium],opus[high],opus[xhigh],opus[max],sonnet,") || !strings.HasSuffix(got, ",haiku[max]") ||
		ms[3].Label != "Opus" || ms[3].Effort != "high" || ms[3].Name != "Opus, high" {
		t.Errorf("claude: %s", got)
	}
}

func TestPiModels(t *testing.T) {
	stubModelsCLI(t, map[string]string{"--list-models": "provider    model                     context  max-out  thinking  images\nanthropic   claude-opus-5-5           1M       128K     yes       yes\nopenrouter  ~anthropic/claude-latest  1M       128K     yes       yes\n"})
	ms, err := (&Pi{lookPath: func(n string) (string, error) { return "/bin/" + n, nil }}).Models(context.Background())
	if err != nil || ids(ms) != "anthropic/claude-opus-5-5,openrouter/~anthropic/claude-latest" || ms[0].About != "1M context" {
		t.Errorf("pi: %+v, %v", ms, err)
	}
}

// Cursor's ACP models are a session's model setting's values, as
// session/new gave them (cursor-agent 2026.11.05), each at its one effort.
func TestCursorACPModels(t *testing.T) {
	stubModelsCLI(t, map[string]string{"--list-models": "auto - Auto\nclaude-haiku-5-5-thinking-low - Claude Haiku 5.5 Low\nclaude-haiku-5-5-low - Claude Haiku 5.5 Low No Thinking\ngpt-5.5-extra-high-fast - GPT-5.5 Extra High Fast\n"})
	sessions := t.TempDir()
	os.MkdirAll(filepath.Join(sessions, "s-1"), 0o755)
	os.WriteFile(filepath.Join(sessions, "s-1", "meta.json"), []byte("{}"), 0o644)
	var asked acp.Command
	old := openACPSession
	openACPSession = func(ctx context.Context, cmd acp.Command, auth, dir string) (acp.Session, error) {
		asked = cmd
		if auth != cursorLogin || dir != cmd.Dir {
			t.Errorf("signed in by %q, in %q (agent in %q)", auth, dir, cmd.Dir)
		}
		return acp.Session{ID: "s-1", Config: []acp.ConfigOption{{ID: "mode", Category: "mode"}, {ID: "model", Category: "model", Values: []acp.ConfigValue{
			{Value: "default[]", Name: "Auto"},
			{Value: "claude-haiku-5-5[thinking=true,context=300k,reasoning_effort=high]", Name: "claude-haiku-5-5"},
			{Value: "claude-opus-5-5[context=300k,effort=medium,fast=false]", Name: "claude-opus-5-5"},
			{Value: "gpt-5.5[context=272k,reasoning=extra-high,fast=true]", Name: "gpt-5.5"},
			{Value: "gemini-3-flash[]", Name: "gemini-3-flash"},
		}}}}, nil
	}
	t.Cleanup(func() { openACPSession = old })
	c := &CursorACP{sessions: sessions, cursor: &Cursor{lookPath: func(n string) (string, error) { return "/bin/" + n, nil }}}
	ms, err := c.Models(context.Background())
	if err != nil || ids(ms) != "auto,claude-haiku-5-5,claude-opus-5-5,gpt-5.5,gemini-3-flash" {
		t.Fatalf("models = %s, %v", ids(ms), err)
	}
	if strings.Join(asked.Args, " ") != "/bin/cursor-agent acp" {
		t.Errorf("ran %v", asked.Args)
	}
	if !ms[0].Default || ms[0].Name != "Auto" {
		t.Errorf("auto = %+v", ms[0])
	}
	haiku, opus, gpt, gemini := ms[1], ms[2], ms[3], ms[4]
	if haiku.Effort != "high" || !haiku.Thinking || haiku.Fast || haiku.Vendor != "claude" || haiku.Label != "Claude Haiku 5.5" ||
		haiku.About != "Runs only as: high effort, thinking, 300k context." {
		t.Errorf("haiku = %+v", haiku)
	}
	if gpt.Label != "GPT-5.5" || opus.Label != "Claude Opus 5.5" {
		t.Errorf("labels, from the CLI's list (gpt) or the id (opus): %q, %q", gpt.Label, opus.Label)
	}
	for id, want := range map[string]string{"claude-sonnet-4-6": "Claude Sonnet 4.6", "grok-4.6": "Grok 4.6", "gemini-2.5-flash": "Gemini 2.5 Flash", "glm-5p3-flash": "GLM 5p3 Flash", "claude-haiku-4-5": "Claude Haiku 4.5", "claude-sonnet-4": "Claude Sonnet 4"} {
		if got := idLabel(id); got != want {
			t.Errorf("idLabel(%q) = %q, want %q", id, got, want)
		}
	}
	if opus.Effort != "medium" || opus.Thinking || opus.Fast {
		t.Errorf("opus = %+v", opus)
	}
	if gpt.Effort != "xhigh" || !gpt.Fast {
		t.Errorf("gpt = %+v", gpt)
	}
	if gemini.Effort != "" || gemini.About != "" {
		t.Errorf("gemini = %+v", gemini)
	}
	// What ACP takes back is each one's own value.
	opt := acp.ConfigOption{Values: []acp.ConfigValue{{Value: "claude-haiku-5-5[thinking=true]", Name: "claude-haiku-5-5"}}}
	if opt.Pick(haiku.ID) != "claude-haiku-5-5[thinking=true]" {
		t.Errorf("haiku isn't picked by %q", haiku.ID)
	}
	if _, err := os.Stat(filepath.Join(sessions, "s-1")); !os.IsNotExist(err) {
		t.Errorf("the session asked in is still kept: %v", err)
	}
}
