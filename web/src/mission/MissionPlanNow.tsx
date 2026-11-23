import type { ReactNode } from "react";
import css from "./MissionPlanNow.module.css";

// MissionPlanNow is the step the plan is on, said in place under its row
// where that says more than the row does: its kind and where, what it
// makes, who's on it, its clock, and each check with how it stands, a
// MissionPlanLine each.
export function MissionPlanNow({ children }: { children: ReactNode }) {
  return (
    <div className={css.now} data-plan-now="">
      {children}
    </div>
  );
}
