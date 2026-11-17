import type { Node as PMNode } from "prosemirror-model";
import { Plugin, PluginKey } from "prosemirror-state";
import { Decoration, DecorationSet, type EditorView } from "prosemirror-view";
import { cellContent } from "./cells";
import { schema } from "./schema";
import { quoteRanges } from "./views";

// Notes: what a pass over a page found, drawn on the words it's about.
//
// A pass isn't part of the page: its findings are kept beside it, and
// shown on it only if you choose. So a note is drawn over the document (a
// decoration), never put in it, and is found again by its words each time
// they move. One whose words have gone isn't drawn.

// Note is a finding to draw: its key, the words it's on, and the pass it's
// from (for its colour).
export interface Note {
  key: string;
  quote: string;
  pass: string;
}

interface NotesState {
  notes: Note[];
  decos: DecorationSet;
}

const notesKey = new PluginKey<NotesState>("notes");

// whereIs is where a note's words are: as quoted, or, quoted from the
// markdown, without its marks.
function whereIs(doc: PMNode, quote: string): { from: number; to: number }[] {
  const plain = quote.replace(/[`*]/g, "");
  const tries = [quote, plain, plain.replace(/[.…,;:]+$/, "")];
  for (const exact of tries) {
    const [ranges] = quoteRanges(doc, [{ exact }]);
    if (ranges.length) return ranges;
  }
  return [];
}

function build(doc: PMNode, notes: Note[]): NotesState {
  const decos: Decoration[] = [];
  for (const n of notes) {
    for (const r of whereIs(doc, n.quote)) decos.push(Decoration.inline(r.from, r.to, { class: `note-mark pass-${n.pass}`, "data-note": n.key }));
  }
  return { notes, decos: DecorationSet.create(doc, decos) };
}

// notesPlugin draws notes, and says when one's clicked.
export function notesPlugin(clicked: (key: string) => void): Plugin {
  return new Plugin<NotesState>({
    key: notesKey,
    state: {
      init: (_, state) => build(state.doc, []),
      apply(tr, was, _old, state) {
        const notes = tr.getMeta(notesKey) as Note[] | undefined;
        if (notes) return build(state.doc, notes);
        // Found again when the document changes: words move, come and go.
        return tr.docChanged ? build(state.doc, was.notes) : was;
      },
    },
    props: {
      decorations: (state) => notesKey.getState(state)?.decos,
      handleClick(_view, _pos, event) {
        const mark = event.target instanceof Element ? event.target.closest("[data-note]") : null;
        const key = mark?.getAttribute("data-note");
        if (key) clicked(key);
        return false;
      },
    },
  });
}

// setNotes says which notes to draw.
export function setNotes(view: EditorView, notes: Note[]) {
  view.dispatch(view.state.tr.setMeta(notesKey, notes).setMeta("addToHistory", false));
}

// noteElement is a note's words as they're drawn (the first of them).
export function noteElement(view: EditorView, key: string): HTMLElement | null {
  return view.dom.querySelector<HTMLElement>(`[data-note="${CSS.escape(key)}"]`);
}

// onPage is whether words are in the document.
export const onPage = (doc: PMNode, quote: string) => whereIs(doc, quote).length > 0;

// blockOf is the block at the top of the document that words are in: its
// start and end. None if they're not there.
export function blockOf(doc: PMNode, quote: string): { from: number; to: number } | undefined {
  const [first] = whereIs(doc, quote);
  if (!first) return undefined;
  const $pos = doc.resolve(first.from);
  return $pos.depth < 1 ? undefined : { from: $pos.before(1), to: $pos.after(1) };
}

// putInstead writes words in place of those quoted, where they're in one
// run of a block: a pass's suggestion, taken. It says whether it could.
export function putInstead(view: EditorView, quote: string, markdown: string): boolean {
  const ranges = whereIs(view.state.doc, quote);
  if (ranges.length !== 1) return false;
  // What's suggested is words for a place in a line: its blocks' words, one
  // after another.
  const inline: PMNode[] = [];
  cellContent(markdown).forEach((block, _o, i) => {
    if (i > 0 && inline.length) inline.push(schema.text(" "));
    if (block.isTextblock) block.forEach((n) => inline.push(n));
    else if (block.textContent) inline.push(schema.text(block.textContent));
  });
  if (!inline.length) return false;
  view.dispatch(view.state.tr.replaceWith(ranges[0].from, ranges[0].to, inline).scrollIntoView());
  return true;
}
