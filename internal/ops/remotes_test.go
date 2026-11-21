package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/ref"
)

func loadToml(t *testing.T, toml string) (string, error) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ws.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := Load(root)
	return root, err
}

// A workspace's repos can be of more than one org, and a repo's name here
// needn't be its name on GitHub: each is where its github says, or else
// the org's by its name.
func TestLoad_ReposOfMoreThanOneOrg(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(`
[workspace]
org = "acme"

[repos.api]

[repos.cr-api]
github = "cr-team/api"

[repos.notes]
github = "someone/notes.git"
`), 0o644)
	_, ws, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for repo, want := range map[string]string{"api": "acme/api", "cr-api": "cr-team/api", "notes": "someone/notes"} {
		if got := ws.RemoteOf(repo).String(); got != want {
			t.Errorf("RemoteOf(%s) = %s, want %s", repo, got, want)
		}
	}
	for at, want := range map[string]string{"acme/api": "api", "CR-Team/API": "cr-api", "someone/notes": "notes", "cr-team/notes": "", "other/api": ""} {
		owner, name, _ := strings.Cut(at, "/")
		if got, _ := ws.RepoAt(owner, name); got != want {
			t.Errorf("RepoAt(%s) = %q, want %q", at, got, want)
		}
	}
	if got := strings.Join(ws.Owners(), ","); got != "acme,cr-team,someone" {
		t.Errorf("owners = %s", got)
	}
	if got := elsewhere(ws); len(got) != 2 || got["cr-api"] != "cr-team/api" || got["notes"] != "someone/notes" {
		t.Errorf("elsewhere = %v, want cr-api and notes", got)
	}

	// An address on GitHub is read as the repo that's there, by its name
	// here; written, a ref is addressed where its repo is.
	k := &ref.Known{Repos: map[string]ref.Tree{"api": nil, "cr-api": nil, "notes": nil}, Org: ws.Org, Remotes: ws.Remotes, DefaultBranch: "main"}
	r, ok := ref.ParseAddress("https://github.com/cr-team/api/pull/7", k)
	if !ok || r.Kind != ref.PR || r.Repo != "cr-api" || r.Number != 7 {
		t.Errorf("cr-team/api#7 read as %+v, %v", r, ok)
	}
	if _, ok := ref.ParseAddress("https://github.com/acme/cr-api/pull/7", k); ok {
		t.Error("acme/cr-api is no repo of the workspace's")
	}
	where := ref.Where{Org: ws.Org, Remotes: ws.Remotes, DefaultBranch: "main"}
	if got := (ref.Ref{Kind: ref.PR, Repo: "cr-api", Number: 7}).Address(where); got != "https://github.com/cr-team/api/pull/7" {
		t.Errorf("cr-api#7 addressed %s", got)
	}
	if got := (ref.Ref{Kind: ref.Repo, Repo: "api"}).Address(where); got != "https://github.com/acme/api" {
		t.Errorf("api addressed %s", got)
	}
}

// A github that isn't owner/name, or two repos at one place on GitHub,
// is said when the workspace is read.
func TestLoad_GitHubSaidWrong(t *testing.T) {
	if _, err := loadToml(t, "[workspace]\norg = \"acme\"\n[repos.api]\ngithub = \"api\"\n"); err == nil || !strings.Contains(err.Error(), "owner/name") {
		t.Errorf("github without an owner: %v", err)
	}
	_, err := loadToml(t, "[workspace]\norg = \"acme\"\n[repos.api]\n[repos.api2]\ngithub = \"ACME/api\"\n")
	if err == nil || !strings.Contains(err.Error(), "both acme/api") && !strings.Contains(err.Error(), "both ACME/api") {
		t.Errorf("two repos at acme/api: %v", err)
	}
}
