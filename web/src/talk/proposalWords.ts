import type { TalkProposal } from "../api";
import { artifactNext, kindName } from "./proposals";

// What a card of ws's own says of its kind: what it's called, what
// accepting it does, and what's being done once it's accepted. briefing:
// a brief that starts its mission; kindTitle, a document's kind as the
// workspace names it.

export function proposalLabel(p: TalkProposal, briefing: boolean, kindTitle: string): string {
  return (
    {
      decision: "Decision",
      mission: p.brief ? "Mission" : "Mission idea",
      brief: briefing ? "Mission brief" : "Edit to the brief",
      dispatch: "Crew",
      replan: "Replan",
      comment: "Review comment",
      plan: p.was ? "Edit to the plan" : "The plan",
      document: p.to ? `Edit to the ${kindName(kindTitle)}` : kindTitle,
      artifact: p.to ? `A version of ${p.to}` : "Artifact",
      plugin: p.label ?? "Proposal", // drawn by PluginProposalCard
      automation: "Automation",
      config: p.was?.trim() ? "Config change" : "New config file",
    }[p.kind] ?? "Proposal" // a kind ws no longer has, from before
  );
}

// proposalAccept is what accepting does; withBrief, whether a mission's
// to be created with a brief (else it's saved as an idea).
export function proposalAccept(p: TalkProposal, briefing: boolean, kindTitle: string, withBrief: boolean, artifact?: Parameters<typeof artifactNext>[1]): string {
  return {
    decision: "Record decision",
    mission: withBrief ? "Create mission" : "Save as an idea",
    brief: briefing ? "Confirm and start" : p.plan ? "Confirm goal and plan" : "Accept the brief",
    dispatch: "Start crew",
    replan: "Ask the planner",
    comment: "Add to my review",
    plan: "Accept the plan",
    document: p.to ? "Save the edit" : `Add ${kindName(kindTitle)}`,
    artifact: p.to ? `Add as v${artifactNext(p, artifact)}` : "Add artifact",
    plugin: "Accept",
    automation: p.automation?.replaces ? "Replace and turn on" : "Turn it on",
    config: p.lands === "capsule" ? "Commit in a capsule" : "Open pull request",
  }[p.kind];
}

export const proposalBusy = (p: TalkProposal): string =>
  ({ decision: "Recording…", mission: "Creating…", brief: "Writing…", dispatch: "Starting…", replan: "Asking…", comment: "Adding…", plan: "Writing…", document: p.to ? "Saving…" : "Adding…", artifact: "Adding…", plugin: "Working…", automation: "Turning on…", config: p.lands === "capsule" ? "Committing…" : "Opening…" })[p.kind];
