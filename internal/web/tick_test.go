package web

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

// The change check every open page leans on: every two seconds the daemon
// takes the stores' stamp, and when it's moved builds what GET
// /api/workspace serves and hashes it. These are what each costs on a
// workspace of thirty missions, each with a capsule of its own (git's
// processes, and the time). Run them with go test ./internal/web -bench Tick -run '^$'.
func BenchmarkTick(b *testing.B) {
	s := tickServer(b)
	fingerprint := func() {
		v, err := s.workspaceView()
		if err != nil {
			b.Fatal(err)
		}
		json.Marshal(v)
	}
	fingerprint()
	b.ResetTimer()
	before := git.Started()
	for range b.N {
		fingerprint()
	}
	b.ReportMetric(float64(git.Started()-before)/float64(b.N), "git/op")
}

// BenchmarkTickUnchanged is a tick when nothing's moved: the stamp alone.
func BenchmarkTickUnchanged(b *testing.B) {
	s := tickServer(b)
	s.App.Stamp()
	b.ResetTimer()
	before := git.Started()
	for range b.N {
		s.App.Stamp()
	}
	b.ReportMetric(float64(git.Started()-before)/float64(b.N), "git/op")
}

// BenchmarkTickPast and BenchmarkTickPastUnchanged are the same, with
// 5,000 archived missions besides: a year of a busy team's. What's past
// should cost next to nothing.
func BenchmarkTickPast(b *testing.B) {
	s := tickServer(b)
	pastMissions(b, s, 5000)
	fingerprint := func() {
		v, err := s.workspaceView()
		if err != nil {
			b.Fatal(err)
		}
		json.Marshal(v)
	}
	fingerprint()
	b.ResetTimer()
	before := git.Started()
	for range b.N {
		fingerprint()
	}
	b.ReportMetric(float64(git.Started()-before)/float64(b.N), "git/op")
}

func BenchmarkTickPastUnchanged(b *testing.B) {
	s := tickServer(b)
	pastMissions(b, s, 5000)
	s.App.Stamp()
	b.ResetTimer()
	for range b.N {
		s.App.Stamp()
	}
}

// pastMissions adds n archived missions, each with a capsule that's been
// burned, a decision and a few events.
func pastMissions(b *testing.B, s *Server, n int) {
	w := s.App.(*ops.Workspace)
	for i := range n {
		m, err := w.Missions.Create(fmt.Sprintf("Past %d", i))
		if err != nil {
			b.Fatal(err)
		}
		w.Missions.Update(m.Slug, func(m *mission.Mission) error {
			m.AddCapsuleBranch("api", fmt.Sprintf("past-%d", i), fmt.Sprintf("past-%d", i))
			end := m.Created.Add(time.Hour)
			m.Crew = append(m.Crew, mission.Crew{ID: "c1", Harness: "claude", Started: m.Created, Ended: &end, Exit: "done",
				LastReply: strings.Repeat("What it said at the end of its turn. ", 20)})
			m.State = mission.Archived
			return nil
		})
		w.Missions.Decide(m.Slug, mission.Decision{What: "Ship it", By: "you"})
		w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventState, Text: "archived"})
	}
}

// tickServer is a web UI over thirty missions, each with a capsule that's
// a repository of its own.
func tickServer(b testing.TB) *Server {
	root := b.TempDir()
	storetest.Own(b, root)
	ws := &workspace.Workspace{Root: root, RepoNames: []string{"api"}, DefaultBranch: "main"}
	for _, d := range []string{"repos/api/.ground", "repos/api/.bare"} {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	w := ops.New(&config.Config{}, ws, nil, ops.Options{})
	for i := range 30 {
		m, err := w.Missions.Create(fmt.Sprintf("Mission %d", i))
		if err != nil {
			b.Fatal(err)
		}
		name := fmt.Sprintf("work-%d", i)
		dir := filepath.Join(ws.RepoDir("api"), name)
		os.MkdirAll(dir, 0o755)
		for _, args := range [][]string{{"init", "-q", "--initial-branch=" + name}, {"-c", "user.email=a@b", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "start"}} {
			if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
				b.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		if _, err := w.Attach(m.Slug, "api", name, false, ops.AsYou); err != nil {
			b.Fatal(err)
		}
	}
	return &Server{Root: root, App: w}
}

// BenchmarkMissionRead is what a mission's page costs each time it reads
// its mission (GET /api/missions/{slug}), with nothing changed since the
// last read: every button on the page reads it again, and so does every
// event.
func BenchmarkMissionRead(b *testing.B) {
	s := tickServer(b)
	read := func() {
		d, err := s.missionDetail("mission-7", false)
		if err != nil {
			b.Fatal(err)
		}
		json.Marshal(d)
	}
	read()
	b.ResetTimer()
	before := git.Started()
	for range b.N {
		read()
	}
	b.ReportMetric(float64(git.Started()-before)/float64(b.N), "git/op")
}

// What a page reads after every button and every change costs no git once
// it's been read, until a capsule moves: then it's what the capsule now is.
func TestReads_StartNoGitUntilACapsuleMoves(t *testing.T) {
	s := tickServer(t)
	read := func() *MissionDetail {
		t.Helper()
		d, err := s.missionDetail("mission-7", false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.workspaceView(); err != nil {
			t.Fatal(err)
		}
		return d
	}
	first := read()
	if len(first.Capsules) != 1 || first.Capsules[0].Head == "" {
		t.Fatalf("mission-7's capsules = %+v", first.Capsules)
	}
	before := git.Started()
	for range 3 {
		read()
	}
	if n := git.Started() - before; n != 0 {
		t.Errorf("3 reads of a mission and the workspace, nothing moved, started git %d times: want none", n)
	}

	dir := filepath.Join(s.Root, "repos", "api", "work-7")
	if out, err := exec.Command("git", "-C", dir, "-c", "user.email=a@b", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "more").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	head, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if got := read().Capsules[0].Head; got != strings.TrimSpace(string(head)) || got == first.Capsules[0].Head {
		t.Errorf("after a commit the capsule's head is %q, want %q", got, strings.TrimSpace(string(head)))
	}
}
