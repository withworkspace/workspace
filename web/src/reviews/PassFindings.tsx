import type { ReactNode } from "react";
import css from "./PassFindings.module.css";

// PassFindings are what a pass found, one under another (PassFinding).
export function PassFindings({ children }: { children: ReactNode }) {
  return <ul className={css.findings}>{children}</ul>;
}
