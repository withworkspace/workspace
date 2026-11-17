package ops

import (
	"errors"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/workspace"
)

// fixture is a workspace with a repo, frontend, with capsules as folders,
// and a mission, "Cache", with the brief given.
func fixture(t *testing.T, brief string, capsules ...string) (*Workspace, *[]string) {
	t.Helper()
	root := t.TempDir()
	storetest.Own(t, root)
	ws := &workspace.Workspace{Root: root, RepoNames: []string{"frontend"}}
	for _, c := range capsules {
		os.MkdirAll(filepath.Join(ws.RepoDir("frontend"), c), 0o755)
	}
	var runs []string
	w := New(&config.Config{}, ws, nil, Options{Orchestrate: func(slug, crewID, trigger string) { runs = append(runs, slug+"/"+trigger) }})
	m, err := w.Missions.Create("Cache")
	if err != nil {
		t.Fatal(err)
	}
	if brief != "" {
		w.Missions.WriteBrief(m.Slug, brief)
	}
	for _, c := range capsules {
		if _, err := w.Attach(m.Slug, "frontend", c, false, AsYou); err != nil {
			t.Fatal(err)
		}
	}
	return w, &runs
}

func refusedWith(t *testing.T, err error, code string) {
	t.Helper()
	var r *Refused
	if !errors.As(err, &r) || r.Code != code {
		t.Errorf("err = %v, want refused %s", err, code)
	}
}

// Who may do what is ops's to say, for every front end.
func TestActors(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n")
	_, err := w.AcceptStage("cache", AsCrew("c1"), "done")
	refusedWith(t, err, "not-yours")
	_, err = w.Answer("cache", AsOrchestrator, "yes")
	refusedWith(t, err, "not-yours")
	_, err = w.SetBrief("cache", "## Goal\nMine now.\n", AsCrew("c1"))
	refusedWith(t, err, "brief-is-yours")
	if _, err := w.AcceptStage("nope", AsYou, ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such mission: %v", err)
	}
	if _, err := w.Answer("cache", AsYou, "  "); err == nil {
		t.Error("answered with nothing")
	} else if _, ok := err.(*Invalid); !ok {
		t.Errorf("an empty answer is %T", err)
	}
}

// Everything you do to a mission starts the orchestrator's count of crew
// started without you again; the orchestrator's own doing doesn't.
func TestYouActing(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "one")
	count := func() int {
		m, _ := w.Missions.Load("cache")
		return m.Dispatches
	}
	set := func() {
		w.Missions.Update("cache", func(m *mission.Mission) error { m.Dispatches = 2; return nil })
	}
	for _, step := range []struct {
		name string
		do   func() error
	}{
		{"park", func() error { _, err := w.SetState("cache", mission.Parked, AsYou); return err }},
		{"reopen", func() error { _, err := w.Reopen("cache", mission.Parked, AsYou); return err }},
		{"brief", func() error { _, err := w.SetBrief("cache", "## Goal\nCache it well.\n", AsYou); return err }},
		{"title", func() error { return w.SetTitle("cache", "Cache lookups", AsYou) }},
		{"detach", func() error { return w.Detach("cache", "frontend", "one", AsYou) }},
	} {
		name, do := step.name, step.do
		set()
		if err := do(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := count(); got != 0 {
			t.Errorf("after %s, %d dispatches", name, got)
		}
	}
	set()
	w.Missions.Ask("cache", "Ship it?", "orchestrator")
	if _, err := w.Dismiss("cache", AsOrchestrator); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 2 {
		t.Errorf("the orchestrator taking its question back reset the count: %d", got)
	}
}

func TestDismiss(t *testing.T) {
	w, _ := fixture(t, "")
	_, err := w.Dismiss("cache", AsYou)
	refusedWith(t, err, "nothing-to-dismiss")
	w.Missions.QueueDispatch("cache", &mission.Dispatch{Text: "build it"})
	w.Missions.Ask("cache", "Ship it?", "orchestrator")
	if d, err := w.Dismiss("cache", AsYou); err != nil || d.String() != "Question dismissed." {
		t.Errorf("first: %v, %v", d, err)
	}
	// The orchestrator only takes back a question: crew queued are yours.
	_, err = w.Dismiss("cache", AsOrchestrator)
	refusedWith(t, err, "nothing-to-dismiss")
	if d, err := w.Dismiss("cache", AsYou); err != nil || d.String() != "Queued crew dismissed." {
		t.Errorf("second: %v, %v", d, err)
	}
}

// A check-in's Go on starts the next stage as ws's own dispatch; any
// other answer goes to the orchestrator, and is a decision.
func TestAnswer(t *testing.T) {
	w, runs := fixture(t, "## Stages\n1. Propose it in `frontend/one`. Check in with me.\n2. Build it in `frontend/two`.\n", "one", "two")
	host := &fakeHost{}
	w.opts.Host = host
	harness.Register(fakeHarness{})
	w.Config().Crew.Harness = "fake-crew"
	w.Config().Orchestrator.Dispatch = "auto"
	acc, err := w.AcceptStage("cache", AsYou, "")
	if err != nil || !strings.Contains(acc.Next, "the user is asked before stage 2") {
		t.Fatalf("accept: %+v, %v", acc, err)
	}
	if a, err := w.Answer("cache", AsYou, "go on"); err != nil || a.Next != "started stage 2: c1 in frontend/two" {
		t.Errorf("go on: %+v, %v", a, err)
	}
	if len(host.specs) != 1 || host.specs[0].Capsule != "frontend/two" || !strings.Contains(host.specs[0].Prompt, "You're on stage 2") || len(*runs) != 0 {
		// Not the specs whole: their environment is the machine's.
		var started []string
		for _, s := range host.specs {
			started = append(started, s.Key()+" in "+s.Capsule)
		}
		t.Errorf("started %v, orchestrator ran %v", started, *runs)
	}
	if all, _ := w.Missions.Decisions("cache"); len(all) != 0 {
		t.Errorf("a check-in's go on is a decision: %+v", all)
	}

	w.Missions.Ask("cache", "Which cache?", "orchestrator", "Redis", "In memory")
	if a, err := w.Answer("cache", AsYou, "In memory"); err != nil || a.Next != "the orchestrator is acting on it" {
		t.Errorf("an answer: %+v, %v", a, err)
	}
	if len(*runs) != 1 || (*runs)[0] != "cache/answer" {
		t.Errorf("orchestrator runs = %v", *runs)
	}
	if all, _ := w.Missions.Decisions("cache"); len(all) != 1 || all[0].What != "In memory" {
		t.Errorf("decisions = %+v", all)
	}
	_, err = w.Answer("cache", AsYou, "again")
	refusedWith(t, err, "nothing-asked")
}

func TestAttach(t *testing.T) {
	w, _ := fixture(t, "", "one")
	other, _ := w.Missions.Create("Other")
	_, err := w.Attach(other.Slug, "frontend", "one", false, AsYou)
	refusedWith(t, err, "capsule-taken")
	if a, err := w.Attach(other.Slug, "frontend", "one", true, AsYou); err != nil || a.From != "cache" {
		t.Errorf("moving it: %+v, %v", a, err)
	}
	if m, _ := w.Missions.Load("cache"); m.HasCapsule("frontend", "one") {
		t.Error("moved, but the old mission still has it")
	}
	err = w.Detach("cache", "frontend", "one", AsYou)
	refusedWith(t, err, "not-attached")
	if _, err := w.Attach("cache", "frontend", workspace.GroundDir, false, AsYou); err == nil {
		t.Error("attached the ground")
	}
}

// A landing is public the moment it's made: only you land.
func TestMayLand(t *testing.T) {
	refusedWith(t, MayLand(AsCrew("c2")), "not-yours")
	refusedWith(t, MayLand(AsOrchestrator), "not-yours")
	if err := MayLand(AsYou); err != nil {
		t.Errorf("you may land: %v", err)
	}
	w, _ := fixture(t, "")
	_, err := w.Land("frontend", "one", AsCrew("c2"), workspace.LandOptions{})
	refusedWith(t, err, "not-yours")
}

// fakeHost starts nothing, and remembers what it was asked to.
type fakeHost struct{ specs []crew.Spec }

func (h *fakeHost) Launch(s crew.Spec) (int, error)    { h.specs = append(h.specs, s); return 4242, nil }
func (h *fakeHost) Stop(string) error                  { return crew.ErrNotRunning }
func (h *fakeHost) Say(string, string) (string, error) { return "", crew.ErrNotRunning }

// fakeHarness is crew that would run a shell, its prompt typed in.
type fakeHarness struct{}

func (fakeHarness) Name() string     { return "fake-crew" }
func (fakeHarness) Available() error { return nil }
func (fakeHarness) Start(b harness.Brief) (harness.Launch, error) {
	return harness.Launch{Args: []string{"/bin/sh"}, Dir: b.Dir, Prompt: b.Prompt}, nil
}
func (fakeHarness) Resume(string, string) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}

// The orchestrator looks at each event once, and only when it's on.
func TestNotify(t *testing.T) {
	w, runs := fixture(t, "")
	w.Missions.StartCrew("cache", "shell", "")
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "built it")
	done := mission.OrchestratorEvent{Crew: "c1", Trigger: "done"}
	if !w.Notify("cache", done) || w.Notify("cache", done) {
		t.Error("not looked at once")
	}
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "built it again") // a new event
	if !w.Notify("cache", done) {
		t.Error("a new event wasn't looked at")
	}
	if len(*runs) != 2 {
		t.Errorf("runs = %v", *runs)
	}
	off := false
	w.Config().Orchestrator.Enabled = &off
	if w.Notify("cache", mission.OrchestratorEvent{Trigger: "review"}) {
		t.Error("looked at with the orchestrator off")
	}
}
