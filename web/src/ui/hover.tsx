import { createContext, type ReactNode, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { NoRefs } from "../refs/known";

// HoverCard says more about something on hover (or focus): a card drawn
// over the page, below it, or above when there's no room below, kept on
// screen. It waits a moment before showing, so passing over doesn't flash,
// and stays while the pointer's on it.
export function HoverCard({ trigger, children, className }: { trigger: ReactNode; children: ReactNode; className?: string }) {
  const host = useRef<HTMLSpanElement>(null);
  const hover = useHoverDelay<true>();
  return (
    <span
      ref={host}
      className={`hover-host${className ? ` ${className}` : ""}`}
      onMouseEnter={() => hover.over(true)}
      onMouseLeave={hover.out}
      onFocus={() => hover.over(true)}
      onBlur={hover.out}
      tabIndex={0}
    >
      {trigger}
      {hover.shown && host.current && (
        <FloatingCard anchor={host.current} onEnter={hover.stay} onLeave={hover.out}>
          {/* Inside a card, refs are words: no cards in cards. */}
          <RefContext.Provider value={undefined}>{children}</RefContext.Provider>
        </FloatingCard>
      )}
    </span>
  );
}

// useHoverDelay is what's hovered (shown), a moment after the pointer
// comes (over), so passing over doesn't flash, until a moment after it
// goes (out), so it can move onto the card; stay keeps it while it's on
// the card, and clear hides it at once.
export function useHoverDelay<T>(hideAfter = 120) {
  const [shown, setShown] = useState<T>();
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  const act = useMemo(
    () => ({
      over(v: T) {
        clearTimeout(timer.current);
        timer.current = setTimeout(() => setShown(() => v), 350);
      },
      out() {
        clearTimeout(timer.current);
        timer.current = setTimeout(() => setShown(undefined), hideAfter);
      },
      stay() {
        clearTimeout(timer.current);
      },
      clear() {
        clearTimeout(timer.current);
        setShown(undefined);
      },
    }),
    [hideAfter],
  );
  return { shown, ...act };
}

// FloatingCard is a hover card's panel: under what it's about (anchor), or
// over it where there's no room below, kept on screen, and placed again as
// it grows (what it shows arrives). Given beside (a list the anchor is in,
// say), it goes at that's side instead, level with the anchor, so it
// doesn't cover what's next in the list: left of it, or right where
// there's no room on the left; under the anchor, as ever, where there's
// room on neither.
export function FloatingCard({
  anchor,
  beside,
  className,
  onEnter,
  onLeave,
  children,
}: {
  anchor: HTMLElement;
  beside?: HTMLElement | null;
  className?: string;
  onEnter: () => void;
  onLeave: () => void;
  children: ReactNode;
}) {
  const card = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ top: number; left: number }>();
  useLayoutEffect(() => {
    const place = () => {
      if (!card.current || !anchor.isConnected) return;
      const at = anchor.getBoundingClientRect();
      const c = card.current.getBoundingClientRect();
      if (beside?.isConnected) {
        const edge = beside.getBoundingClientRect();
        const fits = (x: number) => x >= 12 && x + c.width <= window.innerWidth - 12;
        const x = [edge.left - 10 - c.width, edge.right + 10].find(fits);
        if (x !== undefined) {
          setPos({ top: Math.max(12, Math.min(at.top - 10, window.innerHeight - c.height - 12)), left: x });
          return;
        }
      }
      const left = Math.max(12, Math.min(at.left - 6, window.innerWidth - c.width - 12));
      const below = at.bottom + 6;
      setPos({ top: below + c.height > window.innerHeight - 12 ? Math.max(12, at.top - 6 - c.height) : below, left });
    };
    place();
    const grown = new ResizeObserver(place);
    if (card.current) grown.observe(card.current);
    return () => grown.disconnect();
  }, [anchor, beside]);
  return createPortal(
    <div
      ref={card}
      className={`hovercard${className ? ` ${className}` : ""}`}
      role="tooltip"
      style={pos ? { top: pos.top, left: pos.left } : { top: -9999, left: -9999 }}
      onMouseEnter={onEnter}
      onMouseLeave={onLeave}
    >
      <NoRefs>{children}</NoRefs>
    </div>,
    document.body,
  );
}

// Refs are the names a page knows (c1, d2, t3, a capsule, a document) and
// how to draw each: text that mentions one shows it with its hover card.
export interface Refs {
  pattern: RegExp; // matches any of them, as whole words
  render: (ref: string, key: number) => ReactNode | null;
}

export const RefContext = createContext<Refs | undefined>(undefined);

// linkRefs draws the refs in text.
export function linkRefs(text: string, refs: Refs | undefined, at = 0): ReactNode[] {
  if (!refs) return [text];
  const out: ReactNode[] = [];
  let last = 0;
  for (const m of text.matchAll(refs.pattern)) {
    const i = m.index ?? 0;
    const drawn = refs.render(m[0], at + i);
    if (!drawn) continue;
    if (i > last) out.push(text.slice(last, i));
    out.push(drawn);
    last = i + m[0].length;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}
