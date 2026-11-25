import type { ReactNode } from "react";
import css from "./OrchestratorNone.module.css";

// OrchestratorNone is a line said where a list would be, with nothing in
// it: no mission it's on, nothing said, no calls.
export function OrchestratorNone({ children }: { children: ReactNode }) {
  return <div className={css.none}>{children}</div>;
}
