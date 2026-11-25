import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./ReviewViewBody.module.css";

// ReviewViewBody is one of a review's views in its pane: hidden while
// another shows, keeping what it holds. One that's read (read), as
// against the changes, keeps to a measure.
export function ReviewViewBody({ label, hidden, read, children }: { label: string; hidden: boolean; read?: boolean; children: ReactNode }) {
  return (
    <section className={cx(css.view, read && css.read)} hidden={hidden} aria-label={label}>
      {children}
    </section>
  );
}
