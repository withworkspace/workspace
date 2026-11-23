import type { ReactNode } from "react";
import css from "./FindNone.module.css";

// FindNone is what Find's list says with nothing in it: that nothing
// matches, or what to type.
export function FindNone({ children }: { children: ReactNode }) {
  return <p className={css.none}>{children}</p>;
}
