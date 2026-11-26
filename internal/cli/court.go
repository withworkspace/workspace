package cli

import (
	"fmt"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/ui"
)

// Where a mission's ball is (mission.Resolve) is worked out from the same
// snapshot everywhere.

// styledLabel is a headline's label coloured for its court.
func styledLabel(h mission.Headline) string {
	switch h.Kind {
	case mission.KindDeciding:
		return ui.Blue.Render(h.Label)
	case mission.KindLaunching, mission.KindParked, mission.KindArchived:
		return ui.Dim.Render(h.Label)
	case mission.KindMerge, mission.KindLand, mission.KindWrapUp, mission.KindAccept, mission.KindApprove, mission.KindLanded:
		return ui.Green.Render(h.Label)
	case mission.KindReady:
		return ui.Blue.Render(h.Label)
	}
	switch h.Court {
	case mission.CourtAgent:
		return ui.Green.Render(h.Label)
	case mission.CourtOutside:
		return ui.Blue.Render(h.Label)
	}
	return ui.Orange.Render(h.Label)
}

// missionSnapshot is what the court reads of a mission.
func missionSnapshot(ctx *ops.Workspace, v ops.MissionView, brief string, events []mission.Event) mission.Snapshot {
	return ops.MissionSnapshot(v, brief, events, ops.ReReviews(ctx.WS(), v.M), ops.DocThreadCourt(ctx.WS(), v, brief), !ctx.Config().OrchestratorEnabled())
}

// conclusionLines is an ending, suggested or made, under its summary: Z
// as checked, part by part.
func conclusionLines(c mission.Conclusion) []string {
	var out []string
	for i, z := range c.Z {
		said := "not met"
		if z.Met {
			said = "met"
		}
		line := fmt.Sprintf("Z %d %s", i+1, said)
		if z.How != "" {
			line += ": " + oneLine(z.How)
		}
		out = append(out, line+" ("+oneLine(z.Z)+")")
	}
	return out
}

// courtWords says whose court a mission is in: to you, or (forAgents) to
// crew and the orchestrator, for whom you're "the user".
func courtWords(c mission.Court, forAgents bool) string {
	if c == mission.CourtYou && !forAgents {
		return "your court"
	}
	return prompt.CourtWords(c)
}
