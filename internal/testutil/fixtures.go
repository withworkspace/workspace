package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// The workspaces and steps the tests of ws's commands share.

// Run runs a ws command that has to work: the test fails if it doesn't.
func Run(t *testing.T, w *Workspace, gh github.Client, args ...string) Result {
	t.Helper()
	r := RunCommand(t, w.Root, gh, args...)
	if r.Err != nil {
		t.Fatalf("ws %s: %v\nstderr: %s", strings.Join(args, " "), r.Err, r.Stderr)
	}
	return r
}

// WithStdin has the next command read text on stdin.
func WithStdin(t *testing.T, text string) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(text)
	f.Seek(0, 0)
	old := os.Stdin
	os.Stdin = f
	t.Cleanup(func() {
		os.Stdin = old
		f.Close()
	})
}

// GitOut is what git prints in dir.
func GitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// Subjects are the subjects of rev's commits in dir, newest first, on one
// line.
func Subjects(t *testing.T, dir, rev string) string {
	t.Helper()
	return strings.ReplaceAll(strings.TrimSpace(GitOut(t, dir, "log", "--format=%s", rev)), "\n", " | ")
}

// CommitIn writes a file in a repo's capsule (or its source) and commits it.
func CommitIn(t *testing.T, dir, path, text, subject string) {
	t.Helper()
	full := filepath.Join(dir, path)
	os.MkdirAll(filepath.Dir(full), 0o755)
	if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	GitCmd(t, dir, "add", "-A")
	GitCmd(t, dir, "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "-m", subject)
}

// CapsuleDir is where a repo's capsule is in w.
func CapsuleDir(w *Workspace, repo, name string) string {
	return filepath.Join(w.Root, "repos", repo, name)
}

// MissionWorkspace is a workspace with two repos, frontend (fe) and
// backend, which has a branch called existing, and no mission or crew in
// the environment.
func MissionWorkspace(t *testing.T) *Workspace {
	t.Helper()
	t.Setenv(mission.EnvMission, "")
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(cli.ShellPWD, "")
	return SetupWorkspace(t, WorkspaceOpts{
		Org:           "test-org",
		DefaultBranch: "main",
		Repos: []RepoOpts{
			{Name: "frontend", Aliases: []string{"fe"}},
			{Name: "backend", Branches: []string{"existing"}},
		},
	})
}

// LandWorkspace is a workspace with a repo that takes commits straight to
// main (records) and one that takes pull requests (api).
func LandWorkspace(t *testing.T, verify string) *Workspace {
	t.Helper()
	t.Setenv(mission.EnvMission, "")
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(mission.EnvRole, "")
	t.Setenv(cli.ShellPWD, "")
	return SetupWorkspace(t, WorkspaceOpts{
		Org:           "test-org",
		DefaultBranch: "main",
		Repos: []RepoOpts{
			{Name: "records", Landing: "direct", Verify: verify},
			{Name: "api"},
		},
	})
}

// LoadMission is the mission called slug in w.
func LoadMission(t *testing.T, w *Workspace, slug string) *mission.Mission {
	t.Helper()
	m, err := mission.Store{Root: w.Root}.Load(slug)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// LogKinds are the kinds of the events in a mission's log, in order.
func LogKinds(t *testing.T, w *Workspace, slug string) []string {
	t.Helper()
	events, err := mission.Store{Root: w.Root}.ReadLog(slug, 0)
	if err != nil {
		t.Fatal(err)
	}
	kinds := make([]string, len(events))
	for i, e := range events {
		kinds[i] = e.Kind
	}
	return kinds
}

// EchoOrchestrator "orchestrates" by printing its role and mission, so tests can see
// it ran with the orchestrator's environment.
type EchoOrchestrator struct{}

func (EchoOrchestrator) Name() string     { return "echo-orchestrator" }
func (EchoOrchestrator) Available() error { return nil }
func (EchoOrchestrator) Start(harness.Brief) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}
func (EchoOrchestrator) Resume(string, string) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}
func (EchoOrchestrator) Orchestrate(r harness.OrchestratorRun) (harness.Launch, error) {
	return harness.Launch{Args: []string{"/bin/sh", "-c", `echo "ran as $WS_ROLE on $WS_MISSION with ` + r.Model + ` in ` + strings.Join(r.Capsules, ",") + `"`, r.Prompt}, Dir: r.Dir}, nil
}
func (EchoOrchestrator) OrchestratorModel() string { return "echo-small" }

// OrchestrateFixture is a MissionWorkspace with a mission, cache, whose
// one capsule is backend/cache-lookup and whose one crew, c1, is at work
// in it: its orchestrator is the EchoOrchestrator.
func OrchestrateFixture(t *testing.T) (*Workspace, mission.Store) {
	t.Helper()
	w := MissionWorkspace(t)
	harness.Register(EchoOrchestrator{})
	t.Setenv(harness.EnvDefault, "echo-orchestrator")
	store := mission.Store{Root: w.Root}
	Run(t, w, nil, "mission", "new", "Cache", "--brief", "## Goal\nCache the lookup.\n## Acceptance\n- tests pass")
	t.Setenv(mission.EnvMission, "cache")
	Run(t, w, nil, "lift", "backend", "cache-lookup")
	t.Setenv(mission.EnvMission, "")
	store.StartCrew("cache", "cursor", "conv-1")
	store.SetCrewProcess("cache", "c1", 0, "backend/cache-lookup")
	store.SaveTranscript("cache", "c1", "hangup", "…\nAll tests pass.\nOpened PR.\n")
	return w, store
}
