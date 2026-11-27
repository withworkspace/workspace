package ops

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/mission"
)

// slowCheck prints two lines and waits for a file named go in its
// capsule: a check that's still running while the test looks.
const slowCheck = "printf 'building\\nok  internal/cli  4.2s\\n'; until [ -f go ]; do sleep 0.02; done"

// fastLines follows a check's output every few milliseconds, for a test.
func fastLines(t *testing.T) {
	t.Helper()
	was := checkLineEvery
	checkLineEvery = 5 * time.Millisecond
	t.Cleanup(func() { checkLineEvery = was })
}

// whileRunning waits until what ws is running on the mission is as want
// says, and returns it.
func whileRunning(t *testing.T, w *Workspace, want func(*mission.Checking) bool) mission.Checking {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if m, err := w.Missions.Load("cache"); err == nil && m.Checking != nil && want(m.Checking) {
			return *m.Checking
		}
		time.Sleep(5 * time.Millisecond)
	}
	m, _ := w.Missions.Load("cache")
	t.Fatalf("never running as wanted: %+v", m.Checking)
	return mission.Checking{}
}

// logTexts is what the mission's log says of checks.
func logTexts(t *testing.T, w *Workspace) []string {
	t.Helper()
	events, err := w.Missions.ReadLog("cache", 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range events {
		if e.Kind == mission.EventChecks {
			out = append(out, e.Text)
		}
	}
	return out
}

func hasPrefixIn(texts []string, prefix string) bool {
	return slices.ContainsFunc(texts, func(s string) bool { return strings.HasPrefix(s, prefix) })
}

// While a Work step's checks run, what's running is kept on the mission:
// the command, n of N, when it started and when it's stopped, the last line
// it's printed, and the judge's checks after it; then the judge's turn.
// Each one's start, end and result is logged, to the mission's log and
// exec.log; the round, once recorded, clears it, and keeps the command's
// output as it always did.
func TestChecks_WhatsRunningIsKeptAndLogged(t *testing.T) {
	fastLines(t)
	w, _, dir := workFixture(t, "1. met: cache.go keeps each lookup")
	w.Missions.WritePlan("cache", strings.Replace(workPlan, "`test -f cache.go` passes", "`"+slowCheck+"` passes", 1))
	var judging *mission.Checking
	fake.during = func() {
		m, _ := w.Missions.Load("cache")
		judging = m.Checking
	}
	t.Cleanup(func() { fake.during = nil })
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	said := make(chan string)
	go func() { said <- w.CrewDone("cache", "c1") }()

	c := whileRunning(t, w, func(c *mission.Checking) bool { return c.Line != "" })
	if c.Step != 1 || c.Crew != "c1" || c.N != 1 || c.Of != 2 || c.Command != slowCheck || c.Line != "ok  internal/cli  4.2s" ||
		!slices.Equal(c.Judged, []int{2}) || c.Timeout() != checkTimeout || c.Since.IsZero() {
		t.Errorf("while the command runs: %+v", c)
	}
	m, _ := w.Missions.Load("cache")
	heads := mission.Resolve(mission.Snapshot{M: m, State: mission.Active, OrchestratorOff: true})
	if !slices.ContainsFunc(heads, func(h mission.Headline) bool {
		return h.Kind == mission.KindChecking && strings.HasPrefix(h.Text, "Check 1 of 2 for step 1: `"+slowCheck+"`, ") &&
			strings.Contains(h.Text, "(limit 15m). Last line: “ok  internal/cli  4.2s”. The judge reads check 2 after")
	}) {
		t.Errorf("headlines: %+v", heads)
	}
	if !hasPrefixIn(logTexts(t, w), "step 1, check 1 of 2 started: `"+slowCheck+"`") {
		t.Errorf("its start isn't logged: %q", logTexts(t, w))
	}

	os.WriteFile(filepath.Join(dir, "go"), nil, 0o644)
	select {
	case s := <-said:
		if !strings.Contains(s, "step 1") {
			t.Errorf("CrewDone said %q", s)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the checks never finished")
	}
	if judging == nil || !judging.Judging() || judging.Judge != "the judge (fake-answer)" || !slices.Equal(judging.Judged, []int{2}) || judging.Timeout() != judgeTimeout {
		t.Errorf("while the judge reads: %+v", judging)
	}
	m, _ = w.Missions.Load("cache")
	if m.Checking != nil {
		t.Errorf("still running once the round's recorded: %+v", m.Checking)
	}
	r, _ := stepRecord(t, w, 1).Record.LatestChecks()
	if ev := r.Results[0].Evidence; !strings.HasPrefix(ev, "passed in ") || !strings.HasSuffix(ev, "s:\nbuilding\nok  internal/cli  4.2s") {
		t.Errorf("evidence = %q", ev)
	}

	texts := logTexts(t, w)
	for _, want := range []string{
		"step 1, check 1 of 2 started: `" + slowCheck + "`",
		"step 1, check 1 of 2: `" + slowCheck + "` passed in ",
		"step 1: the judge (fake-answer) started reading check 2",
		"step 1: the judge (fake-answer) finished in ",
		"step 1's checks, round 1",
	} {
		if !hasPrefixIn(texts, want) {
			t.Errorf("the log lacks %q: %q", want, texts)
		}
	}
	if !slices.ContainsFunc(texts, func(s string) bool { return strings.HasSuffix(s, ": met") && strings.Contains(s, "check 1 of 2: ") }) ||
		!slices.ContainsFunc(texts, func(s string) bool {
			return strings.Contains(s, "finished in") && strings.HasSuffix(s, ": check 2 met")
		}) {
		t.Errorf("results aren't logged: %q", texts)
	}
	log, _ := os.ReadFile(execlog.Path(w.Root()))
	for _, want := range []string{"[check cache step 1, 1 of 2] start pid ", "$ sh -c ", "[check cache step 1, 1 of 2] end pid ", ": passed in ", "[judge cache] start", "[judge cache] end", "[judge cache] step 1: the judge (fake-answer) finished in "} {
		if !strings.Contains(string(log), want) {
			t.Errorf("exec.log lacks %q:\n%s", want, log)
		}
	}
}

// A judge that can't be asked: that's logged, and the round clears what
// was running.
func TestChecks_AJudgeThatCantBeAskedIsLogged(t *testing.T) {
	w, _, dir := workFixture(t, "")
	w.cur.cfg.Judge.Harness = "no-such-harness"
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	w.CrewDone("cache", "c1")
	if !hasPrefixIn(logTexts(t, w), "step 1: the judge couldn't judge check 2: ") {
		t.Errorf("log: %q", logTexts(t, w))
	}
	if m, _ := w.Missions.Load("cache"); m.Checking != nil {
		t.Errorf("still running: %+v", m.Checking)
	}
}

// A Verify step's commands are followed and logged the same way.
func TestVerify_ItsCommandsAreFollowedAndLogged(t *testing.T) {
	fastLines(t)
	w, _, _ := reposFixture(t, strings.Replace(verifyPlan, "`test -f fixed` passes", "`"+slowCheck+"` passes", 1))
	c, _ := w.Missions.StartCrew("cache", "shell", "")
	w.Missions.SetCrewProcess("cache", c.ID, 1, "frontend/one")
	w.Missions.JoinStage("cache", c.ID, 1)
	w.Missions.SetCrewStatus("cache", c.ID, mission.CrewDone, "fixed")
	if _, err := w.AcceptStage("cache", AsOrchestrator, "fixed"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(w.WS().RepoDir("frontend"), "one")
	os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644)
	handed := make(chan error)
	go func() {
		_, err := w.HandOver("cache", "c2", verifierBundle("met"), dir)
		handed <- err
	}()
	got := whileRunning(t, w, func(c *mission.Checking) bool { return c.Line != "" })
	if got.Step != 2 || got.Crew != "c2" || got.N != 2 || got.Of != 3 || got.Command != slowCheck || len(got.Judged) != 0 || got.Line != "ok  internal/cli  4.2s" {
		t.Errorf("while the verify step's command runs: %+v", got)
	}
	os.WriteFile(filepath.Join(dir, "go"), nil, 0o644)
	select {
	case err := <-handed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the checks never finished")
	}
	if m, _ := w.Missions.Load("cache"); m.Checking != nil {
		t.Errorf("still running: %+v", m.Checking)
	}
	texts := logTexts(t, w)
	for _, want := range []string{"step 2, check 2 of 3 started: `" + slowCheck + "`", "step 2, check 2 of 3: `" + slowCheck + "` passed in "} {
		if !hasPrefixIn(texts, want) {
			t.Errorf("the log lacks %q: %q", want, texts)
		}
	}
	log, _ := os.ReadFile(execlog.Path(w.Root()))
	if !strings.Contains(string(log), "[check cache step 2, 2 of 3] start pid ") || !strings.Contains(string(log), "[check cache step 2, 2 of 3] end pid ") {
		t.Errorf("exec.log:\n%s", log)
	}
}

// Following a command's output changes nothing of what's kept of it.
func TestRunCheck_FollowingKeepsTheSameEvidence(t *testing.T) {
	fastLines(t)
	dir := t.TempDir()
	c := mission.Check{Command: "printf 'one\\n\\033[31mtwo\\033[0m\\n'; sleep 0.05; printf 'three'; sleep 0.05; exit 1", Expect: mission.CheckPasses}
	plain := runCheckWithin(dir, c, os.Environ(), checkHooks{}, checkTimeout)
	var lines []string
	pid := 0
	followed := runCheckWithin(dir, c, os.Environ(), checkHooks{Started: func(p int) { pid = p }, Line: func(l string) { lines = append(lines, l) }}, checkTimeout)
	if plain != followed || !strings.HasSuffix(plain.Evidence, "one\n\x1b[31mtwo\x1b[0m\nthree") {
		t.Errorf("without following: %+v\nfollowing: %+v", plain, followed)
	}
	if pid == 0 || len(lines) == 0 || lines[len(lines)-1] != "three" {
		t.Errorf("pid %d, lines %q", pid, lines)
	}
}

// The last line is the one being printed, if it's begun, else the last
// whole one that isn't blank: without colours, progress redrawn with \r
// taken at its latest.
func TestLastLine(t *testing.T) {
	var l lastLine
	if l.Line() != "" {
		t.Errorf("before anything: %q", l.Line())
	}
	for _, step := range []struct{ write, want string }{
		{"compiling", "compiling"},
		{"…\n", "compiling…"},
		{"\n\n", "compiling…"},
		{"\x1b[32mok\x1b[0m  internal/cli\t4.2s\n", "ok  internal/cli 4.2s"},
		{" 10%\r 50%\r", "50%"},
		{" 90%", "90%"},
		{"\rdone\n", "done"},
	} {
		l.Write([]byte(step.write))
		if got := l.Line(); got != step.want {
			t.Errorf("after %q: %q, want %q", step.write, got, step.want)
		}
	}
	if l.String() != "compiling…\n\n\n\x1b[32mok\x1b[0m  internal/cli\t4.2s\n 10%\r 50%\r 90%\rdone\n" {
		t.Errorf("all of it: %q", l.String())
	}
	var long lastLine
	long.Write([]byte(strings.Repeat("x", 5000)))
	if r := []rune(long.Line()); len(r) != lineMax || r[len(r)-1] != '…' {
		t.Errorf("a long line: %d runes", len(r))
	}
}
