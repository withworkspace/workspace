import { getWorktrees, type Worktree } from "../api";
import { useLoaded } from "../load";
import { HoverCardLine } from "../ui/HoverCard";
import { type PageRef, refHref } from "./refs";
import { RefCardHead } from "./RefCardHead";
import { RefGo } from "./RefGo";

// RefRepoCard is a repo's card: its default branch, and its capsules.
export function RefRepoCard({ r, label }: { r: Extract<PageRef, { kind: "repo" }>; label?: string }) {
  const { value: wts, error } = useLoaded(() => getWorktrees(r.repo), [r.repo]);
  const ground = wts?.find((w: Worktree) => w.ground);
  const capsules = (wts ?? []).filter((w) => !w.ground);
  return (
    <>
      <RefCardHead icon="box" title={label || r.repo} aside={label && label !== r.repo ? r.repo : undefined} />
      {ground && (
        <HoverCardLine dim>
          Default branch {ground.branch}
          {ground.behind ? `, ${ground.behind} behind` : ""}
        </HoverCardLine>
      )}
      {wts && (
        <HoverCardLine dim>
          {capsules.length === 0 ? "No capsules" : `${capsules.length} ${capsules.length === 1 ? "capsule" : "capsules"}: ${capsules.map((c) => c.name).join(", ")}`}
        </HoverCardLine>
      )}
      {error && <HoverCardLine dim>Could not read: {error}</HoverCardLine>}
      <RefGo to={refHref(r)}>Open in Files</RefGo>
    </>
  );
}
