import type { ReactNode } from "react";
import css from "./BelowRunsList.module.css";

// BelowRunsList is Below's Runs' services, a BelowRunRow each.
export function BelowRunsList({ children }: { children: ReactNode }) {
  return <ul className={css.list}>{children}</ul>;
}
