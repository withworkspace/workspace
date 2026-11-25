import { type ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { Anchor } from "../../ui/FloatingCard";
import css from "./OverFrame.module.css";

// OverFrame is a panel of ws's by something in a plugin's page: under the
// button it was asked from, its right edge on the button's where there's
// no room to its right, kept in the window. Escape, or a press anywhere
// else, dismisses it, unless it's busy.
export function OverFrame({ anchor, label, busy, dismiss, children }: { anchor?: Anchor; label: string; busy: boolean; dismiss: () => void; children: ReactNode }) {
  const panel = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ top: number; left: number }>();
  useLayoutEffect(() => {
    const place = () => {
      if (!panel.current) return;
      const p = panel.current.getBoundingClientRect();
      if (!anchor?.isConnected) {
        setPos({ top: 72, left: Math.max(12, (window.innerWidth - p.width) / 2) });
        return;
      }
      const at = anchor.getBoundingClientRect();
      const left = at.left + p.width <= window.innerWidth - 12 ? at.left : at.right - p.width;
      const below = at.bottom + 6;
      setPos({
        top: below + p.height > window.innerHeight - 12 ? Math.max(12, at.top - 6 - p.height) : below,
        left: Math.max(12, Math.min(left, window.innerWidth - p.width - 12)),
      });
    };
    place();
    const grown = new ResizeObserver(place);
    if (panel.current) grown.observe(panel.current);
    window.addEventListener("resize", place);
    return () => {
      grown.disconnect();
      window.removeEventListener("resize", place);
    };
  }, [anchor]);
  // What's said through a ref, so the listeners are set once.
  const leave = useRef(dismiss);
  leave.current = busy ? () => {} : dismiss;
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      e.stopPropagation();
      leave.current();
    };
    const press = (e: PointerEvent) => {
      if (e.target instanceof Node && panel.current?.contains(e.target)) return;
      leave.current();
    };
    document.addEventListener("keydown", key, true);
    document.addEventListener("pointerdown", press, true);
    return () => {
      document.removeEventListener("keydown", key, true);
      document.removeEventListener("pointerdown", press, true);
    };
  }, []);
  return createPortal(
    <>
      {/* The frame takes no presses while it's up: one on it dismisses, as one anywhere does. */}
      <div className={css.catch} onPointerDown={() => leave.current()} />
      <div ref={panel} className={css.panel} role="dialog" aria-label={label} style={pos ? { top: pos.top, left: pos.left } : { top: -9999, left: -9999 }}>
        {children}
      </div>
    </>,
    document.body,
  );
}
