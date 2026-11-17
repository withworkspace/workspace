import "prosemirror-view/style/prosemirror.css";
import { EditorState, Plugin } from "prosemirror-state";
import { Decoration, DecorationSet, EditorView } from "prosemirror-view";
import { useEffect, useMemo, useRef } from "react";
import { type Change, trackChanges } from "./track";
import { highlighting, nodeViews } from "./views";

// TrackedDoc shows a document with suggested edits in place, as tracked
// changes: what each takes out struck through, what it puts in underlined,
// and its Accept and Decline where it ends. It's the editor's drawing of
// the document (read-only), so it reads as the document does.

export interface TrackedChange extends Change {
  by: string; // who suggested it: "agent", "you", a crew id
}

export interface TrackedProps {
  text: string;
  changes: TrackedChange[];
  // focus is the change to show (scrolled to, marked); settling, the one
  // being written in.
  focus?: string;
  settling?: string;
  onSettle: (id: string, action: "accept" | "decline") => void;
  onFocus?: (id: string) => void;
  // onUnplaced hears of the changes that can't be shown in place (their
  // text isn't in the document once, or they overlap one placed already).
  onUnplaced?: (ids: string[]) => void;
}

export default function TrackedDoc({ text, changes, focus, settling, onSettle, onFocus, onUnplaced }: TrackedProps) {
  const host = useRef<HTMLDivElement>(null);
  const tracked = useMemo(() => trackChanges(text, changes), [text, changes]);
  const on = useRef({ onSettle, onFocus, onUnplaced });
  on.current = { onSettle, onFocus, onUnplaced };
  useEffect(() => {
    if (tracked.unplaced.length) on.current.onUnplaced?.(tracked.unplaced);
  }, [tracked]);

  useEffect(() => {
    const by = new Map(changes.map((c) => [c.id, c.by]));
    const mark = (id: string) => (id === focus ? " tc-focus" : "");
    const decos: Decoration[] = [];
    for (const s of tracked.spans) decos.push(Decoration.inline(s.from, s.to, { class: `tc tc-${s.kind}${mark(s.id)}`, "data-change": s.id }));
    for (const w of tracked.wholes) {
      decos.push(Decoration.node(w.pos, w.pos + tracked.doc.nodeAt(w.pos)!.nodeSize, { class: `tc-block tc-${w.kind}${mark(w.id)}`, "data-change": w.id }));
    }
    for (const e of tracked.ends) {
      const state = { focused: e.id === focus, settling: e.id === settling };
      decos.push(Decoration.widget(e.pos, () => bar(e.id, by.get(e.id) ?? "", state, e.inline, on), { side: 1, key: `${e.id}:${state.focused}:${state.settling}` }));
    }
    const set = DecorationSet.create(tracked.doc, decos);
    const view = new EditorView(host.current!, {
      state: EditorState.create({ doc: tracked.doc, plugins: [highlighting, new Plugin({ props: { decorations: () => set } })] }),
      editable: () => false,
      nodeViews,
      attributes: { class: "markdown pm tracked", "aria-label": "The document with its suggested edits" },
      handleClick: (_view, _pos, event) => {
        const id = (event.target as HTMLElement).closest("[data-change]")?.getAttribute("data-change");
        if (id) on.current.onFocus?.(id);
        return false;
      },
    });
    const frame = requestAnimationFrame(() => view.dom.querySelector(".tc-focus")?.scrollIntoView({ block: "center" }));
    return () => {
      cancelAnimationFrame(frame);
      view.destroy();
    };
  }, [tracked, changes, focus, settling]);

  return (
    <div className="prose">
      <div ref={host} />
    </div>
  );
}

// bar is a change's Accept and Decline, where it ends: in its line, or
// under a block it puts in or takes out.
function bar(id: string, by: string, state: { focused: boolean; settling: boolean }, inline: boolean, on: { current: Pick<TrackedProps, "onSettle" | "onFocus"> }): HTMLElement {
  const el = document.createElement(inline ? "span" : "div");
  el.className = `tc-bar${state.focused ? " focused" : ""}`;
  el.contentEditable = "false";
  el.dataset.change = id;
  const who = document.createElement("span");
  who.className = "tc-by";
  who.textContent = by === "agent" ? "The agent's edit" : by === "you" ? "Your edit" : `${by}'s edit`;
  el.append(who);
  if (state.settling) {
    const busy = document.createElement("span");
    busy.className = "tc-busy";
    busy.textContent = "Writing it in…";
    el.append(busy);
    return el;
  }
  const button = (label: string, key: string, action: "accept" | "decline", primary: boolean) => {
    const b = document.createElement("button");
    b.type = "button";
    b.className = `small ${primary ? "primary" : "ghost"}`;
    b.textContent = label;
    if (state.focused) {
      const k = document.createElement("kbd");
      k.textContent = key;
      b.append(" ", k);
    }
    b.title = `${label} it`;
    b.addEventListener("mousedown", (e) => e.preventDefault());
    b.addEventListener("click", (e) => {
      e.stopPropagation();
      on.current.onSettle(id, action);
    });
    return b;
  };
  el.append(button("Decline", "d", "decline", false), button("Accept", "a", "accept", true));
  return el;
}
