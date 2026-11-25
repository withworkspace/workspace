import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./FilesExplorer.module.css";

// FilesExplorer is a worktree's page: the repos beside it (before, the
// Files page's), then the worktree: its bar across the top of what its tab
// shows (children). What's opened over it (a finder) is after.
export function FilesExplorer({ before, after, children }: { before?: ReactNode; after?: ReactNode; children: ReactNode }) {
  return (
    <div className={cx(css.explorer, !!before && css.withBefore)}>
      {before}
      <div className={css.main}>{children}</div>
      {after}
    </div>
  );
}
