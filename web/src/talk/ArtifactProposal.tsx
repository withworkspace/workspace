import { type ArtifactInfo, frameURL, type TalkProposal } from "../api";
import { ArtifactMeta } from "./ArtifactMeta";
import { addedVersion, staleWords } from "./proposals";
import { TalkProposalFrame } from "./TalkProposalFrame";
import { TalkProposalFrames } from "./TalkProposalFrames";
import { TalkProposalStale } from "./TalkProposalStale";
import { TalkProposalWhat } from "./TalkProposalWhat";

// An artifact a conversation's agent proposed, or a version of one, in its
// card under the reply: its note, then its page running in a small
// sandboxed frame, served as a version is (/frame/w9/proposals/2, its
// files with it where it has them), with Open larger. A version is shown
// beside the one it's based on, and the card says when versions were
// added since. Added, the frame is the version it made (a3.v1); declined,
// it's still the proposal's, which stays in the conversation.
export function ArtifactProposalBody({ p, src, artifact }: { p: TalkProposal; src: string; artifact?: ArtifactInfo }) {
  const added = p.state === "accepted" ? addedVersion(p.result) : undefined;
  const stale = p.state === "open" ? staleWords(p, artifact) : "";
  const beside = !added && p.to && p.on ? p.on : 0;
  const name = artifact?.title ?? p.title ?? p.to ?? "Artifact";
  return (
    <>
      <TalkProposalWhat said>{p.note}</TalkProposalWhat>
      {stale && (
        <TalkProposalStale>
          {stale}. Will still be based on v{p.on}.
        </TalkProposalStale>
      )}
      <TalkProposalFrames two={!!beside}>
        {beside > 0 && <TalkProposalFrame src={frameURL(p.to ?? "", beside)} caption={`v${beside}, before`} title={`${name}, v${beside}`} />}
        {added ? <TalkProposalFrame src={frameURL(added.id, added.n)} title={`${name}, v${added.n}`} /> : <TalkProposalFrame src={src} caption={beside ? "Proposed" : undefined} title={`${name}, as proposed`} />}
      </TalkProposalFrames>
      <ArtifactMeta p={p} artifact={artifact} />
    </>
  );
}
