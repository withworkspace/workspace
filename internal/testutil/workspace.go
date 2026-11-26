package testutil

import (
	"fmt"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// WorkspaceOpts configures a test workspace.
type WorkspaceOpts struct {
	Org           string
	DefaultBranch string
	Repos         []RepoOpts
	HiddenGroups  []string // written as [groups.<name>] hidden = true
}

// RepoOpts configures a single repo in the test workspace.
type RepoOpts struct {
	Name        string
	Branches    []string // extra remote branches (each gets a commit)
	Aliases     []string
	DisplayName string
	AfterCreate string
	Group       string
	NotCloned   bool // create the source repo but leave it uncloned; see RedirectClones
	// Landing is how work lands in it ("direct": commits straight to the
	// default branch, with ws land), and Verify the command that must pass
	// before it does. A direct repo's source takes pushes to the branch it
	// has checked out, as a hosted remote would.
	Landing string
	Verify  string
}

// Workspace holds paths to the created test workspace.
type Workspace struct {
	Root    string            // workspace root containing ws.toml
	Org     string            // GitHub org from WorkspaceOpts
	Sources map[string]string // repo name -> source repo path (the "remote")
}

// RedirectClones makes git clone each repo's GitHub URL from its local
// source repo instead, so code paths that clone (setup, first use of a
// hidden repo) run without the network.
func (w *Workspace) RedirectClones(t *testing.T) {
	t.Helper()
	i := 0
	for name, src := range w.Sources {
		t.Setenv(fmt.Sprintf("GIT_CONFIG_KEY_%d", i), "url."+src+".insteadOf")
		t.Setenv(fmt.Sprintf("GIT_CONFIG_VALUE_%d", i), fmt.Sprintf("https://github.com/%s/%s.git", w.Org, name))
		i++
	}
	t.Setenv("GIT_CONFIG_COUNT", fmt.Sprint(i))
}

// SetupWorkspace creates a complete workspace with real git repos in t.TempDir().
func SetupWorkspace(t *testing.T, opts WorkspaceOpts) *Workspace {
	t.Helper()

	root := t.TempDir()
	sources := make(map[string]string)
	// What ws records: a test has no daemon to serve them, so it owns them.
	storetest.Own(t, root)

	// Write ws.toml
	writeWSToml(t, root, opts)
	harness.Register(FakeConverser{})

	// Create repos dir
	reposDir := filepath.Join(root, "repos")
	if err := os.MkdirAll(reposDir, 0755); err != nil {
		t.Fatal(err)
	}

	for _, repo := range opts.Repos {
		// The source repo acts as the "remote".
		srcDir := t.TempDir()
		sources[repo.Name] = srcDir
		copyRepo(t, repoTemplate(t, opts.DefaultBranch, repo), srcDir, filepath.Join(reposDir, repo.Name), !repo.NotCloned)
	}

	return &Workspace{Root: root, Org: opts.Org, Sources: sources}
}

func writeWSToml(t *testing.T, root string, opts WorkspaceOpts) {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "[workspace]\norg = %q\ndefault_branch = %q\n\n", opts.Org, opts.DefaultBranch)

	for _, repo := range opts.Repos {
		fmt.Fprintf(&b, "[repos.%s]\n", repo.Name)
		if repo.DisplayName != "" {
			fmt.Fprintf(&b, "display_name = %q\n", repo.DisplayName)
		}
		if len(repo.Aliases) > 0 {
			fmt.Fprintf(&b, "aliases = [%s]\n", quotedList(repo.Aliases))
		}
		if repo.AfterCreate != "" {
			fmt.Fprintf(&b, "after_create = %q\n", repo.AfterCreate)
		}
		if repo.Group != "" {
			fmt.Fprintf(&b, "group = %q\n", repo.Group)
		}
		if repo.Landing != "" {
			fmt.Fprintf(&b, "landing = %q\n", repo.Landing)
		}
		if repo.Verify != "" {
			fmt.Fprintf(&b, "verify = %q\n", repo.Verify)
		}
		b.WriteString("\n")
	}

	for _, group := range opts.HiddenGroups {
		fmt.Fprintf(&b, "[groups.%s]\nhidden = true\n\n", group)
	}

	// The PR writer is FakeConverser: a test never has a real agent
	// write a pull request.
	b.WriteString("[pr_writer]\nharness = \"fake-converse\"\n\n")

	if err := os.WriteFile(filepath.Join(root, "ws.toml"), []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}
}

func quotedList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return strings.Join(quoted, ", ")
}

func initRepo(t *testing.T, dir, defaultBranch string) {
	t.Helper()
	GitCmd(t, dir, "init", "--initial-branch="+defaultBranch)
	GitCmd(t, dir, "config", "user.email", "test@test.com")
	GitCmd(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	GitCmd(t, dir, "add", ".")
	GitCmd(t, dir, "commit", "-m", "initial")
}

// GitCmd runs a git command. Exported so integration tests can use it.
func GitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}
