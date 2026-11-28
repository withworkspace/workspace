// Package job is work for an agent that outlives whoever asked for it: an
// orchestrator's run, a conversation's answer. The workspace's daemon runs
// each as it's asked for, one at a time per lane, so two runs never act
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
	PR          = "pr"          // a pull request the PR writer writes, then ws opens
	Decisions   = "decisions"   // the decision look, on crew's decisions on a mission
	Verify      = "verify"      // a Verify step's verifier started again once the crew it sent back report done
)

// Job is one piece of work.
type Job struct {
	Kind    string `json:"kind"`
	Mission string `json:"mission,omitempty"` // "" for the workspace's
	ID      string `json:"id,omitempty"`      // the conversation; the orchestrator's crew; the crew whose done is checked; the capsule whose pull request is written
	Why     string `json:"why,omitempty"`     // the orchestrator's trigger: done, ended, answer…
	// Since is when an orchestrator's run was marked underway on its
	// mission, so the run can end that mark and no other.
	Since time.Time `json:"since,omitzero"`
}

// Subject is what the job is about: a newer job for the same subject
// takes the place of one waiting, as the agent will look at everything
// there is by then.
func (j Job) Subject() string {
	switch j.Kind {
	case Orchestrate:
		return "orchestrate " + j.Mission
	case Check:
		return "check " + j.Mission // one round at a time per mission
	}
	return j.Kind + " " + j.Mission + "/" + j.ID
}

// Lane is what the job acts on: one job runs in a lane at a time. A
// mission's orchestrator, its checks and its verifier's start again all
// move its plan on and launch its crew, so they take turns on the mission; a conversation's answer,
// its thread's and its draft each hold the conversation's answering mark,
// so they take turns on the conversation.
func (j Job) Lane() string {
	switch j.Kind {
	case Orchestrate, Check, Verify:
		return "mission " + j.Mission
	case Answer, Thread, Draft:
		return "conversation " + j.Mission + "/" + j.ID
	}
	return j.Subject()
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

// Queue runs jobs: each lane's one at a time, in the order asked for. A
// job asked for while its lane's is running waits; a newer one for the
// same subject takes the place of one waiting (Subject).
type Queue struct {
	// Run does a job. A panic in it fails that job alone.
	Run func(Job) error
	// Queued is told each job as it's taken, before it runs, and gives it
	// back as it's to run: marked underway, say. It's told them one at a
	// time, in the order they're queued, so the mark a job is given is
	// never one a job taken after it has written over.
	Queued func(Job) Job
	// Log is told what happened to each job.
	Log func(format string, args ...any)

	taking  sync.Mutex // held from a job's marking until it's queued
	mu      sync.Mutex
	running map[string]bool  // by lane
	waiting map[string][]Job // by lane, in the order they're to run
	stopped bool             // Stop: nothing more's run
	wg      sync.WaitGroup
}

// Start takes a job, to run now or once its lane's running job is done.
func (q *Queue) Start(j Job) {
	q.taking.Lock()
	defer q.taking.Unlock()
	if q.Queued != nil {
		j = q.Queued(j)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.stopped {
		q.logf("%s isn't run: ws is going", j)
		return
	}
	if q.running == nil {
		q.running, q.waiting = map[string]bool{}, map[string][]Job{}
	}
	lane := j.Lane()
	if q.running[lane] {
		for i, old := range q.waiting[lane] {
			if old.Subject() == j.Subject() {
				q.logf("%s waits in place of %s", j, old)
				q.waiting[lane][i] = j
				return
			}
		}
		q.waiting[lane] = append(q.waiting[lane], j)
		return
	}
	q.running[lane] = true
	q.wg.Add(1)
	go q.loop(lane, j)
}

// loop runs a lane's jobs until none is waiting.
func (q *Queue) loop(lane string, j Job) {
	defer q.wg.Done()
	for {
		q.run(j)
		q.mu.Lock()
		next := q.waiting[lane]
		if len(next) == 0 || q.stopped {
			delete(q.waiting, lane)
			delete(q.running, lane)
			q.mu.Unlock()
			return
		}
		j, q.waiting[lane] = next[0], next[1:]
		q.mu.Unlock()
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

// Stop has the queue take no more jobs, and drop those waiting: what's
// running is let finish (it's ended by ending what it's run for). It
// waits up to grace for them, and says whether they finished. What a job
// dropped was for is the next process's to take up: its mark is still on
// it, or it's asked for again.
func (q *Queue) Stop(grace time.Duration) bool {
	q.mu.Lock()
	q.stopped = true
	for lane, js := range q.waiting {
		for _, j := range js {
			q.logf("%s isn't run: ws is going", j)
		}
		delete(q.waiting, lane)
	}
	q.mu.Unlock()
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(grace):
		return false
	}
}

func (q *Queue) logf(format string, args ...any) {
	if q.Log != nil {
		q.Log(format, args...)
	}
}
