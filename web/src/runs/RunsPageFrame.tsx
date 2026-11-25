import type { ReactNode } from "react";
import css from "./RunsPageFrame.module.css";

// RunsPageFrame is the Runs page's frame: a page's width and room, as
// every page has.
export function RunsPageFrame({ children }: { children: ReactNode }) {
  return <div className={css.page}>{children}</div>;
}
