package mission_test

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

// Crew raising a mission like one in play are refused, told where to note
// what they found; one that's different work is raised with --distinct, and
// you raise what you like.
func TestMissionNewRefusesOneLikeAMissionInPlay(t *testing.T) {
	w := testutil.MissionWorkspace(t)
	testutil.Run(t, w, nil, "mission", "new", "Cache", "--brief", "## Goal\nCache it.")
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c1")
	first := "internal/ops TestLinkNames (refs_test.go groundRepo) commits with the user's own git signing config: 1Password failed"
	testutil.Run(t, w, nil, "mission", "new", first)

	t.Setenv(mission.EnvCrew, "c2")
	again := "Tests in internal/ops sign their commits through 1Password, so TestLinkNames fails; groundRepo in refs_test.go should use testgit.NoSigning"
	r := testutil.RunCommand(t, w.Root, nil, "mission", "new", again)
	if r.Err == nil || !strings.Contains(r.Err.Error(), "this looks like internal-ops-testlinknames-refs-test-go") || !strings.Contains(r.Err.Error(), "ws mission note -m internal-ops-testlinknames-refs-test-go") {
		t.Fatalf("a duplicate: %v", r.Err)
	}
	testutil.Run(t, w, nil, "mission", "new", "--distinct", again)
	testutil.Run(t, w, nil, "mission", "new", "The reader's contents rail scrolls with the text")

	t.Setenv(mission.EnvMission, "")
	t.Setenv(mission.EnvCrew, "")
	testutil.Run(t, w, nil, "mission", "new", "TestLinkNames signs commits through 1Password, refs_test.go groundRepo")
	missions, _ := (mission.Store{Root: w.Root}).InPlay()
	if len(missions) != 5 {
		var slugs []string
		for _, m := range missions {
			slugs = append(slugs, m.Slug)
		}
		t.Errorf("missions: %v, want cache, the first, the distinct one, the reader's and yours", slugs)
	}
}
