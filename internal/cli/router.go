package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/router"
	"github.com/withworkspace/workspace/internal/runs"
)

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
