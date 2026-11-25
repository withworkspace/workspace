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

// A terminal crew's model is switched by its harness's command, typed in
// key by key and submitted; a harness with none says so and types
// nothing.
func TestTermRunSwitchModel(t *testing.T) {
	k := &typed{}
	r := TermRun{S: Spec{Slug: "m", CrewID: "c1", Harness: "claude"}, T: k, Root: t.TempDir()}
	said, err := r.SwitchModel("opus")
	if err != nil || !strings.Contains(said, "opus") {
		t.Fatalf("%q, %v", said, err)
	}
	var line strings.Builder
	for _, e := range k.keys[:len(k.keys)-1] {
		line.WriteString(e.Text)
	}
	if line.String() != "/model opus" || k.keys[len(k.keys)-1].Code != uv.KeyEnter || k.keys[6].Code != uv.KeySpace {
		t.Errorf("typed %q, then %+v", line.String(), k.keys[len(k.keys)-1])
	}

	k = &typed{}
	r = TermRun{S: Spec{Slug: "m", CrewID: "c2", Harness: "shell"}, T: k, Root: t.TempDir()}
	if _, err := r.SwitchModel("opus"); err == nil || len(k.keys) > 0 {
		t.Errorf("shell: %v, typed %d keys", err, len(k.keys))
	}
}
