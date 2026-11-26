import { HoverCardLine } from "../ui/HoverCard";
import { GROUND, type PageRef, refHref } from "./refs";
import { RefGo } from "./RefGo";

// RefGone is what a card says of a capsule that's gone, and where its file
// or folder is now: on the default branch, most likely, once it's landed.
export function RefGone({ r, there }: { r: Extract<PageRef, { kind: "file" | "dir" }>; there: boolean }) {
  return (
    <>
      <HoverCardLine dim>
        Capsule {r.worktree} is gone{r.commit ? ` (was at ${r.commit.slice(0, 7)})` : ""}.{" "}
        {there ? "On the default branch now:" : "Not on the default branch either."}
      </HoverCardLine>
      {there && <RefGo to={refHref({ ...r, worktree: GROUND })}>On the default branch now</RefGo>}
    </>
  );
}
