package store

import (
	"os"
	"path/filepath"
	"strings"
)

// Disk says why a workspace's records are at risk where they are, or "":
// on a network disk, whose locking a write-ahead log can't rely on and
// whose writes can be lost in a way a local disk's can't; or in a folder
// a sync service copies, which can copy the file halfway through a write.
// ws doctor says so.
func Disk(root string) string {
	if why := syncedBy(root); why != "" {
		return "it's in a folder " + why + " copies, which can copy the records halfway through a write"
	}
	if kind := networkDisk(root); kind != "" {
		return "it's on a network disk (" + kind + "), where SQLite's write-ahead log isn't safe"
	}
	return ""
}

// syncedBy names the sync service whose folder path is in, or "".
func syncedBy(path string) string {
	path, _ = filepath.Abs(path)
	if p, err := filepath.EvalSymlinks(path); err == nil {
		path = p
	}
	home, _ := os.UserHomeDir()
	under := func(dir string) bool {
		return dir != "" && (path == dir || strings.HasPrefix(path, dir+string(filepath.Separator)))
	}
	switch {
	case home != "" && under(filepath.Join(home, "Library", "Mobile Documents")):
		return "iCloud Drive"
	case home != "" && under(filepath.Join(home, "Library", "CloudStorage")):
		return "a cloud storage app (Dropbox, Google Drive, OneDrive…)"
	}
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		switch {
		case part == "Dropbox" || strings.HasPrefix(part, "Dropbox ("):
			return "Dropbox"
		case part == "Google Drive" || part == "My Drive":
			return "Google Drive"
		case part == "OneDrive" || strings.HasPrefix(part, "OneDrive - "):
			return "OneDrive"
		}
	}
	return ""
}
