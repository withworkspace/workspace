import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { Node as PMNode } from "prosemirror-model";
import { Transform } from "prosemirror-transform";
import { describe, expect, it } from "vitest";
import { type Markdown, parseMarkdown, saveMarkdown } from "./markdown";
import { schema } from "./schema";

// The markdown bridge is tested on every markdown file in the repo (most
// written by agents) and on one written to have every corner. That's most
// of the web UI's tests' time, and a file's tests run one after another, so
// the documents are dealt out to SHARDS files (markdown.documents-N.test.ts)
// that run side by side: each calls everyDocument with its number.
export const SHARDS = 6;

const repo = join(import.meta.dirname, "../../..");
export const tricky = readFileSync(join(import.meta.dirname, "testdata/tricky.md"), "utf8");

function corpus(): [string, string][] {
  return [
    ["tricky.md", tricky],
    ["tricky.md (CRLF)", tricky.replace(/\n/g, "\r\n")],
    ...execFileSync("git", ["ls-files", "*.md", ":!:.ground", ":!:web/src"], { cwd: repo, encoding: "utf8" })
      .split("\n")
      .filter(Boolean)
      .map((p): [string, string] => [p, readFileSync(join(repo, p), "utf8")]),
  ];
}

// shard is the documents shard n (from 1) tries. A document's tests take
// about as long as it is long, up to where tries caps them, so the longest
// go first, each to the shard with the least markdown so far: MANUAL.md,
// the slowest by far, has a shard to itself.
function shard(n: number): [string, string][] {
  const shards: { size: number; docs: [string, string][] }[] = Array.from({ length: SHARDS }, () => ({ size: 0, docs: [] }));
  for (const doc of corpus().sort((a, b) => b[1].length - a[1].length || a[0].localeCompare(b[0]))) {
    const least = shards.reduce((a, b) => (b.size < a.size ? b : a));
    least.size += doc[1].length;
    least.docs.push(doc);
  }
  return shards[n - 1].docs;
}

// typed is doc with words typed at the start of the textblock at pos.
export function typed(doc: PMNode, pos: number, words = "EDIT "): PMNode {
  return new Transform(doc).insert(pos + 1, schema.text(words)).doc;
}

// region is the markdown an edit to the textblock at pos may change: its
// own, or its row's in a table (its table's, if the table lines up).
function region(md: Markdown, pos: number): { start: number; end: number } | undefined {
  const $pos = md.doc.resolve(pos + 1);
  for (let d = $pos.depth; d > 0; d--) {
    const node = $pos.node(d);
    if (node.type.name === "table_row") {
      const table = $pos.node(d - 1);
      const t = md.origins.get(table)!;
      const lines = md.source.slice(t.start, t.end).split(/\r?\n/).map((l) => l.trim());
      return lines.every((l) => l.length === lines[0].length) ? t : md.origins.get(node);
    }
    if (node.isTextblock) return md.origins.get(node);
  }
  return undefined;
}

function textblocks(doc: PMNode): number[] {
  const out: number[] = [];
  doc.descendants((node, pos) => {
    if (node.isTextblock) out.push(pos);
    return !node.isTextblock;
  });
  return out;
}

// tries is how many of a document's blocks a test tries, when trying one
// saves the document saves times: most, unless that would parse more than
// a megabyte of markdown in all. Each save parses the whole document again
// (saveMarkdown checks that what it wrote reads back), and that's nearly
// all of a save's time: about 1ms a KB, 150ms for MANUAL.md at 156KB. So a
// long document is tried at fewer blocks, down to three, and one of up to
// 50KB at as many as ever.
function tries(src: string, most: number, saves: number): number {
  return Math.max(3, Math.min(most, Math.floor(1_000_000 / (saves * src.length))));
}

// everyDocument is the bridge's tests on shard n's documents.
export function everyDocument(n: number) {
  const docs = shard(n);

  // A long document's tests are slow (see tries), and several times slower on
  // a busy machine: MANUAL.md's blocks coming and going took 4s alone, against
  // the default 5s, and 6–8s with the load at 16 (18s at 50). Past three
  // blocks tried, a document's tests also grow as it does. They run without
  // waiting on anything, so a timeout can't stop one early, only fail it once
  // it's done: 60s costs nothing and leaves room for both.
  describe("the markdown bridge, on every document", { timeout: 60_000 }, () => {
    if (n === 1)
      it("has a file for every shard", () => {
        const missing = Array.from({ length: SHARDS }, (_, i) => `markdown.documents-${i + 1}.test.ts`).filter((f) => !existsSync(join(import.meta.dirname, f)));
        expect(missing).toEqual([]);
      });

    it.each(docs)("%s: loses nothing written anew", (_, src) => {
      const md = parseMarkdown(src);
      const { text } = saveMarkdown(md.doc);
      expect(parseMarkdown(text).doc.toJSON()).toEqual(md.doc.toJSON());
    });
  
    it.each(docs)("%s: saves unchanged as it was", (_, src) => {
      const md = parseMarkdown(src);
      const copy = PMNode.fromJSON(schema, md.doc.toJSON());
      expect(saveMarkdown(copy, md)).toEqual({ text: src, how: "same" });
    });
  
    it.each(docs)("%s: an edit to a block changes only that block", (_, src) => {
      const md = parseMarkdown(src);
      const blocks = textblocks(md.doc);
      // Long documents: twenty blocks, spread through it, fewer in the longest
      // (see tries). (saveMarkdown checks that what it writes reads back as
      // doc: "blocks" says it did.)
      const step = Math.max(1, Math.floor(blocks.length / tries(src, 20, 1)));
      const wrong: string[] = [];
      for (let i = 0; i < blocks.length; i += step) {
        const pos = blocks[i];
        const r = region(md, pos);
        if (!r) continue;
        const doc = typed(md.doc, pos);
        const { text, how } = saveMarkdown(doc, md);
        const line = src.slice(0, r.start).split("\n").length;
        if (how !== "blocks") wrong.push(`line ${line}: written ${how}`);
        else if (!text.startsWith(src.slice(0, r.start)) || !text.endsWith(src.slice(r.end))) wrong.push(`line ${line}: changed more than its block`);
      }
      expect(wrong).toEqual([]);
    });
  
    it.each(docs)("%s: blocks come and go block by block", (_, src) => {
      const md = parseMarkdown(src);
      const tops: number[] = [];
      md.doc.forEach((_, pos) => tops.push(pos));
      const step = Math.max(1, Math.floor(tops.length / tries(src, 10, 2)));
      const wrong: string[] = [];
      for (let i = 0; i < tops.length; i += step) {
        const pos = tops[i];
        const node = md.doc.nodeAt(pos)!;
        const line = src.slice(0, md.origins.get(node)?.start ?? 0).split("\n").length;
        const added = new Transform(md.doc).insert(pos + node.nodeSize, schema.nodes.paragraph.create(null, schema.text("New words."))).doc;
        const after = saveMarkdown(added, md);
        if (after.how !== "blocks") wrong.push(`line ${line}: a paragraph after it was written ${after.how}`);
        if (md.doc.childCount > 1) {
          const gone = saveMarkdown(new Transform(md.doc).delete(pos, pos + node.nodeSize).doc, md);
          if (gone.how !== "blocks") wrong.push(`line ${line}: removing it was written ${gone.how}`);
        }
      }
      expect(wrong).toEqual([]);
    });
  });
}
