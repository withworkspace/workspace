package harness

import (
	"context"
	"os"
	"testing"
)

func TestParseModelName(t *testing.T) {
	for id, want := range map[string]modelName{
		"claude-opus-5-5-high-fast":         {Vendor: "claude", Family: "opus", Version: "5.5", Effort: "high", Fast: true},
		"claude-opus-5-5-medium":            {Vendor: "claude", Family: "opus", Version: "5.5", Effort: "medium"},
		"claude-4.6-sonnet-medium-thinking": {Vendor: "claude", Family: "sonnet", Version: "4.6", Effort: "medium", Thinking: true},
		"claude-opus-4-7-thinking-low":      {Vendor: "claude", Family: "opus", Version: "4.7", Effort: "low", Thinking: true},
		"claude-4-sonnet":                   {Vendor: "claude", Family: "sonnet", Version: "4"},
		"gpt-5.6-sol-xhigh-fast":            {Vendor: "gpt", Family: "sol", Version: "5.6", Effort: "xhigh", Fast: true},
		"gpt-5.5-extra-high":                {Vendor: "gpt", Version: "5.5", Effort: "xhigh"},
		"gpt-5.3-codex-fast":                {Vendor: "gpt", Family: "codex", Version: "5.3", Fast: true},
		"gpt-5.1":                           {Vendor: "gpt", Version: "5.1"},
		"cursor-grok-4.6-high":              {Vendor: "grok", Version: "4.6", Effort: "high"},
		"gemini-3.8-flash-low":              {Vendor: "gemini", Family: "flash", Version: "3.8", Effort: "low"},
		"muse-spark-1.3-minimal":            {Vendor: "muse", Family: "spark", Version: "1.3", Effort: "minimal"},
		"kimi-k2.7-code":                    {Vendor: "kimi", Family: "code", Version: "k2.7"},
		"deepseek-v41-flash-1017-none":      {Vendor: "deepseek", Family: "flash-1017", Version: "v41", Effort: "none"},
		"composer-2.5-fast":                 {Vendor: "composer", Version: "2.5", Fast: true},
	} {
		got, ok := parseModelName(id)
		if !ok || got != want {
			t.Errorf("parseModelName(%q) = %+v, %v; want %+v", id, got, ok, want)
		}
	}
	for _, id := range []string{"auto", "", "opus", "fast"} {
		if got, ok := parseModelName(id); ok {
			t.Errorf("parseModelName(%q) = %+v, want none", id, got)
		}
	}
}

func TestModelLabel(t *testing.T) {
	for name, want := range map[string]string{
		"Claude Opus 5.5 1M High Fast":              "Claude Opus 5.5",
		"Claude Opus 5.5 1M":                        "Claude Opus 5.5",
		"Claude Opus 5 1M Extra High Thinking Fast": "Claude Opus 5",
		"Claude Haiku 5.5  Low No Thinking":         "Claude Haiku 5.5",
		"Grok 4.7  Low Fast\u200b\u200b":            "Grok 4.7",
		"Kimi K3":                                   "Kimi K3",
	} {
		if got := modelLabel(name); got != want {
			t.Errorf("modelLabel(%q) = %q, want %q", name, got, want)
		}
	}
}

// Cursor's whole list, as its CLI gave it on 2026-11-25: every name but
// "auto" is placed, and each model's names give it one label.
func TestCursorListPlaced(t *testing.T) {
	list, err := os.ReadFile("testdata/cursor-list-models.txt")
	if err != nil {
		t.Fatal(err)
	}
	stubModelsCLI(t, map[string]string{"--list-models": string(list)})
	ms, err := (&Cursor{lookPath: func(n string) (string, error) { return "/bin/" + n, nil }}).Models(context.Background())
	if err != nil || len(ms) < 200 {
		t.Fatalf("%d models, %v", len(ms), err)
	}
	labels := map[string]map[string]bool{}
	for _, m := range ms {
		if m.Vendor == "" {
			if m.ID != "auto" {
				t.Errorf("%s isn't placed", m.ID)
			}
			continue
		}
		key := m.Vendor + "/" + m.Family + "/" + m.Version
		if labels[key] == nil {
			labels[key] = map[string]bool{}
		}
		labels[key][m.Label] = true
	}
	for key, ls := range labels {
		if len(ls) != 1 {
			t.Errorf("%s has labels %v", key, ls)
		}
	}
}
