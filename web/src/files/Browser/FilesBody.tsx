import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./FilesBody.module.css";

// FilesBody is the tree and the file side by side, or one alone (single:
// a narrow window, or the file alone as the panes are picked).
export function FilesBody({ single, children }: { single?: boolean; children: ReactNode }) {
  return <div className={cx(css.body, single && css.single)}>{children}</div>;
}

// FilesOther is what another tab shows (its runs, an app): the room of the
// tree and the file, both.
export function FilesOther({ children }: { children: ReactNode }) {
  return <div className={css.other}>{children}</div>;
}
