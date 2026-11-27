import type { ReactNode } from "react";
import css from "./AutomationRuns.module.css";

// AutomationRuns is an automation's runs, the latest first: each an
// AutomationRunItem, in one bordered list.
export function AutomationRuns({ children }: { children: ReactNode }) {
  return <ol className={css.runs}>{children}</ol>;
}
