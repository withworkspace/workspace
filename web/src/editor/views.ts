import type { Node as PMNode } from "prosemirror-model";
import { NodeSelection, Plugin, PluginKey } from "prosemirror-state";
import { Decoration, DecorationSet, type EditorView, type NodeView } from "prosemirror-view";
import { createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { Mermaid } from "../markdown/Mermaid";
import { view as openViewer } from "../ui/Viewer";
import { highlightCode, loadHighlighter } from "../review/highlight";
import { drawsDiagrams, isDiagram, showSource, showsSource } from "./diagrams";
import { schema } from "./schema";
import { matchQuote } from "../annotate/anchor";

// How a document draws while it's edited, where the schema's plain HTML
// isn't enough: tasks with boxes you can tick, code highlighted with its
// language shown, and a word on what to write when it's empty. It's drawn
// as the reader draws it, so it looks the same when you start typing; a
// ```mermaid block as its diagram, as the reader draws it too.

const { nodes: N } = schema;

// Item is a list item: with a box to tick, if it's a task.
class Item implements NodeView {
  dom: HTMLLIElement;
  contentDOM: HTMLElement;
  private box?: HTMLInputElement;

  constructor(
    private node: PMNode,
    view: EditorView,
    getPos: () => number | undefined,
  ) {
    this.dom = document.createElement("li");
    if (node.attrs.checked === null) {
      this.contentDOM = this.dom;
      return;
    }
    this.dom.className = "task";
    const box = document.createElement("input");
    box.type = "checkbox";
    box.checked = node.attrs.checked;
    box.contentEditable = "false";
    box.setAttribute("aria-label", "Done");
    box.disabled = !view.editable;
    // The box takes the click, not the editor (which would move the
    // cursor there).
    box.addEventListener("mousedown", (e) => e.preventDefault());
    // The browser ticks it; the document follows. (Preventing the click
    // would have the browser untick it again afterwards.)
    box.addEventListener("click", () => {
      const pos = getPos();
      if (pos === undefined || !view.editable) return;
      view.dispatch(view.state.tr.setNodeMarkup(pos, null, { ...this.node.attrs, checked: !this.node.attrs.checked }));
    });
    const body = document.createElement("div");
    body.className = "task-body";
    this.dom.append(box, body);
    this.box = box;
    this.contentDOM = body;
  }

  update(node: PMNode): boolean {
    if (node.type !== this.node.type || (node.attrs.checked === null) !== (this.node.attrs.checked === null)) return false;
    this.node = node;
    if (this.box) this.box.checked = node.attrs.checked;
    return true;
  }

  stopEvent(e: Event): boolean {
    return !!this.box && e.target === this.box;
  }

  ignoreMutation(m: MutationRecord | { type: "selection"; target: Node }): boolean {
    return !!this.box && m.target === this.box;
  }
}

// Code is a fenced block, its language shown in the corner as the reader
// shows it.
class Code implements NodeView {
  dom: HTMLElement;
  contentDOM: HTMLElement;
  private lang: HTMLElement;

  constructor(private node: PMNode) {
    this.dom = document.createElement("pre");
    this.dom.className = "code-block";
    this.lang = document.createElement("span");
    this.lang.className = "code-lang";
    this.lang.contentEditable = "false";
    this.contentDOM = document.createElement("code");
    this.dom.append(this.lang, this.contentDOM);
    this.label();
  }

  private label() {
    this.lang.textContent = this.node.attrs.lang;
    this.lang.hidden = !this.node.attrs.lang;
  }

  update(node: PMNode): boolean {
    if (node.type !== this.node.type || isDiagram(node) !== isDiagram(this.node)) return false;
    this.node = node;
    this.label();
    return true;
  }

  ignoreMutation(m: MutationRecord | { type: "selection"; target: Node }): boolean {
    return m.type !== "selection" && this.lang.contains(m.target);
  }
}

// Diagram is a ```mermaid block: drawn as the reader draws it, or its
// source, to edit, with a toggle top left to show one or the other. Which
// it shows is the editor's (diagrams.ts), handed to it as a decoration;
// its text is drawn again when it's shown drawn, not as it's typed.
class Diagram implements NodeView {
  dom: HTMLElement;
  contentDOM: HTMLElement;
  private toggle: HTMLElement;
  private buttons: { drawn: HTMLButtonElement; source: HTMLButtonElement };
  private drawing: HTMLElement;
  private code: HTMLElement;
  private lang: HTMLElement;
  private root: Root;
  private source = false;
  private drawn?: string; // the text the drawing's of

  constructor(
    private node: PMNode,
    private view: EditorView,
    private getPos: () => number | undefined,
    decorations: readonly Decoration[],
  ) {
    this.dom = document.createElement("div");
    this.dom.className = "diagram-block";

    this.toggle = document.createElement("div");
    this.toggle.className = "segmented diagram-toggle";
    this.toggle.contentEditable = "false";
    this.toggle.setAttribute("role", "group");
    this.toggle.setAttribute("aria-label", "Show the diagram or its source");
    const button = (label: string, source: boolean) => {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = label;
      // The button takes the click, not the editor (which would move the
      // cursor there).
      b.addEventListener("mousedown", (e) => e.preventDefault());
      b.addEventListener("click", () => this.show(source));
      return b;
    };
    this.buttons = { drawn: button("Diagram", false), source: button("Source", true) };
    this.toggle.append(this.buttons.drawn, this.buttons.source);

    // Clicking the drawing selects the block, as clicking an image does.
    this.drawing = document.createElement("div");
    this.drawing.className = "diagram-drawing";
    this.drawing.contentEditable = "false";
    this.drawing.addEventListener("mousedown", (e) => {
      const pos = this.getPos();
      if (pos === undefined || !this.view.editable || e.button !== 0) return;
      e.preventDefault();
      this.view.dispatch(this.view.state.tr.setSelection(NodeSelection.create(this.view.state.doc, pos)));
      this.view.focus();
    });
    // A double click opens it in the viewer, to move around in.
    this.drawing.title = "Double-click to open it larger";
    this.drawing.addEventListener("dblclick", () => {
      const svg = this.drawing.querySelector("svg");
      if (svg) openViewer({ kind: "svg", svg: svg.outerHTML, title: "Diagram" });
    });
    this.root = createRoot(this.drawing);

    this.code = document.createElement("pre");
    this.code.className = "code-block";
    this.lang = document.createElement("span");
    this.lang.className = "code-lang";
    this.lang.contentEditable = "false";
    this.lang.textContent = node.attrs.lang;
    this.contentDOM = document.createElement("code");
    this.code.append(this.lang, this.contentDOM);

    this.dom.append(this.toggle, this.drawing, this.code);
    this.set(showsSource(decorations));
  }

  // show asks the editor to show the block drawn or as its source.
  private show(source: boolean) {
    const pos = this.getPos();
    if (pos === undefined || source === this.source) return;
    this.view.dispatch(showSource(this.view.state, pos, source));
    if (source && this.view.editable) this.view.focus();
  }

  // set shows it drawn or as its source.
  private set(source: boolean) {
    this.source = source;
    this.dom.classList.toggle("shows-source", source);
    this.buttons.drawn.setAttribute("aria-pressed", String(!source));
    this.buttons.source.setAttribute("aria-pressed", String(source));
    this.drawing.hidden = source;
    this.code.hidden = !source;
    if (!source) this.draw();
  }

  private draw() {
    const code = this.node.textContent;
    if (code === this.drawn) return;
    this.drawn = code;
    this.root.render(createElement(Mermaid, { code, viewable: false }));
  }

  update(node: PMNode, decorations: readonly Decoration[]): boolean {
    if (!isDiagram(node)) return false;
    this.node = node;
    const source = showsSource(decorations);
    if (source !== this.source) this.set(source);
    // Changed while it's drawn (an undo): drawn again.
    else if (!source) this.draw();
    return true;
  }

  stopEvent(e: Event): boolean {
    return this.toggle.contains(e.target as Node) || this.drawing.contains(e.target as Node);
  }

  // What's drawn, the toggle and the label aren't the document; the source
  // is, and what wraps it (typing over all of it can take the browser past
  // it).
  ignoreMutation(m: MutationRecord | { type: "selection"; target: Node }): boolean {
    if (m.type === "selection") return false;
    return this.toggle.contains(m.target) || this.drawing.contains(m.target) || this.lang.contains(m.target);
  }

  destroy() {
    // Not while React may be drawing (the page the editor's in going).
    const root = this.root;
    queueMicrotask(() => root.unmount());
  }
}

export const nodeViews = {
  list_item: (node: PMNode, view: EditorView, getPos: () => number | undefined) => new Item(node, view, getPos),
  code_block: (node: PMNode, view: EditorView, getPos: () => number | undefined, decorations: readonly Decoration[]) =>
    isDiagram(node) && drawsDiagrams(view.state) ? new Diagram(node, view, getPos, decorations) : new Code(node),
};

// --- Highlighting

const highlightKey = new PluginKey<DecorationSet>("highlight");

// Each code block's highlighting, kept while the block's unchanged (and
// forgotten when highlight.js arrives, to be done again in colour).
let tokens = new WeakMap<PMNode, { from: number; to: number; cls: string }[]>();

const entity: Record<string, string> = { amp: "&", lt: "<", gt: ">", quot: '"', "#x27": "'", "#39": "'" };

// spans is where highlight.js's classes fall in a block's text: its HTML
// read back as runs of text, each with the classes of the spans it's in.
function spans(node: PMNode): { from: number; to: number; cls: string }[] {
  const known = tokens.get(node);
  if (known) return known;
  const out: { from: number; to: number; cls: string }[] = [];
  const html = highlightCode(node.textContent, node.attrs.lang || undefined);
  const open: string[] = [];
  let at = 0;
  for (const m of html.matchAll(/<span class="([^"]*)">|<\/span>|[^<]+/g)) {
    if (m[1] !== undefined) open.push(m[1]);
    else if (m[0] === "</span>") open.pop();
    else {
      const text = m[0].replace(/&(amp|lt|gt|quot|#x27|#39);/g, (_, e: string) => entity[e]);
      if (open.length) out.push({ from: at, to: at + text.length, cls: open.join(" ") });
      at += text.length;
    }
  }
  tokens.set(node, out);
  return out;
}

function highlights(doc: PMNode): DecorationSet {
  const decos: Decoration[] = [];
  doc.descendants((node, pos) => {
    if (node.type !== N.code_block) return true;
    if (node.attrs.lang) for (const s of spans(node)) decos.push(Decoration.inline(pos + 1 + s.from, pos + 1 + s.to, { class: s.cls }));
    return false;
  });
  return DecorationSet.create(doc, decos);
}

// highlighting colours code blocks by their language, once highlight.js
// has loaded.
export const highlighting = new Plugin<DecorationSet>({
  key: highlightKey,
  state: {
    init: (_, state) => highlights(state.doc),
    apply: (tr, set, _old, state) => (tr.docChanged || tr.getMeta(highlightKey) ? highlights(state.doc) : set),
  },
  props: {
    decorations: (state) => highlightKey.getState(state),
  },
  view(view) {
    let live = true;
    loadHighlighter().then(() => {
      tokens = new WeakMap();
      if (live) view.dispatch(view.state.tr.setMeta(highlightKey, true));
    });
    return {
      destroy: () => {
        live = false;
      },
    };
  },
});

// placeholder shows a word on what to write while the document's empty.
export function placeholder(text: string): Plugin {
  return new Plugin({
    props: {
      decorations(state) {
        const doc = state.doc;
        const first = doc.firstChild;
        if (!text || doc.childCount !== 1 || !first?.isTextblock || first.content.size) return null;
        return DecorationSet.create(doc, [Decoration.node(0, first.nodeSize, { class: "empty", "data-placeholder": text })]);
      },
    },
  });
}

// --- Threads

// Quote is text a thread is on, and its kind (ask, change, note), for its
// colour.
export interface Quote {
  exact: string;
  prefix?: string;
  kind: string;
}

// quoteMarks highlights the words threads are on, as the reader does, and
// keeps each on its words as text around it changes.
export function quoteMarks(quotes: Quote[]): Plugin {
  const key = new PluginKey<DecorationSet>("quotes");
  return new Plugin<DecorationSet>({
    key,
    state: {
      init: (_, state) => findQuotes(state.doc, quotes),
      apply: (tr, set) => set.map(tr.mapping, tr.doc),
    },
    props: {
      decorations: (state) => key.getState(state),
    },
  });
}

// findQuotes is where each quote is in the document: its text block by
// block, a line break between blocks, as the reader's index has it
// (annotate/anchor.ts), any run of white space as one space (the reader's
// text keeps a paragraph's line breaks, the editor's doesn't); the first
// place it's found, or the first after its prefix.
function findQuotes(doc: PMNode, quotes: Quote[]): DecorationSet {
  const decos: Decoration[] = [];
  quoteRanges(doc, quotes).forEach((ranges, i) => {
    for (const r of ranges) decos.push(Decoration.inline(r.from, r.to, { class: `thread-mark k-${quotes[i].kind}` }));
  });
  return DecorationSet.create(doc, decos);
}

// quoteRanges is where each quote is in the document, as runs of text next
// to each other (one per block it's in); none for one that isn't there.
export function quoteRanges(doc: PMNode, quotes: { exact: string; prefix?: string; suffix?: string }[]): { from: number; to: number }[][] {
  let text = "";
  const at: number[] = [];
  const add = (ch: string, pos: number) => {
    if (/\s/.test(ch)) {
      if (!text || text.endsWith(" ")) return;
      ch = " ";
    }
    text += ch;
    at.push(pos);
  };
  doc.descendants((node, pos) => {
    if (!node.isTextblock) return true;
    add("\n", -1);
    node.forEach((child, offset) => {
      if (child.isText) for (let i = 0; i < child.text!.length; i++) add(child.text![i], pos + 1 + offset + i);
    });
    return false;
  });
  return quotes.map((q) => {
    const out: { from: number; to: number }[] = [];
    const m = matchQuote(text, q);
    if (!m) return out;
    const [i, end] = m;
    // A run of characters next to each other in the document, one each (a
    // quote across blocks is a run per block).
    let from = -1;
    let to = -1;
    const flush = () => {
      if (from >= 0) out.push({ from, to });
      from = -1;
    };
    for (let k = i; k < end; k++) {
      const p = at[k];
      if (p < 0) flush();
      else if (from >= 0 && p === to) to = p + 1;
      else {
        flush();
        from = p;
        to = p + 1;
      }
    }
    flush();
    return out;
  });
}
