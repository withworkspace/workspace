package ops

import (
	"testing"

	"github.com/withworkspace/workspace/internal/workspace"
)

// The repos view says how work lands in each: through a pull request,
// where nothing says otherwise, or straight to its default branch.
func TestReposSayHowWorkLands(t *testing.T) {
	w, _ := fixture(t, "")
	landing := func() string {
		t.Helper()
		repos := w.Repos()
		if len(repos) != 1 {
			t.Fatalf("repos = %+v", repos)
		}
		return repos[0].Landing
	}
	if got := landing(); got != workspace.LandPR {
		t.Errorf("landing, nothing said = %q, want %q", got, workspace.LandPR)
	}
	w.WS().Landings = map[string]string{"frontend": workspace.LandDirect}
	if got := landing(); got != workspace.LandDirect {
		t.Errorf("landing, said direct = %q, want %q", got, workspace.LandDirect)
	}
}
