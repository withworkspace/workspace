package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
)

// In the daemon, a pull request you wrote is opened behind the answer: the
// answer says it's opening, the mission shows it, and the daemon's job
// pushes and opens it as you wrote it. What's at hand is still refused at
// once.
func TestOpenPRBehind(t *testing.T) {
	w := liftWorld(t)
	var created []github.NewPR
	w.opts.GitHub = &github.Stub{CreatePRFn: func(_, _ string, n github.NewPR) (*github.PR, error) {
		created = append(created, n)
		return &github.PR{Number: 7, URL: "https://example.com/7", HeadRefName: n.Head}, nil
	}}
	var jobs []string
	w.opts.Jobs = JobsOf(func(j job.Job) { jobs = append(jobs, j.Kind+" "+j.Mission+"/"+j.ID) }, job.PR)
	m, err := w.Missions.Create("Cache")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Lift("api", "cache-work", "", CapsuleOptions{Mission: m.Slug, By: AsYou}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(w.WS().RepoDir("api"), "cache-work")

	_, err = w.OpenPR(m.Slug, "api/cache-work", NewPullRequest{Title: "Cache the lookup", Body: "Why."}, AsYou)
	refusedWith(t, err, "nothing")

	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package main\n"), 0o644)
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "cache"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	out, err := w.OpenPR(m.Slug, "api/cache-work", NewPullRequest{Title: "Cache the lookup", Body: "Why."}, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Opening || out.PR != nil || out.String() != "Opening api/cache-work's pull request." {
		t.Errorf("answered: %+v (%s)", out, out)
	}
	if len(created) != 0 {
		t.Errorf("opened before the answer: %+v", created)
	}
	if !slices.Equal(jobs, []string{"pr cache/api/cache-work"}) {
		t.Errorf("jobs: %v", jobs)
	}
	m, _ = w.Missions.Load(m.Slug)
	if hs := mission.Resolve(mission.Snapshot{M: m, State: mission.Active}); !slices.ContainsFunc(hs, func(h mission.Headline) bool {
		return h.Kind == mission.KindWritingPR && h.Text == "Opening api/cache-work's pull request" && h.By == "ws"
	}) {
		t.Errorf("headlines: %+v", hs)
	}
	_, err = w.OpenPR(m.Slug, "api/cache-work", NewPullRequest{Title: "Again"}, AsYou)
	refusedWith(t, err, "pr-writing")

	// The daemon's job.
	if said := w.OpenWrittenPR(m.Slug, "api/cache-work"); !strings.HasPrefix(said, "Opened api#7") {
		t.Errorf("the job said %q", said)
	}
	if len(created) != 1 || created[0].Title != "Cache the lookup" || created[0].Body != "Why." || created[0].Head != "cache-work" {
		t.Errorf("opened: %+v", created)
	}
	if m, _ = w.Missions.Load(m.Slug); len(m.WritingPRs) != 0 {
		t.Errorf("still marked: %+v", m.WritingPRs)
	}
}
