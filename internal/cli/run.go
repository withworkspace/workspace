package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/router"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/workspace"
)

// ws run: something started for someone and ended for them (internal/runs).

func newRunCmd() *cobra.Command {
	var name, forKind string
	var ownerPID int
	var ttl float64
	var wait time.Duration
	var withs []string
	var task, fresh bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "run [flags] [<recipe> | -- <command>]",
		Short: "Run a dev server on a free port, ended for you when its owner goes, or a task (tests, lint) kept for what it came to",
		Long: `ws run starts a command where you stand (a capsule, .ground or .silo) on a
free port, and ends it for you: when its owner goes, or when its time is
up, whichever is first. Nobody has to remember to stop it.

{port} in the command is replaced with the port, and $PORT is set too:

  ws run -- pnpm dev --port {port}
  ws run --name api --for mission --ttl 8 -- go run ./cmd/api

A repo can say how it's run, in its ws.repo.toml, and ws run <name> runs
that recipe ({port} as above; ready, a path it's up once it answers;
stop, run where it ran once it's ended). ws run alone lists them:

  [run.service.web]
  command = "pnpm dev --port {port} --strictPort"
  ready = "/"

  [run.service.stack]
  command = "docker compose up"
  stop = "docker compose down"

A recipe runs once in a worktree: asked for where it's running already,
ws run gives you that run, and it's then yours as well as whoever
started it, ended only once all of you have gone.

A recipe can say what it needs running first, and how it's told where:

  [run.service.web]
  command = "pnpm dev --port {port} --strictPort"
  needs.api = { run = "backend/server", env = "API_URL", mount = "/api" }

run is a service of another repo ("server" is this repo's own), env the
variable its address is handed over in, and mount a path below the run's
own name that it answers at (ws router). A need with a lot to say can be
a table of its own, [run.service.web.needs.api].

ws run web then finds the server where it's nearest (the same worktree
for this repo's; for another repo's, the mission's capsule there, or its
ground), starts it if it isn't running, waits for it, and starts web with
its address ({api} and {api.port} in the command say it too). One run of
the server serves every run that needs it, and it's ended once none does.
--with api=<capsule>, ground, silo or an address (https://staging…) says
what meets a need in place of the nearest.

It waits until the run is up (something listens on the port, or its
recipe's ready path answers), then prints the link and when the run will
be ended. Its output is kept: ws run logs.

That's a service. A task is the other type of run: tests, lint, a build,
which ends by itself. A recipe under [run.task] is one, and --task says
it of a command:

  [run.task.test]
  command = "go test -coverprofile=coverage.out ./..."
  reports.coverage = { path = "coverage.out", format = "gocover" }   # a file it writes, kept with its result

  ws run test
  ws run --task -- pnpm lint

A task has no port. ws run prints what it prints, waits for it to end
(--wait 0 doesn't), and exits as it did. What it came to is kept: how it
exited, how long it took, all it printed, the commit it ran at, and its
reports, with the last 10 runs of the same task behind it. ws run show
<name> says it, and ws run show <name>#3 an earlier one. A task that's
running where it's asked for again is waited for, not started twice.

A task that hasn't ended after its timeout is stopped, and kept as
stopped: an hour, unless its recipe says (timeout = "10m") or --timeout
does. What's meant to be kept up is a service.

A task that names its inputs is cached: asked for again with the same
inputs as a run of it that passed, that pass is reused rather than run,
here or in any capsule of the workspace, and its outputs are put back:

  [run.task.build]
  command = "pnpm build"
  inputs = ["src/**", "package.json", "pnpm-lock.yaml"]
  outputs = ["dist"]
  env = ["NODE_ENV"]

inputs and outputs are paths from where it runs (a file, or a folder and
all that's in it) or patterns (src/**/*.ts). Its inputs are the files they
name, changed, new and ignored ones too (node_modules only where it's
named), its command, and the variables env names. Its outputs, and its
reports, are what a reuse puts back, once what they name there now is
removed. A task that changes files its outputs don't name is told so, as
a reuse won't put those back. Only a pass is kept, and only a task that
needs no running services is cached. --fresh runs it anyway, and keeps
this run in place of the pass; ws run cache lists what's kept.

A task can be run after others of the same worktree:

  [run.task.coverage]
  command = "pnpm test --coverage dist"
  inputs = ["test/**"]
  after = ["build"]

ws run coverage runs build first (or reuses it), and coverage once it has
passed: a build that fails is coverage's fail, and its command isn't run.
Where both are cached, build's key is part of coverage's, and the files
build's outputs name aren't read as coverage's inputs: build's key says
them. A reused coverage has build asked for too, so dist is put back.

Its owner (--for):
  pid       the process that started it: your terminal's shell, or the
            agent whose command it is (the nearest above ws that's been
            running a minute or more; --owner names another)
  capsule   the capsule it runs in, until it's burned or landed
  mission   the mission ($WS_MISSION, or the capsule's), until it ends
  step      the step of the mission's plan crew are on, until it passes
  crew      crew ($WS_CREW), until their session closes
  conversation
            the conversation ($WS_CONVERSATION), until it's resolved
  none      nobody: only its time, or ws run stop, ends it

The default is the step for crew (or their session, when they're on no
step), the conversation for a conversation's agent, and pid for everyone
else. Every run also has a time (--ttl, in hours, 4 unless
said; 0 is no limit, which crew can't have): ws run keep gives it more.

A run is named for its command's first word, or --name. If a running run
in the same place has that name, it gets the next free one (web-2). It's
said as <name>.<worktree>.<repo> everywhere: web.my-capsule.workspace.

The daemon ends runs: it looks every 15 seconds, and as it starts. A run
isn't the daemon's child, so restarting ws leaves it running.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true // what's wrong now is the run's, not how it was asked for
			w, err := loadOps()
			if err != nil {
				return err
			}
			here, err := whereRun(w)
			if err != nil {
				return err
			}
			recipes, err := runRecipes(w, here)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return listRecipes(here, recipes)
			}
			if _, ok := recipes[args[0]]; !ok && len(args) == 1 && cmd.ArgsLenAtDash() < 0 && typelessRecipe(w, here, args[0]) {
				return fmt.Errorf("[run.%s] in %s isn't read: a recipe is a service or a task, and is written as one, [run.service.%s] or [run.task.%s]", args[0], recipeFileOf(here), args[0], args[0])
			}
			ask := ops.RunAsk{Command: commandOf(args)}
			if task {
				ask.Type = runs.Task
			}
			// forHere is where its owner is asked after: where it runs,
			// which for a silo's recipe is the silo.
			forHere := here
			if recipe, ok := recipes[args[0]]; ok && len(args) == 1 && cmd.ArgsLenAtDash() < 0 {
				if strings.TrimSpace(recipe.Command) == "" {
					return fmt.Errorf("%s in %s has no command", recipe.Table(args[0]), recipeFileOf(here))
				}
				if task {
					return fmt.Errorf("--task is for a command: %s is a recipe, which is a %s by where it's written (%s)", args[0], recipe.Type, recipe.Table(args[0]))
				}
				ask = ops.RunAsk{Recipe: args[0], Fresh: fresh}
				if fresh && !recipe.Cached() {
					return fmt.Errorf("--fresh runs a cached task when a pass is kept: %s names no inputs, so it's always run", args[0])
				}
				if recipe.Where == "silo" {
					silo := w.WS().SiloWorktree(here.repo)
					if _, err := os.Stat(silo); err != nil {
						return fmt.Errorf("%s runs in %s's silo, and it has none: ws silo point %s <capsule> makes one", args[0], here.repo, here.repo)
					}
					forHere = runHere{repo: here.repo, worktree: workspace.SiloDir, dir: silo}
				}
			}
			with, err := withOf(withs)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("owner") {
				if forKind != "" && forKind != string(runs.ByPID) {
					return fmt.Errorf("--owner names a process: it's --for pid, not --for %s", forKind)
				}
				forKind = string(runs.ByPID)
			}
			crew := os.Getenv(mission.EnvCrew)
			if ttl < 0 {
				return errors.New("--ttl is a number of hours, 0 or more")
			}
			if ttl == 0 && crew != "" {
				return errors.New("crew's runs need an end: --ttl takes hours, more than 0")
			}
			owner, err := runOwner(w, forHere, runs.Kind(forKind), ownerPID)
			if err != nil {
				return err
			}
			store := runs.Store{Root: w.WS().Root}
			// The daemon first: it ends runs, and serves the names that
			// what this one needs is reached by.
			watchErr := ensureRunsWatched(w.WS().Root)
			ask.Name, ask.Repo, ask.Worktree, ask.Dir = name, here.repo, here.worktree, here.dir
			ask.Owner, ask.TTL, ask.By = owner, time.Duration(ttl*float64(time.Hour)), ops.ActorFromEnv().By()
			ask.With, ask.Wait, ask.Mission = with, wait, os.Getenv(mission.EnvMission)
			// A task's time is its timeout: --timeout, or --ttl where that
			// was said, over its recipe's.
			isTask := task || recipes[ask.Recipe].IsTask()
			switch {
			case cmd.Flags().Changed("timeout") && !isTask:
				return errors.New("--timeout is a task's, which is stopped if it hasn't ended by then: a service has --ttl")
			case cmd.Flags().Changed("timeout") && cmd.Flags().Changed("ttl"):
				return errors.New("--timeout and --ttl both say how long a task may take: say one")
			case cmd.Flags().Changed("timeout") && timeout <= 0:
				return errors.New("--timeout is how long the task may take (10m, 1h30m), more than 0")
			case cmd.Flags().Changed("timeout"):
				ask.Timeout = timeout
			case isTask && cmd.Flags().Changed("ttl") && ttl == 0:
				ask.Timeout = -1 // no limit, as --ttl 0 says of any run
			case isTask && cmd.Flags().Changed("ttl"):
				ask.Timeout = ask.TTL
			}
			up, err := w.StartRun(ask)
			var ended *runs.EndedError
			if errors.As(err, &ended) {
				if tail := ended.Run.Tail(20); tail != "" {
					fmt.Fprintf(os.Stderr, "%s\n\n", tail)
				}
				return fmt.Errorf("%w: its log is above, and ws run logs %s has it all", err, ended.Run.Ref())
			}
			if err != nil {
				return err
			}
			run := up.Run
			switch want := runs.Slug(name); {
			case !up.Started:
				fmt.Fprintf(os.Stderr, "%s is running here already (started %s by %s): it's yours as well now.\n", run.Name, run.Created.Local().Format("15:04"), run.By)
			case want != "" && run.Name != want:
				fmt.Fprintf(os.Stderr, "%s is running already: this one is %s.\n", want, run.Name)
			case want == "" && ask.Recipe == "" && run.Name != runs.NameFor(run.Command):
				fmt.Fprintf(os.Stderr, "%s is running already: this one is %s.\n", runs.NameFor(run.Command), run.Name)
			}
			for _, n := range up.Needs {
				if n.Started {
					fmt.Fprintf(os.Stderr, "Started %s for %s%s.\n", n.Run, n.Name, whereSaid(n))
				}
			}
			for _, note := range up.Notes {
				fmt.Fprintln(os.Stderr, note)
			}
			if run.IsTask() {
				if watchErr != nil {
					fmt.Fprintf(os.Stderr, "%v\n\n", watchErr)
				}
				limit := time.Duration(0) // until it ends
				if cmd.Flags().Changed("wait") {
					if limit = wait; wait == 0 {
						fmt.Fprintf(os.Stdout, "name   %s\nlog    %s\n\nws run show %s says what it came to.\n", run.NRef(), run.LogPath(), run.Ref())
						return nil
					}
				}
				cmd.SilenceErrors = true // it exits as the task did, with what the task said
				return followTask(store, run, limit)
			}
			if wait > 0 {
				ready, err := store.Wait(run, wait)
				if errors.As(err, &ended) {
					if tail := ended.Run.Tail(20); tail != "" {
						fmt.Fprintf(os.Stderr, "%s\n\n", tail)
					}
					return fmt.Errorf("%w: its log is above, and ws run logs %s has it all", err, run.Ref())
				}
				if err != nil {
					return err
				}
				if !ready {
					fmt.Fprintf(os.Stderr, "%s is running, but wasn't up on %d in %s: ws run logs %s says what it's doing.\n\n", run.Ref(), run.Port, wait, run.Ref())
				}
			}
			out := os.Stdout
			link := router.RunAddress(w.WS().Root, run)
			fmt.Fprintf(out, "name   %s\nlink   %s\n", run.Ref(), link)
			if link != run.Link() {
				fmt.Fprintf(out, "local  %s\n", run.Link())
			}
			fmt.Fprintf(out, "port   %d\nlog    %s\n", run.Port, run.LogPath())
			for _, n := range up.Needs {
				fmt.Fprintf(out, "needs  %s\n", needSaid(n.Need, link, link != run.Link()))
			}
			// As List has it: with who needs it, which says when it ends.
			if now, err := store.Find(run.Ref(), run.Repo, run.Worktree); err == nil {
				run = now
			}
			fmt.Fprintf(out, "\nIt ends %s. ws run stop %s ends it now.\n", runs.Ends(run, time.Now()), run.Ref())
			if watchErr != nil {
				fmt.Fprintf(os.Stderr, "\n%v\n", watchErr)
			}
			return nil
		},
	}
	cmd.Flags().SetInterspersed(false) // what follows the command is the command's
	cmd.Flags().StringVarP(&name, "name", "n", "", "what it's called (default: its command's first word)")
	cmd.Flags().StringVar(&forKind, "for", "", "who it's for: pid, capsule, mission, step, crew, conversation or none (default: crew's step, a conversation, else pid)")
	cmd.Flags().IntVar(&ownerPID, "owner", 0, "the process it lasts as long as (with --for pid)")
	cmd.Flags().Float64Var(&ttl, "ttl", runs.DefaultTTL.Hours(), "how long it may last, in hours (0: no limit)")
	cmd.Flags().DurationVar(&wait, "wait", time.Minute, "how long to wait for it, and each run it needs, to be up (0: don't); for a task, how long to wait for it to end (unsaid: until it does)")
	cmd.Flags().DurationVar(&timeout, "timeout", runs.TaskTimeout, "for a task: how long it may take before it's stopped (10m, 1h30m), over its recipe's timeout")
	cmd.Flags().BoolVar(&task, "task", false, "the command ends by itself (tests, lint, a build): no port, and what it came to is kept")
	cmd.Flags().BoolVar(&fresh, "fresh", false, "run a cached task even where a pass with the same inputs is kept, and keep this one in its place")
	cmd.Flags().StringArrayVar(&withs, "with", nil, "what meets one of the recipe's needs, in place of the nearest: <need>=<capsule>, ground, silo, or an address (https://…)")
	cmd.AddCommand(newRunLsCmd(), newRunShowCmd(), newRunStopCmd(), newRunLogsCmd(), newRunKeepCmd(), newRunPointCmd(), newRunCacheCmd())
	return cmd
}

// followTask prints what a task prints until it ends, or limit has passed
// (0: no limit), then says what it came to and exits as it did. Asked to
// stop (ctrl-c), it stops the task, which is kept as stopped.
func followTask(store runs.Store, run runs.Run, limit time.Duration) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	type end struct {
		run   runs.Run
		ended bool
		err   error
	}
	done := make(chan end, 1)
	go func() {
		now, ended, err := store.WaitEnd(run, limit, func(s string) { _, _ = io.WriteString(os.Stdout, s) })
		done <- end{now, ended, err}
	}()
	var e end
	select {
	case e = <-done:
	case <-stop:
		if now, err := store.Find(run.Ref(), run.Repo, run.Worktree); err == nil && now.State == runs.Running {
			if err := store.End(now); err != nil {
				fmt.Fprintf(os.Stderr, "\nCouldn't stop %s: %v\n", run.NRef(), err)
			}
		}
		fmt.Fprintf(os.Stderr, "\nStopped %s.\n", run.NRef())
		return &ExitCodeError{Code: 130}
	}
	if e.err != nil {
		return e.err
	}
	if !e.ended {
		fmt.Fprintf(os.Stderr, "\n%s is still running after %s: ws run logs -f %s follows it, and ws run show %s says what it came to.\n", run.NRef(), limit, run.Ref(), run.Ref())
		return nil
	}
	switch {
	case e.run.Reused != nil:
		fmt.Fprintf(os.Stderr, "\n%s passed: %s. --fresh runs it.\n", e.run.NRef(), ops.ReusedSaid(e.run))
		if len(e.run.AfterRan) > 0 {
			fmt.Fprintf(os.Stderr, "What it's run after was asked for too, to put its outputs back: %s.\n", strings.Join(e.run.AfterRan, ", "))
		}
	case e.run.Result() == "stopped" && e.run.Why != "":
		fmt.Fprintf(os.Stderr, "\n%s stopped%s: %s.\n", e.run.NRef(), tookSaid(e.run), e.run.Why)
	default:
		fmt.Fprintf(os.Stderr, "\n%s %s%s.\n", e.run.NRef(), e.run.Result(), tookSaid(e.run))
	}
	if files := reportsKept(e.run); len(files) > 0 {
		fmt.Fprintf(os.Stderr, "Kept with it: %s.\n", strings.Join(files, ", "))
	}
	for _, line := range cacheSaid(e.run) {
		fmt.Fprintln(os.Stderr, line)
	}
	for _, rp := range e.run.Reports {
		if len(e.run.ReportFiles(rp.Name)) == 0 {
			fmt.Fprintf(os.Stderr, "It wrote no %s report (%s).\n", rp.Name, rp.Path)
		}
	}
	switch {
	case e.run.Passed():
		return nil
	case e.run.State == runs.Exited && e.run.Status != 0:
		return &ExitCodeError{Code: e.run.Status}
	}
	return &ExitCodeError{Code: 1}
}

// tookSaid is how long an ended run took, to follow what it came to: " in
// 42s". A reused run wasn't run, so took nothing to say.
func tookSaid(r runs.Run) string {
	if r.Reused != nil {
		return ", reused"
	}
	t := r.Took()
	switch {
	case t <= 0:
		return ""
	case t < 10*time.Second:
		return " in " + t.Round(100*time.Millisecond).String()
	}
	return " in " + t.Round(time.Second).String()
}

// reportsKept is a task's reports that were kept, each as name (n files)
// or its one file's name.
func reportsKept(r runs.Run) []string {
	var out []string
	for _, rp := range r.Reports {
		switch files := r.ReportFiles(rp.Name); len(files) {
		case 0:
		case 1:
			out = append(out, rp.Name+" ("+filepath.Base(files[0])+")")
		default:
			out = append(out, fmt.Sprintf("%s (%d files)", rp.Name, len(files)))
		}
	}
	return out
}

// withOf is --with's pairs, by need: api=ground, api=https://staging….
func withOf(said []string) (map[string]string, error) {
	if len(said) == 0 {
		return nil, nil
	}
	out := map[string]string{}
	for _, s := range said {
		need, what, ok := strings.Cut(s, "=")
		need, what = strings.TrimSpace(need), strings.TrimSpace(what)
		if !ok || need == "" || what == "" {
			return nil, fmt.Errorf("--with is <need>=<capsule>, ground, silo or an address, not %s", s)
		}
		out[need] = what
	}
	return out, nil
}

// whereSaid says how a need's run was chosen, when it's worth saying: in
// another repo, ws picked the worktree.
func whereSaid(n ops.NeedUp) string {
	if n.Where == "" || n.Where == "as you said" {
		return ""
	}
	return fmt.Sprintf(" (%s: --with %s=<capsule> runs another)", n.Where, n.Name)
}

// needSaid is one of a run's needs on a line: its name, what meets it,
// the address the run was given, and where it's mounted. link is the
// run's own, by name when named is true: a mount is only there then, and
// is said as its path alone otherwise.
func needSaid(n runs.Need, link string, named bool) string {
	if n.IsFolder() {
		line := fmt.Sprintf("%s  %s/%s  %s", n.Name, n.Repo, runs.Label(n.Worktree), n.Dir)
		if n.Env != "" {
			line += "  $" + n.Env
		}
		return line
	}
	what := n.Run
	if what == "" {
		what = "outside ws"
	}
	line := fmt.Sprintf("%s  %s  %s", n.Name, what, n.Addr)
	if n.Env != "" {
		line += "  $" + n.Env
	}
	if n.Mount != "" {
		if named {
			line += "  at " + strings.TrimRight(link, "/") + n.Mount
		} else {
			line += "  at " + n.Mount
		}
	}
	return line
}

// ensureRunsWatched has the workspace's daemon up, as it's what ends runs.
// Tests replace it.
var ensureRunsWatched = func(root string) error {
	c, err := daemon.Connect(root)
	if err != nil {
		if old, ok := errors.AsType[*daemon.Incompatible](err); ok {
			return fmt.Errorf("The daemon here is another ws (pid %d): it won't end this run by itself until it's restarted onto this one (ws daemon restart). ws run ls ends what's due meanwhile.", old.Pid)
		}
		return fmt.Errorf("The daemon didn't start, so nothing will end this run by itself: %v. ws run ls ends what's due.", err)
	}
	c.Close()
	return nil
}

// runHere is where a run starts: the repo and worktree you stand in, and
// the folder.
type runHere struct{ repo, worktree, dir string }

func whereRun(w *ops.Workspace) (runHere, error) {
	cwd, err := userDir()
	if err != nil {
		return runHere{}, err
	}
	root := w.WS().Root
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	if c, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = c
	}
	repo, worktree, ok := workspace.DetectRepo(root, cwd)
	if ok && worktree != ".bare" {
		return runHere{repo: repo, worktree: worktree, dir: cwd}, nil
	}
	// Anywhere in the workspace but its repos is the workspace's own place:
	// its recipes are ws.toml's, and they run at its root.
	if rel, err := filepath.Rel(root, cwd); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		if rel != "repos" && !strings.HasPrefix(rel, "repos"+string(filepath.Separator)) {
			return runHere{dir: root}, nil
		}
	}
	return runHere{}, errors.New("a run starts in a capsule, .ground or .silo, or for the whole workspace at its root: stand in one")
}

// recipeFileOf is the file where you stand's recipes are said in.
func recipeFileOf(here runHere) string {
	if here.repo == "" {
		return config.FileName
	}
	return config.RepoFileName
}

// runOwner is who a run is for: kind, or the default for whoever's asking:
// crew's step (or with none, their session), a conversation's agent's
// conversation, else the process that asked.
func runOwner(w *ops.Workspace, here runHere, kind runs.Kind, pid int) (runs.Owner, error) {
	slug := os.Getenv(mission.EnvMission)
	crew := os.Getenv(mission.EnvCrew)
	talk := os.Getenv(conversation.EnvConversation)
	if kind == "" {
		switch {
		case crew != "" && slug != "":
			if o, err := runOwner(w, here, runs.ByStep, 0); err == nil {
				return o, nil
			}
			return runOwner(w, here, runs.ByCrew, 0)
		case talk != "":
			kind = runs.ByConversation
		default:
			kind = runs.ByPID
		}
	}
	switch kind {
	case runs.ByNone:
		return runs.Owner{Kind: runs.ByNone}, nil
	case runs.ByCapsule:
		if here.repo == "" {
			return runs.Owner{}, errors.New("the workspace's own runs aren't in a capsule: say --for pid, mission or none")
		}
		if strings.HasPrefix(here.worktree, ".") {
			return runs.Owner{}, fmt.Errorf("%s isn't a capsule, and is never burned: say --for pid, mission or none", here.worktree)
		}
		return runs.Owner{Kind: runs.ByCapsule}, nil
	case runs.ByMission:
		if slug == "" {
			m, ok := w.Missions.FindByCapsule(here.repo, here.worktree)
			if !ok {
				return runs.Owner{}, fmt.Errorf("%s/%s isn't a mission's capsule, and $%s isn't set: say --for pid, capsule or none", here.repo, here.worktree, mission.EnvMission)
			}
			slug = m.Slug
		}
		if _, err := w.Missions.Load(slug); err != nil {
			return runs.Owner{}, fmt.Errorf("mission %s: %w", slug, err)
		}
		return runs.Owner{Kind: runs.ByMission, Mission: slug}, nil
	case runs.ByStep, runs.ByCrew:
		if slug == "" || crew == "" {
			return runs.Owner{}, fmt.Errorf("--for %s is for crew: $%s and $%s say whose", kind, mission.EnvMission, mission.EnvCrew)
		}
		m, err := w.Missions.Load(slug)
		if err != nil {
			return runs.Owner{}, fmt.Errorf("mission %s: %w", slug, err)
		}
		if kind == runs.ByCrew {
			if !slices.ContainsFunc(m.Crew, func(c mission.Crew) bool { return c.ID == crew }) {
				return runs.Owner{}, fmt.Errorf("mission %s has no crew %s", slug, crew)
			}
			return runs.Owner{Kind: runs.ByCrew, Mission: slug, Crew: crew}, nil
		}
		st, ok := crewStep(m, crew)
		if !ok {
			return runs.Owner{}, fmt.Errorf("crew %s of mission %s are on no step that's still going: say --for crew or mission", crew, slug)
		}
		return runs.Owner{Kind: runs.ByStep, Mission: slug, Step: st.N, StepText: st.Text, Crew: crew}, nil
	case runs.ByConversation:
		if talk == "" {
			return runs.Owner{}, fmt.Errorf("--for conversation is for a conversation's agent: $%s says which", conversation.EnvConversation)
		}
		if _, err := conversationStore(w.WS().Root, slug).Load(talk); err != nil {
			return runs.Owner{}, fmt.Errorf("conversation %s: %w", talk, err)
		}
		return runs.Owner{Kind: runs.ByConversation, Mission: slug, Conversation: talk}, nil
	case runs.ByPID:
		procs, err := runs.ReadProcs()
		if err != nil {
			procs = runs.Procs{}
		}
		if pid > 0 {
			o, ok := runs.OwnerOf(procs, pid)
			if !ok {
				return runs.Owner{}, fmt.Errorf("--owner %d isn't a running process", pid)
			}
			return o, nil
		}
		o := runs.FindOwner(procs, os.Getppid(), time.Now())
		if o.Kind == "" {
			// Nothing above has been running long enough to be its owner
			// (or ws can't see): its time alone ends it.
			return runs.Owner{Kind: runs.ByNone}, nil
		}
		return o, nil
	}
	return runs.Owner{}, fmt.Errorf("--for is pid, capsule, mission, step, crew, conversation or none, not %s", kind)
}

// conversationStore is a mission's conversations, or with no mission, the
// workspace's.
func conversationStore(root, slug string) conversation.Store {
	if slug == "" {
		return conversation.ForWorkspace(root)
	}
	return conversation.For(root, slug)
}

// crewStep is the step crew are on: the latest of the mission's steps
// they're recorded on that hasn't passed.
func crewStep(m *mission.Mission, crew string) (mission.StageRecord, bool) {
	for _, st := range slices.Backward(m.Stages) {

		if st.Accepted == nil && slices.Contains(st.Crew, crew) {
			return st, true
		}
	}
	return mission.StageRecord{}, false
}

// runRecipes is the services and tasks of the ws.repo.toml where you
// stand, or at the workspace's root, of its ws.toml.
func runRecipes(w *ops.Workspace, here runHere) (map[string]config.RunRecipe, error) {
	if here.repo == "" {
		return w.Config().Run.Recipes(), nil
	}
	top := filepath.Join(w.WS().RepoDir(here.repo), here.worktree)
	cfg, err := config.ParseRepoConfig(filepath.Join(top, config.RepoFileName))
	if err != nil || cfg == nil {
		return nil, err
	}
	return cfg.Run.Recipes(), nil
}

// typelessRecipe is whether name is written where you stand as
// [run.<name>], in neither [run.service] nor [run.task]: a table ws doesn't
// read, which would otherwise be run as a command of that name.
func typelessRecipe(w *ops.Workspace, here runHere, name string) bool {
	unread := w.Config().Unknown
	if here.repo != "" {
		cfg, err := config.ParseRepoConfig(filepath.Join(w.WS().RepoDir(here.repo), here.worktree, config.RepoFileName))
		if err != nil || cfg == nil {
			return false
		}
		unread = cfg.Unknown
	}
	return slices.ContainsFunc(unread, func(entry string) bool {
		return entry == recipeFileOf(here)+": run."+name
	})
}

// listRecipes says what ws run <name> can run where you stand.
func listRecipes(here runHere, recipes map[string]config.RunRecipe) error {
	if len(recipes) == 0 {
		if here.repo == "" {
			fmt.Fprintf(os.Stderr, "The workspace has no recipes of its own ([run.service.<name>] or [run.task.<name>] in %s): ws run -- <command> runs one by hand.\n", config.FileName)
			return nil
		}
		fmt.Fprintf(os.Stderr, "%s has no recipes ([run.service.<name>] or [run.task.<name>] in its %s): ws run -- <command> runs one by hand.\n", here.repo, config.RepoFileName)
		return nil
	}
	needy, tasks := false, false
	for _, rc := range recipes {
		needy = needy || len(rc.Needs) > 0
		tasks = tasks || rc.IsTask()
	}
	// typeOf is a recipe's type in its own column, where any is a task.
	typeOf := func(rc config.RunRecipe) string {
		if !tasks {
			return ""
		}
		if rc.IsTask() {
			return "task\t"
		}
		return "service\t"
	}
	head := "RECIPE\t"
	if tasks {
		head += "TYPE\t"
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if needy {
		fmt.Fprintln(tw, head+"COMMAND\tNEEDS")
	} else {
		fmt.Fprintln(tw, head+"COMMAND")
	}
	for _, n := range slices.Sorted(maps.Keys(recipes)) {
		rc := recipes[n]
		if !needy {
			fmt.Fprintf(tw, "%s\t%s%s\n", n, typeOf(rc), rc.Command)
			continue
		}
		var needs []string
		for _, name := range slices.Sorted(maps.Keys(rc.Needs)) {
			if n := rc.Needs[name]; n.IsFolder() {
				needs = append(needs, name+" ("+strings.TrimSpace(n.Repo)+"'s folder)")
				continue
			}
			repo, recipe := rc.Needs[name].Recipe(name)
			if repo != "" {
				recipe = repo + "/" + recipe
			}
			if recipe == name {
				needs = append(needs, name)
			} else {
				needs = append(needs, name+" ("+recipe+")")
			}
		}
		fmt.Fprintf(tw, "%s\t%s%s\t%s\n", n, typeOf(rc), rc.Command, strings.Join(needs, ", "))
	}
	return tw.Flush()
}

func newRunLsCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List the workspace's runs (ending any whose time is up first)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadOps()
			if err != nil {
				return err
			}
			root := w.WS().Root
			store := runs.Store{Root: root}
			swept, err := w.SweepRuns(time.Now())
			for _, s := range swept {
				if s.Run.State == runs.Running {
					fmt.Fprintf(os.Stderr, "Ended %s: %s.\n", s.Run.Ref(), s.Why)
				}
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Not everything due was ended: %v\n", err)
			}
			list, err := store.List()
			if err != nil {
				return err
			}
			if asJSON {
				return writeRunsJSON(os.Stdout, root, list)
			}
			if len(list) == 0 {
				fmt.Fprintln(os.Stderr, "No runs.")
				return nil
			}
			now := time.Now()
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tPORT\tSTATE\tENDS\tLINK")
			for _, r := range list {
				state, ends := string(r.State), runs.Ends(r, now)
				if r.State == runs.Exited {
					state = fmt.Sprintf("exited %d, %s", r.Status, r.Ended.Local().Format("15:04"))
				}
				if r.State != runs.Running {
					ends = "-"
				}
				if r.IsTask() {
					// A task: what it came to, where a service has a port and a link.
					if r.State != runs.Running {
						state = fmt.Sprintf("%s%s, %s", r.Result(), tookSaid(r), r.Ended.Local().Format("15:04"))
					}
					fmt.Fprintf(tw, "%s\t-\t%s\t%s\t-\n", r.NRef(), state, ends)
					continue
				}
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n", r.Ref(), r.Port, state, ends, router.RunAddress(root, r))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "as JSON")
	return cmd
}

// runJSON is a run as ws run ls --json says it.
type runJSON struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
	// Type is service or task. A task has no port or link; it says which
	// run of it this is (N), what it came to (Result: running, passed,
	// failed (exit n) or stopped), how long it took in seconds, the commit
	// it ran at, and the reports kept with it.
	Type    string       `json:"type"`
	N       int          `json:"n,omitempty"`
	Result  string       `json:"result,omitempty"`
	Took    float64      `json:"took,omitempty"`
	Commit  string       `json:"commit,omitempty"`
	Dirty   bool         `json:"dirty,omitempty"`
	Why     string       `json:"why,omitempty"`
	Reports []reportJSON `json:"reports,omitempty"`
	// Reused is the pass it is, for a cached task that wasn't run; Kept
	// the key a pass of one was kept under, KeepFailed why it couldn't
	// be, and Undeclared the files it changed outside its outputs.
	Reused     *runs.Reused `json:"reused,omitempty"`
	Kept       string       `json:"kept,omitempty"`
	KeepFailed string       `json:"keep_failed,omitempty"`
	Undeclared []string     `json:"undeclared,omitempty"`
	// After are the runs of the tasks it was run after, by their numbers.
	After   []string   `json:"after,omitempty"`
	Repo    string     `json:"repo"`
	Where   string     `json:"where"`
	Dir     string     `json:"dir"`
	Command string     `json:"command"`
	Port    int        `json:"port"`
	Link    string     `json:"link"`
	Local   string     `json:"local"`
	State   string     `json:"state"`
	Status  *int       `json:"status,omitempty"`
	Owner   runs.Owner `json:"owner"`
	// Also are who else it's for; Needs what it was started needing; and
	// NeededBy the running runs that need it, which keep it.
	Also     []runs.Owner `json:"also,omitempty"`
	Needs    []runs.Need  `json:"needs,omitempty"`
	NeededBy []string     `json:"needed_by,omitempty"`
	Expires  *time.Time   `json:"expires,omitempty"`
	Ends     string       `json:"ends,omitempty"`
	By       string       `json:"by,omitempty"`
	Log      string       `json:"log"`
}

// reportJSON is one of a task's reports: what its recipe calls it, its
// format, and the files kept for it.
type reportJSON struct {
	Name   string   `json:"name"`
	Format string   `json:"format,omitempty"`
	Path   string   `json:"path"`
	Files  []string `json:"files"`
}

func runJSONOf(root string, r runs.Run, now time.Time) runJSON {
	j := runJSON{
		Ref: r.Ref(), Name: r.Name, Type: string(runs.Service), Repo: r.Repo, Where: runs.Label(r.Worktree), Dir: r.Dir,
		Command: r.Command, Port: r.Port, Link: router.RunAddress(root, r), Local: r.Link(), State: string(r.State),
		Owner: r.Owner, Also: r.Also, Needs: r.Needs, NeededBy: r.NeededBy, By: r.By, Log: r.LogPath(),
	}
	if r.IsTask() {
		j.Type, j.Ref, j.N, j.Link, j.Local = string(runs.Task), r.NRef(), r.N, "", ""
		j.Result, j.Took, j.Commit, j.Dirty, j.Why = r.Result(), r.Took().Round(time.Millisecond).Seconds(), r.Commit, r.Dirty, r.Why
		j.Reused, j.Kept, j.KeepFailed, j.Undeclared, j.After = r.Reused, r.Kept, r.KeepFailed, r.Undeclared, r.AfterRan
		for _, rp := range r.Reports {
			files := r.ReportFiles(rp.Name)
			if files == nil {
				files = []string{}
			}
			j.Reports = append(j.Reports, reportJSON{Name: rp.Name, Format: rp.Format, Path: rp.Path, Files: files})
		}
	}
	if r.State == runs.Exited {
		s := r.Status
		j.Status = &s
	}
	if !r.Expires.IsZero() {
		e := r.Expires
		j.Expires = &e
	}
	if r.State == runs.Running {
		j.Ends = runs.Ends(r, now)
	}
	return j
}

func writeRunsJSON(out io.Writer, root string, list []runs.Run) error {
	now := time.Now()
	rows := make([]runJSON, 0, len(list))
	for _, r := range list {
		rows = append(rows, runJSONOf(root, r, now))
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func newRunShowCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Say what a run came to: a task's result, the commit it ran at, its reports, and the runs before it",
		Long: `ws run show says where a run stands. For a task, that's what it came to:
passed or failed and how it exited, how long it took, the commit it ran
at, where all it printed is kept, the reports kept with it, and the
earlier runs of the same task behind it. <name>#3 is the third run of a
task, while it's kept.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadOps()
			if err != nil {
				return err
			}
			store, r, err := findRun(w, args[0])
			if err != nil {
				return err
			}
			root, now := w.WS().Root, time.Now()
			var past []runs.Run
			if r.IsTask() && !strings.Contains(args[0], "#") {
				past = store.Past(r)
			}
			if asJSON {
				out := struct {
					runJSON
					Past []runJSON `json:"past,omitempty"`
				}{runJSON: runJSONOf(root, r, now)}
				for _, p := range past {
					out.Past = append(out.Past, runJSONOf(root, p, now))
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			out := os.Stdout
			if !r.IsTask() {
				fmt.Fprintf(out, "name     %s\ntype     service\nstate    %s\nlink     %s\nport     %d\ncommand  %s\nlog      %s\n", r.Ref(), r.State, router.RunAddress(root, r), r.Port, r.Command, r.LogPath())
				if r.State == runs.Running {
					fmt.Fprintf(out, "\nIt ends %s.\n", runs.Ends(r, now))
				}
				return nil
			}
			fmt.Fprintf(out, "name     %s\ntype     task\nresult   %s%s\n", r.NRef(), r.Result(), tookSaid(r))
			if r.Why != "" {
				fmt.Fprintf(out, "why      %s\n", r.Why)
			}
			if r.State != runs.Running {
				fmt.Fprintf(out, "ended    %s\n", r.Ended.Local().Format("2 Jan 15:04"))
			}
			if r.Commit != "" {
				at := r.Commit
				if len(at) > 12 {
					at = at[:12]
				}
				if r.Dirty {
					at += ", with changes not committed"
				}
				fmt.Fprintf(out, "commit   %s\n", at)
			}
			if r.Reused != nil {
				fmt.Fprintf(out, "reused   %s\n", strings.TrimPrefix(ops.ReusedSaid(r), "reused "))
			}
			fmt.Fprintf(out, "command  %s\nlog      %s\n", r.Command, r.LogPath())
			if r.Key != "" {
				fmt.Fprintf(out, "outputs  %s\n", strings.Join(r.Outputs, " "))
			}
			if len(r.AfterRan) > 0 {
				fmt.Fprintf(out, "after    %s\n", strings.Join(r.AfterRan, " "))
			} else if len(r.After) > 0 {
				var names []string
				for _, a := range r.After {
					names = append(names, a.Recipe)
				}
				fmt.Fprintf(out, "after    %s\n", strings.Join(names, " "))
			}
			for _, rp := range r.Reports {
				files := r.ReportFiles(rp.Name)
				what := strings.Join(files, " ")
				if len(files) == 0 {
					what = "not written (" + rp.Path + ")"
					if r.State == runs.Running {
						what = "kept once it ends (" + rp.Path + ")"
					}
				}
				format := ""
				if rp.Format != "" {
					format = "  " + rp.Format
				}
				fmt.Fprintf(out, "report   %s%s  %s\n", rp.Name, format, what)
			}
			if lines := cacheSaid(r); len(lines) > 0 {
				fmt.Fprintf(out, "\n%s\n", strings.Join(lines, "\n"))
			}
			if len(past) > 0 {
				fmt.Fprintln(out, "\nBefore it:")
				tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
				for _, p := range past {
					at := p.Commit
					if len(at) > 12 {
						at = at[:12]
					}
					fmt.Fprintf(tw, "  #%d\t%s%s\t%s\t%s\n", p.N, p.Result(), tookSaid(p), p.Ended.Local().Format("2 Jan 15:04"), at)
				}
				tw.Flush()
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "as JSON")
	return cmd
}

// findRun is the run ref names, from where you stand.
func findRun(w *ops.Workspace, ref string) (runs.Store, runs.Run, error) {
	store := runs.Store{Root: w.WS().Root}
	here, _ := whereRun(w)
	r, err := store.Find(ref, here.repo, here.worktree)
	return store, r, err
}

func newRunStopCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "stop [name...]",
		Short: "End runs now: those named, or every one where you stand",
		Long: `ws run stop ends the runs named (as ws run ls says them, or by name alone
where you stand). With none named, it ends every run in the capsule you
stand in; with --all, every run in the workspace.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadOps()
			if err != nil {
				return err
			}
			store := runs.Store{Root: w.WS().Root}
			var which []runs.Run
			switch {
			case all && len(args) > 0:
				return errors.New("stop takes names or --all, not both")
			case len(args) > 0:
				for _, a := range args {
					_, r, err := findRun(w, a)
					if err != nil {
						return err
					}
					which = append(which, r)
				}
			default:
				list, err := store.List()
				if err != nil {
					return err
				}
				here, herr := whereRun(w)
				if !all && herr != nil {
					return errors.New("stand in a capsule to stop its runs, or name them, or say --all")
				}
				for _, r := range list {
					if all || (r.Repo == here.repo && r.Worktree == here.worktree) {
						which = append(which, r)
					}
				}
			}
			if len(which) == 0 {
				fmt.Fprintln(os.Stderr, "Nothing to stop.")
				return nil
			}
			var errs []error
			stopped := map[string]bool{}
			for _, r := range which {
				stopped[r.Ref()] = true
			}
			needs := false
			for _, r := range which {
				if err := store.End(r); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", r.Ref(), err))
					continue
				}
				switch {
				case r.IsTask() && r.State != runs.Running:
					fmt.Fprintf(os.Stderr, "Cleared %s, and the runs kept behind it.\n", r.Ref())
				case r.IsTask():
					fmt.Fprintf(os.Stderr, "Stopped %s: it's kept, as stopped (ws run stop %s again clears it).\n", r.NRef(), r.Ref())
				default:
					fmt.Fprintf(os.Stderr, "Stopped %s.\n", r.Ref())
				}
				needs = needs || len(r.Needs) > 0
				// What still runs and needed it is left without it.
				left := slices.DeleteFunc(slices.Clone(r.NeededBy), func(ref string) bool { return stopped[ref] })
				switch len(left) {
				case 0:
				case 1:
					fmt.Fprintf(os.Stderr, "%s needed it, and is still running: asking for that again (ws run) starts this again.\n", left[0])
				default:
					fmt.Fprintf(os.Stderr, "%s needed it, and are still running: asking for either again (ws run) starts this again.\n", strings.Join(left, " and "))
				}
			}
			// What was started for those, and nothing needs now, ends with
			// them, not at the daemon's next look.
			if needs {
				swept, _ := w.SweepRuns(time.Now())
				for _, s := range swept {
					if s.Run.State == runs.Running {
						fmt.Fprintf(os.Stderr, "Ended %s: %s.\n", s.Run.Ref(), s.Why)
					}
				}
			}
			return errors.Join(errs...)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "every run in the workspace")
	return cmd
}

func newRunLogsCmd() *cobra.Command {
	var follow bool
	var lines int
	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "Print a run's output (-f to follow it)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadOps()
			if err != nil {
				return err
			}
			store, r, err := findRun(w, args[0])
			if err != nil {
				return err
			}
			f, err := os.Open(r.LogPath())
			if err != nil {
				return err
			}
			defer f.Close()
			if lines > 0 {
				fmt.Fprintln(os.Stdout, r.Tail(lines))
				if _, err := f.Seek(0, io.SeekEnd); err != nil {
					return err
				}
			} else if _, err := io.Copy(os.Stdout, f); err != nil {
				return err
			}
			if !follow {
				return nil
			}
			for {
				if _, err := io.Copy(os.Stdout, f); err != nil {
					return err
				}
				now, err := store.Find(r.Ref(), r.Repo, r.Worktree)
				if err != nil || now.State != runs.Running {
					_, _ = io.Copy(os.Stdout, f) // to a terminal: one that's gone has nothing to be shown
					return nil
				}
				time.Sleep(250 * time.Millisecond)
			}
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "keep printing what it writes until it ends")
	cmd.Flags().IntVarP(&lines, "lines", "n", 0, "only the last n lines")
	return cmd
}

func newRunKeepCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "keep <name> [hours]",
		Short: "Give a run more time: hours from now (4 unless said; 0 for no limit)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			hours := runs.DefaultTTL.Hours()
			if len(args) == 2 {
				h, err := strconv.ParseFloat(args[1], 64)
				if err != nil || h < 0 {
					return fmt.Errorf("hours is a number, 0 or more, not %s", args[1])
				}
				hours = h
			}
			if hours == 0 && os.Getenv(mission.EnvCrew) != "" {
				return errors.New("crew's runs need an end: give it hours, more than 0")
			}
			w, err := loadOps()
			if err != nil {
				return err
			}
			store, r, err := findRun(w, args[0])
			if err != nil {
				return err
			}
			if r.State != runs.Running {
				return fmt.Errorf("%s has ended: ws run starts it again", r.Ref())
			}
			if strings.Contains(args[0], "#") {
				return fmt.Errorf("%s is a run that's kept, not one that's running", args[0])
			}
			r, err = store.Keep(r, time.Duration(hours*float64(time.Hour)), time.Now())
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "%s ends %s.\n", r.Ref(), runs.Ends(r, time.Now()))
			return nil
		},
	}
}

// commandOf is the command a run's words after -- say, as the shell it
// runs in reads it: one word is a command line of its own (ws run --
// 'pnpm build && pnpm preview'); several are each a word, quoted where the
// shell would otherwise split or read them (ws run -- sh -c 'echo hi').
func commandOf(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	words := make([]string, len(args))
	for i, a := range args {
		words[i] = a
		if a == "" || strings.ContainsFunc(a, func(r rune) bool { return !plainWord(r) }) {
			words[i] = shellQuote(a)
		}
	}
	return strings.Join(words, " ")
}

// plainWord is whether r needs no quoting in a shell word. { and } are
// kept plain, so {port} reads as written.
func plainWord(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./:=@%+,{}", r)
}
