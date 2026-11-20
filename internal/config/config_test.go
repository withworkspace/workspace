package config

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestParse(t *testing.T) {
	content := `
[workspace]
org = "my-org"
default_branch = "main"

[repos]
repo-a = {}
repo-b = {}
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Workspace.Org != "my-org" {
		t.Errorf("org = %q, want %q", cfg.Workspace.Org, "my-org")
	}
	if cfg.Workspace.DefaultBranch != "main" {
		t.Errorf("default_branch = %q, want %q", cfg.Workspace.DefaultBranch, "main")
	}
	if len(cfg.Repos) != 2 {
		t.Fatalf("repos count = %d, want 2", len(cfg.Repos))
	}
	if _, ok := cfg.Repos["repo-a"]; !ok {
		t.Error("missing repo-a")
	}
}

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte("[workspace]\norg=\"x\"\ndefault_branch=\"main\"\n[repos]\n"), 0644)
	nested := filepath.Join(root, "repos", "my-repo", "main", "src")
	os.MkdirAll(nested, 0755)

	found, err := Discover(nested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != root {
		t.Errorf("discovered %q, want %q", found, root)
	}
}

func TestParseDisplayNameAndAliases(t *testing.T) {
	content := `
[workspace]
org = "my-org"
default_branch = "main"

[repos]
backend = { display_name = "API Server", aliases = ["api", "be"] }
simple-repo = {}
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repo := cfg.Repos["backend"]
	if repo.DisplayName != "API Server" {
		t.Errorf("display_name = %q, want %q", repo.DisplayName, "API Server")
	}
	if len(repo.Aliases) != 2 || repo.Aliases[0] != "api" || repo.Aliases[1] != "be" {
		t.Errorf("aliases = %v, want [api be]", repo.Aliases)
	}

	simple := cfg.Repos["simple-repo"]
	if simple.DisplayName != "" {
		t.Errorf("simple-repo display_name = %q, want empty", simple.DisplayName)
	}
	if len(simple.Aliases) != 0 {
		t.Errorf("simple-repo aliases = %v, want empty", simple.Aliases)
	}
}

func TestParseWorkspaceDisplayName(t *testing.T) {
	content := `
[workspace]
org = "my-org"
default_branch = "main"
display_name = "My Workspace"

[repos]
repo-a = {}
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Workspace.DisplayName != "My Workspace" {
		t.Errorf("display_name = %q, want %q", cfg.Workspace.DisplayName, "My Workspace")
	}
}

func TestParseWorkspaceDisplayNameOptional(t *testing.T) {
	content := `
[workspace]
org = "my-org"
default_branch = "main"

[repos]
repo-a = {}
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Workspace.DisplayName != "" {
		t.Errorf("display_name = %q, want empty", cfg.Workspace.DisplayName)
	}
}

func TestParseColor(t *testing.T) {
	content := `
[workspace]
org = "my-org"
default_branch = "main"

[repos]
repo-a = { color = "#ff5f87" }
repo-b = { color = "168" }
repo-c = {}
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.toml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Repos["repo-a"].Color != "#ff5f87" {
		t.Errorf("repo-a color = %q, want %q", cfg.Repos["repo-a"].Color, "#ff5f87")
	}
	if cfg.Repos["repo-b"].Color != "168" {
		t.Errorf("repo-b color = %q, want %q", cfg.Repos["repo-b"].Color, "168")
	}
	if cfg.Repos["repo-c"].Color != "" {
		t.Errorf("repo-c color = %q, want empty", cfg.Repos["repo-c"].Color)
	}
}

func TestDiscoverNotFound(t *testing.T) {
	tmp := t.TempDir()
	_, err := Discover(tmp)
	if err == nil {
		t.Error("expected error when ws.toml not found")
	}
}

func TestLoad(t *testing.T) {
	root := t.TempDir()
	content := `[workspace]
org = "test-org"
default_branch = "main"

[repos]
repo-a = {}
`
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(content), 0644)

	// Create a nested directory to load from
	nested := filepath.Join(root, "repos", "repo-a", "main")
	os.MkdirAll(nested, 0755)

	cfg, foundRoot, err := Load(nested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if foundRoot != root {
		t.Errorf("root = %q, want %q", foundRoot, root)
	}
	if cfg.Workspace.Org != "test-org" {
		t.Errorf("org = %q, want %q", cfg.Workspace.Org, "test-org")
	}
}

func TestLoad_NotFound(t *testing.T) {
	tmp := t.TempDir()
	_, _, err := Load(tmp)
	if err == nil {
		t.Error("expected error when ws.toml not found")
	}
}

func TestMerge_AppendsAliases(t *testing.T) {
	base := &Config{
		Repos: map[string]RepoConfig{
			"repo-a": {Aliases: []string{"a", "alpha"}},
		},
	}
	local := &Config{
		Repos: map[string]RepoConfig{
			"repo-a": {Aliases: []string{"my-a"}},
		},
	}
	merged := Merge(base, local)
	got := merged.Repos["repo-a"].Aliases
	want := []string{"a", "alpha", "my-a"}
	if len(got) != len(want) {
		t.Fatalf("aliases = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("aliases[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Verify base was not mutated
	if len(base.Repos["repo-a"].Aliases) != 2 {
		t.Errorf("base was mutated: aliases = %v", base.Repos["repo-a"].Aliases)
	}
}

func TestMerge_ReplacesAfterCreate(t *testing.T) {
	base := &Config{
		Repos: map[string]RepoConfig{
			"repo-a": {AfterCreate: "npm install"},
		},
	}
	local := &Config{
		Repos: map[string]RepoConfig{
			"repo-a": {AfterCreate: "bun install"},
		},
	}
	merged := Merge(base, local)
	if got := merged.Repos["repo-a"].AfterCreate; got != "bun install" {
		t.Errorf("after_create = %q, want %q", got, "bun install")
	}
}

func TestMerge_ReplacesColor(t *testing.T) {
	base := &Config{
		Repos: map[string]RepoConfig{
			"repo-a": {Color: "#ff0000"},
		},
	}
	local := &Config{
		Repos: map[string]RepoConfig{
			"repo-a": {Color: "#00ff00"},
		},
	}
	merged := Merge(base, local)
	if got := merged.Repos["repo-a"].Color; got != "#00ff00" {
		t.Errorf("color = %q, want %q", got, "#00ff00")
	}
}

func TestMerge_ReplacesDisplayName(t *testing.T) {
	base := &Config{
		Repos: map[string]RepoConfig{
			"repo-a": {DisplayName: "Repo A"},
		},
	}
	local := &Config{
		Repos: map[string]RepoConfig{
			"repo-a": {DisplayName: "My Repo A"},
		},
	}
	merged := Merge(base, local)
	if got := merged.Repos["repo-a"].DisplayName; got != "My Repo A" {
		t.Errorf("display_name = %q, want %q", got, "My Repo A")
	}
}

func TestMerge_IgnoresUnknownRepos(t *testing.T) {
	base := &Config{
		Repos: map[string]RepoConfig{
			"repo-a": {Color: "#ff0000"},
		},
	}
	local := &Config{
		Repos: map[string]RepoConfig{
			"repo-a":   {Color: "#00ff00"},
			"repo-new": {Color: "#0000ff", AfterCreate: "make"},
		},
	}
	merged := Merge(base, local)
	if _, ok := merged.Repos["repo-new"]; ok {
		t.Error("unknown repo 'repo-new' should not appear in merged config")
	}
	if len(merged.Repos) != 1 {
		t.Errorf("repos count = %d, want 1", len(merged.Repos))
	}
}

func TestMerge_EmptyFieldsNoOverride(t *testing.T) {
	base := &Config{
		Repos: map[string]RepoConfig{
			"repo-a": {
				DisplayName: "Original",
				Color:       "#ff0000",
				AfterCreate: "npm install",
				Aliases:     []string{"a"},
			},
		},
	}
	local := &Config{
		Repos: map[string]RepoConfig{
			"repo-a": {}, // all fields empty/zero
		},
	}
	merged := Merge(base, local)
	repo := merged.Repos["repo-a"]
	if repo.DisplayName != "Original" {
		t.Errorf("display_name = %q, want %q", repo.DisplayName, "Original")
	}
	if repo.Color != "#ff0000" {
		t.Errorf("color = %q, want %q", repo.Color, "#ff0000")
	}
	if repo.AfterCreate != "npm install" {
		t.Errorf("after_create = %q, want %q", repo.AfterCreate, "npm install")
	}
	if len(repo.Aliases) != 1 || repo.Aliases[0] != "a" {
		t.Errorf("aliases = %v, want [a]", repo.Aliases)
	}
}

func TestLoad_WithLocalConfig(t *testing.T) {
	root := t.TempDir()
	base := `[workspace]
org = "test-org"
default_branch = "main"

[repos.repo-a]
color = "#ff0000"
after_create = "npm install"
aliases = ["a"]
`
	local := `[repos.repo-a]
color = "#00ff00"
after_create = "bun install"
aliases = ["my-a"]
`
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(base), 0644)
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(local), 0644)

	cfg, foundRoot, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if foundRoot != root {
		t.Errorf("root = %q, want %q", foundRoot, root)
	}
	repo := cfg.Repos["repo-a"]
	if repo.Color != "#00ff00" {
		t.Errorf("color = %q, want %q", repo.Color, "#00ff00")
	}
	if repo.AfterCreate != "bun install" {
		t.Errorf("after_create = %q, want %q", repo.AfterCreate, "bun install")
	}
	want := []string{"a", "my-a"}
	if len(repo.Aliases) != len(want) {
		t.Fatalf("aliases = %v, want %v", repo.Aliases, want)
	}
	for i := range want {
		if repo.Aliases[i] != want[i] {
			t.Errorf("aliases[%d] = %q, want %q", i, repo.Aliases[i], want[i])
		}
	}
}

func TestLoad_WithBoardedSection(t *testing.T) {
	root := t.TempDir()
	base := `[workspace]
org = "test-org"
default_branch = "main"

[repos.repo-a]
color = "#ff0000"
`
	local := `[repos.repo-a]
color = "#00ff00"

[boarded]
repo-a = ["main", "feature-x"]
repo-b = ["main"]
`
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(base), 0644)
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(local), 0644)

	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Repos["repo-a"].Color != "#00ff00" {
		t.Errorf("color = %q, want %q", cfg.Repos["repo-a"].Color, "#00ff00")
	}
	if len(cfg.Boarded) != 2 {
		t.Fatalf("boarded count = %d, want 2", len(cfg.Boarded))
	}
	if len(cfg.Boarded["repo-a"]) != 2 || cfg.Boarded["repo-a"][0] != "main" || cfg.Boarded["repo-a"][1] != "feature-x" {
		t.Errorf("boarded[repo-a] = %v, want [main feature-x]", cfg.Boarded["repo-a"])
	}
	if len(cfg.Boarded["repo-b"]) != 1 || cfg.Boarded["repo-b"][0] != "main" {
		t.Errorf("boarded[repo-b] = %v, want [main]", cfg.Boarded["repo-b"])
	}
}

func TestLoad_NoBoardedSection(t *testing.T) {
	root := t.TempDir()
	content := `[workspace]
org = "test-org"
default_branch = "main"

[repos.repo-a]
`
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(content), 0644)

	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Boarded == nil {
		t.Error("Boarded should be initialized (empty map), not nil")
	}
	if len(cfg.Boarded) != 0 {
		t.Errorf("boarded count = %d, want 0", len(cfg.Boarded))
	}
}

// A ws.local.toml with other settings but no [boarded] or [silo] still leaves
// both maps ready to add to: ws lift used to panic boarding into a nil map.
func TestLoad_LocalConfigWithoutBoardedOrSilo(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(`[workspace]
org = "test-org"
default_branch = "main"

[repos.repo-a]
`), 0644)
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(`[orchestrator]
model = "auto"
`), 0644)

	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Boarded == nil || cfg.Silo == nil {
		t.Fatalf("Boarded = %v, Silo = %v: both should be empty maps, not nil", cfg.Boarded, cfg.Silo)
	}
	cfg.Boarded["repo-a"] = append(cfg.Boarded["repo-a"], "cap") // must not panic
	if cfg.Orchestrator.Model != "auto" {
		t.Errorf("orchestrator model = %q, want auto", cfg.Orchestrator.Model)
	}
}

func TestLoad_WithoutLocalConfig(t *testing.T) {
	root := t.TempDir()
	content := `[workspace]
org = "test-org"
default_branch = "main"

[repos.repo-a]
color = "#ff0000"
`
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(content), 0644)
	// No ws.local.toml

	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Repos["repo-a"].Color != "#ff0000" {
		t.Errorf("color = %q, want %q", cfg.Repos["repo-a"].Color, "#ff0000")
	}
}

func TestSaveBoarded_CreatesFile(t *testing.T) {
	root := t.TempDir()
	boarded := map[string][]string{
		"repo-a": {"main", "feature-x"},
		"repo-b": {"main"},
	}
	if err := SaveBoarded(root, boarded); err != nil {
		t.Fatalf("SaveBoarded() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, "ws.local.toml"))
	if err != nil {
		t.Fatalf("reading ws.local.toml: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "[boarded]") {
		t.Error("expected [boarded] section in output")
	}
	if !strings.Contains(s, "repo-a") {
		t.Error("expected repo-a in output")
	}
}

func TestSaveBoarded_PreservesExistingRepoOverrides(t *testing.T) {
	root := t.TempDir()
	existing := `[repos.repo-a]
color = "#ff0000"

[boarded]
repo-a = ["old-branch"]
`
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(existing), 0644)

	boarded := map[string][]string{
		"repo-a": {"main", "feature-x"},
	}
	if err := SaveBoarded(root, boarded); err != nil {
		t.Fatalf("SaveBoarded() error: %v", err)
	}

	// Reload and verify both repo overrides and boarded section
	cfg := &LocalConfig{}
	_, err := toml.DecodeFile(filepath.Join(root, "ws.local.toml"), cfg)
	if err != nil {
		t.Fatalf("re-parse error: %v", err)
	}
	if cfg.Repos["repo-a"].Color != "#ff0000" {
		t.Errorf("color was lost: got %q", cfg.Repos["repo-a"].Color)
	}
	if len(cfg.Boarded["repo-a"]) != 2 || cfg.Boarded["repo-a"][0] != "main" {
		t.Errorf("boarded = %v, want [main feature-x]", cfg.Boarded["repo-a"])
	}
}

// Boarding nothing writes nothing; unboarding the last capsule takes its
// section away (ws.local.toml is edited key by key, edit.go).
func TestSaveBoarded_EmptyMap(t *testing.T) {
	root := t.TempDir()
	if err := SaveBoarded(root, map[string][]string{}); err != nil {
		t.Fatalf("SaveBoarded() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ws.local.toml")); !os.IsNotExist(err) {
		t.Errorf("an empty board wrote a file: %v", err)
	}
	if err := SaveBoarded(root, map[string][]string{"api": {"feat"}}); err != nil {
		t.Fatal(err)
	}
	if err := SaveBoarded(root, map[string][]string{}); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(root, "ws.local.toml"))
	if s := string(content); strings.Contains(s, "[boarded]") || strings.Contains(s, "api") {
		t.Errorf("unboarded, the file says:\n%s", s)
	}
}

func TestLoad_WithGitProtocol(t *testing.T) {
	root := t.TempDir()
	base := `[workspace]
org = "test-org"
default_branch = "main"

[repos.repo-a]
`
	local := `git = "ssh"

[repos.repo-a]
color = "#00ff00"
`
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(base), 0644)
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(local), 0644)

	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Git != "ssh" {
		t.Errorf("git = %q, want %q", cfg.Git, "ssh")
	}
}

func TestLoad_GitProtocolDefault(t *testing.T) {
	root := t.TempDir()
	content := `[workspace]
org = "test-org"
default_branch = "main"

[repos.repo-a]
`
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(content), 0644)

	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Git != "" {
		t.Errorf("git = %q, want empty", cfg.Git)
	}
}

func TestLoad_InvalidGitProtocol(t *testing.T) {
	root := t.TempDir()
	base := `[workspace]
org = "test-org"
default_branch = "main"

[repos.repo-a]
`
	local := `git = "ftp"
`
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(base), 0644)
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(local), 0644)

	_, _, err := Load(root)
	if err == nil {
		t.Error("expected error for invalid git protocol")
	}
}

func TestSaveBoarded_PreservesGitField(t *testing.T) {
	root := t.TempDir()
	existing := `git = "ssh"

[repos.repo-a]
color = "#ff0000"

[boarded]
repo-a = ["old-branch"]
`
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(existing), 0644)

	boarded := map[string][]string{
		"repo-a": {".ground"},
	}
	if err := SaveBoarded(root, boarded); err != nil {
		t.Fatalf("SaveBoarded() error: %v", err)
	}

	cfg := &LocalConfig{}
	_, err := toml.DecodeFile(filepath.Join(root, "ws.local.toml"), cfg)
	if err != nil {
		t.Fatalf("re-parse error: %v", err)
	}
	if cfg.Git != "ssh" {
		t.Errorf("git field was lost: got %q", cfg.Git)
	}
}

func TestParseRepoConfig_Valid(t *testing.T) {
	content := `[capsule]
copy_from_ground = [".env", "config/local.yaml"]
after_create = "npm install"
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.repo.toml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := ParseRepoConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Capsule.CopyFromGround) != 2 {
		t.Fatalf("copy_from_ground count = %d, want 2", len(cfg.Capsule.CopyFromGround))
	}
	if cfg.Capsule.CopyFromGround[0] != ".env" {
		t.Errorf("copy_from_ground[0] = %q, want %q", cfg.Capsule.CopyFromGround[0], ".env")
	}
	if cfg.Capsule.CopyFromGround[1] != "config/local.yaml" {
		t.Errorf("copy_from_ground[1] = %q, want %q", cfg.Capsule.CopyFromGround[1], "config/local.yaml")
	}
	if cfg.Capsule.AfterCreate != "npm install" {
		t.Errorf("after_create = %q, want %q", cfg.Capsule.AfterCreate, "npm install")
	}
}

func TestParseRepoConfig_MissingFile(t *testing.T) {
	cfg, err := ParseRepoConfig(filepath.Join(t.TempDir(), "nonexistent.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config for missing file")
	}
}

func TestParseRepoConfig_Malformed(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.repo.toml")
	os.WriteFile(path, []byte("this is not valid toml [[["), 0644)

	_, err := ParseRepoConfig(path)
	if err == nil {
		t.Error("expected error for malformed TOML")
	}
}

func TestParseRepoConfig_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.repo.toml")
	os.WriteFile(path, []byte(""), 0644)

	cfg, err := ParseRepoConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config for empty file")
	}
	if len(cfg.Capsule.CopyFromGround) != 0 {
		t.Errorf("copy_from_ground = %v, want empty", cfg.Capsule.CopyFromGround)
	}
	if cfg.Capsule.AfterCreate != "" {
		t.Errorf("after_create = %q, want empty", cfg.Capsule.AfterCreate)
	}
}

func TestParseRepoConfig_OnlyAfterCreate(t *testing.T) {
	content := `[capsule]
after_create = "make setup"
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.repo.toml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := ParseRepoConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Capsule.AfterCreate != "make setup" {
		t.Errorf("after_create = %q, want %q", cfg.Capsule.AfterCreate, "make setup")
	}
	if len(cfg.Capsule.CopyFromGround) != 0 {
		t.Errorf("copy_from_ground = %v, want empty", cfg.Capsule.CopyFromGround)
	}
}

func TestLocalConfigParseSilo(t *testing.T) {
	root := t.TempDir()
	base := `[workspace]
org = "test-org"
default_branch = "main"

[repos.repo-a]
`
	local := `[silo]
repo-a = "capsule-feat-x"
repo-b = "capsule-feat-y"
`
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(base), 0644)
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(local), 0644)

	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Silo) != 2 {
		t.Fatalf("silo count = %d, want 2", len(cfg.Silo))
	}
	if cfg.Silo["repo-a"] != "capsule-feat-x" {
		t.Errorf("silo[repo-a] = %q, want %q", cfg.Silo["repo-a"], "capsule-feat-x")
	}
	if cfg.Silo["repo-b"] != "capsule-feat-y" {
		t.Errorf("silo[repo-b] = %q, want %q", cfg.Silo["repo-b"], "capsule-feat-y")
	}
}

func TestSaveSilo(t *testing.T) {
	root := t.TempDir()
	// Pre-populate with boarded data to ensure it's not clobbered
	existing := `[boarded]
repo-a = ["main", "feature-x"]
`
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(existing), 0644)

	silo := map[string]string{
		"repo-a": "capsule-feat-x",
	}
	if err := SaveSilo(root, silo); err != nil {
		t.Fatalf("SaveSilo() error: %v", err)
	}

	// Reload and verify silo was persisted
	var local LocalConfig
	if _, err := toml.DecodeFile(filepath.Join(root, "ws.local.toml"), &local); err != nil {
		t.Fatalf("re-parse error: %v", err)
	}
	if local.Silo["repo-a"] != "capsule-feat-x" {
		t.Errorf("silo[repo-a] = %q, want %q", local.Silo["repo-a"], "capsule-feat-x")
	}
	// Verify boarded was not clobbered
	if len(local.Boarded["repo-a"]) != 2 || local.Boarded["repo-a"][0] != "main" {
		t.Errorf("boarded was clobbered: got %v, want [main feature-x]", local.Boarded["repo-a"])
	}
}

func TestRepoConfigSilo(t *testing.T) {
	content := `[capsule]
copy_from_ground = [".env"]
after_create = "npm install"

[silo]
after_switch = "npm run db:migrate"
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.repo.toml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := ParseRepoConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Silo.AfterSwitch != "npm run db:migrate" {
		t.Errorf("silo.after_switch = %q, want %q", cfg.Silo.AfterSwitch, "npm run db:migrate")
	}
	// Verify capsule section still works
	if cfg.Capsule.AfterCreate != "npm install" {
		t.Errorf("capsule.after_create = %q, want %q", cfg.Capsule.AfterCreate, "npm install")
	}
}

func TestLoad_Groups(t *testing.T) {
	root := t.TempDir()
	base := `[workspace]
org = "test-org"
default_branch = "main"

[groups.platform]
hidden = true

[repos.app]
group = "product"

[repos.infra]
group = "platform"

[repos.mobile]
`
	local := `[groups.platform]
hidden = false

[groups.mine]
hidden = true

[repos.mobile]
group = "mine"
`
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(base), 0644)
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(local), 0644)

	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.IsHidden("platform") {
		t.Error("platform hidden, want local override to show it")
	}
	if cfg.IsHidden("product") {
		t.Error("product hidden, want visible (no setting)")
	}
	if !cfg.IsHidden("mine") {
		t.Error("mine visible, want hidden (declared only in ws.local.toml)")
	}
	if got := cfg.Repos["infra"].Group; got != "platform" {
		t.Errorf("infra group = %q, want platform", got)
	}
	if got := cfg.Repos["mobile"].Group; got != "mine" {
		t.Errorf("mobile group = %q, want mine (local override)", got)
	}
}

func TestLoad_InvalidGroupName(t *testing.T) {
	root := t.TempDir()
	base := `[workspace]
org = "test-org"

[repos.app]
group = "a,b"
`
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(base), 0644)

	_, _, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "invalid group name") {
		t.Fatalf("err = %v, want invalid group name", err)
	}
}

func TestSaveGroupVisibility_SetAndReset(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(`[workspace]
org = "test-org"

[groups.platform]
hidden = true

[repos.infra]
group = "platform"
`), 0644)
	if err := SaveBoarded(root, map[string][]string{"infra": {"fix"}}); err != nil {
		t.Fatal(err)
	}

	shown := false
	if err := SaveGroupVisibility(root, "platform", &shown); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IsHidden("platform") {
		t.Error("platform hidden after show, want visible")
	}
	if got := cfg.Boarded["infra"]; len(got) != 1 || got[0] != "fix" {
		t.Errorf("boarded = %v, want preserved [fix]", got)
	}

	if err := SaveGroupVisibility(root, "platform", nil); err != nil {
		t.Fatal(err)
	}
	cfg, _, err = Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.IsHidden("platform") {
		t.Error("platform visible after reset, want team default (hidden)")
	}
	data, _ := os.ReadFile(filepath.Join(root, LocalFileName))
	if strings.Contains(string(data), "platform") {
		t.Errorf("ws.local.toml still mentions platform after reset:\n%s", data)
	}
}

func TestOrchestratorConfig(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, FileName), []byte("[workspace]\norg = \"acme\"\ndefault_branch = \"main\"\n"), 0o644)
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	// The model's default belongs to the harness (harness.Roles), so it's
	// empty here.
	if cfg.Orchestrator.Model != "" || cfg.Orchestrator.Harness != "" || !cfg.OrchestratorEnabled() || !cfg.OrchestratorAutoDispatch() || cfg.OrchestratorMaxDispatches() != 3 {
		t.Errorf("defaults: %+v %v", cfg.Orchestrator, cfg.OrchestratorEnabled())
	}

	os.WriteFile(filepath.Join(root, FileName), []byte("[workspace]\norg = \"acme\"\ndefault_branch = \"main\"\n[orchestrator]\nmodel = \"team-model\"\nharness = \"cursor\"\n"), 0o644)
	os.WriteFile(filepath.Join(root, LocalFileName), []byte("[orchestrator]\nenabled = false\n"), 0o644)
	cfg, _, _ = Load(root)
	if cfg.Orchestrator.Model != "team-model" || cfg.Orchestrator.Harness != "cursor" || cfg.OrchestratorEnabled() {
		t.Errorf("team model + local off: %+v %v", cfg.Orchestrator, cfg.OrchestratorEnabled())
	}

	os.WriteFile(filepath.Join(root, LocalFileName), []byte("[orchestrator]\nmodel = \"mine\"\nharness = \"claude\"\n"), 0o644)
	cfg, _, _ = Load(root)
	if cfg.Orchestrator.Model != "mine" || cfg.Orchestrator.Harness != "claude" || !cfg.OrchestratorEnabled() {
		t.Errorf("local model: %+v %v", cfg.Orchestrator, cfg.OrchestratorEnabled())
	}

	os.WriteFile(filepath.Join(root, LocalFileName), []byte("[orchestrator]\ndispatch = \"ask\"\nmax_dispatches = 5\n"), 0o644)
	cfg, _, _ = Load(root)
	if cfg.OrchestratorAutoDispatch() || cfg.OrchestratorMaxDispatches() != 5 {
		t.Errorf("local dispatch: %v %d", cfg.OrchestratorAutoDispatch(), cfg.OrchestratorMaxDispatches())
	}
	os.WriteFile(filepath.Join(root, LocalFileName), []byte("[orchestrator]\ndispatch = \"sometimes\"\n"), 0o644)
	if _, _, err := Load(root); err == nil || !strings.Contains(err.Error(), "must be \"auto\" or \"ask\"") {
		t.Errorf("bad dispatch: %v", err)
	}
}

// [crew] and [briefing] pick a role's harness and model; ws.local.toml
// overrides ws.toml field by field, and other local settings don't drop them.
func TestAgentRoleConfig(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, FileName), []byte(`[workspace]
org = "acme"
default_branch = "main"

[repos.api]

[crew]
harness = "cursor"
model = "claude-opus-5-5-high"

[briefing]
harness = "claude"
`), 0o644)
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Crew.AgentConfig != (AgentConfig{Harness: "cursor", Model: "claude-opus-5-5-high"}) || cfg.Briefing != (AgentConfig{Harness: "claude"}) {
		t.Errorf("team: crew %+v briefing %+v", cfg.Crew, cfg.Briefing)
	}

	os.WriteFile(filepath.Join(root, LocalFileName), []byte(`[crew]
harness = "claude"
model = "opus"

[briefing]
model = "sonnet"

[repos.api]
color = "red"
`), 0o644)
	cfg, _, err = Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Crew.AgentConfig != (AgentConfig{Harness: "claude", Model: "opus"}) || cfg.Briefing != (AgentConfig{Harness: "claude", Model: "sonnet"}) {
		t.Errorf("local: crew %+v briefing %+v", cfg.Crew, cfg.Briefing)
	}
}

// [crew] allowed lists the harnesses and models crew may be launched on
// beside the default. ws.local.toml's replaces ws.toml's whole (an empty
// one too); setting other [crew] keys there leaves the team's list alone.
func TestCrewAllowedConfig(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, FileName), []byte(`[workspace]
org = "acme"

[crew]
harness = "claude"
model = "opus"
allowed = ["claude:sonnet", "cursor:gpt-5.5-high", "pi:sonnet:high"]
`), 0o644)
	load := func() *Config {
		t.Helper()
		cfg, _, err := Load(root)
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	cfg := load()
	if got := strings.Join(cfg.Crew.Allowed, " "); got != "claude:sonnet cursor:gpt-5.5-high pi:sonnet:high" || cfg.Crew.Harness != "claude" || cfg.Crew.Model != "opus" {
		t.Errorf("team: %+v", cfg.Crew)
	}
	if len(cfg.Unknown) != 0 {
		t.Errorf("unknown: %v", cfg.Unknown)
	}

	os.WriteFile(filepath.Join(root, LocalFileName), []byte("[crew]\nmodel = \"haiku\"\n"), 0o644)
	if cfg := load(); len(cfg.Crew.Allowed) != 3 || cfg.Crew.Model != "haiku" {
		t.Errorf("local model only: %+v", cfg.Crew)
	}
	os.WriteFile(filepath.Join(root, LocalFileName), []byte("[crew]\nallowed = [\"codex:gpt-6\"]\n"), 0o644)
	if cfg := load(); strings.Join(cfg.Crew.Allowed, " ") != "codex:gpt-6" || cfg.Crew.Model != "opus" {
		t.Errorf("local allowed isn't the list: %+v", cfg.Crew)
	}
	os.WriteFile(filepath.Join(root, LocalFileName), []byte("[crew]\nallowed = []\n"), 0o644)
	if cfg := load(); len(cfg.Crew.Allowed) != 0 {
		t.Errorf("an empty local list didn't empty it: %+v", cfg.Crew)
	}

	// [briefing] has no allowed: ws doctor says so.
	os.WriteFile(filepath.Join(root, LocalFileName), []byte("[briefing]\nallowed = [\"claude:opus\"]\n"), 0o644)
	if cfg := load(); strings.Join(cfg.Unknown, " ") != "ws.local.toml: briefing.allowed" {
		t.Errorf("unknown = %v", cfg.Unknown)
	}

	for _, bad := range []string{":opus", "", "  ", "cl aude:opus"} {
		os.WriteFile(filepath.Join(root, LocalFileName), []byte("[crew]\nallowed = ["+strconv.Quote(bad)+"]\n"), 0o644)
		if _, _, err := Load(root); err == nil || !strings.Contains(err.Error(), "invalid [crew] allowed") {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestParsePick(t *testing.T) {
	for in, want := range map[string][2]string{
		"claude:opus":                  {"claude", "opus"},
		" claude : opus":               {"claude", "opus"},
		"cursor":                       {"cursor", ""},
		"cursor:":                      {"cursor", ""},
		"pi:sonnet:high":               {"pi", "sonnet:high"},
		"pi:anthropic/claude-opus-5-5": {"pi", "anthropic/claude-opus-5-5"},
	} {
		h, m, err := ParsePick(in)
		if err != nil || h != want[0] || m != want[1] {
			t.Errorf("%q: %q %q, %v", in, h, m, err)
		}
	}
}

// [missions] records names the records repo; ws.local.toml can override it,
// and other local settings don't drop it.
func TestMissionsRecordsConfig(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, FileName), []byte("[workspace]\norg = \"acme\"\ndefault_branch = \"main\"\n\n[missions]\nrecords = \"records\"\n"), 0o644)
	os.WriteFile(filepath.Join(root, LocalFileName), []byte("[crew]\nharness = \"claude\"\n"), 0o644)
	cfg, _, err := Load(root)
	if err != nil || cfg.Missions.Records != "records" {
		t.Fatalf("records = %q, %v", cfg.Missions.Records, err)
	}
	os.WriteFile(filepath.Join(root, LocalFileName), []byte("[missions]\nrecords = \"my-notes\"\n"), 0o644)
	if cfg, _, _ = Load(root); cfg.Missions.Records != "my-notes" {
		t.Errorf("local records = %q", cfg.Missions.Records)
	}
}

// What ws.toml says of conversations and pages holds when there's a
// ws.local.toml too: its own [conversation] settings go over them, field by
// field, and it has nothing to say of pages.
func TestLoad_SharedConversationAndPagesWithLocalConfig(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(`[workspace]
org = "acme"

[conversation]
harness = "claude"
model = "sonnet"

[pages]
style = "guides/style.md"

[pages.passes.wording]
title = "Wording"
file = "guides/style.md"
kinds = ["tell"]
`), 0644)
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(`[conversation]
model = "haiku"
`), 0644)
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Conversation.Harness != "claude" || cfg.Conversation.Model != "haiku" {
		t.Errorf("[conversation] = %+v, want claude on haiku", cfg.Conversation)
	}
	p, ok := cfg.Pages.Passes["wording"]
	if cfg.Pages.Style != "guides/style.md" || !ok || p.Title != "Wording" || p.File != "guides/style.md" || len(p.Kinds) != 1 {
		t.Errorf("[pages] = %+v", cfg.Pages)
	}
}

func TestLoad_Team(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(`[workspace]
org = "acme"

[team]
window = "3w"

[team.people.ana]
name = "Ana Diaz"
logins = ["ana-acme"]
emails = ["ana@acme.example"]
`), 0644)
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(`[team.people.ben]
name = "Ben Okoro"
logins = ["ben-acme"]
`), 0644)
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	ana, ben := cfg.Team.People["ana"], cfg.Team.People["ben"]
	if cfg.Team.Window != "3w" || ana.Name != "Ana Diaz" || len(ana.Logins) != 1 || len(ana.Emails) != 1 || ben.Name != "Ben Okoro" {
		t.Errorf("[team] = %+v", cfg.Team)
	}
	// The workspace's own watched paths.
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte("[team.watch.patterns]\ntitle = \"Pattern\"\npaths = [\"docs:technical/patterns/**\"]\n"), 0644)
	if cfg, _, err = Load(root); err != nil || cfg.Team.Watch["patterns"].Title != "Pattern" || len(cfg.Team.Watch["patterns"].Paths) != 1 {
		t.Errorf("[team.watch] = %+v, %v", cfg.Team.Watch, err)
	}
	// Gists are written unless it says not to.
	if !cfg.TeamNarrative() {
		t.Error("narrative is off by default")
	}
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte("[team]\nnarrative = false\nmodel = \"haiku\"\n"), 0644)
	if cfg, _, err = Load(root); err != nil || cfg.TeamNarrative() || cfg.Team.Model != "haiku" || cfg.Team.Window != "3w" {
		t.Errorf("[team] with a local file = %+v, %v", cfg.Team, err)
	}
}

func TestLoad_Landing(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, FileName), []byte("[workspace]\norg = \"o\"\ndefault_branch = \"main\"\n\n[repos.records]\nlanding = \"direct\"\nverify = \"just lint\"\n\n[repos.api]\n\n[repos.docs]\nlanding = \"pr\"\n"), 0o644)
	// What this machine says wins.
	os.WriteFile(filepath.Join(root, LocalFileName), []byte("[repos.api]\nlanding = \"direct\"\n\n[repos.records]\nverify = \"make check\"\n"), 0o644)

	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if r := cfg.Repos["records"]; r.Landing != LandingDirect || r.Verify != "make check" {
		t.Errorf("records: %+v", r)
	}
	if r := cfg.Repos["api"]; r.Landing != LandingDirect {
		t.Errorf("api: %+v", r)
	}
	if r := cfg.Repos["docs"]; r.Landing != LandingPR {
		t.Errorf("docs: %+v", r)
	}
}

func TestLoad_LandingMustBeOneItKnows(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, FileName), []byte("[workspace]\norg = \"o\"\ndefault_branch = \"main\"\n\n[repos.records]\nlanding = \"straight\"\n"), 0o644)

	if _, _, err := Load(root); err == nil || !strings.Contains(err.Error(), `invalid landing "straight"`) {
		t.Errorf("Load: %v, want it to refuse the landing", err)
	}
}

// Kinds of document are the team's, in ws.toml: a ws.local.toml beside it
// doesn't lose them.
func TestLoad_DocumentKinds(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, FileName), []byte("[workspace]\norg = \"o\"\ndefault_branch = \"main\"\n\n[repos.records]\n\n[documents.bp]\ntitle = \"Build proposal\"\npath = \"records:bp/proposed/{date}-{slug}.md\"\nadd = \"docs: add bp {date} {slug}\"\namend = \"docs: amend bp {date} {slug}\"\nguide = \".agents/skills/build-proposals/SKILL.md\"\npasses = [\"share\", \"missing\"]\n\n[documents.bp.moves.accept]\nto = \"bp/accepted/\"\n"), 0o644)
	os.WriteFile(filepath.Join(root, LocalFileName), []byte("[crew]\nmodel = \"haiku\"\n"), 0o644)

	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	want := DocumentKind{Title: "Build proposal", Path: "records:bp/proposed/{date}-{slug}.md", Add: "docs: add bp {date} {slug}", Amend: "docs: amend bp {date} {slug}", Guide: ".agents/skills/build-proposals/SKILL.md", Passes: []string{"share", "missing"},
		Moves: map[string]DocumentMove{"accept": {To: "bp/accepted/"}}}
	if got := cfg.Documents["bp"]; !reflect.DeepEqual(got, want) {
		t.Errorf("bp: %+v", got)
	}
}

// ws.local.toml is written by ws (a lift boards the capsule), from what it
// read of it. What it didn't read is kept: it's the user's file too.
func TestUpdateLocal_KeepsWhatItDoesntRead(t *testing.T) {
	root := t.TempDir()
	existing := `git = "ssh"
editor = "zed"

[repos.api]
color = "#ff0000"
alias = ["a"]

[pages]
style = "docs/style.md"

[boarded]
api = ["old"]
`
	os.WriteFile(filepath.Join(root, LocalFileName), []byte(existing), 0o644)

	if err := SaveBoarded(root, map[string][]string{"api": {"main", "feature-x"}}); err != nil {
		t.Fatalf("SaveBoarded: %v", err)
	}

	var got map[string]any
	if _, err := toml.DecodeFile(filepath.Join(root, LocalFileName), &got); err != nil {
		t.Fatalf("what was written doesn't parse: %v", err)
	}
	if got["editor"] != "zed" {
		t.Errorf("a top-level key ws doesn't read was dropped: %v", got["editor"])
	}
	if pages, _ := got["pages"].(map[string]any); pages["style"] != "docs/style.md" {
		t.Errorf("a section ws doesn't read there was dropped: %v", got["pages"])
	}
	api, _ := got["repos"].(map[string]any)["api"].(map[string]any)
	if alias, _ := api["alias"].([]any); len(alias) != 1 || alias[0] != "a" {
		t.Errorf("an unknown key in a known table was dropped: %v", api)
	}
	if api["color"] != "#ff0000" || got["git"] != "ssh" {
		t.Errorf("what ws does read was lost: %v", got)
	}
	if b, _ := got["boarded"].(map[string]any)["api"].([]any); len(b) != 2 || b[0] != "main" {
		t.Errorf("boarded = %v, want what was saved (not what was there)", got["boarded"])
	}
}

// A file ws reads all of is written as it always was.
func TestUpdateLocal_NothingUnknown(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, LocalFileName), []byte("git = \"ssh\"\n"), 0o644)
	if err := SaveSilo(root, map[string]string{"api": "x"}); err != nil {
		t.Fatal(err)
	}
	var local LocalConfig
	if _, err := toml.DecodeFile(filepath.Join(root, LocalFileName), &local); err != nil {
		t.Fatal(err)
	}
	if local.Git != "ssh" || local.Silo["api"] != "x" {
		t.Errorf("got %+v", local)
	}
}

// A key ws doesn't read is passed over when the config loads, and named
// in Unknown for ws doctor: once for a whole table.
func TestLoad_NamesWhatItDoesntRead(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, FileName), []byte(`[workspace]
org = "acme"

[repos.api]
alias = ["a"]

[mission]
records = "notes"
`), 0o644)
	os.WriteFile(filepath.Join(root, LocalFileName), []byte("[pages]\nstyle = \"s.md\"\n\n[boarded]\n"), 0o644)

	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ws.toml: repos.api.alias", "ws.toml: mission", "ws.local.toml: pages"}
	if !slices.Equal(cfg.Unknown, want) {
		t.Errorf("Unknown = %q, want %q", cfg.Unknown, want)
	}
}

// A plugin's config is the plugin's to say, tables nested in it too: none
// of it is named as unread, from either file.
func TestLoad_PluginConfigIsNotUnknown(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, FileName), []byte(`[workspace]
org = "acme"

[plugin.jira]
url = "https://jira.example"

[plugin.jira.queries.bugs]
label = "Bugs"
`), 0o644)
	os.WriteFile(filepath.Join(root, LocalFileName), []byte(`[plugin.jira]
token = "t"

[plugin.jira.queries.mine]
label = "Mine"
jql = "assignee = currentUser()"
`), 0o644)

	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Unknown) != 0 {
		t.Errorf("Unknown = %q, want none", cfg.Unknown)
	}
	if got := cfg.Plugin["jira"]["queries"].(map[string]any)["mine"].(map[string]any)["jql"]; got != "assignee = currentUser()" {
		t.Errorf("plugin.jira.queries.mine.jql = %v", got)
	}
}

func TestParseRepoConfig_NamesWhatItDoesntRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ws.repo.toml")
	os.WriteFile(path, []byte("landing = \"direct\"\nverfy = \"make test\"\n"), 0o644)
	rc, err := ParseRepoConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"ws.repo.toml: verfy"}; !slices.Equal(rc.Unknown, want) {
		t.Errorf("Unknown = %q, want %q", rc.Unknown, want)
	}
}

// A review's passes are the team's (ws.toml) with your own added
// (ws.local.toml): one of yours named as one of the team's replaces it.
func TestLoad_ReviewPasses(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(`[workspace]
org = "acme"

[reviews.passes.security]
skill = "security-review"
kinds = ["vulnerability", "hardening"]
blocks = ["vulnerability"]
repos = ["api"]

[reviews.passes.tests]
looks = "The team's."
`), 0644)
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(`[reviews.passes.tests]
looks = "Mine."
cold = true

[reviews.passes.naming]
looks = "Names that mislead."
`), 0644)
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Unknown) != 0 {
		t.Errorf("unknown: %v", cfg.Unknown)
	}
	ps := cfg.Reviews.Passes
	if len(ps) != 3 {
		t.Fatalf("passes: %+v", ps)
	}
	if s := ps["security"]; s.Skill != "security-review" || len(s.Blocks) != 1 || s.Blocks[0] != "vulnerability" || len(s.Repos) != 1 {
		t.Errorf("the team's: %+v", s)
	}
	if p := ps["tests"]; p.Looks != "Mine." || !p.Cold {
		t.Errorf("yours in place of the team's: %+v", p)
	}
	if ps["naming"].Looks == "" {
		t.Errorf("yours: %+v", ps["naming"])
	}
}

func TestDocuments_ReadAsTrueFalseOrAFolder(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, FileName), []byte(`
[repos.records]
documents = false
[repos.docs]
documents = true
[repos.api]
documents = "./docs"
[repos.web]
`), 0o644)
	os.WriteFile(filepath.Join(dir, LocalFileName), []byte(`
[repos.docs]
documents = false
`), 0o644)
	cfg, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]*Documents{
		"records": {On: false},
		"docs":    {On: false}, // ws.local.toml turns it off
		"api":     {On: true, Folder: "docs/"},
		"web":     nil,
	}
	for repo, w := range want {
		got := cfg.Repos[repo].Documents
		if (got == nil) != (w == nil) || (got != nil && *got != *w) {
			t.Errorf("%s: documents = %+v, want %+v", repo, got, w)
		}
	}
	if len(cfg.Unknown) != 0 {
		t.Errorf("unknown keys: %v", cfg.Unknown)
	}
	// Written back as it was said.
	for _, d := range []Documents{{On: true}, {}, {On: true, Folder: "docs/"}} {
		b, _ := d.MarshalTOML()
		var back Documents
		var v any
		if err := toml.Unmarshal(append([]byte("v = "), b...), &struct{ V *any }{&v}); err != nil {
			t.Fatal(err)
		}
		if err := back.UnmarshalTOML(v); err != nil || back != d {
			t.Errorf("%+v came back %+v (%v)", d, back, err)
		}
	}
	if err := new(Documents).UnmarshalTOML("../elsewhere"); err == nil {
		t.Error("a folder out of the repo was taken")
	}
}

// An accent is a hex colour, #rgb or #rrggbb, given back as #rrggbb in
// lowercase; anything else is refused, as it's put into the web UI's CSS.
func TestParseAccent(t *testing.T) {
	for in, want := range map[string]string{
		"#336699":     "#336699",
		"#3A7BD5":     "#3a7bd5",
		"#abc":        "#aabbcc",
		"#F0f":        "#ff00ff",
		"  #000000  ": "#000000",
	} {
		if got, err := ParseAccent(in); err != nil || got != want {
			t.Errorf("ParseAccent(%q) = %q, %v, want %q", in, got, err, want)
		}
	}
	for _, in := range []string{
		"", "#", "336699", "#33669", "#3366999", "#33669980", "#abcd", "#ggg", "#12345z",
		"red", "rgb(51, 102, 153)", "##33669", "#336699;", "#fff;}body{display:none",
		`#fff" onload="x`, "#fff\n0", "#fff 0", "＃336699", "#３３６",
	} {
		if got, err := ParseAccent(in); err == nil {
			t.Errorf("ParseAccent(%q) = %q, want it refused", in, got)
		}
	}
}

// [ui] accent: unset, there's none (the web UI's own default applies);
// the team's in ws.toml, yours in ws.local.toml winning; one that isn't a
// hex colour is refused.
func TestLoad_UIAccent(t *testing.T) {
	root := t.TempDir()
	team := func(s string) {
		os.WriteFile(filepath.Join(root, FileName), []byte("[workspace]\norg = \"o\"\n"+s), 0o644)
	}
	local := func(s string) { os.WriteFile(filepath.Join(root, LocalFileName), []byte(s), 0o644) }
	accent := func() string {
		t.Helper()
		cfg, _, err := Load(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Unknown) > 0 {
			t.Errorf("unknown keys: %v", cfg.Unknown)
		}
		return cfg.UIAccent()
	}

	team("")
	if a := accent(); a != "" {
		t.Errorf("unset: %q, want none", a)
	}
	team("\n[ui]\naccent = \"#2A9D8F\"\n")
	if a := accent(); a != "#2a9d8f" {
		t.Errorf("the team's: %q", a)
	}
	local("[ui]\naccent = \"#c33\"\n")
	if a := accent(); a != "#cc3333" {
		t.Errorf("yours over the team's: %q", a)
	}
	local("[ui]\n")
	if a := accent(); a != "#2a9d8f" {
		t.Errorf("yours cleared: %q, want the team's", a)
	}
	team("")
	local("[ui]\naccent = \"#123456\"\n")
	if a := accent(); a != "#123456" {
		t.Errorf("yours alone: %q", a)
	}

	for _, bad := range []string{"violet", "#12345", "#fff;}body{color:red"} {
		local("[ui]\naccent = \"" + bad + "\"\n")
		if _, _, err := Load(root); err == nil || !strings.Contains(err.Error(), "[ui] accent") {
			t.Errorf("yours %q: %v, want it refused", bad, err)
		}
		local("")
		team("\n[ui]\naccent = \"" + bad + "\"\n")
		if _, _, err := Load(root); err == nil || !strings.Contains(err.Error(), "[ui] accent") {
			t.Errorf("the team's %q: %v, want it refused", bad, err)
		}
		team("")
	}
	// What isn't a hex colour never comes out, however the config was made.
	if a := (&Config{UI: UIConfig{Accent: "url(x)"}}).UIAccent(); a != "" {
		t.Errorf("a config not loaded: %q", a)
	}
}

// Where the web UI's places go is one arrangement: yours, saying either
// list, replaces the team's two; saying neither, the team's stands.
func TestLoad_UIPlaces(t *testing.T) {
	team := `[workspace]
org = "test-org"

[repos.repo-a]

[ui]
sidebar = ["missions", "reviews"]
menu = ["team"]
`
	for _, tc := range []struct {
		name, local   string
		sidebar, menu []string
	}{
		{"the team's", "", []string{"missions", "reviews"}, []string{"team"}},
		{"yours, both", "[ui]\nsidebar = [\"reviews\"]\nmenu = [\"files\", \"team\"]\n", []string{"reviews"}, []string{"files", "team"}},
		{"yours, the menu alone", "[ui]\nmenu = [\"missions\"]\n", nil, []string{"missions"}},
		{"yours, an accent alone", "[ui]\naccent = \"#123456\"\n", []string{"missions", "reviews"}, []string{"team"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "ws.toml"), []byte(team), 0644)
			if tc.local != "" {
				os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(tc.local), 0644)
			}
			cfg, _, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(cfg.UI.Sidebar, tc.sidebar) || !slices.Equal(cfg.UI.Menu, tc.menu) {
				t.Errorf("sidebar %q, menu %q; want %q, %q", cfg.UI.Sidebar, cfg.UI.Menu, tc.sidebar, tc.menu)
			}
		})
	}
}

// What's the team's alone is still there once yours is laid over it:
// where a plugin's kind of thing goes is ws.toml's to say.
func TestMerge_KeepsWhereThingsGo(t *testing.T) {
	base := &Config{Things: map[string]ThingKind{"sketch": {Paths: []string{"records:sketches/{slug}/", "docs:architecture/{slug}/"}}}}
	merged := Merge(base, &Config{})
	if got := merged.Things["sketch"].Paths; len(got) != 2 || got[1] != "docs:architecture/{slug}/" {
		t.Errorf("where a sketch goes, merged with a local config: %q", got)
	}
}
