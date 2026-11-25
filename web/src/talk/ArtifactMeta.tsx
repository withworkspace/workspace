import type { ArtifactInfo, TalkProposal } from "../api";
import { filesWords } from "../artifacts/shown";
import { artifactWhere } from "./proposals";
import { TalkProposalMeta } from "./TalkProposalMeta";
import { TalkProposalWarnings } from "./TalkProposalWarnings";

// ArtifactMeta is where a proposed artifact would go, its size, and what
// in it won't load in the frame (said to the agent too, when it proposed).
export function ArtifactMeta({ p, artifact }: { p: TalkProposal; artifact?: ArtifactInfo }) {
  const chips: string[] = [];
  if (p.state === "open") {
    const where = artifactWhere(p, artifact);
    if (where) chips.push(where);
  }
  if (p.size) chips.push(filesWords(p.files?.length, p.size));
  return (
    <>
      <TalkProposalMeta items={chips} />
      <TalkProposalWarnings warnings={p.warnings} />
    </>
  );
}
