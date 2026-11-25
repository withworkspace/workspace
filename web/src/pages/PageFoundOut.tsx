import type { ReactNode } from "react";
import css from "./PageFoundOut.module.css";

// PageFoundOut is what was found out about a finding, under it on its
// pass's tab: what's been said (PageTurns, flush), and a box to ask more.
export function PageFoundOut({ children }: { children: ReactNode }) {
  return <div className={css.foundOut}>{children}</div>;
}
