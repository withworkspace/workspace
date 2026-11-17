package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/orchestrator"
)

// Work for an agent that outlives whoever asked for it (the orchestrator's
// runs, conversations' and threads' answers) is a job the workspace's
// server runs (job.Queue): one at a time per mission or conversation, each
// a child of the server. A command asks the server over its socket,
// starting it if it isn't running; inside the server, it's asked directly.

// serverJobs is this process's server's jobs, when it's the workspace's
// server.
var serverJobs *job.Queue

// startJob has the workspace's daemon run a job.
func startJob(root string, j job.Job) error {
	if serverJobs != nil {
		serverJobs.Start(j)
		return nil
	}
	c, err := daemon.Connect(root)
	if err != nil {
		return err
	}
	defer c.Close()
	return c.StartJob(j)
}

// spawnOrchestrator has the server run the orchestrator for an event on a
// mission.
func spawnOrchestrator(root, slug, crewID, trigger string) {
	j := job.Job{Kind: job.Orchestrate, Mission: slug, ID: crewID, Why: trigger}
	if err := startJob(root, j); err != nil {
		execlog.Note(root, "orchestrator "+slug, "couldn't start for %s: %v", orchestrator.Describe(crewID, trigger), err)
	}
}

// spawnChecks has the server run and judge the checks of the step crew
// reported done at.
func spawnChecks(root, slug, crewID string) {
	if err := startJob(root, job.Job{Kind: job.Check, Mission: slug, ID: crewID}); err != nil {
		execlog.Note(root, "checks "+slug, "couldn't start for %s's done: %v", crewID, err)
	}
}

// checkSpawner starts a step's checks; tests replace it.
var checkSpawner = spawnChecks

// spawnAnswer has the server answer a conversation (kind job.Answer) or a
// thread (job.Thread), or draft what a resolved thread decided (job.Draft).
func spawnAnswer(kind string) func(root, slug, id string) {
	return func(root, slug, id string) {
		if err := startJob(root, job.Job{Kind: kind, Mission: slug, ID: id}); err != nil {
			execlog.Note(root, kind+" "+firstNonEmpty(slug, "workspace")+"/"+id, "couldn't start: %v", err)
		}
	}
}

// jobQueue is the server's jobs: each marked underway as it's taken, so
// whoever asked sees it at once, and run in the server.
func jobQueue(root string, logf func(string, ...any)) *job.Queue {
	return &job.Queue{Queued: markJob(root), Run: runJob(root), Log: logf}
}

// markJob marks a job underway as it's taken: an orchestrator's run on its
// mission, an answer on its conversation.
func markJob(root string) func(job.Job) job.Job {
	return func(j job.Job) job.Job {
		switch j.Kind {
		case job.Orchestrate:
			if run, err := (mission.Store{Root: root}).StartOrchestrating(j.Mission, orchestrator.Describe(j.ID, j.Why), orchestrator.RunFor()); err == nil {
				j.Since = run.Since
			}
		case job.Answer, job.Thread:
			conversation.For(root, j.Mission).StartAnswering(j.ID, os.Getpid())
		}
		return j
	}
}

// runJob does a job, in this process.
func runJob(root string) func(job.Job) error {
	return func(j job.Job) error {
		ctx, err := LoadOps(root)
		if err != nil {
			return err
		}
		switch j.Kind {
		case job.Orchestrate:
			w := io.Discard
			if f, err := os.OpenFile(filepath.Join(missionStore(ctx).Dir(), j.Mission, "orchestrator.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
				defer f.Close()
				w = f
			}
			return orchestrate(ctx, orchestrateOptions{Slug: j.Mission, Crew: j.ID, Trigger: j.Why, Auto: true, Since: j.Since}, w)
		case job.Answer, job.Thread:
			return ctx.AnswerConversation(doc.Scope{Mission: j.Mission}, j.ID)
		case job.Draft:
			return ctx.DraftDecision(j.Mission, j.ID)
		case job.Check:
			said, err := ctx.RunChecks(j.Mission, j.ID)
			if said != "" {
				execlog.Note(root, "checks "+j.Mission, "%s", said)
			}
			return err
		}
		return fmt.Errorf("no such job as %q", j.Kind)
	}
}

// reconcileJobs is what's left of jobs a server before this one was
// running when it went: an orchestrator's run still marked underway is
// taken again, as its event wasn't; a conversation's answer is taken up
// again the next time it's said to (an answering mark whose server's gone
// counts for nothing).
func reconcileJobs(root string, q *job.Queue, logf func(string, ...any)) {
	missions, err := mission.Store{Root: root}.InPlay()
	if err != nil {
		return
	}
	for _, m := range missions {
		// A step's checks that were running, or hadn't started, are run.
		brief, _ := mission.Store{Root: root}.ReadBrief(m.Slug)
		if st, crewID, ok := m.UncheckedDone(brief); ok {
			logf("%s's done at step %d of %s wasn't checked: checking it", crewID, st.N, m.Slug)
			q.Start(job.Job{Kind: job.Check, Mission: m.Slug, ID: crewID})
		}
		if m.Orchestrating == nil {
			continue
		}
		mission.Store{Root: root}.EndOrchestrating(m.Slug, *m.Orchestrating)
		logf("the orchestrator was running on %s when the server before this one went: running it again", m.Slug)
		q.Start(job.Job{Kind: job.Orchestrate, Mission: m.Slug, Why: "restarted"})
	}
}
