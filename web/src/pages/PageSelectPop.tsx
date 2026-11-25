import type { ReactNode } from "react";
import css from "./PageSelectPop.module.css";

// PageSelectPop is under words selected in the page: what to do with them
// (ask about them, or for a change). It doesn't take the selection away.
export function PageSelectPop({ top, left, children }: { top: number; left: number; children: ReactNode }) {
  return (
    <div className={css.pop} style={{ top, left }} role="toolbar" aria-label="The words selected" onMouseDown={(e) => e.preventDefault()}>
      {children}
    </div>
  );
}
