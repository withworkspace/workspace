package runs

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/harness"
)

// The test binary stands in for a dev server: run with WS_RUNS_LISTEN, it
// listens on $PORT until it's ended.
func TestMain(m *testing.M) {
	if os.Getenv("WS_RUNS_LISTEN") != "" {
		l, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("PORT"))
		if err != nil {
			fmt.Println("couldn't listen:", err)
			os.Exit(3)
		}
		fmt.Println("listening on", l.Addr())
		for {
			c, err := l.Accept()
			if err != nil {
				os.Exit(0)
			}
			c.Close()
		}
	}
	os.Exit(m.Run())
}

func canReadProcs(t *testing.T) {
	t.Helper()
	if _, err := ReadProcs(); err != nil {
		t.Skipf("processes can't be read here: %v", err)
	}
}

// A workspace's runs, and a capsule to run in.
func setup(t *testing.T) (Store, string) {
	t.Helper()
	canReadProcs(t)
	root := t.TempDir()
	dir := filepath.Join(root, "repos", "demo", "feat")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := Store{Root: root}
	t.Cleanup(func() {
		list, _ := s.List()
		for _, r := range list {
			s.End(r)
		}
	})
	return s, dir
}

func start(t *testing.T, s Store, dir, name, command string, owner Owner, ttl time.Duration) Run {
	t.Helper()
	r, err := s.Start(Start{Name: name, Repo: "demo", Worktree: "feat", Dir: dir, Command: command, Owner: owner, TTL: ttl, By: "you"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// An owner to end.
func owner(t *testing.T) (Owner, func()) {
	t.Helper()
	cmd := exec.Command("sleep", "300")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	end := func() { cmd.Process.Kill(); cmd.Wait() }
	t.Cleanup(end)
	procs, _ := ReadProcs()
	o, ok := OwnerOf(procs, cmd.Process.Pid)
	if !ok {
		t.Fatal("the owner isn't running")
	}
	return o, end
}

func listener() string {
	return "WS_RUNS_LISTEN=1 " + os.Args[0]
}

func TestStartListensOnItsPort(t *testing.T) {
	s, dir := setup(t)
	r := start(t, s, dir, "web", listener()+" --port {port}", Owner{Kind: ByNone}, time.Hour)
	if r.Port < PortBase || r.Port >= PortBase+PortRange {
		t.Fatalf("port %d is outside the runs' range", r.Port)
	}
	if !strings.Contains(r.Command, fmt.Sprintf("--port %d", r.Port)) {
		t.Errorf("{port} wasn't replaced: %s", r.Command)
	}
	ready, err := s.Wait(r, 10*time.Second)
	if err != nil || !ready {
		t.Fatalf("Wait = %v, %v; log:\n%s", ready, err, r.Tail(10))
	}
	if r.Ref() != "web.feat.demo" {
		t.Errorf("Ref = %s", r.Ref())
	}
	b, err := os.ReadFile(filepath.Join(s.Dir(), "demo", "feat", "web", "run.json"))
	if err != nil || !strings.Contains(string(b), `"kind": "none"`) {
		t.Errorf("run.json: %v\n%s", err, b)
	}
}

func TestStartSaysWhenItExitsFirst(t *testing.T) {
	s, dir := setup(t)
	r := start(t, s, dir, "bad", "echo nope; exit 7", Owner{Kind: ByNone}, time.Hour)
	_, err := s.Wait(r, 10*time.Second)
	ended, ok := err.(*EndedError)
	if !ok {
		t.Fatalf("Wait's error = %v, want it ended", err)
	}
	if ended.Run.State != Exited || ended.Run.Status != 7 {
		t.Errorf("state %s, status %d; want exited 7", ended.Run.State, ended.Run.Status)
	}
	if !strings.Contains(ended.Run.Tail(5), "nope") {
		t.Errorf("its log hasn't what it said: %q", ended.Run.Tail(5))
	}
}

func TestANameTakenGetsTheNextFree(t *testing.T) {
	s, dir := setup(t)
	a := start(t, s, dir, "web", "sleep 300", Owner{Kind: ByNone}, time.Hour)
	b := start(t, s, dir, "web", "sleep 300", Owner{Kind: ByNone}, time.Hour)
	if a.Name != "web" || b.Name != "web-2" {
		t.Fatalf("names %s and %s, want web and web-2", a.Name, b.Name)
	}
	if a.Port == b.Port {
		t.Errorf("both on %d", a.Port)
	}
	// One that's ended gives its name up.
	if err := s.End(a); err != nil {
		t.Fatal(err)
	}
	c := start(t, s, dir, "web", "exit 0", Owner{Kind: ByNone}, time.Hour)
	if c.Name != "web" {
		t.Fatalf("name %s, want web again", c.Name)
	}
	waitState(t, s, c, Exited)
	d := start(t, s, dir, "web", "sleep 300", Owner{Kind: ByNone}, time.Hour)
	if d.Name != "web" {
		t.Errorf("name %s, want web: the one before had exited", d.Name)
	}
}

func TestARunStartedAgainKeepsItsPort(t *testing.T) {
	s, dir := setup(t)
	a := start(t, s, dir, "web", "sleep 300", Owner{Kind: ByNone}, time.Hour)
	s.End(a)
	b := start(t, s, dir, "web", "sleep 300", Owner{Kind: ByNone}, time.Hour)
	if a.Port != b.Port {
		t.Errorf("port %d, then %d", a.Port, b.Port)
	}
}

func waitState(t *testing.T, s Store, r Run, want State) Run {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		now, err := load(r.Path, s.snapshot())
		if err == nil && now.State == want {
			return now
		}
	}
	t.Fatalf("%s never got to %s", r.Ref(), want)
	return r
}

func TestSweepEndsARunWhoseOwnerHasGone(t *testing.T) {
	s, dir := setup(t)
	o, end := owner(t)
	// A child it starts is in its group, and ends with it.
	r := start(t, s, dir, "web", "sleep 300 & sleep 300", o, time.Hour)
	other := start(t, s, dir, "api", "sleep 300", Owner{Kind: ByNone}, time.Hour)
	waitGroup(t, s, r, 2)

	if swept, _ := s.Sweep(nil, time.Now()); len(swept) != 0 {
		t.Fatalf("swept %v while its owner was there", swept)
	}
	end()
	swept, err := s.Sweep(nil, time.Now())
	if err != nil || len(swept) != 1 || swept[0].Run.Name != "web" {
		t.Fatalf("Sweep = %v, %v; want web", swept, err)
	}
	if !strings.Contains(swept[0].Why, "its owner") {
		t.Errorf("why: %s", swept[0].Why)
	}
	if n := len(s.snapshot().group(r.PID, r.Began)); n != 0 {
		t.Errorf("%d of its processes are still running", n)
	}
	if _, err := os.Stat(r.Path); !os.IsNotExist(err) {
		t.Errorf("its folder is still there: %v", err)
	}
	if now, err := load(other.Path, s.snapshot()); err != nil || now.State != Running {
		t.Errorf("api, which nobody owns, was ended too: %v %s", err, now.State)
	}
}

func waitGroup(t *testing.T, s Store, r Run, n int) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if len(s.snapshot().group(r.PID, r.Began)) >= n {
			return
		}
	}
	t.Fatalf("%s's group never had %d processes", r.Ref(), n)
}

func TestAnOwnerWhosePIDIsGivenOutAgainHasGone(t *testing.T) {
	s, dir := setup(t)
	o, _ := owner(t)
	o.Began = o.Began.Add(-time.Hour) // what the record says isn't the process with its pid now
	start(t, s, dir, "web", "sleep 300", o, time.Hour)
	if swept, _ := s.Sweep(nil, time.Now()); len(swept) != 1 {
		t.Fatalf("swept %d, want the run whose owner's pid is another's now", len(swept))
	}
}

func TestSweepEndsARunWhoseTimeIsUp(t *testing.T) {
	s, dir := setup(t)
	r := start(t, s, dir, "web", "sleep 300", Owner{Kind: ByNone}, time.Hour)
	if swept, _ := s.Sweep(nil, time.Now().Add(59*time.Minute)); len(swept) != 0 {
		t.Fatal("swept before its time")
	}
	if _, err := s.Keep(r, 2*time.Hour, time.Now()); err != nil {
		t.Fatal(err)
	}
	if swept, _ := s.Sweep(nil, time.Now().Add(90*time.Minute)); len(swept) != 0 {
		t.Fatal("swept though it was kept for longer")
	}
	swept, _ := s.Sweep(nil, time.Now().Add(121*time.Minute))
	if len(swept) != 1 || swept[0].Why != "its time was up" {
		t.Fatalf("Sweep = %v", swept)
	}
}

func TestSweepAsksAfterOwnersInWSsRecords(t *testing.T) {
	s, dir := setup(t)
	start(t, s, dir, "web", "sleep 300", Owner{Kind: ByStep, Mission: "m1", Step: 2, StepText: "Build it"}, time.Hour)
	start(t, s, dir, "api", "sleep 300", Owner{Kind: ByPID, PID: os.Getpid()}, time.Hour)
	ended := ""
	var asked []Owner
	owners := OwnersFunc(func(o Owner) string { asked = append(asked, o); return ended })
	if swept, _ := s.Sweep(owners, time.Now()); len(swept) != 0 {
		t.Fatal("swept while its step was going")
	}
	if len(asked) != 1 || asked[0].StepText != "Build it" {
		t.Fatalf("asked after %v, want the step alone (a pid isn't ws's records')", asked)
	}
	ended = "step 2 of mission m1 has passed"
	swept, _ := s.Sweep(owners, time.Now())
	if len(swept) != 1 || swept[0].Why != ended || swept[0].Run.Name != "web" {
		t.Fatalf("Sweep = %v", swept)
	}
	if swept, _ := s.Sweep(nil, time.Now()); len(swept) != 0 {
		t.Fatal("with nobody to ask, something was swept")
	}
}

func TestEndRunsTheRecipesStop(t *testing.T) {
	s, dir := setup(t)
	r, err := s.Start(Start{Name: "stack", Repo: "demo", Worktree: "feat", Dir: dir, Command: "sleep 300",
		Recipe: "stack", Stop: "echo down $PORT > stopped", Owner: Owner{Kind: ByNone}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.End(r); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "stopped"))
	if err != nil || strings.TrimSpace(string(b)) != fmt.Sprintf("down %d", r.Port) {
		t.Fatalf("stop said %q, %v", b, err)
	}
	// A stop that fails says so, and the run's removed all the same.
	r, _ = s.Start(Start{Name: "stack", Repo: "demo", Worktree: "feat", Dir: dir, Command: "sleep 300",
		Stop: "echo nope; exit 3", Owner: Owner{Kind: ByNone}}, time.Now())
	err = s.End(r)
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("End = %v, want the stop's failure", err)
	}
	if _, serr := os.Stat(r.Path); !os.IsNotExist(serr) {
		t.Error("the run's still there")
	}
}

func TestWaitAsksTheReadyPath(t *testing.T) {
	s, dir := setup(t)
	// Something listens, but nothing answers HTTP: not up by its path.
	r, err := s.Start(Start{Name: "web", Repo: "demo", Worktree: "feat", Dir: dir, Command: listener(),
		Ready: "/health", Owner: Owner{Kind: ByNone}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ready, err := s.Wait(r, 2*time.Second)
	if ready || err != nil {
		t.Fatalf("Wait = %v, %v; want not up, no error", ready, err)
	}
	if !r.Listening() {
		t.Error("it isn't listening at all")
	}
}

func TestSweepEndsARunWhoseCapsuleHasGone(t *testing.T) {
	s, dir := setup(t)
	start(t, s, dir, "web", "sleep 300", Owner{Kind: ByCapsule}, time.Hour)
	if swept, _ := s.Sweep(nil, time.Now()); len(swept) != 0 {
		t.Fatal("swept while its capsule was there")
	}
	os.RemoveAll(dir)
	swept, _ := s.Sweep(nil, time.Now())
	if len(swept) != 1 || swept[0].Why != "where it ran has gone" {
		t.Fatalf("Sweep = %v", swept)
	}
}

func TestARunThatEndedIsKeptForAnHour(t *testing.T) {
	s, dir := setup(t)
	r := start(t, s, dir, "web", "exit 2", Owner{Kind: ByNone}, time.Hour)
	r = waitState(t, s, r, Exited)
	if r.Status != 2 {
		t.Errorf("status %d, want 2", r.Status)
	}
	if swept, _ := s.Sweep(nil, time.Now().Add(KeepEnded-time.Minute)); len(swept) != 0 {
		t.Fatal("removed before its hour was up")
	}
	if swept, _ := s.Sweep(nil, time.Now().Add(KeepEnded+time.Minute)); len(swept) != 1 {
		t.Fatal("kept past its hour")
	}
}

func TestARunKilledOutsideWSIsGone(t *testing.T) {
	s, dir := setup(t)
	r := start(t, s, dir, "web", "sleep 300", Owner{Kind: ByNone}, time.Hour)
	waitGroup(t, s, r, 1)
	syscall.Kill(-r.PID, syscall.SIGKILL)
	waitState(t, s, r, Gone)
}

func TestEndOnlyRemovesARunsFolder(t *testing.T) {
	s, _ := setup(t)
	other := t.TempDir()
	os.WriteFile(filepath.Join(other, recordFile), []byte("{}"), 0o644)
	if err := s.End(Run{Path: other}); err == nil {
		t.Fatal("End removed a folder outside the runs'")
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatal("it's gone")
	}
}

func TestFindOwnerIsTheNearestThatsBeenRunningAMinute(t *testing.T) {
	now := time.Now()
	p := ProcsOf([]harness.Proc{
		{PID: 10, PPID: 1, Start: now.Add(-time.Hour), Name: "login"},
		{PID: 20, PPID: 10, Start: now.Add(-10 * time.Minute), Name: "zsh"},
		{PID: 30, PPID: 20, Start: now.Add(-2 * time.Second), Name: "just"},
		{PID: 40, PPID: 30, Start: now.Add(-time.Second), Name: "sh"},
	})
	o := FindOwner(p, 40, now)
	if o.PID != 20 || o.Command != "zsh" || o.Kind != ByPID {
		t.Errorf("FindOwner = %+v, want zsh (20)", o)
	}
	if o := FindOwner(ProcsOf([]harness.Proc{{PID: 40, PPID: 1, Start: now}}), 40, now); o.Kind != "" {
		t.Errorf("FindOwner = %+v, want none", o)
	}
	if o := FindOwner(Procs{}, 40, now); o.Kind != "" {
		t.Errorf("FindOwner with no processes = %+v, want none", o)
	}
}

func TestProcessesThatCantBeReadHaveNotGone(t *testing.T) {
	if (Procs{}).gone(12345, time.Now()) {
		t.Error("gone, by processes nobody could read")
	}
}

func TestEnds(t *testing.T) {
	now := time.Date(2026, 11, 19, 14, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		r    Run
		want string
	}{
		{Run{Owner: Owner{Kind: ByPID, PID: 42, Command: "zsh"}, Expires: now.Add(4 * time.Hour)}, "when pid 42 (zsh) exits, or in 4h00"},
		{Run{Worktree: "feat", Owner: Owner{Kind: ByCapsule}}, "when capsule feat is burned or landed"},
		{Run{Owner: Owner{Kind: ByMission, Mission: "m1"}, Expires: now.Add(90*time.Minute + time.Second)}, "when mission m1 ends, or in 1h31"},
		{Run{Owner: Owner{Kind: ByNone}, Expires: now.Add(time.Hour)}, "in 1h00"},
		{Run{Owner: Owner{Kind: ByNone}}, "only when it's stopped"},
	} {
		if got := Ends(c.r, now); got != c.want {
			t.Errorf("Ends = %q, want %q", got, c.want)
		}
	}
}

func TestNameFor(t *testing.T) {
	for cmd, want := range map[string]string{
		"pnpm dev --port {port}":   "pnpm",
		"FOO=1 ./bin/My_Server -x": "my-server",
		"docker compose up":        "docker",
		"=":                        "run",
	} {
		if got := NameFor(cmd); got != want {
			t.Errorf("NameFor(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestFind(t *testing.T) {
	s, dir := setup(t)
	start(t, s, dir, "web", "sleep 300", Owner{Kind: ByNone}, time.Hour)
	other := filepath.Join(s.Root, "repos", "demo", "other")
	os.MkdirAll(other, 0o755)
	s.Start(Start{Name: "web", Repo: "demo", Worktree: "other", Dir: other, Command: "sleep 300", Owner: Owner{Kind: ByNone}}, time.Now())
	if r, err := s.Find("web", "demo", "feat"); err != nil || r.Worktree != "feat" {
		t.Errorf("web, standing in feat: %v %v", r.Ref(), err)
	}
	if r, err := s.Find("web.other.demo", "", ""); err != nil || r.Worktree != "other" {
		t.Errorf("web.other.demo: %v %v", r.Ref(), err)
	}
	if _, err := s.Find("web", "", ""); err == nil || !strings.Contains(err.Error(), "several") {
		t.Errorf("web, from nowhere: %v, want it to say there are several", err)
	}
}

func TestALookingRunEndsWhenNobodyHasLooked(t *testing.T) {
	s, dir := setup(t)
	start(t, s, dir, "web", "sleep 300", Owner{Kind: ByLooking, Mission: "m1"}, time.Hour)
	now := time.Now()
	if swept, _ := s.Sweep(nil, now.Add(LookLinger-time.Minute)); len(swept) != 0 {
		t.Fatal("swept while it had been looked at")
	}
	// Someone has m1 open 10 minutes on: it lasts 15 from then.
	if err := s.Looked(Looking{Missions: map[string]bool{"m1": true}}, now.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if swept, _ := s.Sweep(nil, now.Add(20*time.Minute)); len(swept) != 0 {
		t.Fatal("swept though it was looked at since")
	}
	swept, _ := s.Sweep(nil, now.Add(26*time.Minute))
	if len(swept) != 1 || !strings.Contains(swept[0].Why, "nobody has looked") {
		t.Fatalf("Sweep = %v", swept)
	}
}

// An app's run (one of the workspace's own, opened from its page) is kept
// by its page being open, as a mission's is, and by no other page's.
func TestAnAppsRunEndsWhenNobodyHasItsPageOpen(t *testing.T) {
	s, _ := setup(t)
	r, err := s.Start(Start{Name: "docs", Dir: s.Root, Command: "sleep 300", Recipe: "docs", Owner: Owner{Kind: ByLooking, App: "docs"}, By: "you"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if r.Ref() != "docs" || !r.OfWorkspace() || !strings.HasSuffix(r.Path, filepath.Join("@workspace", "@", "docs")) {
		t.Fatalf("ref %s, kept at %s", r.Ref(), r.Path)
	}
	now := time.Now()
	// A mission's page open, and a worktree's files, keep nothing of this.
	s.Looked(Looking{Missions: map[string]bool{"": true, "m1": true}, Worktrees: map[string]bool{"/": true, "demo/feat": true}}, now.Add(10*time.Minute))
	if swept, _ := s.Sweep(nil, now.Add(20*time.Minute)); len(swept) != 1 || !strings.Contains(swept[0].Why, "nobody has looked") {
		t.Fatalf("Sweep = %v", swept)
	}

	r, _ = s.Start(Start{Name: "docs", Dir: s.Root, Command: "sleep 300", Recipe: "docs", Owner: Owner{Kind: ByLooking, App: "docs"}, By: "you"}, now)
	// Its page opens it again every minute while it's open.
	if err := s.LookedRun(r, now.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if swept, _ := s.Sweep(nil, now.Add(20*time.Minute)); len(swept) != 0 {
		t.Fatal("swept though its page was open since")
	}
	if got := Ends(r, now); got != "15 minutes after nobody has docs open" {
		t.Errorf("Ends = %q", got)
	}
}

// One started from the Files page is looked at while its own worktree is
// open there, not any mission, nor another worktree.
func TestALookingRunFromFilesIsLookedAtByItsWorktree(t *testing.T) {
	s, dir := setup(t)
	start(t, s, dir, "web", "sleep 300", Owner{Kind: ByLooking}, time.Hour)
	now := time.Now()
	s.Looked(Looking{Missions: map[string]bool{"m1": true}, Worktrees: map[string]bool{"demo/other": true}}, now.Add(10*time.Minute))
	if swept, _ := s.Sweep(nil, now.Add(16*time.Minute)); len(swept) != 1 {
		t.Fatalf("a mission open, or another worktree, kept it: swept %v", swept)
	}

	start(t, s, dir, "web", "sleep 300", Owner{Kind: ByLooking}, time.Hour)
	now = time.Now()
	s.Looked(Looking{Worktrees: map[string]bool{"demo/feat": true}}, now.Add(10*time.Minute))
	if swept, _ := s.Sweep(nil, now.Add(20*time.Minute)); len(swept) != 0 {
		t.Fatal("swept though its files were looked at since")
	}
}

// A recipe's run, as tests ask for one.
func recipe(name string, o Owner, ttl time.Duration, needs ...Need) Start {
	return Start{Name: name, Repo: "demo", Worktree: "feat", Command: "sleep 300", Recipe: name, Owner: o, TTL: ttl, By: "you", Needs: needs}
}

func ensure(t *testing.T, s Store, dir string, o Start, now time.Time) (Run, bool) {
	t.Helper()
	o.Dir = dir
	r, started, err := s.Ensure(o, now)
	if err != nil {
		t.Fatal(err)
	}
	return r, started
}

func names(swept []Swept) string {
	var out []string
	for _, s := range swept {
		out = append(out, s.Run.Name)
	}
	return strings.Join(out, ",")
}

// Asked for twice, a recipe is one run: the second asker is one more of
// who it's for, and it has the longer of their times.
func TestARecipeRunsOnceInAWorktree(t *testing.T) {
	s, dir := setup(t)
	now := time.Now()
	a, _ := owner(t)
	b, _ := owner(t)
	first, started := ensure(t, s, dir, recipe("web", a, time.Hour), now)
	if !started {
		t.Fatal("the first wasn't started")
	}
	second, started := ensure(t, s, dir, recipe("web", b, 3*time.Hour), now)
	if started || second.PID != first.PID || second.Name != "web" {
		t.Fatalf("the second is %s (pid %d, started %v), want the first (pid %d)", second.Name, second.PID, started, first.PID)
	}
	if len(second.Also) != 1 || second.Also[0].PID != b.PID {
		t.Errorf("who else it's for: %+v", second.Also)
	}
	if want := now.Add(3 * time.Hour).UTC().Truncate(time.Second); !second.Expires.Equal(want) {
		t.Errorf("expires %s, want the later, %s", second.Expires, want)
	}
	// The same asker again adds nothing, and a shorter time takes none away.
	third, _ := ensure(t, s, dir, recipe("web", b, time.Hour), now)
	if len(third.Also) != 1 || !third.Expires.Equal(second.Expires) {
		t.Errorf("asked again: also %+v, expires %s", third.Also, third.Expires)
	}
	// A command that's no recipe's still gets the next free name.
	other := start(t, s, dir, "web", "sleep 300", Owner{Kind: ByNone}, time.Hour)
	if other.Name != "web-2" {
		t.Errorf("a command named web is %s", other.Name)
	}
	if list, _ := s.List(); len(list) != 2 {
		t.Errorf("%d runs, want the recipe's one and the command's", len(list))
	}
}

// Two ws starting a recipe at one moment start it once: the one that
// finds the name being claimed waits for what the other records.
func TestARecipeAskedForTwiceAtOnceIsStartedOnce(t *testing.T) {
	s, dir := setup(t)
	type got struct {
		r       Run
		started bool
		err     error
	}
	out := make(chan got, 2)
	for range 2 {
		go func() {
			o := recipe("web", Owner{Kind: ByNone}, time.Hour)
			o.Dir = dir
			r, started, err := s.Ensure(o, time.Now())
			out <- got{r, started, err}
		}()
	}
	a, b := <-out, <-out
	if a.err != nil || b.err != nil {
		t.Fatal(a.err, b.err)
	}
	if a.started == b.started || a.r.PID != b.r.PID {
		t.Errorf("started %v and %v, pids %d and %d: want one start, and both the same run", a.started, b.started, a.r.PID, b.r.PID)
	}
	if list, _ := s.List(); len(list) != 1 || list[0].Name != "web" {
		t.Errorf("runs: %v", list)
	}
}

// A ws that read the processes before another started the recipe doesn't
// take the run for ended, and its name for its own: it looks again, finds
// it running, and is one more it's for.
func TestARecipeStartedSinceTheProcessesWereReadIsntStartedAgain(t *testing.T) {
	s, dir := setup(t)
	o := recipe("web", Owner{Kind: ByNone}, time.Hour)
	o.Dir = dir
	first, started, err := s.Ensure(o, time.Now())
	if err != nil || !started {
		t.Fatal(started, err)
	}
	// The other ws: what it reads of the processes is from before the run
	// began, until it looks again.
	late, reads := s, 0
	late.Procs = func() (Procs, error) {
		if reads++; reads <= 2 {
			return Procs{Known: true}, nil // read, with the run not among them yet
		}
		return ReadProcs()
	}
	second, started, err := late.Ensure(o, time.Now())
	if err != nil || started || second.PID != first.PID {
		t.Errorf("started %v, pid %d (the first's is %d), %v: want the run that's there", started, second.PID, first.PID, err)
	}
	if list, _ := s.List(); len(list) != 1 || list[0].PID != first.PID || list[0].State != Running {
		t.Errorf("runs: %+v", list)
	}
}

func TestARunIsEndedOnceEveryoneItsForHasGone(t *testing.T) {
	s, dir := setup(t)
	now := time.Now()
	a, endA := owner(t)
	b, endB := owner(t)
	ensure(t, s, dir, recipe("web", a, time.Hour), now)
	r, _ := ensure(t, s, dir, recipe("web", b, time.Hour), now)
	if got := Ends(r, now); !strings.Contains(got, fmt.Sprintf("when pid %d", a.PID)) || !strings.Contains(got, fmt.Sprintf(", and when pid %d", b.PID)) {
		t.Errorf("Ends = %q", got)
	}
	endA()
	if swept, _ := s.Sweep(nil, now); len(swept) != 0 {
		t.Fatalf("swept %s with one it's for still there", names(swept))
	}
	endB()
	swept, _ := s.Sweep(nil, now)
	if names(swept) != "web" || strings.Count(swept[0].Why, "has gone") != 2 {
		t.Fatalf("Sweep = %v", swept)
	}
}

// What a run needs is kept while it runs, and counted afresh each sweep:
// two runs needing one thing keep it until the last of them has gone.
func TestWhatARunNeedsIsKeptUntilTheLastThatNeedsItGoes(t *testing.T) {
	s, dir := setup(t)
	now := time.Now()
	api, _ := ensure(t, s, dir, recipe("api", Owner{Kind: ByRun}, 0), now)
	need := Need{Name: "api", Run: api.Ref(), Repo: "demo", Worktree: "feat", Recipe: "api", Addr: api.Link()}
	a, endA := owner(t)
	b, endB := owner(t)
	ensure(t, s, dir, recipe("web", a, time.Hour, need), now)
	ensure(t, s, dir, recipe("admin", b, time.Hour, need), now)

	list, _ := s.List()
	var kept Run
	for _, r := range list {
		if r.Name == "api" {
			kept = r
		}
	}
	if strings.Join(kept.NeededBy, ",") != "admin.feat.demo,web.feat.demo" {
		t.Fatalf("api is needed by %v", kept.NeededBy)
	}
	if got := Ends(kept, now); got != "when admin.feat.demo and web.feat.demo have ended" {
		t.Errorf("Ends = %q", got)
	}
	if swept, _ := s.Sweep(nil, now); len(swept) != 0 {
		t.Fatalf("swept %s while both ran", names(swept))
	}
	endA()
	if swept, _ := s.Sweep(nil, now); names(swept) != "web" {
		t.Fatalf("swept %q, want web alone: admin still needs api", names(swept))
	}
	endB()
	// The run that needed it and the run it needed go in one sweep.
	swept, _ := s.Sweep(nil, now)
	if names(swept) != "admin,api" {
		t.Fatalf("swept %q, want admin and api", names(swept))
	}
	for _, sw := range swept {
		if sw.Run.Name == "api" && sw.Why != "nothing needs it now" {
			t.Errorf("api: %s", sw.Why)
		}
	}
}

// A run that was someone's before another needed it outlasts them, and
// its time, while it's needed; a run whose place has gone doesn't.
func TestARunSomethingNeedsOutlastsItsOwnerAndItsTime(t *testing.T) {
	s, dir := setup(t)
	now := time.Now()
	o, end := owner(t)
	api, _ := ensure(t, s, dir, recipe("api", o, time.Hour), now)
	need := Need{Name: "api", Run: api.Ref(), Addr: api.Link()}
	web, _ := ensure(t, s, dir, recipe("web", Owner{Kind: ByNone}, 4*time.Hour, need), now)
	end()
	later := now.Add(2 * time.Hour)
	if swept, _ := s.Sweep(nil, later); len(swept) != 0 {
		t.Fatalf("swept %s, though web needs api", names(swept))
	}
	list, _ := s.List()
	if got := Ends(list[0], later); !strings.HasSuffix(got, ", and not while web.feat.demo needs it") {
		t.Errorf("Ends = %q", got)
	}
	if why := Stale(list[0], s.snapshot(), nil, later); why != "" {
		t.Errorf("Stale = %q for a run that's needed", why)
	}
	if err := s.End(web); err != nil {
		t.Fatal(err)
	}
	if swept, _ := s.Sweep(nil, later); names(swept) != "api" {
		t.Fatalf("swept %q once nothing needed it", names(swept))
	}

	// Needed or not, a run goes with the place it ran in.
	gone := filepath.Join(filepath.Dir(dir), "gone")
	os.MkdirAll(gone, 0o755)
	o2 := recipe("db", Owner{Kind: ByRun}, 0)
	o2.Worktree = "gone"
	db, _ := ensure(t, s, gone, o2, now)
	ensure(t, s, dir, recipe("web", Owner{Kind: ByNone}, time.Hour, Need{Name: "db", Run: db.Ref(), Addr: db.Link()}), now)
	os.RemoveAll(gone)
	if swept, _ := s.Sweep(nil, now); names(swept) != "db" || swept[0].Why != "where it ran has gone" {
		t.Fatalf("Sweep = %v", swept)
	}
}

// A run started for another's need is up before the run it's for is
// recorded: it's held until then, and no longer.
func TestARunStartedForANeedIsHeldWhileWhatNeedsItStarts(t *testing.T) {
	s, dir := setup(t)
	now := time.Now()
	o := recipe("api", Owner{Kind: ByRun}, 0)
	o.Hold = now.Add(time.Minute)
	api, _ := ensure(t, s, dir, o, now)
	if swept, _ := s.Sweep(nil, now.Add(30*time.Second)); len(swept) != 0 {
		t.Fatalf("swept %s while it was held", names(swept))
	}
	if got := Ends(api, now); got != "within seconds, as nothing needs it now" {
		t.Errorf("Ends = %q", got)
	}
	api, err := s.HoldUntil(api, time.Time{})
	if err != nil || !api.Hold.IsZero() {
		t.Fatalf("HoldUntil: %v, %s", err, api.Hold)
	}
	if swept, _ := s.Sweep(nil, now); names(swept) != "api" || swept[0].Why != "nothing needs it now" {
		t.Fatalf("Sweep = %v", swept)
	}
}

// A start killed before it recorded anything leaves a folder with no
// record: the name isn't lost to it.
func TestAStartThatDiedGivesUpItsName(t *testing.T) {
	s, dir := setup(t)
	old := claimWait
	claimWait = 50 * time.Millisecond
	t.Cleanup(func() { claimWait = old })
	dead := filepath.Join(s.Dir(), "demo", "feat", "web")
	os.MkdirAll(dead, 0o755)
	past := time.Now().Add(-time.Minute)
	os.Chtimes(dead, past, past)
	r, started := ensure(t, s, dir, recipe("web", Owner{Kind: ByNone}, time.Hour), time.Now())
	if !started || r.Name != "web" {
		t.Errorf("started %v as %s, want web", started, r.Name)
	}
}

// A run that listens on ::1 alone is listening, so its Ready path is asked
// there too: it isn't left waiting on 127.0.0.1.
func TestUpAsksTheReadyPathOnTheOtherLoopback(t *testing.T) {
	l, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skip("no IPv6 loopback here:", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if c, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port))); err == nil {
		c.Close()
		l.Close()
		t.Skip("something else has the port on 127.0.0.1")
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.Error(w, "not yet", http.StatusServiceUnavailable)
		}
	})}
	go srv.Serve(l)
	t.Cleanup(func() { srv.Close() })

	if r := (Run{Port: port, Ready: "/health"}); !r.Up() {
		t.Error("a run answering its Ready path on ::1 isn't up")
	}
	if r := (Run{Port: port, Ready: "/other"}); r.Up() {
		t.Error("a run answering 503 on ::1 is up")
	}
}
