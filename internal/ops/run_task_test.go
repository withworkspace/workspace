package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/runs"
)

// A recipe under [run.task] is run to its end, with no port, and kept with
// the reports it names.
func TestStartRunATaskIsKeptWithItsReports(t *testing.T) {
	w, front := needsFixture(t)
	write(t, filepath.Join(front, "ws.repo.toml"), `
[run.task.test]
command = "echo cov > coverage.out; echo ran; exit 2"

[run.task.test.reports.coverage]
path = "coverage.out"
format = "gocover"

[run.service.server]
command = "sleep 300"

[run.service.server.needs.test]

[run.task.slow]
command = "sleep 300"
timeout = "90s"

[run.task.never]
command = "true"
timeout = "soon"

[run.service.twin]
command = "sleep 300"

[run.task.twin]
command = "true"
`)
	up, err := w.StartRun(runAsk(front, "test"))
	if err != nil {
		t.Fatal(err)
	}
	if !up.Run.IsTask() || up.Run.Port != 0 || up.Run.N != 1 {
		t.Fatalf("run = %+v: want a task, no port, its first run", up.Run.Record)
	}
	r, ended, err := w.runStore().WaitEnd(up.Run, 10*time.Second, nil)
	if err != nil || !ended {
		t.Fatalf("it didn't end: %v", err)
	}
	if r.Result() != "failed (exit 2)" || r.Tail(1) != "ran" {
		t.Errorf("result %q, last words %q", r.Result(), r.Tail(1))
	}
	if len(r.Reports) != 1 || r.Reports[0].Format != "gocover" || len(r.ReportFiles("coverage")) != 1 {
		t.Errorf("reports = %+v, files %v", r.Reports, r.ReportFiles("coverage"))
	}
	// What the page reads of it: the run, and those behind it.
	if _, err := w.StartRun(runAsk(front, "test")); err != nil {
		t.Fatal(err)
	}
	latest, past, err := w.Run("test.cache.frontend")
	if err != nil || latest.N != 2 || len(past) != 1 || past[0].N != 1 {
		t.Errorf("latest %d, past %d, %v: want run 2 with run 1 behind it", latest.N, len(past), err)
	}
	if one, _, err := w.Run("test.cache.frontend#1"); err != nil || one.N != 1 || one.Status != 2 {
		t.Errorf("#1 = %d (exit %d), %v", one.N, one.Status, err)
	}

	// A task's time is its timeout: an hour unsaid, its recipe's, or the
	// asker's over that; and asking for it again gives it no more.
	if got := latest.Expires.Sub(latest.Created); got != runs.TaskTimeout {
		t.Errorf("test may take %s, want %s", got, runs.TaskTimeout)
	}
	slow, err := w.StartRun(runAsk(front, "slow"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.runStore().End(slow.Run) })
	if got := slow.Run.Expires.Sub(slow.Run.Created); got != 90*time.Second {
		t.Errorf("slow may take %s, want its recipe's 90s", got)
	}
	again := runAsk(front, "slow")
	again.Timeout = time.Hour
	if up, err := w.StartRun(again); err != nil || up.Started || !up.Run.Expires.Equal(slow.Run.Expires) {
		t.Errorf("slow asked for again: started %v, ends %s (was %s), %v", up.Started, up.Run.Expires, slow.Run.Expires, err)
	}
	// Its timeout passed, it's stopped, and kept as that, with why.
	swept, err := w.runStore().Sweep(runs.OwnersFunc(func(runs.Owner) string { return "" }), slow.Run.Expires.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	var why string
	for _, sw := range swept {
		if sw.Run.Name == "slow" {
			why = sw.Why
		}
	}
	if !strings.Contains(why, "timed out after 1m30s") {
		t.Errorf("slow was swept for %q", why)
	}
	if now, _, err := w.Run("slow.cache.frontend"); err != nil || now.Result() != "stopped" || !strings.Contains(now.Why, "timed out after 1m30s") {
		t.Errorf("slow = %s (%q), %v", now.Result(), now.Why, err)
	}

	for recipe, want := range map[string]string{
		"server": "is a task: a run can only need a service",
		"twin":   "twin is both a service and a task in ws.repo.toml",
		"never":  `[run.task.never] in ws.repo.toml: timeout must be a duration (10m, 1h30m), not "soon"`,
	} {
		if _, err := w.StartRun(runAsk(front, recipe)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", recipe, err, want)
		}
	}
}

// A step's check that's a command is run as a task, and its result names
// the run that's kept of it.
func TestChecks_ACommandIsKeptAsARun(t *testing.T) {
	if _, err := runs.ReadProcs(); err != nil {
		t.Skipf("processes can't be read here: %v", err)
	}
	w, _, dir := workFixture(t, "1. met: it's cached")
	plan := strings.Replace(workPlan, "`test -f cache.go` passes", "`ws run unit` passes", 1)
	plan = strings.Replace(plan, "- check: the lookup is cached", "- check: `echo all of it; test -f cache.go` passes\n   - check: the lookup is cached", 1)
	if err := w.Missions.WritePlan("cache", plan); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "ws.repo.toml"), "[run.task.unit]\ncommand = \"echo unit ran > unit.out; echo units pass\"\n[run.task.unit.reports.out]\npath = \"unit.out\"\n")
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	w.CrewDone("cache", "c1")

	st := stepRecord(t, w, 1)
	if len(st.Record.Checks) != 1 {
		t.Fatalf("rounds = %d", len(st.Record.Checks))
	}
	res := st.Record.Checks[0].Results
	if res[0].Result != mission.CheckMet || res[0].Run != "unit.one.frontend#1" || !strings.HasSuffix(res[0].Evidence, "units pass") {
		t.Errorf("the recipe's check = %+v: want met, by the recipe's own run", res[0])
	}
	if res[1].Result != mission.CheckMet || res[1].Run != "check-1-2.one.frontend#1" {
		t.Errorf("the command's check = %+v", res[1])
	}
	if res[2].Run != "" {
		t.Errorf("the judge's check names a run: %+v", res[2])
	}
	unit, _, err := w.Run(res[0].Run)
	if err != nil || len(unit.ReportFiles("out")) != 1 || unit.Check == nil || unit.Check.Step != 1 || unit.Check.N != 1 || unit.Owner.Mission != "cache" {
		t.Errorf("the recipe's run = %+v, %v: want it kept with its report, for check 1 of step 1", unit.Record, err)
	}
	cmd, _, err := w.Run(res[1].Run)
	if err != nil || !cmd.Passed() || cmd.Tail(1) != "all of it" {
		t.Errorf("the command's run = %q %q, %v", cmd.Result(), cmd.Tail(1), err)
	}
}
