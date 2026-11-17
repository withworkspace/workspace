import type { Node as PMNode } from "prosemirror-model";
import { Plugin, PluginKey } from "prosemirror-state";
import { Decoration, DecorationSet, type EditorView } from "prosemirror-view";

// Refs: the things a document's words name (a file, a pull request, a
// mission), drawn as what they are. Nothing's written into the document:
// a ref is its words, found again each time they change, and marked over
// them. The page it's in says what's a ref (it knows what there is to
// name), shows a card when one's hovered, and goes to it on ⌘-click.

// RefMark is a ref in a run of text: where, a key that names it, its kind,
// and how it's drawn (refs/views.tsx): its shape and accent colour, text
// shown in place of its name (a repo's label) while the cursor's not in
// it, and words to show after it (where a mission stands, a page's
// title), with their tone.
export interface RefMark {
  from: number;
  to: number;
  key: string;
  kind: string;
  shape?: string;
  accent?: string;
  text?: string;
  label?: string;
  tone?: string;
  // words: a link's words are the writer's own, not the ref's name.
  words?: boolean;
}

// RefHost is the page: find says what's a ref in a run of text; over and
// out are the pointer coming onto one and leaving it; follow is ⌘-click.
export interface RefHost {
  find(text: string): RefMark[];
  // at is the ref a link is to, by its address, with its words.
  at?(href: string, words: string): RefMark | undefined;
  over(key: string, el: HTMLElement): void;
  out(): void;
  follow(key: string): void;
}

// The plugin's state: what's drawn, and whether any ref is shown as its
// own text (only then does moving the cursor change what's drawn).
interface Drawn {
  set: DecorationSet;
  shown: boolean;
}

const refsKey = new PluginKey<Drawn>("refs");

// pill is a ref's own text, drawn in place of its name.
function pill(r: RefMark): HTMLElement {
  const el = document.createElement("span");
  el.className = `named-ref k-${r.kind} shape-${r.shape ?? "words"} ref-shown`;
  if (r.accent) el.style.setProperty("--ref-accent", r.accent);
  el.contentEditable = "false";
  el.textContent = r.text ?? "";
  el.dataset.ref = r.key;
  return el;
}

function build(doc: PMNode, host: RefHost | undefined, sel?: { from: number; to: number }): Drawn {
  if (!host) return { set: DecorationSet.empty, shown: false };
  const decos: Decoration[] = [];
  let anyText = false;
  const draw = (from: number, to: number, r: RefMark) => {
    // Its own text in place of its name, until the cursor's in it: then
    // the name, to edit. Not for words of the writer's own.
    const text = r.words ? undefined : r.text;
    anyText ||= !!text;
    const shown = text && !(sel && sel.from <= to && sel.to >= from);
    const style = r.accent ? `--ref-accent: ${r.accent}` : "";
    if (shown) {
      decos.push(Decoration.widget(from, () => pill(r), { side: -1, key: `pill:${r.key}:${text}:${r.accent ?? ""}`, marks: [] }));
      decos.push(Decoration.inline(from, to, { class: "ref-hidden" }));
    } else {
      decos.push(Decoration.inline(from, to, { class: `named-ref k-${r.kind} shape-${r.words ? "words" : (r.shape ?? "words")}${r.words ? " ref-words" : ""}`, style, "data-ref": r.key }));
    }
    if (r.label) {
      const label = r.label;
      const tone = r.tone;
      decos.push(
        Decoration.widget(
          to,
          () => {
            const el = document.createElement("span");
            el.className = `ref-label${tone ? ` tone-${tone}` : ""}`;
            el.contentEditable = "false";
            el.textContent = label;
            el.dataset.ref = r.key;
            return el;
          },
          { side: 1, key: `ref:${r.key}:${label}:${tone ?? ""}`, marks: [] },
        ),
      );
    }
  };
  doc.descendants((node, pos, parent) => {
    if (node.type.name === "code_block") return false;
    if (!node.isText || !node.text || parent?.type.name === "heading") return true;
    // A link to a ref (as Link what's named writes one) is drawn as the
    // ref is; with words of the writer's own, as those. Any other link is
    // somewhere to go already.
    const link = node.marks.find((m) => m.type.name === "link");
    if (link) {
      const r = host.at?.(String(link.attrs.href ?? ""), node.text);
      if (r) draw(pos, pos + node.nodeSize, r);
      return true;
    }
    for (const r of host.find(node.text)) draw(pos + r.from, pos + r.to, r);
    return true;
  });
  return { set: DecorationSet.create(doc, decos), shown: anyText };
}

// refsPlugin draws refs, and tells the page when one's hovered or
// followed.
export function refsPlugin(host: () => RefHost | undefined): Plugin {
  const at = (target: EventTarget | null) => (target instanceof Element ? target.closest<HTMLElement>("[data-ref]") : null);
  return new Plugin<Drawn>({
    key: refsKey,
    state: {
      init: (_, state) => build(state.doc, host(), state.selection),
      // Again when the cursor moves, for a ref shown as its own text.
      apply: (tr, was, _old, state) => (tr.docChanged || (tr.selectionSet && was.shown) || tr.getMeta(refsKey) ? build(state.doc, host(), state.selection) : was),
    },
    props: {
      decorations: (state) => refsKey.getState(state)?.set,
      handleDOMEvents: {
        mouseover(_view, e) {
          const el = at(e.target);
          if (el?.dataset.ref) host()?.over(el.dataset.ref, el);
          return false;
        },
        mouseout(_view, e) {
          if (at(e.target) && !at(e.relatedTarget)) host()?.out();
          return false;
        },
      },
      handleClick(_view, _pos, e) {
        const el = at(e.target);
        if (!el?.dataset.ref || !(e.metaKey || e.ctrlKey)) return false;
        host()?.follow(el.dataset.ref);
        return true;
      },
    },
  });
}

// refreshRefs finds the refs again: what there is to name has changed.
export function refreshRefs(view: EditorView) {
  view.dispatch(view.state.tr.setMeta(refsKey, true).setMeta("addToHistory", false));
}

// Link is a name found in a run of text, and the address to link it to.
export interface Link {
  from: number;
  to: number;
  href: string;
}

// linkNamed makes the names in a document links (Link what's named): each
// run of text not in code blocks, headings or a link already is given to
// links (with whether it's code: a name in backticks is linked whole), and
// what it finds is linked, in one step that undo takes back. It says how
// many it linked.
export function linkNamed(view: EditorView, links: (text: string, code: boolean) => Link[]): number {
  const { state } = view;
  const link = state.schema.marks.link;
  const tr = state.tr;
  let n = 0;
  state.doc.descendants((node, pos, parent) => {
    if (node.type.name === "code_block") return false;
    if (!node.isText || !node.text || parent?.type.name === "heading" || node.marks.some((m) => m.type === link)) return true;
    for (const l of links(node.text, node.marks.some((m) => m.type.name === "code"))) {
      tr.addMark(pos + l.from, pos + l.to, link.create({ href: l.href }));
      n++;
    }
    return true;
  });
  if (n) view.dispatch(tr);
  return n;
}
