import type { ReactNode } from "react";
import css from "./FilesOverview.module.css";

// FilesOverview is what the file's pane says with no file open (what's
// changed, and how to find the rest), or of a file that isn't there: a
// column in the middle of the pane.
export function FilesOverview({ children }: { children: ReactNode }) {
  return <div className={css.overview}>{children}</div>;
}
