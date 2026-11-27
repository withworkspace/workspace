package runs

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// keepEnv, set in a run's environment, has this test binary keep a pass
// when the wrapper calls it, as ws run cache keep would (TestMain).
const keepEnv = "WS_RUNS_TEST_KEEP"

func TestPatternsMatchAsTheySay(t *testing.T) {
	for _, c := range []struct {
		p, name string
		want    bool
	}{
		{"src", "src/a/b.ts", true},
		{"src", "src", true},
		{"src", "srcs/a.ts", false},
		{"package.json", "package.json", true},
		{"src/**", "src/a/b.ts", true},
		{"src/**/*.ts", "src/b.ts", true},
		{"src/**/*.ts", "src/a/b/c.ts", true},
		{"src/**/*.ts", "src/a/b/c.js", false},
		{"**/*.go", "main.go", true},
		{"**/*.go", "cmd/x/main.go", true},
		{"*.out", "cover.out", true},
		{"*.out", "sub/cover.out", false},
		{"dist/**", "dist", false},
		{"dist/**", "dist/x/y.js", true},
	} {
		if got := Matches(c.p, c.name); got != c.want {
			t.Errorf("Matches(%q, %q) = %v, want %v", c.p, c.name, got, c.want)
		}
	}
	for _, bad := range []string{"", "/etc/passwd", "../up", "a/../../b"} {
		if _, err := cleanPattern(bad); err == nil {
			t.Errorf("%q was taken", bad)
		}
	}
}

func TestFilesWalksOnlyWhatAPatternCouldName(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"dist/a.js", "dist/x/b.js", "cover.out", "node_modules/m/cover.out", "src/c.ts"} {
		writeFile(t, filepath.Join(dir, f), f)
	}
	got, err := Files(dir, []string{"dist", "*.out", "**/*.out"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"cover.out", "dist/a.js", "dist/x/b.js"}; !slices.Equal(got, want) {
		t.Errorf("files = %v, want %v: node_modules isn't walked unless it's named", got, want)
	}
	if got, _ := Files(dir, []string{"node_modules/**/*.out"}); !slices.Equal(got, []string{"node_modules/m/cover.out"}) {
		t.Errorf("named, node_modules is walked: %v", got)
	}
}

// gitRepo makes a repo with files committed, and gives where it is.
func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	for f, body := range files {
		writeFile(t, filepath.Join(dir, f), body)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "commit", "-qm", "first")
	return dir
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAKeyChangesWithWhatItsInputsNameAndNothingElse(t *testing.T) {
	dir := gitRepo(t, map[string]string{
		".gitignore": "dist/\nnode_modules/\n", "src/a.ts": "a", "src/b.ts": "b", "README.md": "hi",
	})
	of := KeyOf{Repo: "demo", Command: "pnpm test", Dir: dir, Inputs: []string{"src/**", "dist"}, Outputs: []string{"coverage"}}
	key := func() string {
		t.Helper()
		k, err := Key(of)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	first := key()
	if first == "" || key() != first {
		t.Fatal("the same files, a different key")
	}
	writeFile(t, filepath.Join(dir, "README.md"), "changed")
	writeFile(t, filepath.Join(dir, "node_modules/x/i.js"), "installed")
	if key() != first {
		t.Error("a file no input names changed the key")
	}
	steps := []struct {
		what string
		do   func()
	}{
		{"a changed file, not committed", func() { writeFile(t, filepath.Join(dir, "src/a.ts"), "a2") }},
		{"a new file", func() { writeFile(t, filepath.Join(dir, "src/c.ts"), "c") }},
		{"an ignored file an input names", func() { writeFile(t, filepath.Join(dir, "dist/out.js"), "built") }},
		{"an ignored file changed", func() { writeFile(t, filepath.Join(dir, "dist/out.js"), "built again") }},
		{"a file gone", func() { os.Remove(filepath.Join(dir, "src/b.ts")) }},
		{"the command", func() { of.Command = "pnpm test --ci" }},
		{"the inputs as written, naming the same files", func() { of.Inputs = []string{"src", "dist/**"} }},
		{"a variable", func() { of.Env = map[string]string{"NODE_ENV": "test"} }},
		{"the outputs", func() { of.Outputs = []string{"coverage", "junit.xml"} }},
	}
	seen := map[string]string{first: "the first"}
	for _, s := range steps {
		s.do()
		k := key()
		if was, ok := seen[k]; ok {
			t.Errorf("%s: the key is %s's", s.what, was)
		}
		seen[k] = s.what
	}

	// The same files in another worktree of the repo: the same key.
	other := filepath.Join(t.TempDir(), "other")
	git(t, dir, "add", "-A")
	git(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "commit", "-qm", "second")
	git(t, dir, "worktree", "add", "-q", other)
	writeFile(t, filepath.Join(other, "dist/out.js"), "built again")
	here := key()
	of.Dir = other
	if key() != here {
		t.Error("the same files in another worktree, another key")
	}

	if _, err := Key(KeyOf{Dir: t.TempDir(), Inputs: []string{"src"}}); err == nil || !strings.Contains(err.Error(), "git") {
		t.Errorf("outside git: %v", err)
	}
}

// keepCmd is this test binary, as the command a wrapper keeps a pass with.
func keepCmd(t *testing.T) []string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return []string{exe}
}

// cachedTask starts a cached task in dir, whose pass the wrapper keeps
// through this test binary.
func cachedTask(t *testing.T, s Store, dir, command, key string, outputs []string) Run {
	t.Helper()
	r, err := s.Start(Start{
		Name: "build", Type: Task, Repo: "demo", Worktree: "feat", Dir: dir, Command: command, Recipe: "build",
		Owner: Owner{Kind: ByNone}, TTL: time.Hour, By: "you", Key: key, Outputs: outputs,
		Keep: keepCmd(t), Env: append(os.Environ(), keepEnv+"=1"),
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAPassIsKeptAndReusedWithItsOutputsPutBack(t *testing.T) {
	s, _ := setup(t)
	dir := gitRepo(t, map[string]string{".gitignore": "dist/\n", "src/a.ts": "a"})
	r := ended(t, s, cachedTask(t, s, dir,
		"mkdir -p dist/x && echo built > dist/x/a.js && echo note > stray.txt && echo building", "k1", []string{"dist"}))
	if !r.Passed() || r.Kept != "k1" || r.KeepFailed != "" {
		t.Fatalf("%s, kept %q, failed %q", r.Result(), r.Kept, r.KeepFailed)
	}
	if !slices.Equal(r.Undeclared, []string{"stray.txt"}) {
		t.Errorf("undeclared = %v: stray.txt was written outside its outputs", r.Undeclared)
	}
	e, ok := s.Lookup("k1")
	if !ok || e.From != "build.feat.demo#1" || e.Files != 1 || e.Recipe != "build" {
		t.Fatalf("entry = %+v, %v", e, ok)
	}

	// The outputs as they are now aren't the pass's: they're put back.
	os.RemoveAll(filepath.Join(dir, "dist"))
	writeFile(t, filepath.Join(dir, "dist/stale.js"), "old")
	again, started, err := s.Reuse(Start{
		Name: "build", Type: Task, Repo: "demo", Worktree: "feat", Dir: dir, Command: "x", Recipe: "build",
		Owner: Owner{Kind: ByNone}, Key: "k1", Outputs: []string{"dist"},
	}, e, time.Now())
	if err != nil || !started {
		t.Fatalf("reuse: %v, %v", started, err)
	}
	if !again.Passed() || again.N != 2 || again.Reused == nil || again.Reused.From != "build.feat.demo#1" {
		t.Fatalf("reused run = %s #%d, %+v", again.Result(), again.N, again.Reused)
	}
	if again.Tail(1) != "building" {
		t.Errorf("its output = %q: the pass's", again.Tail(1))
	}
	if b, err := os.ReadFile(filepath.Join(dir, "dist/x/a.js")); err != nil || string(b) != "built\n" {
		t.Errorf("dist/x/a.js = %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "dist/stale.js")); err == nil {
		t.Error("dist/stale.js is still there: what the outputs named is removed first")
	}
	if got, err := s.Find("build#1", "demo", "feat"); err != nil || got.Reused != nil {
		t.Errorf("build#1, the run that passed, is behind it: %v", err)
	}
}

func TestAFailIsNotKept(t *testing.T) {
	s, _ := setup(t)
	dir := gitRepo(t, map[string]string{"src/a.ts": "a"})
	r := ended(t, s, cachedTask(t, s, dir, "echo nope; exit 1", "k2", nil))
	if r.Passed() || r.Kept != "" {
		t.Fatalf("%s, kept %q", r.Result(), r.Kept)
	}
	if _, ok := s.Lookup("k2"); ok {
		t.Error("a fail was kept")
	}
}

func TestTheCacheDropsWhatWasLeastLatelyUsed(t *testing.T) {
	s, _ := setup(t)
	defer func(was int64) { CacheMax = was }(CacheMax)
	for i, k := range []string{"old", "mid", "new"} {
		dir := filepath.Join(s.CacheDir(), k)
		writeFile(t, filepath.Join(dir, entryFile), `{"key":"`+k+`","bytes":100}`)
		at := time.Now().Add(time.Duration(i-3) * time.Hour)
		os.Chtimes(filepath.Join(dir, entryFile), at, at)
	}
	CacheMax = 250
	s.evict("new")
	var left []string
	for _, e := range s.Cached() {
		left = append(left, e.Key)
	}
	if !slices.Equal(left, []string{"new", "mid"}) {
		t.Errorf("left %v: the oldest goes first", left)
	}
}

// Two runs with the same inputs passing at once: each is kept, the later
// in place of the earlier, and neither fails for the other.
func TestTwoPassesOfOneKeyAtOnceAreBothKept(t *testing.T) {
	s, _ := setup(t)
	dir := gitRepo(t, map[string]string{".gitignore": "dist/\n", "src/a.ts": "a"})
	a := cachedTask(t, s, dir, "mkdir -p dist && echo a > dist/a.js", "same", []string{"dist"})
	b, err := s.Start(Start{
		Name: "build-b", Type: Task, Repo: "demo", Worktree: "feat", Dir: dir, Command: "mkdir -p dist && echo a > dist/a.js",
		Owner: Owner{Kind: ByNone}, TTL: time.Hour, Key: "same", Outputs: []string{"dist"},
		Keep: keepCmd(t), Env: append(os.Environ(), keepEnv+"=1"),
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []Run{ended(t, s, a), ended(t, s, b)} {
		if r.Kept != "same" || r.KeepFailed != "" {
			t.Errorf("%s: kept %q, failed %q", r.Name, r.Kept, r.KeepFailed)
		}
	}
	if _, ok := s.Lookup("same"); !ok {
		t.Error("nothing's kept")
	}
	// And straight at it: one in place already.
	tmp := filepath.Join(s.CacheDir(), ".new-x")
	writeFile(t, filepath.Join(tmp, entryFile), "{}")
	if err := putInPlace(tmp, filepath.Join(s.CacheDir(), "same")); err != nil {
		t.Errorf("in place of one there: %v", err)
	}
}

// A task's wrapper runs its after script first, and its command only if
// that passed.
func TestATaskRunsAfterWhatItNamesAndOnlyIfThatPassed(t *testing.T) {
	s, dir := setup(t)
	run := func(after string) Run {
		t.Helper()
		r, err := s.Start(Start{
			Name: "cover", Type: Task, Repo: "demo", Worktree: "feat", Dir: dir, Command: "echo covered",
			Owner: Owner{Kind: ByNone}, TTL: time.Hour, After: []After{{Recipe: "build"}},
			AfterCmd: []string{"/bin/sh", "-c", after, "after"},
		}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return ended(t, s, r)
	}
	if r := run("echo building"); !r.Passed() || r.Tail(5) != "building\ncovered" {
		t.Errorf("after passed: %s, %q", r.Result(), r.Tail(5))
	}
	if r := run("echo broke; exit 4"); r.Status != 4 || r.Tail(5) != "broke" {
		t.Errorf("after failed: %s, %q: its status, and no command", r.Result(), r.Tail(5))
	}
}

func TestAKeyHasItsAftersKeysAndNotTheirOutputs(t *testing.T) {
	dir := gitRepo(t, map[string]string{".gitignore": "dist/\n", "test/t.txt": "t"})
	writeFile(t, filepath.Join(dir, "dist/out.js"), "built")
	of := KeyOf{Command: "cover", Dir: dir, Inputs: []string{"test", "dist"}, After: []After{{Recipe: "build", Key: "b1", Outputs: []string{"dist"}}}}
	k1, err := Key(of)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "dist/out.js"), "stale")
	if k, _ := Key(of); k != k1 {
		t.Error("dist changed the key: it's build's, said by its key")
	}
	of.After[0].Key = "b2"
	if k, _ := Key(of); k == k1 {
		t.Error("build's key changed, and the key didn't")
	}
	of.After[0].Key = ""
	if _, err := Key(of); err == nil || !strings.Contains(err.Error(), "run after build, which isn't cached") {
		t.Errorf("after an uncached task: %v", err)
	}
}
