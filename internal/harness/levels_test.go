package harness

import "testing"

func TestForLevel(t *testing.T) {
	t.Setenv(EnvDefault, "")
	claude := Roles{Crew: Pick{Harness: "claude"}}
	for level, want := range map[string]Pick{
		"rote":   {Harness: "claude", Model: ClaudeSmallModel},
		"normal": {Harness: "claude"},
		"hard":   {Harness: "claude"},
	} {
		if got := claude.ForLevel(level); got != want {
			t.Errorf("claude %s = %v, want %v", level, got, want)
		}
	}
	set := Roles{Crew: Pick{Harness: "claude", Model: "sonnet"}, CrewLevels: map[string]string{"rote": "cursor:auto", "hard": "opus"}}
	if got := set.ForLevel("rote"); got != (Pick{Harness: "cursor", Model: "auto"}) {
		t.Errorf("set rote = %v", got)
	}
	if got := set.ForLevel("hard"); got != (Pick{Harness: "claude", Model: "opus"}) {
		t.Errorf("set hard = %v", got)
	}
	if got := set.ForLevel("normal"); got != (Pick{Harness: "claude", Model: "sonnet"}) {
		t.Errorf("set normal = %v", got)
	}
	picks := set.CrewPicks()
	for _, p := range []Pick{{Harness: "cursor", Model: "auto"}, {Harness: "claude", Model: "opus"}} {
		if !set.AllowsCrew(p) {
			t.Errorf("level pick %v not allowed (picks %v)", p, picks)
		}
	}
	pi := Roles{Crew: Pick{Harness: "pi", Model: "sonnet"}}
	if got := pi.ForLevel("rote"); got != (Pick{Harness: "pi", Model: "sonnet"}) {
		t.Errorf("pi rote = %v, want crew's own", got)
	}
}
