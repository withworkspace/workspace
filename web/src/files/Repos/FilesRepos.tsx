import type { ReactNode } from "react";
import css from "./FilesRepos.module.css";

// FilesRepos is the workspace's repos, the Files page's column: its head
// and how many, over the groups of them (children).
export function FilesRepos({ count, children }: { count: number; children: ReactNode }) {
  return (
    <nav className={css.repos} aria-label="Repos">
      <div className={css.head}>
        <h1>Repos</h1>
        <span className="dim">{count}</span>
      </div>
      <div className={css.list}>{children}</div>
    </nav>
  );
}
