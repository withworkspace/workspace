package harness

import (
	"context"
	"errors"
	"strings"
	"testing"
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
	ms, err = (&CursorACP{cursor: cur}).Models(context.Background())
	if err != nil || ids(ms) != "auto,claude-haiku-5-5,gpt-5.4-mini" || ms[1].Name != "claude-haiku-5-5" {
		t.Errorf("cursor-acp: %+v, %v", ms, err)
	}
	ms, err = (&Codex{lookPath: found}).Models(context.Background())
	if err != nil || ids(ms) != "gpt-6.1-sol,gpt-6.1-sol[low],gpt-6.1-sol[high]" || !strings.Contains(ms[0].About, "default effort: low") {
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
