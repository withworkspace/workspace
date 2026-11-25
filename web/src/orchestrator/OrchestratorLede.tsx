import type { ReactNode } from "react";
import css from "./OrchestratorLede.module.css";

// OrchestratorLede is what the orchestrator is, said under the page's head.
export function OrchestratorLede({ children }: { children: ReactNode }) {
  return <p className={css.lede}>{children}</p>;
}
