package daemon

import (
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// A job asked for from a command reaches the server over its socket, is
// marked underway as it's taken, and runs there.
func TestJobs_RunInTheServer(t *testing.T) {
	ws := jobsWorkspace(t)
	slug := seedJobsMission(t, ws.Root)
	srv := &Server{Root: ws.Root, IdleTimeout: time.Minute}
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

	c, err := Dial(ws.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.StartJob(job.Job{Kind: job.Orchestrate, Mission: slug, ID: "c1", Why: "done"}); err != nil {
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
	ws := jobsWorkspace(t)
	slug := seedJobsMission(t, ws.Root)
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
	ws := jobsWorkspace(t)
	slug := seedJobsMission(t, ws.Root)
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

// What a server before this one had marked underway on a mission, and
// didn't live to clear, is cleared: the planner's turn went with it, and
// the crew it was starting were never recorded. Neither lapses with time,
// so nothing else would.
func TestJobs_MarksLeftByAServerThatWentAreCleared(t *testing.T) {
	ws := jobsWorkspace(t)
	slug := seedJobsMission(t, ws.Root)
	store := mission.Store{Root: ws.Root}
	if _, err := store.StartPlanning(slug, "t1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StartLaunching(slug, "step 1"); err != nil {
		t.Fatal(err)
	}
	q := &job.Queue{Run: func(j job.Job) error { return nil }}
	reconcileJobs(ws.Root, q, t.Logf)
	q.Wait()
	if m, _ := store.Load(slug); m.Planning != nil || m.Launching != nil {
		t.Errorf("still marked: planning %+v, launching %+v", m.Planning, m.Launching)
	}
}

type jobsRoot struct{ Root string }

// jobsWorkspace is a workspace whose records the test owns.
func jobsWorkspace(t *testing.T) jobsRoot {
	t.Helper()
	root := t.TempDir()
	storetest.Own(t, root)
	return jobsRoot{Root: root}
}

// seedJobsMission makes a mission, and returns its slug.
func seedJobsMission(t *testing.T, root string) string {
	t.Helper()
	m, err := mission.Store{Root: root}.Create("Served")
	if err != nil {
		t.Fatal(err)
	}
	return m.Slug
}
