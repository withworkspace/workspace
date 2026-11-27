package orchestrate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

// The orchestrator's flag is a question with answers to pick; answering it
// clears the flag and runs the orchestrator again with the answer.
func TestFlag_AnsweringRunsTheOrchestratorWithTheAnswer(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	runs, restore := cli.CaptureOrchestratorRuns()
	defer restore()

	t.Setenv("WS_ROLE", "orchestrator")
	testutil.Run(t, w, nil, "mission", "flag", "-m", "cache", "Commit and open PRs?", "--option", "Commit both and open PRs", "--option", "Leave them uncommitted")
	t.Setenv("WS_ROLE", "")
	if m, _ := store.Load("cache"); m.Asking() == nil || len(m.Asking().Options) != 2 {
		t.Fatalf("flag = %+v", m.Asking())
	}

	t.Setenv(mission.EnvCrew, "c1")
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "answer", "-m", "cache", "yes"); r.Err == nil || !strings.Contains(r.Err.Error(), "only the user answers") {
		t.Errorf("crew answering: %v", r.Err)
	}
	t.Setenv(mission.EnvCrew, "")

	r := testutil.Run(t, w, nil, "mission", "answer", "-m", "cache", "Commit both and open PRs")
	if !strings.Contains(r.Stderr, "passed to the orchestrator") || strings.Join(*runs, ",") != "cache//answer" {
		t.Errorf("stderr %q, runs %v", r.Stderr, *runs)
	}
	m, _ := store.Load("cache")
	if m.Asking() != nil || m.Answer == nil || m.Answer.Question != "Commit and open PRs?" || m.Answer.Text != "Commit both and open PRs" {
		t.Errorf("after answering: flag %+v answer %+v", m.Asking(), m.Answer)
	}

	prompt := testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--trigger", "answer", "--print-prompt").Stdout
	for _, want := range []string{
		"You asked: Commit and open PRs?", "They answered: Commit both and open PRs", "Act on the answer now",
		`--option "<answer>"`, "don't flag the same question",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}

	if r := testutil.RunCommand(t, w.Root, nil, "mission", "answer", "-m", "cache", "again"); r.Err == nil || !strings.Contains(r.Err.Error(), "nothing to answer") {
		t.Errorf("answering twice: %v", r.Err)
	}
}

func TestFlag_AnswerWithTheOrchestratorOff(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	runs, restore := cli.CaptureOrchestratorRuns()
	defer restore()
	os.WriteFile(filepath.Join(w.Root, "ws.local.toml"), []byte("[orchestrator]\nenabled = false\n"), 0o644)
	store.Ask("cache", "Which way?", "orchestrator")
	r := testutil.Run(t, w, nil, "mission", "answer", "-m", "cache", "Left")
	if !strings.Contains(r.Stderr, "orchestrator is off ([orchestrator] enabled = false)") || len(*runs) != 0 {
		t.Errorf("stderr %q, runs %v", r.Stderr, *runs)
	}
}

// Crew the user asked to answer the question answer it like the user: the
// orchestrator runs once, with their answer, and their part is done.
func TestFlag_AnswerByCrewAskedToAnswer(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	runs, restore := cli.CaptureOrchestratorRuns()
	defer restore()
	store.Ask("cache", "Commit and open PRs?", "orchestrator", "Commit both", "Leave them")
	store.StartCrewAs("cache", "shell", "", mission.RoleAnswer) // c2, asked to answer
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c2")

	testutil.Run(t, w, nil, "mission", "answer", "Commit both: the tests pass and the proposal matches")
	m, _ := store.Load("cache")
	c2 := m.Crew[1]
	if m.Asking() != nil || m.Answer == nil || m.Answer.By != "c2" || c2.Status != mission.CrewDone || !strings.HasPrefix(c2.Note, "answered: Commit both") {
		t.Fatalf("flag %+v answer %+v c2 %+v", m.Asking(), m.Answer, c2)
	}
	// Habit: it reports done too. The answer already ran the orchestrator.
	testutil.Run(t, w, nil, "mission", "done", "answered the orchestrator")
	if strings.Join(*runs, ",") != "cache//answer" {
		t.Errorf("orchestrator runs = %v, want just the answer's", *runs)
	}
	t.Setenv(mission.EnvMission, "")
	t.Setenv(mission.EnvCrew, "")
	prompt := testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--trigger", "answer", "--print-prompt").Stdout
	if !strings.Contains(prompt, "Crew c2, which the user asked to answer it for them, answered: Commit both: the tests pass") {
		t.Errorf("prompt doesn't say who answered:\n%s", prompt)
	}
}
