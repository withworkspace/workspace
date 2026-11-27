package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/mission"
)

// sayingHost is a host whose crew are running: what's said to them is
// kept.
type sayingHost struct{ said []string }

func (h *sayingHost) Launch(crew.Spec) (int, error) { return deadPID, nil }
func (h *sayingHost) Stop(string) error             { return nil }
func (h *sayingHost) Say(key, text string) (string, error) {
	h.said = append(h.said, key+": "+text)
	return "typed in", nil
}

// inTurnSinceDone puts c1 in a turn begun a minute after it reported
// done, as a message sent to it since does.
func inTurnSinceDone(t *testing.T, w *Workspace) {
	t.Helper()
	if _, err := w.Missions.Update("cache", func(m *mission.Mission) error {
		c := &m.Crew[0]
		at := c.StatusAt.Add(time.Minute)
		c.Activity, c.ActivityAt = mission.ActivityWorking, &at
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// What you say to crew who've reported done doesn't hold their step: its
// checks run on their done, and met, ws passes it.
func TestChecks_YourMessageDoesntHoldTheStep(t *testing.T) {
	w, runs, dir := workFixture(t, "1. met: cache.go keeps each lookup")
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	w.Missions.TalkedTo("cache", "c1", "Why a map?")
	w.Missions.CrewHook("cache", "c1", mission.Hook{Prompted: true})
	w.Missions.CrewHook("cache", "c1", mission.Hook{Activity: mission.ActivityWaiting, Reply: "It's keyed by org."})

	if said := w.CrewDone("cache", "c1"); !strings.Contains(said, "passed step 1") {
		t.Fatalf("CrewDone said %q", said)
	}
	if st := stepRecord(t, w, 1); !st.Accepted() || st.Record.AcceptedBy != "ws" {
		t.Errorf("step 1 = %+v", st.Record)
	}
	if len(*runs) != 0 {
		t.Errorf("the orchestrator was woken: %v", *runs)
	}
}

// Checks that come round met while their crew are in a turn begun since
// their done don't pass the step under them. The mission says so, and
// it's passed when the turn ends, with nobody asked.
func TestChecks_MetMidTurnPassesWhenTheTurnEnds(t *testing.T) {
	w, runs, dir := workFixture(t, "1. met: cache.go keeps each lookup")
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	inTurnSinceDone(t, w)

	said := w.CrewDone("cache", "c1")
	if !strings.Contains(said, "passes when c1's current turn ends") || stepRecord(t, w, 1).Accepted() {
		t.Fatalf("CrewDone said %q, step 1 = %+v", said, stepRecord(t, w, 1).Record)
	}
	if !logged(t, w, "step 1's checks met (round 1): passes when c1's current turn ends") {
		t.Error("the mission doesn't say why it isn't passed")
	}

	w.Missions.CrewHook("cache", "c1", mission.Hook{Activity: mission.ActivityWaiting, Reply: "It's keyed by org."})
	w.TurnEnded("cache", "c1")
	if st := stepRecord(t, w, 1); !st.Accepted() || st.Record.AcceptedBy != "ws" {
		t.Errorf("after the turn: step 1 = %+v", st.Record)
	}
	m, _ := w.Missions.Load("cache")
	if _, open := m.StepTask(1); open || len(*runs) != 0 {
		t.Errorf("someone was asked: tasks %+v, runs %v", m.Tasks, *runs)
	}

	// A turn's end with nothing held is the orchestrator's to look at, as
	// before.
	w2, runs2, _ := workFixture(t, "")
	w2.Missions.CrewHook("cache", "c1", mission.Hook{Activity: mission.ActivityWaiting})
	w2.TurnEnded("cache", "c1")
	if strings.Join(*runs2, " ") != "cache/quiet" {
		t.Errorf("a quiet turn: runs %v", *runs2)
	}
}

// commitIn commits everything in dir, a capsule that's a repo (made one
// if it isn't yet), and gives the commit.
func commitIn(t *testing.T, dir, message string) string {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		gitIn(t, dir, "init", "-q")
	}
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", message)
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// Crew who commit more on a message sent after their done have changed
// what was checked: ws asks them to report done again rather than pass
// the step on the round that ran, and that takes their done back until
// they do.
func TestChecks_WorkChangedSinceIsCheckedAgain(t *testing.T) {
	w, _, dir := workFixture(t, "1. met: cache.go keeps each lookup")
	host := &sayingHost{}
	w.opts.Host = host
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	checked := commitIn(t, dir, "feat: cache the lookup")
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	inTurnSinceDone(t, w)
	if said := w.CrewDone("cache", "c1"); !strings.Contains(said, "passes when c1's current turn ends") {
		t.Fatalf("CrewDone said %q", said)
	}

	// The message was more to do, and they did it.
	os.WriteFile(filepath.Join(dir, "evict.go"), []byte("package cache\n"), 0o644)
	commitIn(t, dir, "feat: evict what's stale")
	w.Missions.CrewHook("cache", "c1", mission.Hook{Activity: mission.ActivityWaiting, Reply: "Evicts now too."})
	w.TurnEnded("cache", "c1")

	if stepRecord(t, w, 1).Accepted() {
		t.Fatal("passed on a round that ran before the work changed")
	}
	if len(host.said) != 1 || !strings.Contains(host.said[0], "report done again") || !strings.Contains(host.said[0], checked[:9]) {
		t.Fatalf("c1 was told %q", host.said)
	}
	m, _ := w.Missions.Load("cache")
	if c := m.Crew[0]; c.Status != "" {
		t.Errorf("their done still stands: %+v", c)
	}
	if !logged(t, w, "the work changed since") {
		t.Error("the mission doesn't say the work changed")
	}

	// Reporting again, it's checked as it is now, and passes.
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached, and evicts")
	if said := w.CrewDone("cache", "c1"); !strings.Contains(said, "passed step 1") {
		t.Errorf("the second done: %q", said)
	}
	if r, _ := stepRecord(t, w, 1).Record.LatestChecks(); r.N != 2 || r.Commit == checked {
		t.Errorf("round = %+v", r)
	}
}

// With its crew gone, work that's changed since the checks ran is yours
// to say: there's nobody to ask to report again.
func TestChecks_WorkChangedWithCrewGoneIsYours(t *testing.T) {
	w, _, dir := workFixture(t, "1. met: cache.go keeps each lookup")
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	commitIn(t, dir, "feat: cache the lookup")
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	inTurnSinceDone(t, w)
	w.CrewDone("cache", "c1")
	os.WriteFile(filepath.Join(dir, "evict.go"), []byte("package cache\n"), 0o644)
	commitIn(t, dir, "feat: evict what's stale")
	w.Missions.EndCrew("cache", "c1", "exit 0")

	said, ok := w.settleChecked("cache", "c1")
	m, _ := w.Missions.Load("cache")
	task, open := m.StepTask(1)
	if !ok || !open || task.Verb != mission.TaskChecks || !strings.Contains(task.Text, "The work changed since checks ran") {
		t.Fatalf("settling said %q (%v), task %+v", said, ok, task)
	}
	if a, err := w.AnswerTask("cache", task.N, AsYou, mission.ChecksPass); err != nil || a.Next == "" {
		t.Fatalf("passing it: %+v, %v", a, err)
	}
	if st := stepRecord(t, w, 1); !st.Accepted() || st.Record.AcceptedBy != "you" {
		t.Errorf("step 1 = %+v", st.Record)
	}
}

// "Pass it" that can't pass the step leaves its question open, with why:
// answered and closed, nothing on the mission would say it's still yours.
func TestAnswer_PassItThatCantPassStaysOpen(t *testing.T) {
	w, _, _ := unmetRound(t)
	if _, err := w.Flag("cache", "Check 2 can't be met as it's written. Pass it?", AsOrchestrator); err != nil {
		t.Fatal(err)
	}
	m, _ := w.Missions.Load("cache")
	task, _ := m.StepTask(1)
	// Sent back since, and at work on it.
	w.Missions.Nudged("cache", "c1", "put check 2 right", "orchestrator")
	w.Missions.CrewHook("cache", "c1", mission.Hook{Prompted: true})

	if _, err := w.AnswerTask("cache", task.N, AsYou, mission.ChecksPass); err == nil || !strings.Contains(err.Error(), "c1 has not reported done") {
		t.Fatalf("passing it under working crew: %v", err)
	}
	m, _ = w.Missions.Load("cache")
	if _, open := m.StepTask(1); !open || stepRecord(t, w, 1).Accepted() {
		t.Fatalf("the question closed, or the step passed: tasks %+v", m.Tasks)
	}

	// They stop without reporting again: it's yours to pass now.
	w.Missions.CrewHook("cache", "c1", mission.Hook{Activity: mission.ActivityWaiting, Reply: "It was right already."})
	if _, err := w.AnswerTask("cache", task.N, AsYou, mission.ChecksPass); err != nil {
		t.Fatalf("passing it once they've stopped: %v", err)
	}
	m, _ = w.Missions.Load("cache")
	if st := stepRecord(t, w, 1); !st.Accepted() || st.Record.AcceptedBy != "you" || len(m.Tasks) != 0 {
		t.Errorf("step 1 = %+v, tasks %+v", st.Record, m.Tasks)
	}
}

// A command met on the capsule's files, all committed, isn't run again
// for a round on the same files: a done reported again with nothing
// changed, or with only its history rewritten. Files that have changed,
// or aren't all committed, have it run.
func TestChecks_ACommandMetOnTheSameFilesIsntRunAgain(t *testing.T) {
	w, _, dir := workFixture(t, "1. unmet: nothing keeps a lookup")
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	first := commitIn(t, dir, "feat: cache the lookup")
	done := func(note string) mission.CheckRound {
		t.Helper()
		w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, note)
		w.CrewDone("cache", "c1")
		r, _ := stepRecord(t, w, 1).Record.LatestChecks()
		return r
	}
	ran := func() int {
		t.Helper()
		events, _ := w.Missions.ReadLog("cache", 0)
		n := 0
		for _, e := range events {
			if e.Kind == mission.EventChecks && strings.Contains(e.Text, "check 1 of 2 started") {
				n++
			}
		}
		return n
	}

	r1 := done("cached")
	if r1.Tree == "" || r1.Results[0].Result != mission.CheckMet || ran() != 1 {
		t.Fatalf("round 1 = %+v, ran %d", r1, ran())
	}

	// Reported again, nothing changed: the judge looks again, the command
	// doesn't run.
	r2 := done("cached, as I said")
	if r2.N != 2 || ran() != 1 || r2.Results[0].Result != mission.CheckMet || r2.Results[0].Run != r1.Results[0].Run ||
		!strings.Contains(r2.Results[0].Evidence, "met in round 1 at "+first[:9]) || !strings.Contains(r2.Results[0].Evidence, "not run again") {
		t.Fatalf("round 2 = %+v, ran %d", r2.Results[0], ran())
	}
	if !logged(t, w, "was met in round 1 at "+first[:9]+", and the files are the same now: not run again") {
		t.Error("the log doesn't say it wasn't run")
	}

	// Its history rewritten, its files the same.
	gitIn(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--amend", "-m", "feat: keep each lookup")
	if r3 := done("reworded"); r3.Commit == first || ran() != 1 {
		t.Errorf("round 3 = %+v, ran %d", r3.Results[0], ran())
	}

	// A file changed and not committed: it's run.
	os.WriteFile(filepath.Join(dir, "evict.go"), []byte("package cache\n"), 0o644)
	if r4 := done("evicts"); r4.Tree != "" || ran() != 2 {
		t.Errorf("round 4 = %+v, ran %d", r4, ran())
	}
	// Committed, the files are others than any it was met on: run.
	commitIn(t, dir, "feat: evict what's stale")
	if r5 := done("evicts, committed"); r5.Tree == "" || r5.Tree == r1.Tree || ran() != 3 {
		t.Errorf("round 5 = %+v, ran %d", r5, ran())
	}
}
