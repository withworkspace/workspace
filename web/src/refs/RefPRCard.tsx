import { useLoaded } from "../load";
import { HoverCardLine } from "../ui/HoverCard";
import { prOf } from "./prs";
import { type PageRef, refHref } from "./refs";
import { RefCardHead } from "./RefCardHead";
import { RefChip } from "./RefChip";
import { RefGo } from "./RefGo";

// RefPRCard is a pull request's card: its title, who opened it, where it
// stands and its checks, and how much it changes.
export function RefPRCard({ r }: { r: Extract<PageRef, { kind: "pr" }> }) {
  const { value: pr, error } = useLoaded(() => prOf(r.repo, r.number), [r.repo, r.number]);
  return (
    <>
      <RefCardHead icon="pr" title={pr ? pr.title : `${r.repo}#${r.number}`} />
      {pr && (
        <>
          <HoverCardLine dim>
            {r.repo}#{r.number}, by {pr.author}
            {pr.draft ? ", a draft" : ""}
          </HoverCardLine>
          <HoverCardLine>
            {pr.words}
            {pr.checks && <RefChip tone={pr.checks === "success" ? "done" : pr.checks === "failure" ? "danger" : "quiet"}>checks {pr.checks === "success" ? "pass" : pr.checks === "failure" ? "fail" : "running"}</RefChip>}
          </HoverCardLine>
          <HoverCardLine dim>
            +{pr.additions} −{pr.deletions}, {pr.files} {pr.files === 1 ? "file" : "files"}
          </HoverCardLine>
        </>
      )}
      {error && <HoverCardLine dim>Not loaded from GitHub, or not found.</HoverCardLine>}
      {!pr && !error && <HoverCardLine dim>Loading…</HoverCardLine>}
      <RefGo to={refHref(r)}>Review</RefGo>
    </>
  );
}
