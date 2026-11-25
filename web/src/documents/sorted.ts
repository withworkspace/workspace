import type { SpaceDoc } from "../api";

// How a kind's list is sorted.
export type Sort = "made" | "title" | "stands" | "changed";

// sorted is a kind's documents in the order asked: by what made them (a
// number, the oldest first; a date, the newest first), title, where they
// stand (in the order of a document's life), or when they last changed.
export function sorted(docs: SpaceDoc[], by: Sort, states: string[] = []): SpaceDoc[] {
  const made = (d: SpaceDoc) => d.number ?? d.date ?? "";
  const dated = docs.some((d) => d.date && !d.number);
  const cmp: Record<Sort, (a: SpaceDoc, b: SpaceDoc) => number> = {
    made: (a, b) => (dated ? made(b).localeCompare(made(a)) : made(a).localeCompare(made(b), undefined, { numeric: true })),
    title: (a, b) => a.title.localeCompare(b.title, undefined, { numeric: true }),
    stands: (a, b) => states.indexOf(a.state ?? "") - states.indexOf(b.state ?? ""),
    changed: (a, b) => (b.changed ?? "").localeCompare(a.changed ?? ""),
  };
  return [...docs].sort((a, b) => cmp[by](a, b) || a.path.localeCompare(b.path));
}
