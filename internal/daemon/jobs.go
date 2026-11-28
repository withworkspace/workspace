package daemon

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/orchestrator"
)

// Work for an agent that outlives whoever asked for it (the orchestrator's
// runs, conversations' and threads' answers, a step's checks, a pull
// request's writing) is a job the daemon runs (job.Queue): one at a time
// per mission or conversation, each a child of the server. A command asks
// over the socket (Client.StartJob); the daemon's own operations start
// them on the queue directly.

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
			if _, err := conversation.For(root, j.Mission).StartAnswering(j.ID, os.Getpid()); err != nil {
				log.Printf("conversation %s: couldn't mark it being answered: %v", j.ID, err)
			}
		}
		return j
	}
}

// runJob does a job, in this process, through the daemon's operations.
func (d *daemon) runJob(j job.Job) error {
	ctx, root := d.w, d.cfg.Root
	switch j.Kind {
	case job.Orchestrate:
		w := io.Discard
		if f, err := os.OpenFile(filepath.Join(ctx.Missions.Dir(), j.Mission, "orchestrator.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			defer f.Close()
			w = f
		}
		return ctx.Orchestrate(ops.OrchestratorRun{Slug: j.Mission, Crew: j.ID, Trigger: j.Why, Auto: true, Since: j.Since,
			Said: func(line string) { fmt.Fprintln(w, line) }})
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
	case job.Verify:
		if said := ctx.VerifyAgain(j.Mission, j.ID); said != "" {
			execlog.Note(root, "verify "+j.Mission, "%s", said)
		}
		return nil
	case job.PR:
		if said := ctx.OpenWrittenPR(j.Mission, j.ID); said != "" {
			execlog.Note(root, "pr-writer "+j.Mission, "%s", said)
		}
		return nil
	}
	return fmt.Errorf("no such job as %q", j.Kind)
}

// reconcileJobs is what's left of jobs a server before this one was
// running when it went: an orchestrator's run still marked underway is
// taken again, as its event wasn't; a conversation's answer is taken up
// again the next time it's said to (an answering mark whose server's gone
// counts for nothing). What that server had marked underway on a mission
// and didn't live to clear is cleared: the planner's turn, whose run went
// with it, and crew it was starting and hadn't recorded.
func reconcileJobs(root string, q *job.Queue, logf func(string, ...any)) {
	// A server restarted in place has the pid of the one before it, so
	// that one's answering marks would look like this one's, live.
	if n, err := conversation.EndAnsweringBy(root, os.Getpid()); err == nil && n > 0 {
		logf("%d conversations were being answered when the server before this one went: they're answered the next time something's said", n)
	}
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
		for _, p := range m.WritingPRs {
			logf("the PR writer was writing %s's pull request on %s when the server before this one went: writing it again", p.Capsule, m.Slug)
			q.Start(job.Job{Kind: job.PR, Mission: m.Slug, ID: p.Capsule})
		}
		if m.Planning != nil {
			logf("the planner's turn in %s of %s was underway when the server before this one went: it's over", m.Planning.In, m.Slug)
			if err := (mission.Store{Root: root}).EndPlanning(m.Slug, m.Planning.In); err != nil {
				logf("%s: couldn't clear it: %v", m.Slug, err)
			}
		}
		if m.Moving != nil {
			logf("ws was moving %s on from step %d when the server before this one went: it's where it got to", m.Slug, m.Moving.Step)
			if err := (mission.Store{Root: root}).EndMoving(m.Slug, *m.Moving); err != nil {
				logf("%s: couldn't clear it: %v", m.Slug, err)
			}
		}
		if m.Launching != nil {
			logf("crew were being started on %s when the server before this one went: they weren't", m.Slug)
			if err := (mission.Store{Root: root}).EndLaunching(m.Slug, *m.Launching); err != nil {
				logf("%s: couldn't clear it: %v", m.Slug, err)
			}
		}
		if m.Orchestrating == nil {
			continue
		}
		if err := (mission.Store{Root: root}).EndOrchestrating(m.Slug, *m.Orchestrating); err != nil {
			logf("%s: couldn't clear it: %v", m.Slug, err)
		}
		logf("the orchestrator was running on %s when the server before this one went: running it again", m.Slug)
		q.Start(job.Job{Kind: job.Orchestrate, Mission: m.Slug, Why: "restarted"})
	}
}
