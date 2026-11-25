import type { ReactNode } from "react";
import css from "./SaidList.module.css";

// SaidList is what's been said on a pull request itself, one under another.
export function SaidList({ children }: { children: ReactNode }) {
  return <div className={css.list}>{children}</div>;
}
