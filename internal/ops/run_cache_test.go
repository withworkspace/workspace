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
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	was := keepWith
	keepWith = func() []string { return []string{exe} }
	t.Cleanup(func() { keepWith = was })
	t.Setenv(keepEnv, "1")

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
