package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/release"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/web"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Config is what the daemon is given by ws's commands, which start it:
// where it runs, which ws it is, and the little only the commands know.
type Config struct {
	Root string
	// Build is the ws it is (its version), and Exe its binary ("" when it
	// can't be found, when the daemon doesn't restart itself).
	Build, Exe string
	Log        *log.Logger
	// WS is this ws and where its releases are: what a look for updates
	// measures against, and what an upgrade from the web UI replaces.
	WS release.Upgrader
	// Labels are ws's commands as the web UI names them, and Reserved
	// says whether a word after ws is one of its commands, which a plugin
	// can't take.
	Labels   []web.CommandLabel
	Reserved func(word string) bool
	// Show answers what a plugin asks ws show (protocol.MethodShow), from
	// the workspace: the views are the commands'. nil refuses it.
	Show func(w *ops.Workspace, ctx context.Context, params json.RawMessage) (any, error)
}

// daemon is the workspace's daemon as it runs: its server (crew, and the
// socket commands reach it on), the workspace's operations it holds for
// its life, which its jobs, loops, web UI and plugins all work through,
// and what it keeps.
type daemon struct {
	cfg     Config
	s       *Server
	w       *ops.Workspace
	jobs    *job.Queue
	plugins *plugin.Host
	silos   *workspace.SiloWatcher
	prs     *ops.PRCache
	// runsPoke has the runs swept now (crew have ended, and the runs
	// their session owned with them).
	runsPoke chan struct{}
}

// jobsGrace is how long the daemon waits, going, for jobs whose agents
// it's ended to finish.
const jobsGrace = 10 * time.Second

// Run is the workspace's daemon, until it's shut down. restart says it
// shut down to be started again (Server.Restart): by then everything it
// held is closed.
func Run(cfg Config) (restart bool, err error) {
	root := cfg.Root
	if cfg.Log == nil {
		cfg.Log = log.New(os.Stderr, "daemon ", log.LstdFlags)
	}
	d := &daemon{cfg: cfg, runsPoke: make(chan struct{}, 1), prs: ops.NewPRCache()}
	s := &Server{Root: root, Log: cfg.Log, Build: cfg.Build, Exe: cfg.Exe}
	d.s = s
	// This process is the daemon: it takes the records itself, and
	// never starts another to borrow them from. (Something that read
	// a record before it had them would start one, which would do
	// the same.)
	store.Start = func(string) error {
		return errors.New("the daemon reads its records once it's taken them, and starts no other")
	}
	// What ws records is this process's to serve: taken before the
	// daemon's own socket answers, so a command that finds the daemon
	// finds its records.
	records, err := store.Own(root)
	if err != nil {
		return false, err
	}
	defer records.Close()
	if err := records.CopyError(); err != nil {
		s.Log.Printf("records: the day's copy wasn't taken: %v", err)
	}
	// Crew a server before this one left running when it died have
	// nobody to record their end: record them as lost.
	if n := (mission.Store{Root: root}).ReconcileCrew(crew.ProcessAlive); n > 0 {
		s.Log.Printf("recorded %d crew as lost: their server went without ending them", n)
	}

	// Jobs (the orchestrator's runs, conversations' answers) run in
	// here, a child of the server each.
	d.jobs = &job.Queue{Queued: markJob(root), Log: s.Log.Printf}
	d.jobs.Run = d.runJob
	s.Jobs = d.jobs
	// The jobs' agents live as long as the daemon does: as it goes, their
	// turns are ended (process group and all) and waited for, before the
	// records they write to are closed. What they were for is left to the
	// daemon after (reconcileJobs).
	life, end := context.WithCancel(context.Background())
	defer end()
	// The workspace's plugins run in here, one process each, started
	// when one's first needed. Commands reach them over their socket, and
	// what they ask ws is answered from the workspace's operations.
	d.plugins = &plugin.Host{Root: root, Version: cfg.Build, Log: s.Log, Hold: s.Hold, Reserved: cfg.Reserved,
		// How a plugin's things are named is kept in the workspace's
		// records, by the plugin: names are found from there.
		Keep: func(name string, refs []protocol.Ref) ([]string, error) { return plugin.KeepRefs(root, name, refs) },
	}
	// The workspace's operations, for the daemon's life: its config read
	// again when ws.toml or ws.local.toml changes. Crew run in this
	// server, jobs on its queue, plugins in its host; the web UI, the
	// watcher and plugins share what GitHub said (prs).
	upgrader := cfg.WS
	upgrader.Exe = cfg.Exe // as it was when the daemon started: an install since has moved it
	d.w = ops.Open(root, ops.Options{
		Host:    s,
		Live:    s.Transcript,
		Jobs:    ops.JobsOf(d.jobs.Start),
		Plugins: hostRunner{d.plugins},
		PRs:     d.prs,
		WS:      upgrader,
		Pending: s.Pending,
		AtWork:  s.AtWork,
		Life:    life,
	})
	d.w.KeepTeam()
	// A restart that isn't to be now waits for crew mid-turn.
	s.CrewAtWork = d.w.CrewAtWork
	s.OnCrewEnded = func(spec crew.Spec, exit string, stopped bool) {
		// Crew's end is an event the orchestrator may look at (it does
		// when crew ended by themselves).
		d.w.Notify(spec.Slug, mission.OrchestratorEvent{Crew: spec.CrewID, Trigger: "ended", Stopped: stopped})
		d.pokeRuns() // runs their session owned
	}
	s.TrustFolders = func() bool {
		l, err := config.ReadLocal(root)
		return err == nil && l.Crew.TrustFolders
	}
	// A turn of crew over ACP that ended without the crew saying
	// where it is, as a terminal crew's stop hook hands on.
	s.OnACPTurnEnded = d.w.TurnEnded
	// The silos are kept from here: answered for before the socket is.
	silosStop := make(chan struct{})
	defer close(silosStop)
	d.keepSilos(silosStop)
	if err := s.Listen(); err != nil {
		return false, err
	}
	records.SetHold(s.Hold) // a command's connection keeps the daemon up, as a page does
	reconcileJobs(root, d.jobs, s.Log.Printf)
	d.plugins.Answer = d.answerPlugin
	if ln, err := d.plugins.Listen(); err != nil {
		s.Log.Printf("plugins: commands can't reach them: %v", err)
	} else {
		defer ln.Close()
		go d.plugins.Serve(ln)
	}
	defer d.plugins.Shutdown()
	// The web UI, and the pull requests it lists, which the daemon keeps
	// current behind it: the same service, so a page shows what a look
	// found.
	w := d.startWeb()
	if w != nil {
		s.WebPort = w.Info().Port
		defer w.Close()
	}
	// Crew the daemon before this one shut down under, while they
	// were still at their work, carry on.
	go func() {
		for _, r := range d.w.ResumeInterrupted() {
			s.Log.Printf("%s", r)
		}
	}()
	stop := make(chan struct{})
	defer close(stop)
	d.loops(w, stop)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		sig := <-sigs
		s.Shutdown("signal " + sig.String())
	}()
	// Deferred last, so done first: before the plugins, the web UI and
	// the records are let go of.
	defer func() {
		end()
		if !d.jobs.Stop(jobsGrace) {
			s.Log.Printf("jobs still running %s after their agents were ended: going without them", jobsGrace)
		}
	}()
	err = s.Serve()
	return s.Restarting(), err
}

// pokeRuns has the daemon's runs swept now, if a sweep isn't already due.
func (d *daemon) pokeRuns() {
	select {
	case d.runsPoke <- struct{}{}:
	default:
	}
}
