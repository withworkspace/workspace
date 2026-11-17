import type { Node as PMNode } from "prosemirror-model";
import { parseMarkdown } from "./markdown";

// An edit offered as suggestions, rather than made: each passage it
// changes is a suggestion of its own, on a thread of its own, to accept or
// decline one by one. A passage is whole lines of the markdown (the
// bridge changes only the lines an edit touches, so they're few), widened
// until what it replaces is in the document once, as a suggestion must be.

export interface Proposed {
  from: string; // the lines it replaces, as they are
  to: string; // what they'd be
  line: number; // the first of them, from 1
  endLine: number;
  quote: string; // the words they read as, for the thread to be on
}

// lines is text's lines, each with its line break.
const lines = (t: string) => t.match(/[^\n]*\n|[^\n]+$/g) ?? [];

export function suggestionsFor(base: string, edited: string): Proposed[] {
  const a = lines(base);
  const b = lines(edited);
  // The passages that differ, by a longest common subsequence of lines
  // (between what's the same at either end).
  let pre = 0;
  while (pre < a.length && pre < b.length && a[pre] === b[pre]) pre++;
  let post = 0;
  while (post < a.length - pre && post < b.length - pre && a[a.length - 1 - post] === b[b.length - 1 - post]) post++;
  const n = a.length - pre - post;
  const m = b.length - pre - post;
  const L = Array.from({ length: n + 1 }, () => new Int32Array(m + 1));
  for (let i = n - 1; i >= 0; i--) for (let j = m - 1; j >= 0; j--) L[i][j] = a[pre + i] === b[pre + j] ? L[i + 1][j + 1] + 1 : Math.max(L[i + 1][j], L[i][j + 1]);
  type Hunk = { a0: number; a1: number; b0: number; b1: number };
  const hunks: Hunk[] = [];
  let open: Hunk | undefined;
  for (let i = 0, j = 0; i < n || j < m; ) {
    if (i < n && j < m && a[pre + i] === b[pre + j]) {
      if (open) hunks.push(open);
      open = undefined;
      i++;
      j++;
      continue;
    }
    open ??= { a0: pre + i, a1: pre + i, b0: pre + j, b1: pre + j };
    if (j >= m || (i < n && L[i + 1][j] >= L[i][j + 1])) open.a1 = pre + ++i;
    else open.b1 = pre + ++j;
  }
  if (open) hunks.push(open);

  // Widened: a line put in is said with the line before it (or after, at
  // the top), and a passage that's in the document more than once takes
  // in a line more until it's there once. Passages that meet join.
  const count = (s: string) => base.split(s).length - 1;
  const out: Hunk[] = [];
  for (const h of hunks) {
    const prev = out[out.length - 1];
    const widen = (up: boolean) => {
      if (up && h.a0 > 0) {
        h.a0--;
        h.b0--;
      } else if (h.a1 < a.length) {
        h.a1++;
        h.b1++;
      } else return false;
      return true;
    };
    if (h.a0 === h.a1) widen(h.a0 > 0);
    for (let up = true; count(a.slice(h.a0, h.a1).join("")) !== 1; up = !up) if (!widen(up) && !widen(!up)) break;
    if (prev && h.a0 <= prev.a1) {
      prev.a1 = Math.max(prev.a1, h.a1);
      prev.b1 = Math.max(prev.b1, h.b1);
    } else out.push(h);
  }

  const doc = parseMarkdown(base);
  const offsetOf = (line: number) => a.slice(0, line).join("").length;
  return out
    .filter((h) => count(a.slice(h.a0, h.a1).join("")) === 1)
    .map((h) => ({
      from: a.slice(h.a0, h.a1).join(""),
      to: b.slice(h.b0, h.b1).join(""),
      line: h.a0 + 1,
      endLine: Math.max(h.a0 + 1, h.a1),
      quote: wordsIn(doc.doc, doc.origins, offsetOf(h.a0), offsetOf(h.a1)),
    }));
}

// wordsIn is what the text from start to end reads as: the words of the
// textblocks in it, a line break between them, as the reader's index has
// them.
function wordsIn(doc: PMNode, origins: WeakMap<PMNode, { start: number; end: number }>, start: number, end: number): string {
  const out: string[] = [];
  doc.descendants((node) => {
    const o = origins.get(node);
    if (o && (o.end <= start || o.start >= end)) return false;
    if (node.isTextblock) {
      if (node.textContent.trim()) out.push(node.textContent);
      return false;
    }
    return true;
  });
  return out.join("\n").slice(0, 500);
}
