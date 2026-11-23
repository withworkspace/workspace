import type { ReactNode } from "react";
import css from "./MissionPlan.module.css";

// MissionPlan is a mission's stages down a line: those done, the one it's
// on, and those to come, a MissionPlanStep each.
export function MissionPlan({ children }: { children: ReactNode }) {
  return <ol className={css.plan}>{children}</ol>;
}
