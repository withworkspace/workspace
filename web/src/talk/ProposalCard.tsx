import type { ArtifactInfo, DocKind, TalkProposal } from "../api";
import { PluginProposalCard } from "./PluginProposal";
import { RevisionChanges } from "./RevisionChanges";
import { WsProposal } from "./WsProposal";

export interface ProposalCardProps {
  p: TalkProposal;
  revisedFrom?: TalkProposal;
  kinds?: DocKind[];
  artifacts?: ArtifactInfo[];
  frameOf?: (n: number) => string;
  inReview?: boolean;
  slug?: string;
  replying?: boolean;
  anchor?: string;
  onSettle: (n: number, action: "accept" | "decline", edits?: object) => Promise<void>;
}

// ProposalCard is something the agent proposed, under its reply: a
// decision to record, a mission to create (ready, with its brief) or save
// as an idea, an edit to the mission's brief (briefing it, with its plan),
// or its plan, crew to start, a replan by the mission's planner, a review
// comment, a document to add (of one of the workspace's kinds, named by
// kinds), or an artifact (or a version of one, of artifacts), running in
// the frame frameOf gives it: ws's own (WsProposal), or a plugin's
// (PluginProposalCard). Edit it first, or not; confirming does it, as
// you. inReview is a review comment's card on the review itself (in its
// ask bar's card): it doesn't lead there. slug is the mission the
// conversation's about, if it's a mission's: a replan leads to its
// planner's conversation there. replying: the agent's still writing the
// reply it came with, so it can't be confirmed until that's there to read.
// anchor is its id on the page, where it's gone to from beside the
// conversation.
export function ProposalCard(props: ProposalCardProps) {
  if (props.p.kind === "plugin")
    return (
      <PluginProposalCard
        p={props.p}
        changes={props.revisedFrom && props.p.state === "open" ? <RevisionChanges was={props.revisedFrom} now={props.p} /> : undefined}
        replying={props.replying}
        anchor={props.anchor}
        onSettle={props.onSettle}
      />
    );
  return <WsProposal {...props} />;
}
