package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/withworkspace/workspace/internal/space"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/workspace"
)

// What the daemon keeps of the workspace's spaces of documents, so a
// page's read of one answers at once: the records (each document, read
// from its ground) and each space's activity (what changed lately, what's
// in flight in capsules, how its ground stands). Reading them from git was
// a process per space on every read, and a few per capsule for a space's
// activity: a quarter of a second, each time a document opened.
//
// A read is answered from what's kept, and whether that's still so is
// known from git's own files (capsuleStamp: no process). When a ground or
// a capsule has moved, or what's kept of a space's activity is older than
// spaceFloor (what's uncommitted moves nothing git keeps), it's read again
// behind the read, one look at a time. A look that finds something other
// than the pages were given moves the workspace's spacesSeen and says so
// on Seen, so the pages read again.

// spaceFloor is how long a space's activity is shown as it was last read
// before it's read again, behind a read: a document edited in a capsule,
// or on a ground, moves nothing a stamp sees.
var spaceFloor = 10 * time.Second

// behind runs a look behind a read: at once, by itself (a test holds it).
var behind = func(look func()) { go look() }

// keptSpaces is what's kept of the workspace's spaces.
type keptSpaces struct {
	mu sync.Mutex
	// read is the grounds' stamp as the records were last read from them,
	// and problem why that read failed, if it did.
	read    string
	problem error
	// of is each space's activity as last read, by repo.
	of map[string]*keptActivity
	// looking: a look is under way; again: something was asked of it
	// since it began, so it looks once more when it's done.
	looking, again bool
	// seen counts the looks that found something other than the pages
	// were given; told says so at once, to the one that listens (Seen).
	seen atomic.Int64
	told chan struct{}
}

// keptActivity is a space's activity as of stamp (its ground's and its
// capsules'), read at at. Each flight's mission and pull request aren't
// kept: they're said as they are at each read.
type keptActivity struct {
	stamp   string
	at      time.Time
	lately  []SpaceCommit
	flights []SpaceFlight
	fresh   SpaceFresh
	// wanted: a read found it stale, and the next look reads it again.
	wanted bool
}

func (k *keptSpaces) init() {
	if k.of == nil {
		k.of = map[string]*keptActivity{}
		k.told = make(chan struct{}, 1)
	}
}

// Seen says when a look behind a read has found something other than
// the pages were given, a space or a capsule: the views' stamps say it
// too (SpacesSeen, dirtyMoved), so their events are due. One listener:
// the daemon's.
func (w *Workspace) Seen() <-chan struct{} {
	w.spaces.mu.Lock()
	defer w.spaces.mu.Unlock()
	w.spaces.init()
	// A capsule found other than a read was given is said here too.
	w.git.mu.Lock()
	w.git.found = w.tellSeen
	w.git.mu.Unlock()
	return w.spaces.told
}

// tellSeen says on Seen that something's other than the pages were
// given: a look behind a read found it, or git's written where it's
// watched (WatchGit). It doesn't wait.
func (w *Workspace) tellSeen() {
	w.spaces.mu.Lock()
	w.spaces.init()
	told := w.spaces.told
	w.spaces.mu.Unlock()
	select {
	case told <- struct{}{}:
	default: // one waiting to be heard stands for this
	}
}

// SpacesSeen names how many times a look behind a read has found a space
// other than the pages were given: a page showing a space reads it again
// when it moves.
func (w *Workspace) SpacesSeen() string {
	if n := w.spaces.seen.Load(); n > 0 {
		return strconv.FormatInt(n, 10)
	}
	return ""
}

// groundsStamp names what the records' documents are read from: which
// spaces there are, and each one's ground as git keeps it. No process.
func groundsStamp(ws *workspace.Workspace) string {
	var b strings.Builder
	for _, s := range ws.Spaces() {
		fmt.Fprintf(&b, "%s %s %s\n", s.Repo, s.Folder, capsuleStamp(ws.MainWorktree(s.Repo)))
	}
	return b.String()
}

// activityStamp names what a space's activity is read from: its ground,
// and each capsule of its repo, as git keeps them. No process.
func activityStamp(ws *workspace.Workspace, s workspace.Space) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", s.Folder, capsuleStamp(ws.MainWorktree(s.Repo)))
	capsules, _ := workspace.ListWorktrees(ws.RepoDir(s.Repo))
	for _, name := range capsules {
		fmt.Fprintf(&b, "%s %s\n", name, capsuleStamp(filepath.Join(ws.RepoDir(s.Repo), name)))
	}
	return b.String()
}

// spacesKept is the records, with the spaces' documents as they were last
// read in, and why that read failed, if it did. Where a ground has moved
// since, they're read again behind it.
func (w *Workspace) spacesKept() (*store.DB, error) {
	db, err := w.Records()
	if err != nil {
		return nil, err
	}
	stamp := groundsStamp(w.WS())
	w.spaces.mu.Lock()
	defer w.spaces.mu.Unlock()
	w.spaces.init()
	if stamp != w.spaces.read {
		w.lookLocked()
	}
	return db, w.spaces.problem
}

// activityKept is a space's activity as last read, and whether it's been
// read at all. Where it's moved since, or it's past spaceFloor, it's read
// again behind it.
func (w *Workspace) activityKept(s workspace.Space) (keptActivity, bool) {
	stamp := activityStamp(w.WS(), s)
	w.spaces.mu.Lock()
	defer w.spaces.mu.Unlock()
	w.spaces.init()
	k := w.spaces.of[s.Repo]
	if k == nil {
		k = &keptActivity{}
		w.spaces.of[s.Repo] = k
	}
	if k.at.IsZero() || k.stamp != stamp || time.Since(k.at) > spaceFloor {
		k.wanted = true
		w.lookLocked()
	}
	return *k, !k.at.IsZero()
}

// keepFresh is how a space's ground stands, as a fetch or a pull has just
// said: kept, so the next read says it too.
func (w *Workspace) keepFresh(repo string, f SpaceFresh) {
	w.spaces.mu.Lock()
	defer w.spaces.mu.Unlock()
	if k := w.spaces.of[repo]; k != nil && !k.at.IsZero() {
		k.fresh = f
	}
}

// lookLocked has a look begin, or the one under way look again when it's
// done. w.spaces.mu is held.
func (w *Workspace) lookLocked() {
	w.spaces.again = true
	if w.spaces.looking {
		return
	}
	w.spaces.looking = true
	behind(w.look)
}

// look reads again what's moved: the records where a ground has, and each
// space's activity a read wanted. It tells of what it found that's other
// than what was kept.
func (w *Workspace) look() {
	ctx := context.Background()
	for {
		w.spaces.mu.Lock()
		if !w.spaces.again {
			w.spaces.looking = false
			w.spaces.mu.Unlock()
			return
		}
		w.spaces.again = false
		read := w.spaces.read
		var wanted []string
		for repo, k := range w.spaces.of {
			if k.wanted {
				wanted = append(wanted, repo)
				k.wanted = false
			}
		}
		w.spaces.mu.Unlock()

		found := false
		ws := w.WS()
		// The stamp's taken before the read: a ground that moves during
		// it is read again next time.
		if stamp := groundsStamp(ws); stamp != read {
			_, st, err := w.Spaces(ctx)
			w.spaces.mu.Lock()
			found = st.Read > 0 || st.Removed > 0 || fmt.Sprint(err) != fmt.Sprint(w.spaces.problem)
			w.spaces.read, w.spaces.problem = stamp, err
			w.spaces.mu.Unlock()
		}
		for _, repo := range wanted {
			s, ok := ws.SpaceOf(repo)
			if !ok {
				continue
			}
			stamp := activityStamp(ws, s)
			now := w.readActivity(ctx, s)
			now.stamp, now.at = stamp, time.Now()
			w.spaces.mu.Lock()
			k := w.spaces.of[repo]
			if k == nil || k.at.IsZero() || !reflect.DeepEqual(k.lately, now.lately) || !reflect.DeepEqual(k.flights, now.flights) || k.fresh != now.fresh {
				found = true
			}
			now.wanted = k != nil && k.wanted
			w.spaces.of[repo] = &now
			w.spaces.mu.Unlock()
		}
		if found {
			w.spaces.seen.Add(1)
			select {
			case w.spaces.told <- struct{}{}:
			default: // one waiting to be heard stands for this
			}
		}
	}
}

// readActivity reads a space's activity from git: its ground's commits
// and how it stands, and its capsules' changes to its documents. Titles
// are the records'.
func (w *Workspace) readActivity(ctx context.Context, s workspace.Space) keptActivity {
	out := keptActivity{lately: []SpaceCommit{}, flights: []SpaceFlight{}}
	ground := w.WS().MainWorktree(s.Repo)
	if _, err := os.Stat(ground); err != nil {
		return out // not cloned here
	}
	titles := map[string]string{}
	if db, err := w.Records(); err == nil {
		if docs, err := space.Docs(ctx, db, s.Repo); err == nil {
			for _, d := range docs {
				titles[d.Path] = d.Title
			}
		}
	}
	if lately, err := spaceLately(ctx, ground, s, titles); err == nil {
		out.lately = lately
	}
	out.flights = w.spaceInFlight(ctx, s, titles)
	out.fresh = w.spaceFresh(ctx, s)
	return out
}
