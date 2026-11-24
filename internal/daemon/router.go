package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

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
			if err := router.Write(dir, RouteFile(root, web, local.Router.Name, dir)); err != nil {
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

// RouteFile is the workspace's routes as they are now: its web UI, its
// runs, and its silos.
func RouteFile(root string, web int, name, dir string) router.File {
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
	up := map[string]runs.Run{}
	for _, r := range list {
		if r.State == runs.Running {
			up[r.Ref()] = r
		}
	}
	for _, r := range list {
		key := router.RunKey(r.Name, r.Worktree, r.Repo)
		switch r.State {
		case runs.Running:
			rte := router.Route{Port: r.Port}
			// What it needs, where it's mounted below the run's own name.
			for _, n := range r.Needs {
				path := router.MountPath(n.Mount)
				if path == "" {
					continue
				}
				m := router.Mount{Path: path, Need: n.Name, Run: n.Run, Strip: n.Strip}
				if n.Run == "" {
					m.URL = n.Addr
				} else {
					m.Port = up[n.Run].Port
				}
				rte.Mounts = append(rte.Mounts, m)
			}
			f.Routes[key] = rte
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
			return cfg.Run.Recipes()
		}
	}
	return nil
}
