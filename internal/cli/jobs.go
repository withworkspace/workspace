package cli

import (
	"fmt"

	"github.com/withworkspace/workspace/internal/daemon"
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

// startJob has the workspace's daemon run a job.
func startJob(root string, j job.Job) error {
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

// spawnPRWriter has the server write and open the pull request the PR
// writer's marked writing for a mission's capsule. Where it can't be
// asked, the mark comes off: nothing would ever open it.
func spawnPRWriter(root, slug, capsule string) {
	if err := startJob(root, job.Job{Kind: job.PR, Mission: slug, ID: capsule}); err != nil {
		execlog.Note(root, "pr-writer "+slug, "couldn't start for %s: %v", capsule, err)
		store := mission.Store{Root: root}
		store.EndWritingPR(slug, capsule)
		store.AppendLogAfter(slug, mission.Event{Kind: mission.EventNote, Crew: "ws", Text: fmt.Sprintf("couldn't have %s's pull request written (%v): open it again", capsule, err)})
	}
}

// prWriterSpawner starts the PR writer; tests replace it.
var prWriterSpawner = spawnPRWriter

// spawnAnswer has the server answer a conversation (kind job.Answer) or a
// thread (job.Thread), or draft what a resolved thread decided (job.Draft).
func spawnAnswer(kind string) func(root, slug, id string) {
	return func(root, slug, id string) {
		if err := startJob(root, job.Job{Kind: kind, Mission: slug, ID: id}); err != nil {
			execlog.Note(root, kind+" "+firstNonEmpty(slug, "workspace")+"/"+id, "couldn't start: %v", err)
			endPlanning(root, job.Job{Kind: kind, Mission: slug, ID: id})
		}
	}
}

// endPlanning ends the planner's turn a job was to have, when the job
// won't run: the mark was set as the turn was asked for (ops), and the run
// that would have ended it never began. It's nothing for a job that isn't
// an answer in a mission's planner's conversation.
func endPlanning(root string, j job.Job) {
	if j.Kind == job.Answer && j.Mission != "" {
		mission.Store{Root: root}.EndPlanning(j.Mission, j.ID) //nolint:errcheck // a mark
	}
}
