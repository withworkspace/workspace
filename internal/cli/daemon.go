package cli

import (
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/web"
)

// newDeckCmd is what's left of ws deck: the deck was ws's terminal UI for
// missions, and the web UI (ws web) has taken its place; its server is
// the daemon (ws daemon).
func newDeckCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "deck",
		Short:  "Gone: ws web opens the web UI, and ws daemon manages the workspace's daemon",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return errors.New("the deck has gone: ws web opens the web UI, which does what it did, and ws daemon manages the workspace's daemon (ws deck server was its old name)")
		},
	}
}

// --- ws daemon ---

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the workspace's daemon, which runs crew and agents and serves the web UI",
		Long: `The workspace's daemon holds what outlives a command: crew (agents working
on missions), the orchestrator's runs and conversations' answers, and the
web UI. ws starts it when it's first needed (a launch, ws web), and it
stays up until it's closed (ws daemon stop), however long nothing's
running or open.

It keeps the ws it started from. When another's installed in its place
(ws upgrade), it starts again on that one once nothing's at work: no crew
mid-turn, no agent's run. ws daemon restart asks for the same, and with
--now doesn't wait.`,
	}
	cmd.AddCommand(newDaemonRunCmd(), newDaemonStatusCmd(), newDaemonRestartCmd(), newDaemonStopCmd())
	return cmd
}

func newDaemonRunCmd() *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:    "run",
		Short:  "Run the daemon in the foreground (ws starts it when it's needed)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Asked first: once another ws is installed over this one, Linux
			// gives this one's path as deleted.
			exe := ownBinary()
			if root == "" {
				ctx, err := loadOps()
				if err != nil {
					return err
				}
				root = ctx.WS().Root
			}
			restart, err := runDaemon(cmd, root, exe)
			if err != nil || !restart {
				return err
			}
			// Everything the daemon held is closed. This process becomes the
			// ws now at its path, with the pid, environment and log it has:
			// started afresh by a command, it would take that command's.
			execlog.Note(root, "daemon", "restarting onto %s", exe)
			err = syscall.Exec(exe, append([]string{exe}, os.Args[1:]...), os.Environ())
			return fmt.Errorf("restarting onto %s: %w", exe, err)
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "workspace root (default: discovered from the current directory)")
	return cmd
}

// ownBinary is the ws this process runs, as a file: "" if it can't be
// found, when the daemon doesn't restart itself.
func ownBinary() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return exe
}

// runDaemon is the daemon, until it's shut down. restart says it shut down
// to be started again (daemon.Server.Restart): by then everything it held
// is closed.
func runDaemon(cmd *cobra.Command, root, exe string) (restart bool, err error) {
	s := &daemon.Server{Root: root, Log: log.New(os.Stderr, "daemon ", log.LstdFlags), Build: cmd.Root().Version, Exe: exe}
	// A restart that isn't to be now waits for crew mid-turn. If
	// the missions can't be read, it waits: nothing says none is.
	s.CrewAtWork = func() []string {
		ctx, err := LoadOps(root)
		if err != nil {
			return []string{fmt.Sprintf("the workspace's missions, which can't be read (%v)", err)}
		}
		return ctx.CrewAtWork()
	}
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
	runsPoke := make(chan struct{}, 1)
	orchestrate := orchestrateHook(root)
	s.OnCrewEnded = func(spec crew.Spec, exit string, stopped bool) {
		orchestrate(spec, exit, stopped)
		pokeRuns(runsPoke) // runs their session owned
	}
	s.TrustFolders = func() bool {
		l, err := config.ReadLocal(root)
		return err == nil && l.Crew.TrustFolders
	}
	// A turn of crew over ACP that ended without the crew saying
	// where it is, as a terminal crew's stop hook hands on.
	s.OnACPTurnEnded = func(slug, crewID string) { orchestrateQuietTurn(root, slug, crewID) }
	if err := s.Listen(); err != nil {
		return false, err
	}
	records.SetHold(s.Hold) // a command's connection keeps the daemon up, as a page does
	serverHost = s          // crew launched here (from the web UI) need no socket
	// Jobs (the orchestrator's runs, conversations' answers) run in
	// here, a child of the server each.
	s.Jobs = jobQueue(root, s.Log.Printf)
	serverJobs = s.Jobs
	reconcileJobs(root, s.Jobs, s.Log.Printf)
	prs := ops.NewPRCache() // the web UI, the PR watcher and plugins share what GitHub said
	// The workspace's plugins run in here, one process each, started
	// when one's first needed. Commands reach them over their socket, and
	// what they ask ws is answered from the workspace's operations.
	pluginHost = newPluginHost(s, cmd.Root().Version)
	pluginHost.Answer = answerPlugin(openOps(root, prs))
	if ln, err := pluginHost.Listen(); err != nil {
		s.Log.Printf("plugins: commands can't reach them: %v", err)
	} else {
		defer ln.Close()
		go pluginHost.Serve(ln)
	}
	defer pluginHost.Shutdown()
	if w := startWeb(s, prs, cmd.Root().Version); w != nil {
		s.WebPort = w.Info().Port
		defer w.Close()
	}
	// Crew the daemon before this one shut down under, while they
	// were still at their work, carry on.
	go resumeInterrupted(root, s.Log.Printf)
	stop := make(chan struct{})
	defer close(stop)
	go watchPRsLoop(root, prs, stop)
	go watchHeldLoop(root, s.ACPCallDone, stop)          // turns held on crew's own work
	go closeDoneLoop(root, s.Log.Printf, stop)           // crew that reported done, their turn over
	go sweepAsksLoop(root, s.Log.Printf, stop)           // asks nobody kept
	go watchLimitsLoop(root, s.Log.Printf, stop)         // steps out of time
	go watchWaitsLoop(root, s.Log.Printf, stop)          // Wait steps ws watches
	go sweepArtifactsLoop(root, s.Log.Printf, stop)      // artifacts on this machine that expired
	go sweepRunsLoop(root, s.Log.Printf, runsPoke, stop) // runs whose owner has gone, or whose time is up
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		sig := <-sigs
		s.Shutdown("signal " + sig.String())
	}()
	err = s.Serve()
	return s.Restarting(), err
}

// resumeInterrupted resumes the crew the last daemon shut down under that
// were still at their work, and logs what it did with each.
func resumeInterrupted(root string, logf func(string, ...any)) {
	ctx, err := LoadOps(root)
	if err != nil {
		logf("resuming crew the last daemon shut down under: %v", err)
		return
	}
	for _, r := range ctx.ResumeInterrupted() {
		logf("%s", r)
	}
}

func newDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether the daemon is running, and the crew it holds",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			c, err := daemon.DialAny(ctx.WS().Root)
			if err != nil {
				if !daemon.IsNoServer(err) {
					// Most often a sandbox, which closes sockets to a command.
					return fmt.Errorf("can't reach the daemon on %s (%w): a command in a sandbox can't, so run ws outside it", daemon.SocketPath(ctx.WS().Root), err)
				}
				fmt.Fprintf(ui.Stderr, "  %s no daemon running\n", ui.Dim.Render("·"))
				return nil
			}
			defer c.Close()
			if !c.Compatible() {
				fmt.Fprintf(ui.Stderr, "  %s a daemon from another ws version is running (pid %d); ws daemon restart --now starts it again on this ws\n", ui.Orange.Render("⚠"), c.ServerPid())
				return nil
			}
			crews, err := c.List()
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s daemon running, pid %d%s (%s)\n", ui.Green.Render("●"), c.ServerPid(), buildWords(c.Build()), daemon.SocketPath(ctx.WS().Root))
			if next, wanted := c.RestartWanted(); wanted {
				waits, _ := c.AtWork()
				for _, line := range restartLines(next, waits) {
					fmt.Fprintf(ui.Stderr, "    %s %s\n", ui.Orange.Render("↻"), line)
				}
			} else if exe := ownBinary(); c.Exe() != "" && c.Exe() != exe {
				fmt.Fprintf(ui.Stderr, "    %s it runs the ws at %s, not this one (%s, at %s)\n", ui.Dim.Render("·"), c.Exe(), cmd.Root().Version, exe)
			}
			if info, err := web.ReadInfo(ctx.WS().Root); err == nil && info.Pid == c.ServerPid() && info.Port != 0 {
				fmt.Fprintf(ui.Stderr, "    %s web UI on %s (ws web opens it)\n", ui.Dim.Render("·"), info.Origin())
			}
			if db, err := store.Dial(ctx.WS().Root); err != nil {
				fmt.Fprintf(ui.Stderr, "    %s records: %v\n", ui.Orange.Render("⚠"), err)
			} else {
				layouts, _ := db.Layouts(cmd.Context())
				size := int64(0)
				for _, f := range []string{"", "-wal"} {
					if info, err := os.Stat(store.Path(ctx.WS().Root) + f); err == nil {
						size += info.Size()
					}
				}
				fmt.Fprintf(ui.Stderr, "    %s records in %s, %s (%s)\n", ui.Dim.Render("·"), store.Path(ctx.WS().Root), humanBytes(size), layoutLine(layouts))
			}
			if idle, ok := c.IdleTimeout(); ok {
				fmt.Fprintf(ui.Stderr, "    %s %s\n", ui.Dim.Render("·"), idleLine(idle))
			}
			if len(crews) == 0 {
				fmt.Fprintf(ui.Stderr, "    %s no crew\n", ui.Dim.Render("·"))
			}
			for _, cr := range crews {
				fmt.Fprintf(ui.Stderr, "    %s %s %s in %s %s\n", ui.Green.Render("●"+cr.Spec.CrewID), cr.Spec.Slug, cr.Spec.Harness, cr.Spec.Capsule, ui.Dim.Render(fmt.Sprintf("pid %d", cr.Pid)))
			}
			return nil
		},
	}
}

// buildWords is a daemon's ws as status says it after its pid: none for a
// daemon from a ws before it said.
func buildWords(build string) string {
	if build == "" {
		return ""
	}
	return ", ws " + build
}

// restartLines say that a daemon is to restart, onto which ws if it found
// another in its place, and what it's waiting for.
func restartLines(next string, waits []string) []string {
	first := "it's to restart once nothing's at work"
	if next != "" {
		first = "ws " + next + " is installed: it restarts onto it once nothing's at work"
	}
	if len(waits) == 0 {
		return []string{first + ", which is now"}
	}
	lines := []string{first + " (ws daemon restart --now doesn't wait)"}
	for _, w := range waits {
		lines = append(lines, "waiting for "+w)
	}
	return lines
}

// yoursToDo refuses what's for whoever runs the workspace when crew or the
// orchestrator ask, saying why it isn't theirs: an agent that's hit
// trouble reaches for the daemon, and stopping it hangs every crew up.
func yoursToDo(what, why string) error {
	if a := ops.ActorFromEnv(); !a.IsYou() {
		return fmt.Errorf("%s is for whoever runs the workspace, not %s: %s. Say what you ran into instead (ws mission ask)", what, a.By(), why)
	}
	return nil
}

func newDaemonRestartCmd() *cobra.Command {
	var now bool
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Start the daemon again on the ws that's installed, once nothing's at work",
		Long: `Starts the daemon again on the ws now at its path: the one an upgrade
installed, which it doesn't run until it's restarted. It keeps its
environment and its web UI's link.

It waits until nothing's at work (no crew mid-turn, no agent's run), and
says what it's waiting for. Crew that are waiting for you are hung up and
reopened as they were. With --now it doesn't wait: crew mid-turn are hung
up too, and told to carry on once they're resumed.

A daemon does this by itself when it finds another ws in its place.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if err := yoursToDo("restarting the daemon", "it hangs up every crew, you among them"); err != nil {
				return err
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			root := ctx.WS().Root
			c, err := daemon.DialAny(root)
			if err != nil {
				if !daemon.IsNoServer(err) {
					return err
				}
				fmt.Fprintf(ui.Stderr, "  %s no daemon running: it starts on this ws when it's next needed\n", ui.Dim.Render("·"))
				return nil
			}
			defer c.Close()
			if c.Compatible() {
				waits, err := c.Restart(now)
				switch {
				case err == nil && len(waits) > 0:
					for _, line := range restartLines("", waits) {
						fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Orange.Render("↻"), line)
					}
					return nil
				case err == nil:
					<-c.Lost()
					return sayRestarted(root)
				case !daemon.IsUnknownOp(err):
					return err
				}
			}
			// A daemon from a ws before it could restart itself, or that
			// speaks another protocol: it's stopped, and started from this ws.
			if !now {
				return fmt.Errorf("the daemon running (pid %d) is from a ws before it could restart itself once nothing's at work: ws daemon restart --now restarts it at once, hanging up the crew it holds (those at their work are resumed)", c.ServerPid())
			}
			if err := c.Shutdown(); err != nil {
				return err
			}
			<-c.Lost()
			nc, err := daemon.Connect(root)
			if err != nil {
				return err
			}
			nc.Close()
			return sayRestarted(root)
		},
	}
	cmd.Flags().BoolVar(&now, "now", false, "don't wait for crew mid-turn or an agent's run: they're hung up, and resumed")
	return cmd
}

// sayRestarted waits for the daemon that's restarting to answer, and says
// which ws it's on.
func sayRestarted(root string) error {
	var c *daemon.Client
	var err error
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if c, err = daemon.DialAny(root); err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("the daemon shut down to restart, and hasn't come back (see %s): %w", daemon.LogPath(root), err)
	}
	defer c.Close()
	fmt.Fprintf(ui.Stderr, "  %s daemon restarted, pid %d%s\n", ui.Green.Render("✓"), c.ServerPid(), buildWords(c.Build()))
	return nil
}

func newDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop every crew the daemon holds, and the daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if err := yoursToDo("stopping the daemon", "it hangs up every crew, you among them"); err != nil {
				return err
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			c, err := daemon.DialAny(ctx.WS().Root)
			if err != nil {
				if !daemon.IsNoServer(err) {
					return err
				}
				fmt.Fprintf(ui.Stderr, "  %s no daemon running\n", ui.Dim.Render("·"))
				return nil
			}
			defer c.Close()
			var crews []daemon.CrewInfo
			if c.Compatible() {
				crews, _ = c.List()
			}
			if err := c.Shutdown(); err != nil {
				return err
			}
			<-c.Lost()
			fmt.Fprintf(ui.Stderr, "  %s daemon stopped (%d crew hung up)\n", ui.Green.Render("✓"), len(crews))
			return nil
		},
	}
}

// humanBytes is a size as it's said: 812 KB, 3.4 MB.
func humanBytes(n int64) string {
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%d bytes", n)
	case n < 1<<20:
		return fmt.Sprintf("%d KB", n>>10)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
}

// idleLine is when a daemon with the given idle timeout exits: when it's
// closed, if it has none (the default), or once it's been idle that long.
func idleLine(timeout time.Duration) string {
	if timeout <= 0 {
		return "it stays open until it's closed (ws daemon stop)"
	}
	return "it exits after " + inWords(timeout) + " idle"
}

// inWords is a while as it's said, to the minute: a minute, 45 minutes,
// an hour, 2 hours.
func inWords(d time.Duration) string {
	d = d.Round(time.Minute)
	n, unit := int(d/time.Minute), "minute"
	if d >= time.Hour && d%time.Hour == 0 {
		n, unit = int(d/time.Hour), "hour"
	}
	switch {
	case n == 1 && unit == "hour":
		return "an hour"
	case n == 1:
		return "a minute"
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// layoutLine is the records' parts and their layouts: "store 1, space 2".
func layoutLine(layouts map[string]int) string {
	var parts []string
	for _, name := range slices.Sorted(maps.Keys(layouts)) {
		parts = append(parts, fmt.Sprintf("%s %d", name, layouts[name]))
	}
	return "layouts: " + strings.Join(parts, ", ")
}
