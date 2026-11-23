import { type CSSProperties, type KeyboardEvent, type ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import css from "./DockTabs.module.css";

type DockTabsProps = {
  label: string;
  // The list of every tab (DockTabList), for when they don't all fit: it's
  // drawn at the row's end only then.
  list?: ReactNode;
  // Changes each time the tab in front is asked for again (picked from
  // the list while it was already in front, scrolled out of sight): it's
  // brought back into sight.
  shown?: number;
  children: ReactNode;
};

// How near an end counts as at it: a pixel's rounding either way.
const near = 2;

// DockTabs is what's open in a dock, a tab each (DockTab). The arrow keys
// go from tab to tab, opening each as they reach it; Home and End go to
// the first and the last.
//
// With more than fit, the tabs give up room first (all but the one in
// front, down to a few letters each), and then the row scrolls sideways:
// by a trackpad as it is, by a mouse's wheel turned the usual way, and
// the one in front is always brought into sight. It fades at whichever
// end has more, and the list of them all is at its end, for whoever
// would sooner pick from a list than scroll.
export function DockTabs({ label, list, shown, children }: DockTabsProps) {
  const row = useRef<HTMLDivElement>(null);
  const [more, setMore] = useState({ before: false, after: false });

  // The tab in front, and whether all of it is in the row's sight.
  const front = () => row.current?.querySelector<HTMLElement>('[role="tab"][aria-selected="true"]')?.parentElement;
  const sighted = (): boolean => {
    const [el, on] = [row.current, front()];
    if (!el || !on) return true;
    const [r, o] = [el.getBoundingClientRect(), on.getBoundingClientRect()];
    return o.left >= r.left - near && o.right <= r.right + near;
  };
  const reveal = () => front()?.scrollIntoView?.({ block: "nearest", inline: "nearest" });
  // Whether it was, when last looked at: so it can be told that the
  // row's room changed under it from that you scrolled away from it.
  const inSight = useRef(true);

  // Which ends have more: as the row's scrolled, as it's resized, and as
  // tabs come and go. A tab in front that a change of room has pushed out
  // of sight (the names arriving, the dock made narrower) is brought
  // back; one you scrolled away from is left where you put it.
  useLayoutEffect(() => {
    const el = row.current;
    if (!el) return;
    const look = () => {
      const before = el.scrollLeft > near;
      const after = el.scrollLeft + el.clientWidth < el.scrollWidth - near;
      setMore((m) => (m.before === before && m.after === after ? m : { before, after }));
    };
    const scrolled = () => {
      inSight.current = sighted();
      look();
    };
    const resized = () => {
      if (inSight.current && !sighted()) reveal();
      look();
    };
    look();
    el.addEventListener("scroll", scrolled, { passive: true });
    const sized = typeof ResizeObserver === "undefined" ? undefined : new ResizeObserver(resized);
    sized?.observe(el);
    for (const tab of el.children) sized?.observe(tab);
    return () => {
      el.removeEventListener("scroll", scrolled);
      sized?.disconnect();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [children]);

  // A mouse's wheel turns up and down: over the row, that's along it.
  // (A trackpad swiped sideways already is, and is left alone.)
  useEffect(() => {
    const el = row.current;
    if (!el) return;
    const wheel = (e: WheelEvent) => {
      if (e.ctrlKey || Math.abs(e.deltaY) <= Math.abs(e.deltaX) || el.scrollWidth <= el.clientWidth) return;
      e.preventDefault();
      el.scrollLeft += e.deltaMode === 1 ? e.deltaY * 32 : e.deltaY;
    };
    el.addEventListener("wheel", wheel, { passive: false });
    return () => el.removeEventListener("wheel", wheel);
  }, []);

  // The one in front is in sight: when another comes to the front (picked
  // from the list, by the keys, by the page), when it's asked for again
  // (shown), and when the list comes or goes at the row's end, which
  // changes the row's room. Not otherwise: scrolling away from it is
  // yours to do.
  const overflowing = more.before || more.after;
  const seen = useRef<{ on?: Element | null; overflowing: boolean; shown?: number }>({ overflowing: false });
  useLayoutEffect(() => {
    const on = front();
    if (on === seen.current.on && overflowing === seen.current.overflowing && shown === seen.current.shown) return;
    seen.current = { on, overflowing, shown };
    reveal();
    inSight.current = true;
  });

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const tabs = [...e.currentTarget.querySelectorAll<HTMLElement>('[role="tab"]')];
    const at = tabs.findIndex((t) => t === document.activeElement);
    if (at < 0) return;
    const to = e.key === "ArrowRight" ? at + 1 : e.key === "ArrowLeft" ? at - 1 : e.key === "Home" ? 0 : e.key === "End" ? tabs.length - 1 : undefined;
    const next = to === undefined ? undefined : tabs[(to + tabs.length) % tabs.length];
    if (!next) return;
    e.preventDefault();
    next.focus();
    next.click();
  };
  const fades = { "--fade-before": more.before ? "20px" : "0px", "--fade-after": more.after ? "20px" : "0px" } as CSSProperties;
  return (
    <div className={css.wrap}>
      <div ref={row} role="tablist" aria-label={label} className={css.tabs} style={fades} onKeyDown={onKeyDown}>
        {children}
      </div>
      {overflowing && list}
    </div>
  );
}
