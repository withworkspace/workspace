import type { ReactNode, Ref } from "react";
import { cx } from "../ui/cx";
import css from "./PagePanel.module.css";

// PagePanel is what's opened over the page's words, at a place in them:
// a cell's panel, the box that asks for a change, or (note) a pass's
// finding, set in and padded as a comment's card is. With no place yet
// it's laid out unseen, so its size is known before it's placed.
export function PagePanel({ ref, at, label, note, children }: { ref?: Ref<HTMLDivElement>; at?: { top: number; left: number }; label: string; note?: boolean; children: ReactNode }) {
  return (
    <div className={cx(css.panel, note && css.note)} ref={ref} style={at ? { top: at.top, left: at.left } : { visibility: "hidden" }} role="dialog" aria-label={label}>
      {children}
    </div>
  );
}
