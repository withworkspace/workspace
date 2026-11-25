import type { ReactNode } from "react";
import css from "./MissionWorkMade.module.css";

// MissionWorkMade is what was made in the capsule that's open: its
// outputs, each a MissionWorkRow, which reach the column's edges.
export function MissionWorkMade({ children }: { children: ReactNode }) {
  return <div className={css.made}>{children}</div>;
}
