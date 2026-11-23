import { type RefObject, useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { progress, railRoom, type Section, sectionAt } from "./reading";
import { Button } from "../ui/Button";
import { pageScroller } from "../ui/scroller";

// The reader's contents (#123): a rail beside the text where there's room
// for one, else a pill that sticks to the top once you've scrolled past the
// title, which opens them as a menu. Both say which section you're in and
// how far through you are. reading.ts works that out; this draws it.

const headingAt = (article: HTMLElement | null, anchor: string) => article?.querySelector<HTMLElement>(`[data-anchor="${CSS.escape(anchor)}"]`) ?? null;

// useSections is a document's sections and its title (its first h1), read
// from its text as it's drawn: the h2 and h3 headings the markdown named.
// Read again whenever what's drawn changes, as it's drawn after it loads
// (text names that: it's read again when the document is).
export function useSections(article: RefObject<HTMLElement | null>, text: string | undefined): { title?: string; sections: Section[] } {
  const [found, setFound] = useState<{ title?: string; sections: Section[] }>({ sections: [] });
  useEffect(() => {
    const el = article.current;
    if (!el) return setFound({ sections: [] });
    const read = () => {
      const title = el.querySelector("h1")?.textContent?.trim() || undefined;
      const sections = [...el.querySelectorAll<HTMLElement>("h2[data-anchor], h3[data-anchor]")].map(
        (h): Section => ({ anchor: h.dataset.anchor ?? "", level: h.tagName === "H2" ? 2 : 3, text: h.textContent?.trim() ?? "" }),
      );
      setFound((was) => (was.title === title && sameSections(was.sections, sections) ? was : { title, sections }));
    };
    read();
    const watch = new MutationObserver(read);
    watch.observe(el, { childList: true, subtree: true, characterData: true });
    return () => watch.disconnect();
  }, [article, text]);
  return found;
}

const sameSections = (a: Section[], b: Section[]) => a.length === b.length && a.every((s, i) => s.anchor === b[i].anchor && s.level === b[i].level && s.text === b[i].text);

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

// useRailFits is whether the rail fits beside the text in the reader as
// wide as it is now (the page's width inside its padding).
export function useRailFits(page: RefObject<HTMLElement | null>): boolean {
  const [fits, setFits] = useState(true);
  useLayoutEffect(() => {
    const el = page.current;
    if (!el) return;
    const check = () => {
      const s = getComputedStyle(el);
      setFits(el.clientWidth - parseFloat(s.paddingLeft) - parseFloat(s.paddingRight) >= railRoom);
    };
    check();
    const watch = new ResizeObserver(check);
    watch.observe(el);
    return () => watch.disconnect();
  }, [page]);
  return fits;
}

// Hidden with t, the rail stays hidden from one document to the next until
// it's shown again or the app's reloaded.
let railHiddenNow = false;

export function useRailHidden(): [boolean, () => void] {
  const [hidden, setHidden] = useState(railHiddenNow);
  const toggle = useCallback(() => setHidden((h) => (railHiddenNow = !h)), []);
  return [hidden, toggle];
}

// goToSection scrolls the reader to a section's heading, and marks it for
// a moment so the eye finds it.
export function goToSection(article: HTMLElement | null, anchor: string) {
  const h = headingAt(article, anchor);
  if (!h) return;
  const still = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  h.scrollIntoView({ block: "start", behavior: still ? "auto" : "smooth" });
  h.classList.remove("flash");
  void h.offsetWidth; // so the mark starts again
  h.classList.add("flash");
}

// ContentsRail is the contents beside the text: "On this page", how far
// through you are, and each section, the one you're in marked and those
// you've passed less faint. Hidden (t), it keeps its room, so the text
// doesn't move.
export function ContentsRail({ sections, reading, hidden, go }: { sections: Section[]; reading: Reading; hidden: boolean; go: (anchor: string) => void }) {
  const box = useRef<HTMLElement>(null);
  // In a long list, the section you're in stays in sight.
  useEffect(() => {
    const rail = box.current;
    const on = rail?.querySelector<HTMLElement>(".contents-item.on");
    if (!rail || !on) return;
    if (on.offsetTop < rail.scrollTop || on.offsetTop > rail.scrollTop + rail.clientHeight - 40) rail.scrollTop = on.offsetTop - rail.clientHeight / 2;
  }, [reading.at]);
  return (
    <aside className={`doc-rail${hidden ? " hidden" : ""}`} ref={box} aria-label="On this page" aria-hidden={hidden || undefined}>
      <div className="contents-head">
        <Icon name="list" size={12} /> On this page <span className="contents-pct">{reading.pct}%</span>
      </div>
      <div className="contents-track" aria-hidden>
        <i style={{ width: `${reading.pct}%` }} />
      </div>
      {sections.length > 0 ? (
        <nav className="contents-list">
          {sections.map((s, i) => (
            <a
              key={s.anchor}
              href={`#${s.anchor}`}
              className={`contents-item l${s.level}${i === reading.at ? " on" : i < reading.at ? " read" : ""}`}
              aria-current={i === reading.at ? "location" : undefined}
              tabIndex={hidden ? -1 : undefined}
              onClick={(e) => {
                if (e.metaKey || e.ctrlKey || e.shiftKey) return;
                e.preventDefault();
                go(s.anchor);
              }}
            >
              {s.text}
            </a>
          ))}
        </nav>
      ) : (
        <p className="contents-empty">No sections.</p>
      )}
    </aside>
  );
}

// ContentsPill is the contents where the rail doesn't fit: a bar stuck to
// the top once you've scrolled past the title, saying which section you're
// in and how far through you are. It opens the contents (as t does).
export function ContentsPill({ title, sections, reading, open, toggle, pill }: { title: string; sections: Section[]; reading: Reading; open: boolean; toggle: () => void; pill: RefObject<HTMLButtonElement | null> }) {
  const at = sections[reading.at];
  return (
    <div className={`doc-pill-wrap${reading.past ? " shown" : ""}`}>
      <button type="button" ref={pill} className="doc-pill" data-contents-toggle aria-expanded={open} aria-hidden={!reading.past || undefined} tabIndex={reading.past ? undefined : -1} title="Its contents (t)" onClick={toggle}>
        <span className="doc-pill-doc">{title}</span>
        <Icon name="chevron-right" size={12} />
        <span className="doc-pill-at">{at ? at.text : "Top"}</span>
        <Icon name="chevron-down" size={12} />
        <span className="spacer" />
        <span className="doc-pill-pct">{reading.pct}%</span>
        <span className="doc-pill-line" style={{ width: `${reading.pct}%` }} />
      </button>
    </div>
  );
}

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

// useClickAway closes a menu on a click anywhere outside it, but on what
// toggles it (matching toggles), which does that itself.
export function useClickAway(box: RefObject<HTMLElement | null>, toggles: string, close: () => void) {
  useEffect(() => {
    const down = (e: MouseEvent) => {
      const t = e.target as Element | null;
      if (box.current?.contains(t) || t?.closest?.(toggles)) return;
      close();
    };
    document.addEventListener("mousedown", down);
    return () => document.removeEventListener("mousedown", down);
  }, [box, toggles, close]);
}

// ContentsMenu is the whole contents, opened from the pill or the Contents
// button: each section, the one you're in marked, and its link to copy. A
// click anywhere else closes it, as t and esc do.
export function ContentsMenu({ title, sections, at, anchor, go, copy, close }: { title: string; sections: Section[]; at: number; anchor: () => HTMLElement | null; go: (anchor: string) => void; copy: () => void; close: () => void }) {
  const box = useRef<HTMLDivElement>(null);
  const pos = usePlaceUnder(box, anchor);
  useClickAway(box, "[data-contents-toggle]", close);
  return (
    <div className="contents-menu" ref={box} role="dialog" aria-label="Contents" style={pos ? { top: pos.top, left: pos.left } : { visibility: "hidden" }}>
      <div className="contents-menu-head">
        <Icon name="list" size={12} />
        <b>{title}</b>
        <Button variant="link" className="contents-menu-action" type="button" onClick={copy} title="Copy a link to it">
          Copy its link <Icon name="link" size={12} />
        </Button>
      </div>
      <nav className="contents-menu-list">
        {sections.length > 0 ? (
          sections.map((s, i) => (
            <button type="button" key={s.anchor} className={`contents-menu-item l${s.level}${i === at ? " on" : ""}`} aria-current={i === at ? "location" : undefined} onClick={() => go(s.anchor)}>
              {s.text}
            </button>
          ))
        ) : (
          <p className="contents-empty">No sections.</p>
        )}
      </nav>
      <div className="contents-menu-foot">
        <Keys k="t" /> or <Keys k="esc" /> to close
      </div>
    </div>
  );
}
