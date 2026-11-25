import type { ComponentProps, ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./FilesTree.module.css";

// FilesTree is the tree's pane: its tools at its head, then the tree
// itself (children: a ui/Tree, which scrolls in what's left, or what's
// said while there's none), and a note under it. On the Files page, with
// the repos before it, it's on the panel (the repos are the clear pane).
// rows is what the page listens for over the tree (which key picked a row).
export function FilesTree({ tools, note, panel, rows, children }: { tools: ReactNode; note?: ReactNode; panel?: boolean; rows?: Pick<ComponentProps<"div">, "onKeyDownCapture" | "onPointerDownCapture">; children: ReactNode }) {
  return (
    <aside className={cx(css.tree, panel && css.panel)} aria-label="Files">
      <div className={css.head}>{tools}</div>
      <div className={css.rows} {...rows}>
        {children}
      </div>
      {note && <div className={css.note}>{note}</div>}
    </aside>
  );
}

// filesTreeClass is the tree's own, in the pane: room around its rows.
export const filesTreeClass = css.list;
