// Package ops is ws's application layer: each operation that spans more
// than one of its stores (accepting a stage, answering a mission's
// question, writing a brief), written once. The commands and the web UI
// call it; neither repeats what it does.
//
// An operation returns what happened, as data that also says itself in a
// sentence, or an error from errors.go. Nothing here prints.
package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/find"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/pulls"
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
}

// loaded is the workspace's config, and what follows from it, as last
// read; stamp says which ws.toml and ws.local.toml it was read from.
type loaded struct {
	cfg   *config.Config
	ws    *workspace.Workspace
	gh    github.Client
	stamp string
}

// Options are what ops is given: where crew run, and where agents'
// work is started (the daemon's jobs), which cli gives.
type Options struct {
	// Host is where crew run: the workspace's daemon, reached over its
	// socket from a command, or the server itself, inside it. nil for
	// nowhere: launching is refused, and a stage's crew are queued.
	Host crew.Host
	// Orchestrate runs the orchestrator on a mission, for a crew's event
	// ("" for none) and a trigger (answer, review…).
	Orchestrate func(slug, crewID, trigger string)
	// Check has the daemon run and judge the checks of the step crew
	// reported done at (job.Check). nil runs them there and then.
	Check func(slug, crewID string)
	// Answer has the daemon answer a conversation (kind job.Answer) or a
	// thread (job.Thread) in a mission ("" for the workspace's), or draft
	// what a resolved thread decided (job.Draft). nil for no answers.
	Answer func(kind, slug, id string)
	// Now is the clock (default time.Now): tests set it.
	Now func() time.Time
	// PRs is what GitHub said of pull requests lately, which the views
	// show (the daemon's, shared with its watcher); nil for none.
	PRs *PRCache
	// Reviews is the Reviews page's service, where a test gives its own
	// (PRService otherwise makes one); Team is the Team page's, the
	// daemon's: nil for none.
	Reviews *pulls.Service
	Team    *team.Service
	// GitHub is the client in place of the one the config makes: a test's
	// stub. nil for the config's.
	GitHub github.Client
}

// New is the operations of a workspace whose config and repos are loaded
// (Load), for as long as one command runs. gh is nil for no GitHub.
func New(cfg *config.Config, ws *workspace.Workspace, gh github.Client, o Options) *Workspace {
	return &Workspace{Missions: mission.Store{Root: ws.Root}, root: ws.Root, opts: o, cur: loaded{cfg: cfg, ws: ws, gh: gh}}
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
