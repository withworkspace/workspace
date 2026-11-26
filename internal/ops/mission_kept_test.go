package ops

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// In the daemon, a mission's stamp counts the writes in its folder as the
// watch says them, rather than walking it: one in its files moves it, a
// document's version (which its page isn't built from) doesn't.
func TestMissionStamp_CountsItsFolderAsWatched(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "cache")
	stop, done := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(stop); <-done })
	go func() {
		defer close(done)
		w.WatchGit(stop, t.Logf)
	}()
	waitFor(t, "the files watched", w.missions.watching.Load)
	dir := filepath.Join(w.Missions.Dir(), "cache")
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	time.Sleep(300 * time.Millisecond) // the folder made is said: let it be

	was := w.MissionStamp("cache")
	if _, ok := w.missions.mission("cache"); !ok {
		t.Error("its record wasn't kept for its stamp")
	}
	os.WriteFile(filepath.Join(dir, "docs", "v1.md"), []byte("one"), 0o644)
	time.Sleep(300 * time.Millisecond)
	if now := w.MissionStamp("cache"); now != was {
		t.Error("a document's version moved its stamp")
	}
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("hi"), 0o644)
	waitFor(t, "a file in its folder moves its stamp", func() bool { return w.MissionStamp("cache") != was })
}
