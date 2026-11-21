package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A repo given to the team goes in ws.toml after its other repos, with
// what's said of it and nothing else, and the rest of the file is as it
// was: comments and all.
func TestAddTeamRepo_AfterTheOtherRepos(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, FileName), `[workspace]
org = "acme" # ours

[repos.api]
aliases = ["a"]

# the docs
[repos.docs]

[crew]
harness = "claude"
`)
	err := AddTeamRepo(root, "cr-api", RepoConfig{GitHub: "cr-team/api", DisplayName: "CR API", Aliases: []string{"cra"}, Yours: true})
	if err != nil {
		t.Fatal(err)
	}
	want := `[workspace]
org = "acme" # ours

[repos.api]
aliases = ["a"]

# the docs
[repos.docs]

[repos.cr-api]
github = "cr-team/api"
display_name = "CR API"
aliases = ["cra"]

[crew]
harness = "claude"
`
	if got := readFile(t, filepath.Join(root, FileName)); got != want {
		t.Errorf("ws.toml:\n%s\nwant:\n%s", got, want)
	}
	if err := AddTeamRepo(root, "api", RepoConfig{}); err == nil {
		t.Error("a repo ws.toml has already should be refused")
	}
}

// A repo added for you alone goes at the end of ws.local.toml, its table
// written even with nothing in it; dropped, it goes whole, table and all,
// and what's around it stays.
func TestLocalRepo_AddedAndDroppedWhole(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, FileName), "[workspace]\norg = \"acme\"\n")
	local := filepath.Join(root, LocalFileName)
	writeFile(t, local, "[router]\nenabled = true\n")

	if err := AddLocalRepo(root, "notes", RepoConfig{GitHub: "someone/notes"}); err != nil {
		t.Fatal(err)
	}
	if err := AddLocalRepo(root, "bare", RepoConfig{}); err != nil {
		t.Fatal(err)
	}
	want := "[router]\nenabled = true\n\n[repos.notes]\ngithub = \"someone/notes\"\n\n[repos.bare]\n"
	if got := readFile(t, local); got != want {
		t.Errorf("ws.local.toml:\n%q\nwant\n%q", got, want)
	}
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Repos["notes"].Yours || !cfg.Repos["bare"].Yours {
		t.Errorf("repos = %+v, want notes and bare yours", cfg.Repos)
	}
	if err := AddLocalRepo(root, "notes", RepoConfig{}); err == nil {
		t.Error("a repo ws.local.toml has already should be refused")
	}

	for _, name := range []string{"bare", "notes"} {
		if err := DropLocalRepo(root, name); err != nil {
			t.Fatal(err)
		}
	}
	if got := readFile(t, local); got != "[router]\nenabled = true\n" {
		t.Errorf("after dropping both: %q", got)
	}
}
