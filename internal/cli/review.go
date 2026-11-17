package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

func newReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Reviews of a mission's capsules: read what the user asked for",
		Long: `The user reviews a mission capsule's commits in the web UI:
comments on lines, then a verdict. Once submitted, a review is the
mission's: crew read it with ws review show and act on it, and the
orchestrator decides what follows.

Crew answer each comment by its id: addressed (once committed), wontfix
(with why) or reply (an answer). The user resolves or reopens them, and
re-reviews what changed; a stage whose review asked for changes isn't
accepted until they approve.

A comment's kind says what to do with it: blocking (must change),
suggestion (a judgement call: take it unless you have a reason not to),
question (answer it), nit (do it if it's cheap).`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
	}
	cmd.AddCommand(newReviewShowCmd(), newReviewListCmd(),
		newReviewAnswerCmd(review.Address, "addressed <comment> [what you did]", "Say you've dealt with a review comment (commit first: it records the commit)"),
		newReviewAnswerCmd(review.Decline, "wontfix <comment> <why>", "Say a review comment won't be done, and why"),
		newReviewAnswerCmd(review.Answer, "reply <comment> <text>", "Answer a review comment (a question), or say something about it"),
	)
	return cmd
}

// newReviewAnswerCmd is how crew answer a comment of a submitted review: by
// its id (r1.c2), as $WS_CREW (or you).
func newReviewAnswerCmd(what review.Response, use, short string) *cobra.Command {
	var slug, commit string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			store := review.For(ctx.WS().Root, m.Slug)
			id, body := args[0], strings.Join(args[1:], " ")
			by := ops.AsYou
			if c := os.Getenv(mission.EnvCrew); c != "" {
				by = ops.AsCrew(c)
			}
			if what == review.Address && commit == "" {
				r, _, err := store.FindNote(id)
				if err != nil {
					return err
				}
				repo, name, _ := strings.Cut(r.Capsule, "/")
				dir := filepath.Join(ctx.WS().RepoDir(repo), name)
				commit = workspace.GitRevParse(dir, "HEAD")
				if out, err := git.Line(dir, "status", "--porcelain", "--untracked-files=no"); err == nil && out != "" {
					fmt.Fprintf(ui.Stderr, "  %s %s has uncommitted changes: commit them, or the user won't see them (a review is of commits)\n", ui.Orange.Render("⚠"), r.Capsule)
				}
			}
			r, err := ctx.Respond(ops.ReviewOf{Mission: m.Slug}, id, what, body, commit, by)
			if err != nil {
				return err
			}
			t := r.Tally()
			fmt.Fprintf(ui.Stderr, "  %s %s: %d of %d comments of %s answered\n", ui.Green.Render("✓"), id, t.Answered(), t.Total(), r.ID)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	if what == review.Address {
		cmd.Flags().StringVar(&commit, "commit", "", "the commit that deals with it (default: the capsule's latest)")
	}
	return cmd
}

func newReviewShowCmd() *cobra.Command {
	var slug, format string
	cmd := &cobra.Command{
		Use:   "show [review]",
		Short: "Print a review: its comments with the lines they're on, and what to do",
		Long: `Print a submitted review, by its id (r3), as markdown: the verdict and
summary, then each comment with the lines it's on as they were, and what
to do. Without an id, the latest review of the capsule you're in, or else
of the mission.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			store := review.For(ctx.WS().Root, m.Slug)
			var r *review.Review
			if len(args) == 1 {
				if r, err = store.Load(args[0]); err != nil {
					return err
				}
				if r.State == review.Draft {
					return fmt.Errorf("%s is a draft: it's the user's until they submit it", r.ID)
				}
			} else {
				capsule := ""
				cwd, _ := userDir()
				if repo, name, ok := workspace.DetectRepo(ctx.WS().Root, cwd); ok && m.HasCapsule(repo, name) {
					capsule = repo + "/" + name
				}
				if r, err = store.Latest(capsule); err == nil && r == nil && capsule != "" {
					r, err = store.Latest("")
				}
				if err != nil {
					return err
				}
				if r == nil {
					return errors.New("no reviews of mission " + m.Slug + " yet")
				}
			}
			_ = format // markdown reads well to people and agents alike
			fmt.Print(review.Render(r))
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format: text or llm (both markdown)")
	return cmd
}

func newReviewListCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a mission's reviews",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			all, err := review.For(ctx.WS().Root, m.Slug).List()
			if err != nil {
				return err
			}
			if len(all) == 0 {
				fmt.Fprintf(ui.Stderr, "  %s no reviews of %s yet: a capsule's Review in the web UI starts one\n", ui.Dim.Render("·"), m.Slug)
				return nil
			}
			for _, r := range all {
				what := strings.TrimPrefix(review.Title(r), "Review "+r.ID+" of "+r.Capsule+": ")
				when := ""
				if r.State == review.Draft {
					what = fmt.Sprintf("draft, %d comments", len(r.Comments))
				} else {
					when = r.Submitted.Local().Format("2006-01-02 15:04")
				}
				fmt.Printf("%-4s %-28s %-24s %s\n", r.ID, r.Capsule, what, when)
			}
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}
