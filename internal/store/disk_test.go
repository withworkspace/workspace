package store

import (
	"path/filepath"
	"testing"
)

// A workspace in a folder a sync service copies is said to be at risk;
// one on this machine's own disk isn't.
func TestDisk(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for path, want := range map[string]string{
		filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs", "ws"): "iCloud Drive",
		filepath.Join(home, "Library", "CloudStorage", "Dropbox", "ws"):                 "a cloud storage app (Dropbox, Google Drive, OneDrive…)",
		filepath.Join(home, "Dropbox (Personal)", "work", "ws"):                         "Dropbox",
		filepath.Join(home, "OneDrive - Acme", "ws"):                                    "OneDrive",
		filepath.Join(home, "Projects", "ws"):                                           "",
		filepath.Join(home, "Projects", "dropbox-sync", "ws"):                           "",
	} {
		if got := syncedBy(path); got != want {
			t.Errorf("syncedBy(%s) = %q, want %q", path, got, want)
		}
	}
	if got := Disk(t.TempDir()); got != "" {
		t.Errorf("a temporary folder: %q", got)
	}
}
