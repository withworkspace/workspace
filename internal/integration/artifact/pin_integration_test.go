package artifact_test

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

// You pin what you're on and list it; crew can't; you unpin it.
func TestPin_YouPinListAndUnpin(t *testing.T) {
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{Org: "acme", DefaultBranch: "main"})
	slug := strings.TrimSpace(testutil.Run(t, w, nil, "mission", "new", "--no-edit", "Cache the index").Stdout)

	if r := testutil.Run(t, w, nil, "pin"); r.Stdout != "" || !strings.Contains(r.Stderr, "nothing's pinned") {
		t.Errorf("nothing pinned: %q %q", r.Stdout, r.Stderr)
	}
	if r := testutil.Run(t, w, nil, "pin", slug, "demo#12"); !strings.Contains(r.Stderr, "Pinned "+slug+".") || !strings.Contains(r.Stderr, "Pinned demo#12.") {
		t.Errorf("pinning: %s", r.Stderr)
	}
	if got, want := testutil.Run(t, w, nil, "pin").Stdout, slug+"\tCache the index\ndemo#12\n"; got != want {
		t.Errorf("listed:\n got  %q\n want %q", got, want)
	}

	if r := testutil.RunCommand(t, w.Root, nil, "pin", "no-such-mission"); r.Err == nil || !strings.Contains(r.Err.Error(), "nothing called no-such-mission") {
		t.Errorf("pinning what isn't there: %v", r.Err)
	}
	t.Setenv(mission.EnvCrew, "c1")
	if r := testutil.RunCommand(t, w.Root, nil, "pin", "demo#13"); r.Err == nil || !strings.Contains(r.Err.Error(), "only you pin") {
		t.Errorf("crew pinning: %v", r.Err)
	}
	t.Setenv(mission.EnvCrew, "")

	if r := testutil.Run(t, w, nil, "unpin", "demo#12"); !strings.Contains(r.Stderr, "Unpinned demo#12.") {
		t.Errorf("unpinning: %s", r.Stderr)
	}
	if got := testutil.Run(t, w, nil, "pin").Stdout; got != slug+"\tCache the index\n" {
		t.Errorf("listed after unpinning: %q", got)
	}
}
