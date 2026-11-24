package ops

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The stamp moves when what a view's built from is written (a mission, a
// capsule's HEAD in its repo's bare clone, GitHub's answers), and not for
// what's written all the time and shown nowhere (crew's output).
func TestStamp(t *testing.T) {
	w, _ := fixture(t, "")
	at := time.Now().Add(-time.Hour)
	touch := func(path, text string) {
		t.Helper()
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		at = at.Add(time.Second)
		os.Chtimes(path, at, at)
	}
	head := filepath.Join(w.WS().BareDir("frontend"), "worktrees", "one", "HEAD")
	touch(head, "ref: refs/heads/one\n")
	before := w.Stamp()
	if again := w.Stamp(); again != before {
		t.Fatal("the stamp moved with nothing written")
	}
	touch(filepath.Join(w.Missions.Dir(), "cache", "crew-c1.log"), "working…")
	if w.Stamp() != before {
		t.Error("crew's output moved the stamp")
	}
	touch(filepath.Join(w.Root(), ".ws", "ws.db-wal"), "a write to the records")
	if w.Stamp() != before {
		t.Error("the records' file moved the stamp")
	}
	touch(filepath.Join(w.Root(), ".ws", "cache", "nav", "ab", "cd.gob.gz"), "facts")
	if w.Stamp() != before {
		t.Error("code nav's cache moved the stamp")
	}
	touch(filepath.Join(w.Root(), ".ws", "runs", "frontend", "one", "dev", "out.log"), "listening on :3000")
	if w.Stamp() != before {
		t.Error("a run's output moved the stamp")
	}
	touch(filepath.Join(w.Missions.Dir(), "cache", "brief.md"), "## Goal\nCache it.\n")
	moved := w.Stamp()
	if moved == before {
		t.Error("writing the brief didn't move the stamp")
	}
	touch(head, "ref: refs/heads/other\n")
	if w.Stamp() == moved {
		t.Error("a capsule's HEAD moving didn't move the stamp")
	}
	// An artifact's version is written once, before the record that names
	// it: the record moves the stamp, and the version needn't.
	moved = w.Stamp()
	glow := filepath.Join(w.Root(), ".ws", "artifacts", "2026-11-12-glow")
	touch(filepath.Join(glow, "v1.html"), "<!doctype html>")
	touch(filepath.Join(w.Root(), ".ws", "artifacts", ".discarded", "2026-11-01-old", "artifact.toml"), "title = \"Old\"")
	if w.Stamp() != moved {
		t.Error("an artifact's version, or a discarded one, moved the stamp")
	}
	touch(filepath.Join(glow, "artifact.toml"), "title = \"Glow\"")
	if w.Stamp() == moved {
		t.Error("an artifact's record didn't move the stamp")
	}
}
