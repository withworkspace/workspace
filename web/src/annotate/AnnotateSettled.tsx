import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./AnnotateSettled.module.css";

// AnnotateSettled is a quiet line of what's come of a thread: an edit
// accepted or declined, a decision recorded, crew finished.
export function AnnotateSettled({ children }: { children: ReactNode }) {
  return <div className={cx("dim", css.settled)}>{children}</div>;
}
