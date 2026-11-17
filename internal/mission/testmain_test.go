package mission

import (
	"os"
	"testing"
)

// TestMain runs the tests as no crew on no mission. ws sets WS_CREW and
// WS_MISSION for crew, and crew run these tests too: AppendLog would then
// attribute what the tests record to them. A test that wants either sets it.
func TestMain(m *testing.M) {
	os.Unsetenv(EnvCrew)
	os.Unsetenv(EnvMission)
	os.Exit(m.Run())
}
