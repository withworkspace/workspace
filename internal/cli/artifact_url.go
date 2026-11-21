package cli

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/web"
)

// ws artifact url a3.v2: the address a version runs at, as the web UI's
// frame runs it, which opens with no sign-in. It's for looking at one in
// a browser that isn't signed in to ws: an agent's (Playwright, Chrome),
// or one you send it to. The address opens that version's files and
// nothing else (its key is made from the workspace's token and the
// version), under the frame's policy: no network, no storage, sandboxed.

func newArtifactURLCmd() *cobra.Command {
	var sets []string
	cmd := &cobra.Command{
		Use:   "url <a3|a3.v2>",
		Short: "Print the address a version runs at, to open in any browser (the latest, for a3)",
		Long: `Print the address a version of an artifact runs at: a3.v2 for v2, or a3
for the latest. It's the page the web UI frames, as it runs there (its
files, ws.tweaks, no network, no storage), and it opens with no sign-in,
so a browser that isn't signed in to ws can look at it: an agent's
(Playwright, Chrome), or one you send it to. The address opens that
version's files and nothing else.

--set name=value sets one of its tweaks, as the tray beside the frame
would (again for each): the page opens with them set, the rest as its
first preset has them. ws artifact tweaks says what tweaks are.

  ws artifact url a3.v2 --set density=compact --set accent="#3a7"

The daemon serves it, so the address works while the daemon's running;
ws starts it if it isn't.`,
		Args:   cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			id, n, ok := artifact.ParseRef(args[0])
			if !ok {
				return fmt.Errorf("%q isn't an artifact: they're named a1, a2, … and a version a3.v2 (ws artifact list)", args[0])
			}
			state, err := tweakState(sets)
			if err != nil {
				return err
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			// Which version it is (the latest, for a3), and that there's one
			// to run: the frame would say it can't be shown otherwise.
			if _, _, n, err = ctx.Artifacts().ReadVersion(context.Background(), id, n); err != nil {
				return err
			}
			info, origin, err := webInfo(ctx.WS().Root)
			if err != nil {
				return err
			}
			address := origin + web.FramedPath(info.Token, "/frame/"+id+"/v"+strconv.Itoa(n))
			if q := state.Encode(); q != "" {
				address += "?" + q
			}
			fmt.Println(address)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&sets, "set", nil, "set one of its tweaks, name=value, as the tray would (again for each)")
	return cmd
}

// tweakState is what --set says, as the frame's address says it:
// t.density=compact for each.
func tweakState(sets []string) (url.Values, error) {
	q := url.Values{}
	for _, s := range sets {
		name, value, ok := strings.Cut(s, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("--set %q: say a tweak and its value, as --set density=compact", s)
		}
		q.Set("t."+name, value)
	}
	return q, nil
}
