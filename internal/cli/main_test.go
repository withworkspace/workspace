package cli

import (
	"os"
	"regexp"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/testgit"
	"github.com/withworkspace/workspace/internal/ui"
)

func TestMain(m *testing.M) {
	ui.Profile = colorprofile.NoTTY
	NoDaemon()
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
