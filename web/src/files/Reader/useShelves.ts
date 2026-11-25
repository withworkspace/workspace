import { useEffect, useMemo, useState } from "react";
import { type DocIn, GROUND, getSpace, getTree, type Space, type SpaceDoc, type SpaceRef, worktreeAPI } from "../../api";
import { fileShelves, type Here, type Shelves, spaceShelves } from "../shelves";

// The shelves a document is kept on, for the reader's path and its pager
// (../shelves.ts works out where it is; this loads what that's worked out
// of).

// What the shelves are read from: a space's documents, or a worktree's
// files. Kept, so going from one document to the next draws its path at
// once; read again behind it when it's older than this.
type Source = { space: Space; docs: SpaceDoc[] } | { files: string[] };
const kept = new Map<string, { at: number; source: Source }>();
const fresh = 30_000;

// useShelves is the shelves a document is read among: its space's (a
// space's document on its default branch), else its worktree's markdown.
// None until they're read, or if they can't be.
export function useShelves(where: DocIn | undefined, space: SpaceRef | undefined, here: Here | undefined): Shelves | undefined {
  const key = !where ? undefined : space ? `space:${space.repo}` : `tree:${worktreeAPI(where.repo, where.worktree)}`;
  const [source, setSource] = useState<Source | undefined>(() => (key ? kept.get(key)?.source : undefined));
  useEffect(() => {
    if (!key || !where) return setSource(undefined);
    const had = kept.get(key);
    setSource(had?.source);
    if (had && Date.now() - had.at < fresh) return;
    let stale = false;
    const read: Promise<Source> = space ? getSpace(space.repo).then((r) => ({ space: r.space, docs: r.docs })) : getTree(worktreeAPI(where.repo, where.worktree)).then((t) => (t.unread ? Promise.reject(new Error("still being read")) : { files: t.files.filter((f) => f.status !== "deleted").map((f) => f.path) }));
    read.then(
      (s) => {
        kept.set(key, { at: Date.now(), source: s });
        if (!stale) setSource(s);
      },
      () => {}, // its path is then just its name: nothing to say
    );
    return () => {
      stale = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  const label = where ? `${where.repo}/${where.worktree === GROUND ? "ground" : where.worktree}` : "";
  return useMemo(() => {
    if (!source) return undefined;
    return "space" in source ? spaceShelves(source.space, source.docs, here) : fileShelves(label, source.files, here);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source, label, here?.path, here?.title, here?.kind]);
}
