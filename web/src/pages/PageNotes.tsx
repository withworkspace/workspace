import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PageNotes.module.css";

// PageNotes is what a cell's agent had to say of what it wrote: to you, in
// its panel, never in the page. old is a version's that isn't the latest.
export function PageNotes({ old, children }: { old?: boolean; children: ReactNode }) {
  return (
    <div className={cx(css.notes, old && css.old)}>
      <span className={cx("dim", css.label)}>Its agent's notes, to you</span>
      {children}
    </div>
  );
}
