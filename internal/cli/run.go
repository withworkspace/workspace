package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
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
	cmd := &cobra.Command{
		Use:   "run [flags] [<recipe> | -- <command>]",
		Short: "Run a dev server or the like on a free port, ended for you when its owner goes",
		Long: `ws run starts a command where you stand (a capsule, .ground or .silo) on a
free port, and ends it for you: when its owner goes, or when its time is
up, whichever is first. Nobody has to remember to stop it.

{port} in the command is replaced with the port, and $PORT is set too:

  ws run -- pnpm dev --port {port}
  ws run --name api --for mission --ttl 8 -- go run ./cmd/api

A repo can say how it's run, in its ws.repo.toml, and ws run <name> runs
that recipe ({port} as above; ready, a path it's up once it answers;
stop, run where it ran once it's ended). ws run alone lists them:

  [run.web]
  command = "pnpm dev --port {port} --strictPort"
  ready = "/"

  [run.stack]
  command = "docker compose up"
  stop = "docker compose down"

A recipe runs once in a worktree: asked for where it's running already,
ws run gives you that run, and it's then yours as well as whoever
started it, ended only once all of you have gone.

A recipe can say what it needs running first, and how it's told where:

  [run.web.needs.api]
  run = "backend/server"   # a recipe of another repo; "server" is this repo's
  env = "API_URL"          # its address, in the run's environment
  mount = "/api"           # and below the run's own name (ws router)

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
			ask := ops.RunAsk{Command: commandOf(args)}
			// forHere is where its owner is asked after: where it runs,
			// which for a silo's recipe is the silo.
			forHere := here
			if recipe, ok := recipes[args[0]]; ok && len(args) == 1 && cmd.ArgsLenAtDash() < 0 {
				if strings.TrimSpace(recipe.Command) == "" {
					return fmt.Errorf("[run.%s] in %s has no command", args[0], recipeFileOf(here))
				}
				ask = ops.RunAsk{Recipe: args[0]}
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
	cmd.Flags().DurationVar(&wait, "wait", time.Minute, "how long to wait for it, and each run it needs, to be up (0: don't)")
	cmd.Flags().StringArrayVar(&withs, "with", nil, "what meets one of the recipe's needs, in place of the nearest: <need>=<capsule>, ground, silo, or an address (https://…)")
	cmd.AddCommand(newRunLsCmd(), newRunStopCmd(), newRunLogsCmd(), newRunKeepCmd(), newRunPointCmd())
	return cmd
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
		var old *daemon.Incompatible
		if errors.As(err, &old) {
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
	for i := len(m.Stages) - 1; i >= 0; i-- {
		st := m.Stages[i]
		if st.Accepted == nil && slices.Contains(st.Crew, crew) {
			return st, true
		}
	}
	return mission.StageRecord{}, false
}

// runOwners says whether owners in the workspace's records have ended, for
// a sweep: each mission read once.
type runOwners struct {
	root     string
	missions map[string]*mission.Mission
	missing  map[string]bool
}

func newRunOwners(root string) *runOwners {
	return &runOwners{root: root, missions: map[string]*mission.Mission{}, missing: map[string]bool{}}
}

// mission is the mission slug, read once: gone is true when it isn't
// there at all, and m is nil (gone false) when it couldn't be read.
func (o *runOwners) mission(slug string) (m *mission.Mission, gone bool) {
	if m, ok := o.missions[slug]; ok {
		return m, false
	}
	if o.missing[slug] {
		return nil, true
	}
	m, err := mission.Store{Root: o.root}.Load(slug)
	if errors.Is(err, mission.ErrNotFound) {
		o.missing[slug] = true
		return nil, true
	}
	if err != nil {
		return nil, false
	}
	o.missions[slug] = m
	return m, false
}

func (o *runOwners) Ended(ow runs.Owner) string {
	if ow.Mission != "" {
		m, gone := o.mission(ow.Mission)
		if gone {
			return fmt.Sprintf("mission %s has gone", ow.Mission)
		}
		if m == nil {
			return ""
		}
		if m.Ending != nil || m.State == mission.Archived {
			return fmt.Sprintf("mission %s has ended", ow.Mission)
		}
		switch ow.Kind {
		case runs.ByStep:
			i := slices.IndexFunc(m.Stages, func(st mission.StageRecord) bool { return st.Text == ow.StepText })
			switch {
			case i < 0:
				return fmt.Sprintf("step %d is no longer in mission %s's plan", ow.Step, ow.Mission)
			case m.Stages[i].Skipped:
				return fmt.Sprintf("step %d of mission %s was skipped", m.Stages[i].N, ow.Mission)
			case m.Stages[i].Accepted != nil:
				return fmt.Sprintf("step %d of mission %s has passed", m.Stages[i].N, ow.Mission)
			}
		case runs.ByCrew:
			i := slices.IndexFunc(m.Crew, func(c mission.Crew) bool { return c.ID == ow.Crew })
			if i < 0 || m.Crew[i].Ended != nil {
				return fmt.Sprintf("crew %s of mission %s have closed", ow.Crew, ow.Mission)
			}
		}
	}
	if ow.Kind == runs.ByConversation {
		c, err := conversationStore(o.root, ow.Mission).Load(ow.Conversation)
		switch {
		case errors.Is(err, conversation.ErrNotFound):
			return fmt.Sprintf("conversation %s has gone", ow.Conversation)
		case err != nil:
			return ""
		case c.Status == conversation.Resolved:
			return fmt.Sprintf("conversation %s was resolved", ow.Conversation)
		}
	}
	return ""
}

// runRecipes is the [run.*] of the ws.repo.toml where you stand, or at
// the workspace's root, of its ws.toml.
func runRecipes(w *ops.Workspace, here runHere) (map[string]config.RunRecipe, error) {
	if here.repo == "" {
		return w.Config().Run, nil
	}
	top := filepath.Join(w.WS().RepoDir(here.repo), here.worktree)
	cfg, err := config.ParseRepoConfig(filepath.Join(top, config.RepoFileName))
	if err != nil || cfg == nil {
		return nil, err
	}
	return cfg.Run, nil
}

// listRecipes says what ws run <name> can run where you stand.
func listRecipes(here runHere, recipes map[string]config.RunRecipe) error {
	if len(recipes) == 0 {
		if here.repo == "" {
			fmt.Fprintf(os.Stderr, "The workspace has no recipes of its own ([run.<name>] in %s): ws run -- <command> runs one by hand.\n", config.FileName)
			return nil
		}
		fmt.Fprintf(os.Stderr, "%s has no recipes ([run.<name>] in its %s): ws run -- <command> runs one by hand.\n", here.repo, config.RepoFileName)
		return nil
	}
	needy := false
	for _, rc := range recipes {
		needy = needy || len(rc.Needs) > 0
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if needy {
		fmt.Fprintln(tw, "RECIPE\tCOMMAND\tNEEDS")
	} else {
		fmt.Fprintln(tw, "RECIPE\tCOMMAND")
	}
	for _, n := range slices.Sorted(maps.Keys(recipes)) {
		rc := recipes[n]
		if !needy {
			fmt.Fprintf(tw, "%s\t%s\n", n, rc.Command)
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
		fmt.Fprintf(tw, "%s\t%s\t%s\n", n, rc.Command, strings.Join(needs, ", "))
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
			swept, err := store.Sweep(newRunOwners(root), time.Now())
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
	Ref     string     `json:"ref"`
	Name    string     `json:"name"`
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

func writeRunsJSON(out io.Writer, root string, list []runs.Run) error {
	now := time.Now()
	rows := make([]runJSON, 0, len(list))
	for _, r := range list {
		j := runJSON{
			Ref: r.Ref(), Name: r.Name, Repo: r.Repo, Where: runs.Label(r.Worktree), Dir: r.Dir,
			Command: r.Command, Port: r.Port, Link: router.RunAddress(root, r), Local: r.Link(), State: string(r.State),
			Owner: r.Owner, Also: r.Also, Needs: r.Needs, NeededBy: r.NeededBy, By: r.By, Log: r.LogPath(),
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
		rows = append(rows, j)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
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
				fmt.Fprintf(os.Stderr, "Stopped %s.\n", r.Ref())
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
				swept, _ := store.Sweep(newRunOwners(w.WS().Root), time.Now())
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
					io.Copy(os.Stdout, f)
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
