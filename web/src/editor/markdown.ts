import type { Link, Nodes, Root, RootContent } from "mdast";
import { gfmToMarkdown } from "mdast-util-gfm";
import { defaultHandlers, type Handle, toMarkdown } from "mdast-util-to-markdown";
import type { Node as PMNode } from "prosemirror-model";
import remarkParse from "remark-parse";
import { unified } from "unified";
import { syntax } from "../markdown/syntax";
import { fromMdast, type Origin, toMdast, toMdastBlocks } from "./mdast";

// The markdown bridge: a document's markdown in, the editor's document
// out, and back again, changing only what was edited.
//
// Documents are markdown files in git that crew write and threads anchor
// to, so saving one mustn't reformat it. A block that wasn't touched is
// written back as it was, byte for byte, with the space around it. One
// that was is written in the file's own style (its bullets, emphasis,
// fences). Containers are followed down: change one item of a list and
// only that item is written anew; one row of a table; one paragraph of a
// quote.
//
// What's written is parsed again and must come back as the document that
// was saved. If it doesn't (markdown has corners), the next coarser way is
// tried: each changed top-level block written whole, then the whole
// document.
//
// It parses as the reader does (remark, GitHub-flavoured, with front
// matter and maths), so the two agree on where every block starts and
// ends. Front matter and maths have no node of their own here: they're
// kept as written (a raw block, a raw inline), so an edit beside them
// can't reformat them.

const parser = unified().use(remarkParse).use(syntax);

// Markdown is a document as parsed: its text, the editor's document for
// it, where each block came from, and how it's written.
export interface Markdown {
  source: string;
  doc: PMNode;
  origins: WeakMap<PMNode, Origin>;
  style: Style;
}

// Style is how a file writes what markdown lets you write more than one
// way, read from the file: its first bullet list's bullet, its first
// emphasis's marker, and so on.
export interface Style {
  bullet: "-" | "*" | "+";
  bulletOrdered: "." | ")";
  incrementListMarker: boolean;
  emphasis: "*" | "_";
  strong: "*" | "_";
  fence: "`" | "~";
  rule: "-" | "*" | "_";
  ruleRepetition: number;
  ruleSpaces: boolean;
  setext: boolean;
  // Whether tables pad their columns to line up.
  tableAligned: boolean;
  eol: "\n" | "\r\n";
}

// What a new document's written in: what agents write.
export const defaultStyle: Style = {
  bullet: "-",
  bulletOrdered: ".",
  incrementListMarker: true,
  emphasis: "*",
  strong: "*",
  fence: "`",
  rule: "-",
  ruleRepetition: 3,
  ruleSpaces: false,
  setext: false,
  tableAligned: false,
  eol: "\n",
};

export function parseMarkdown(source: string): Markdown {
  const tree = parser.parse(source);
  const origins = new WeakMap<PMNode, Origin>();
  const doc = fromMdast(tree, source, origins);
  doc.check();
  return { source, doc, origins, style: styleOf(tree, source) };
}

// How a save was written: unchanged; block by block, at any depth; top-level
// block by top-level block; or the whole document anew.
export type How = "same" | "blocks" | "top" | "whole";

// Saved is a document as markdown. lossy says it doesn't read back as the
// document saved, written any way: markdown can't say what it holds.
// Taking out a link's definition, say, leaves [its text][ref] as words.
export interface Saved {
  text: string;
  how: How;
  lossy?: true;
}

// saveMarkdown is doc as markdown. Given the markdown it was parsed from,
// only what changed is written anew.
export function saveMarkdown(doc: PMNode, from?: Markdown): Saved {
  if (!from) return { text: whole(doc, defaultStyle), how: "whole" };
  if (doc.eq(from.doc)) return { text: from.source, how: "same" };
  const blocks = new Writer(from, true).doc(doc);
  if (reads(blocks, doc)) return { text: blocks, how: "blocks" };
  const top = new Writer(from, false).doc(doc);
  if (reads(top, doc)) return { text: top, how: "top" };
  const all = whole(doc, from.style);
  if (reads(all, doc)) return { text: all, how: "whole" };
  // Nothing reads back: what changes least.
  return { text: blocks, how: "blocks", lossy: true };
}

// reads is whether text parses as doc.
function reads(text: string, doc: PMNode): boolean {
  try {
    return parseMarkdown(text).doc.eq(doc);
  } catch {
    return false;
  }
}

function whole(doc: PMNode, style: Style): string {
  return stringify(toMdast(doc) as Root, style, style.tableAligned).replace(/\n/g, style.eol);
}

function stringify(tree: Root, style: Style, aligned: boolean): string {
  return toMarkdown(tree, {
    bullet: style.bullet,
    bulletOrdered: style.bulletOrdered,
    incrementListMarker: style.incrementListMarker,
    emphasis: style.emphasis,
    strong: style.strong,
    fence: style.fence,
    fences: true,
    rule: style.rule,
    ruleRepetition: style.ruleRepetition,
    ruleSpaces: style.ruleSpaces,
    setext: style.setext,
    closeAtx: false,
    listItemIndent: "one",
    resourceLink: false,
    extensions: [gfmToMarkdown({ tablePipeAlign: aligned })],
    handlers: { link: bareLink },
  });
}

// bareLink writes a link whose text is its address as just the address,
// as agents write them (GitHub makes it a link), rather than <…>.
const bareLink: Handle = (node, parent, state, info) => {
  const link = node as Link;
  const text = link.children.length === 1 && link.children[0].type === "text" && !link.title ? link.children[0].value : "";
  const url = /^https?:\/\//i.test(text) ? text : /^www\./i.test(text) ? "http://" + text : /^[\w.+-]+@[\w-]+(\.[\w-]+)+$/.test(text) ? "mailto:" + text : "";
  if (url && url === link.url && !/[\s<>]/.test(text) && /(^|[\s*_~(])$/.test(info.before) && /^($|[\s.,:;!?])/.test(info.after)) return text;
  return defaultHandlers.link(link, parent, state, info);
};

// aligned is whether a table's lines (its markdown) line up.
function aligned(table: string): boolean {
  const lines = table.split(/\r?\n/).map((l) => l.trim());
  return lines.length > 1 && lines.every((l) => l.length === lines[0].length);
}

function styleOf(tree: Root, src: string): Style {
  const s: Partial<Style> = {};
  const visit = (n: Nodes) => {
    const p = n.position;
    const o = p?.start.offset;
    if (p && o !== undefined) {
      const c = src[o];
      switch (n.type) {
        case "list": {
          const first = n.children[0]?.position?.start.offset;
          if (first === undefined) break;
          if (!n.ordered) {
            const b = src[first];
            if (b === "-" || b === "*" || b === "+") s.bullet ??= b;
            break;
          }
          const m = /^(\d{1,9})([.)])/.exec(src.slice(first, first + 11));
          if (!m) break;
          s.bulletOrdered ??= m[2] as Style["bulletOrdered"];
          const second = n.children[1]?.position?.start.offset;
          const m2 = second === undefined ? null : /^\d{1,9}/.exec(src.slice(second, second + 10));
          if (m2) s.incrementListMarker ??= Number(m2[0]) !== Number(m[1]);
          break;
        }
        case "emphasis":
          if (c === "*" || c === "_") s.emphasis ??= c;
          break;
        case "strong":
          if (c === "*" || c === "_") s.strong ??= c;
          break;
        case "code":
          if (c === "`" || c === "~") s.fence ??= c;
          break;
        case "thematicBreak": {
          const t = src.slice(o, p.end.offset).trimEnd();
          const r = t[0];
          if (!s.rule && (r === "-" || r === "*" || r === "_")) {
            s.rule = r;
            s.ruleRepetition = Math.max(3, t.split(r).length - 1);
            s.ruleSpaces = /\s/.test(t);
          }
          break;
        }
        case "heading":
          if (n.depth <= 2) s.setext ??= c !== "#";
          break;
        case "table":
          s.tableAligned ??= aligned(src.slice(o, p.end.offset));
          break;
      }
    }
    if ("children" in n) for (const child of n.children) visit(child);
  };
  visit(tree);
  return { ...defaultStyle, ...s, eol: src.includes("\r\n") ? "\r\n" : "\n" };
}

// Kind is the kind of container blocks are written in: what goes between
// two of them depends on it.
type Kind = "root" | "quote" | "item" | "list";

// Writer writes a document edited from one parsed from markdown, keeping
// what it can of that markdown. Deep, it follows containers down (lists,
// items, quotes, tables); not, it keeps or rewrites whole top-level
// blocks.
class Writer {
  private readonly src: string;
  private readonly eol: string;

  constructor(
    private readonly from: Markdown,
    private readonly deep: boolean,
  ) {
    this.src = from.source;
    this.eol = from.style.eol;
  }

  private at(n: PMNode | null | undefined): Origin | undefined {
    return n ? this.from.origins.get(n) : undefined;
  }

  doc(doc: PMNode): string {
    const was = this.from.doc;
    const first = this.at(was.firstChild);
    const last = this.at(was.lastChild);
    const lead = first ? this.src.slice(0, first.start) : "";
    const tail = last ? this.src.slice(last.end) : this.eol;
    return lead + this.children(doc, was, "", "root") + tail;
  }

  // children writes a container's blocks: each kept, or written anew, with
  // what was between them where two were next to each other before.
  // prefix is what starts each of their lines after the first: a quote's
  // "> ", a list item's indent.
  private children(now: PMNode, was: PMNode, prefix: string, kind: Kind): string {
    const kids = childrenOf(now);
    const olds = childrenOf(was);
    const at = pair(kids, olds);
    let out = "";
    let lastList: { type: string; mark: string } | undefined;
    kids.forEach((kid, i) => {
      if (i > 0) out += this.between(olds, at[i - 1], at[i]) ?? this.join(now, prefix, kind);
      const old = at[i] >= 0 ? olds[at[i]] : undefined;
      if (kind === "list") {
        out += this.item(kid, old, prefix, now, was, i);
        return;
      }
      // Two lists of a kind, one after the other, read as one list: the
      // second takes the other bullet (or number's dot or bracket).
      const mark = this.markOf(kid, old);
      if (mark && lastList?.type === kid.type.name && lastList.mark === mark) {
        const other = { "-": "*", "*": "-", "+": "-", ".": ")", ")": "." }[mark]!;
        out += this.fresh(kid, prefix, old, other);
        lastList = { type: kid.type.name, mark: other };
        return;
      }
      out += this.block(kid, old, prefix);
      lastList = mark ? { type: kid.type.name, mark } : undefined;
    });
    return out;
  }

  // markOf is a list's bullet, or its numbers' dot or bracket, as it'll be
  // written; undefined for a block that isn't a list.
  private markOf(node: PMNode, old: PMNode | undefined): string | undefined {
    const o = this.at(old);
    const text = o ? this.src.slice(o.start, o.start + 11) : "";
    if (node.type.name === "bullet_list") return /^[-*+]/.exec(text)?.[0] ?? this.from.style.bullet;
    if (node.type.name === "ordered_list") return /^\d{1,9}([.)])/.exec(text)?.[1] ?? this.from.style.bulletOrdered;
    return undefined;
  }

  // between is what was between blocks k0 and k1, if they were next to
  // each other.
  private between(olds: PMNode[], k0: number, k1: number): string | undefined {
    if (k0 < 0 || k1 !== k0 + 1) return undefined;
    const a = this.at(olds[k0]);
    const b = this.at(olds[k1]);
    return a && b ? this.src.slice(a.end, b.start) : undefined;
  }

  // join is what goes between two blocks the file has nothing to say about:
  // a blank line, or (in a tight list) a line break.
  private join(container: PMNode, prefix: string, kind: Kind): string {
    const blank = this.eol + prefix.trimEnd() + this.eol + prefix;
    if ((kind === "list" || kind === "item") && !container.attrs.spread) return this.eol + prefix;
    return blank;
  }

  private block(now: PMNode, old: PMNode | undefined, prefix: string): string {
    const o = this.at(old);
    if (old && o) {
      if (now.eq(old)) return this.src.slice(o.start, o.end);
      if (this.deep && now.sameMarkup(old)) {
        let kept: string | undefined;
        switch (now.type.name) {
          case "bullet_list":
          case "ordered_list":
            kept = this.children(now, old, prefix, "list");
            break;
          case "blockquote":
            kept = this.quote(now, old, o, prefix);
            break;
          case "table":
            if (!aligned(this.src.slice(o.start, o.end))) kept = this.table(now, old, prefix);
            break;
          case "cell":
            kept = this.cell(now, old, o, prefix);
            break;
        }
        if (kept !== undefined) return kept;
      }
    }
    return this.fresh(now, prefix, old);
  }

  // cell writes a cell changed inside between its comments as they were.
  private cell(now: PMNode, old: PMNode, o: Origin, prefix: string): string | undefined {
    const first = this.at(old.firstChild);
    const last = this.at(old.lastChild);
    if (!first || !last || !now.childCount) return undefined;
    return this.src.slice(o.start, first.start) + this.children(now, old, prefix, "root") + this.src.slice(last.end, o.end);
  }

  private quote(now: PMNode, old: PMNode, o: Origin, prefix: string): string | undefined {
    const first = this.at(old.firstChild);
    if (!first) return undefined;
    const marker = this.src.slice(o.start, first.start);
    if (!/^> ?$/.test(marker)) return undefined;
    return marker + this.children(now, old, prefix + marker, "quote");
  }

  // item writes a list's ith item. One that's changed inside keeps its
  // marker (renumbered, if its list counts up and it's moved).
  private item(now: PMNode, old: PMNode | undefined, prefix: string, list: PMNode, listWas: PMNode, i: number): string {
    const o = this.at(old);
    if (old && o) {
      if (now.eq(old)) return this.renumber(this.src.slice(o.start, o.end), list, listWas, i);
      const first = this.at(old.firstChild);
      if (this.deep && first && now.sameMarkup(old)) {
        const was = this.src.slice(o.start, first.start);
        // The bullet or number and the 1–4 spaces after it: the indent of
        // the item's lines (a task's [ ] isn't part of it).
        const m = /^(?:[-*+]|\d{1,9}[.)])( {1,4})(?! )/.exec(was);
        if (m) {
          const marker = this.renumber(was, list, listWas, i);
          const indent = m[0].length + marker.length - was.length;
          return marker + this.children(now, old, prefix + " ".repeat(indent), "item");
        }
      }
    }
    // New, or rewritten: a list of just it.
    const ordered = list.type.name === "ordered_list";
    const one = list.type.create(ordered ? { ...list.attrs, start: this.number(list, listWas, i) } : list.attrs, now);
    return this.fresh(one, prefix, listWas);
  }

  private counts(listWas: PMNode): boolean {
    const num = (n: PMNode) => {
      const o = this.at(n);
      const m = o && /^\d{1,9}/.exec(this.src.slice(o.start, o.start + 10));
      return m ? Number(m[0]) : undefined;
    };
    if (listWas.childCount > 1) {
      const a = num(listWas.child(0));
      const b = num(listWas.child(1));
      if (a !== undefined && b !== undefined) return a !== b;
    }
    return this.from.style.incrementListMarker;
  }

  private number(list: PMNode, listWas: PMNode, i: number): number {
    return this.counts(listWas) ? list.attrs.start + i : list.attrs.start;
  }

  private renumber(text: string, list: PMNode, listWas: PMNode, i: number): string {
    if (list.type.name !== "ordered_list" || !this.counts(listWas)) return text;
    return text.replace(/^\d{1,9}/, String(list.attrs.start + i));
  }

  // table writes a table row by row, keeping its delimiter row (the
  // |---|---| under the header) as it was.
  private table(now: PMNode, was: PMNode, prefix: string): string | undefined {
    const head = this.at(was.firstChild);
    if (!head) return undefined;
    const lineStart = this.src.indexOf("\n", head.end) + 1;
    if (lineStart === 0) return undefined;
    let lineEnd = this.src.indexOf("\n", lineStart);
    if (lineEnd < 0) lineEnd = this.src.length;
    if (this.src[lineEnd - 1] === "\r") lineEnd--;
    const delimiter = this.src.slice(lineStart, lineEnd);
    const rows = childrenOf(now);
    const olds = childrenOf(was);
    const at = pair(rows, olds);
    let out = "";
    rows.forEach((row, i) => {
      if (i === 1) {
        const b = at[1] === 1 ? this.at(olds[1]) : undefined;
        out += this.eol + delimiter + (b ? this.src.slice(lineEnd, b.start) : this.eol + prefix);
      } else if (i > 1) {
        out += this.between(olds, at[i - 1], at[i]) ?? this.eol + prefix;
      }
      const old = at[i] >= 0 ? olds[at[i]] : undefined;
      const o = this.at(old);
      if (old && o && row.eq(old)) out += this.src.slice(o.start, o.end);
      else out += stringify({ type: "root", children: [toMdast(now.type.create(now.attrs, row)) as RootContent] }, this.from.style, false).split("\n")[0];
    });
    if (rows.length === 1) out += this.eol + delimiter;
    return out;
  }

  // fresh writes a block anew, in the file's style (and its list's
  // bullet, if it was one, or mark, if given), its lines after the first
  // prefixed.
  private fresh(node: PMNode, prefix: string, old?: PMNode, mark?: string): string {
    let style = this.from.style;
    let tableAligned = style.tableAligned;
    const o = this.at(old);
    if (o) {
      const text = this.src.slice(o.start, o.end);
      if (node.type.name === "bullet_list" && /^[-*+]/.test(text)) style = { ...style, bullet: text[0] as Style["bullet"] };
      if (node.type.name === "ordered_list") {
        const m = /^\d{1,9}([.)])/.exec(text);
        if (m) style = { ...style, bulletOrdered: m[1] as Style["bulletOrdered"], incrementListMarker: this.counts(old!) };
      }
      if (node.type.name === "table") tableAligned = aligned(text);
    }
    if (mark && node.type.name === "bullet_list") style = { ...style, bullet: mark as Style["bullet"] };
    if (mark && node.type.name === "ordered_list") style = { ...style, bulletOrdered: mark as Style["bulletOrdered"] };
    let text = stringify({ type: "root", children: toMdastBlocks(node) }, style, tableAligned).replace(/\n+$/, "");
    if (o && node.type.name === "paragraph") text = rewrap(text, this.src.slice(o.start, o.end), prefix);
    return text
      .split("\n")
      .map((line, i) => (i === 0 ? line : line === "" ? prefix.trimEnd() : prefix + line))
      .join(this.eol);
  }
}

// rewrap breaks a paragraph written anew (on one line) where it broke
// before: after the same words, where they're still there. Markdown joins
// a paragraph's lines with spaces, so where they break is the writer's, to
// keep: an edit to one word changes one line, not the paragraph. was is
// its markdown before, lines after the first starting with prefix (a
// quote's > or an item's indent). A line isn't started with what would
// start a block (a bullet, a number, #, >).
export function rewrap(fresh: string, was: string, prefix: string): string {
  const lines = was.split(/\r?\n/);
  if (lines.length < 2) return fresh;
  const quotes = (prefix.match(/>/g) ?? []).length;
  const unquote = new RegExp(`^(\\s*>){${quotes}}`);
  const old: string[] = [];
  const breakAfter = new Set<number>();
  lines.forEach((line, i) => {
    const body = i === 0 || !quotes ? line : line.replace(unquote, "");
    for (const w of body.trim().split(/\s+/)) if (w) old.push(w);
    if (i < lines.length - 1 && old.length) breakAfter.add(old.length - 1);
  });
  // Words at even places, what's between them at odd: a space to break
  // at, or a line break of its own (a hard break) to keep.
  const parts = fresh.split(/(\s+)/);
  const words = parts.filter((_, i) => i % 2 === 0);
  if (old.length * words.length > 400_000) return fresh;
  // Which old word each new one is (a longest common subsequence).
  const n = words.length;
  const m = old.length;
  const L = Array.from({ length: n + 1 }, () => new Int32Array(m + 1));
  for (let i = n - 1; i >= 0; i--) for (let j = m - 1; j >= 0; j--) L[i][j] = words[i] === old[j] ? L[i + 1][j + 1] + 1 : Math.max(L[i + 1][j], L[i][j + 1]);
  const match = new Array<number>(n).fill(-1);
  for (let i = 0, j = 0; i < n && j < m; ) {
    if (words[i] === old[j]) match[i++] = j++;
    else if (L[i + 1][j] >= L[i][j + 1]) i++;
    else j++;
  }
  const newOf = new Array<number>(m).fill(-1);
  match.forEach((j, i) => j >= 0 && (newOf[j] = i));
  // A break after the same word; or, if that word's changed, before the
  // next line's first, if that hasn't.
  const breaks = new Set<number>();
  for (const k of breakAfter) {
    if (newOf[k] >= 0) breaks.add(newOf[k]);
    else if (k + 1 < m && newOf[k + 1] > 0) breaks.add(newOf[k + 1] - 1);
  }
  const blockStart = /^(?:[-+*]$|\d{1,9}[.)]$|#{1,6}$|>|=+$|-+$|_+$|\*+$|`{3,}|~{3,}|\||<)/;
  for (const i of breaks) {
    const sep = parts[2 * i + 1];
    if (i < n - 1 && !sep.includes("\n") && !blockStart.test(words[i + 1])) parts[2 * i + 1] = "\n";
  }
  return parts.join("");
}

function childrenOf(node: PMNode): PMNode[] {
  const out: PMNode[] = [];
  node.forEach((c) => out.push(c));
  return out;
}

// pair matches a container's blocks now with those it had: for each, the
// index of the same block before (by a longest common subsequence), or,
// where blocks changed, of the one of its kind in its place; -1 for one
// that's new.
export function pair(now: PMNode[], was: PMNode[]): number[] {
  const at = new Array<number>(now.length).fill(-1);
  let a = 0;
  let b = 0;
  while (a < now.length && b < was.length && now[a].eq(was[b])) at[a++] = b++;
  let c = now.length;
  let d = was.length;
  while (c > a && d > b && now[c - 1].eq(was[d - 1])) at[--c] = --d;
  const n = c - a;
  const m = d - b;
  const same = Array.from({ length: n }, (_, i) => Uint8Array.from({ length: m }, (_, j) => (now[a + i].eq(was[b + j]) ? 1 : 0)));
  const L = Array.from({ length: n + 1 }, () => new Int32Array(m + 1));
  for (let i = n - 1; i >= 0; i--) for (let j = m - 1; j >= 0; j--) L[i][j] = same[i][j] ? L[i + 1][j + 1] + 1 : Math.max(L[i + 1][j], L[i][j + 1]);
  let added: number[] = [];
  let gone: number[] = [];
  const settle = () => {
    for (let k = 0; k < added.length && k < gone.length; k++) if (now[added[k]].type === was[gone[k]].type) at[added[k]] = gone[k];
    added = [];
    gone = [];
  };
  let i = 0;
  let j = 0;
  while (i < n || j < m) {
    if (i < n && j < m && same[i][j]) {
      settle();
      at[a + i++] = b + j++;
    } else if (j >= m || (i < n && L[i + 1][j] >= L[i][j + 1])) added.push(a + i++);
    else gone.push(b + j++);
  }
  settle();
  return at;
}
