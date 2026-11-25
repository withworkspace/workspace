import type { ReactNode, Ref } from "react";
import { cx } from "../../ui/cx";
import css from "./FilesTree.module.css";

// FilesTree is the tree's pane: its tools at its head, then its rows
// (children), and a note under them. On the Files page, with the repos
// before it, it's on the panel (the repos are the clear pane).
export function FilesTree({ label, tools, note, panel, rowsRef, children }: { label: string; tools: ReactNode; note?: ReactNode; panel?: boolean; rowsRef?: Ref<HTMLDivElement>; children: ReactNode }) {
  return (
    <aside className={cx(css.tree, panel && css.panel)} aria-label="Files">
      <div className={css.head}>{tools}</div>
      <div className={css.rows} ref={rowsRef} role="tree" aria-label={label}>
        {children}
        {note && <div className={css.note}>{note}</div>}
      </div>
    </aside>
  );
}
