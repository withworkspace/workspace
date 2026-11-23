package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

// markedHost is a host that keeps what ws had marked on the cache mission
// as it was asked to start each crew.
type markedHost struct {
	w      *Workspace
	moving []bool
}

func (h *markedHost) Launch(crew.Spec) (int, error) {
	m, _ := h.w.Missions.Load("cache")
	c := m.Crew[len(m.Crew)-1] // the crew being started hold the launch's mark by now
	h.moving = append(h.moving, m.Moving != nil && c.Starting != nil)
	return 4242, nil
}
func (h *markedHost) Stop(string) error                  { return crew.ErrNotRunning }
func (h *markedHost) Say(string, string) (string, error) { return "", crew.ErrNotRunning }

// A step ws passes and the next it reaches are one move, marked from
// before the step's passed until the next step's crew are being started:
// the crew being started are marked starting while the move's mark still
// stands, and the move's is gone once it's over.
func TestMoving_HeldFromTheChecksToTheNextStepsCrew(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	ws := &workspace.Workspace{Root: root, RepoNames: []string{"frontend"}}
	dir := filepath.Join(ws.RepoDir("frontend"), "one")
	os.MkdirAll(dir, 0o755)
	host := &markedHost{}
	w := New(&config.Config{}, ws, nil, Options{Host: host})
	host.w = w
	harness.Register(fakeHarness{})
	w.Config().Crew.Harness = "fake-crew"
	m, _ := w.Missions.Create("Cache")
	w.Attach(m.Slug, "frontend", "one", false, AsYou)
	plan := "## Z\n- Cached.\n## Stops early\n- None.\n## Steps\n" +
		"1. Work: cache the lookup.\n   - in: `frontend/one`\n   - check: `true` passes\n" +
		"2. Work: test it.\n   - in: `frontend/one`\n" +
		"3. Review: the cache.\n"
	if err := w.Missions.WritePlan("cache", plan); err != nil {
		t.Fatal(err)
	}
	w.Missions.StartCrew("cache", "shell", "")
	w.Missions.SetCrewProcess("cache", "c1", 0, "frontend/one")
	w.Missions.JoinStage("cache", "c1", 1)
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")

	if said := w.CrewDone("cache", "c1"); !strings.Contains(said, "passed step 1") {
		t.Fatalf("CrewDone said %q", said)
	}
	if len(host.moving) != 1 || !host.moving[0] {
		t.Errorf("as step 2's crew were started, the move and their starting were both marked: %v", host.moving)
	}
	got, _ := w.Missions.Load("cache")
	if got.Moving != nil {
		t.Errorf("still moving once it's over: %+v", got.Moving)
	}

	// Going on at a step of yours is the same move.
	w.Missions.SetCrewStatus("cache", "c2", mission.CrewDone, "tested")
	if _, err := w.AcceptStage("cache", AsOrchestrator, "tested"); err != nil {
		t.Fatal(err)
	}
	if got, _ := w.Missions.Load("cache"); got.Moving != nil || len(got.Tasks) != 1 {
		t.Errorf("at your review: moving %+v, tasks %+v", got.Moving, got.Tasks)
	}
}
