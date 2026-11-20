package router

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/runs"
)

// NameOf is the name root goes by on the router, once its daemon has
// written its route file: what it asked for, or with a clash, what it was
// given (Names).
func NameOf(root string) (string, bool) {
	namesMu.Lock()
	defer namesMu.Unlock()
	if time.Since(namesAt) > 2*time.Second {
		files, _ := Read(Dir(), runs.Procs{}) // whose daemons are up is the router's to say
		names, namesAt = Names(files), time.Now()
	}
	n, ok := names[filepath.Clean(root)]
	if !ok {
		n, ok = names[root]
	}
	return n, ok
}

var (
	namesMu sync.Mutex
	names   map[string]string
	namesAt time.Time
)

// Address is where key (a run's, or "" for the web UI) of root is reached
// by name, when root has names and a router is serving them.
func Address(root, key string) (string, bool) {
	name, ok := NameOf(root)
	if !ok {
		return "", false
	}
	port := Serving()
	if port == 0 {
		return "", false
	}
	return Link(Host(key, name), port), true
}

// Published is whether the router answers for a run of root's by its name
// now: its route (key, at port) is in root's route file, and has been for
// long enough that the router has read it. A run is up on its port before
// its daemon has written its route (every couple of seconds), and a page
// that sends you to its name before then finds nothing there.
func Published(root, key string, port int) bool {
	path := filepath.Join(Dir(), fileName(filepath.Clean(root)))
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	f, err := read(path)
	if err != nil {
		return false
	}
	return f.Routes[key].Port == port && time.Since(fi.ModTime()) > readEvery
}

// RunAddress is where a run is reached: by its name, when it has one, or
// at 127.0.0.1 and its port.
func RunAddress(root string, r runs.Run) string {
	if a, ok := Address(root, RunKey(r.Name, r.Worktree, r.Repo)); ok {
		return a
	}
	return r.Link()
}
