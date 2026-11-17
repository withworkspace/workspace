import type { Node as PMNode } from "prosemirror-model";
import { type EditorState, NodeSelection, Plugin, PluginKey, Selection, TextSelection, type Transaction } from "prosemirror-state";
import { Decoration, DecorationSet, type EditorView } from "prosemirror-view";
import { schema } from "./schema";

// Diagrams: a ```mermaid block is drawn as its diagram (views.ts), with a
// toggle in its corner to show its source, to edit, and back. Most are an
// agent's, to be read; editing one is the exception.
//
// Which way a block shows is the editor's, not the document's: a mark on
// the block (a node decoration, which its view is handed), kept on it as
// the text around it changes. Showing one or the other changes nothing in
// the markdown and isn't an edit. A block that's empty shows its source,
// to be typed into.
//
// A diagram drawn hides its text, so the cursor never stays in it (after
// an undo, say): the block is selected whole instead, as an image is, and
// nothing's typed where it can't be seen. The arrow keys select it whole
// too, where they'd go into it.

const { nodes: N } = schema;

const key = new PluginKey<DecorationSet>("diagrams");

export const isDiagram = (node: PMNode) => node.type === N.code_block && node.attrs.lang === "mermaid";

// drawsDiagrams says whether an editor draws its diagrams (has this
// plugin); one that doesn't (a document's tracked changes) shows them as
// code.
export const drawsDiagrams = (state: EditorState) => !!key.getState(state);

// showsSource says whether a block's view is to show its source, from the
// decorations it's handed.
export const showsSource = (decos: readonly Decoration[]) => decos.some((d) => d.spec.source);

// sourceAt says whether the diagram at pos shows its source.
export function sourceAt(state: EditorState, pos: number): boolean {
  return marked(key.getState(state) ?? DecorationSet.empty, pos).length > 0;
}

const marked = (set: DecorationSet, pos: number) => set.find(pos, pos, (spec) => spec.source).filter((d) => d.from === pos);
const mark = (pos: number, node: PMNode) => Decoration.node(pos, pos + node.nodeSize, {}, { source: true });

// showSource shows the diagram at pos as its source (with the cursor at
// the end of it, to write on), or drawn.
export function showSource(state: EditorState, pos: number, source: boolean): Transaction {
  const tr = state.tr.setMeta(key, { pos, source });
  const node = state.doc.nodeAt(pos);
  if (source && node) tr.setSelection(TextSelection.create(tr.doc, pos + node.nodeSize - 1));
  return tr;
}

// withEmpties marks the empty diagrams that aren't yet to show their
// source: a new block's, or one whose text has all gone.
function withEmpties(doc: PMNode, set: DecorationSet): DecorationSet {
  const add: Decoration[] = [];
  doc.descendants((node, pos) => {
    if (isDiagram(node) && !node.content.size && !marked(set, pos).length) add.push(mark(pos, node));
    return !node.isTextblock;
  });
  return add.length ? set.add(doc, add) : set;
}

// cursorOut selects whole the drawn diagram the cursor's in, if it's in
// one.
function cursorOut(state: EditorState): Transaction | null {
  const sel = state.selection;
  if (!(sel instanceof TextSelection) || !sel.$anchor.sameParent(sel.$head) || !isDiagram(sel.$head.parent)) return null;
  const pos = sel.$head.before();
  return sourceAt(state, pos) ? null : state.tr.setSelection(NodeSelection.create(state.doc, pos));
}

const arrows: Record<string, { dir: 1 | -1; edge: "up" | "down" | "left" | "right" }> = {
  ArrowUp: { dir: -1, edge: "up" },
  ArrowDown: { dir: 1, edge: "down" },
  ArrowLeft: { dir: -1, edge: "left" },
  ArrowRight: { dir: 1, edge: "right" },
};

// arrowOnto is an arrow key that leaves the cursor's block for a diagram
// that's drawn: it's selected whole. (The browser would go past it, its
// text being hidden.)
function arrowOnto(view: EditorView, e: KeyboardEvent): boolean {
  const arrow = arrows[e.key];
  const sel = view.state.selection;
  if (!arrow || e.shiftKey || e.altKey || e.metaKey || e.ctrlKey || !(sel instanceof TextSelection) || !sel.empty) return false;
  if (!sel.$head.parent.isTextblock || !view.endOfTextblock(arrow.edge)) return false;
  const $out = view.state.doc.resolve(arrow.dir < 0 ? sel.$head.before() : sel.$head.after());
  const next = Selection.findFrom($out, arrow.dir);
  if (!(next instanceof TextSelection) || !isDiagram(next.$head.parent)) return false;
  const pos = next.$head.before();
  if (sourceAt(view.state, pos)) return false;
  view.dispatch(view.state.tr.setSelection(NodeSelection.create(view.state.doc, pos)).scrollIntoView());
  return true;
}

export const diagrams = new Plugin<DecorationSet>({
  key,
  state: {
    init: (_, state) => withEmpties(state.doc, DecorationSet.empty),
    apply(tr, set) {
      set = set.map(tr.mapping, tr.doc);
      const to = tr.getMeta(key) as { pos: number; source: boolean } | undefined;
      const node = to && tr.doc.nodeAt(to.pos);
      if (to && node && isDiagram(node)) {
        set = set.remove(marked(set, to.pos));
        if (to.source) set = set.add(tr.doc, [mark(to.pos, node)]);
      }
      return tr.docChanged ? withEmpties(tr.doc, set) : set;
    },
  },
  props: {
    decorations: (state) => key.getState(state),
    handleKeyDown: (view, e) => arrowOnto(view, e),
  },
  appendTransaction: (trs, _old, state) => (trs.some((tr) => tr.selectionSet || tr.docChanged || tr.getMeta(key)) ? cursorOut(state) : null),
  view(view: EditorView) {
    // A document that starts with a diagram starts with the cursor in it.
    // (Not while the editor's being made: what it dispatches to may not
    // be there yet.)
    queueMicrotask(() => {
      const tr = view.isDestroyed ? null : cursorOut(view.state);
      if (tr) view.dispatch(tr);
    });
    return {};
  },
});
