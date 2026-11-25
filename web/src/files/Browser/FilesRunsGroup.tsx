import type { ReactNode } from "react";
import { GroupHead } from "../../ui/GroupHead";
import { Rows } from "../../ui/Rows";
import css from "./FilesRunsGroup.module.css";

// FilesRunsGroup is one group of a worktree's runs (its apps, its tools,
// its tasks) under its head: what can be run from it (start, its task
// recipes not running), then its rows, or what's said with none (quiet).
export function FilesRunsGroup({ title, count, start, quiet, children }: { title: string; count?: number; start?: ReactNode; quiet?: ReactNode; children?: ReactNode }) {
  return (
    <section className={css.group}>
      <GroupHead count={count} className={css.head}>
        {title}
      </GroupHead>
      {start && <div className={css.start}>{start}</div>}
      {quiet ? <p className={css.quiet}>{quiet}</p> : <Rows className={css.rows}>{children}</Rows>}
    </section>
  );
}
