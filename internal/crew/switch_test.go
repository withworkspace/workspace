package crew

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

type typed struct{ keys []uv.KeyPressEvent }

func (t *typed) RenderScrolled(int) string  { return "" }
func (t *typed) SendKey(k uv.KeyPressEvent) { t.keys = append(t.keys, k) }
func (t *typed) Paste(string)               { panic("pasted: a command is typed") }

// lines are what was typed, a line for each Enter.
func (t *typed) lines() []string {
	var out []string
	var line strings.Builder
	for _, e := range t.keys {
		if e.Code == uv.KeyEnter {
			out = append(out, line.String())
			line.Reset()
			continue
		}
		line.WriteString(e.Text)
	}
	return out
}

// A terminal crew's model is switched by its harness's commands, each
// typed in key by key and submitted: Claude Code's /model, then its
// /effort (auto, the model's own, where the name gives none). A harness
// with none says so and types nothing.
func TestTermRunSwitchModel(t *testing.T) {
	k := &typed{}
	r := TermRun{S: Spec{Slug: "m", CrewID: "c1", Harness: "claude"}, T: k, Root: t.TempDir()}
	said, err := r.SwitchModel("opus")
	if err != nil || !strings.Contains(said, "opus") {
		t.Fatalf("%q, %v", said, err)
	}
	if got := strings.Join(k.lines(), "|"); got != "/model opus|/effort auto" || k.keys[6].Code != uv.KeySpace {
		t.Errorf("typed %q", got)
	}
	k = &typed{}
	r.T = k
	if _, err := r.SwitchModel("haiku[high]"); err != nil || strings.Join(k.lines(), "|") != "/model haiku|/effort high" {
		t.Errorf("haiku[high]: typed %q, %v", k.lines(), err)
	}
	k = &typed{}
	r.T = k
	if _, err := r.SwitchModel("haiku[loud]"); err == nil || len(k.keys) > 0 {
		t.Errorf("an effort Claude Code doesn't take: %v, typed %d keys", err, len(k.keys))
	}

	k = &typed{}
	r = TermRun{S: Spec{Slug: "m", CrewID: "c2", Harness: "shell"}, T: k, Root: t.TempDir()}
	if _, err := r.SwitchModel("opus"); err == nil || len(k.keys) > 0 {
		t.Errorf("shell: %v, typed %d keys", err, len(k.keys))
	}
}
