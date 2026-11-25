import type { ReactNode } from "react";
import css from "./RunsPageLede.module.css";

// RunsPageLede is what the Runs page is for, under its head.
export function RunsPageLede({ children }: { children: ReactNode }) {
  return <p className={css.lede}>{children}</p>;
}
