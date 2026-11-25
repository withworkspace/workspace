import type { ReactNode } from "react";
import css from "./FilesRunsNone.module.css";

// FilesRunsNone is what the Runs tab says where there's nothing to show
// (children: an Empty), in the middle of its room.
export function FilesRunsNone({ children }: { children: ReactNode }) {
  return <div className={css.none}>{children}</div>;
}
