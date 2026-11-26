import { create } from "zustand";
import type { DiffFile } from "../api";
import type { Row } from "./model";

// A review's comments, as a list to read and go to: the Comments tab
// beside a review (panes/CommentsPane), and N and P on the page. The page
// holds them (its rows); while it's open it says what they are here, for
// the tab, which is drawn far from it in the tree, and how to go to one.

// CommentEntry is one comment as the list says it.
export interface CommentEntry {
  key: string; // its row's, to go to
  id: string;
  path: string; // "" on the whole change
  lines: string; // "L12", "L12–14", "" with none
  removed: boolean; // on removed lines
  kind: string; // suggestion, question, nit…; "" for none said
  state: CommentState;
  body: string;
  last?: { by: string; body: string }; // the latest reply
  replies: number;
}

// How a comment stands, as the list sorts them: yours, not sent; open
// (addressed too: it's still for you to settle); or settled (resolved,
// won't fix).
export type CommentState = "draft" | "open" | "addressed" | "resolved" | "wontfix";

export type CommentFilter = "all" | "open" | "draft" | "resolved";

const settled = (s: CommentState) => s === "resolved" || s === "wontfix";

export const inFilter = (f: CommentFilter) => (c: CommentEntry) =>
  f === "all" || (f === "draft" ? c.state === "draft" : f === "resolved" ? settled(c.state) : !settled(c.state) && c.state !== "draft");

// commentEntries is the page's comments in the order they're read: each
// file's, in the files' order, then those on files the diff doesn't have.
export function commentEntries(rows: Row[][], orphans: Row[], files: DiffFile[]): CommentEntry[] {
  return [...rows.flat(), ...orphans].flatMap((r): CommentEntry[] => {
    if (r.type !== "comment") return [];
    const n = r.placed.note;
    const last = n.thread.at(-1);
    return [
      {
        key: r.key,
        id: n.id,
        path: n.path || files[r.file]?.path || "",
        lines: !n.path || !n.line ? "" : n.endLine > n.line ? `L${n.line}–${n.endLine}` : `L${n.line}`,
        removed: n.side === "old",
        kind: n.kind,
        state: r.placed.draft ? "draft" : ((["open", "addressed", "resolved", "wontfix"] as const).find((s) => s === n.status) ?? "open"),
        body: n.body,
        last: last && { by: last.by, body: last.body },
        replies: n.thread.length,
      },
    ];
  });
}

// stepFrom is the comment n along from the one at (back, with n -1),
// round from one end to the other; the first (or last) with none.
export function stepFrom(list: CommentEntry[], at: string | undefined, n: 1 | -1): CommentEntry | undefined {
  if (!list.length) return undefined;
  const i = list.findIndex((c) => c.key === at);
  if (i < 0) return n > 0 ? list[0] : list[list.length - 1];
  return list[(i + n + list.length) % list.length];
}

interface ReviewComments {
  // The page that holds them, by its address; nothing while no review's
  // open.
  of?: string;
  list: CommentEntry[];
  // The one last gone to, marked in the list.
  at?: string;
  filter: CommentFilter;
  // Going to one: the page shows the changes, with it in the middle.
  go?: (key: string) => void;
  say: (of: string, list: CommentEntry[], go: (key: string) => void) => void;
  gone: (of: string) => void;
  setAt: (at: string | undefined) => void;
  setFilter: (f: CommentFilter) => void;
}

export const useReviewComments = create<ReviewComments>()((set, get) => ({
  list: [],
  filter: "all",
  say: (of, list, go) => set((s) => ({ of, list, go, at: s.of === of ? s.at : undefined })),
  gone: (of) => {
    if (get().of === of) set({ of: undefined, list: [], go: undefined, at: undefined });
  },
  setAt: (at) => set({ at }),
  setFilter: (filter) => set({ filter }),
}));
