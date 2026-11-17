package doc

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The workspace's versions are kept in .ws/docs/versions, a mission's with
// the mission; one kept where the workspace's used to be is still read,
// and moved on the server's start.
func TestVersions(t *testing.T) {
	root := t.TempDir()
	ws, m := Versions{Root: root}, Versions{Root: root, Scope: OfMission("cache")}
	v, err := ws.Keep([]byte("# Plan\n"))
	if err != nil || v != Version([]byte("# Plan\n")) {
		t.Fatalf("keep: %q, %v", v, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".ws", "docs", "versions", v+".md")); err != nil {
		t.Errorf("not in .ws/docs/versions: %v", err)
	}
	if _, err := m.Text(v); !errors.Is(err, ErrNotFound) {
		t.Errorf("a mission has the workspace's version: %v", err)
	}
	if _, err := ws.Text("../../etc"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a version that's a path: %v", err)
	}

}
