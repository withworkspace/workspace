// Package router gives workspaces and their runs names under .localhost:
// <workspace>.localhost for its web UI, and
// <run>.<worktree>.<repo>.<workspace>.localhost for a run (internal/runs).
// Browsers send every *.localhost to this machine themselves, and treat it
// as a secure context, with no DNS and no certificate.
//
// One router serves every workspace on the machine, and it's no process
// of its own: each daemon with [router] enabled tries to listen on the
// router's port, and the one the system lets is the router. The others try
// again every RetryEvery, so when it goes, one of them takes over. What
// goes where is files, one a workspace (Dir): each daemon writes its own,
// and the router reads them all, skipping those whose daemon has gone.
// Nothing is said between daemons but a newer router asking an older one
// to step down (yield).
package router

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/runs"
)

// Version is the route files' and the router's: a router reads files up
// to its own version, and steps down for a newer one.
const Version = 1

// Suffix is what every name ends with.
const Suffix = ".localhost"

// File is one workspace's routes, as its daemon writes them.
type File struct {
	Version int `json:"version"`
	// PID and Began are the daemon's that writes it: once it's gone, the
	// file is skipped (and removed by whoever sees it first).
	PID   int       `json:"pid"`
	Began time.Time `json:"began,omitzero"`
	Root  string    `json:"root"`
	// Name is the name it asks for; Created when it first did, which
	// settles a clash (Names). Given is the name it was given, kept while
	// its daemon runs, so a name doesn't change under its links when
	// another workspace comes or goes.
	Name    string    `json:"name"`
	Given   string    `json:"given,omitempty"`
	Created time.Time `json:"created"`
	// Web is its web UI's port.
	Web int `json:"web"`
	// Routes are its runs, by their name below the workspace's
	// (web.my-capsule.api).
	Routes map[string]Route `json:"routes,omitempty"`
	// Silos are its repos' silos, by repo.
	Silos map[string]Silo `json:"silos,omitempty"`
}

// Route is a run: its port while it runs, or what became of it.
type Route struct {
	Port int `json:"port,omitempty"`
	// Ended says how it ended, for one that isn't running.
	Ended string `json:"ended,omitempty"`
}

// Silo is a repo's silo: the worktree it's pointed at, and the recipes
// that run in it (where = "silo").
type Silo struct {
	Target  string   `json:"target"`
	Recipes []string `json:"recipes,omitempty"`
}

// Dir is where the route files are: $XDG_STATE_HOME/ws/routes, or
// ~/.local/state/ws/routes. $WS_ROUTES says another, for tests and tries
// that want a router of their own.
func Dir() string {
	if d := os.Getenv("WS_ROUTES"); d != "" {
		return d
	}
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		home, _ := os.UserHomeDir()
		state = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(state, "ws", "routes")
}

// fileName is root's route file's name.
func fileName(root string) string { return hashOf(root, 16) + ".json" }

func hashOf(s string, n int) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return fmt.Sprintf("%016x", h.Sum64())[:n]
}

// Write writes f as its workspace's route file in dir, if it's changed.
func Write(dir string, f File) error {
	f.Version = Version
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	path := filepath.Join(dir, fileName(f.Root))
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, b) {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := path + ".new"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Remove removes root's route file from dir.
func Remove(dir, root string) error {
	err := os.Remove(filepath.Join(dir, fileName(root)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Created is when root's route file first asked for its name: kept when
// it's written again, so a daemon restarting keeps its place in a clash.
func Created(dir, root string, now time.Time) time.Time {
	if f, err := read(filepath.Join(dir, fileName(root))); err == nil && !f.Created.IsZero() {
		return f.Created
	}
	return now.UTC().Truncate(time.Second)
}

// Give is the name f goes by, among the route files in dir (its own
// among them or not): the name it was given before, if it was and no one
// that came first has it; else as Names settles it.
func Give(dir string, f File, procs runs.Procs) string {
	if old, err := read(filepath.Join(dir, fileName(f.Root))); err == nil && old.Name == f.Name {
		f.Given = old.Given
	}
	files, _ := Read(dir, procs)
	others := files[:0:0]
	for _, o := range files {
		if o.Root != f.Root {
			others = append(others, o)
		}
	}
	return Names(append(others, f))[f.Root]
}

func read(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, err
	}
	return f, nil
}

// Read is the route files in dir whose daemons are running, by root.
// Newer are those of a version this ws can't read, whose names it can only
// say are a newer ws's.
func Read(dir string, procs runs.Procs) (files []File, newer []File) {
	paths, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, p := range paths {
		f, err := read(p)
		if err != nil || f.Root == "" {
			continue
		}
		if procs.Known && f.PID > 0 {
			if pr, ok := procs.ByPID[f.PID]; !ok || (!f.Began.IsZero() && !sameSecond(pr.Start, f.Began)) {
				os.Remove(p) // its daemon has gone without removing it
				continue
			}
		}
		if f.Version > Version {
			newer = append(newer, f)
			continue
		}
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Root < files[j].Root })
	return files, newer
}

func sameSecond(a, b time.Time) bool {
	d := a.Sub(b)
	return d < time.Second && d > -time.Second
}

var notLabel = regexp.MustCompile(`[^a-z0-9]+`)

// Label is s as one label of a name: lower case letters, numbers and
// single dashes, 63 at most.
func Label(s string) string {
	l := strings.Trim(notLabel.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(l) > 63 {
		l = strings.TrimRight(l[:63], "-")
	}
	return l
}

// NameFor is the name a workspace asks for: what it says, or its folder's.
func NameFor(root, said string) string {
	if n := Label(said); n != "" {
		return n
	}
	if n := Label(filepath.Base(root)); n != "" {
		return n
	}
	return "ws"
}

// Names is the name each workspace goes by, by root. Each keeps the name
// it was given (Given) while that's its alone. Otherwise, where two ask
// for the same, the one that asked first has it, and the other goes by it
// with four of its root's hash after (ws-test-3fa2): a clash is never
// refused, and the name it settles on stays the same.
func Names(files []File) map[string]string {
	fs := append([]File(nil), files...)
	sort.Slice(fs, func(i, j int) bool {
		if !fs[i].Created.Equal(fs[j].Created) {
			return fs[i].Created.Before(fs[j].Created)
		}
		return fs[i].Root < fs[j].Root
	})
	out, taken := map[string]string{}, map[string]bool{}
	// Those given a name keep it, the first to have it first.
	for _, f := range fs {
		if f.Given != "" && !taken[f.Given] {
			out[f.Root], taken[f.Given] = f.Given, true
		}
	}
	for _, f := range fs {
		if _, ok := out[f.Root]; ok {
			continue
		}
		n := f.Name
		if taken[n] {
			n = f.Name + "-" + hashOf(f.Root, 4)
		}
		out[f.Root], taken[n] = n, true
	}
	return out
}

// RunKey is a run's name below its workspace's: <run>.<worktree>.<repo>.
func RunKey(name, worktree, repo string) string {
	return Label(name) + "." + Label(runs.Label(worktree)) + "." + Label(repo)
}

// Host is a name in full: what's below the workspace's (a run's key, or ""
// for its web UI), the workspace's, and the suffix.
func Host(key, workspace string) string {
	if key == "" {
		return workspace + Suffix
	}
	return key + "." + workspace + Suffix
}
