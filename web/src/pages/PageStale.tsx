import type { ReactNode } from "react";
import css from "./PageStale.module.css";

// PageStale says what's changed since a cell was written, and offers to
// write it again (children).
export function PageStale({ words, children }: { words: ReactNode; children: ReactNode }) {
  return (
    <div className={css.stale}>
      <span>{words}</span>
      {children}
    </div>
  );
}
