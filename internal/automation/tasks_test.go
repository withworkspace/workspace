package automation

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/events"
)

const coverageAll = `# Run coverage once a day, on every repo that has it.
when = every("day 03:00")
may = ["run", "notify"]

def run(e):
    targets = [r for r in repos() if "coverage" in r.tasks]
    started = [run_task("coverage", repo = r.name) for r in targets]
    results = wait_all(started, timeout = "2h")
    for res in results:
        if res and res.passed:
            notify("%s %s" % (res.repo, res.reports.coverage.total))
        else:
            notify("no coverage")
`

// A run asks for a task on each repo that has it and is given a handle
// at once; ws starts them as there's room (one at a time here), and the
// run waits for all of them, given what each came to: its status and
// what its reports say. Starting and waiting are steps: a run that's
// picked up again asks for nothing twice.
func TestTasks_FanOutAndJoin(t *testing.T) {
	e, got := engine(t)
	later := clock(e)
	e.MaxTasks = 1
	var started []string
	e.Hooks.Repos = func() []RepoInfo {
		return []RepoInfo{
			{Name: "api", Group: "backend", Tasks: []TaskInfo{{Name: "coverage", Reports: map[string]string{"coverage": "gocover"}}, {Name: "test"}}},
			{Name: "web", Group: "frontend", Tasks: []TaskInfo{{Name: "coverage"}}},
			{Name: "records"},
		}
	}
	e.Hooks.RunTask = func(a *Automation, repo, recipe string, chain []string) (string, int, error) {
		started = append(started, fmt.Sprintf("%s %s %s by %s", recipe, repo, strings.Join(chain, ">"), a.Name))
		return recipe + ".ground." + repo, 3, nil
	}
	e.Hooks.Report = func(ref string, n int) map[string]map[string]any {
		if strings.HasSuffix(ref, ".api") {
			return map[string]map[string]any{"coverage": {"total": 71.4}}
		}
		return map[string]map[string]any{"coverage": {"total": 55.0}}
	}
	if _, err := e.Add("coverage", []byte(coverageAll)); err != nil {
		t.Fatal(err)
	}
	later(25 * time.Hour)
	e.RunDue()
	if strings.Join(started, "|") != "coverage api coverage by coverage" {
		t.Fatalf("started %q: one at a time", started)
	}
	queued, _ := Tasks(e.Root, TaskQueued, 0)
	if len(queued) != 1 || queued[0].Repo != "web" {
		t.Errorf("queued %+v", queued)
	}
	end := func(repo, status string) {
		emit(t, e.Root, events.Event{Kind: events.RunEnded, Repo: repo, Run: "coverage.ground." + repo, RunN: 3, Status: status, By: events.ByWorkflow("coverage")})
	}
	end("api", "passed")
	e.RunDue()
	if len(started) != 2 || len(got.notified) != 0 {
		t.Fatalf("after api: started %q, notified %q", started, got.notified)
	}
	end("web", "passed")
	e.RunDue()
	e.RunDue()
	if strings.Join(got.notified, "|") != "coverage: api 71.4 |coverage: web 55 " {
		t.Errorf("notified %q", got.notified)
	}
	if len(started) != 2 {
		t.Errorf("asked again: %q", started)
	}
	if runs, _ := Runs(e.Root, "coverage", 1); runs[0].State != Ran {
		t.Errorf("run %+v", runs[0])
	}
}

// A task that can't be run, or a repo with no such task, is said: the
// first as what the wait comes to, the second as the run's failure.
func TestTasks_NotRun(t *testing.T) {
	e, got := engine(t)
	later := clock(e)
	e.Hooks.Repos = func() []RepoInfo { return []RepoInfo{{Name: "api", Tasks: []TaskInfo{{Name: "coverage"}}}} }
	e.Hooks.RunTask = func(*Automation, string, string, []string) (string, int, error) {
		return "", 0, fmt.Errorf("its ground isn't there")
	}
	e.Add("one", []byte(strings.Replace(coverageAll, `[r for r in repos() if "coverage" in r.tasks]`, `repos(name = "api")`, 1)))
	later(25 * time.Hour)
	e.RunDue()
	e.RunDue()
	if strings.Join(got.notified, "|") != "one: no coverage " {
		t.Errorf("notified %q", got.notified)
	}
	tasks, _ := Tasks(e.Root, TaskEnded, 0)
	if len(tasks) != 1 || tasks[0].Status != "not run: its ground isn't there" {
		t.Errorf("tasks %+v", tasks)
	}

	e.Add("bad", []byte(`when = every("day 03:00")
may = ["run"]

def run(e):
    run_task("lint", repo = "api")
`))
	later(25 * time.Hour)
	e.RunDue()
	if runs, _ := Runs(e.Root, "bad", 1); len(runs) != 1 || !strings.Contains(runs[0].Error, "api has no task lint: it has coverage") {
		t.Errorf("runs %+v", runs)
	}
}

// Without may run, it can't ask for one.
func TestTasks_MayRun(t *testing.T) {
	e, _ := engine(t)
	later := clock(e)
	e.Add("sneaky", []byte(`when = every("day 03:00")

def run(e):
    run_task("coverage", repo = "api")
`))
	later(25 * time.Hour)
	e.RunDue()
	if runs, _ := Runs(e.Root, "sneaky", 1); len(runs) != 1 || !strings.Contains(runs[0].Error, "may not") {
		t.Errorf("runs %+v", runs)
	}
}

// A dry run asks for nothing: it says what it'd run, and a wait on it is
// given unknown.
func TestTasks_DryRun(t *testing.T) {
	e, _ := engine(t)
	e.Hooks.Repos = func() []RepoInfo { return []RepoInfo{{Name: "api", Tasks: []TaskInfo{{Name: "coverage"}}}} }
	a, err := Check("coverage", []byte(coverageAll))
	if err != nil {
		t.Fatal(err)
	}
	dr, err := e.DryRun(a, 3*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Ran == 0 {
		t.Fatalf("%+v", dr)
	}
	l := dr.Lines[0]
	if strings.Join(l.Did, "|") != "run coverage on api|notify: no coverage" || len(l.Waited) != 1 || !strings.Contains(l.Waited[0], "all of coverage on api") {
		t.Errorf("line %+v", l)
	}
	if tasks, _ := Tasks(e.Root, "", 0); len(tasks) != 0 {
		t.Errorf("a dry run asked for %+v", tasks)
	}
}

const stuck = `# Jira issues in review or blocked with no movement for 5 days, each weekday morning.
when = every("weekday 09:00")
may = ["output", "notify"]

def run(e):
    issues = things("jira:issue", status = ["In Review", "Blocked"], unchanged_for = "5d")
    rows = ["| %s | %s | %s | %d days |" % (t.ref, t.title, t.fields.status, int(t.idle_days)) for t in issues]
    output("| Issue | Title | Status | Still for |\n|---|---|---|---|\n" + "\n".join(rows))
    if issues:
        notify("%d issues haven't moved in 5 days" % len(issues))
`

// things() is a plugin's things, as ws keeps them: those whose fields are
// as asked, and that haven't changed in so long.
func TestThings_Stuck(t *testing.T) {
	e, got := engine(t)
	later := clock(e)
	e.Hooks.Things = func(kind string) ([]ThingInfo, error) {
		now := e.now()
		if kind != "jira:issue" {
			return nil, nil
		}
		issue := func(ref, status string, idle time.Duration) ThingInfo {
			return ThingInfo{Ref: ref, Kind: kind, Title: "Issue " + ref, Changed: now.Add(-idle), Fields: map[string]string{"status": status}}
		}
		return []ThingInfo{
			issue("RAM-1", "In Review", 9*24*time.Hour),
			issue("RAM-2", "Blocked", 6*24*time.Hour),
			issue("RAM-3", "In Review", 2*24*time.Hour), // moved lately
			issue("RAM-4", "Done", 30*24*time.Hour),     // not a status asked for
		}, nil
	}
	if _, err := e.Add("stuck", []byte(stuck)); err != nil {
		t.Fatal(err)
	}
	later(4 * 24 * time.Hour) // a weekday morning's in there
	e.RunDue()
	if len(got.notified) != 1 || got.notified[0] != "stuck: 2 issues haven't moved in 5 days " {
		t.Fatalf("notified %q", got.notified)
	}
	shown, _, _ := Outputs(e.Root, "stuck", 0)
	if !strings.Contains(shown.Body, "| RAM-1 | Issue RAM-1 | In Review | 9 days |") || !strings.Contains(shown.Body, "RAM-2") || strings.Contains(shown.Body, "RAM-3") {
		t.Errorf("output %q", shown.Body)
	}
}
