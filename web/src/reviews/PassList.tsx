import type { ReactNode } from "react";
import css from "./PassList.module.css";

// PassList is a review's passes, one under another.
export function PassList({ children }: { children: ReactNode }) {
  return <div className={css.list}>{children}</div>;
}
