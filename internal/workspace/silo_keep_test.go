package workspace

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

func TestFullSync_LeavesWhatsTheSame(t *testing.T) {
	src := initGitRepo(t)
	dst := t.TempDir()
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(src, "b.txt"), []byte("b"), 0644)

	if n, err := FullSync(src, dst); err != nil || n != 2 {
		t.Fatalf("first sync = %d, %v; want 2 files changed", n, err)
	}
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(dst, "a.txt"), old, old) // as if a.txt was synced an hour ago
	os.Chtimes(filepath.Join(src, "a.txt"), old, old)
	if n, err := FullSync(src, dst); err != nil || n != 0 {
		t.Fatalf("sync of what's the same = %d, %v; want 0", n, err)
	}
	if info, _ := os.Stat(filepath.Join(dst, "a.txt")); !info.ModTime().Equal(old) {
		t.Errorf("a.txt was written again by a sync that had nothing to do")
	}

	// Touched but the same: compared, not written.
	now := time.Now()
	os.Chtimes(filepath.Join(src, "a.txt"), now, now)
	if n, _ := FullSync(src, dst); n != 0 {
		t.Errorf("a touched file the same as the silo's counts as changed: %d", n)
	}

	os.WriteFile(filepath.Join(src, "b.txt"), []byte("bb"), 0644)
	os.Remove(filepath.Join(src, "a.txt"))
	if n, err := FullSync(src, dst); err != nil || n != 2 {
		t.Fatalf("sync of a change and a removal = %d, %v; want 2", n, err)
	}
	if got, _ := os.ReadFile(filepath.Join(dst, "b.txt")); string(got) != "bb" {
		t.Errorf("b.txt = %q, want bb", got)
	}
	if _, err := os.Stat(filepath.Join(dst, "a.txt")); !os.IsNotExist(err) {
		t.Errorf("a.txt is still in the silo after it went from the capsule")
	}
}

// siloFixture is a workspace with a repo r whose silo is kept: capsules
// one and two, .ground holding ws.repo.toml's hooks, and .silo.
type siloFixture struct {
	root, repo string
	log        bytes.Buffer
}

func newSiloFixture(t *testing.T, repoToml string) *siloFixture {
	t.Helper()
	f := &siloFixture{root: t.TempDir()}
	f.repo = filepath.Join(f.root, "repos", "r")
	for _, c := range []string{"one", "two", GroundDir} {
		dir := filepath.Join(f.repo, c)
		os.MkdirAll(dir, 0755)
		git.Run(dir, "init", "-q")
		os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("build/\n"), 0644)
		os.WriteFile(filepath.Join(dir, "main.txt"), []byte("in "+c), 0644)
		os.MkdirAll(filepath.Join(dir, "src", "deep"), 0755)
		os.WriteFile(filepath.Join(dir, "src", "deep", "x.txt"), []byte("x"), 0644)
		os.MkdirAll(filepath.Join(dir, "build", "out"), 0755)
		os.WriteFile(filepath.Join(dir, "build", "out", "big.bin"), []byte("ignored"), 0644)
	}
	if repoToml != "" {
		os.WriteFile(filepath.Join(f.repo, GroundDir, "ws.repo.toml"), []byte(repoToml), 0644)
	}
	os.MkdirAll(filepath.Join(f.repo, SiloDir), 0755)
	os.WriteFile(filepath.Join(f.root, "ws.local.toml"), []byte("[silo]\nr = \"one\"\n"), 0644)
	return f
}

func (f *siloFixture) start(t *testing.T) *SiloWatcher {
	t.Helper()
	w := &Workspace{Root: f.root}
	sw, err := NewSiloWatcher(w, log.New(&f.log, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	sw.RepoDir = func(string) string { return f.repo }
	sw.SiloWorktree = func(string) string { return filepath.Join(f.repo, SiloDir) }
	sw.MainWorktree = func(string) string { return filepath.Join(f.repo, GroundDir) }
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		sw.Watch(stop, map[string]string{"r": "one"})
		close(done)
	}()
	t.Cleanup(func() { close(stop); <-done })
	return sw
}

func (f *siloFixture) silo(name string) string { return filepath.Join(f.repo, SiloDir, name) }

// eventually waits for ok, failing with what the watcher said if it never is.
func (f *siloFixture) eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: never happened. The watcher said:\n%s", what, f.log.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func readString(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

func TestSiloWatcher_KeepsASilo(t *testing.T) {
	f := newSiloFixture(t, "[silo]\nafter_switch = \"echo switched >> hooks.log\"\nafter_change = \"echo changed >> hooks.log\"\n")
	sw := f.start(t)

	// As it starts: the silo's given the capsule's files, marked, and its
	// switch hook run, as it had no capsule's files before.
	f.eventually(t, "the first sync", func() bool { return readString(f.silo("main.txt")) == "in one" })
	f.eventually(t, "the switch hook", func() bool { return strings.Contains(readString(f.silo("hooks.log")), "switched") })
	if got := SiloTarget(filepath.Join(f.repo, SiloDir)); got != "one" {
		t.Errorf("silo marked %q, want one", got)
	}
	if _, err := os.Stat(f.silo("build")); !os.IsNotExist(err) {
		t.Errorf("an ignored folder was synced")
	}
	st := sw.States()
	if len(st) != 1 || !st[0].Watching || st[0].Target != "one" {
		t.Fatalf("states = %+v, want r watching one", st)
	}
	// ., src and src/deep: not build or build/out, which are ignored.
	if st[0].Dirs != 3 {
		t.Errorf("watching %d folders, want 3 (none ignored)", st[0].Dirs)
	}

	// An edit reaches it, and the change hook runs.
	os.WriteFile(filepath.Join(f.repo, "one", "src", "deep", "x.txt"), []byte("edited"), 0644)
	f.eventually(t, "the edit", func() bool { return readString(f.silo("src/deep/x.txt")) == "edited" })
	f.eventually(t, "the change hook", func() bool { return strings.Contains(readString(f.silo("hooks.log")), "changed") })

	// A folder made with its files already in it.
	os.MkdirAll(filepath.Join(f.repo, "one", "fresh", "inner"), 0755)
	os.WriteFile(filepath.Join(f.repo, "one", "fresh", "inner", "n.txt"), []byte("new"), 0644)
	f.eventually(t, "a made folder's file", func() bool { return readString(f.silo("fresh/inner/n.txt")) == "new" })

	// Pointed elsewhere by ws.local.toml alone (as ws burn does): synced,
	// marked, and the switch hook run again.
	os.WriteFile(filepath.Join(f.root, "ws.local.toml"), []byte("[silo]\nr = \"two\"\n"), 0644)
	f.eventually(t, "the re-point", func() bool { return readString(f.silo("main.txt")) == "in two" })
	f.eventually(t, "the second switch hook", func() bool { return strings.Count(readString(f.silo("hooks.log")), "switched") == 2 })
	if _, err := os.Stat(f.silo("fresh")); !os.IsNotExist(err) {
		t.Errorf("one's folder stayed in the silo pointed at two")
	}
	f.eventually(t, "the state", func() bool { st := sw.States(); return len(st) == 1 && st[0].Target == "two" })
	if st := sw.States(); st[0].Hook == nil || st[0].Hook.Name != "after_switch" {
		t.Errorf("last hook = %+v, want after_switch", st[0].Hook)
	}
}

func TestSiloWatcher_LeavesAMarkedPointing(t *testing.T) {
	f := newSiloFixture(t, "[silo]\nafter_switch = \"echo switched >> hooks.log\"\n")
	sw := f.start(t)
	f.eventually(t, "the first switch hook", func() bool { return strings.Contains(readString(f.silo("hooks.log")), "switched") })

	// ws silo point's way: synced and marked, then recorded. Its hooks
	// are its own to run, so the daemon runs none.
	if _, err := FullSync(filepath.Join(f.repo, "two"), filepath.Join(f.repo, SiloDir)); err != nil {
		t.Fatal(err)
	}
	MarkSiloTarget(filepath.Join(f.repo, SiloDir), "two")
	os.WriteFile(filepath.Join(f.root, "ws.local.toml"), []byte("[silo]\nr = \"two\"\n"), 0644)
	f.eventually(t, "the re-point", func() bool { st := sw.States(); return len(st) == 1 && st[0].Target == "two" && st[0].Watching })
	time.Sleep(300 * time.Millisecond)
	if n := strings.Count(readString(f.silo("hooks.log")), "switched"); n != 1 {
		t.Errorf("the switch hook ran %d times, want once: ws silo point had run it", n)
	}

	// Let go of when it's no longer named.
	os.WriteFile(filepath.Join(f.root, "ws.local.toml"), []byte(""), 0644)
	f.eventually(t, "letting go", func() bool { return len(sw.States()) == 0 })
}

func TestSiloWatcher_SaysWhyItCantKeep(t *testing.T) {
	f := newSiloFixture(t, "")
	os.RemoveAll(filepath.Join(f.repo, SiloDir))
	sw := f.start(t)
	f.eventually(t, "the state", func() bool { return len(sw.States()) == 1 })
	st := sw.States()[0]
	if st.Watching || !strings.Contains(st.Error, SiloDir) {
		t.Errorf("state = %+v, want not watching, saying there's no .silo", st)
	}
	if err := sw.Resync("r"); err == nil {
		t.Errorf("a resync of a silo that isn't kept says nothing")
	}
}

func TestSiloWatcher_HookFailureKept(t *testing.T) {
	f := newSiloFixture(t, "[silo]\nafter_switch = \"echo boom; exit 3\"\n")
	sw := f.start(t)
	f.eventually(t, "the hook", func() bool { st := sw.States(); return len(st) == 1 && st[0].Hook != nil })
	h := sw.States()[0].Hook
	if h.Error == "" || !strings.Contains(h.Output, "boom") {
		t.Errorf("hook = %+v, want its failure and what it said", h)
	}
}
