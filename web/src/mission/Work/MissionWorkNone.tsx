import type { ReactNode } from "react";
import css from "./MissionWorkNone.module.css";

// MissionWorkNone says a group of the Work tab's list has nothing in it
// yet, and when it will.
export function MissionWorkNone({ children }: { children: ReactNode }) {
  return <p className={css.none}>{children}</p>;
}
