import { HoverCardLine } from "../ui/HoverCard";
import { type IssueIs, useIssueIs } from "./issues";
import { isOutside, type PageRef } from "./refs";
import { RefCardHead } from "./RefCardHead";
import css from "./RefIssueCard.module.css";
import { RefGo } from "./RefGo";
import { RefPRCard } from "./RefPRCard";

// RefIssueCard is an issue's card. Its name is a pull request's too, so
// it asks GitHub which it is, as a pull request's card does: an open pull
// request is drawn as one; anything else is the issue, with its link out
// (GitHub sends an issue's address on to its pull request's, if it's one
// that's closed). href is where it is; is, what GitHub said, where that's
// known already (a story): nothing's asked then.
export function RefIssueCard({ r, href, is: said }: { r: Extract<PageRef, { kind: "issue" }>; href: string; is?: IssueIs }) {
  const asked = useIssueIs(r.repo, r.number, said);
  if (asked.is === "pr") return <RefPRCard r={{ kind: "pr", repo: r.repo, number: r.number }} />;
  return (
    <>
      <RefCardHead icon="circle-dot" title={`Issue ${r.number}`} aside={r.repo} />
      <HoverCardLine dim>
        <span className={css.name}>
          {r.repo}#{r.number}
        </span>
        , on GitHub
      </HoverCardLine>
      {asked.is === "issue" && <HoverCardLine dim>Not an open pull request.</HoverCardLine>}
      {asked.is === "unknown" && <HoverCardLine dim>Couldn't ask GitHub whether it's a pull request.</HoverCardLine>}
      {asked.is === "asking" && <HoverCardLine dim>Asking GitHub…</HoverCardLine>}
      {isOutside(href) ? <RefGo href={href}>Open on GitHub</RefGo> : <RefGo to={href}>Open the repo in Files</RefGo>}
    </>
  );
}
