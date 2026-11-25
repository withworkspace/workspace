import type { ReactNode } from "react";
import css from "./FilesPick.module.css";

// FilesPick is what's said beside the repos with none open (children: an
// Empty), in the middle of a solid pane.
export function FilesPick({ children }: { children: ReactNode }) {
  return <div className={css.pick}>{children}</div>;
}
