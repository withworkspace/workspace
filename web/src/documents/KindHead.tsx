import { useMemo } from "react";
import type { SpaceDoc, SpaceKind } from "../api";
import type { Sort } from "./sorted";
import { SpaceKindHead } from "./SpaceKindHead";

// KindHead is a kind's list's heading, drawn as a SpaceKindHead: how many
// (of them, how many a search found), how many stand where, and the
// sorts that fit the kind (by number, or newest first; by where it
// stands, where it can stand somewhere).
export function KindHead({
  kind,
  docs,
  sort,
  setSort,
  stand,
  setStand,
  found,
}: {
  kind: SpaceKind;
  docs: SpaceDoc[]; // all of the kind's
  sort: Sort;
  setSort: (s: Sort) => void;
  stand: string; // "" for all
  setStand: (s: string) => void;
  found?: number; // of them, how many a search found
}) {
  const counts = useMemo(() => {
    const c = new Map<string, number>();
    for (const d of docs) c.set(d.state ?? "", (c.get(d.state ?? "") ?? 0) + 1);
    return c;
  }, [docs]);
  const states = kind.states ?? [];
  const sorts: [Sort, string][] = [["made", docs.some((d) => d.number) ? "By number" : "Newest first"], ["title", "By title"], ...(states.length > 0 ? [["stands", "By status"] as [Sort, string]] : []), ["changed", "Recently changed"]];
  return (
    <SpaceKindHead
      title={kind.title}
      count={found === undefined ? docs.length : `${found} of ${docs.length}`}
      states={states.map((s): [string, number] => [s, counts.get(s) ?? 0])}
      stand={stand}
      setStand={setStand}
      sorts={sorts}
      sort={sort}
      setSort={(s) => setSort(s as Sort)}
    />
  );
}
