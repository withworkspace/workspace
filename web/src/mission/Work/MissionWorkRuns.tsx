import type { ReactNode } from "react";
import css from "./MissionWorkRuns.module.css";

// MissionWorkRuns is a capsule's runs: a MissionWorkRun each.
export function MissionWorkRuns({ children }: { children: ReactNode }) {
  return <ul className={css.runs}>{children}</ul>;
}
