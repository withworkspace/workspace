package conversation

import (
	"os"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
)

// TestMain runs the tests as no crew on no mission. ws sets WS_CREW and
// WS_MISSION for crew, and crew run these tests too: mission.AppendLog would
// then attribute what the tests record to them.
func TestMain(m *testing.M) {
	os.Unsetenv(mission.EnvCrew)
	os.Unsetenv(mission.EnvMission)
	os.Exit(m.Run())
}
