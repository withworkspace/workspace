import { Fragment, type Node as PMNode } from "prosemirror-model";
import { type Command, type EditorState, Plugin, PluginKey, TextSelection } from "prosemirror-state";
import { Decoration, DecorationSet, type EditorView, type NodeView } from "prosemirror-view";
import { parseMarkdown, saveMarkdown } from "./markdown";
import { schema } from "./schema";

// Cells: places in a page where you asked for something and wrote on.
//
// A line that starts with >> is what you're asking for; ⏎ makes it a cell,
// with the cursor on the line after. A line that starts with >? is a
// question: it's answered beside the page, and holds nothing until you ask
// for that. An agent fills the cell with what it
// writes (a table, a list, an answer), and writes it again when you ask
// for a change. What's in a cell is ordinary document: you can edit it,
// and in the file it's markdown between two comments (mdast.ts).
//
// The editor only draws cells and says when one's asked for or opened: the
// page it's in talks to the server (pages/Page.tsx).

const { nodes: N } = schema;

// CellState is where a cell stands, for drawing it: starting (asked for,
// no agent yet), filling, filled, stale (what it was written from has
// changed since), or failed; note is what its agent's doing now, what's
// changed, or what went wrong.
export interface CellState {
  status: "starting" | "filling" | "filled" | "stale" | "failed";
  note?: string;
}

// CellHost is the page a document's cells are in.
export interface CellHost {
  // asked: a cell was just asked for. key names it until the server's
  // numbered it (nameCell). goal says what it's for, when the page asked
  // it of itself (a check, a cold read); holds is what it's made around,
  // when that's words of yours to be changed (wrapCell).
  // ask: it's a question (>?), to be answered beside the page.
  asked(key: string, prompt: string, goal?: string, holds?: string, ask?: boolean): void;
  // open: a cell's head was clicked (or ⌘⏎ pressed in it): its id, or its
  // key while it has none.
  open(id: string, key: string): void;
}

const cellsKey = new PluginKey<Map<string, CellState>>("cells");

// asking is what a line asks for, if it's a line of its own at the top of
// the document that starts with >> (something to be written there) or >?
// (a question, to be answered beside the page).
export function asking(node: PMNode): { prompt: string; ask: boolean } | undefined {
  if (node.type !== N.paragraph) return undefined;
  const m = /^>([>?])[ \u00a0]+(\S[^]*)$/.exec(node.textContent);
  const prompt = m?.[2].trim();
  return m && prompt ? { prompt, ask: m[1] === "?" } : undefined;
}

// askCell is ⏎ on a line that starts with >>: the line becomes a cell, and
// the cursor goes on to a line after it.
function askCell(host: () => CellHost | undefined): Command {
  return (state, dispatch) => {
    const { $from, empty } = state.selection;
    if (!empty || $from.depth !== 1) return false;
    const asked = asking($from.parent);
    if (!asked) return false;
    if (dispatch) {
      const { prompt, ask } = asked;
      const key = Math.random().toString(36).slice(2, 10);
      const pos = $from.before(1);
      const cell = N.cell.create({ key, prompt, ask });
      const tr = state.tr.replaceWith(pos, pos + $from.parent.nodeSize, [cell, N.paragraph.create()]);
      tr.setSelection(TextSelection.create(tr.doc, pos + cell.nodeSize + 1));
      dispatch(tr.scrollIntoView());
      host()?.asked(key, prompt, undefined, undefined, ask);
    }
    return true;
  };
}

// openCell is ⌘⏎ with the cursor in a cell: its page opens it, to ask for
// a change.
function openCell(host: () => CellHost | undefined): Command {
  return (state, dispatch) => {
    const { $from } = state.selection;
    if ($from.depth < 1 || $from.node(1).type !== N.cell) return false;
    if (dispatch) host()?.open($from.node(1).attrs.id, $from.node(1).attrs.key);
    return true;
  };
}

// cellPlugins make >> lines into cells, and draw each cell as its page
// says it stands. They go before the editor's other keys: ⏎ on a >> line
// is theirs.
export function cellPlugins(host: () => CellHost | undefined): Plugin[] {
  const ask = askCell(host);
  const open = openCell(host);
  return [
    new Plugin<Map<string, CellState>>({
      key: cellsKey,
      state: {
        init: () => new Map(),
        apply: (tr, was) => tr.getMeta(cellsKey) ?? was,
      },
      props: {
        handleKeyDown(view, e) {
          if (e.key !== "Enter" || e.shiftKey || e.altKey || e.isComposing) return false;
          return (e.metaKey || e.ctrlKey ? open : ask)(view.state, view.dispatch, view);
        },
        decorations(state) {
          const states = cellsKey.getState(state);
          const decos: Decoration[] = [];
          state.doc.forEach((node, pos) => {
            if (node.type === N.cell) {
              const name = node.attrs.id || node.attrs.key;
              const st = states?.get(name);
              decos.push(Decoration.node(pos, pos + node.nodeSize, {}, { cell: st ?? null }));
            } else if (asking(node)) {
              decos.push(Decoration.node(pos, pos + node.nodeSize, { class: `cell-asking${asking(node)?.ask ? " question" : ""}` }));
            }
          });
          return DecorationSet.create(state.doc, decos);
        },
      },
    }),
  ];
}

// CellView draws a cell: a head saying what was asked for and where it
// stands (clicked, the page opens it), then what it holds, to edit as any
// of the document.
export class CellView implements NodeView {
  dom: HTMLElement;
  contentDOM: HTMLElement;
  private head: HTMLElement;
  private prompt: HTMLElement;
  private status: HTMLElement;
  private glyph: HTMLElement;

  constructor(
    private node: PMNode,
    private host: () => CellHost | undefined,
    decorations: readonly Decoration[],
  ) {
    this.dom = document.createElement("div");
    this.head = document.createElement("div");
    this.head.className = "cell-head";
    this.head.contentEditable = "false";
    this.head.setAttribute("role", "button");
    this.head.title = "Ask for a change, or see what it read";
    const glyph = document.createElement("span");
    glyph.className = "cell-glyph";
    glyph.setAttribute("aria-hidden", "true");
    this.glyph = glyph;
    this.prompt = document.createElement("span");
    this.prompt.className = "cell-prompt";
    this.status = document.createElement("span");
    this.status.className = "cell-status";
    this.head.append(glyph, this.prompt, this.status);
    // The head takes the press (the editor would put the cursor there).
    this.head.addEventListener("mousedown", (e) => e.preventDefault());
    this.head.addEventListener("click", () => this.host()?.open(this.node.attrs.id, this.node.attrs.key));
    this.contentDOM = document.createElement("div");
    this.contentDOM.className = "cell-body";
    this.dom.append(this.head, this.contentDOM);
    this.draw(decorations);
  }

  private draw(decorations: readonly Decoration[]) {
    const st = decorations.map((d) => d.spec.cell as CellState | null | undefined).find((s) => s !== undefined);
    const status = st?.status ?? (this.node.attrs.id ? "filled" : "starting");
    // A question that holds nothing is only its head.
    const ask = this.node.attrs.ask && !this.node.childCount;
    this.dom.className = `cell s-${status}${this.node.childCount ? "" : " blank"}${ask ? " ask" : ""}${st?.note && status === "filled" ? " noted" : ""}`;
    this.glyph.textContent = ask ? "?" : "»";
    this.head.title = ask ? "Read the answer, ask more, or have it put in the page" : "Ask for a change, or see what it read";
    this.dom.dataset.cell = this.node.attrs.id || this.node.attrs.key;
    this.prompt.textContent = this.node.attrs.prompt;
    const note = st?.note ?? (status === "starting" ? "Starting…" : "");
    this.status.textContent = note;
    this.status.hidden = !note;
  }

  update(node: PMNode, decorations: readonly Decoration[]): boolean {
    if (node.type !== this.node.type) return false;
    this.node = node;
    this.draw(decorations);
    return true;
  }

  stopEvent(e: Event): boolean {
    return e.target instanceof Node && this.head.contains(e.target);
  }

  // What's drawn around the cell's content is ours, not the document's.
  ignoreMutation(m: MutationRecord | { type: "selection"; target: Node }): boolean {
    return m.type !== "selection" && !this.contentDOM.contains(m.target);
  }
}

export const cellView =
  (host: () => CellHost | undefined) =>
  (node: PMNode, _view: EditorView, _getPos: () => number | undefined, decorations: readonly Decoration[]) =>
    new CellView(node, host, decorations);

// --- What the page does to cells

// Quiet changes aren't yours to undo: an id arriving, a cell filled.
const quiet = (view: EditorView) => view.state.tr.setMeta("addToHistory", false);

// cellsIn is a document's cells, in order.
export function cellsIn(doc: PMNode): { pos: number; node: PMNode }[] {
  const out: { pos: number; node: PMNode }[] = [];
  doc.forEach((node, pos) => {
    if (node.type === N.cell) out.push({ pos, node });
  });
  return out;
}

const find = (doc: PMNode, test: (n: PMNode) => boolean) => cellsIn(doc).find((c) => test(c.node));
const named = (name: string) => (n: PMNode) => !!name && (n.attrs.id === name || n.attrs.key === name);

// setCells says where each cell stands (by id, or key), to draw them.
export function setCells(view: EditorView, states: Map<string, CellState>) {
  view.dispatch(quiet(view).setMeta(cellsKey, states));
}

// nameCell gives the cell asked for as key its id, once the server's
// numbered it.
export function nameCell(view: EditorView, key: string, id: string): boolean {
  const c = find(view.state.doc, (n) => n.attrs.key === key);
  if (!c) return false;
  view.dispatch(quiet(view).setNodeMarkup(c.pos, null, { ...c.node.attrs, id, key: "" }));
  return true;
}

// caughtUp has the editor take the selection as the browser has it now. It
// follows the browser's a moment behind, and a key pressed straight after
// a click would otherwise act where the cursor was.
export function caughtUp(view: EditorView) {
  const sel = document.getSelection();
  if (!sel?.anchorNode || !sel.focusNode || !view.dom.contains(sel.anchorNode) || !view.dom.contains(sel.focusNode)) return;
  try {
    const a = view.posAtDOM(sel.anchorNode, sel.anchorOffset);
    const f = view.posAtDOM(sel.focusNode, sel.focusOffset);
    const now = TextSelection.create(view.state.doc, a, f);
    if (!now.eq(view.state.selection)) view.dispatch(view.state.tr.setSelection(now));
  } catch {
    // Somewhere that isn't a place in the document: as it was, then.
  }
}

// blocksAt is the blocks at the top of the document that the selection
// takes in (the one the cursor's in, with nothing selected): what "change
// this" means. None if it's in a cell (the cell's asked, instead), or
// there are no words in them.
export function blocksAt(state: EditorState): { from: number; to: number } | undefined {
  const { $from, $to } = state.selection;
  if ($from.depth < 1 || $to.depth < 1) return undefined;
  const from = $from.before(1);
  const to = $to.after(1);
  let cell = false;
  let words = false;
  state.doc.nodesBetween(from, to, (node, _pos, parent) => {
    if (parent !== state.doc) return false;
    cell ||= node.type === N.cell;
    words ||= !!node.textContent.trim();
    return false;
  });
  return cell || !words ? undefined : { from, to };
}

// wrapCell makes a cell around blocks of yours, asking for them to be
// changed: they stay as they are until its agent's written them again, and
// are kept with what you asked, to put back.
export function wrapCell(view: EditorView, range: { from: number; to: number }, prompt: string, host: CellHost) {
  const content = view.state.doc.slice(range.from, range.to).content;
  const key = Math.random().toString(36).slice(2, 10);
  view.dispatch(view.state.tr.replaceWith(range.from, range.to, N.cell.create({ key, prompt }, content)));
  host.asked(key, prompt, undefined, asMarkdown(content));
}

// askAbout asks a question about words of yours: a question cell after the
// blocks they're in, quoting them, answered beside the page (nothing's put
// in it). It's >? asked of a selection.
export function askAbout(view: EditorView, range: { from: number; to: number }, words: string, question: string, host: CellHost) {
  const quoted = words.replace(/\s+/g, " ").trim();
  const clipped = quoted.length > 160 ? `${quoted.slice(0, 157)}…` : quoted;
  const prompt = clipped ? `About “${clipped}”: ${question}` : question;
  const key = Math.random().toString(36).slice(2, 10);
  view.dispatch(view.state.tr.insert(range.to, N.cell.create({ key, prompt, ask: true })).scrollIntoView());
  host.asked(key, prompt, undefined, undefined, true);
}

// Planned is a block of an outline to put into the document: where it
// goes (the words of a heading already there, or here, start or end), the
// heading it goes under, and a sentence on what it'll say.
export interface Planned {
  key: string;
  after?: string;
  heading?: string;
  level?: number;
  summary: string;
}

const headingText = (s: string) => s.replace(/^#+\s*/, "").replace(/[:.\s]+$/, "").trim().toLowerCase();

// sectionEnd is where the section under a heading ends (before the next
// heading at its level or above, or the document's end), with the
// heading's level. None if no heading reads that way.
function sectionEnd(doc: PMNode, heading: string): { pos: number; level: number } | undefined {
  const want = headingText(heading);
  let level = 0;
  let at: number | undefined;
  doc.forEach((node, pos) => {
    if (at !== undefined) return;
    if (!level) {
      if (node.type === N.heading && headingText(node.textContent) === want) level = node.attrs.level;
      return;
    }
    if (node.type === N.heading && node.attrs.level <= level) at = pos;
  });
  return level ? { pos: at ?? doc.content.size, level } : undefined;
}

// levelAt is the level a heading put at pos would be, to sit with what's
// before it: that of the heading before, and 2 at least.
function levelAt(doc: PMNode, pos: number): number {
  let level = 2;
  doc.forEach((node, at) => {
    if (at < pos && node.type === N.heading) level = Math.max(2, node.attrs.level);
  });
  return level;
}

// placeBlocks puts an outline's blocks into the document, each empty under
// its heading, where the plan says: where the cell that planned it is
// (here, in its place), at the end of a section already there, or at the
// document's start or end. The cell that planned it goes: its blocks are
// what's there now. Blocks whose place can't be found go at the end. Each
// block is a cell named for its outline and its key (w12-b1), to be
// written into as any cell is.
export function placeBlocks(view: EditorView, origin: string, blocks: Planned[]) {
  const tr = quiet(view);
  const o = find(tr.doc, (n) => n.attrs.id === origin);
  let here = o ? o.pos : undefined;
  if (o) {
    // A document has a block in it, always: its blocks are about to be.
    if (tr.doc.childCount === 1) tr.replaceWith(o.pos, o.pos + o.node.nodeSize, N.paragraph.create());
    else tr.delete(o.pos, o.pos + o.node.nodeSize);
  }
  // Where the last block put at each place ended: the next goes after it.
  const ends = new Map<string, number>();
  for (const b of blocks) {
    const where = headingText(b.after || "end");
    let pos = ends.get(where);
    let level = b.level;
    if (pos === undefined) {
      const section = where === "here" || where === "start" || where === "end" ? undefined : sectionEnd(tr.doc, where);
      if (where === "here") pos = here ?? tr.doc.content.size;
      else if (where === "start") pos = tr.doc.firstChild?.type === N.heading ? tr.doc.firstChild.nodeSize : 0;
      else pos = section?.pos ?? tr.doc.content.size;
      level ??= section?.level;
    }
    const nodes: PMNode[] = [];
    if (b.heading) nodes.push(N.heading.create({ level: level || levelAt(tr.doc, pos) }, schema.text(b.heading)));
    nodes.push(N.cell.create({ id: `${origin}-${b.key}`, prompt: b.summary }));
    const size = nodes.reduce((n, node) => n + node.nodeSize, 0);
    tr.insert(pos, nodes);
    // What was put after this place has moved down by it.
    for (const [k, end] of ends) if (end >= pos) ends.set(k, end + size);
    if (here !== undefined && here >= pos && where !== "here") here += size;
    ends.set(where, pos + size);
    if (where === "here") here = pos + size;
  }
  view.dispatch(tr);
}

// placeCell puts a cell that's already numbered into the document, at a
// place between blocks at its top: a question asked beside the page (of a
// pass's finding), now to be written into it.
export function placeCell(view: EditorView, pos: number, id: string, prompt: string) {
  if (find(view.state.doc, (n) => n.attrs.id === id)) return;
  view.dispatch(view.state.tr.insert(Math.min(pos, view.state.doc.content.size), N.cell.create({ id, prompt, ask: true })));
  cellElement(view, id)?.scrollIntoView({ block: "center", behavior: "smooth" });
}

// unask puts a cell that couldn't be started back as the line that asked
// for it.
export function unask(view: EditorView, key: string) {
  const c = find(view.state.doc, (n) => n.attrs.key === key);
  if (!c) return;
  view.dispatch(quiet(view).replaceWith(c.pos, c.pos + c.node.nodeSize, N.paragraph.create(null, schema.text(">> " + c.node.attrs.prompt))));
}

// cellContent is markdown an agent wrote, as a cell's blocks (a cell
// inside it is only its blocks: cells don't nest).
export function cellContent(markdown: string): Fragment {
  const blocks: PMNode[] = [];
  const add = (node: PMNode) => (node.type === N.cell ? node.forEach(add) : blocks.push(node));
  parseMarkdown(markdown.trim() + "\n").doc.forEach(add);
  return Fragment.from(blocks);
}

const asMarkdown = (content: Fragment) => saveMarkdown(N.doc.create(null, content.size ? content : N.paragraph.create())).text.trim();

// cellMarkdown is what a cell holds now, as markdown; plainCell is markdown
// an agent wrote, as it reads once it's in a cell. Where the two differ,
// the cell's been edited since.
export function cellMarkdown(view: EditorView, id: string): string | undefined {
  const c = find(view.state.doc, (n) => n.attrs.id === id);
  return c && asMarkdown(c.node.content);
}

export const plainCell = (markdown: string) => asMarkdown(cellContent(markdown));

// fillCell writes what a cell's agent wrote (its version v: the first it
// wrote is 1) into the cell, in place of what was there.
export function fillCell(view: EditorView, id: string, v: number, markdown: string): boolean {
  const c = find(view.state.doc, (n) => n.attrs.id === id);
  if (!c) return false;
  const tr = quiet(view).replaceWith(c.pos + 1, c.pos + c.node.nodeSize - 1, cellContent(markdown));
  // Written into, a question is a cell like any other.
  view.dispatch(tr.setNodeMarkup(c.pos, null, { ...c.node.attrs, v: Math.max(v, c.node.attrs.v), ask: false }));
  return true;
}

// keepCell leaves what a cell holds in the document as yours, and the
// cell's gone; dropCell takes it out, with what it holds. Both are yours to
// undo.
export function keepCell(view: EditorView, name: string) {
  const c = find(view.state.doc, named(name));
  if (c) view.dispatch(view.state.tr.replaceWith(c.pos, c.pos + c.node.nodeSize, c.node.content.size ? c.node.content : N.paragraph.create()));
}

export function dropCell(view: EditorView, name: string) {
  const c = find(view.state.doc, named(name));
  if (!c) return;
  const tr = view.state.tr;
  // A document has a block in it, always.
  if (view.state.doc.childCount === 1) tr.replaceWith(c.pos, c.pos + c.node.nodeSize, N.paragraph.create());
  else tr.delete(c.pos, c.pos + c.node.nodeSize);
  view.dispatch(tr);
}

// cellElement is a cell as it's drawn, to put something beside.
export function cellElement(view: EditorView, name: string): HTMLElement | null {
  return name ? view.dom.querySelector<HTMLElement>(`.cell[data-cell="${CSS.escape(name)}"]`) : null;
}
