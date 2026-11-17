import type { Node as PMNode } from "prosemirror-model";
import { EditorState, NodeSelection, TextSelection } from "prosemirror-state";
import { describe, expect, it } from "vitest";
import { diagrams, drawsDiagrams, isDiagram, showSource, sourceAt } from "./diagrams";
import { parseMarkdown, saveMarkdown } from "./markdown";
import { schema } from "./schema";

const { nodes: N } = schema;

const md = "# Flow\n\n```mermaid\ngraph TD\n  A --> B\n```\n\nAfter.\n\n```mermaid\n```\n\n```ts\nconst a = 1;\n```\n";

function open(text = md) {
  const parsed = parseMarkdown(text);
  return { parsed, state: EditorState.create({ doc: parsed.doc, plugins: [diagrams] }) };
}

// blocks is where each code block is, in order.
function blocks(doc: PMNode): number[] {
  const out: number[] = [];
  doc.descendants((node, pos) => {
    if (node.type === N.code_block) out.push(pos);
    return !node.isTextblock;
  });
  return out;
}

describe("diagrams", () => {
  it("draws a diagram with text, and shows an empty one's source", () => {
    const { state } = open();
    const [drawn, empty, ts] = blocks(state.doc);
    expect(drawsDiagrams(state)).toBe(true);
    expect(isDiagram(state.doc.nodeAt(drawn)!)).toBe(true);
    expect(isDiagram(state.doc.nodeAt(ts)!)).toBe(false);
    expect(sourceAt(state, drawn)).toBe(false);
    expect(sourceAt(state, empty)).toBe(true);
    expect(sourceAt(state, ts)).toBe(false);
  });

  it("an editor without it shows diagrams as code", () => {
    expect(drawsDiagrams(EditorState.create({ doc: open().parsed.doc }))).toBe(false);
  });

  it("shows the source and back without changing the document", () => {
    const { parsed, state } = open();
    const [at] = blocks(state.doc);
    const tr = showSource(state, at, true);
    expect(tr.docChanged).toBe(false);
    const source = state.apply(tr);
    expect(sourceAt(source, at)).toBe(true);
    // The cursor at the end of its text, to write on.
    expect(source.selection.head).toBe(at + state.doc.nodeAt(at)!.nodeSize - 1);

    const back = source.apply(showSource(source, at, false));
    expect(sourceAt(back, at)).toBe(false);
    expect(back.doc.eq(parsed.doc)).toBe(true);
    expect(saveMarkdown(back.doc, parsed).text).toBe(md);
  });

  it("keeps a block's source shown as it's edited, and as what's before it changes", () => {
    const { parsed, state } = open();
    const [at] = blocks(state.doc);
    let s = state.apply(showSource(state, at, true));
    s = s.apply(s.tr.insertText("\n  B --> C"));
    expect(sourceAt(s, at)).toBe(true);
    s = s.apply(s.tr.insert(0, N.paragraph.create(null, schema.text("Before."))));
    const [moved] = blocks(s.doc);
    expect(moved).toBeGreaterThan(at);
    expect(sourceAt(s, moved)).toBe(true);
    expect(saveMarkdown(s.doc, parsed).text).toContain("```mermaid\ngraph TD\n  A --> B\n  B --> C\n```");
  });

  it("selects a drawn diagram whole when the cursor lands in it", () => {
    const { state } = open();
    const [at] = blocks(state.doc);
    const s = state.apply(state.tr.setSelection(TextSelection.create(state.doc, at + 3)));
    expect(s.selection).toBeInstanceOf(NodeSelection);
    expect(s.selection.from).toBe(at);
  });

  it("lets the cursor into a diagram showing its source", () => {
    const { state } = open();
    const [, empty] = blocks(state.doc);
    const s = state.apply(state.tr.setSelection(TextSelection.create(state.doc, empty + 1)));
    expect(s.selection).toBeInstanceOf(TextSelection);
    expect(s.selection.head).toBe(empty + 1);
  });

  it("shows the source of a diagram made empty, and of a new one", () => {
    const { state } = open();
    const [at] = blocks(state.doc);
    const node = state.doc.nodeAt(at)!;
    let s = state.apply(state.tr.delete(at + 1, at + node.nodeSize - 1));
    expect(sourceAt(s, at)).toBe(true);
    s = s.apply(s.tr.insert(0, N.code_block.create({ lang: "mermaid" })));
    expect(sourceAt(s, 0)).toBe(true);
  });

  it("a toggle on a block that isn't a diagram does nothing", () => {
    const { state } = open();
    const [, , ts] = blocks(state.doc);
    const s = state.apply(state.tr.setMeta(diagrams.spec.key!, { pos: ts, source: true }));
    expect(sourceAt(s, ts)).toBe(false);
  });
});
