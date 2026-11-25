import type { ReactNode } from "react";
import css from "./PageSources.module.css";

// PageSources are the files a cell was written from, under it in its
// panel: a PageSourceChip each.
export function PageSources({ children }: { children: ReactNode }) {
  return (
    <div className={css.sources}>
      <span className="dim">From</span>
      {children}
    </div>
  );
}
