import type { ReactNode } from "react";
import css from "./PageMapSum.module.css";

// PageMapSum is the map's first line: chips for what's open, out of date,
// not filled and filling.
export function PageMapSum({ children }: { children: ReactNode }) {
  return <div className={css.sum}>{children}</div>;
}
