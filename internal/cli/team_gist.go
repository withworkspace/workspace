package cli

import (
	"time"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/team"
)

// What an agent writes for the Team page: a change's gist, and what's
// worth knowing of it. It's given the change in its prompt (title,
// description, files, diff) and no tools, replies in a form ws reads, and
// its session isn't kept.

// gistTurnFor is how long writing one gist may take.
const gistTurnFor = 2 * time.Minute

// teamWriter has an agent write a gist. It reads the config each time, so
// changing [team] takes effect on the next one.
func teamWriter(root string) func(team.Ask) (string, error) {
	return func(ask team.Ask) (string, error) {
		ctx, err := LoadOps(root)
		if err != nil {
			return "", err
		}
		pick, h, err := ops.Roles(ctx.Config()).ForTeam()
		if err != nil {
			return "", err
		}
		run := harness.ConverseRun{Prompt: team.GistPrompt(ask, ctx.WS().DisplayNameFor), Dir: root, Model: pick.Model, Alone: true, Once: true}
		rep, err := ops.RunHeadless(root, "team gist "+ask.Subject.Strand, crew.Env(root, "", ""), "", gistTurnFor, h, run)
		if err != nil {
			return "", err
		}
		return rep.Output, nil
	}
}

// teamSummariser has an agent write a period's account, as teamWriter
// has one write a gist: no tools, no session kept.
func teamSummariser(root string) func(team.SummaryAsk) (string, error) {
	return func(ask team.SummaryAsk) (string, error) {
		ctx, err := LoadOps(root)
		if err != nil {
			return "", err
		}
		pick, h, err := ops.Roles(ctx.Config()).ForTeam()
		if err != nil {
			return "", err
		}
		run := harness.ConverseRun{Prompt: team.SummaryPrompt(ask, ctx.WS().DisplayNameFor), Dir: root, Model: pick.Model, Alone: true, Once: true}
		rep, err := ops.RunHeadless(root, "team account since "+ask.Since.Format("2006-01-02"), crew.Env(root, "", ""), "", gistTurnFor, h, run)
		if err != nil {
			return "", err
		}
		return rep.Output, nil
	}
}
