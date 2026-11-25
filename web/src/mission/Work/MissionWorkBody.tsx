import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionWorkBody.module.css";

// MissionWorkBody is what's under the head of what's open in the Work
// tab: a column to read, and beside it (files) a capsule's changed files.
// On a wide window each column scrolls by itself; narrower, the files go
// under the rest.
export function MissionWorkBody({ children, files }: { children: ReactNode; files?: ReactNode }) {
  return (
    <div className={cx(css.body, !files && css.one)}>
      <div className={css.col}>{children}</div>
      {files && <div className={css.col}>{files}</div>}
    </div>
  );
}
