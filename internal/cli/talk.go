package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// Talking with an agent about the workspace, before any mission: thinking
// something through, reading code, comparing options. Each conversation is
// w1, w2, … in the workspace's records, lives in the web UI, and its agent
// reads anything and changes nothing of the workspace's (where its shell
// is sandboxed, it runs things too); a run per turn resumes its session.

// talkStore is the conversations a command means: a mission's (-m), or
// the workspace's.
func talkStore(ctx *ops.Workspace, slug string) (conversation.Store, error) {
	if slug == "" {
		return conversation.ForWorkspace(ctx.WS().Root), nil
	}
	m, err := resolveMission(ctx, slug)
	if err != nil {
		return conversation.Store{}, err
	}
	return conversation.For(ctx.WS().Root, m.Slug), nil
}

// talkPath is a conversation's page in the web UI, or with no id, the list
// it's in.
func talkPath(store conversation.Store, id string) string {
	if store.Workspace() {
		if id == "" {
			return "/conversations"
		}
		return "/conversations/" + id
	}
	if id == "" {
		return "/missions/" + store.Slug + "?tab=talk"
	}
	return "/missions/" + store.Slug + "/conversations/" + id
}

func newTalkCmd() *cobra.Command {
	var printOnly bool
	var slug string
	cmd := &cobra.Command{
		Use:   "talk [what you're thinking about]",
		Short: "Think something through with an agent, in the web UI",
		Long: `Start a conversation with an agent about the workspace: a problem, an
idea, a question about the code. It reads anything here (the repos, the
missions, GitHub) and changes nothing of the workspace's: what's to be
kept, it proposes. On Claude Code it runs things too, in a sandbox, and
lifts capsules of the conversation's own to change files in, which are
put away with it. The conversation is in the web UI, which this opens;
without a first message, it opens the list of them.

With -m, it's about that mission, on its Talk tab: its plan, what to do
next, what was decided.`,
		Args:   cobra.ArbitraryArgs,
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			store, err := talkStore(ctx, slug)
			if err != nil {
				return err
			}
			path := talkPath(store, "")
			if text := strings.TrimSpace(strings.Join(args, " ")); text != "" {
				c, err := ctx.StartTalk(doc.Scope{Mission: store.Slug}, text, ops.AsYou)
				if err != nil {
					return err
				}
				path = talkPath(store, c.ID)
				fmt.Fprintf(ui.Stderr, "  %s started %s\n", ui.Green.Render("✓"), c.ID)
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
				return fmt.Errorf("opening your browser: %w (ws talk --print prints the link)", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&printOnly, "print", false, "print the link instead of opening it")
	cmd.Flags().StringVarP(&slug, "mission", "m", "", "talk about this mission, on its Talk tab")
	return cmd
}

func newConversationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "conversation",
		Aliases: []string{"conversations"},
		Short:   "The workspace's conversations with an agent (ws talk starts one)",
		Long: `Conversations about the workspace: w1, w2, … Each is in the web UI (ws
talk opens it), and ws show w3 shows one, with its summary and latest
turns. With -m, a mission's: t1, t2, … on its Talk tab (numbered with
its threads).`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
	}
	cmd.AddCommand(newConversationListCmd(), newConversationAnswerCmd(), newConversationAllowedCmd(), newConversationStatusCmd(conversation.Resolved), newConversationStatusCmd(conversation.Open),
		newConversationSettleCmd(true), newConversationSettleCmd(false), newConversationKeepCmd())
	return cmd
}

func newConversationListCmd() *cobra.Command {
	var all bool
	var slug string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the workspace's conversations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			store, err := talkStore(ctx, slug)
			if err != nil {
				return err
			}
			list, err := store.List()
			if err != nil {
				return err
			}
			n := 0
			for i := len(list) - 1; i >= 0; i-- {
				c := list[i]
				if c.Kind != conversation.Talk || ((c.Status != conversation.Open || c.Ephemeral) && !all) {
					continue
				}
				n++
				fmt.Printf("%-4s %s\n", c.ID, talkLine(c))
			}
			if n == 0 {
				fmt.Fprintf(ui.Stderr, "  %s no conversations: ws talk \"…\" starts one\n", ui.Dim.Render("·"))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "resolved ones too, and asks nobody's kept")
	cmd.Flags().StringVarP(&slug, "mission", "m", "", "a mission's conversations (none: the workspace's)")
	return cmd
}

func newConversationStatusCmd(status string) *cobra.Command {
	var slug string
	use, short := "resolve <conversation>", "Put a conversation away: you're done with it"
	if status == conversation.Open {
		use, short = "reopen <conversation>", "Take a resolved conversation up again"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			store, err := talkStore(ctx, slug)
			if err != nil {
				return err
			}
			c, err := ctx.SetTalkStatus(doc.Scope{Mission: store.Slug}, args[0], status, ops.ActorFromEnv())
			if errors.Is(err, conversation.ErrNotYours) {
				return errors.New("only the user resolves or reopens a conversation")
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s is %s\n", ui.Green.Render("✓"), c.ID, c.Status)
			return nil
		},
	}
	cmd.Flags().StringVarP(&slug, "mission", "m", "", "a mission's conversation (none: the workspace's)")
	return cmd
}

func newConversationKeepCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "keep <conversation>",
		Short: "Keep an ask: listed with the rest, about its document, and not swept",
		Long: `An ask from a document's ask bar isn't kept unless you keep it: it isn't
listed, and it goes an hour after its last turn. Keeping it makes it a
conversation like any, in Conversations, about the document it was asked
of (as Keep as a conversation does in the web UI).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			store, err := talkStore(ctx, slug)
			if err != nil {
				return err
			}
			c, err := ctx.KeepAsk(doc.Scope{Mission: store.Slug}, args[0], ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s is kept\n", ui.Green.Render("✓"), c.ID)
			return nil
		},
	}
	cmd.Flags().StringVarP(&slug, "mission", "m", "", "a mission's conversation (none: the workspace's)")
	return cmd
}

// talkLine is a conversation in a line: its title (or how it began), where
// it stands, and its summary's start.
func talkLine(c *conversation.Conversation) string {
	name := c.Title
	if name == "" && len(c.Turns) > 0 {
		name = "“" + clipLine(c.Turns[0].Body, 60) + "”"
	}
	state := c.Status
	switch {
	case c.Answering && c.Status == conversation.Open:
		state = "the agent's answering"
	case c.WaitingOnYou():
		state = "the agent replied"
	}
	if c.Ephemeral {
		state = "an ask nobody's kept, " + state
	}
	line := name + " (" + state + ")"
	if c.Summary != "" {
		line += ": " + clipLine(c.Summary, 100)
	}
	return line
}

// newConversationAllowedCmd is Claude Code's PreToolUse hook in a
// conversation's run: it says to allow a shell command the conversation's
// list allows, as ws reads it (a proposal's HTML on stdin, which Claude
// Code's own rules refuse), and what a sandboxed run may run or write
// besides; to refuse, saying what to do, a ws or gh command that isn't
// allowed; and nothing for anything else, which its rules then decide.
// See harness.GuardAnswer.
func newConversationAllowedCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "allowed <allow-list file>",
		Short:  "Say whether a conversation's agent may run a command (Claude Code runs this)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// A proposal's HTML is in the command: up to 1 MB, and its flags.
			event, err := io.ReadAll(io.LimitReader(os.Stdin, 8<<20))
			if err != nil {
				return nil // nothing said: Claude Code's rules decide
			}
			// Where its tools may write is asked as it stands now: a
			// capsule its agent lifted a moment ago is there.
			folders := func() []string {
				ctx, err := loadOps()
				if err != nil {
					return nil
				}
				return ctx.ConversationFolders(os.Getenv(mission.EnvMission), agentConversation())
			}
			if answer := harness.GuardAnswer(event, args[0], folders); answer != "" {
				fmt.Println(answer)
			}
			return nil
		},
	}
}

// talkSpawner starts the agent answering a conversation; tests replace it.
var talkSpawner = spawnAnswer(job.Answer)

func newConversationAnswerCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:    "answer <conversation>",
		Short:  "Have the agent answer a conversation (ws runs this itself)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			return ctx.AnswerConversation(doc.Scope{Mission: slug}, args[0])
		},
	}
	cmd.Flags().StringVarP(&slug, "mission", "m", "", "the mission whose conversation it is (none: the workspace's)")
	return cmd
}
