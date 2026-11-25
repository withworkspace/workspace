import type { ReactNode } from "react";
import css from "./MissionBriefPlanProblem.module.css";

// MissionBriefPlanProblem is why ws didn't save a plan, as ws said it:
// every problem, a line each.
export function MissionBriefPlanProblem({ children }: { children: ReactNode }) {
  return <pre className={css.problem}>{children}</pre>;
}
