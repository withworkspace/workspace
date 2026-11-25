import type { TalkProposal } from "../api";
import { BriefDiff } from "./BriefDiff";
import { revisedText } from "./proposals";
import { TalkProposalChanges, talkProposalChangedClass } from "./TalkProposalChanges";

// RevisionChanges is what a revision changed of the proposal it revises,
// folded: the words each is mostly made of, as they read and as they do.
export function RevisionChanges({ was, now }: { was: TalkProposal; now: TalkProposal }) {
  const a = revisedText(was);
  const b = revisedText(now);
  if (!a && !b) return null;
  return <TalkProposalChanges n={was.n}>{a === b ? <div className="dim">Its words are as they were.</div> : <BriefDiff className={talkProposalChangedClass} was={a} now={b} />}</TalkProposalChanges>;
}
