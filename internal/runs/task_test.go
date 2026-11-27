package runs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A task: one that ends by itself, and is kept for what it came to.

func task(t *testing.T, s Store, dir, name, command string, reports ...Report) Run {
	t.Helper()
	r, err := s.Start(Start{Name: name, Type: Task, Repo: "demo", Worktree: "feat", Dir: dir, Command: command,
		Owner: Owner{Kind: ByNone}, TTL: time.Hour, By: "you", Commit: "abc123", Reports: reports}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func ended(t *testing.T, s Store, r Run) Run {
	t.Helper()
	now, done, err := s.WaitEnd(r, 10*time.Second, nil)
	if err != nil || !done {
		t.Fatalf("%s didn't end: %v", r.Ref(), err)
	}
	return now
}

func TestATaskHasNoPortAndIsKeptForWhatItCameTo(t *testing.T) {
	s, dir := setup(t)
	r := task(t, s, dir, "test", `echo "PORT=[$PORT]"; echo two; exit 3`)
	if r.Port != 0 || !r.IsTask() || r.N != 1 {
		t.Fatalf("port %d, task %v, n %d: want no port, a task, its first run", r.Port, r.IsTask(), r.N)
	}
	r = ended(t, s, r)
	if r.State != Exited || r.Status != 3 || r.Result() != "failed (exit 3)" || r.Passed() {
		t.Errorf("state %s status %d result %q", r.State, r.Status, r.Result())
	}
	if got := r.Tail(5); got != "PORT=[]\ntwo" {
		t.Errorf("its output = %q: all it printed, and no PORT", got)
	}
	if r.Commit != "abc123" {
		t.Errorf("commit = %q", r.Commit)
	}
	// A sweep long after a service would have been removed leaves it.
	if swept, err := s.Sweep(nil, time.Now().Add(2*KeepEnded)); err != nil || len(swept) != 0 {
		t.Fatalf("swept %s, %v: a task's result is kept", names(swept), err)
	}
	if swept, _ := s.Sweep(nil, time.Now().Add(KeepResults+time.Hour)); names(swept) != "test" {
		t.Errorf("swept %q: it goes once KeepResults have passed", names(swept))
	}
}

func TestATaskRunAgainKeepsTheRunsBeforeIt(t *testing.T) {
	s, dir := setup(t)
	first := ended(t, s, task(t, s, dir, "lint", "echo first; exit 1"))
	second := ended(t, s, task(t, s, dir, "lint", "echo second"))
	if second.Name != "lint" || second.N != 2 || !second.Passed() {
		t.Fatalf("the second run is %s #%d, %s: want lint #2, passed", second.Name, second.N, second.Result())
	}
	list, _ := s.List()
	if len(list) != 1 || list[0].Past != 1 {
		t.Fatalf("list = %d runs, the first with %d behind it: want the latest, with 1 behind", len(list), list[0].Past)
	}
	past := s.Past(list[0])
	if len(past) != 1 || past[0].N != 1 || past[0].Status != 1 || past[0].Tail(1) != "first" {
		t.Fatalf("past = %+v", past)
	}
	if got, err := s.Find("lint.feat.demo#1", "", ""); err != nil || got.Tail(1) != "first" {
		t.Errorf("lint#1 = %q, %v", got.Tail(1), err)
	}
	if got, err := s.Find("lint#2", "demo", "feat"); err != nil || got.N != 2 {
		t.Errorf("lint#2 = %d, %v", got.N, err)
	}
	if _, err := s.Find("lint#9", "demo", "feat"); err == nil {
		t.Error("lint#9 was found")
	}
	_ = first
	// No more than KeepPast are kept behind it.
	for range KeepPast + 2 {
		ended(t, s, task(t, s, dir, "lint", "true"))
	}
	list, _ = s.List()
	if list[0].Past != KeepPast || list[0].N != KeepPast+4 {
		t.Errorf("%d behind run %d: want %d behind run %d", list[0].Past, list[0].N, KeepPast, KeepPast+4)
	}
	// Ending one that has ended clears it, and the runs behind it.
	if err := s.End(list[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), pastDir)); !os.IsNotExist(err) {
		t.Errorf("its past is still there: %v", err)
	}
}

func TestATaskKeepsTheReportsItWrote(t *testing.T) {
	s, dir := setup(t)
	// One from before the run isn't this run's.
	os.WriteFile(filepath.Join(dir, "old.xml"), []byte("old"), 0o644)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(dir, "old.xml"), old, old)
	r := ended(t, s, task(t, s, dir, "test", "mkdir -p junit && echo cov > coverage.out && echo a > junit/a.xml && echo b > junit/b.xml",
		Report{Name: "coverage", Path: "coverage.out", Format: "gocover"},
		Report{Name: "tests", Path: "junit/*.xml", Format: "junit"},
		Report{Name: "stale", Path: "old.xml"},
		Report{Name: "none", Path: "nothing.out"}))
	if got := r.ReportFiles("coverage"); len(got) != 1 || filepath.Base(got[0]) != "coverage.out" {
		t.Errorf("coverage = %v", got)
	}
	if b, _ := os.ReadFile(filepath.Join(r.ReportsDir(), "coverage", "coverage.out")); strings.TrimSpace(string(b)) != "cov" {
		t.Errorf("coverage.out = %q", b)
	}
	if got := r.ReportFiles("tests"); len(got) != 2 {
		t.Errorf("tests = %v: want both files the pattern matches", got)
	}
	if got := r.ReportFiles("stale"); len(got) != 0 {
		t.Errorf("stale = %v: a file from before the run isn't its report", got)
	}
	if got := r.ReportFiles("none"); len(got) != 0 {
		t.Errorf("none = %v", got)
	}
	for _, bad := range []Report{{Name: "x", Path: "../up"}, {Name: "x", Path: "/abs"}, {Name: "x", Path: "a; rm -rf b"}, {Name: "Bad Name", Path: "a"}, {Name: "x"}} {
		if _, err := s.Start(Start{Name: "bad", Type: Task, Repo: "demo", Worktree: "feat", Dir: dir, Command: "true", Reports: []Report{bad}}, time.Now()); err == nil {
			t.Errorf("report %+v was taken", bad)
		}
	}
}

func TestATaskStoppedIsKeptAsStopped(t *testing.T) {
	s, dir := setup(t)
	o, end := owner(t)
	r, err := s.Start(Start{Name: "slow", Type: Task, Repo: "demo", Worktree: "feat", Dir: dir, Command: "sleep 300", Owner: o, TTL: time.Hour}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	waitGroup(t, s, r, 1)
	end()
	swept, err := s.Sweep(nil, time.Now())
	if err != nil || names(swept) != "slow" {
		t.Fatalf("swept %q, %v: its owner has gone", names(swept), err)
	}
	now := waitState(t, s, r, Gone)
	if now.Result() != "stopped" || !strings.Contains(now.Why, ") exited") {
		t.Errorf("result %q, why %q: want stopped, as its owner went", now.Result(), now.Why)
	}
	// And a recipe's task that's running is the one run of it.
	a, started := ensure(t, s, dir, Start{Name: "t", Type: Task, Repo: "demo", Worktree: "feat", Recipe: "t", Command: "sleep 300", Owner: Owner{Kind: ByNone}, TTL: time.Hour}, time.Now())
	b, again := ensure(t, s, dir, Start{Name: "t", Type: Task, Repo: "demo", Worktree: "feat", Recipe: "t", Command: "sleep 300", Owner: Owner{Kind: ByNone}, TTL: time.Hour}, time.Now())
	if !started || again || a.PID != b.PID {
		t.Errorf("started %v, again %v, pids %d and %d: want one run", started, again, a.PID, b.PID)
	}
}
