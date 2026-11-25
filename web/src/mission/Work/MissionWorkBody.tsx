import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionWorkBody.module.css";

// MissionWorkBody is what's under the head of what's open in the Work
// tab: a column to read, and beside it (files) a capsule's changed files.
// On a wide window each column scrolls by itself; narrower, the files go
// under the rest. fills is the tab filling what's under the mission's
// bar, as the mission page's does: only then do its columns scroll.
export function MissionWorkBody({ children, files, fills }: { children: ReactNode; files?: ReactNode; fills?: boolean }) {
  return (
    <div className={cx(css.body, !files && css.one, fills && css.fills)}>
      <div className={css.col}>{children}</div>
      {files && <div className={css.col}>{files}</div>}
    </div>
  );
}
