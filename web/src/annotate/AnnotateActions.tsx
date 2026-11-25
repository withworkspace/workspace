import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./AnnotateActions.module.css";

// AnnotateActions is a line of what's pressed at the foot of something in
// a thread (a suggested edit, a decision, a new comment): start at its
// start, the rest at its end. Where it sits is its holder's, by className.
export function AnnotateActions({ start, className, children }: { start?: ReactNode; className?: string; children: ReactNode }) {
  return (
    <div className={cx(css.actions, className)}>
      {start}
      <span className={css.spacer} />
      {children}
    </div>
  );
}
