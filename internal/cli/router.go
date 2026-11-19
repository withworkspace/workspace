package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/router"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/workspace"
)

// The daemon's part in the names (internal/router), while ws.local.toml's
// [router] enabled says so: it writes the workspace's route file every
// publishEvery, and tries to be the machine's router. It reads the setting
// again each time, so turning it on or off needs no restart.

// publishEvery is how often the daemon writes its route file again.
const publishEvery = 2 * time.Second

func routerLoop(root string, web int, logf func(string, ...any), stop <-chan struct{}) {
	dir := router.Dir()
	var stopRouter context.CancelFunc
	off := func() {
		if stopRouter != nil {
			stopRouter()
			stopRouter = nil
			logf("router: names off")
		}
		router.Remove(dir, root)
	}
	defer off()
	t := time.NewTicker(publishEvery)
	defer t.Stop()
	for {
		local, err := config.ReadLocal(root)
		switch {
		case err != nil:
			// A file mid-edit: as it was, until it reads.
		case !local.Router.Enabled:
			off()
		default:
			if err := router.Write(dir, routeFile(root, web, local.Router.Name, dir)); err != nil {
				logf("router: couldn't write the route file: %v", err)
			}
			if stopRouter == nil {
				ctx, cancel := context.WithCancel(context.Background())
				stopRouter = cancel
				go (&router.Router{Dir: dir, Logf: logf}).Run(ctx)
			}
		}
		select {
		case <-stop:
			return
		case <-t.C:
		}
	}
}

// routeFile is the workspace's routes as they are now: its web UI, its
// runs, and its silos.
func routeFile(root string, web int, name, dir string) router.File {
	now := time.Now()
	f := router.File{
		PID: os.Getpid(), Root: filepath.Clean(root), Name: router.NameFor(root, name),
		Created: router.Created(dir, root, now), Web: web, Routes: map[string]router.Route{},
	}
	procs, err := runs.ReadProcs()
	if err == nil {
		if pr, ok := procs.ByPID[f.PID]; ok {
			f.Began = pr.Start
		}
	}
	f.Given = router.Give(dir, f, procs)
	list, _ := runs.Store{Root: root}.List()
	for _, r := range list {
		key := router.RunKey(r.Name, r.Worktree, r.Repo)
		switch r.State {
		case runs.Running:
			f.Routes[key] = router.Route{Port: r.Port}
		case runs.Exited:
			f.Routes[key] = router.Route{Ended: fmt.Sprintf("exited (%d) at %s", r.Status, r.Ended.Local().Format("15:04"))}
		default:
			f.Routes[key] = router.Route{Ended: "was ended outside ws"}
		}
	}
	if local, err := config.ReadLocal(root); err == nil {
		for repo, target := range local.Silo {
			s := router.Silo{Target: target}
			for n, rc := range siloRecipes(root, repo) {
				if rc.Where == "silo" {
					s.Recipes = append(s.Recipes, n)
				}
			}
			sort.Strings(s.Recipes)
			if f.Silos == nil {
				f.Silos = map[string]router.Silo{}
			}
			f.Silos[repo] = s
		}
	}
	return f
}

// siloRecipes is the recipes of repo's silo: as its ws.repo.toml says, or
// with no silo yet, as its .ground's does.
func siloRecipes(root, repo string) map[string]config.RunRecipe {
	for _, wt := range []string{workspace.SiloDir, workspace.GroundDir} {
		cfg, err := config.ParseRepoConfig(filepath.Join(root, "repos", repo, wt, config.RepoFileName))
		if err == nil && cfg != nil {
			return cfg.Run
		}
	}
	return nil
}

func newRouterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "router",
		Short: "Where the workspace's names are served (<name>.localhost), and what they are",
		Long: `With [router] enabled = true in ws.local.toml, the workspace's daemon gives
it and its runs names under .localhost:

  <workspace>.localhost                         its web UI
  <run>.<capsule>.<repo>.<workspace>.localhost  a run (ws run)
  <run>.silo.<repo>.<workspace>.localhost       a run in a repo's silo

Browsers send any *.localhost to this machine by themselves. One router
serves every workspace on the machine: whichever daemon gets its port (80,
or 7080 where 80 can't be had) serves them all, and another takes over if
it goes. [router] name = "…" names the workspace; it's its folder's
otherwise, and where two ask for one name, the later has four letters of
its own after it.

ws router says whether names are on here, where they're served, and this
workspace's names.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadOps()
			if err != nil {
				return err
			}
			root := w.WS().Root
			local, err := config.ReadLocal(root)
			if err != nil {
				return err
			}
			out := os.Stdout
			port := router.Serving()
			switch {
			case port == 0:
				fmt.Fprintln(out, "router   none running")
			case port == router.Fallback:
				fmt.Fprintf(out, "router   on :%d (80 is another program's, or kept by the system)\n", port)
			default:
				fmt.Fprintf(out, "router   on :%d\n", port)
			}
			if h, ok := router.WhoIs(port); ok && port > 0 {
				fmt.Fprintf(out, "held by  pid %d\n", h.PID)
			}
			if !local.Router.Enabled {
				fmt.Fprintf(out, "\nThis workspace has no names: [router] enabled = true in %s gives it some.\n", config.LocalFileName)
				return nil
			}
			name, ok := router.NameOf(root)
			if !ok {
				fmt.Fprintln(out, "\nIts daemon hasn't written its routes yet: it does once it's running (ws web starts it).")
				return nil
			}
			if want := router.NameFor(root, local.Router.Name); want != name {
				fmt.Fprintf(out, "\n%s was another workspace's when this one's names began: it's %s while its daemon runs.\n", want, name)
			}
			if port == 0 {
				fmt.Fprintln(out, "\nNo router is serving: its daemon becomes it once it's up.")
				return nil
			}
			fmt.Fprintln(out)
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintf(tw, "web UI\t%s\n", router.Link(router.Host("", name), port))
			list, _ := runs.Store{Root: root}.List()
			for _, r := range list {
				if r.State == runs.Running {
					fmt.Fprintf(tw, "%s\t%s\n", r.Ref(), router.RunAddress(root, r))
				}
			}
			return tw.Flush()
		},
	}
}
