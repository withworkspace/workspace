import type { TalkProposal } from "../api";
import { BriefDiff } from "./BriefDiff";
import { revisedText } from "./proposals";
import { TalkProposalChanges, talkProposalChangedClass } from "./TalkProposalChanges";
import { Text } from "../ui/Text";

// RevisionChanges is what a revision changed of the proposal it revises,
// folded: the words each is mostly made of, as they read and as they do.
export function RevisionChanges({ was, now }: { was: TalkProposal; now: TalkProposal }) {
  const a = revisedText(was);
  const b = revisedText(now);
  if (!a && !b) return null;
  return <TalkProposalChanges n={was.n}>{a === b ? <Text as="div" dim>Its words are as they were.</Text> : <BriefDiff className={talkProposalChangedClass} was={a} now={b} />}</TalkProposalChanges>;
}
