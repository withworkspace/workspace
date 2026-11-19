package review_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/testutil"
)

// A review submitted in the web UI is crew's to read with ws review show
// (from their capsule, the latest of it), listed with ws review list, and
// the orchestrator gets it whole when run for it. Drafts stay the user's.
func TestReview_ShowListAndOrchestrate(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	os.WriteFile(filepath.Join(capsule, "cache.go"), []byte("package cache\n\nvar ttl = 0\n"), 0o644)
	testutil.GitCmd(t, capsule, "add", "-A")
	testutil.GitCmd(t, capsule, "commit", "-q", "-m", "cache")
	d, err := review.Compare(capsule, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	store := review.For(w.Root, "cache")
	diffed := review.Diffed{Dir: capsule, BaseRef: "origin/main", Base: d.Base, Head: d.Head}
	at := time.Now()
	store.AddNote("backend/cache-lookup", diffed, review.NewNote{Path: "cache.go", Side: "new", Line: 3, Kind: "blocking", Body: "A zero TTL caches nothing."}, at)
	if r := testutil.RunCommand(t, w.Root, nil, "review", "show", "-m", "cache"); r.Err == nil || !strings.Contains(r.Err.Error(), "no reviews") {
		t.Errorf("a draft showed: %v %s", r.Err, r.Stdout)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "review", "show", "-m", "cache", "r1"); r.Err == nil || !strings.Contains(r.Err.Error(), "draft") {
		t.Errorf("draft by id: %v", r.Err)
	}
	if _, err := store.Submit("backend/cache-lookup", diffed, review.Changes, "One thing.", at); err != nil {
		t.Fatal(err)
	}

	r := testutil.Run(t, w, nil, "review", "show", "-m", "cache", "-f", "llm")
	for _, want := range []string{"# Review r1 of backend/cache-lookup: changes requested", "> One thing.", "### r1.c1 · blocking · cache.go:3 at", "3  var ttl = 0", "A zero TTL caches nothing.", "## What to do"} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("show lacks %q:\n%s", want, r.Stdout)
		}
	}
	// From inside the capsule, no mission or id needed.
	t.Chdir(capsule)
	if r := testutil.RunCommand(t, capsule, nil, "review", "show"); r.Err != nil || !strings.Contains(r.Stdout, "Review r1 of backend/cache-lookup") {
		t.Errorf("from the capsule: %v %s", r.Err, r.Stdout)
	}
	if r := testutil.Run(t, w, nil, "review", "list", "-m", "cache"); !strings.Contains(r.Stdout, "r1   backend/cache-lookup") || !strings.Contains(r.Stdout, "changes requested") {
		t.Errorf("list:\n%s", r.Stdout)
	}

	p := testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--trigger", "review", "--print-prompt")
	for _, want := range []string{"submitted the review below (mission:reviews/r1.md)", "latest last): c1.", "A zero TTL caches nothing."} {
		if !strings.Contains(p.Stdout, want) {
			t.Errorf("orchestrator prompt lacks %q", want)
		}
	}
	if show := testutil.Run(t, w, nil, "mission", "show", "cache", "-f", "llm"); !strings.Contains(show.Stdout, "mission:reviews/r1.md") {
		t.Errorf("mission show doesn't list the review:\n%s", show.Stdout)
	}
}

// The loop after a review: a stage whose review asked for changes isn't
// accepted; crew answer each comment; once they've committed and stopped,
// it's back with the user (the orchestrator is told, and the web UI shows
// it); the user's approval lets the stage be accepted.
func TestReview_TheLoop(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	os.WriteFile(filepath.Join(capsule, "cache.go"), []byte("package cache\n\nvar ttl = 0\n"), 0o644)
	testutil.GitCmd(t, capsule, "add", "-A")
	testutil.GitCmd(t, capsule, "commit", "-q", "-m", "cache")
	d, _ := review.Compare(capsule, "origin/main")
	reviews := review.For(w.Root, "cache")
	diffed := review.Diffed{Dir: capsule, BaseRef: "origin/main", Base: d.Base, Head: d.Head}
	at := time.Now()
	reviews.AddNote("backend/cache-lookup", diffed, review.NewNote{Path: "cache.go", Side: "new", Line: 3, Kind: "blocking", Body: "A zero TTL caches nothing."}, at)
	reviews.AddNote("backend/cache-lookup", diffed, review.NewNote{Path: "cache.go", Side: "new", Line: 1, Kind: "nit", Body: "Doc comment?"}, at)
	reviews.Submit("backend/cache-lookup", diffed, review.Changes, "", at)

	t.Setenv("WS_ROLE", "orchestrator")
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "accept", "-m", "cache", "looks done"); r.Err == nil || !strings.Contains(r.Err.Error(), "asked for changes in review r1 of backend/cache-lookup (0 of 2 comments answered)") {
		t.Fatalf("accepted over a review asking for changes: %v", r.Err)
	}
	t.Setenv("WS_ROLE", "")

	// Crew fix it, commit, and answer each comment.
	os.WriteFile(filepath.Join(capsule, "cache.go"), []byte("package cache\n\nvar ttl = 60\n"), 0o644)
	testutil.GitCmd(t, capsule, "commit", "-q", "-am", "ttl 60")
	fixed := strings.TrimSpace(testutil.GitOut(t, capsule, "rev-parse", "HEAD"))
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c1")
	testutil.Run(t, w, nil, "review", "addressed", "r1.c1", "TTL is 60s now.")
	testutil.Run(t, w, nil, "review", "wontfix", "r1.c2", "The package doc covers it.")
	if r := testutil.RunCommand(t, w.Root, nil, "review", "reply", "r1.c1"); r.Err == nil {
		t.Error("a reply needs words")
	}
	r1, _ := reviews.Load("r1")
	if c := r1.Comments[0]; c.Status != review.Addressed || c.Thread[0].By != "c1" || c.Thread[0].Commit != fixed {
		t.Errorf("addressed = %+v", c)
	}
	if !strings.Contains(testutil.Run(t, w, nil, "review", "show").Stdout, "### r1.c2 · nit · cache.go:1 at") {
		t.Error("show lacks the answered comment")
	}

	// While c1 is still at it, nothing's back with the user; once it's done,
	// the capsule is.
	ws, _ := testutil.WebWorkspace(w.Root)()
	if len(ws.Missions[0].ReReview) != 0 {
		t.Errorf("re-review with crew still working: %v", ws.Missions[0].ReReview)
	}
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(mission.EnvMission, "")
	store.SetCrewStatus("cache", "c1", mission.CrewDone, "fixed the ttl")
	ws, _ = testutil.WebWorkspace(w.Root)()
	if got := ws.Missions[0].ReReview; len(got) != 1 || got[0] != "backend/cache-lookup" {
		t.Errorf("re-review = %v", got)
	}
	p := testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--crew", "c1", "--trigger", "done", "--print-prompt").Stdout
	for _, want := range []string{"## Reviews", "r1 of backend/cache-lookup: changes requested", "1 addressed, 1 won't fix", "waiting for the user's re-review", "don't try to accept"} {
		if !strings.Contains(p, want) {
			t.Errorf("orchestrator prompt lacks %q", want)
		}
	}

	// The user approves the re-review: the stage can be accepted.
	reviews.Submit("backend/cache-lookup", review.Diffed{Dir: capsule, BaseRef: "origin/main", Base: d.Base, Head: fixed}, review.Approve, "", at.Add(time.Minute))
	t.Setenv("WS_ROLE", "orchestrator")
	testutil.Run(t, w, nil, "mission", "accept", "-m", "cache", "approved")
}
