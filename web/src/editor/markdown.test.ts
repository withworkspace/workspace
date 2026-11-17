import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { Node as PMNode } from "prosemirror-model";
import { Transform } from "prosemirror-transform";
import { describe, expect, it } from "vitest";
import { type Markdown, parseMarkdown, saveMarkdown } from "./markdown";
import { schema } from "./schema";

// The markdown bridge is tested on every markdown file in the repo (most
// written by agents) and on one written to have every corner.

const repo = join(import.meta.dirname, "../../..");
const tricky = readFileSync(join(import.meta.dirname, "testdata/tricky.md"), "utf8");

const corpus: [string, string][] = [
  ["tricky.md", tricky],
  ["tricky.md (CRLF)", tricky.replace(/\n/g, "\r\n")],
  ...execFileSync("git", ["ls-files", "*.md", ":!:.ground", ":!:web/src"], { cwd: repo, encoding: "utf8" })
    .split("\n")
    .filter(Boolean)
    .map((p): [string, string] => [p, readFileSync(join(repo, p), "utf8")]),
];

// edited is doc with words typed at the start of the textblock at pos.
function typed(doc: PMNode, pos: number, words = "EDIT "): PMNode {
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

// find is the position of the first node that passes.
function find(doc: PMNode, test: (n: PMNode) => boolean): number {
  let at = -1;
  doc.descendants((node, pos) => {
    if (at < 0 && test(node)) at = pos;
    return at < 0;
  });
  if (at < 0) throw new Error("not found");
  return at;
}

// replaced is md with the first `from` in its document's text replaced.
const replaced = (m: Markdown, from: string, to: string) => {
  let at = -1;
  m.doc.descendants((node, pos) => {
    if (at < 0 && node.isText && node.text!.includes(from)) at = pos + node.text!.indexOf(from);
    return at < 0;
  });
  return new Transform(m.doc).replaceWith(at, at + from.length, schema.text(to)).doc;
};

const hasText = (text: string) => (n: PMNode) => n.isTextblock && n.textContent.includes(text);

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

// A long document's tests are slow (see tries), and several times slower on
// a busy machine: MANUAL.md's blocks coming and going took 4s alone, against
// the default 5s, and 6–8s with the load at 16 (18s at 50). Past three
// blocks tried, a document's tests also grow as it does. They run without
// waiting on anything, so a timeout can't stop one early, only fail it once
// it's done: 60s costs nothing and leaves room for both.
describe("the markdown bridge, on every document", { timeout: 60_000 }, () => {
  it.each(corpus)("%s: loses nothing written anew", (_, src) => {
    const md = parseMarkdown(src);
    const { text } = saveMarkdown(md.doc);
    expect(parseMarkdown(text).doc.toJSON()).toEqual(md.doc.toJSON());
  });

  it.each(corpus)("%s: saves unchanged as it was", (_, src) => {
    const md = parseMarkdown(src);
    const copy = PMNode.fromJSON(schema, md.doc.toJSON());
    expect(saveMarkdown(copy, md)).toEqual({ text: src, how: "same" });
  });

  it.each(corpus)("%s: an edit to a block changes only that block", (_, src) => {
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

  it.each(corpus)("%s: blocks come and go block by block", (_, src) => {
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

describe("the markdown bridge, edit by edit", () => {
  const md = parseMarkdown(tricky);
  // after is the document with f applied, saved: what changed, as the
  // lines around it.
  const save = (doc: PMNode) => {
    const out = saveMarkdown(doc, md);
    expect(parseMarkdown(out.text).doc.eq(doc)).toBe(true);
    return out;
  };

  it("writes new emphasis in the file's style", () => {
    const pos = find(md.doc, hasText("A paragraph with"));
    const start = pos + 1 + md.doc.nodeAt(pos)!.textContent.indexOf("a soft break");
    const doc = new Transform(md.doc).addMark(start, start + "a soft break".length, schema.marks.strong.create()).doc;
    const { text, how } = save(doc);
    expect(how).toBe("blocks");
    expect(text).toContain("__a soft break__");
    // The rest of the paragraph, bare links and all, is as it was.
    expect(text).toContain("and a bare https://example.com/path link, www.example.org and team@example.com.");
  });

  it("puts a new paragraph between two blocks with a blank line", () => {
    const pos = find(md.doc, (n) => n.type.name === "heading" && n.textContent === "Lists");
    const after = pos + md.doc.nodeAt(pos)!.nodeSize;
    const doc = new Transform(md.doc).insert(after, schema.nodes.paragraph.create(null, schema.text("New words."))).doc;
    const { text } = save(doc);
    expect(text).toBe(tricky.replace("## Lists\n\n", "## Lists\n\nNew words.\n\n"));
  });

  it("removes a block and what was before it", () => {
    const pos = find(md.doc, hasText("Text after the rule."));
    const doc = new Transform(md.doc).delete(pos, pos + md.doc.nodeAt(pos)!.nodeSize).doc;
    const { text } = save(doc);
    expect(text).toBe(tricky.replace("***\n\nText after the rule.\n", "***\n"));
  });

  it("ticks a task's box and nothing else", () => {
    const pos = find(md.doc, (n) => n.type.name === "list_item" && n.attrs.checked === false && n.textContent === "A task");
    const doc = new Transform(md.doc).setNodeMarkup(pos, null, { ...md.doc.nodeAt(pos)!.attrs, checked: true }).doc;
    expect(save(doc).text).toBe(tricky.replace("- [ ] A task\n", "- [x] A task\n"));
  });

  it("adds an item with its list's bullet", () => {
    const pos = find(md.doc, (n) => n.type.name === "bullet_list" && n.textContent.startsWith("A star list"));
    const list = md.doc.nodeAt(pos)!;
    const item = schema.nodes.list_item.create(null, schema.nodes.paragraph.create(null, schema.text("A third")));
    const doc = new Transform(md.doc).insert(pos + list.nodeSize - 1, item).doc;
    expect(save(doc).text).toBe(tricky.replace("    * another\n", "    * another\n* A third\n"));
  });

  it("renumbers a list that counts up when an item goes in at the top", () => {
    const pos = find(md.doc, (n) => n.type.name === "ordered_list" && n.textContent.startsWith("Paren"));
    const item = schema.nodes.list_item.create(null, schema.nodes.paragraph.create(null, schema.text("First now")));
    const doc = new Transform(md.doc).insert(pos + 1, item).doc;
    expect(save(doc).text).toBe(tricky.replace("1) Paren numbers\n2) Count up\n", "1) First now\n2) Paren numbers\n3) Count up\n"));
  });

  it("numbers a new item as its list does", () => {
    const pos = find(md.doc, (n) => n.type.name === "ordered_list" && n.textContent.startsWith("Same number"));
    const list = md.doc.nodeAt(pos)!;
    const item = schema.nodes.list_item.create(null, schema.nodes.paragraph.create(null, schema.text("And this")));
    const doc = new Transform(md.doc).insert(pos + list.nodeSize - 1, item).doc;
    expect(save(doc).text).toBe(tricky.replace("1. Like this\n", "1. Like this\n1. And this\n"));
  });

  it("changes one row of a table", () => {
    const pos = find(md.doc, (n) => n.type.name === "table_cell" && n.textContent === "Nothing.");
    const doc = typed(md.doc, pos, "Says ");
    const { text, how } = save(doc);
    expect(how).toBe("blocks");
    expect(text).toBe(tricky.replace("| Note | To say something. | Nothing. |", "| Note | To say something. | Says Nothing. |"));
  });

  it("changes a paragraph in a quote in a quote", () => {
    const pos = find(md.doc, hasText("nested quote"));
    expect(save(typed(md.doc, pos)).text).toBe(tricky.replace("> > nested quote", "> > EDIT nested quote"));
  });

  it("keeps a code block's fence and info", () => {
    const pos = find(md.doc, (n) => n.type.name === "code_block" && n.attrs.lang === "go");
    expect(save(typed(md.doc, pos, "// x\n")).text).toBe(tricky.replace('```go title="main.go"\nfunc', '```go title="main.go"\n// x\nfunc'));
  });

  it("keeps a file's line endings", () => {
    const crlf = parseMarkdown(tricky.replace(/\n/g, "\r\n"));
    const pos = find(crlf.doc, (n) => n.type.name === "bullet_list" && n.textContent.startsWith("A star list"));
    const list = crlf.doc.nodeAt(pos)!;
    const item = schema.nodes.list_item.create(null, schema.nodes.paragraph.create(null, schema.text("A third")));
    const { text } = saveMarkdown(new Transform(crlf.doc).insert(pos + list.nodeSize - 1, item).doc, crlf);
    expect(text).toBe(tricky.replace("    * another\n", "    * another\n* A third\n").replace(/\n/g, "\r\n"));
  });

  it("writes a new document as agents write them", () => {
    const { paragraph, heading, bullet_list, list_item } = schema.nodes;
    const doc = schema.nodes.doc.create(null, [
      heading.create({ level: 1 }, schema.text("Plan")),
      paragraph.create(null, [schema.text("Do "), schema.text("this", [schema.marks.em.create()]), schema.text(".")]),
      bullet_list.create(null, [list_item.create(null, paragraph.create(null, schema.text("one"))), list_item.create(null, paragraph.create(null, schema.text("two")))]),
    ]);
    expect(saveMarkdown(doc).text).toBe("# Plan\n\nDo *this*.\n\n- one\n- two\n");
  });

  it("starts an empty file", () => {
    const empty = parseMarkdown("");
    const doc = typed(empty.doc, 0, "Hello");
    expect(saveMarkdown(doc, empty).text).toBe("Hello\n");
  });

  it("keeps a paragraph's lines where they broke, a word in it changed", () => {
    const src = "Intro.\n\nThe demo repo is a tiny Go program used for trying ws missions. There is no\nPython program in it, so a mission that asks for one has nothing to build\nagainst and no agreed location or run command.\n\nAfter.\n";
    const m = parseMarkdown(src);
    expect(saveMarkdown(replaced(m, "tiny", "small"), m)).toEqual({ text: src.replace("tiny", "small"), how: "blocks" });
    expect(saveMarkdown(replaced(m, "nothing to build", "nothing at all to build"), m).text).toBe(src.replace("nothing to build", "nothing at all to build"));
  });

  it("keeps lines where they broke in an item, a quote, and around a hard break", () => {
    const item = "- First item that runs\n  over two lines, and on.\n- Second.\n";
    const mi = parseMarkdown(item);
    expect(saveMarkdown(replaced(mi, "runs", "goes"), mi).text).toBe(item.replace("runs", "goes"));
    const quote = "> A quote that runs\n> over two lines.\n";
    const mq = parseMarkdown(quote);
    expect(saveMarkdown(replaced(mq, "over", "across"), mq).text).toBe(quote.replace("over", "across"));
    const hard = "A hard break follows\\\nand this is after it, on a line\nof its own.\n";
    const mh = parseMarkdown(hard);
    expect(saveMarkdown(replaced(mh, "after it", "after the break"), mh).text).toBe(hard.replace("after it", "after the break"));
  });

  it("gives a list the other bullet when it comes up against one like it", () => {
    // Without the heading between them, two lists of "-" read as one.
    const m = parseMarkdown("- one\n- two\n\n## Then\n\n- three\n- four\n\nAfter.\n");
    const pos = find(m.doc, (n) => n.type.name === "heading");
    const doc = new Transform(m.doc).delete(pos, pos + m.doc.nodeAt(pos)!.nodeSize).doc;
    expect(saveMarkdown(doc, m)).toEqual({ text: "- one\n- two\n\n* three\n* four\n\nAfter.\n", how: "blocks" });
  });

  it("changes least when markdown can't say what's left", () => {
    // Without its definition, [the doc][d] is just words.
    const m = parseMarkdown("See [the doc][d].\n\n[d]: https://example.com\n\nAfter.\n");
    const pos = find(m.doc, (n) => n.type.name === "raw_block");
    const doc = new Transform(m.doc).delete(pos, pos + m.doc.nodeAt(pos)!.nodeSize).doc;
    expect(saveMarkdown(doc, m)).toEqual({ text: "See [the doc][d].\n\nAfter.\n", how: "blocks", lossy: true });
  });
});

describe("what the editor has no node for", () => {
  const source = "---\ntitle: '15. Pipeline'\nstatus: proposed\n---\n\n# Pipeline\n\nA score $E_{gh}$ between 0 and 100, for $5 to $10.\n\n$$\nR_{ghs} = \\delta_{hi}\\, \\omega_{si}\n$$\n\nThe end.\n";
  it("keeps front matter and maths as written", () => {
    const m = parseMarkdown(source);
    expect(m.doc.child(0).type.name).toBe("raw_block");
    expect(m.doc.child(0).attrs.text).toBe("---\ntitle: '15. Pipeline'\nstatus: proposed\n---");
    expect(m.doc.child(1).type.name).toBe("heading");
    expect(m.doc.child(2).child(1).type.name).toBe("raw_inline");
    expect(m.doc.child(2).child(1).attrs.text).toBe("$E_{gh}$");
    expect(m.doc.child(3).type.name).toBe("raw_block");
    expect(saveMarkdown(m.doc, m).text).toBe(source);
  });
  it("writes them back as they were when what's beside them is edited", () => {
    const m = parseMarkdown(source);
    expect(saveMarkdown(replaced(m, "between", "from"), m)).toEqual({ text: source.replace("between", "from"), how: "blocks" });
    expect(saveMarkdown(replaced(m, "The end", "The very end"), m)).toEqual({ text: source.replace("The end", "The very end"), how: "blocks" });
  });
});
