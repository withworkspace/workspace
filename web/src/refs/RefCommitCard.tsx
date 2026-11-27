import { HoverCardLine } from "../ui/HoverCard";
import { isOutside, type PageRef } from "./refs";
import { RefCardHead } from "./RefCardHead";
import css from "./RefCommitCard.module.css";
import { RefGo } from "./RefGo";

// RefCommitCard is a commit's card: its sha, short, of its repo (and
// whole under it, where it was written whole), and its link out. Its subject isn't asked for. href
// is where it is.
export function RefCommitCard({ r, href }: { r: Extract<PageRef, { kind: "commit" }>; href: string }) {
  return (
    <>
      <RefCardHead icon="commit" title={`Commit ${r.commit.slice(0, 7)}`} aside={r.repo} />
      {r.commit.length > 7 && (
        <HoverCardLine dim wrap>
          <span className={css.sha}>{r.commit}</span>
        </HoverCardLine>
      )}
      {isOutside(href) ? <RefGo href={href}>Open on GitHub</RefGo> : <RefGo to={href}>Open the repo in Files</RefGo>}
    </>
  );
}
