import type { SpaceDoc, SpaceDocChange } from "../api";

// Where a space's page puts things: each row's and each list's id, and
// its documents by folder.

// libraryID is a row of a kind's list; docID a row of a folder's.
export const libraryID = (path: string) => `lib-${path.replace(/[^\w-]/g, "_")}`;
export const docID = (path: string) => `doc-${path.replace(/[^\w-]/g, "_")}`;

// listID is a list's section in a space's page: a kind's, or a folder's
// (below the space's own folder, with its slash; "" for its top).
export const listID = (of: "kind" | "folder", name: string) => `list-${of}-${name.replace(/[^\w-]/g, "_")}`;

// byFolder is a space's documents in their folders (relative to the
// space's own), the top first, then by name; each folder's by title.
export function byFolder(docs: SpaceDoc[], root: string): [string, SpaceDoc[]][] {
  const by = new Map<string, SpaceDoc[]>();
  for (const d of docs) {
    const rel = d.path.slice(root.length);
    const at = rel.lastIndexOf("/");
    const folder = at < 0 ? "" : rel.slice(0, at + 1);
    by.set(folder, [...(by.get(folder) ?? []), d]);
  }
  // By path: a numbered or dated document's name keeps them in order.
  for (const list of by.values()) list.sort((a, b) => a.path.localeCompare(b.path, undefined, { numeric: true }));
  return [...by].sort(([a], [b]) => (a === "" ? -1 : b === "" ? 1 : a.localeCompare(b)));
}

// mark is how a change to a document is marked: added, removed, changed.
export const mark = (how: SpaceDocChange["how"]) => (how === "added" ? "+" : how === "removed" ? "−" : "~");
