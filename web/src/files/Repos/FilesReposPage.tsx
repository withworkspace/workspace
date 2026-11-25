import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./FilesReposPage.module.css";

// FilesReposPage is the Files page with no repo open: the repos (column),
// and beside them what's said of none (children: a FilesPick), where
// there's room for both.
export function FilesReposPage({ narrow, column, children }: { narrow?: boolean; column: ReactNode; children?: ReactNode }) {
  return (
    <div className={cx(css.page, narrow && css.narrow)}>
      {column}
      {children}
    </div>
  );
}
