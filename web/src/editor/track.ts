import type { Mark, Node as PMNode } from "prosemirror-model";
import { pair, parseMarkdown } from "./markdown";
import type { Origin } from "./mdast";
import { schema } from "./schema";

// Suggested edits shown in place, as tracked changes: the document as it
// would read with them, and what each takes out still there, struck
// through, beside what it puts in. A suggestion is a replacement in the
// markdown (from, exactly once, becomes to); what it changes is found by
// comparing the document before and after, block by block (as the bridge
// pairs them) and, in a block that changed, word by word.

export interface Change {
  id: string; // the thread it's on
  from: string;
  to: string;
}

// Span is part of a block taken out (del) or put in (ins) by a change;
// Whole is a whole block taken out or put in.
export interface Span {
  from: number;
  to: number;
  kind: "del" | "ins";
  id: string;
}

export interface Whole {
  pos: number;
  kind: "del" | "ins";
  id: string;
}

// Tracked is the document to show: its spans and whole blocks, where each
// change ends (to put its accept and decline by), and which changes
// couldn't be placed (their text isn't in the document once, or they
// overlap one placed already).
export interface Tracked {
  doc: PMNode;
  spans: Span[];
  wholes: Whole[];
  ends: { id: string; pos: number; inline: boolean }[];
  placed: string[];
  unplaced: string[];
}

interface Info {
  inline?: { from: number; to: number; kind: "del" | "ins" }[];
  whole?: "del" | "ins";
  id: string;
}

export function trackChanges(text: string, changes: Change[]): Tracked {
  // Where each goes, in the text before and after.
  const spots: { c: Change; a: number; b: number; na: number; nb: number }[] = [];
  const unplaced: string[] = [];
  for (const c of changes) {
    const a = c.from ? text.indexOf(c.from) : -1;
    if (a < 0 || text.indexOf(c.from, a + 1) >= 0) unplaced.push(c.id);
    else spots.push({ c, a, b: a + c.from.length, na: 0, nb: 0 });
  }
  spots.sort((x, y) => x.a - y.a);
  const kept: typeof spots = [];
  for (const s of spots) {
    if (kept.length && s.a < kept[kept.length - 1].b) unplaced.push(s.c.id);
    else kept.push(s);
  }
  let applied = "";
  let from = 0;
  for (const s of kept) {
    applied += text.slice(from, s.a);
    s.na = applied.length;
    applied += s.c.to;
    s.nb = applied.length;
    from = s.b;
  }
  applied += text.slice(from);

  const was = parseMarkdown(text);
  const now = parseMarkdown(applied);
  // whose is a change to a block: the one whose text it's in, before or
  // after (an empty one touches where it was).
  const whose = (o?: Origin, n?: Origin): string | undefined => {
    for (const s of kept) {
      if (o && o.start <= s.b && s.a <= o.end) return s.c.id;
      if (n && n.start <= s.nb && s.na <= n.end) return s.c.id;
    }
    return kept[0]?.c.id;
  };
  const info = new WeakMap<PMNode, Info>();

  const blocks = (nowKids: PMNode[], wasKids: PMNode[]): PMNode[] => {
    const at = pair(nowKids, wasKids);
    const used = new Set(at.filter((k) => k >= 0));
    const out: PMNode[] = [];
    let next = 0; // the next of wasKids not yet out
    const gone = (until: number) => {
      for (; next < until; next++) {
        if (used.has(next)) continue;
        const old = wasKids[next];
        info.set(old, { whole: "del", id: whose(was.origins.get(old)) ?? "" });
        out.push(old);
      }
    };
    nowKids.forEach((kid, i) => {
      const k = at[i];
      if (k >= 0) gone(k);
      const old = k >= 0 ? wasKids[k] : undefined;
      if (k >= 0) next = Math.max(next, k + 1);
      if (old && kid.eq(old)) return out.push(kid);
      const id = whose(old && was.origins.get(old), now.origins.get(kid)) ?? "";
      if (!old) {
        info.set(kid, { whole: "ins", id });
        return out.push(kid);
      }
      if (kid.isTextblock && kid.type === old.type) return out.push(merged(old, kid, id));
      if (!kid.isTextblock && !kid.isAtom && kid.type === old.type && kid.type !== schema.nodes.table) {
        return out.push(kid.type.create(kid.attrs, blocks(childrenOf(kid), childrenOf(old)), kid.marks));
      }
      info.set(old, { whole: "del", id });
      info.set(kid, { whole: "ins", id });
      out.push(old, kid);
    });
    gone(wasKids.length);
    return out;
  };

  // merged is a textblock that changed: its words now, with those taken
  // out beside them, a run of changes at a time (what goes, then what
  // comes). Space, or a word of a letter or two, between two changes is
  // part of them: "no tests of its own" becoming "one test" reads as one
  // change, not five.
  const merged = (old: PMNode, kid: PMNode, id: string): PMNode => {
    const a = tokens(old);
    const b = tokens(kid);
    const L = Array.from({ length: a.length + 1 }, () => new Int32Array(b.length + 1));
    for (let i = a.length - 1; i >= 0; i--) for (let j = b.length - 1; j >= 0; j--) L[i][j] = a[i].key === b[j].key ? L[i + 1][j + 1] + 1 : Math.max(L[i + 1][j], L[i][j + 1]);
    // Runs: the same in both, or changed (what goes, what comes).
    type Run = { same: boolean; dels: PMNode[]; inss: PMNode[] };
    const runs: Run[] = [];
    const run = (same: boolean) => {
      const last = runs[runs.length - 1];
      if (last && last.same === same) return last;
      const r = { same, dels: [], inss: [] };
      runs.push(r);
      return r;
    };
    for (let i = 0, j = 0; i < a.length || j < b.length; ) {
      if (i < a.length && j < b.length && a[i].key === b[j].key) {
        const r = run(true);
        r.dels.push(a[i++].node);
        r.inss.push(b[j++].node);
      } else if (j >= b.length || (i < a.length && L[i + 1][j] >= L[i][j + 1])) run(false).dels.push(a[i++].node);
      else run(false).inss.push(b[j++].node);
    }
    // Small sameness between changes joins them.
    const small = (r: Run) => r.inss.reduce((n, t) => n + (t.text ?? "x").trim().length, 0) <= 2;
    for (let k = runs.length - 2; k > 0; k--) {
      const [before, mid, after] = [runs[k - 1], runs[k], runs[k + 1]];
      if (!mid.same || before.same || after.same || !small(mid)) continue;
      before.dels.push(...mid.dels, ...after.dels);
      before.inss.push(...mid.inss, ...after.inss);
      runs.splice(k, 2);
    }
    const content: PMNode[] = [];
    const inline: NonNullable<Info["inline"]> = [];
    let at = 0;
    const put = (n: PMNode, kind?: "del" | "ins") => {
      content.push(n);
      if (kind) {
        const last = inline[inline.length - 1];
        if (last && last.kind === kind && last.to === at) last.to += n.nodeSize;
        else inline.push({ from: at, to: at + n.nodeSize, kind });
      }
      at += n.nodeSize;
    };
    for (const r of runs) {
      if (r.same) {
        for (const n of r.inss) put(n);
        continue;
      }
      // What's the same at either end of a change (a space joined in) is
      // left out of it.
      let lead = 0;
      while (lead < r.dels.length && lead < r.inss.length && r.dels[lead].eq(r.inss[lead])) lead++;
      let trail = 0;
      while (trail < r.dels.length - lead && trail < r.inss.length - lead && r.dels[r.dels.length - 1 - trail].eq(r.inss[r.inss.length - 1 - trail])) trail++;
      for (const n of r.inss.slice(0, lead)) put(n);
      for (const n of r.dels.slice(lead, r.dels.length - trail)) put(n, "del");
      for (const n of r.inss.slice(lead, r.inss.length - trail)) put(n, "ins");
      for (const n of r.inss.slice(r.inss.length - trail)) put(n);
    }
    const node = kid.type.create(kid.attrs, content, kid.marks);
    info.set(node, { inline, id });
    return node;
  };

  let doc: PMNode;
  try {
    doc = schema.nodes.doc.create(null, blocks(childrenOf(now.doc), childrenOf(was.doc)));
    doc.check();
  } catch {
    // Something the comparison couldn't put together: the document after,
    // unmarked, rather than nothing.
    doc = now.doc;
  }

  const spans: Span[] = [];
  const wholes: Whole[] = [];
  const last = new Map<string, { pos: number; inline: boolean }>();
  doc.descendants((node, pos) => {
    const it = info.get(node);
    if (!it) return true;
    if (it.whole) {
      wholes.push({ pos, kind: it.whole, id: it.id });
      last.set(it.id, { pos: pos + node.nodeSize, inline: false });
      return false;
    }
    // Its Accept and Decline at the end of the block, not mid-sentence.
    for (const sp of it.inline ?? []) spans.push({ from: pos + 1 + sp.from, to: pos + 1 + sp.to, kind: sp.kind, id: it.id });
    if (it.inline?.length) last.set(it.id, { pos: pos + node.nodeSize - 1, inline: true });
    return false;
  });
  return {
    doc,
    spans,
    wholes,
    ends: [...last].map(([id, e]) => ({ id, ...e })),
    placed: kept.map((s) => s.c.id),
    unplaced,
  };
}

function childrenOf(node: PMNode): PMNode[] {
  const out: PMNode[] = [];
  node.forEach((c) => out.push(c));
  return out;
}

// tokens are a textblock's words and the space between them, each with
// its marks, and its other inline nodes (an image, a break) one apiece.
function tokens(tb: PMNode): { node: PMNode; key: string }[] {
  const out: { node: PMNode; key: string }[] = [];
  const marks = (ms: readonly Mark[]) => ms.map((m) => m.type.name + JSON.stringify(m.attrs)).join(",");
  tb.forEach((child) => {
    if (child.isText) {
      for (const piece of child.text!.match(/\s+|[^\s]+/g) ?? []) out.push({ node: schema.text(piece, child.marks), key: piece + "\u0000" + marks(child.marks) });
    } else out.push({ node: child, key: "\u0001" + JSON.stringify(child.toJSON()) });
  });
  return out;
}
