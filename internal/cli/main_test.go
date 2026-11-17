package cli

import (
	"os"
	"regexp"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/plugin/fakeplugin"
	"github.com/withworkspace/workspace/internal/testgit"
	"github.com/withworkspace/workspace/internal/ui"
)

func TestMain(m *testing.M) {
	fakeplugin.RunIfAsked() // this binary, started as a plugin (plugin_test.go)
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
	// Never spawn real orchestrators from tests: os.Executable is the test binary.
	orchestrateSpawner = func(root, slug, crewID, trigger string) {}
	// A step's checks run there and then, as the server would run them.
	checkSpawner = func(root, slug, crewID string) {
		if ctx, err := LoadOps(root); err == nil {
			ctx.RunChecks(slug, crewID)
		}
	}
	cleanup := testgit.NoSigning()
	os.Setenv(harness.EnvCursorBin, "/nonexistent/cursor-agent-for-tests") // hermetic doctor output
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// sgr is an SGR (colour and text attribute) sequence; other escapes, such as
// OSC 8 hyperlinks, aren't.
var sgr = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

// plainView renders a model's view content without styles, hyperlinks kept;
// golden files are recorded unstyled.
func plainView(v interface{ render() string }) string {
	return sgr.ReplaceAllString(v.render(), "")
}
