package team

import (
	"strings"
	"testing"
)

func TestTouched(t *testing.T) {
	own := []Watch{
		{Name: "patterns", Title: "Pattern", Paths: []string{"docs:technical/patterns/**"}},
		{Name: "ci", Paths: []string{"build/*.yml"}}, // in place of the built-in
		{Name: "lint"}, // turned off
	}
	watches := Watches(own)
	titles := func(repo string, paths ...string) string {
		var out []string
		for _, touch := range touched(watches, repo, paths) {
			out = append(out, touch.Title+"("+strings.Join(touch.Files, ",")+")")
		}
		return strings.Join(out, " ")
	}
	for _, c := range []struct{ repo, path, want string }{
		{"api", "AGENTS.md", "Guide(AGENTS.md)"},
		{"api", "packages/core/AGENTS.md", "Guide(packages/core/AGENTS.md)"},
		{"api", "docs/build-proposal-template.md", "Guide(docs/build-proposal-template.md)"},
		{"api", ".agents/skills/atomic-repo/SKILL.md", "Skill(.agents/skills/atomic-repo/SKILL.md)"},
		{"api", "ws.repo.toml", "Workspace(ws.repo.toml)"},
		{"api", "src/AGENTS.md.go", ""},
		{"api", ".github/workflows/ci.yml", ""}, // the built-in CI was replaced
		{"api", "build/ci.yml", "Ci(build/ci.yml)"},
		{"api", "build/steps/ci.yml", ""}, // * is within one folder
		{"api", ".eslintrc.json", ""},     // lint is off
		{"docs", "technical/patterns/pagination.md", "Pattern(technical/patterns/pagination.md)"},
		{"api", "technical/patterns/pagination.md", ""}, // only in docs
	} {
		if got := titles(c.repo, c.path); got != c.want {
			t.Errorf("%s:%s touches %q, want %q", c.repo, c.path, got, c.want)
		}
	}
	// With nothing of the workspace's own, the built-ins.
	if got := touched(Watches(nil), "api", []string{".github/workflows/ci.yml", ".eslintrc.json", "main.go"}); len(got) != 2 || got[0].Title != "CI" || got[1].Title != "Lint" {
		t.Errorf("built-ins = %+v", got)
	}
}

func TestTouches(t *testing.T) {
	w := newWorld(t)
	w.commit("Ana Diaz", "ana@acme.example", "docs: say how to land", at(9, 9), "AGENTS.md")
	sha := w.commit("Cal Reyes", "cal@acme.example", "ci: cache the build (#4)", at(9, 10), ".github/workflows/ci.yml")
	w.commit("Cal Reyes", "cal@acme.example", "fix: a bug", at(9, 11), "main.go")
	w.svc.GitHub = nil
	snap, err := w.svc.Look(false, w.now.AddDate(0, 0, -3))
	if err != nil {
		t.Fatal(err)
	}
	by := map[string][]Touch{}
	for _, d := range w.svc.Timeline(snap, at(9, 0)).Days {
		for _, it := range d.Items {
			by[it.Name] = it.Touches
		}
	}
	if got := by["Say how to land"]; len(got) != 1 || got[0].Name != "guides" || got[0].Files[0] != "AGENTS.md" {
		t.Errorf("the guide's change = %+v", got)
	}
	if got := by["Cache the build"]; len(got) != 1 || got[0].Title != "CI" {
		t.Errorf("the workflow's change = %+v", got)
	}
	if got := by["A bug"]; len(got) != 0 {
		t.Errorf("a fix touches %+v", got)
	}
	// An agent reading it is told.
	ask := w.svc.asked(Subject{Repo: "api", Of: OfLanded, Commits: []string{sha}}, Material{Files: []FileChange{{Path: ".github/workflows/ci.yml"}}})
	if len(ask.Touches) != 1 || ask.Touches[0].Title != "CI" {
		t.Errorf("asked with %+v", ask.Touches)
	}
	// In a change across repos, a file says which it's in.
	ask = w.svc.asked(Subject{Parts: []Subject{{Repo: "api"}, {Repo: "bff"}}}, Material{Files: []FileChange{{Repo: "bff", Path: "AGENTS.md"}}})
	if len(ask.Touches) != 1 || ask.Touches[0].Files[0] != "bff:AGENTS.md" {
		t.Errorf("across repos, asked with %+v", ask.Touches)
	}
}
