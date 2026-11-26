package cli

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/router"
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
picks another). With ws's router on (ws router), it opens at the
workspace's name instead, such as http://<workspace>.localhost.

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
	link, err := webURL(ctx.WS().Root, path)
	if err != nil {
		return err
	}
	if printOnly {
		fmt.Println(link)
		return nil
	}
	if err := openBrowser(link); err != nil {
		return fmt.Errorf("opening your browser: %w (ws --print prints the link)", err)
	}
	fmt.Fprintf(ui.Stderr, "  %s opened %s%s\n", ui.Green.Render("✓"), linkOrigin(link), path)
	return nil
}

// webURL is a link that signs the browser in to the workspace's web UI and
// opens path, starting the daemon if it isn't running.
func webURL(root, path string) (string, error) {
	info, origin, err := webInfo(root)
	if err != nil {
		return "", err
	}
	// With names on, it's the web UI's name: a browser signs in by name,
	// its cookie being the name's.
	return origin + strings.TrimPrefix(info.SignInURL(path), info.Origin()), nil
}

// webInfo is the running web UI's Info, and where it's reached
// (webOrigin), starting the daemon if it isn't running.
func webInfo(root string) (web.Info, string, error) {
	c, err := daemon.Connect(root)
	if err != nil {
		return web.Info{}, "", err
	}
	defer c.Close()
	if c.WebPort() == 0 {
		return web.Info{}, "", noWebError(root, c)
	}
	// The server wrote web.json before it answered: it starts its web UI
	// first.
	info, err := web.ReadInfo(root)
	if err != nil {
		return web.Info{}, "", err
	}
	if info.Pid != c.ServerPid() || info.Port != c.WebPort() {
		return web.Info{}, "", fmt.Errorf("%s doesn't match the running daemon (pid %d, web UI on port %d); `ws daemon restart --now` and try again", web.InfoPath(root), c.ServerPid(), c.WebPort())
	}
	return info, webOrigin(root, info), nil
}

// webOrigin is where the web UI is reached: its name on ws's router when
// names are on and a router is serving them, else 127.0.0.1 and its port.
func webOrigin(root string, info web.Info) string {
	if a, ok := router.Address(root, ""); ok {
		return strings.TrimSuffix(a, "/")
	}
	return info.Origin()
}

// linkOrigin is link's scheme and host, for saying where it opened
// without its sign-in token.
func linkOrigin(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return link
	}
	return u.Scheme + "://" + u.Host
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
