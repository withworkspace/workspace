package runs

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// The daemon keeps where every run stands in memory (Keeper): each one's
// record, whether it has ended, whether its process is there, whether a
// service answers on its port, and how many earlier runs a task keeps.
// The pages' stamps of runs (Stamp, StampIn, Running, Starting) are then a
// lookup: no folder's globbed, no file's read, and no port's dialled when
// a page asks.
//
// What moves it is what's written under the store's folder (the daemon
// watches it and calls Look), and its own asks of what no file says
// (Probe): a run's process gone without its wrapper saying so, a service
// coming up on its port or going down. A service that's starting is asked
// every ProbeStarting; the rest every ProbeEvery. Look reads only the
// records that moved.
//
// Outside the daemon nothing's kept: Keeping is false, and the stamps are
// the Store's own, read from the files.

var (
	// ProbeStarting is how often a run that's starting (a service not
	// answering on its port yet, or a folder claimed with no record in it
	// yet) is asked after: the page waiting on it hears soon.
	ProbeStarting = 250 * time.Millisecond
	// ProbeEvery is how often the runs that are running are asked after
	// otherwise: whether each one's process is still there.
	ProbeEvery = time.Second
	// upEvery is how often a service that's up is asked whether it still
	// answers.
	upEvery = 5 * time.Second
	// pendingFor is how long a folder with no record in it is waited on as
	// a start under way: one older is a start that was killed.
	pendingFor = 10 * time.Second
)

// Keeper is where the store's runs stand, kept current by the daemon.
type Keeper struct {
	Store Store

	keeping atomic.Bool
	version atomic.Int64 // moves whenever where any run stands does

	mu   sync.Mutex
	runs map[string]*keptRun // by folder
	// pending are folders claimed for a run with no record in them yet,
	// by when they were made.
	pending map[string]time.Time
	all     string
	places  map[string]string // each place's stamp (StampIn), by its folder
}

// keptRun is what's kept of one run.
type keptRun struct {
	dir, place string
	// size and mod are its record's, as last read; rec is what it says.
	size, mod int64
	rec       Record
	// exit is when its wrapper said it ended (0: it hasn't).
	exit int64
	// past is how many earlier runs a task keeps (0 for a service).
	past int
	// alive: its process is there; up: a service answers on its port, as
	// asked at upAt.
	alive, up bool
	upAt      time.Time
}

func (r *keptRun) running() bool { return r.exit == 0 }

// starting is whether it's running and not yet up: anything but a
// service answering on its port (a task, until it ends).
func (r *keptRun) starting() bool {
	return r.running() && (r.rec.IsTask() || r.rec.Port <= 0 || !r.up)
}

// line is what the stamps are made of: what Store.stamp says of a run.
func (r *keptRun) line() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %d %d", r.dir, r.size, r.mod)
	if r.exit != 0 {
		fmt.Fprintf(&b, " exit %d", r.exit)
		return b.String()
	}
	fmt.Fprintf(&b, " alive %v", r.alive)
	if !r.rec.IsTask() && r.rec.Port > 0 {
		fmt.Fprintf(&b, " up %v", r.up)
	}
	if r.rec.IsTask() {
		fmt.Fprintf(&b, " past %d", r.past)
	}
	return b.String()
}

// Keeping is whether the daemon's keeping the runs: Look has been called.
func (k *Keeper) Keeping() bool { return k.keeping.Load() }

// Version moves whenever where any run stands does.
func (k *Keeper) Version() int64 { return k.version.Load() }

// Folders are the folders a run's start, end or clearing away writes in:
// the store's, each repo's and place's in it, and each run's own. Not
// below a run's (its reports), nor the earlier runs (.past): a task's
// next run writes its record as they move.
// Until the store's folder is made (nothing's been run), its parent is
// watched for it.
func (k *Keeper) Folders() []string {
	root := k.Store.Dir()
	if _, err := os.Stat(root); err != nil {
		return []string{filepath.Dir(root)}
	}
	out := []string{}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if rel == pastDir {
			return filepath.SkipDir
		}
		out = append(out, path)
		if rel != "." && strings.Count(rel, string(filepath.Separator)) == 2 {
			return filepath.SkipDir // a run's own: not its reports
		}
		return nil
	})
	return out
}

// Heeds is whether a file written at path moves where a run stands:
// what's in the store's folder (or the folder itself, made), not a run's
// output.
func (k *Keeper) Heeds(path string) bool {
	root := k.Store.Dir()
	if path != root && !strings.HasPrefix(path, root+string(filepath.Separator)) {
		return false
	}
	return filepath.Base(path) != logFile
}

// Look reads where every run stands from the store's folder, reading
// again only the records that have moved, and asks after a run it hasn't
// seen. It says whether anything moved.
func (k *Keeper) Look(now time.Time) bool {
	k.keeping.Store(true)
	dirs, _ := filepath.Glob(filepath.Join(k.Store.Dir(), "*", "*", "*"))
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.runs == nil {
		k.runs, k.pending = map[string]*keptRun{}, map[string]time.Time{}
	}
	seen := map[string]bool{}
	clear(k.pending)
	for _, dir := range dirs {
		if strings.HasPrefix(filepath.Base(filepath.Dir(filepath.Dir(dir))), pastDir) {
			continue
		}
		info, err := os.Stat(filepath.Join(dir, recordFile))
		if err != nil {
			if fi, err := os.Stat(dir); err == nil && fi.IsDir() && now.Sub(fi.ModTime()) < pendingFor {
				k.pending[dir] = fi.ModTime()
			}
			continue
		}
		seen[dir] = true
		r := k.runs[dir]
		if r == nil || r.size != info.Size() || r.mod != info.ModTime().UnixNano() {
			var rec Record
			b, err := os.ReadFile(filepath.Join(dir, recordFile))
			if err != nil || json.Unmarshal(b, &rec) != nil {
				delete(seen, dir) // half-written: as the Store, it's not there yet
				continue
			}
			fresh := r == nil
			if fresh {
				r = &keptRun{dir: dir, place: filepath.Dir(dir)}
				k.runs[dir] = r
			}
			if fresh || rec.PID != r.rec.PID || rec.Port != r.rec.Port {
				r.upAt = time.Time{} // another process: asked after below
			}
			r.size, r.mod, r.rec = info.Size(), info.ModTime().UnixNano(), rec
		}
		r.exit = 0
		if ex, err := os.Stat(filepath.Join(dir, exitFile)); err == nil {
			r.exit = ex.ModTime().UnixNano()
		}
		r.past = 0
		if r.rec.IsTask() {
			if past, err := os.ReadDir(k.Store.pastOf(dir)); err == nil {
				r.past = len(past)
			}
		}
		if r.running() {
			r.alive = r.rec.PID > 0 && syscall.Kill(r.rec.PID, 0) == nil
			if !r.rec.IsTask() && r.rec.Port > 0 && r.upAt.IsZero() {
				r.up, r.upAt = Run{Record: r.rec}.Listening(), now
			}
		}
	}
	for dir := range k.runs {
		if !seen[dir] {
			delete(k.runs, dir)
		}
	}
	return k.restamp()
}

// Probe asks after what no file says: each running run's process, a
// service's port (every ProbeStarting while it's starting, upEvery once
// it's up). It says whether anything moved, and when it's next due.
func (k *Keeper) Probe(now time.Time) (moved bool, next time.Duration) {
	k.mu.Lock()
	defer k.mu.Unlock()
	next = ProbeEvery
	if len(k.pending) > 0 {
		next = ProbeStarting // Look reads it once its record's there
	}
	for _, r := range k.runs {
		if !r.running() {
			continue
		}
		r.alive = r.rec.PID > 0 && syscall.Kill(r.rec.PID, 0) == nil
		if r.rec.IsTask() || r.rec.Port <= 0 {
			continue
		}
		if !r.up || now.Sub(r.upAt) >= upEvery {
			r.up, r.upAt = Run{Record: r.rec}.Listening(), now
		}
		if !r.up && r.alive {
			next = ProbeStarting
		}
	}
	return k.restamp(), next
}

// Pending is whether a folder's been claimed for a run that hasn't
// written its record yet: Look is due again soon.
func (k *Keeper) Pending() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.pending) > 0
}

// restamp works out the stamps again, moving the version if they've
// moved. k.mu is held.
func (k *Keeper) restamp() bool {
	dirs := make([]string, 0, len(k.runs))
	for dir := range k.runs {
		dirs = append(dirs, dir)
	}
	slices.Sort(dirs)
	all := fnv.New64a()
	byPlace := map[string][]string{}
	for _, dir := range dirs {
		r := k.runs[dir]
		l := r.line()
		fmt.Fprintln(all, l)
		byPlace[r.place] = append(byPlace[r.place], l)
	}
	places := make(map[string]string, len(byPlace))
	for place, lines := range byPlace {
		h := fnv.New64a()
		for _, l := range lines {
			fmt.Fprintln(h, l)
		}
		places[place] = strconv.FormatUint(h.Sum64(), 16)
	}
	stamp := strconv.FormatUint(all.Sum64(), 16)
	if stamp == k.all {
		return false
	}
	k.all, k.places = stamp, places
	k.version.Add(1)
	return true
}

// Stamp is Store.Stamp as kept: false while nothing's kept.
func (k *Keeper) Stamp() (string, bool) {
	if !k.Keeping() {
		return "", false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.all, true
}

// StampIn is Store.StampIn as kept: false while nothing's kept.
func (k *Keeper) StampIn(repo, worktree string) (string, bool) {
	if !k.Keeping() {
		return "", false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.places[k.Store.place(repo, worktree)], true
}

// Running is Store.Running as kept (kept false while nothing's kept).
func (k *Keeper) Running(repo, worktree, recipe string) (run Run, ok, kept bool) {
	r, kept := k.find(repo, worktree, recipe)
	if r == nil {
		return Run{}, false, kept
	}
	return Run{Record: r.rec, Path: r.dir, State: Running}, true, true
}

// Starting is Store.Starting as kept (kept false while nothing's kept):
// how far its output's got is read from its log, which isn't kept.
func (k *Keeper) Starting(repo, worktree, recipe string) (out string, ok, kept bool) {
	r, kept := k.find(repo, worktree, recipe)
	if r == nil {
		return "", false, kept
	}
	k.mu.Lock()
	starting := r.starting()
	k.mu.Unlock()
	if !starting {
		return "", false, true
	}
	if info, err := os.Stat(filepath.Join(r.dir, logFile)); err == nil {
		return fmt.Sprintf("%d %d", info.Size(), info.ModTime().UnixNano()), true, true
	}
	return "", true, true
}

// find is the run of recipe in repo's worktree that's running, as kept.
func (k *Keeper) find(repo, worktree, recipe string) (*keptRun, bool) {
	if !k.Keeping() {
		return nil, false
	}
	place := k.Store.place(repo, worktree)
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, r := range k.runs {
		if r.place == place && r.rec.Recipe == recipe && r.running() {
			return r, true
		}
	}
	return nil, true
}
