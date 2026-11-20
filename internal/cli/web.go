package cli

import (
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/nav"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/router"
	"github.com/withworkspace/workspace/internal/team"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/web"
)

func newWebCmd() *cobra.Command {
	var printOnly bool
	cmd := &cobra.Command{
		Use:   "web [path]",
		Short: "Open the workspace's web UI in your browser",
		Long: `Open the workspace's web UI: missions, crew, reviews, conversations and
pages. It runs in the workspace's daemon (started if it isn't running), on
127.0.0.1 at a port of its own ([web] port in ws.toml or ws.local.toml
picks another).

The link signs your browser in, so open it with this command rather than
typing the address. A path opens that page, such as /missions/<slug>.`,
		Args: cobra.MaximumNArgs(1),
		// Usage is for wrong arguments, not for what went wrong after.
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/"
			if len(args) == 1 {
				path = "/" + strings.TrimPrefix(args[0], "/")
			}
			return openWeb(path, printOnly)
		},
	}
	addPrintFlag(cmd, &printOnly)
	return cmd
}

func addPrintFlag(cmd *cobra.Command, printOnly *bool) {
	cmd.Flags().BoolVar(printOnly, "print", false, "print the sign-in link instead of opening it")
}

// openWeb opens path in the workspace's web UI, or prints its link.
func openWeb(path string, printOnly bool) error {
	ctx, err := loadOps()
	if err != nil {
		return err
	}
	url, err := webURL(ctx.WS().Root, path)
	if err != nil {
		return err
	}
	if printOnly {
		fmt.Println(url)
		return nil
	}
	if err := openBrowser(url); err != nil {
		return fmt.Errorf("opening your browser: %w (ws --print prints the link)", err)
	}
	info, _ := web.ReadInfo(ctx.WS().Root)
	fmt.Fprintf(ui.Stderr, "  %s opened %s%s\n", ui.Green.Render("✓"), info.Origin(), path)
	return nil
}

// webURL is a link that signs the browser in to the workspace's web UI and
// opens path, starting the daemon if it isn't running.
func webURL(root, path string) (string, error) {
	c, err := daemon.Connect(root)
	if err != nil {
		return "", err
	}
	defer c.Close()
	if c.WebPort() == 0 {
		return "", noWebError(root, c)
	}
	// The server wrote web.json before it answered: it starts its web UI
	// first.
	info, err := web.ReadInfo(root)
	if err != nil {
		return "", err
	}
	if info.Pid != c.ServerPid() || info.Port != c.WebPort() {
		return "", fmt.Errorf("%s doesn't match the running daemon (pid %d, web UI on port %d); `ws daemon restart --now` and try again", web.InfoPath(root), c.ServerPid(), c.WebPort())
	}
	link := info.SignInURL(path)
	// With names on, it's the web UI's name: a browser signs in by name,
	// its cookie being the name's.
	if a, ok := router.Address(root, ""); ok {
		link = strings.TrimSuffix(a, "/") + strings.TrimPrefix(link, info.Origin())
	}
	return link, nil
}

// noWebError says why a running daemon has no web UI, and what
// restarting it costs.
func noWebError(root string, c *daemon.Client) error {
	crews, _ := c.List()
	restart := "it holds no crew, so `ws daemon stop` and then `ws web` loses nothing"
	if n := len(crews); n > 0 {
		restart = fmt.Sprintf("`ws daemon stop` and then `ws web` restarts it, stopping its %d crew (their pages resume them)", n)
	}
	return fmt.Errorf("the daemon running for this workspace (pid %d) has no web UI: it was started by a ws from before there was one, or its web UI couldn't start (see %s); %s",
		c.ServerPid(), daemon.LogPath(root), restart)
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Run()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Run()
	default:
		return exec.Command("xdg-open", url).Run()
	}
}

// startWeb runs the web UI inside the daemon. A web UI that can't
// start leaves the daemon running without one.
func startWeb(s *daemon.Server, prs *ops.PRCache, version string) *web.Server {
	w := newWeb(s, prs)
	w.Version = version
	if err := w.Start(); err != nil {
		s.Log.Printf("no web UI: %v", err)
		return nil
	}
	return w
}

// newWeb is the web UI for a daemon's workspace, with everything it
// asks of ws given to it: web.Server knows nothing of the workspace, the
// missions' verbs or crew's terminals but what's set here. A hook left
// unset is an endpoint that says "not found" for good, so there's a test
// that none is.
func newWeb(s *daemon.Server, prs *ops.PRCache) *web.Server {
	ctx, err := LoadOps(s.Root)
	port := 0
	var teamSvc *team.Service
	if err == nil {
		port = ctx.Config().Web.Port
		teamSvc = teamService(ctx)
	}
	app := openOps(s.Root, prs, func(o *ops.Options) { o.Team = teamSvc }, daemonWS(s))
	return &web.Server{
		Root: s.Root, Port: port, Log: s.Log, App: app, Hold: s.Hold, Shutdown: s.Shutdown,
		Reviews: app.PRService(), Team: teamSvc, Find: app.FindIndex(),
		Nav:     nav.New(navCache(s.Root)),
		Plugins: pluginHost,
		Labels:  commandLabels(newBuiltinRootCmd("")),
		Names:   func() []string { return webNames(s.Root) },
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

// navCache is where code navigation keeps what it's parsed.
func navCache(root string) string {
	return filepath.Join(root, ".ws", "cache", "nav")
}

// reviewPath is where the web UI reviews a mission's capsule.
func reviewPath(slug, capsule string) string {
	return "/missions/" + slug + "/review/" + capsule
}

// docPath is where the web UI shows a mission's document (an artifact, by
// its ref), to read and comment on.
func docPath(slug, ref string) string {
	return "/missions/" + slug + "/artifact?" + url.Values{"ref": {ref}}.Encode()
}
