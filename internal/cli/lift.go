package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

func newLiftCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lift <repo> <capsule-name> [base]",
		Short: "Create a new capsule",
		Long: `Create a new branch and worktree for fresh work. Use "." as the repo to
infer from the current directory. Base defaults to origin/<default-branch>.

Run by a conversation's agent, the capsule is that conversation's, to try
things in: it joins no mission, stays out of your editor, and is put away
when the conversation is (its branch kept, so lifting it again brings it
back). Naming one that's there already, lifted by hand and no mission's,
has the conversation work in it: it's left where it is when the
conversation's put away.

Examples:
  ws lift frontend my-feature
  ws lift . my-feature
  ws lift frontend my-feature develop`,
		Args: cobra.RangeArgs(2, 3),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return completeRepoNames(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}

			repo, err := resolveRepo(ctx, args[0])
			if err != nil {
				return err
			}
			if err := ensureCloned(ctx.WS(), repo); err != nil {
				return err
			}

			branch := args[1]

			base := "origin/" + ctx.WS().DefaultBranch
			if len(args) == 3 {
				base = args[2]
			}

			if id := agentConversation(); id != "" {
				return liftForConversation(ctx, id, repo, branch, base)
			}
			c, err := ctx.Lift(repo, branch, base, capsuleOptions(ctx))
			return madeCapsule(ctx, c, err, "Lift off!")
		},
	}
}

// liftForConversation lifts a capsule for the conversation the agent
// running this is answering, and says where it is and whose.
func liftForConversation(ctx *ops.Workspace, id, repo, branch, base string) error {
	c, err := ctx.LiftForConversation(os.Getenv(mission.EnvMission), id, repo, branch, base, ops.CapsuleOptions{Show: showCapsuleMaking, By: ops.ActorFromEnv()})
	return madeForConversation(c, err, id)
}

// madeForConversation says where a capsule lifted, docked or joined for a
// conversation is, and whose, and prints the cd the shell's ws runs.
func madeForConversation(c ops.Capsule, err error, id string) error {
	if err != nil {
		return err
	}
	if c.Joined {
		fmt.Fprintf(ui.Stderr, "Joined %s/%s, which was there already, at %s: %s works in it now, to change, build, commit and, when you're asked, push. It's left where it is when the conversation's put away.\n", c.Repo, c.Name, c.Path, id)
		fmt.Printf("cd %s\n", shellQuote(c.Path))
		return nil
	}
	if c.HookErr != nil {
		fmt.Fprintf(ui.Stderr, "  %s after_create hook failed: %v\n", ui.Orange.Render("⚠"), c.HookErr)
	}
	fmt.Fprintf(ui.Stderr, "\n%s/%s is lifted for %s, at %s: yours to change, build and commit in. It's put away with the conversation, its branch kept.\n", c.Repo, c.Name, id, c.Path)
	// As ws lift always ends: the shell's ws (a function, in the agent's
	// shell as in yours) runs what's printed, and goes there.
	fmt.Printf("cd %s\n", shellQuote(c.Path))
	return nil
}
