package daemon

import (
	"fmt"
	"path/filepath"

	"github.com/withworkspace/workspace/internal/nav"
	"github.com/withworkspace/workspace/internal/router"
	"github.com/withworkspace/workspace/internal/web"
)

// startWeb runs the web UI inside the daemon. A web UI that can't start
// leaves the daemon running without one.
func (d *daemon) startWeb() *web.Server {
	w := d.newWeb()
	if err := w.Start(); err != nil {
		d.s.Log.Printf("no web UI: %v", err)
		return nil
	}
	return w
}

// newWeb is the web UI for the daemon's workspace, with everything it asks
// of ws given to it: web.Server knows nothing of the workspace, the
// missions' verbs or crew's terminals but what's set here. A hook left
// unset is an endpoint that says "not found" for good, so there's a test
// that none is.
func (d *daemon) newWeb() *web.Server {
	root, app := d.cfg.Root, d.w
	return &web.Server{
		Root: root, Port: app.Config().Web.Port, Log: d.s.Log, App: app, Hold: d.s.Hold, Shutdown: d.s.Shutdown,
		Version: d.cfg.Build,
		Reviews: app.PRService(), Team: app.Team(), Find: app.FindIndex(),
		Nav:        nav.New(NavCache(root)),
		Plugins:    d.plugins,
		Labels:     d.cfg.Labels,
		Names:      func() []string { return webNames(root) },
		Silos:      d.siloStates,
		ResyncSilo: d.resyncSilo,
		Show:       d.showView(),
	}
}

// webNames are the names the web UI is reached at through ws's router:
// <workspace>.localhost, with the router's port where it isn't 80.
func webNames(root string) []string {
	name, ok := router.NameOf(root)
	if !ok {
		return nil
	}
	host := router.Host("", name)
	return []string{host, fmt.Sprintf("%s:%d", host, router.Fallback), host + ":80"}
}

// NavCache is where code navigation keeps what it's parsed.
func NavCache(root string) string {
	return filepath.Join(root, ".ws", "cache", "nav")
}
