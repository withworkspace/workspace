import type { ReactNode } from "react";
import css from "./RunPanelNeeds.module.css";

// RunPanelNeeds is what a run needs, in its panel: a RunPanelNeed each.
export function RunPanelNeeds({ children }: { children: ReactNode }) {
  return <ul className={css.needs}>{children}</ul>;
}

// RunPanelNeed is one of them: its name, then what meets it (another
// run, an address outside ws, a repo's folder) and how it's handed over
// (its children, RunPanelAsides among them).
export function RunPanelNeed({ name, children }: { name: string; children: ReactNode }) {
  return (
    <li>
      <span className={css.need}>{name}</span>
      {children}
    </li>
  );
}
