// Package ops is ws's application layer: each operation that spans more
// than one of its stores (accepting a stage, answering a mission's
// question, writing a brief), written once. The commands and the web UI
// call it; neither repeats what it does.
//
// An operation returns what happened, as data that also says itself in a
// sentence, or an error from errors.go. Nothing here prints.
package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/find"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/outbox"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/release"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/team"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Workspace is one workspace's operations. A command makes one for its
// run (New); the daemon opens one for its life (Open), whose config is
// read again when it changes.
type Workspace struct {
	Missions mission.Store
	root     string
	opts     Options
	reload   bool

	mu  sync.Mutex
	cur loaded

	prOnce sync.Once
	prSvc  *pulls.Service

	findOnce sync.Once
	findIx   *find.Index

	// git is what git said of each capsule, kept for the views pages read
	// until the capsule moves.
	git keptGit
	// caps is every repo's worktrees, kept by the daemon (capsulekeeper.go).
	caps capsuleKeeper
	// runs is where every run stands, kept by the daemon (runskeeper.go).
	runsKept runs.Keeper
	// overview is each mission's row in the workspace's view, kept till
	// what it's built from moves (overview_kept.go).
	overview keptOverview
	// spaces is what's kept of the spaces of documents, for the pages that
	// read them (spaces_kept.go).
	spaces keptSpaces
	// missions is what a mission's page is told by that's kept: the
	// writes in its folder, and its record as last read (mission_kept.go).
	missions keptMissions

	// updates is one look for later releases, or one upgrade, at a time;
	// plugins is one plugin added, updated or removed at a time.
	updates sync.Mutex
	plugins sync.Mutex
	// gh is what GitHub last said of whose repos can be added and what
	// each has, and the repos added being cloned (addrepo.go).
	gh ghListings
	// upgraded is the ws an upgrade from here last put in the daemon's
	// place (a string): said to be waiting from the moment it's installed,
	// before the daemon's found it there.
	upgraded atomic.Value
}

// loaded is the workspace's config, and what follows from it, as last
// read; stamp says which ws.toml and ws.local.toml it was read from.
type loaded struct {
	cfg   *config.Config
	ws    *workspace.Workspace
	gh    github.Client
	stamp string
}

// Jobs starts agents' work that outlives whoever asked for it (job.Job).
type Jobs interface {
	// Takes is whether it starts jobs of a kind: one it doesn't is done
	// as Options.Jobs says.
	Takes(kind string) bool
	// Start starts a job of a kind it takes.
	Start(job.Job)
}

// JobsOf is Jobs that starts each job of the kinds given (every kind, for
// none) with start.
func JobsOf(start func(job.Job), kinds ...string) Jobs {
	return jobsFunc{start: start, kinds: kinds}
}

type jobsFunc struct {
	start func(job.Job)
	kinds []string
}

func (f jobsFunc) Takes(kind string) bool { return len(f.kinds) == 0 || slices.Contains(f.kinds, kind) }
func (f jobsFunc) Start(j job.Job)        { f.start(j) }

// takes is whether the workspace's Jobs start jobs of kind.
func (w *Workspace) takes(kind string) bool {
	return w.opts.Jobs != nil && w.opts.Jobs.Takes(kind)
}

// start has the workspace's Jobs start j, if they take its kind, and says
// whether they did.
func (w *Workspace) start(j job.Job) bool {
	if !w.takes(j.Kind) {
		return false
	}
	w.opts.Jobs.Start(j)
	return true
}

// Options are what ops is given: where crew run, and where agents'
// work is started (the daemon's jobs), which cli gives.
type Options struct {
	// Host is where crew run: the workspace's daemon, reached over its
	// socket from a command, or the server itself, inside it. nil for
	// nowhere: launching is refused, and a stage's crew are queued.
	Host crew.Host
	// Jobs is where agents' work that outlives whoever asked for it is
	// started: the daemon's queue, in it, or the way to it from a command.
	// A job it doesn't take (nil takes none) is done as ws does without a
	// daemon: a step's checks (job.Check) are run, and a pull request
	// (job.PR) written and opened, there and then; the orchestrator
	// (job.Orchestrate) isn't run, and conversations (job.Answer,
	// job.Thread, job.Draft) aren't answered.
	Jobs Jobs
	// Plugins is the workspace's plugins as they run, in this process (the
	// daemon's); nil reaches them through the daemon (PluginRunners).
	// Close on it does nothing.
	Plugins PluginRunner
	// Live is what running crew show now (by key, slug/crew), from where
	// they run: the orchestrator reads it. nil, or "" for crew it isn't
	// running, reads what they left.
	Live func(key string) string
	// Now is the clock (default time.Now): tests set it.
	Now func() time.Time
	// Life is the workspace's life in this process (the daemon's): ended,
	// the agents' turns it's running end with it, and what they were for
	// is left for the next to do. nil is never.
	Life context.Context
	// PRs is what GitHub said of pull requests lately, which the views
	// show (the daemon's, shared with its watcher); nil for none.
	PRs *PRCache
	// Reviews is the Reviews page's service, where a test gives its own
	// (PRService otherwise makes one); Team is the Team page's, the
	// daemon's: nil for none.
	Reviews *pulls.Service
	Team    *team.Service
	// Outbox is where what's said to somewhere slow (a reply on a pull
	// request, to GitHub) is sent from, behind the answer: the daemon's.
	// nil sends each as it's said, and waits.
	Outbox *outbox.Queue
	// GitHub is the client in place of the one the config makes: a test's
	// stub. nil for the config's.
	GitHub github.Client
	// WS is the ws this process is, and where its releases are: what a
	// look for updates measures against, and an upgrade replaces (the
	// daemon's). Its zero value is a ws nothing's looked for.
	WS release.Upgrader
	// Pending is a ws that's been installed in the daemon's place which it
	// isn't running yet ("" for none): it restarts onto it once nothing's
	// at work, and AtWork is what is. nil for nothing to ask.
	Pending func() string
	AtWork  func() []string
}

// New is the operations of a workspace whose config and repos are loaded
// (Load), for as long as one command runs. gh is nil for no GitHub.
func New(cfg *config.Config, ws *workspace.Workspace, gh github.Client, o Options) *Workspace {
	w := &Workspace{Missions: mission.Store{Root: ws.Root}, root: ws.Root, opts: o, cur: loaded{cfg: cfg, ws: ws, gh: gh}}
	// What the records' words name is found as this workspace names things
	// (refs.go): the latest to be made for a root says how.
	store.Name(ws.Root, w.namesKnown)
	return w
}

// Open is the operations of the workspace at root for a process that
// holds them (the daemon): its config, its repos and GitHub are read again
// whenever ws.toml or ws.local.toml has changed, and not otherwise. One
// whose config doesn't read yet has none (no repos, no GitHub) until it
// does.
func Open(root string, o Options) *Workspace {
	w := New(&config.Config{}, &workspace.Workspace{Root: root}, nil, o)
	w.reload = true
	w.loaded()
	// Held for the process's life (the daemon's), its way of finding names
	// stands: a workspace loaded for a moment after it, with its index cold,
	// doesn't take its place.
	store.KeepName(w.Root(), w.namesKnown)
	return w
}

// configStamp names ws.toml and ws.local.toml as they are now: their sizes
// and when they were written.
func configStamp(root string) string {
	stamp := ""
	for _, name := range []string{config.FileName, config.LocalFileName} {
		if info, err := os.Stat(filepath.Join(root, name)); err == nil {
			stamp += fmt.Sprintf("%s:%d:%d;", name, info.Size(), info.ModTime().UnixNano())
		}
	}
	return stamp
}

// loaded is the config as it is now: read again if it's changed. A config
// that no longer reads keeps the last one that did, and says why in the
// process log.
func (w *Workspace) loaded() loaded {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.reload {
		return w.cur
	}
	stamp := configStamp(w.root)
	if stamp == w.cur.stamp {
		return w.cur
	}
	cfg, ws, gh, err := Load(w.root)
	if err != nil {
		execlog.Note(w.root, "ws", "the workspace's config didn't read, so the last that did is kept: %v", err)
		w.cur.stamp = stamp // not tried again until it changes
		return w.cur
	}
	w.cur = loaded{cfg: cfg, ws: ws, gh: gh, stamp: stamp}
	return w.cur
}

// Root is the workspace's folder.
func (w *Workspace) Root() string { return w.root }

// Config is the workspace's config.
func (w *Workspace) Config() *config.Config { return w.loaded().cfg }

// WS is the workspace's repos, as its config has them.
func (w *Workspace) WS() *workspace.Workspace { return w.loaded().ws }

// GitHub is where GitHub is asked: nil for nowhere.
func (w *Workspace) GitHub() github.Client {
	if w.opts.GitHub != nil {
		return w.opts.GitHub
	}
	return w.loaded().gh
}

// ActorKind is what kind of someone asks for an operation.
type ActorKind int

const (
	You          ActorKind = iota // the user, from a command or the web UI
	Crew                          // an agent working on a mission
	Orchestrator                  // the agent deciding what's next
	WS                            // ws itself, moving a plan on
)

// Actor is who asks. Rules that matter (crew don't accept their own stage,
// crew don't land) are checked against it here, whoever's front end it is.
type Actor struct {
	Kind ActorKind
	ID   string // a crew's id
}

// Actors that need no id.
var (
	AsYou          = Actor{Kind: You}
	AsOrchestrator = Actor{Kind: Orchestrator}
	AsWS           = Actor{Kind: WS}
)

// AsCrew is crew id.
func AsCrew(id string) Actor { return Actor{Kind: Crew, ID: id} }

// ActorFromEnv is who's running this process: the orchestrator (WS_ROLE),
// crew (WS_CREW), else you.
func ActorFromEnv() Actor {
	if os.Getenv(mission.EnvRole) == mission.RoleOrchestrator {
		return AsOrchestrator
	}
	if c := os.Getenv(mission.EnvCrew); c != "" {
		return AsCrew(c)
	}
	return AsYou
}

// By is how the actor is written in a mission's records: "you", a crew's
// id, "orchestrator" or "ws".
func (a Actor) By() string {
	switch a.Kind {
	case Crew:
		return a.ID
	case Orchestrator:
		return mission.RoleOrchestrator
	case WS:
		return "ws"
	}
	return "you"
}

// IsYou is whether the user asked.
func (a Actor) IsYou() bool { return a.Kind == You }

// load is a mission, or ErrNotFound.
func (w *Workspace) load(slug string) (*mission.Mission, error) {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return nil, wrap(err)
	}
	return m, nil
}

// youActed records that you stepped in on a mission: the orchestrator's
// count of crew started without you starts again. Everything you do that
// changes a mission counts.
func (w *Workspace) youActed(slug string, by Actor) {
	if by.IsYou() {
		w.Missions.UserActed(slug)
	}
}
