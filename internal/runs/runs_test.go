package runs

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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
	r, err := s.Keep(r, 2*time.Hour, time.Now())
	if err != nil {
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

func TestSweepEndsAMissionsRunOnceItEnds(t *testing.T) {
	s, dir := setup(t)
	start(t, s, dir, "web", "sleep 300", Owner{Kind: ByMission, Mission: "m1"}, time.Hour)
	inPlay := map[string]bool{"m1": true}
	known := true
	missions := func() (map[string]bool, bool) { return inPlay, known }
	if swept, _ := s.Sweep(missions, time.Now()); len(swept) != 0 {
		t.Fatal("swept while its mission was in play")
	}
	inPlay, known = map[string]bool{}, false
	if swept, _ := s.Sweep(missions, time.Now()); len(swept) != 0 {
		t.Fatal("swept when the missions couldn't be asked")
	}
	known = true
	swept, _ := s.Sweep(missions, time.Now())
	if len(swept) != 1 || swept[0].Why != "mission m1 has ended" {
		t.Fatalf("Sweep = %v", swept)
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
		{Run{Record: Record{Owner: Owner{Kind: ByPID, PID: 42, Command: "zsh"}, Expires: now.Add(4 * time.Hour)}}, "when pid 42 (zsh) exits, or in 4h00"},
		{Run{Record: Record{Worktree: "feat", Owner: Owner{Kind: ByCapsule}}}, "when capsule feat is burned or landed"},
		{Run{Record: Record{Owner: Owner{Kind: ByMission, Mission: "m1"}, Expires: now.Add(90*time.Minute + time.Second)}}, "when mission m1 ends, or in 1h31"},
		{Run{Record: Record{Owner: Owner{Kind: ByNone}, Expires: now.Add(time.Hour)}}, "in 1h00"},
		{Run{Record: Record{Owner: Owner{Kind: ByNone}}}, "only when it's stopped"},
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
