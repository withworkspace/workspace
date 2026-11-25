import type { DocQuote, Thread } from "../api";
import { rangeOf, type Target, type TextIndex } from "./anchor";

// Where a document's threads are, and where what's drawn over it goes:
// worked out by AnnotatedDoc (Annotate.tsx) after each draw.

// Spot is where a thread is drawn: a range of text, or a whole block with
// no words of its own (a diagram).
export type Spot = { kind: "text"; start: number; end: number; range: Range } | { kind: "block"; el: HTMLElement };

// Pick is what a new thread is on.
export interface Pick {
  line: number;
  endLine: number;
  quote: DocQuote;
  spot: Spot;
}

export interface Placed {
  thread: Thread;
  spot?: Spot; // none: not found in this version
  pos: number; // where it is in the text, to order by
}

export const COMPOSER = "composer";
export const GAP = 8;

export interface Box {
  top: number;
  height: number;
}

// Geo is where everything over the document goes, measured after it's
// drawn: the margin's cards, the cursor, the comment button, the marks on
// commented diagrams, and the bars by what's changed.
export interface Geo {
  cards: Record<string, number>;
  height: number;
  cursor?: Box;
  button?: number;
  marks: (Box & { id: string; left: number; width: number })[];
  bars: (Box & { i: number; removed: boolean })[];
  // anchors are where each thread's text starts, for the markers that stand
  // in for the margin on a narrow window.
  anchors?: { id: string; top: number }[];
}

export const noGeo: Geo = { cards: {}, height: 0, marks: [], bars: [] };

export function spotRect(s: Spot): DOMRect | undefined {
  if (s.kind === "block") return s.el.isConnected ? s.el.getBoundingClientRect() : undefined;
  const rects = s.range.getClientRects();
  for (const r of rects) if (r.width > 0) return r;
  return rects[0];
}

export function pickOf(ix: TextIndex, t: Target): Pick | undefined {
  const range = rangeOf(ix, t.start, t.end);
  return range && { line: t.line, endLine: t.endLine, quote: t.quote, spot: { kind: "text", start: t.start, end: t.end, range } };
}

// stack places cards level with what they're on, without overlapping:
// the one in focus exactly, the rest pushed away from it.
export function stack(want: { id: string; top: number }[], height: (id: string) => number, focus?: string): Record<string, number> {
  const pos = want.map((w) => w.top);
  const f = Math.max(0, want.findIndex((w) => w.id === focus));
  for (let i = f + 1; i < want.length; i++) pos[i] = Math.max(want[i].top, pos[i - 1] + height(want[i - 1].id) + GAP);
  for (let i = f - 1; i >= 0; i--) pos[i] = Math.min(want[i].top, pos[i + 1] - height(want[i].id) - GAP);
  const lift = pos.length && pos[0] < 0 ? -pos[0] : 0;
  return Object.fromEntries(want.map((w, i) => [w.id, Math.round(pos[i] + lift)]));
}

export function caretAt(x: number, y: number): { node: Node; offset: number } | undefined {
  const p = document.caretPositionFromPoint?.(x, y);
  if (p) return { node: p.offsetNode, offset: p.offset };
  const r = (document as Document & { caretRangeFromPoint?: (x: number, y: number) => Range | null }).caretRangeFromPoint?.(x, y);
  return r ? { node: r.startContainer, offset: r.startOffset } : undefined;
}

export const num = (id: string) => Number(id.slice(1));

// sinceYou is the version of the document you last commented on (a thread
// you opened, or a reply), and when.
export function sinceYou(threads: Thread[]): { version: string; at: string } | undefined {
  let best: { version: string; at: string } | undefined;
  for (const t of threads)
    for (const turn of t.turns)
      if (turn.by === "you" && turn.version && (!best || new Date(turn.at).getTime() > new Date(best.at).getTime())) best = { version: turn.version, at: turn.at };
  return best;
}
