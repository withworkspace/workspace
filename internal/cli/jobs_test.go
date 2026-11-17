package cli

import (
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
)

// A job asked for from a command reaches the server over its socket, is
// marked underway as it's taken, and runs there.
func TestJobs_RunInTheServer(t *testing.T) {
	ws := serverWorkspace(t, map[string][]string{"frontend": {"work"}})
	slug := seedMission(t, ws, "Served", "")
	srv := &daemon.Server{Root: ws.Root, IdleTimeout: time.Minute}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var ran []job.Job
	marked := false
	srv.Jobs = &job.Queue{
		Queued: func(j job.Job) job.Job {
			j = markJob(ws.Root)(j)
			m, _ := mission.Store{Root: ws.Root}.Load(slug)
			mu.Lock()
			marked = m.Orchestrating != nil
			mu.Unlock()
			return j
		},
		Run: func(j job.Job) error { mu.Lock(); ran = append(ran, j); mu.Unlock(); return nil },
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Shutdown("test over") })

	if err := startJob(ws.Root, job.Job{Kind: job.Orchestrate, Mission: slug, ID: "c1", Why: "done"}); err != nil {
		t.Fatal(err)
	}
	srv.Jobs.Wait()
	mu.Lock()
	defer mu.Unlock()
	if len(ran) != 1 || ran[0].ID != "c1" || ran[0].Since.IsZero() || !marked {
		t.Errorf("ran %+v, marked %v", ran, marked)
	}
}

// A server that went while the orchestrator ran leaves its run marked
// underway: the next server clears the mark and runs it again.
func TestJobs_ARunCutShortIsTakenAgain(t *testing.T) {
	ws := serverWorkspace(t, map[string][]string{"frontend": {"work"}})
	slug := seedMission(t, ws, "Served", "")
	store := mission.Store{Root: ws.Root}
	if _, err := store.StartOrchestrating(slug, "c1 (done)", time.Hour); err != nil {
		t.Fatal(err)
	}
	var ran []job.Job
	q := &job.Queue{Run: func(j job.Job) error { ran = append(ran, j); return nil }}
	reconcileJobs(ws.Root, q, t.Logf)
	q.Wait()
	if m, _ := store.Load(slug); m.Orchestrating != nil {
		t.Errorf("still marked underway: %+v", m.Orchestrating)
	}
	if len(ran) != 1 || ran[0].Mission != slug || ran[0].Why != "restarted" {
		t.Errorf("ran %+v", ran)
	}
}

// Two events at once on one mission: one run, then one more, never two
// together; a third while one waits takes the waiting one's place.
func TestJobs_OneRunAtATimePerMission(t *testing.T) {
	ws := serverWorkspace(t, map[string][]string{"frontend": {"work"}})
	slug := seedMission(t, ws, "Served", "")
	release := make(chan struct{})
	var mu sync.Mutex
	running, most := 0, 0
	var ran []string
	q := &job.Queue{Run: func(j job.Job) error {
		mu.Lock()
		running++
		most = max(most, running)
		ran = append(ran, j.ID)
		mu.Unlock()
		if j.ID == "c1" {
			<-release
		}
		mu.Lock()
		running--
		mu.Unlock()
		return nil
	}}
	q.Start(job.Job{Kind: job.Orchestrate, Mission: slug, ID: "c1", Why: "done"})
	time.Sleep(50 * time.Millisecond)
	q.Start(job.Job{Kind: job.Orchestrate, Mission: slug, ID: "c2", Why: "done"})
	q.Start(job.Job{Kind: job.Orchestrate, Mission: slug, ID: "c3", Why: "done"})
	close(release)
	q.Wait()
	mu.Lock()
	defer mu.Unlock()
	if most != 1 || len(ran) != 2 || ran[0] != "c1" || ran[1] != "c3" {
		t.Errorf("ran %v, %d at once", ran, most)
	}
}
