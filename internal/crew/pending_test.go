package crew

import (
	"reflect"
	"testing"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// What a harness's adapter says is left running is kept as the mission
// keeps it; unknown, like nothing, is no work.
func TestLeftRunning(t *testing.T) {
	if got := LeftRunning(harness.Pending{}); got != nil {
		t.Errorf("unknown: %+v", got)
	}
	if got := LeftRunning(harness.Pending{Known: true}); got != nil {
		t.Errorf("nothing: %+v", got)
	}
	got := LeftRunning(harness.Pending{Known: true, Work: []harness.Work{
		{What: "just test", From: harness.FromHarness, Wakes: harness.WakeYes},
		{What: "nohup node server.js", From: harness.FromProcesses, PID: 4242, Wakes: harness.WakeNo},
		{What: "sleep 120", From: harness.FromProcesses, PID: 4343, Wakes: harness.WakeUnknown},
		{What: "watch", From: harness.FromProcesses, PID: 4444},
		{What: "npm run dev", From: harness.FromHarness, Wakes: harness.WakeNo, Call: "call_7"},
	}})
	want := []mission.Work{
		{What: "just test", From: "harness", Wakes: mission.WakeYes},
		{What: "nohup node server.js", From: "processes", PID: 4242, Wakes: mission.WakeNo},
		{What: "sleep 120", From: "processes", PID: 4343, Wakes: mission.WakeUnknown},
		{What: "watch", From: "processes", PID: 4444, Wakes: mission.WakeUnknown},
		{What: "npm run dev", From: "harness", Wakes: mission.WakeNo, Call: "call_7"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("work:\n got %+v\nwant %+v", got, want)
	}
}
