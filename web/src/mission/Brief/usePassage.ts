import { type RefObject, useEffect, useState } from "react";
import { pageScroller } from "../../ui/scroller";
import { pickedIn } from "./find";

// A passage picked out of the documents, and where in the window what's
// offered of it stands: over its middle.
export interface PassageAt {
  left: number;
  top: number;
  passage: string;
}

// usePassage follows what's picked out of the documents (within): the
// passage, once the pointer's let go or a key's extended it, and none
// again once they scroll under it or the pointer goes down elsewhere.
// tool is what's offered of it: a press in it isn't elsewhere.
export function usePassage(within: RefObject<HTMLElement | null>, tool: RefObject<HTMLElement | null>): [PassageAt | undefined, () => void] {
  const [at, setAt] = useState<PassageAt>();
  useEffect(() => {
    const el = within.current;
    if (!el) return;
    const look = () =>
      setTimeout(() => {
        const passage = pickedIn(el);
        const sel = getSelection();
        if (!passage || !sel?.rangeCount) return setAt(undefined);
        const r = sel.getRangeAt(0).getBoundingClientRect();
        setAt({ left: Math.max(120, Math.min(r.left + r.width / 2, window.innerWidth - 120)), top: Math.max(el.getBoundingClientRect().top + 40, r.top - 8), passage });
      });
    const mine = (e: Event) => !!tool.current?.contains(e.target as Node | null);
    // (A drag that started in the text can end outside it.)
    const up = (e: MouseEvent) => !mine(e) && look();
    const keyed = (e: KeyboardEvent) => e.shiftKey && look();
    const away = (e: MouseEvent) => !mine(e) && setAt(undefined);
    const gone = () => setAt(undefined);
    document.addEventListener("mouseup", up);
    document.addEventListener("keyup", keyed);
    document.addEventListener("mousedown", away);
    // Whatever scrolls them: the documents together, or each by itself.
    el.addEventListener("scroll", gone, { passive: true, capture: true });
    pageScroller()?.addEventListener("scroll", gone, { passive: true });
    window.addEventListener("resize", gone);
    return () => {
      document.removeEventListener("mouseup", up);
      document.removeEventListener("keyup", keyed);
      document.removeEventListener("mousedown", away);
      el.removeEventListener("scroll", gone, { capture: true });
      pageScroller()?.removeEventListener("scroll", gone);
      window.removeEventListener("resize", gone);
    };
  }, [within, tool]);
  return [at, () => setAt(undefined)];
}
