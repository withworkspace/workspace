package cli

import (
	"testing"

	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
)

// A turn asked for that no job will have (the daemon couldn't be asked)
// is ended: only an answer's, in that conversation.
func TestJobs_ATurnNoJobWillHaveIsEnded(t *testing.T) {
	ws := serverWorkspace(t, map[string][]string{"frontend": {"work"}})
	slug := seedMission(t, ws, "Served", "")
	store := mission.Store{Root: ws.Root}
	store.StartPlanning(slug, "t1")
	endPlanning(ws.Root, job.Job{Kind: job.Thread, Mission: slug, ID: "t1"})
	endPlanning(ws.Root, job.Job{Kind: job.Answer, Mission: slug, ID: "t2"})
	if m, _ := store.Load(slug); m.Planning == nil {
		t.Fatal("ended by a job that wasn't its turn")
	}
	endPlanning(ws.Root, job.Job{Kind: job.Answer, Mission: slug, ID: "t1"})
	if m, _ := store.Load(slug); m.Planning != nil {
		t.Errorf("still marked after its job couldn't run: %+v", m.Planning)
	}
}
