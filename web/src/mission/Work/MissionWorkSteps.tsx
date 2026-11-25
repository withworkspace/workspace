import type { ReactNode } from "react";
import css from "./MissionWorkSteps.module.css";

// MissionWorkSteps is the plan's steps that ran in a capsule, joined by a
// line: a MissionWorkStep each.
export function MissionWorkSteps({ children }: { children: ReactNode }) {
  return <ol className={css.steps}>{children}</ol>;
}
