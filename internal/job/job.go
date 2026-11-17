// Package job is work for an agent that outlives whoever asked for it: an
// orchestrator's run, a conversation's answer. The workspace's daemon runs
// each as it's asked for, one at a time per subject, so two runs never act
// on the same mission or conversation at once.
package job

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

// Kinds of job.
const (
	Orchestrate = "orchestrate" // the orchestrator, on a mission
	Answer      = "answer"      // a conversation's agent, answering it
	Thread      = "thread"      // a thread's agent, answering it
	Draft       = "draft"       // a resolved thread's agent, drafting what was decided
	Check       = "check"       // a Work step's checks, run and judged once its crew reported done
)

// Job is one piece of work.
type Job struct {
	Kind    string `json:"kind"`
	Mission string `json:"mission,omitempty"` // "" for the workspace's
	ID      string `json:"id,omitempty"`      // the conversation; the orchestrator's crew; the crew whose done is checked
	Why     string `json:"why,omitempty"`     // the orchestrator's trigger: done, ended, answer…
	// Since is when an orchestrator's run was marked underway on its
	// mission, so the run can end that mark and no other.
	Since time.Time `json:"since,omitzero"`
}

// Subject is what the job is about: one job runs on it at a time.
func (j Job) Subject() string {
	switch j.Kind {
	case Orchestrate:
		return "orchestrate " + j.Mission
	case Check:
		return "check " + j.Mission // one round at a time per mission
	}
	return j.Kind + " " + j.Mission + "/" + j.ID
}

func (j Job) String() string {
	if j.Kind == Orchestrate {
		s := "the orchestrator on " + j.Mission
		if j.ID != "" || j.Why != "" {
			s += " for " + j.ID + " (" + j.Why + ")"
		}
		return s
	}
	return fmt.Sprintf("%s %s/%s", j.Kind, j.Mission, j.ID)
}

// Queue runs jobs: each subject's one at a time, in the order asked for. A
// job asked for while its subject's is running waits; a newer one for the
// same subject takes the place of one waiting, as the agent will look at
// everything there is by then.
type Queue struct {
	// Run does a job. A panic in it fails that job alone.
	Run func(Job) error
	// Queued is told each job as it's taken, before it runs, and gives it
	// back as it's to run: marked underway, say.
	Queued func(Job) Job
	// Log is told what happened to each job.
	Log func(format string, args ...any)

	mu      sync.Mutex
	running map[string]bool
	waiting map[string]Job
	wg      sync.WaitGroup
}

// Start takes a job, to run now or once its subject's running job is done.
func (q *Queue) Start(j Job) {
	if q.Queued != nil {
		j = q.Queued(j)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.running == nil {
		q.running, q.waiting = map[string]bool{}, map[string]Job{}
	}
	s := j.Subject()
	if q.running[s] {
		if old, ok := q.waiting[s]; ok {
			q.logf("%s waits in place of %s", j, old)
		}
		q.waiting[s] = j
		return
	}
	q.running[s] = true
	q.wg.Add(1)
	go q.loop(s, j)
}

// loop runs a subject's jobs until none is waiting.
func (q *Queue) loop(s string, j Job) {
	defer q.wg.Done()
	for {
		q.run(j)
		q.mu.Lock()
		next, ok := q.waiting[s]
		delete(q.waiting, s)
		if !ok {
			delete(q.running, s)
			q.mu.Unlock()
			return
		}
		q.mu.Unlock()
		j = next
	}
}

func (q *Queue) run(j Job) {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			q.logf("%s failed: %v\n%s", j, r, debug.Stack())
		}
	}()
	if err := q.Run(j); err != nil {
		q.logf("%s failed after %s: %v", j, time.Since(start).Round(time.Second), err)
		return
	}
	q.logf("%s done in %s", j, time.Since(start).Round(time.Second))
}

// Busy is whether any job is running.
func (q *Queue) Busy() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.running) > 0
}

// Wait waits for every job taken so far, and those waiting behind them.
func (q *Queue) Wait() { q.wg.Wait() }

func (q *Queue) logf(format string, args ...any) {
	if q.Log != nil {
		q.Log(format, args...)
	}
}
