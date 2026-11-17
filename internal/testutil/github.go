package testutil

import (
	"testing"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/workspace"
)

// StubGitHubAuth replaces the GitHub auth check with a no-op for the
// duration of the test, so doctor tests don't depend on real gh auth.
func StubGitHubAuth(t *testing.T) {
	t.Helper()
	orig := workspace.CheckGitHubAuthFunc
	workspace.CheckGitHubAuthFunc = func() workspace.CheckResult {
		return workspace.CheckResult{Name: "gh auth", Status: workspace.CheckOK}
	}
	t.Cleanup(func() { workspace.CheckGitHubAuthFunc = orig })
}

// StubClient is a test double for github.Client (github.Stub, which
// packages that testutil itself imports can use too).
type StubClient = github.Stub
