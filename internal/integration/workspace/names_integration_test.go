package workspace_test

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/testutil"
)

// What a mission's brief names is listed under the thing it names, as the
// mission's brief; and the brief is read by the ref it's listed under.
func TestShow_ABriefThatNamesAPluginsThing(t *testing.T) {
	root := naming(t)
	res := testutil.RunCommand(t, root, nil, "mission", "new", "Fix the refresh", "--brief", "## Goal\nWhat RAM-12 asks for, and no more.")
	if res.Err != nil {
		t.Fatalf("new: %v\n%s", res.Err, res.Stderr)
	}
	slug := "fix-the-refresh"
	// The brief was written outside the records: it's read as it's asked after.
	thing := testutil.RunCommand(t, root, nil, "show", "RAM-12", "-f", "llm")
	for _, want := range []string{
		"Said of it in the workspace",
		"Named in (1, 1 still standing):",
		"brief  a mission's brief, mission " + slug + ", in play: Fix the refresh",
	} {
		if !strings.Contains(thing.Stdout, want) {
			t.Errorf("RAM-12 lacks %q:\n%s", want, thing.Stdout)
		}
	}
	brief := testutil.RunCommand(t, root, nil, "show", "brief", "--mission", slug, "-f", "llm")
	if brief.Err != nil {
		t.Fatalf("show brief: %v\n%s", brief.Err, brief.Stderr)
	}
	for _, want := range []string{"The brief of mission " + slug + ": Fix the refresh", "What RAM-12 asks for, and no more.", "ws show --mission " + slug} {
		if !strings.Contains(brief.Stdout, want) {
			t.Errorf("the brief lacks %q:\n%s", want, brief.Stdout)
		}
	}
}
