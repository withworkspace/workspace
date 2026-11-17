import { Fragment, type Node as PMNode } from "prosemirror-model";
import { Transform } from "prosemirror-transform";
import { describe, expect, it } from "vitest";
import { EditorState, TextSelection } from "prosemirror-state";
import type { EditorView } from "prosemirror-view";
import { askAbout, asking, blocksAt, cellContent, placeBlocks, plainCell } from "./cells";
import { parseMarkdown, saveMarkdown } from "./markdown";
import { schema } from "./schema";

// A cell is what an agent wrote in a page, between two comments in its
// file: the bridge reads it as one block, and writes back only what
// changed in or around it.

const N = schema.nodes;

const page = `# Token cache

The routes we have:

<!-- ws:cell w7 v2 | a table of the existing routes -->

| Route | Handler |
|---|---|
| /tokens | tokens.go |

Two of them **cache**.

<!-- /ws:cell -->

After it.
`;

const cellOf = (doc: PMNode) => {
  let at = -1;
  doc.forEach((n, pos) => {
    if (n.type === N.cell && at < 0) at = pos;
  });
  return { pos: at, node: doc.nodeAt(at)! };
};

describe("cells in markdown", () => {
  it("reads a cell as one block, with what it holds", () => {
    const { doc } = parseMarkdown(page);
    expect(doc.childCount).toBe(4);
    const { node } = cellOf(doc);
    expect(node.attrs).toMatchObject({ id: "w7", v: 2, prompt: "a table of the existing routes" });
    expect(node.childCount).toBe(2);
    expect(node.child(0).type).toBe(N.table);
    expect(node.child(1).textContent).toBe("Two of them cache.");
  });

  it("saves unchanged as it was, and written anew reads the same", () => {
    const md = parseMarkdown(page);
    expect(saveMarkdown(md.doc, md)).toEqual({ text: page, how: "same" });
    const anew = saveMarkdown(md.doc).text;
    expect(parseMarkdown(anew).doc.eq(md.doc)).toBe(true);
  });

  it("an edit inside a cell changes only its block", () => {
    const md = parseMarkdown(page);
    const { pos, node } = cellOf(md.doc);
    // Into the cell's paragraph: past the cell's start and its table.
    const para = pos + 1 + node.child(0).nodeSize;
    const doc = new Transform(md.doc).insert(para + 1, schema.text("All ")).doc;
    const out = saveMarkdown(doc, md);
    expect(out.how).toBe("blocks");
    expect(out.text).toBe(page.replace("Two of them", "All Two of them"));
  });

  it("an edit outside a cell leaves it byte for byte", () => {
    const md = parseMarkdown(page);
    const doc = new Transform(md.doc).insert(md.doc.child(0).nodeSize + 1, schema.text("Now: ")).doc;
    expect(saveMarkdown(doc, md).text).toBe(page.replace("The routes we have", "Now: The routes we have"));
  });

  it("a cell filled again is written between new comments", () => {
    const md = parseMarkdown(page);
    const { pos, node } = cellOf(md.doc);
    const filled = N.cell.create({ ...node.attrs, v: 4 }, Fragment.from(N.paragraph.create(null, schema.text("Just /tokens."))));
    const doc = new Transform(md.doc).replaceWith(pos, pos + node.nodeSize, filled).doc;
    const out = saveMarkdown(doc, md);
    expect(out.lossy).toBeUndefined();
    expect(out.text).toBe(`# Token cache

The routes we have:

<!-- ws:cell w7 v4 | a table of the existing routes -->

Just /tokens.

<!-- /ws:cell -->

After it.
`);
  });

  it("a new, empty cell is its two comments", () => {
    const md = parseMarkdown("Before.\n");
    const cell = N.cell.create({ id: "w9", v: 0, prompt: "which of\nthese --> need auth?" });
    const doc = new Transform(md.doc).insert(md.doc.content.size, cell).doc;
    const out = saveMarkdown(doc, md);
    expect(out.text).toBe("Before.\n\n<!-- ws:cell w9 v0 | which of these → need auth? -->\n\n<!-- /ws:cell -->\n");
    const back = cellOf(parseMarkdown(out.text).doc).node;
    expect(back.attrs).toMatchObject({ id: "w9", v: 0, prompt: "which of these → need auth?" });
    expect(back.childCount).toBe(0);
  });

  it("a question is a cell that holds nothing, and says so in its comment", () => {
    const src = "Before.\n\n<!-- ws:cell w9 v0 ask | why per tenant? -->\n\n<!-- /ws:cell -->\n\nAfter.\n";
    const md = parseMarkdown(src);
    const { pos, node } = cellOf(md.doc);
    expect(node.attrs).toMatchObject({ id: "w9", v: 0, ask: true, prompt: "why per tenant?" });
    expect(node.childCount).toBe(0);
    expect(saveMarkdown(md.doc, md)).toEqual({ text: src, how: "same" });
    // Asked for in a new page.
    const fresh = parseMarkdown("Before.\n");
    const asked = new Transform(fresh.doc).insert(fresh.doc.content.size, N.cell.create({ id: "w9", ask: true, prompt: "why per tenant?" })).doc;
    expect(saveMarkdown(asked, fresh).text).toBe("Before.\n\n<!-- ws:cell w9 v0 ask | why per tenant? -->\n\n<!-- /ws:cell -->\n");
    // Written into, it's a cell like any other.
    const written = N.cell.create({ ...node.attrs, v: 1, ask: false }, Fragment.from(N.paragraph.create(null, schema.text("Tokens are per tenant."))));
    const out = saveMarkdown(new Transform(md.doc).replaceWith(pos, pos + node.nodeSize, written).doc, md).text;
    expect(out).toBe("Before.\n\n<!-- ws:cell w9 v1 | why per tenant? -->\n\nTokens are per tenant.\n\n<!-- /ws:cell -->\n\nAfter.\n");
  });

  it("an outline's blocks go in where it says, empty, under their headings", () => {
    const src = "# Token cache\n\nIntro.\n\n## Constraints\n\nNone yet.\n\n### Detail\n\nMore.\n\n## Release\n\nSoon.\n\n<!-- ws:cell w12 v0 | add sections -->\n\n<!-- /ws:cell -->\n\nAfter.\n";
    const md = parseMarkdown(src);
    // An editor, as far as placing goes: a document, and what's done to it.
    let state = EditorState.create({ doc: md.doc });
    const view = { get state() { return state; }, dispatch: (tr: Parameters<EditorView["dispatch"]>[0]) => (state = state.apply(tr)) } as unknown as EditorView;
    placeBlocks(view, "w12", [
      { key: "b1", after: "Constraints", heading: "Dependencies", summary: "What it needs." },
      { key: "b2", after: "constraints:", heading: "Limits", summary: "What it mustn't do." },
      { key: "b3", after: "here", summary: "A closing note." },
      { key: "b4", after: "Nowhere", heading: "Open", summary: "What's open." },
      { key: "b5", after: "start", summary: "A summary." },
    ]);
    const out = saveMarkdown(state.doc, md).text;
    expect(out).toBe(
      [
        "# Token cache",
        "<!-- ws:cell w12-b5 v0 | A summary. -->\n\n<!-- /ws:cell -->",
        "Intro.",
        "## Constraints",
        "None yet.",
        "### Detail",
        "More.",
        // At the end of the section, its own sections included, and at its level.
        "## Dependencies",
        "<!-- ws:cell w12-b1 v0 | What it needs. -->\n\n<!-- /ws:cell -->",
        "## Limits",
        "<!-- ws:cell w12-b2 v0 | What it mustn't do. -->\n\n<!-- /ws:cell -->",
        "## Release",
        "Soon.",
        // Here: in place of the cell that planned it.
        "<!-- ws:cell w12-b3 v0 | A closing note. -->\n\n<!-- /ws:cell -->",
        "After.",
        // A place that isn't there: the end.
        "## Open",
        "<!-- ws:cell w12-b4 v0 | What's open. -->\n\n<!-- /ws:cell -->",
      ].join("\n\n") + "\n",
    );
    // And it reads back as cells named for their blocks.
    const ids: string[] = [];
    parseMarkdown(out).doc.forEach((n) => n.type === N.cell && ids.push(n.attrs.id));
    expect(ids).toEqual(["w12-b5", "w12-b1", "w12-b2", "w12-b3", "w12-b4"]);
  });

  it("a cell that isn't closed stays as it was written", () => {
    const src = "<!-- ws:cell w1 v1 | routes -->\n\nWords.\n";
    const md = parseMarkdown(src);
    expect(md.doc.child(0).type).toBe(N.raw_block);
    expect(saveMarkdown(md.doc, md).text).toBe(src);
    const two = "<!-- ws:cell w1 v1 -->\n\n<!-- ws:cell w2 v1 -->\n\nWords.\n\n<!-- /ws:cell -->\n";
    const doc = parseMarkdown(two).doc;
    expect(doc.child(0).type).toBe(N.raw_block);
    expect(doc.child(1).attrs.id).toBe("w2");
  });

  it("what an agent wrote reads the same however it wrote it, until it's edited", () => {
    const wrote = "| Route | Handler |\n| :-- | --- |\n| /tokens | tokens.go |\n\n\nTwo of them __cache__.\n";
    const plain = plainCell(wrote);
    expect(plainCell(plain)).toBe(plain);
    expect(plainCell(wrote.replace("| --- |", "|---|"))).toBe(plain);
    expect(plainCell(wrote.replace("Two", "All"))).not.toBe(plain);
    // A cell in what it wrote is only its blocks.
    const nested = cellContent("<!-- ws:cell w1 v1 | x -->\n\nWords.\n\n<!-- /ws:cell -->\n");
    expect(nested.childCount).toBe(1);
    expect(nested.child(0).type).toBe(N.paragraph);
  });

  it("a line that starts with >> asks for what follows", () => {
    const p = (text: string) => N.paragraph.create(null, text ? schema.text(text) : null);
    expect(asking(p(">> a table of the routes"))).toEqual({ prompt: "a table of the routes", ask: false });
    expect(asking(p(">>   which need auth?  "))).toEqual({ prompt: "which need auth?", ask: false });
    // >? is a question: answered beside the page.
    expect(asking(p(">? why per tenant"))).toEqual({ prompt: "why per tenant", ask: true });
    expect(asking(p(">?"))).toBeUndefined();
    expect(asking(p(">> "))).toBeUndefined();
    expect(asking(p(">>no space"))).toBeUndefined();
    expect(asking(p("> a quote"))).toBeUndefined();
    expect(asking(N.heading.create({ level: 2 }, schema.text(">> not a line of its own")))).toBeUndefined();
  });

  it("the words to change are the blocks the selection takes in, outside any cell", () => {
    const { doc } = parseMarkdown(page + "\nAnd more.\n\n- an item\n- another\n");
    const at = (from: number, to = from) => blocksAt(EditorState.create({ doc, selection: TextSelection.create(doc, from, to) }));
    const starts: number[] = [];
    doc.forEach((_, pos) => starts.push(pos));
    const [h, intro, cell, after, more, list] = starts;
    // The cursor in a paragraph: that paragraph.
    expect(at(intro + 3)).toEqual({ from: intro, to: cell });
    // From one paragraph into the next: both.
    expect(at(after + 2, more + 3)).toEqual({ from: after, to: list });
    // In a list: the whole list, which is the block at the top.
    expect(at(list + 4)).toEqual({ from: list, to: doc.content.size });
    // In a cell, or taking one in: none. The cell's asked, instead.
    expect(at(cell + 4)).toBeUndefined();
    expect(at(intro + 3, after + 2)).toBeUndefined();
    expect(at(h + 1)).toEqual({ from: h, to: intro });
    // No words: nothing to change.
    const blank = parseMarkdown("Words.\n").doc;
    const empty = new Transform(blank).insert(blank.content.size, N.paragraph.create()).doc;
    expect(blocksAt(EditorState.create({ doc: empty, selection: TextSelection.create(empty, empty.content.size - 1) }))).toBeUndefined();
  });

  it("a cell taken out of its wrapper is its blocks", () => {
    const md = parseMarkdown(page);
    const { pos, node } = cellOf(md.doc);
    const doc = new Transform(md.doc).replaceWith(pos, pos + node.nodeSize, node.content).doc;
    const out = saveMarkdown(doc, md);
    expect(out.text).not.toContain("ws:cell");
    expect(out.text).toContain("| /tokens | tokens.go |\n\nTwo of them **cache**.\n\nAfter it.");
  });
});

describe("askAbout", () => {
  it("puts a question about words selected after their blocks, quoting them, and asks it as a question", () => {
    const md = parseMarkdown("# Plan\n\nKey it per user, and ship on Friday.\n\nAfter.\n");
    let state = EditorState.create({ doc: md.doc });
    const view = { get state() { return state; }, dispatch: (tr: Parameters<EditorView["dispatch"]>[0]) => (state = state.apply(tr)) } as unknown as EditorView;
    let from = 0;
    state.doc.descendants((n, pos) => {
      if (n.isText && n.text?.startsWith("Key it")) from = pos;
    });
    state = state.apply(state.tr.setSelection(TextSelection.create(state.doc, from, from + "Key it per user".length)));
    const range = blocksAt(state)!;
    const asked: unknown[][] = [];

    askAbout(view, range, "Key it per user", "why per user?", { asked: (...a: unknown[]) => asked.push(a), open: () => {} });

    expect(asked).toHaveLength(1);
    expect(asked[0][1]).toBe("About “Key it per user”: why per user?");
    expect(asked[0][4]).toBe(true);
    let cell: PMNode | undefined;
    let index = -1;
    state.doc.forEach((n, _pos, i) => {
      if (n.type === N.cell) [cell, index] = [n, i];
    });
    expect(cell?.attrs).toMatchObject({ prompt: "About “Key it per user”: why per user?", ask: true });
    expect(index).toBe(2); // after the paragraph it's about, before "After."
  });
});

