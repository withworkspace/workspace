// What DiffLines draws (DiffLines.tsx): a diff's rows, and the lines
// picked among them. The rows are as review/model lays a file out
// (fileRows): this says only as much of them as is drawn.

// DiffCell is a line on one side: its number there, its text, and whether
// it's unchanged (c), added (a) or removed (d). shown is an unchanged line
// opened up from between hunks, not one of the diff's own; noeol is a last
// line with no newline after it.
export interface DiffCell {
  num: number;
  text: string;
  kind: "c" | "a" | "d";
  shown?: boolean;
  noeol?: boolean;
}

// DiffLineRow is a line of the diff: the old side's cell, the new side's,
// or both (unchanged; or, side by side, a removal beside what replaced
// it).
export interface DiffLineRow {
  type: "line";
  key: string;
  left?: DiffCell;
  right?: DiffCell;
}

// DiffGapRow is unchanged lines not shown, between hunks: how many
// (unknown past the last hunk until the file's read), and what they're in
// (a function's name, as git says it).
export interface DiffGapRow {
  type: "gap";
  key: string;
  count?: number;
  section?: string;
}

// DiffNoteRow is something said in place of lines: a binary file, a diff
// too large to show until it's asked for (large: it's pressed for that).
export interface DiffNoteRow {
  type: "note";
  key: string;
  text: string;
  large?: boolean;
}

// DiffRow is a row DiffLines knows how to draw. Any other row a page has
// among them (a comment, what a pass found, an editor) is the page's.
export type DiffRow = DiffLineRow | DiffGapRow | DiffNoteRow;

// Keyed is the least a row is: its type and its key.
export interface Keyed {
  type: string;
  key: string;
}

export const isLine = (r: Keyed): r is DiffLineRow => r.type === "line";
export const isGap = (r: Keyed): r is DiffGapRow => r.type === "gap";
export const isNote = (r: Keyed): r is DiffNoteRow => r.type === "note";

// DiffPick is lines picked: from one row to another and with both, by
// their keys, top first. Side by side, side is the column they were
// picked in.
export interface DiffPick {
  from: string;
  to: string;
  side?: "old" | "new";
}

// pickFrom is the rows from where a pick began to where it's got to,
// whichever way it went. Undefined when either isn't among the rows.
export function pickFrom(rows: Keyed[], anchor: string, to: string, side?: "old" | "new"): DiffPick | undefined {
  const a = rows.findIndex((r) => r.key === anchor);
  const b = rows.findIndex((r) => r.key === to);
  if (a < 0 || b < 0) return undefined;
  return { from: rows[Math.min(a, b)]!.key, to: rows[Math.max(a, b)]!.key, side };
}

// pickedRows is where the picked rows are among the rows: the first and
// the last, or nothing when they aren't there.
export function pickedRows(rows: Keyed[], pick?: DiffPick): { from: number; to: number } | undefined {
  if (!pick) return undefined;
  const from = rows.findIndex((r) => r.key === pick.from);
  const to = rows.findIndex((r) => r.key === pick.to);
  return from < 0 || to < from ? undefined : { from, to };
}

// unifiedKind is how a line reads in one column: added, removed, or
// unchanged.
export const unifiedKind = (r: DiffLineRow): DiffCell["kind"] => (r.left && !r.right ? "d" : r.right && !r.left ? "a" : "c");

// digitsOf is how many digits the widest line number among the rows has
// (two at least), for the width of their column.
export function digitsOf(rows: Keyed[]): number {
  let most = 0;
  for (const r of rows) if (isLine(r)) most = Math.max(most, r.left?.num ?? 0, r.right?.num ?? 0);
  return Math.max(2, String(most).length);
}
