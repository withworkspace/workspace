import { type RefObject, useLayoutEffect, useState } from "react";
import { pageScroller } from "../../ui/scroller";

// usePlaceUnder is where a menu of the reader's goes (fixed): under what
// opened it, kept in the window; at the top of the reader when that's
// scrolled out of sight, at the left of what's given as its column.
export function usePlaceUnder(box: RefObject<HTMLElement | null>, anchor: () => HTMLElement | null, column?: () => HTMLElement | null): { top: number; left: number } | undefined {
  const [pos, setPos] = useState<{ top: number; left: number }>();
  useLayoutEffect(() => {
    const a = anchor();
    const el = box.current;
    if (!a || !el) return;
    const r = a.getBoundingClientRect();
    const floor = (pageScroller()?.getBoundingClientRect().top ?? 0) + 8;
    const top = Math.max(floor, r.bottom + 6);
    const from = r.bottom < floor ? (column?.()?.getBoundingClientRect().left ?? r.left) : r.left;
    const left = Math.max(8, Math.min(from, window.innerWidth - el.offsetWidth - 8));
    setPos((was) => (was?.top === top && was.left === left ? was : { top, left }));
  });
  return pos;
}
