import type { ReactNode } from "react";
import css from "./WindowNotice.module.css";

// WindowNotice is a page of its own in place of the whole window, when
// there's nothing else to show: the daemon was closed, or this page isn't
// signed in. ws's mark, what happened (title), and what to do.
export function WindowNotice({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className={css.notice}>
      <div className={css.mark} aria-hidden>
        w
      </div>
      <h1>{title}</h1>
      <p>{children}</p>
    </div>
  );
}
