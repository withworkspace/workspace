import { type RefObject, useEffect, useState } from "react";
import { pageScroller } from "../../ui/scroller";
import { progress, type Section, sectionAt } from "../reading";
import { headingAt } from "./goToSection";

// Reading is where you are in a document: the section you're in (its
// index, -1 before the first), how far through you are (a percentage), and
// whether you've scrolled past its title.
export interface Reading {
  at: number;
  pct: number;
  past: boolean;
}

// useReading follows where you are as you scroll, once a frame at most
// (from when its text is there to read).
export function useReading(article: RefObject<HTMLElement | null>, sections: Section[], text: string | undefined): Reading {
  const [reading, setReading] = useState<Reading>({ at: -1, pct: 0, past: false });
  useEffect(() => {
    const main = pageScroller();
    const el = article.current;
    if (!main || !el) return;
    let frame = 0;
    const measure = () => {
      frame = 0;
      const top = main.getBoundingClientRect().top;
      const tops = sections.map((s) => (headingAt(el, s.anchor)?.getBoundingClientRect().top ?? Infinity) - top);
      const next = { at: sectionAt(tops), pct: progress(main.scrollTop, main.scrollHeight, main.clientHeight), past: el.getBoundingClientRect().top < top };
      setReading((was) => (was.at === next.at && was.pct === next.pct && was.past === next.past ? was : next));
    };
    const soon = () => {
      if (!frame) frame = requestAnimationFrame(measure);
    };
    measure();
    main.addEventListener("scroll", soon, { passive: true });
    const resized = new ResizeObserver(soon);
    resized.observe(el);
    resized.observe(main);
    return () => {
      main.removeEventListener("scroll", soon);
      resized.disconnect();
      if (frame) cancelAnimationFrame(frame);
    };
  }, [article, sections, text]);
  return reading;
}
