import { type RefObject, useEffect, useState } from "react";
import { pageScroller } from "../../ui/scroller";
import { docsIn } from "./find";
import type { Heading } from "./useHeadings";

// useBeingRead is which of the documents' headings are being read: the
// last heading above a line near the top, in each thing that scrolls (the
// page, the documents together, or each by itself).
export function useBeingRead(docs: RefObject<HTMLElement | null>, heads: Heading[]): Set<HTMLElement> {
  const [here, setHere] = useState<Set<HTMLElement>>(new Set());
  useEffect(() => {
    const el = docs.current;
    if (!el) return;
    const main = pageScroller();
    const scrolls = (x: HTMLElement) => x.scrollHeight > x.clientHeight + 1 && getComputedStyle(x).overflowY !== "visible";
    const look = () => {
      const panes = docsIn(el).filter(scrolls);
      const scrollers = panes.length ? panes : [scrolls(el) || !main ? el : main];
      const read = new Set<HTMLElement>();
      for (const sc of scrollers) {
        const line = Math.max(sc.getBoundingClientRect().top, 0) + 110;
        const mine = heads.filter((h) => sc.contains(h.el));
        let cur: HTMLElement | undefined;
        for (const h of mine) if (h.el.getBoundingClientRect().top <= line) cur = h.el;
        cur ??= mine[0]?.el;
        if (cur) read.add(cur);
      }
      setHere((was) => (was.size === read.size && [...read].every((x) => was.has(x)) ? was : read));
    };
    look();
    const on = [el, ...docsIn(el), ...(main ? [main] : [])];
    for (const sc of on) sc.addEventListener("scroll", look, { passive: true });
    window.addEventListener("resize", look);
    return () => {
      for (const sc of on) sc.removeEventListener("scroll", look);
      window.removeEventListener("resize", look);
    };
  }, [docs, heads]);
  return here;
}
