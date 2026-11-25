// What CodeLines draws beside a file's lines (CodeLines.tsx), worked out
// of what's known of the file: which lines are marked as changed, which
// folds open for a line that's gone to, and which lines a drag picks.

// CodeMark is how a line's changed since the file's base: added, or
// modified, or lines were removed just after it.
export type CodeMark = "added" | "modified" | "removedAfter";

// CodeChange is a change to a run of lines (from 1): what the server says
// of a file. A deleted one names the line the lines were removed after.
export interface CodeChange {
  kind: "added" | "modified" | "deleted";
  start: number;
  end?: number;
}

// marksOf is each changed line's mark. A line that's added or modified
// and has lines removed after it says the first: it's the one that reads.
export function marksOf(changes: CodeChange[]): Map<number, CodeMark> {
  const marks = new Map<number, CodeMark>();
  for (const c of changes) {
    if (c.kind === "deleted") {
      if (!marks.has(c.start)) marks.set(c.start, "removedAfter");
      continue;
    }
    for (let n = c.start; n <= (c.end ?? c.start); n++) marks.set(n, c.kind);
  }
  return marks;
}

// CodeFold is lines that fold away under the line at: those after it, to
// and with to (both from 1). The line at stays.
export interface CodeFold {
  at: number;
  to: number;
}

// CodeRange is a run of lines, from one to another and with both.
export interface CodeRange {
  from: number;
  to: number;
}

// opened is what's folded, less any fold that hides one of these lines:
// for a line that's gone to.
export function opened(folds: CodeFold[], folded: ReadonlySet<number>, lines: number[]): ReadonlySet<number> {
  const hiding = folds.filter((f) => folded.has(f.at) && lines.some((n) => n > f.at && n <= f.to));
  if (!hiding.length) return folded;
  const next = new Set(folded);
  for (const f of hiding) next.delete(f.at);
  return next;
}

// rangeFrom is the lines from where a pick began to where it's got to,
// whichever way it went.
export const rangeFrom = (anchor: number, to: number): CodeRange => ({ from: Math.min(anchor, to), to: Math.max(anchor, to) });
