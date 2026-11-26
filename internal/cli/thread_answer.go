package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
)

// The thread agent: when you post a thread on a document (asking, or
// asking for a change) or reply in one, an agent answers it in the
// thread. Each thread has its own agent session, resumed turn by turn,
// that starts from ws show t3 and may only read: its final message is its
// reply.

// threadSpawner starts the thread agent for a thread; tests replace it.
var threadSpawner = spawnAnswer(job.Thread)

// draftSpawner starts the thread agent drafting what was decided in a
// thread you've resolved; tests replace it.
var draftSpawner = spawnAnswer(job.Draft)

func newThreadAnswerCmd() *cobra.Command {
	var slug string
	var draft bool
	cmd := &cobra.Command{
		Use:    "answer <thread>",
		Short:  "Have the thread agent answer a thread (ws runs this itself)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			if draft {
				return ctx.DraftDecision(m.Slug, args[0])
			}
			return ctx.AnswerConversation(doc.OfMission(m.Slug), args[0])
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().BoolVar(&draft, "draft", false, "draft what was decided in a resolved thread, instead")
	return cmd
}

// threadStore is the conversations a thread is in: its mission's, or (no
// mission) the workspace's.
func threadStore(ctx *ops.Workspace, m *mission.Mission) conversation.Store {
	if m == nil {
		return conversation.ForWorkspace(ctx.WS().Root)
	}
	return conversation.For(ctx.WS().Root, m.Slug)
}

func clipString(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
