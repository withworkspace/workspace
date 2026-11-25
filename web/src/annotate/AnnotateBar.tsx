import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./AnnotateBar.module.css";

// AnnotateBar is the line over a document being commented on: how its
// threads stand (chips, at its start), and what's done to it (at its end:
// send changes to crew, show what's changed, approve).
export function AnnotateBar({ counts, children }: { counts?: ReactNode; children?: ReactNode }) {
  return (
    <div className={css.bar}>
      <span className={css.counts}>{counts}</span>
      <span className={css.spacer} />
      {children}
    </div>
  );
}

// AnnotateBarNote is a few quiet words among what's done, in the bar.
export function AnnotateBarNote({ children }: { children: ReactNode }) {
  return <span className={cx("dim", css.note)}>{children}</span>;
}
