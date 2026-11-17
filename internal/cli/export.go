package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws export: a mission written out as the folder of files it was before
// the records were kept in SQLite.
func newExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export <mission> [dir]",
		Short: "Write a mission out as files: its record, log, decisions, conversations and reviews",
		Long: `Write a mission out as a folder of files, to read without ws or hand to
someone: its folder as it is (the brief, files/, crew's output,
transcripts), and beside it what ws records of it as the files they were
once kept in: mission.toml, log.jsonl, decisions.jsonl, conversations/
(one .jsonl a thread or talk) and reviews/ (r1.json…, viewed.json).

dir is where (by default a folder named for the mission, here); it has to
be new or empty.`,
		Args:              cobra.RangeArgs(1, 2),
		PreRun:            func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, args[0])
			if err != nil {
				return err
			}
			dir := m.Slug
			if len(args) == 2 {
				dir = args[1]
			}
			if dir, err = filepath.Abs(dir); err != nil {
				return err
			}
			out, err := ctx.ExportMission(m.Slug, dir)
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s wrote %s to %s: its record, log and decisions, %d %s, %d %s, and %d %s from its folder\n",
				ui.Green.Render("✓"), out.Slug, out.Dir, out.Conversations, plural(out.Conversations, "conversation"), out.Reviews, plural(out.Reviews, "review"), out.Files, plural(out.Files, "file"))
			return nil
		},
	}
}
