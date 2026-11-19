package router

import (
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

// RunAddress is where a run is reached: by its name, when it has one, or
// at 127.0.0.1 and its port.
func RunAddress(root string, r runs.Run) string {
	if a, ok := Address(root, RunKey(r.Name, r.Worktree, r.Repo)); ok {
		return a
	}
	return r.Link()
}
