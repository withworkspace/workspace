import { getJSON, type DiffFile } from "../api";
import { canExpand, gaps, type Row } from "./model";

// Code navigation in a review: which words are names, where one leads, and
// where in the diff that is. The server answers (internal/nav); this is
// asking it, and placing what it says.

// A name on a line: from and to in UTF-16 units (a JS string's), and its
// part: r a use, m a member, d a definition, i part of an import.
export type Named = [from: number, to: number, part: "r" | "m" | "d" | "i"];

export interface NavNames {
  lang: string;
  why?: string;
  broken?: boolean;
  lines: Record<string, Named[]>;
}

export interface NavTarget {
  path: string;
  line: number;
  col: number;
  endLine: number;
  kind: string;
  container?: string;
  signature?: string;
  doc?: string;
}

export interface NavAnswer {
  name: string;
  answer: "resolved" | "candidates" | "external" | "builtin" | "unresolved";
  how?: string;
  targets: NavTarget[];
  external?: { module?: string; name?: string; url?: string };
  partial?: boolean;
}

export interface NavRef {
  path: string;
  line: number;
  col: number;
  text: string;
  maybe?: boolean;
}

export interface NavReferences {
  name: string;
  def?: NavTarget;
  uses: NavRef[];
  partial?: boolean;
}

// Side is which commit a name is read at: the diff's base or its head.
export type Side = "old" | "new";

// Place is a name's place: a file at a commit, a line (from 1) and a column.
export interface Place {
  rev: string;
  path: string;
  line: number;
  col: number;
}

// getNavNow is the commit a worktree's files are as they are now: its
// HEAD, or one made of what's not committed (GET …/nav/now), for the
// explorer to ask names at.
export const getNavNow = (api: string) => getJSON<{ rev: string }>(`${api}/nav/now`).then((n) => n.rev);

const names = new Map<string, Promise<NavNames>>();

// getNames is a file's names at a commit, asked once.
export function getNames(api: string, rev: string, path: string): Promise<NavNames> {
  const key = `${api}\u0000${rev}\u0000${path}`;
  let p = names.get(key);
  if (!p) {
    p = getJSON<NavNames>(`${api}/nav/names?${new URLSearchParams({ rev, path })}`);
    p.catch(() => names.delete(key));
    names.set(key, p);
  }
  return p;
}

const answers = new Map<string, Promise<NavAnswer>>();

// getDefinition is where the name at a place leads, asked once.
export function getDefinition(api: string, at: Place): Promise<NavAnswer> {
  const key = `${api}\u0000${at.rev}\u0000${at.path}\u0000${at.line}\u0000${at.col}`;
  let p = answers.get(key);
  if (!p) {
    const q = new URLSearchParams({ rev: at.rev, path: at.path, line: String(at.line), col: String(at.col) });
    p = getJSON<NavAnswer>(`${api}/nav/definition?${q}`);
    p.catch(() => answers.delete(key));
    answers.set(key, p);
  }
  return p;
}

const references = new Map<string, Promise<NavReferences>>();

// getReferences is who uses the definition at a place, asked once.
export function getReferences(api: string, at: Place): Promise<NavReferences> {
  const key = `${api}\u0000${at.rev}\u0000${at.path}\u0000${at.line}\u0000${at.col}`;
  let p = references.get(key);
  if (!p) {
    const q = new URLSearchParams({ rev: at.rev, path: at.path, line: String(at.line), col: String(at.col) });
    p = getJSON<NavReferences>(`${api}/nav/references?${q}`);
    p.catch(() => references.delete(key));
    references.set(key, p);
  }
  return p;
}

// UseGroup is a group of uses as the panel lists them: in this change,
// or elsewhere; maybes apart.
export interface UseGroup {
  title: string;
  uses: NavRef[];
  maybes: NavRef[];
}

// groupUses puts uses in the change first (on a changed line, then
// elsewhere in a changed file), then the rest by file.
export function groupUses(uses: NavRef[], inChange: (path: string) => boolean, changed: (path: string, line: number) => boolean): UseGroup[] {
  const order = (a: NavRef, b: NavRef) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.line - b.line);
  const here = uses.filter((u) => inChange(u.path)).sort((a, b) => Number(changed(b.path, b.line)) - Number(changed(a.path, a.line)) || order(a, b));
  const there = uses.filter((u) => !inChange(u.path)).sort(order);
  const group = (title: string, list: NavRef[]): UseGroup => ({ title, uses: list.filter((u) => !u.maybe), maybes: list.filter((u) => u.maybe) });
  return [group("In this change", here), group("Elsewhere", there)].filter((g) => g.uses.length || g.maybes.length);
}

// nameAt is the name on a line at a column, if one's there.
export function nameAt(n: NavNames | undefined, line: number, col: number): Named | undefined {
  return n?.lines[String(line)]?.find(([from, to]) => col >= from && col < to);
}

// rowOf is where a line of a file is in the diff, on a side: its row's
// key, or undefined when it isn't shown (folded into a gap, or not in the
// diff at all).
export function rowOf(rows: Row[], side: Side, line: number): string | undefined {
  for (const r of rows) {
    if (r.type === "line" && (side === "new" ? r.right : r.left)?.num === line) return r.key;
  }
  return undefined;
}

// gapOf is the gap a line is folded into, if any: its index among the
// file's gaps, and how many lines it hides (unknown for the last, before
// the file's been read).
export function gapOf(f: DiffFile, blob: string[] | undefined, side: Side, line: number): { gap: number; size?: number } | undefined {
  if (!canExpand(f)) return undefined;
  const gs = gaps(f, blob);
  for (let i = 0; i < gs.length; i++) {
    const g = gs[i];
    const n = side === "new" ? line : line - g.offset;
    if (n >= g.start && (g.end === undefined || n <= g.end)) return { gap: i, size: g.end === undefined ? undefined : g.end - g.start + 1 };
  }
  return undefined;
}

// textOffset is how far into an element's text a point in it is, in
// UTF-16 units: the column of a caret in a line of code.
export function textOffset(root: Node, node: Node, offset: number): number | undefined {
  let n = 0;
  const walk = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  for (let t = walk.nextNode(); t; t = walk.nextNode()) {
    if (t === node) return n + offset;
    n += t.textContent?.length ?? 0;
  }
  // A point on an element, not in its text: count the text before it.
  if (node.nodeType === Node.ELEMENT_NODE && root.contains(node)) {
    const before = document.createRange();
    before.setStart(root, 0);
    before.setEnd(node, offset);
    return before.toString().length;
  }
  return undefined;
}

// rangeOf is a stretch of an element's text, as a Range: from and to in
// UTF-16 units.
export function rangeOf(root: Node, from: number, to: number): Range | undefined {
  const r = document.createRange();
  let n = 0;
  let started = false;
  const walk = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  for (let t = walk.nextNode(); t; t = walk.nextNode()) {
    const len = t.textContent?.length ?? 0;
    if (!started && from <= n + len) {
      r.setStart(t, from - n);
      started = true;
    }
    if (started && to <= n + len) {
      r.setEnd(t, to - n);
      return r;
    }
    n += len;
  }
  return undefined;
}

// caretAt is the text node and offset under a point on the page.
export function caretAt(x: number, y: number): { node: Node; offset: number } | undefined {
  const d = document as Document & {
    caretPositionFromPoint?: (x: number, y: number) => { offsetNode: Node; offset: number } | null;
    caretRangeFromPoint?: (x: number, y: number) => Range | null;
  };
  if (d.caretPositionFromPoint) {
    const p = d.caretPositionFromPoint(x, y);
    return p ? { node: p.offsetNode, offset: p.offset } : undefined;
  }
  const r = d.caretRangeFromPoint?.(x, y);
  return r ? { node: r.startContainer, offset: r.startOffset } : undefined;
}

// howWords is how an answer was found, as the card says it.
export function answerWords(a: NavAnswer): string {
  switch (a.answer) {
    case "candidates":
      return `${a.targets.length}${a.partial ? "+" : ""} named ${a.name}`;
    case "external":
      return a.external?.module ? `External: ${a.external.module}` : "External";
    case "builtin":
      return "Built in";
    case "unresolved":
      return "Not found";
  }
  return a.how ?? "";
}
