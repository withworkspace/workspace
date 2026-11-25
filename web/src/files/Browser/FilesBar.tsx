import type { ReactNode } from "react";
import css from "./FilesBar.module.css";

// The class of the crumbs in a FilesBar's where: they sit on its line.
export const filesBarCrumbsClass = css.crumbs;

// FilesBar is the row across the top of a worktree: how many panes (at its
// start), where it is (where), its tabs, and at its end what it's part of
// (aside: a capsule's mission).
export function FilesBar({ panes, where, tabs, aside }: { panes?: ReactNode; where: ReactNode; tabs?: ReactNode; aside?: ReactNode }) {
  return (
    <div className={css.bar}>
      {panes}
      <div className={css.where}>{where}</div>
      {tabs}
      {aside && <div className={css.aside}>{aside}</div>}
    </div>
  );
}
