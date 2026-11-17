import type { DocQuote } from "../api";

// Comments on a document are kept by the text they highlight, as it reads
// rendered (a quote, with a little either side), and the markdown lines
// it's in. To draw them, the rendered document is indexed: its text in
// reading order, a line break between blocks, with where each text node
// starts. A quote is found again by searching that text, so a comment
// stays put while the words it's on do, wherever they move.
//
// The markdown renderer marks the blocks a reader picks out with
// data-block and their lines (data-ls, data-le), and what isn't the
// document's words (a code block's language, a diagram) with data-skip.

export interface TextIndex {
  text: string;
  nodes: Text[];
  starts: number[];
  at: Map<Text, number>; // a node's place in nodes
}

const SKIP = "[data-skip]";
export const BLOCK = "[data-block]";
const CONTEXT = 32; // characters kept either side of a quote

export function buildIndex(root: HTMLElement): TextIndex {
  const ix: TextIndex = { text: "", nodes: [], starts: [], at: new Map() };
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let last: Element | null = null;
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    const t = n as Text;
    const parent = t.parentElement;
    if (!parent || parent.closest(SKIP)) continue;
    // Text between blocks is the renderer's whitespace, not the document's.
    const block = parent.closest(BLOCK);
    if (!block || !root.contains(block)) continue;
    if (block !== last && ix.text && !ix.text.endsWith("\n")) ix.text += "\n";
    last = block;
    ix.at.set(t, ix.nodes.length);
    ix.nodes.push(t);
    ix.starts.push(ix.text.length);
    ix.text += t.data;
  }
  return ix;
}

// offsetOf is where a point in the document (a selection's end, say) is
// in the index's text: between nodes, where the next one starts.
export function offsetOf(ix: TextIndex, container: Node, offset: number): number {
  const i = ix.at.get(container as Text);
  if (i !== undefined) return ix.starts[i] + Math.min(offset, (container as Text).length);
  const point = document.createRange();
  try {
    point.setStart(container, offset);
  } catch {
    return ix.text.length;
  }
  let lo = 0;
  let hi = ix.nodes.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (point.comparePoint(ix.nodes[mid], 0) < 0) lo = mid + 1;
    else hi = mid;
  }
  return lo < ix.nodes.length ? ix.starts[lo] : ix.text.length;
}

// locate is the node and offset at a place in the index's text. A place in
// the break between two blocks is the start of the next, going forward,
// or the end of the last.
function locate(ix: TextIndex, pos: number, forward: boolean): { node: Text; offset: number } | undefined {
  if (!ix.nodes.length) return undefined;
  let lo = 0;
  let hi = ix.nodes.length - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (ix.starts[mid] <= pos) lo = mid;
    else hi = mid - 1;
  }
  const node = ix.nodes[lo];
  const offset = pos - ix.starts[lo];
  if (offset < 0) return { node, offset: 0 };
  if (offset < node.length || (offset === node.length && !forward)) return { node, offset };
  if (forward && lo + 1 < ix.nodes.length) return { node: ix.nodes[lo + 1], offset: 0 };
  return { node, offset: node.length };
}

// rangeOf is the document's range over [start, end) of the index's text.
export function rangeOf(ix: TextIndex, start: number, end: number): Range | undefined {
  const a = locate(ix, start, true);
  const b = locate(ix, end, false);
  if (!a || !b) return undefined;
  const r = document.createRange();
  r.setStart(a.node, a.offset);
  r.setEnd(b.node, b.offset);
  return r.collapsed ? undefined : r;
}

// blockAt is the block a place in the index's text is in.
export function blockAt(ix: TextIndex, pos: number): HTMLElement | undefined {
  const at = locate(ix, pos, true);
  return (at?.node.parentElement?.closest(BLOCK) as HTMLElement | null) ?? undefined;
}

// Target is text picked out to comment on: where it is in the index, the
// quote it's kept by, and the markdown's lines it's in.
export interface Target {
  start: number;
  end: number;
  quote: DocQuote;
  line: number;
  endLine: number;
}

const lineOf = (el: Element | undefined, attr: "ls" | "le") => Number((el as HTMLElement | undefined)?.dataset[attr] ?? 0);

// targetOf is [start, end) of the index's text as a target: trimmed of
// space at either end, or nothing if that's all it is.
export function targetOf(ix: TextIndex, start: number, end: number): Target | undefined {
  const t = ix.text;
  while (start < end && /\s/.test(t[start])) start++;
  while (end > start && /\s/.test(t[end - 1])) end--;
  if (start >= end) return undefined;
  const line = lineOf(blockAt(ix, start), "ls");
  const endLine = Math.max(line, lineOf(blockAt(ix, end - 1), "le"));
  if (!line) return undefined;
  return {
    start,
    end,
    line,
    endLine,
    quote: { exact: t.slice(start, end), prefix: t.slice(Math.max(0, start - CONTEXT), start), suffix: t.slice(end, end + CONTEXT) },
  };
}

// targetOfRange is a selection in the document as a target.
export function targetOfRange(ix: TextIndex, r: Range): Target | undefined {
  return targetOf(ix, offsetOf(ix, r.startContainer, r.startOffset), offsetOf(ix, r.endContainer, r.endOffset));
}

// blockSpan is where a block's own words are in the index's text: a list
// item's before the list in it.
export function blockSpan(ix: TextIndex, el: HTMLElement): [number, number] {
  const start = offsetOf(ix, el, 0);
  const inner = el.tagName === "LI" ? el.querySelector(":scope > ul, :scope > ol") : null;
  const end = inner ? offsetOf(ix, inner, 0) : offsetOf(ix, el, el.childNodes.length);
  return [start, end];
}

// blockLines is a block's lines, a list item's without the list in it.
export function blockLines(el: HTMLElement): [number, number] {
  const ls = lineOf(el, "ls");
  const inner = el.tagName === "LI" ? (el.querySelector(":scope > ul, :scope > ol") as HTMLElement | null) : null;
  const le = inner ? Math.max(ls, lineOf(inner, "ls") - 1) : lineOf(el, "le");
  return [ls, le];
}

// Whitespace is compared loosely: the same words wrapped differently in
// the markdown read the same rendered, but their text differs.
interface Loose {
  text: string;
  map: number[]; // a loose character's place in the index's text
}

const looseCache = new WeakMap<TextIndex, Loose>();

function loose(ix: TextIndex): Loose {
  let l = looseCache.get(ix);
  if (l) return l;
  let text = "";
  const map: number[] = [];
  let space = false;
  for (let i = 0; i < ix.text.length; i++) {
    if (/\s/.test(ix.text[i])) {
      space = true;
      continue;
    }
    if (space && text) {
      text += " ";
      map.push(i - 1);
    }
    space = false;
    text += ix.text[i];
    map.push(i);
  }
  l = { text, map };
  looseCache.set(ix, l);
  return l;
}

const squash = (s: string) => s.replace(/\s+/g, " ");

// matchQuote is where a quote is in text whose whitespace runs are one
// space each, as [start, end), or nothing. Of more than one place, the one
// whose surroundings match the quote's best, plus what bonus says of a
// place (its lines, say). The reader and the editor both find quotes by
// it, so they can't highlight different places.
export function matchQuote(text: string, q: { exact: string; prefix?: string; suffix?: string }, bonus?: (at: number) => number): [number, number] | undefined {
  const exact = squash(q.exact).trim();
  if (!exact) return undefined;
  const prefix = squash(q.prefix ?? "");
  const suffix = squash(q.suffix ?? "");
  let best = -1;
  let bestScore = -1;
  for (let i = text.indexOf(exact); i >= 0; i = text.indexOf(exact, i + 1)) {
    let score = bonus?.(i) ?? 0;
    for (let k = 1; k <= prefix.length && i - k >= 0 && text[i - k] === prefix[prefix.length - k]; k++) score++;
    const after = i + exact.length;
    for (let k = 0; k < suffix.length && after + k < text.length && text[after + k] === suffix[k]; k++) score++;
    if (score > bestScore) {
      best = i;
      bestScore = score;
    }
  }
  return best < 0 ? undefined : [best, best + exact.length];
}

// find is where a quote is in the index's text now, or nothing: given its
// lines (on the version it was made on), the place in them wins.
export function find(ix: TextIndex, q: DocQuote, lines?: { line: number; endLine: number }): [number, number] | undefined {
  const l = loose(ix);
  const inLines = (i: number) => {
    const ls = lineOf(blockAt(ix, l.map[i]), "ls");
    return ls >= lines!.line && ls <= lines!.endLine ? 1000 : 0;
  };
  const m = matchQuote(l.text, q, lines ? inLines : undefined);
  return m && [l.map[m[0]], l.map[m[1] - 1] + 1];
}

// blockWithLines is the block at exactly those lines, for a comment on a
// whole block with no words of its own to find it by (a diagram).
export function blockWithLines(root: HTMLElement, line: number, endLine: number): HTMLElement | undefined {
  return (root.querySelector(`${BLOCK}[data-ls="${line}"][data-le="${endLine}"]`) as HTMLElement | null) ?? undefined;
}

// blockLike is, in a version of the document a whole-block comment wasn't
// made on, the block that starts with the line its block started with
// (a diagram's fence, say) nearest where it was, or nothing.
export function blockLike(root: HTMLElement, lines: string[], first: string, near: number): HTMLElement | undefined {
  const want = first.trim();
  if (!want) return undefined;
  let best: HTMLElement | undefined;
  for (const el of root.querySelectorAll<HTMLElement>(BLOCK)) {
    const ls = Number(el.dataset.ls);
    if (lines[ls - 1]?.trim() !== want) continue;
    if (!best || Math.abs(ls - near) < Math.abs(Number(best.dataset.ls) - near)) best = el;
  }
  return best;
}
