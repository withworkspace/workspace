import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./FindPane.module.css";

// FindPane is the pane beside Find's list, for what the cursor's on: a
// card's lines, at a width that doesn't change with what they say.
// scrolls: what it shows may be longer than Find is tall (what a plugin's
// thing says), so it scrolls.
//
// What it holds is laid out as a ref's card is, and may be one
// (RefCardBody): it's a ref-card to the rules for those.
export function FindPane({ scrolls, children }: { scrolls?: boolean; children: ReactNode }) {
  return (
    <aside className={cx(css.pane, scrolls && css.scrolls, "ref-card")} aria-label="About it">
      {children}
    </aside>
  );
}
