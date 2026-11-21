package workspace

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/ref"
)

func TestDoctor_AllHealthy(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repos", "repo-a")
	bareDir := filepath.Join(repoDir, ".bare")
	os.MkdirAll(bareDir, 0755)
	os.MkdirAll(filepath.Join(repoDir, "main"), 0755)
	ws := &Workspace{
		Root:          root,
		DefaultBranch: "main",
		RepoNames:     []string{"repo-a"},
	}

	orig := CheckGitHubAuthFunc
	CheckGitHubAuthFunc = func() CheckResult {
		return CheckResult{Name: "gh auth", Status: CheckOK}
	}
	t.Cleanup(func() { CheckGitHubAuthFunc = orig })

	categories := ws.Doctor(nil)
	for _, cat := range categories {
		for _, check := range cat.Checks {
			if check.Status != CheckOK {
				t.Errorf("category %q check %q: got status %d, want CheckOK", cat.Name, check.Name, check.Status)
			}
		}
	}
}

func TestDoctor_RepoNotCloned(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "repos"), 0755)

	ws := &Workspace{
		Root:          root,
		DefaultBranch: "main",
		RepoNames:     []string{"missing-repo"},
	}

	categories := ws.Doctor(nil)
	reposCat := findCategory(categories, "Repos")
	if reposCat == nil {
		t.Fatal("missing Repos category")
	}
	if len(reposCat.Checks) != 1 || reposCat.Checks[0].Status != CheckFail {
		t.Errorf("expected CheckFail for missing repo, got %+v", reposCat.Checks)
	}
}

func TestCheckResult_HasFixFields(t *testing.T) {
	called := false
	r := CheckResult{
		Name:    "test",
		Status:  CheckFail,
		Detail:  "broken",
		Fix:     func() error { called = true; return nil },
		FixHint: "try this",
	}

	if r.Fix == nil {
		t.Fatal("Fix should not be nil")
	}
	if err := r.Fix(); err != nil {
		t.Fatalf("Fix returned error: %v", err)
	}
	if !called {
		t.Fatal("Fix was not called")
	}
	if r.FixHint != "try this" {
		t.Errorf("FixHint = %q, want %q", r.FixHint, "try this")
	}
}

func TestCheckRepos_MissingRepo_HasFix(t *testing.T) {
	root := t.TempDir()
	reposDir := filepath.Join(root, "repos")
	os.MkdirAll(filepath.Join(reposDir, "exists", ".bare"), 0755)
	os.MkdirAll(filepath.Join(reposDir, "missing"), 0755)

	w := &Workspace{
		Root:      root,
		RepoNames: []string{"exists", "missing"},
	}

	cat := w.checkRepos()

	for _, check := range cat.Checks {
		if check.Name == "exists" {
			if check.Status != CheckOK {
				t.Errorf("exists: status = %d, want CheckOK", check.Status)
			}
			if check.Fix != nil {
				t.Error("exists: Fix should be nil for healthy repo")
			}
		}
		if check.Name == "missing" {
			if check.Status != CheckFail {
				t.Errorf("missing: status = %d, want CheckFail", check.Status)
			}
			if check.Fix == nil {
				t.Error("missing: Fix should not be nil for missing repo")
			}
		}
	}
}

func TestCheckTools_MissingGh_HasHint(t *testing.T) {
	w := &Workspace{}

	// Temporarily break PATH to ensure gh isn't found
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", origPath)

	cat := w.checkTools()

	if len(cat.Checks) < 1 {
		t.Fatalf("expected at least 1 check, got %d", len(cat.Checks))
	}
	check := cat.Checks[0]
	if check.Status != CheckFail {
		t.Errorf("status = %d, want CheckFail", check.Status)
	}
	if check.FixHint == "" {
		t.Error("expected FixHint for missing gh tool")
	}
}

func findCategory(cats []CheckCategory, name string) *CheckCategory {
	for i := range cats {
		if cats[i].Name == name {
			return &cats[i]
		}
	}
	return nil
}

// Doctor says, of each owner the repos are under, whether gh can reach
// them: an org that enforces SAML SSO is said with what to do. A clone
// that fetches from somewhere other than where ws.toml says is said too,
// and fixed by pointing it there.
func TestCheckGitHub_EachOwnerAndEachOrigin(t *testing.T) {
	root := t.TempDir()
	ws := &Workspace{
		Root: root, Org: "acme", DefaultBranch: "main",
		RepoNames: []string{"api", "cr-api", "web"},
		Remotes:   map[string]ref.Remote{"cr-api": {Owner: "cr-team", Name: "api"}},
	}
	orig := CheckGitHubReachFunc
	CheckGitHubReachFunc = func(r ref.Remote) error {
		if r.Owner == "cr-team" {
			return errors.New("gh: Resource protected by organization SAML enforcement. (HTTP 403)")
		}
		return nil
	}
	t.Cleanup(func() { CheckGitHubReachFunc = orig })
	// cr-api was cloned from acme's, before ws.toml said where it is.
	bare := ws.BareDir("cr-api")
	if out, err := exec.Command("git", "init", "-q", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	exec.Command("git", "-C", bare, "remote", "add", "origin", "https://github.com/acme/cr-api.git").Run()

	cat := ws.CheckGitHub()
	got := map[string]CheckResult{}
	for _, c := range cat.Checks {
		got[c.Name] = c
	}
	if c := got["acme"]; c.Status != CheckOK || c.Detail != "2 repos reached" {
		t.Errorf("acme: %+v", c)
	}
	if c := got["cr-team"]; c.Status != CheckFail || !strings.Contains(c.FixHint, "orgs/cr-team/sso") {
		t.Errorf("cr-team: %+v", c)
	}
	c, ok := got["cr-api origin"]
	if !ok || c.Status != CheckWarn || c.Fix == nil {
		t.Fatalf("cr-api's origin: %+v", cat.Checks)
	}
	if err := c.Fix(); err != nil {
		t.Fatal(err)
	}
	out, _ := exec.Command("git", "-C", bare, "config", "--get", "remote.origin.url").Output()
	if strings.TrimSpace(string(out)) != "https://github.com/cr-team/api.git" {
		t.Errorf("origin after the fix: %s", out)
	}
	for _, c := range ws.CheckGitHub().Checks {
		if c.Name == "cr-api origin" {
			t.Errorf("still said after the fix: %+v", c)
		}
	}
}
