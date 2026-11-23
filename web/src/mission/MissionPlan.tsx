import type { ReactNode, Ref } from "react";
import { cx } from "../ui/cx";
import css from "./MissionPlan.module.css";

// MissionPlan is a mission's stages down a line: those done, the one it's
// on, and those to come, a MissionPlanStep each. rows: as a list of rows to
// point at (the plan in a mission's details), which reach a little past
// the list each side for their hover, and may have folds among them
// (MissionPlanFold).
export function MissionPlan({ rows, children, ref }: { rows?: boolean; children: ReactNode; ref?: Ref<HTMLOListElement> }) {
  return (
    <ol ref={ref} className={cx(css.plan, rows && css.rows)}>
      {children}
    </ol>
  );
}
