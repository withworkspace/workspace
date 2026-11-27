package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/withworkspace/workspace/internal/runs"
)

// cacheSaid is what's worth saying of a cached task once it has ended:
// that its pass couldn't be kept, and the files it changed that its
// outputs don't name.
func cacheSaid(r runs.Run) []string {
	var out []string
	if r.KeepFailed != "" {
		out = append(out, fmt.Sprintf("Its pass wasn't kept, so it's run again next time: %s", firstLineOf(r.KeepFailed)))
	}
	if len(r.Undeclared) > 0 {
		more := ""
		if len(r.Undeclared) >= 20 {
			more = " and more"
		}
		out = append(out, fmt.Sprintf("It changed %s%s, which its outputs don't name: a reuse won't put them back. Name them in outputs if it wrote them.", strings.Join(r.Undeclared, ", "), more))
	}
	return out
}

func newRunCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "The passes of cached tasks that are kept, to be reused",
		Long: `ws run cache lists the passes kept of cached tasks (those whose recipe names
its inputs): which run each is, how long it took, and what its outputs
come to. A pass is reused for a run of its task with the same inputs, in
any capsule. The workspace keeps 5 GB of them; past that, those least
lately used are dropped.

ws run cache clear drops them all, so every cached task is run again.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadOps()
			if err != nil {
				return err
			}
			store := runs.Store{Root: w.WS().Root}
			all := store.Cached()
			if len(all) == 0 {
				fmt.Println("Nothing's kept. A task whose recipe names its inputs is kept once it passes: ws run --help says how.")
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "PASS\tTOOK\tOUTPUTS\tUSED")
			var total int64
			for _, e := range all {
				total += e.Bytes
				files := fmt.Sprintf("%d files", e.Files)
				if e.Files == 1 {
					files = "1 file"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s, %s\t%s\n", e.From, runs.Short(e.Took), files, runs.SizeSaid(e.Bytes), e.Used.Local().Format("2 Jan 15:04"))
			}
			tw.Flush()
			fmt.Printf("\n%d kept, %s of %s.\n", len(all), runs.SizeSaid(total), runs.SizeSaid(runs.CacheMax))
			return nil
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "Drop every pass kept, so each cached task is run again",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadOps()
			if err != nil {
				return err
			}
			store := runs.Store{Root: w.WS().Root}
			n := len(store.Cached())
			if err := store.ClearCache(); err != nil {
				return err
			}
			fmt.Printf("Dropped %d.\n", n)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:    "keep <run folder>",
		Short:  "Keep a cached task's pass: what its wrapper runs as its command passes",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// What it says is written to the run's keep.log, and read back
			// as why the pass wasn't kept: just that.
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			store, err := runs.StoreOf(args[0])
			if err == nil {
				err = store.KeepPass(args[0], time.Now())
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return &ExitCodeError{Code: 1}
			}
			return nil
		},
	})
	return cmd
}
