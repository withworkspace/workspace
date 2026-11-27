package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
)

// A PR's change is logged once, whoever sees it; a change someone must act
// on goes to the orchestrator unless crew are at work in its capsule.
func TestWatchPRs(t *testing.T) {
	ws := serverWorkspace(t, map[string][]string{"api": {"fix"}})
	store := mission.Store{Root: ws.Root}
	slug := seedMission(t, ws, "Cache tokens", "", [2]string{"api", "fix"})
	var spawned []string
	off := false
	watch := func(pr github.PR, prs mission.PRState, orchestratorOn bool) {
		t.Helper()
		m, err := store.Load(slug)
		if err != nil {
			t.Fatal(err)
		}
		v := ops.MissionView{M: m, State: mission.Review, Capsules: []ops.CapsuleView{
			{Capsule: m.Capsules[0], Exists: true, PR: &pr, PRS: prs},
		}}
		cfg := &config.Config{}
		if !orchestratorOn {
			cfg.Orchestrator.Enabled = &off
		}
		ops.New(cfg, ws, nil, ops.Options{Jobs: ops.JobsOf(func(j job.Job) { spawned = append(spawned, j.Mission+"/"+j.Why) }, job.Orchestrate)}).WatchPRs([]ops.MissionView{v})
	}
	prEvents := func() []string {
		events, _ := store.ReadLog(slug, 0)
		var out []string
		for _, e := range events {
			if e.Kind == mission.EventPR {
				out = append(out, e.Text)
			}
		}
		return out
	}

	watch(github.PR{Number: 88, ReviewDecision: "REVIEW_REQUIRED", StatusRollup: "pending"}, mission.PROpen, true)
	if len(prEvents()) != 0 || len(spawned) != 0 {
		t.Fatalf("a first sighting is news: %v, %v", prEvents(), spawned)
	}

	red := github.PR{Number: 88, ReviewDecision: "REVIEW_REQUIRED", StatusRollup: "failure"}
	watch(red, mission.PROpen, true)
	watch(red, mission.PROpen, true) // the deck and the daemon both look
	if got := prEvents(); strings.Join(got, "|") != "api#88: checks failing" || strings.Join(spawned, " ") != slug+"/pr" {
		t.Errorf("red checks: events %v, spawned %v", got, spawned)
	}

	// Crew at work in the capsule have it in hand.
	store.StartCrew(slug, "claude", "conv")
	store.SetCrewProcess(slug, "c1", 0, "api/fix")
	watch(github.PR{Number: 88, ReviewDecision: "CHANGES_REQUESTED", StatusRollup: "failure"}, mission.PROpen, true)
	if len(spawned) != 1 || len(prEvents()) != 2 {
		t.Errorf("crew at work: events %v, spawned %v", prEvents(), spawned)
	}

	// A conflict is news too.
	store.EndCrewStopped(slug, "c1", "hangup", true)
	watch(github.PR{Number: 88, ReviewDecision: "CHANGES_REQUESTED", StatusRollup: "failure", Mergeable: "CONFLICTING"}, mission.PROpen, true)
	if got := prEvents(); got[len(got)-1] != "api#88 has merge conflicts" || len(spawned) != 2 {
		t.Errorf("a conflict: events %v, spawned %v", got, spawned)
	}

	// Merged, with the orchestrator off: logged, nobody spawned.
	watch(github.PR{Number: 88, MergedAt: time.Now().Format(time.RFC3339)}, mission.PRMerged, false)
	if got := prEvents(); got[len(got)-1] != "api#88 merged" || len(spawned) != 2 {
		t.Errorf("merged: events %v, spawned %v", got, spawned)
	}
}

// A merge that passes the step the mission's at (an Open Pull Request,
// ws's own) is ws's to handle: the orchestrator isn't woken for it. One
// that doesn't, is.
func TestWatchPRs_MergePassingAStep(t *testing.T) {
	ws := serverWorkspace(t, map[string][]string{"api": {"fix", "other"}})
	store := mission.Store{Root: ws.Root}
	slug := seedMission(t, ws, "Cache tokens", "", [2]string{"api", "fix"})
	store.WritePlan(slug, "## Z\n- Merged.\n## Stops early\n- None.\n## Steps\n1. Open Pull Request\n   - in: `api/fix`\n2. Review: what merged.\n")
	spawned := 0
	watch := func(slug string, pr github.PR, prs mission.PRState) {
		t.Helper()
		m, err := store.Load(slug)
		if err != nil {
			t.Fatal(err)
		}
		v := ops.MissionView{M: m, State: mission.Review, Capsules: []ops.CapsuleView{{Capsule: m.Capsules[0], Exists: true, PR: &pr, PRS: prs}}}
		ops.New(&config.Config{}, ws, nil, ops.Options{Jobs: ops.JobsOf(func(job.Job) { spawned++ }, job.Orchestrate)}).WatchPRs([]ops.MissionView{v})
	}
	watch(slug, github.PR{Number: 7, StatusRollup: "success"}, mission.PROpen) // first sighting
	watch(slug, github.PR{Number: 7, MergedAt: time.Now().Format(time.RFC3339)}, mission.PRMerged)
	m, _ := store.Load(slug)
	if states, _ := m.StageProgress(""); spawned != 0 || !states[0].Accepted() || states[0].Record.AcceptedBy != "ws" {
		t.Errorf("a merge that passes the step: spawned %d, step 1 %+v", spawned, states[0].Record)
	}

	other := seedMission(t, ws, "Other", "", [2]string{"api", "other"})
	watch(other, github.PR{Number: 8, StatusRollup: "success"}, mission.PROpen)
	if watch(other, github.PR{Number: 8, MergedAt: time.Now().Format(time.RFC3339)}, mission.PRMerged); spawned != 1 {
		t.Errorf("a merge that passes nothing: spawned %d", spawned)
	}
}
