import type { ReactNode } from "react";
import css from "./PageKindPicks.module.css";

// PageKindPicks are the workspace's kinds of document, to start a page
// as one of: a PageKindPick each.
export function PageKindPicks({ children }: { children: ReactNode }) {
  return (
    <div className={css.picks} role="radiogroup" aria-label="Kind of document">
      {children}
    </div>
  );
}
