package testutil

import (
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/plugin/fakeplugin"
	"github.com/withworkspace/workspace/internal/testgit"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

// inMain is whether Main is running this binary's tests.
var inMain bool

// Main is the TestMain of a package that tests ws's commands: it runs the
// tests where nothing of the machine's or the network's changes what they
// see, and removes what they shared when they're over.
//
//	func TestMain(m *testing.M) { testutil.Main(m) }
func Main(m *testing.M) {
	fakeplugin.RunIfAsked() // this binary, started as a plugin
	// A test's stand-in agent, recording what its ws mission note would:
	// the test binary run again, reaching the test's records as a command
	// does (WS_TEST_NOTE=<root> <binary> <slug> <text>).
	if root := os.Getenv("WS_TEST_NOTE"); root != "" {
		if err := (mission.Store{Root: root}).AppendLog(os.Args[1], mission.Event{Kind: mission.EventNote, Text: os.Args[2]}); err != nil {
			os.Stderr.WriteString(err.Error() + "\n")
			os.Exit(1)
		}
		os.Exit(0)
	}
	ui.Profile = colorprofile.NoTTY
	cli.NoDaemon()
	// Tests make commits: never ask the machine's signing agent for them.
	cleanup := testgit.NoSigning()
	// Hermetic: whether the Cursor CLI is installed here, or gh is signed
	// in (which doctor asks GitHub), mustn't change what doctor reports.
	os.Setenv(harness.EnvCursorBin, "/nonexistent/cursor-agent-for-tests")
	workspace.CheckGitHubAuthFunc = func() workspace.CheckResult {
		return workspace.CheckResult{Name: "gh auth", Status: workspace.CheckOK}
	}
	// Crew are typed their prompt as soon as their screen has been still a
	// moment: a test's stand-in draws at once.
	crew.PromptSettle = 50 * time.Millisecond
	inMain = true
	code := m.Run()
	removeShared()
	cleanup()
	os.Exit(code)
}
