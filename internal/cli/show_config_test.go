package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/workspace"
)

func TestConfigView(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("ws.toml", "[workspace]\norg = \"acme\"\n\n[repos.web]\n[repos.api]\ngroup = \"core\"\n[repos.notes]\n\n[crew]\nharness = \"claude\"\n")
	write("PROCESS.md", "# How we work\n")
	// web: a JavaScript app with no ws.repo.toml.
	write("repos/web/.ground/package.json", `{"scripts": {"dev": "vite", "test": "vitest"}}`)
	write("repos/web/.ground/pnpm-lock.yaml", "")
	write("repos/web/.ground/.env.example", "API_URL=\n")
	write("repos/web/.ground/docs/intro.md", "# Intro\n")
	// api: configured already.
	write("repos/api/.ground/ws.repo.toml", "[capsule]\nafter_create = \"go mod download\"\n\n[run.service.server]\ncommand = \"go run ./cmd/api\"\n\n[run.task.test]\ncommand = \"go test ./...\"\n")
	write("repos/api/.ground/go.mod", "module api\n")
	// notes isn't cloned.

	ws := &workspace.Workspace{Root: root, RepoNames: []string{"api", "notes", "web"}, RepoGroups: map[string]string{"api": "core"}}
	out := configView(ws).render(false, false)
	for _, want := range []string{
		"- ws.toml (" + filepath.Join(root, "ws.toml") + ") says: workspace, repos (3), crew.",
		"- ws.local.toml isn't there",
		"- Neither file says: [missions], [agents], [groups],",
		"has process PROCESS.md; no guide, skills, PR guide",
		"### Repos (3)",
		"- api (group core): repos/api/.ground",
		"  ws.repo.toml says: capsule, run; services server; tasks test.",
		"  Signs: go.mod.",
		"- notes: not cloned here",
		"- web: repos/web/.ground",
		"  No ws.repo.toml.",
		"  Signs: package.json (scripts: dev, test), pnpm-lock.yaml, .env.example, docs/.",
		"# The guide to setting ws up", // after
		"- `run.task.<name>.inputs`:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ws show config is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Signs: package.json (scripts: dev, test), pnpm-lock.yaml, .env.example, docs/, ") {
		t.Errorf("a sign that isn't there:\n%s", out)
	}
}

func TestPackageScriptsAreCapped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "package.json")
	var b strings.Builder
	b.WriteString(`{"scripts": {`)
	for i := range maxScripts + 3 {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"s` + string(rune('a'+i)) + `": "x"`)
	}
	b.WriteString("}}")
	os.WriteFile(path, []byte(b.String()), 0o644)
	got := packageScripts(path)
	if len(got) != maxScripts+1 || got[maxScripts] != "3 more" {
		t.Errorf("packageScripts = %v", got)
	}
}
