import type { ReactNode } from "react";
import css from "./PageMap.module.css";

// PageMap is the page's map, beside it: a PageMapSum of what's open or
// out of date, then PageMapSections (its outline, cells, passes, what's
// open, the files it rests on), or a PageMapEmpty.
export function PageMap({ children }: { children: ReactNode }) {
  return (
    <aside className={css.map} aria-label="Page map">
      {children}
    </aside>
  );
}
