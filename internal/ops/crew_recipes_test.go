package ops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/prompt"
)

func TestCapsuleRecipes(t *testing.T) {
	dir := t.TempDir()
	if got := capsuleRecipes(dir); got != nil {
		t.Errorf("with no ws.repo.toml: %v", got)
	}
	toml := `[run.service.web]
command = "pnpm dev --port {port}"
description = "The storefront"

[run.task.test]
command = "go test ./..."
`
	if err := os.WriteFile(filepath.Join(dir, config.RepoFileName), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	got := capsuleRecipes(dir)
	want := []prompt.Recipe{
		{Name: "test", Type: "task", Command: "go test ./..."},
		{Name: "web", Type: "service", Command: "pnpm dev --port {port}", Description: "The storefront"},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("recipes: %+v", got)
	}
}
