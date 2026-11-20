import { type ArtifactInfo, frameURL, type TalkProposal } from "../api";
import { filesWords } from "../artifacts/shown";
import { Icon } from "../ui/Icon";
import { addedVersion, artifactWhere, staleWords } from "./proposals";
import { Chip } from "../ui/Chip";

// An artifact a conversation's agent proposed, or a version of one, in its
// card under the reply: its note, then its page running in a small
// sandboxed frame, served as a version is (/frame/w9/proposals/2, its
// files with it where it has them), with Open larger. A version is shown beside the one it's based on, and the
// card says when versions were added since. Added, the frame is the
// version it made (a3.v1); declined, it's still the proposal's, which
// stays in the conversation.

export function ArtifactProposalBody({ p, src, artifact }: { p: TalkProposal; src: string; artifact?: ArtifactInfo }) {
  const added = p.state === "accepted" ? addedVersion(p.result) : undefined;
  const stale = p.state === "open" ? staleWords(p, artifact) : "";
  const beside = !added && p.to && p.on ? p.on : 0;
  const name = artifact?.title ?? p.title ?? p.to ?? "The artifact";
  return (
    <>
      <div className="proposal-what said-text">{p.note}</div>
      {stale && (
        <div className="proposal-stale">
          <Icon name="alert" size={13} /> {stale}. Adding it still bases it on v{p.on}.
        </div>
      )}
      <div className={`proposal-frames${beside ? " two" : ""}`}>
        {beside > 0 && <SmallFrame src={frameURL(p.to ?? "", beside)} caption={`v${beside}, which it revises`} title={`${name}, v${beside}`} />}
        {added ? (
          <SmallFrame src={frameURL(added.id, added.n)} title={`${name}, v${added.n}`} />
        ) : (
          <SmallFrame src={src} caption={beside ? "Proposed" : undefined} title={`${name}, as proposed`} />
        )}
      </div>
      <ArtifactMeta p={p} artifact={artifact} />
    </>
  );
}

// SmallFrame is a version, or a proposal, running in a small frame, as the
// artifact's page runs one: sandbox="allow-scripts", no allow-same-origin,
// so its scripts run in an origin of their own and its policy lets
// nothing leave. Open larger opens it alone, in a tab of its own, under
// the same policy.
function SmallFrame({ src, caption, title }: { src: string; caption?: string; title: string }) {
  return (
    <figure className="proposal-frame">
      {caption && <figcaption className="dim">{caption}</figcaption>}
      <iframe src={src} sandbox="allow-scripts" referrerPolicy="no-referrer" loading="lazy" title={title} />
      <a href={src} target="_blank" rel="noopener noreferrer" className="button-link" title="Open it alone, in a tab of its own (still sandboxed)">
        <Icon name="external" size={12} /> Open larger
      </a>
    </figure>
  );
}

// ArtifactMeta is where a proposed artifact would go, its size, and what
// in it won't load in the frame (said to the agent too, when it proposed).
function ArtifactMeta({ p, artifact }: { p: TalkProposal; artifact?: ArtifactInfo }) {
  const chips: string[] = [];
  if (p.state === "open") {
    const where = artifactWhere(p, artifact);
    if (where) chips.push(where);
  }
  if (p.size) chips.push(filesWords(p.files?.length, p.size));
  return (
    <>
      {chips.length > 0 && (
        <div className="proposal-meta">
          {chips.map((c) => (
            <Chip tone="quiet" key={c}>
              {c}
            </Chip>
          ))}
        </div>
      )}
      {!!p.warnings?.length && (
        <ul className="proposal-warnings">
          {p.warnings.map((w) => (
            <li key={w}>
              <Icon name="alert" size={12} /> {w}
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
