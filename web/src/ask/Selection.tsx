import { type Ref, type RefObject, useEffect, useImperativeHandle, useRef, useState } from "react";
import { pageScroller } from "../ui/scroller";
import { type Passage, passageOf } from "./ask";
import { SelectionToolbar } from "./SelectionToolbar";

// The toolbar over a passage picked out of what's asked about (#123): Ask
// about this (a), Explain, and Link (SelectionToolbar). A page gives it
// where text is picked out of, and what each does; the ask bar does the
// asking. This is when it's up, and where.

// passageIn is what's selected in root, as a passage: its text, a little
// either side, and the section it's in (the h2 or h3 before it). undefined
// with nothing (or too little) selected there.
export const passageIn = (root: HTMLElement | null) => picked(root)?.passage;

// picked is what's selected in root, and where: a selection that runs on
// past it (a drag past the end of a line, into what's beside the text) is
// what of it is in root.
function picked(root: HTMLElement | null): { passage: Passage; range: Range } | undefined {
  const sel = getSelection();
  if (!root || !sel || sel.isCollapsed || !sel.rangeCount) return undefined;
  const r = sel.getRangeAt(0).cloneRange();
  if (!root.contains(r.commonAncestorContainer)) {
    if (!r.intersectsNode(root)) return undefined;
    const all = document.createRange();
    all.selectNodeContents(root);
    if (r.compareBoundaryPoints(Range.START_TO_START, all) < 0) r.setStart(all.startContainer, all.startOffset);
    if (r.compareBoundaryPoints(Range.END_TO_END, all) > 0) r.setEnd(all.endContainer, all.endOffset);
  }
  const before = document.createRange();
  before.selectNodeContents(root);
  before.setEnd(r.startContainer, r.startOffset);
  const after = document.createRange();
  after.selectNodeContents(root);
  after.setStart(r.endContainer, r.endOffset);
  let section: Passage["section"];
  for (const h of root.querySelectorAll<HTMLElement>("h2[data-anchor], h3[data-anchor]")) {
    if (h.contains(r.startContainer) || h.compareDocumentPosition(r.startContainer) & Node.DOCUMENT_POSITION_FOLLOWING) {
      section = { anchor: h.dataset.anchor ?? "", title: h.textContent?.trim() ?? "" };
    } else break;
  }
  const passage = passageOf(r.toString(), before.toString(), after.toString(), section);
  return passage && { passage, range: r };
}

// SelectionToolsHandle drops the toolbar (esc), saying whether it was up.
export interface SelectionToolsHandle {
  hide: () => boolean;
}

export function SelectionTools({
  within,
  ask,
  explain,
  link,
  ref,
}: {
  within: RefObject<HTMLElement | null>;
  ask: (p: Passage) => void;
  explain: (p: Passage) => void;
  link?: (p: Passage) => void; // a link to where it is (its section)
  ref?: Ref<SelectionToolsHandle>;
}) {
  const [at, setAt] = useState<{ left: number; top: number; passage: Passage }>();
  const bar = useRef<HTMLDivElement>(null);
  useImperativeHandle(ref, () => ({
    hide: () => {
      if (!at) return false;
      setAt(undefined);
      return true;
    },
  }));
  // Up once a passage is picked out (by the mouse, or shift and the
  // arrows), over its middle; down at a click anywhere else, or as the page
  // scrolls or resizes under it.
  useEffect(() => {
    const el = within.current;
    if (!el) return;
    const look = () =>
      setTimeout(() => {
        const p = picked(el);
        if (!p) return setAt(undefined);
        const r = p.range.getBoundingClientRect();
        const floor = (pageScroller()?.getBoundingClientRect().top ?? 0) + 44;
        setAt({ left: Math.max(120, Math.min(r.left + r.width / 2, window.innerWidth - 120)), top: Math.max(floor, r.top - 8), passage: p.passage });
      });
    const mine = (e: Event) => !!bar.current?.contains(e.target as Node | null);
    // (A drag that started in the text can end outside it.)
    const up = (e: MouseEvent) => !mine(e) && look();
    const keyed = (e: KeyboardEvent) => e.shiftKey && look();
    const away = (e: MouseEvent) => !mine(e) && setAt(undefined);
    const gone = () => setAt(undefined);
    document.addEventListener("mouseup", up);
    document.addEventListener("keyup", keyed);
    document.addEventListener("mousedown", away);
    pageScroller()?.addEventListener("scroll", gone, { passive: true });
    window.addEventListener("resize", gone);
    return () => {
      document.removeEventListener("mouseup", up);
      document.removeEventListener("keyup", keyed);
      document.removeEventListener("mousedown", away);
      pageScroller()?.removeEventListener("scroll", gone);
      window.removeEventListener("resize", gone);
    };
  }, [within]);
  if (!at) return null;
  const done = (fn: (p: Passage) => void) => () => {
    setAt(undefined);
    fn(at.passage);
    getSelection()?.removeAllRanges();
  };
  return <SelectionToolbar ref={bar} left={at.left} top={at.top} onAsk={done(ask)} onExplain={done(explain)} onLink={link && done(link)} />;
}
