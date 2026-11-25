import { useLoaded } from "../load";
import { HoverCardLine } from "../ui/HoverCard";
import { excerptRoom, treeOf } from "./cards";
import { goneCapsule, GROUND, type Known, type PageRef, refHref } from "./refs";
import { RefCardHead } from "./RefCardHead";
import { RefExcerpt, RefExcerptLine } from "./RefExcerpt";
import { RefGo } from "./RefGo";
import { RefGone } from "./RefGone";

// RefDirCard is a folder's card: what's in it, folders first.
export function RefDirCard({ r: named, known, pane }: { r: Extract<PageRef, { kind: "dir" }>; known?: Known; pane?: boolean }) {
  const gone = goneCapsule(named, known);
  const r = gone ? { ...named, worktree: GROUND } : named;
  const { value: tree, error } = useLoaded(() => treeOf(r.repo, r.worktree), [r.repo, r.worktree]);
  const inside = new Set<string>();
  for (const f of tree?.files ?? []) {
    if (!f.path.startsWith(r.path + "/")) continue;
    const rest = f.path.slice(r.path.length + 1);
    const slash = rest.indexOf("/");
    inside.add(slash < 0 ? rest : rest.slice(0, slash + 1));
  }
  const names = [...inside].sort((a, b) => Number(b.endsWith("/")) - Number(a.endsWith("/")) || a.localeCompare(b));
  return (
    <>
      <RefCardHead icon="layers" title={`${r.path}/`} mono />
      <HoverCardLine dim>
        {r.repo}, {r.worktree === GROUND ? "default branch" : r.worktree}
        {tree && !tree.unread ? `, ${names.length} ${names.length === 1 ? "thing" : "things"} in it` : ""}
      </HoverCardLine>
      {tree?.unread && <HoverCardLine dim>Its files are still being read.</HoverCardLine>}
      {gone && !tree?.unread && <RefGone r={named} there={names.length > 0} />}
      {error && <HoverCardLine dim>It can't be read: {error}</HoverCardLine>}
      {names.length > 0 && (
        <RefExcerpt raised={pane} more={names.length > excerptRoom}>
          {names.slice(0, excerptRoom).map((n) => (
            <RefExcerptLine key={n}>{n}</RefExcerptLine>
          ))}
        </RefExcerpt>
      )}
      <RefGo to={refHref(r)}>Open it in Files</RefGo>
    </>
  );
}
