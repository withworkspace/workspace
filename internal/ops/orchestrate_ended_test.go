package ops

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/orchestrator"
	"github.com/withworkspace/workspace/internal/turn"
)

// slowOrchestrator is an orchestrator that takes its time: ws goes first.
type slowOrchestrator struct{ *fakeConverser }

func (slowOrchestrator) Name() string              { return "slow-orchestrator" }
func (slowOrchestrator) OrchestratorModel() string { return "" }
func (slowOrchestrator) Orchestrate(harness.OrchestratorRun) (harness.Launch, error) {
	return harness.Launch{Args: []string{"sleep", "60"}}, nil
}

// An orchestrator's run that ws goes during is ended, and left marked
// underway for the daemon after to run again: it's not the mission's to
// hear it did nothing.
func TestOrchestrate_EndedAsWSGoes(t *testing.T) {
	w, _ := fixture(t, "")
	harness.Register(slowOrchestrator{&fakeConverser{}})
	w.cur.cfg.Orchestrator.Harness = "slow-orchestrator"
	life, end := context.WithCancel(context.Background())
	w.opts.Life = life
	run, err := w.Missions.StartOrchestrating("cache", "a look", orchestrator.RunFor())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- w.Orchestrate(OrchestratorRun{Slug: "cache", Trigger: "done", Auto: true, Since: run.Since})
	}()
	time.Sleep(300 * time.Millisecond)
	end()
	select {
	case err := <-done:
		if !errors.Is(err, turn.ErrEnded) {
			t.Fatalf("err = %v, want ErrEnded", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the run outlived ws")
	}
	m, _ := w.Missions.Load("cache")
	if m.Orchestrating == nil {
		t.Error("its mark was ended: the daemon after won't run it again")
	}
	if q, asking := m.Question(); asking {
		t.Errorf("the mission asks %q", q.Text)
	}
}
