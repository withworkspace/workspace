import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./FilesView.module.css";

// FilesView is the file's pane, beside the tree: the file open, or what's
// said with none. Solid while the tree's beside it (the tree is the clear
// pane, or the repos are); clear alone.
export function FilesView({ solid, children }: { solid?: boolean; children: ReactNode }) {
  return (
    <section className={cx(css.view, solid && css.solid)} aria-label="File">
      {children}
    </section>
  );
}
