package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/runs"
)

// hooked has this test's tasks keep their passes, and run what they're run
// after, through this test binary (TestMain).
func hooked(t *testing.T) {
	t.Helper()
	keep, after := keepWith, afterWith
	t.Cleanup(func() { keepWith, afterWith = keep, after })
	testHooks()
	t.Setenv(keepEnv, "1")
}

func gitAt(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// A task whose recipe names its inputs is run once for them: asked for
// again, here or in another capsule, with the same inputs, it's that pass,
// its outputs put back.
func TestStartRun_ACachedTaskIsReusedForTheSameInputs(t *testing.T) {
	w, front := needsFixture(t)
	hooked(t)

	write(t, filepath.Join(front, "ws.repo.toml"), `
[run.task.build]
command = "mkdir -p dist && cat src/a.txt > dist/out.txt && echo built"
inputs = ["src"]
outputs = ["dist"]

[run.task.needy]
command = "true"
inputs = ["src"]
needs.db = {}

[run.service.db]
command = "sleep 300"
`)
	write(t, filepath.Join(front, ".gitignore"), "dist/\n")
	write(t, filepath.Join(front, "src", "a.txt"), "one\n")
	gitAt(t, front, "init", "-q")
	gitAt(t, front, "add", "-A")
	gitAt(t, front, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "first")

	ran := func(dir, worktree string, fresh bool) runs.Run {
		t.Helper()
		a := runAsk(dir, "build")
		a.Worktree, a.Fresh = worktree, fresh
		up, err := w.StartRun(a)
		if err != nil {
			t.Fatal(err)
		}
		r, ended, err := w.runStore().WaitEnd(up.Run, 10*time.Second, nil)
		if err != nil || !ended {
			t.Fatalf("it didn't end: %v", err)
		}
		if !r.Passed() {
			t.Fatalf("%s: %s", r.NRef(), r.Result())
		}
		return r
	}

	first := ran(front, "cache", false)
	if first.Reused != nil || first.Kept == "" || first.KeepFailed != "" {
		t.Fatalf("the first run: reused %+v, kept %q, failed %q", first.Reused, first.Kept, first.KeepFailed)
	}
	again := ran(front, "cache", false)
	if again.Reused == nil || again.Reused.From != "build.cache.frontend#1" || again.N != 2 || again.Tail(1) != "built" {
		t.Fatalf("asked again: reused %+v, #%d, %q", again.Reused, again.N, again.Tail(1))
	}

	// Another capsule with the same files: the pass is reused there, its
	// outputs put back where it's asked for.
	other := filepath.Join(filepath.Dir(front), "other")
	gitAt(t, front, "worktree", "add", "-q", other)
	there := ran(other, "other", false)
	if there.Reused == nil || there.Reused.From != "build.cache.frontend#1" {
		t.Fatalf("in another capsule: reused %+v", there.Reused)
	}
	if b, err := os.ReadFile(filepath.Join(other, "dist", "out.txt")); err != nil || string(b) != "one\n" {
		t.Errorf("its output there = %q, %v", b, err)
	}
	if ReusedSaid(there) != "reused from build.cache.frontend#1, which took "+runs.Short(there.Reused.Took) {
		t.Errorf("said %q", ReusedSaid(there))
	}

	// The same files, but that capsule's ws.repo.toml says the command
	// otherwise: it's run there.
	toml, _ := os.ReadFile(filepath.Join(other, "ws.repo.toml"))
	write(t, filepath.Join(other, "ws.repo.toml"), strings.Replace(string(toml), "&& echo built", "&& echo built there", 1))
	if r := ran(other, "other", false); r.Reused != nil || r.Tail(1) != "built there" {
		t.Errorf("with its command changed there: reused %+v, %q", r.Reused, r.Tail(1))
	}

	// An input changed, or --fresh: it's run.
	write(t, filepath.Join(front, "src", "a.txt"), "two\n")
	if r := ran(front, "cache", false); r.Reused != nil {
		t.Errorf("with src/a.txt changed it was reused from %s", r.Reused.From)
	}
	if r := ran(front, "cache", true); r.Reused != nil {
		t.Errorf("--fresh was reused from %s", r.Reused.From)
	}

	// One that needs a service isn't cached, and says so.
	up, err := w.StartRun(runAsk(front, "needy"))
	if err != nil {
		t.Fatal(err)
	}
	if said := strings.Join(up.Notes, " "); !strings.Contains(said, "needy needs db up") || up.Run.Key != "" {
		t.Errorf("needy: notes %q, key %q", said, up.Run.Key)
	}
}

// A task run after another has it run first, or reused, and is run only
// once it has passed. Where both are cached, the first's key is in the
// second's, and what the first writes isn't read as the second's input.
func TestStartRun_ATaskIsRunAfterTheTasksItNames(t *testing.T) {
	w, front := needsFixture(t)
	hooked(t)
	write(t, filepath.Join(w.WS().Root, "ws.toml"), "[workspace]\norg = \"acme\"\n\n[repos.frontend]\n[repos.backend]\n")
	write(t, filepath.Join(front, "ws.repo.toml"), `
[run.task.build]
command = "test ! -f src/broken && mkdir -p dist && cat src/a.txt > dist/out.txt && echo built"
inputs = ["src"]
outputs = ["dist"]

[run.task.cover]
command = "cat dist/out.txt test/t.txt > cover.txt && echo covered"
inputs = ["test", "dist"]
outputs = ["cover.txt"]
after = ["build"]

[run.task.loop-a]
command = "true"
after = ["loop-b"]

[run.task.loop-b]
command = "true"
after = ["loop-a"]

[run.task.on-service]
command = "true"
after = ["db"]

[run.service.db]
command = "sleep 300"
`)
	write(t, filepath.Join(front, ".gitignore"), "dist/\ncover.txt\n")
	write(t, filepath.Join(front, "src", "a.txt"), "one\n")
	write(t, filepath.Join(front, "test", "t.txt"), "t\n")
	gitAt(t, front, "init", "-q")
	gitAt(t, front, "add", "-A")
	gitAt(t, front, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "first")

	cover := func(fresh bool) (RunUp, runs.Run) {
		t.Helper()
		a := runAsk(front, "cover")
		a.Fresh = fresh
		up, err := w.StartRun(a)
		if err != nil {
			t.Fatal(err)
		}
		r, ended, err := w.runStore().WaitEnd(up.Run, 20*time.Second, nil)
		if err != nil || !ended {
			t.Fatalf("cover didn't end: %v", err)
		}
		return up, r
	}

	_, first := cover(false)
	if !first.Passed() || first.Reused != nil {
		t.Fatalf("cover: %s, reused %+v\n%s", first.Result(), first.Reused, first.Tail(20))
	}
	if log := first.Tail(20); !strings.Contains(log, "ws: build first (build.cache.frontend#1)") || !strings.Contains(log, "built") || !strings.HasSuffix(log, "covered") {
		t.Errorf("cover's log, build's then its own:\n%s", log)
	}
	if strings.Join(first.AfterRan, " ") != "build.cache.frontend#1" || len(first.Undeclared) != 0 {
		t.Errorf("ran after %v; undeclared %v: dist is build's", first.AfterRan, first.Undeclared)
	}
	if b, _ := os.ReadFile(filepath.Join(front, "cover.txt")); string(b) != "one\nt\n" {
		t.Errorf("cover.txt = %q", b)
	}

	// Again, with dist gone: cover is reused, and build too, so dist is
	// put back.
	os.RemoveAll(filepath.Join(front, "dist"))
	up, again := cover(false)
	if again.Reused == nil || again.Reused.From != "cover.cache.frontend#1" {
		t.Fatalf("cover again: reused %+v, notes %v", again.Reused, up.Notes)
	}
	if b, err := os.ReadFile(filepath.Join(front, "dist", "out.txt")); err != nil || string(b) != "one\n" {
		t.Errorf("dist/out.txt = %q, %v: build's pass is put back with cover's", b, err)
	}
	if strings.Join(again.AfterRan, " ") != "build.cache.frontend#2" {
		t.Errorf("reused, it ran after %v", again.AfterRan)
	}

	// build's input changed: both are run again, as build's key is in
	// cover's.
	write(t, filepath.Join(front, "src", "a.txt"), "two\n")
	_, third := cover(false)
	if third.Reused != nil || !strings.Contains(third.Tail(20), "built") {
		t.Errorf("with build's input changed: reused %+v\n%s", third.Reused, third.Tail(20))
	}
	if b, _ := os.ReadFile(filepath.Join(front, "cover.txt")); string(b) != "two\nt\n" {
		t.Errorf("cover.txt = %q", b)
	}

	// build fails: cover fails as it did, and its command isn't run.
	write(t, filepath.Join(front, "src", "broken"), "x")
	_, failed := cover(false)
	if failed.Passed() || strings.Contains(failed.Tail(20), "covered") || !strings.Contains(failed.Tail(20), "cover isn't run") {
		t.Errorf("with build failing: %s\n%s", failed.Result(), failed.Tail(20))
	}

	for recipe, want := range map[string]string{
		"loop-a":     "loop-a comes back round: loop-a, which is run after loop-b, which is run after loop-a",
		"on-service": "on-service is run after db, which is a service",
	} {
		if _, err := w.StartRun(runAsk(front, recipe)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", recipe, err, want)
		}
	}
}
